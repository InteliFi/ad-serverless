---
title: "[M2-06] WAF + rate limiting no CloudFront"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P1"]
milestone: "M2 — Infra AWS"
---
## Contexto

O legado tem um filtro de validação anti-injection na aplicação ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §16, portado em [M1-07]) mas nenhuma proteção de borda. Na nova arquitetura, o AWS WAF na distribuição CloudFront ([M2-02]) adiciona uma camada de defesa **antes** das Lambdas — reduzindo custo de invocações maliciosas e bloqueando ataques volumétricos.

⚠️ **Cuidado central:** players de vídeo e SDKs de parceiros enviam URLs com base64 bruto, caracteres estranhos e query strings longas (`/proxy-tracker?u=`, `/vast?vcurl=`). Regras de WAF agressivas derrubariam tráfego legítimo — por isso TUDO começa em modo **COUNT**.

## Especificação detalhada

1. **Web ACL** (WAFv2, escopo `CLOUDFRONT`, região us-east-1 obrigatória para CloudFront), associada à distribuição de [M2-02]:
   - `AWSManagedRulesCommonRuleSet` — modo COUNT inicial; documentar exclusão provável de `SizeRestrictions_QUERYSTRING` e `GenericLFI_QUERYARGUMENTS` (base64 de proxy dispara falso positivo).
   - `AWSManagedRulesSQLiRuleSet` — modo COUNT inicial.
   - Regra custom `rate-limit-ip`: rate-based, 2000 requests/5min por IP, ação BLOCK desde o início (volumetria é segura de bloquear).
   - **Scope-down statement** nas managed rules excluindo os paths `/proxy-tracker`, `/proxy-audit` e `/safeframe/*` da inspeção de query string (são base64 por design; a validação fica no middleware Go).
2. **Logging**: WAF logs para CloudWatch Logs (log group dedicado), com redação de nada (não trafegam segredos).
3. **Plano de promoção COUNT→BLOCK** (documentar em `docs/infra/WAF.md`):
   - 2 semanas coletando matches em COUNT com tráfego real (shadow/canary);
   - análise dos matches: cada regra com 0 falsos positivos → promover a BLOCK;
   - regras com falsos positivos → adicionar exclusão específica e repetir 1 semana.
4. Infra declarada no `serverless.yml` (resources/CloudFormation) — sem cliques no console.

## Arquivos a criar/alterar

- `serverless.yml` — resources: `AWS::WAFv2::WebACL`, `WebACLAssociation` (via CloudFront DistributionConfig), `AWS::Logs::LogGroup`
- `docs/infra/WAF.md` — regras, exclusões e plano COUNT→BLOCK

## Critérios de aceite

- [ ] Web ACL associada à distribuição em dev e prod, tudo via IaC
- [ ] Managed rules em COUNT; rate limit em BLOCK
- [ ] Requisição com base64 longo em `/proxy-tracker?u=...` passa sem match de bloqueio
- [ ] Logs do WAF visíveis no CloudWatch
- [ ] `docs/infra/WAF.md` com o plano de promoção documentado

## Dependências

Bloqueada por: [M2-02]

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §16 (validação na aplicação — complementar, não substituída)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §7
- Issue [M8-05] (security review valida a promoção COUNT→BLOCK)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-06] WAF + rate limiting seguindo docs/issues/M2-06-waf-cloudfront.md e CLAUDE.md. WAFv2 no CloudFront com managed rules em COUNT, rate limit 2000/5min em BLOCK, scope-down para os paths de proxy, logging e docs/infra/WAF.md com plano de promoção. Abrir PR ao final.
```
