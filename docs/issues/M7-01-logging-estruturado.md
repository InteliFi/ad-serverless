---
title: "[M7-01] Logging estruturado padrão (slog JSON) em todos os serviços"
labels: ["epic:M7-observabilidade", "tipo:infra", "prioridade:P1"]
milestone: "M7 — Observabilidade"
---
## Contexto

O legado loga em nível **ERROR global, apenas stdout** ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 "Logging") — na prática quase nada é registrado, e quando algo dá errado falta contexto (qual hotspot? qual campanha? qual request?). Na arquitetura alvo, todo log vai para CloudWatch Logs como **JSON estruturado via `log/slog`** (decisão já registrada em [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 e no CLAUDE.md: "Logs: `log/slog` JSON em stdout; campos padrão `service`, `route`, `cid`, `hid` quando aplicável").

A issue M1-08 entregou o esqueleto do logger em `internal/platform`. Esta issue **padroniza e completa** o uso em TODOS os 9 serviços (`ad`, `vast`, `track`, `redirect`, `postback`, `proxy`, `media`, `report`, `trackerwriter`), define o contrato de campos, os níveis por ambiente e as regras de redação de segredos. É pré-requisito das métricas (M7-02), dos dashboards (M7-03) e da comparação de shadow traffic (M9-02), que dependem de logs consultáveis via CloudWatch Logs Insights.

## Especificação detalhada

### 1. Contrato de campos padrão (todo log de request)

Implementar em `internal/platform/logging/logging.go` um construtor `NewRequestLogger(ctx, req)` que devolve um `*slog.Logger` já enriquecido com:

| Campo | Origem | Obrigatório |
|---|---|---|
| `service` | env var `SERVICE_NAME` (definida por função no serverless.yml, ex.: `vast-handler`) | sim |
| `route` | `req.RouteKey` do payload v2 (ex.: `GET /vast`) | sim |
| `request_id` | `req.RequestContext.RequestID` (**request id do API Gateway**, correlaciona com access logs do API GW) | sim |
| `lambda_request_id` | `lambdacontext.FromContext(ctx)` | sim |
| `cid` | query param `cid` quando presente | quando houver |
| `hid` | query param `hid` quando presente (normalizado UPPER, primeira ocorrência do pipe — mesma regra do /ad) | quando houver |
| `stage` | env var `STAGE` (`dev`/`prod`) | sim |

- Handlers que processam SQS (`tracker-writer`) usam variante `NewSQSLogger(ctx, msg)` com `message_id`, `event_id` (do `TrackingEvent`), `cid`, `hid`.
- O logger enriquecido viaja no `context.Context` (helper `logging.FromContext(ctx)`) para que repositórios e clients HTTP loguem com os mesmos campos sem repassar parâmetros.

### 2. Níveis por env var

- Env var `LOG_LEVEL` (`DEBUG|INFO|WARN|ERROR`), default `INFO` em prod e `DEBUG` em dev, configurada no `serverless.yml` por stage (`${param:logLevel}`).
- Implementar com `slog.LevelVar` para permitir, futuramente, ajuste dinâmico sem redeploy (apenas a estrutura; o ajuste dinâmico NÃO faz parte desta issue).

### 3. Política de conteúdo (o que logar em cada nível)

Substituir a filosofia "ERROR-only" do legado por **INFO estruturado de baixo volume + DEBUG ativável**:

- **INFO (1 linha por request, máximo):** resumo do request — campos padrão + `status`, `duration_ms`, e por serviço: `flow` (A/B/C no vast), `event_type` (tracking), `upstream_host` + `upstream_status` (proxies/vast/postback), `template` (ad-handler), `cache` (`hit|miss`). NUNCA logar o corpo da resposta.
- **DEBUG:** detalhes de pipeline — URL upstream (já redigida, ver §4), decisões de bypass por parceiro, chaves de cache, SQL executado (sem valores de credenciais), tamanho de payloads.
- **WARN:** comportamentos anômalos não fatais (paridade com o legado: `Tracking failure: {...}` do /adtrack, resposta sem tag `<VAST>`, falha de postback upstream que apenas loga).
- **ERROR:** falha que resultou em 5xx ou perda potencial de evento; sempre com `error` (mensagem encadeada via `%w`) e campos padrão.

### 4. Redação de segredos e URLs sensíveis — REGRA INEGOCIÁVEL

- **NUNCA** logar: DSN MySQL, valores de SSM, `intv.ad.signaturekey`, headers `Authorization`/`Cookie`, ou URLs com credenciais embutidas (`https://user:pass@host/...`).
- Implementar `logging.RedactURL(url string) string` que: remove `userinfo` da URL; mascara query params cujo nome case com `(?i)(key|token|secret|password|signature|auth)` substituindo o valor por `***`. Usar SEMPRE que uma URL upstream for logada (vast fetch, proxy-tracker, proxy-audit, postback, video cache, pixel download).
- Teste unitário cobrindo cada caso de redação; lint check documentado no CODE_DOCS_POLICY (revisão de PR verifica uso de `RedactURL`).

### 5. Aplicação nos 9 serviços

Revisar TODOS os `cmd/*/main.go` e pacotes `internal/*` para: (a) usar o logger do contexto; (b) eliminar `fmt.Println`/`log.Printf` remanescentes; (c) garantir a linha INFO de resumo por request via middleware (`internal/middleware/requestlog.go`, encadeado após o recover — panics logam ERROR com stack).

### 6. Retenção e custo

- Configurar `logRetentionInDays: 30` (dev: 14) no serverless.yml para os log groups das 9 funções — sem isso a retenção é infinita e o custo cresce sem limite.

## Arquivos a criar/alterar

- `internal/platform/logging/logging.go`, `redact.go`, `logging_test.go`, `redact_test.go`
- `internal/middleware/requestlog.go` + teste
- `cmd/*/main.go` (9 serviços — injeção do logger e `SERVICE_NAME`)
- `serverless.yml` (env vars `LOG_LEVEL`, `SERVICE_NAME`, `logRetentionInDays`)
- `docs/runbooks/` ganhará exemplos de queries Logs Insights em M8-04 (não nesta issue)

## Critérios de aceite

- [ ] Todo request em qualquer das 9 funções gera exatamente 1 linha INFO de resumo com os campos padrão (`service`, `route`, `request_id`, `stage`, `status`, `duration_ms`)
- [ ] `cid`/`hid` presentes nos logs de /ad, /vast, /adtrack, /vasttrack, /trackingpixel, /redirect, /adtrack/postback quando informados no request
- [ ] `LOG_LEVEL=DEBUG` ativa logs de pipeline sem redeploy de código (apenas env var)
- [ ] `RedactURL` aplicada em TODA URL upstream logada; testes cobrindo userinfo e params sensíveis
- [ ] Nenhum `fmt.Print*`/`log.Print*` fora de `internal/platform/logging` (verificado por `grep` no CI ou regra golangci-lint `forbidigo`)
- [ ] Paridade: `Tracking failure: {...}` continua logado (WARN) no /adtrack com `failureInfo`
- [ ] Log groups com retenção configurada (30d prod / 14d dev)
- [ ] Query Logs Insights de exemplo documentada no PR (filtrar por `cid` em todos os serviços)
- [ ] Código comentado em português; `make lint && make test` verdes

## Dependências

Bloqueada por: M1-08 (internal/httpx + platform: config SSM, slog base)

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 (logging legado ERROR-only, stdout)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (observabilidade) e §4 (internal/platform)
- CLAUDE.md — convenção de logs slog JSON
- [Documentação slog](https://pkg.go.dev/log/slog) · [Lambda + CloudWatch Logs](https://docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs.html)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M7-01] Logging estruturado padrão seguindo
docs/issues/M7-01-logging-estruturado.md e CLAUDE.md. Criar
internal/platform/logging (NewRequestLogger com service/route/request_id do
API GW/cid/hid/stage, FromContext, RedactURL com testes), middleware
requestlog com 1 linha INFO por request, LOG_LEVEL por env var nos 9
serviços no serverless.yml, retenção de logs configurada, eliminar
fmt.Print/log.Print remanescentes. NUNCA logar URLs com credenciais.
Código comentado em português, testes verdes, abrir PR.
```
