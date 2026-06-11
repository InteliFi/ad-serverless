---
title: "[M7-02] Métricas EMF + X-Ray tracing"
labels: ["epic:M7-observabilidade", "tipo:infra", "prioridade:P1"]
milestone: "M7 — Observabilidade"
---
## Contexto

O legado não tem métricas de negócio — só os contadores implícitos da tabela `ad_trackers`. Para operar 9 Lambdas com segurança no cutover (M9) precisamos de métricas custom (eventos por tipo, latência por rota, erros por parceiro upstream) e tracing distribuído (request → API GW → Lambda → MySQL/SQS/upstream). A decisão de stack ([docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1) é **CloudWatch EMF (Embedded Metric Format)** — métricas embutidas nos logs JSON, sem chamadas de API extra, sem custo de PutMetricData, sem latência adicional no hot path — e **AWS X-Ray** ativo nas 9 funções.

EMF foi escolhido porque: (a) o `track-handler` responde em ~5ms e não pode pagar uma chamada síncrona ao CloudWatch; (b) a emissão é uma linha de log com `_aws.CloudWatchMetrics`, agregada pelo CloudWatch automaticamente; (c) integra com o logging estruturado do M7-01.

## Especificação detalhada

### 1. Pacote `internal/platform/metrics` (EMF)

Implementar emissor EMF próprio (struct serializada para stdout junto ao slog) ou via `github.com/prozz/aws-embedded-metrics-golang` — decidir no PR e documentar. Namespace: `AdServerless/{stage}`. Dimensões padrão: `Service` (sempre), segunda dimensão conforme a métrica (máx. 2 dimensões para conter cardinalidade/custo).

**Métricas obrigatórias:**

| Métrica | Tipo | Dimensões | Emitida por |
|---|---|---|---|
| `TrackingEventReceived` | Count | Service, EventType | track-handler (adtrack/vasttrack/trackingpixel) |
| `TrackingEventWritten` | Count | Service, EventType | tracker-writer (após INSERT MySQL + PutItem OK) |
| `SQSMessagesProcessed` / `SQSMessagesFailed` | Count | Service | tracker-writer (partial batch failure conta em Failed) |
| `RequestLatency` | Milliseconds (p50/p99 via histograma EMF) | Service, Route | middleware em todos os handlers HTTP |
| `RequestCount` / `RequestError4xx` / `RequestError5xx` | Count | Service, Route | middleware |
| `UpstreamLatency` | Milliseconds | Service, UpstreamHost | vast fetch, proxy-tracker, proxy-audit, postback, GAM, pixel download |
| `UpstreamError` | Count | Service, UpstreamHost | idem (status >= 400, timeout, erro de rede) |
| `VastFlow` | Count | Service, Flow (A/B/C) | vast-handler |
| `CacheHit` / `CacheMiss` | Count | Service, CacheName (hotspots/override/template) | internal/cache |
| `PostbackUpstreamSent` / `PostbackUpstreamFailed` | Count | Service, Source (modatta/prezao_claro) | postback-handler |

- **`UpstreamHost` é o host do parceiro** (ex.: `videoapi.smartadserver.com`, `cdn.00px.net`, `sdk.adftech.com.br`, `servedby.metrike.com.br`, `pubads.g.doubleclick.net`, `pb.modatta.org`, `api.prezaofree.com.br`) — extraído com `url.Parse(...).Host`, nunca a URL completa (cardinalidade e segredos — ver RedactURL do M7-01).
- Cardinalidade: NÃO usar `cid`/`hid` como dimensão de métrica (alta cardinalidade = custo explosivo). `cid`/`hid` ficam como **propriedades** EMF (consultáveis no Logs Insights, sem virar métrica).

### 2. X-Ray nas 9 funções

- `tracing: { lambda: true, apiGateway: true }` no `provider` do serverless.yml (Active tracing) + permissões `xray:PutTraceSegments`, `xray:PutTelemetryRecords` nas roles (M2-04).
- Instrumentar com `aws-xray-sdk-go`:
  - **Anotações** (indexadas, filtráveis): `cid`, `hid`, `event_type`, `flow`, `upstream_host` — anotações têm baixa cardinalidade de CHAVE, valores podem variar.
  - **Metadados** (não indexados): URL redigida, decisões de bypass.
- Subsegments para: query MySQL (RDS Proxy), `SendMessage` SQS, `PutItem` DynamoDB, S3, e cada fetch upstream.
- Sampling: default (1 req/s + 5%) é suficiente; NÃO usar 100% em prod (custo). Documentar como subir o sampling temporariamente durante o canary (M9-03).

### 3. Propagação de trace nos `http.Client`

- Envolver o transport compartilhado de `internal/httpx` com `xray.Client(...)` / `xray.RoundTripper`, de modo que TODA chamada upstream (vast fetch, proxies, postbacks, GAM, pixel, video cache) gere subsegment com host, status e latência, e propague o header `X-Amzn-Trace-Id` downstream.
- Atenção: o contexto X-Ray precisa fluir pelos `context.Context` — revisar assinaturas que não propagam `ctx` (não pode haver `context.Background()` no caminho do request).
- O cliente do `tracker-writer` (consumidor SQS) inicia segmento a partir do trace header da mensagem SQS quando presente (`AWSTraceHeader`).

### 4. Validação

- Smoke em dev: gerar tráfego sintético (curl nas rotas principais) e verificar no console: métricas no namespace `AdServerless/dev`, service map do X-Ray mostrando API GW → Lambda → MySQL/SQS/upstream.

## Arquivos a criar/alterar

- `internal/platform/metrics/emf.go`, `emf_test.go` (serialização EMF validada contra o spec)
- `internal/platform/xray/xray.go` (helpers de segmento/anotação)
- `internal/middleware/metrics.go` (latência/contagem por rota) + teste
- `internal/httpx/client.go` (RoundTripper X-Ray)
- `cmd/*/main.go` (9 funções — init de métricas/tracing)
- `internal/tracking/`, `internal/vast/`, `internal/proxy/`, `internal/cache/`, `cmd/trackerwriter/` (pontos de emissão)
- `serverless.yml` (tracing ativo, permissões X-Ray, env `METRICS_NAMESPACE`)

## Critérios de aceite

- [ ] Linha EMF validada contra a especificação (teste de serialização: `_aws.CloudWatchMetrics`, namespace, dimensões, unidades)
- [ ] Todas as métricas da tabela emitidas e visíveis no CloudWatch em dev após smoke test (evidência: screenshot/`aws cloudwatch list-metrics` no PR)
- [ ] `cid`/`hid` como propriedades EMF e anotações X-Ray — NUNCA como dimensão de métrica
- [ ] Service map do X-Ray em dev mostra os subsegments MySQL/SQS/DynamoDB/upstream nas rotas com dependências
- [ ] `X-Amzn-Trace-Id` propagado em toda chamada feita pelo `http.Client` compartilhado (teste com servidor httptest inspecionando o header)
- [ ] Overhead medido no hot path (track-handler) < 1ms por request (benchmark no PR)
- [ ] Nenhuma URL completa de upstream como valor de dimensão/anotação sem `RedactURL`
- [ ] Código comentado em português; `make lint && make test` verdes

## Dependências

Bloqueada por: M7-01 (logging estruturado — EMF compartilha o pipeline de stdout)

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1, §3 (tabela das 9 Lambdas)
- [docs/issues/M7-01-logging-estruturado.md](M7-01-logging-estruturado.md) (RedactURL, campos padrão)
- [Especificação EMF](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Specification.html) · [aws-xray-sdk-go](https://github.com/aws/aws-xray-sdk-go)
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) (hosts upstream por endpoint)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M7-02] Métricas EMF + X-Ray seguindo
docs/issues/M7-02-metricas-emf-xray.md e CLAUDE.md. Criar
internal/platform/metrics (EMF, namespace AdServerless/{stage}, métricas da
tabela da issue com dimensões Service/Route/EventType/UpstreamHost),
middleware de latência, X-Ray ativo nas 9 funções com anotações cid/hid e
subsegments MySQL/SQS/DynamoDB/upstream, RoundTripper X-Ray no http.Client
compartilhado propagando X-Amzn-Trace-Id. cid/hid nunca como dimensão.
Smoke em dev com evidências no PR. Código em português, testes verdes.
```
