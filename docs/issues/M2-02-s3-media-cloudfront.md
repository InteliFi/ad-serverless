---
title: "[M2-02] S3 bucket de mídia + CloudFront"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P1"]
milestone: "M2 — Infra AWS"
---
## Contexto

O legado cacheia vídeos de campanhas VAST em disco local (`/tmp/adserver_video_cache[_prod]`, [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 "Video cache") e os serve por `GET /media/{filename}` como `video/mp4` ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §15). O `VastService` reescreve `<MediaFile type="video/mp4">` para `https://ads.inteli.fi/media/{md5}.mp4` ([docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2.2 e §4).

Em Lambda não existe disco compartilhado entre containers — a decisão de arquitetura ([ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 "Mídia") é **S3 + CloudFront**: o S3 substitui o diretório local e o CloudFront entrega da borda. Esta issue provisiona SOMENTE a infraestrutura; a lógica de cache (whitelist `gcdn.2mdn.net`/`googlevideo.com`, bypass de URLs assinadas, download com headers do cliente, skip não-mp4) é da issue **M5-07** (`media-handler` + video cache).

## Especificação detalhada

### 1. Convenção de chave de objeto (contrato com M5-07 — documentar no serverless.yml)

- Chave = **`MD5(url).mp4`** — hex minúsculo do MD5 da URL original do vídeo, paridade exata com o `VideoCacheService` Java (`Nome do arquivo: MD5(url).mp4`, [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §4). Sem prefixos/pastas.
- `Content-Type: video/mp4` definido no upload (responsabilidade do media-handler, M5-07).
- Sem lifecycle de expiração na fase 1: o cache legado era indefinido ("Vídeo | disco + map em memória | indefinido", 03 §9) — expirar objetos seria mudança de comportamento (`melhoria` futura, se desejada, em issue separada).

### 2. Bucket S3 (seção `resources:` do `serverless.yml`)

```yaml
MediaBucket:
  Type: AWS::S3::Bucket
  Properties:
    BucketName: ad-serverless-media-${sls:stage}     # dev → us-east-1, prod → sa-east-1
    PublicAccessBlockConfiguration:
      BlockPublicAcls: true
      BlockPublicPolicy: true
      IgnorePublicAcls: true
      RestrictPublicBuckets: true
    OwnershipControls:
      Rules: [{ ObjectOwnership: BucketOwnerEnforced }]
    BucketEncryption:
      ServerSideEncryptionConfiguration:
        - ServerSideEncryptionByDefault: { SSEAlgorithm: AES256 }
```

Bucket **privado**: nenhum acesso público; leitura externa SOMENTE via CloudFront (OAC); leitura/escrita interna SOMENTE pelo `media-handler` (permissão na M2-04).

### 3. CloudFront com OAC (Origin Access Control)

```yaml
MediaOAC:
  Type: AWS::CloudFront::OriginAccessControl
  Properties:
    OriginAccessControlConfig:
      Name: ad-serverless-media-oac-${sls:stage}
      OriginAccessControlOriginType: s3
      SigningBehavior: always
      SigningProtocol: sigv4

MediaDistribution:
  Type: AWS::CloudFront::Distribution
  Properties:
    DistributionConfig:
      Comment: ad-serverless media (videos VAST) - ${sls:stage}
      Enabled: true
      HttpVersion: http2and3
      PriceClass: PriceClass_All        # OBRIGATÓRIO: inclui as edges da América do Sul (players no Brasil)
      Origins:
        - Id: media-s3
          DomainName: !GetAtt MediaBucket.RegionalDomainName
          OriginAccessControlId: !Ref MediaOAC
          S3OriginConfig: { OriginAccessIdentity: "" }   # vazio = usa OAC (não OAI legada)
      DefaultCacheBehavior:
        TargetOriginId: media-s3
        ViewerProtocolPolicy: redirect-to-https
        AllowedMethods: [GET, HEAD]
        CachedMethods: [GET, HEAD]
        CachePolicyId: 658327ea-f89d-4fab-a63d-7e88639e58f6   # Managed-CachingOptimized
        Compress: false                  # mp4 já é comprimido; gzip/brotli só desperdiça CPU de borda
```

Política do bucket: permitir `s3:GetObject` apenas para o service principal `cloudfront.amazonaws.com` condicionado ao ARN desta distribuição:

```yaml
MediaBucketPolicy:
  Type: AWS::S3::BucketPolicy
  Properties:
    Bucket: !Ref MediaBucket
    PolicyDocument:
      Statement:
        - Effect: Allow
          Principal: { Service: cloudfront.amazonaws.com }
          Action: s3:GetObject
          Resource: !Sub "${MediaBucket.Arn}/*"
          Condition:
            StringEquals:
              AWS:SourceArn: !Sub "arn:aws:cloudfront::${AWS::AccountId}:distribution/${MediaDistribution}"
```

Sem domínio customizado nesta fase: as URLs `https://ads.inteli.fi/media/...` continuam atendidas pelo legado até o cutover (ADR-007); o domínio `d*.cloudfront.net` é consumido internamente pelo `media-handler` (redirect 302 — ARQUITETURA §3).

### 4. Variáveis de ambiente e Outputs (contrato com M5-07)

```yaml
# env (provider ou função media-handler quando existir):
MEDIA_BUCKET: !Ref MediaBucket
MEDIA_CDN_DOMAIN: !GetAtt MediaDistribution.DomainName

# Outputs:
MediaBucketName:      { Value: !Ref MediaBucket }
MediaCdnDomainName:   { Value: !GetAtt MediaDistribution.DomainName }
MediaDistributionId:  { Value: !Ref MediaDistribution }
```

`MediaDistributionId` também será consumido pela M2-06 (associação do WAF).

## Arquivos a criar/alterar

- `serverless.yml` (resources: `MediaBucket`, `MediaBucketPolicy`, `MediaOAC`, `MediaDistribution`; env `MEDIA_BUCKET`/`MEDIA_CDN_DOMAIN`; Outputs)
- `docs/MATRIZ-PARIDADE.md` (linha "video cache /tmp → S3+CloudFront": infra provisionada)

## Critérios de aceite

- [ ] Deploy dev cria bucket `ad-serverless-media-dev` (us-east-1) com Block Public Access total, SSE-S3 e BucketOwnerEnforced
- [ ] Distribuição CloudFront ativa com OAC sigv4 (`aws cloudfront get-distribution --id <id>` mostra `OriginAccessControlId` preenchido e `S3OriginConfig.OriginAccessIdentity` vazio)
- [ ] Teste de paridade da chave: calcular `MD5("https://exemplo.com/video.mp4")`, fazer `aws s3 cp video.mp4 s3://ad-serverless-media-dev/<md5>.mp4 --content-type video/mp4` e `curl -I https://<dominio-cloudfront>/<md5>.mp4` retorna `200` com `Content-Type: video/mp4`
- [ ] Acesso direto ao S3 sem assinatura (`curl https://ad-serverless-media-dev.s3.amazonaws.com/<md5>.mp4`) retorna `403`
- [ ] Segundo `curl -I` na mesma URL retorna `X-Cache: Hit from cloudfront`
- [ ] `PriceClass_All` configurado (edges América do Sul); `AllowedMethods` apenas GET/HEAD
- [ ] NENHUMA regra de lifecycle criada (paridade com cache indefinido do legado — comentário no yml explicando)
- [ ] `npx serverless print --stage prod` resolve bucket `ad-serverless-media-prod` em sa-east-1
- [ ] Comentários do serverless.yml em português; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: #M0-02

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2.2 (rewrite MediaFile), §4 (VideoCacheService: `MD5(url).mp4`, whitelist, bypasses), §9 (caches)
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §15 (`GET /media/{filename}`)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 (config `video.cache.*`)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (Mídia), §3 (`media-handler`), ADR-007 (domínio no cutover)
- Java: `ad-server/src/main/java/.../VideoCacheService.java`, `MediaController.java`
- Issues relacionadas: M5-07 (lógica do cache + media-handler), M2-06 (WAF nesta distribuição)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-02] S3 bucket de mídia + CloudFront no repo
InteliFi/ad-serverless seguindo docs/issues/M2-02-s3-media-cloudfront.md e
CLAUDE.md: adicionar ao serverless.yml o bucket privado
ad-serverless-media-{stage} (Block Public Access, SSE-S3), a distribuição
CloudFront com Origin Access Control sigv4, bucket policy restrita ao ARN da
distribuição, PriceClass_All, cache policy CachingOptimized, env
MEDIA_BUCKET/MEDIA_CDN_DOMAIN e Outputs (incluindo MediaDistributionId para
o WAF da M2-06). Documentar em comentário a convenção de chave MD5(url).mp4
e a ausência proposital de lifecycle. Comentários em português. Validar
deploy dev com upload de teste e curl via CloudFront. Abrir PR ao final.
```
