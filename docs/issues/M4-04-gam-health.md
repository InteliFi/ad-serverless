---
title: "[M4-04] ad-handler: GET /GAM + health checks"
labels: ["epic:M4-adserving", "tipo:port", "prioridade:P1"]
milestone: "M4 — Ad Serving"
---
## Contexto

Dois endpoints simples servidos pela mesma Lambda `ad-handler`: o `/GAM` (HTML de Google Ad Manager buscado de um CloudFront público) e os health checks usados por balanceadores e monitoramento. Port de `GamService.java` e `ApplicationStatusService.java` ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §1 e §3).

## Especificação detalhada

### GET /GAM (paridade exata)

- **Params:** `hid` (obrigatório — identificador do arquivo HTML), `red` (opcional, ignorado no fetch).
- `hid` vazio → **404**.
- Fetch HTTP GET de `https://d26ykw0gs9fv5u.cloudfront.net/public/gam/{hid}.html` usando o client compartilhado ([M1-08]) com timeout 60s (default do legado).
- Resposta upstream vazia ou erro de fetch → **404** (o legado apenas loga e devolve 404 — nunca 5xx).
- Sucesso → **200** `Content-Type: text/html` com o corpo do upstream.
- A URL base do CloudFront deve ser configurável por env var (`GAM_BASE_URL`, default a URL acima) — facilita teste e eventual troca de bucket.

### GET|HEAD /, /health, /healthz

- Resposta **200** `application/json`: `{"version":"<versão>","status":"UP"}`.
- `version` vem da env var `API_VERSION` (injetada no deploy via serverless.yml a partir da tag/commit; fallback `"unknown"`).
- **Não toca banco nem serviços externos** — precisa responder em <10ms mesmo com MySQL fora (é o sinal de vida do canary [M9-03]).
- HEAD responde 200 sem corpo.

## Arquivos a criar/alterar

- `cmd/ad/main.go` — rotas /GAM, /, /health, /healthz no roteador interno
- `internal/adserving/gam.go` + `gam_test.go` (httptest mockando o CloudFront)
- `internal/platform/health.go` + teste

## Critérios de aceite

- [ ] /GAM: sucesso 200 text/html; hid vazio→404; upstream 500/timeout/vazio→404 (testes com httptest)
- [ ] Health: GET e HEAD em /, /health, /healthz → 200 com JSON exato; teste garante ausência de dependências externas (handler funciona com DSN inválido)
- [ ] `make lint && make test` verdes; comentários em português com `// Portado de: GamService.java / ApplicationStatusService.java`

## Dependências

Bloqueada por: [M1-08]

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §1, §3
- Java de origem: `GamService.java`, `ApplicationStatusService.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M4-04] GET /GAM + health checks seguindo docs/issues/M4-04-gam-health.md e CLAUDE.md. Paridade exata de comportamento (404 em qualquer falha do GAM, health sem dependências), testes httptest. Código comentado em português. Abrir PR ao final.
```
