---
title: "[M5-07] Video cache S3 + media-handler"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

O legado cacheia vídeos MP4 de parceiros em disco local (`/tmp/adserver_video_cache`) e os serve via `/media/{filename}` para reduzir dependência de CDNs externas no VAST ([docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.10 e [03-pipeline-vast.md](../legado/03-pipeline-vast.md) §4). Em Lambda não há disco compartilhado entre containers — o cache migra para **S3 + CloudFront** ([M2-02]), mantendo as MESMAS regras de negócio.

## Especificação detalhada

### internal/vast/videocache (usado pelo rewrite de MediaFile [M5-02])

`func URLCacheada(ctx, urlOriginal string, headersCliente, ipCliente string) string` — retorna o path `/media/{filename}` ou `""` (mantém original). Regras de paridade:

1. **Whitelist de domínios** (config `VIDEO_CACHE_WHITELIST`, default `gcdn.2mdn.net,googlevideo.com`): host fora (match por sufixo) → `""`.
2. **Bypass de URLs assinadas Google** (antes da whitelist):
   - host sufixo `gcdn.2mdn.net` E path contém `/videoplayback/` → `""`
   - host sufixo `googlevideo.com` E path contém `/manifest/` → `""`
3. **Somente MP4**: aplicar apenas a `<MediaFile type="video/mp4">` (commit `52936fa` — "skip video cache for non-mp4").
4. **Chave do objeto**: `MD5(urlOriginal)` em hex + `.mp4` (paridade com o nome de arquivo do legado).
5. **Hit**: `HeadObject` no bucket → existe → retorna `/media/{filename}`. Cache em memória de container `url→filename` evita HEADs repetidos (TTL [M1-03]).
6. **Miss**: download da URL com headers do cliente copiados + `Referer: https://ads.inteli.fi/` + `X-Forwarded-For: {ipCliente}`; substituir o placeholder `ip/0.0.0.0` por `ip/{ipCliente}` na URL antes do download; `PutObject` com `Content-Type: video/mp4`; retorna `/media/{filename}`.
7. **Qualquer falha** (download, S3, URL inválida) → log WARN + `""` — o VAST mantém a URL original. **Nunca** propagar erro.
8. ⚠️ Atenção ao timeout: download de vídeo pode ser lento; limitar a ~20s (dentro dos 29s do vast-handler) e, em estouro, devolver `""` — vídeo entra no cache na próxima requisição.

### media-handler (cmd/media/main.go) — GET /media/{filename}

1. Validar `filename`: apenas `[a-f0-9]{32}\.mp4` (protege path traversal — mais estrito que o `normalize()` do legado, documentar).
2. `HeadObject` no bucket: não existe → **404** (paridade: arquivo não encontrado/não legível → 404).
3. Existe → **302** `Location: https://{cloudfront-media-domain}/{filename}` — decisão documentada: a Lambda não faz stream do vídeo porque o payload de resposta é limitado a 6MB; o CloudFront serve o objeto com range requests e cache de borda (players precisam de range — o redirect resolve).
4. Erro inesperado → **500**.

## Arquivos a criar/alterar

- `internal/vast/videocache/videocache.go` + testes (S3 mockado)
- `cmd/media/main.go` + testes
- Integração no rewrite de MediaFile ([M5-02]) — plugar `URLCacheada`

## Critérios de aceite

- [ ] Testes das regras 1–7 (whitelist, bypasses assinados, MD5, hit/miss, substituição de IP, falha→"")
- [ ] media-handler: 302 para objeto existente, 404 para ausente, 400/404 para filename fora do padrão
- [ ] Teste de integração: VAST com MediaFile mp4 da whitelist → URL reescrita para /media/{md5}.mp4 (golden)
- [ ] VAST do Google DoubleClick NÃO passa pelo cache (bypass de [M5-03] tem precedência — teste)
- [ ] `make lint && make test` verdes; comentários em português com `// Portado de: VideoCacheService.java / MediaController.java`

## Dependências

Bloqueada por: [M2-02], [M5-02]

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.10
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §4
- Java de origem: `VideoCacheService.java`, `MediaController.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-07] video cache S3 + media-handler seguindo docs/issues/M5-07-video-cache-s3-media.md e CLAUDE.md. Paridade das regras de whitelist/bypass/MD5/falha-silenciosa com backend S3, media-handler com 302 para CloudFront, testes completos. Código comentado em português. Abrir PR ao final.
```
