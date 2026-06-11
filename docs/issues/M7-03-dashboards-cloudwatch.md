---
title: "[M7-03] Dashboards CloudWatch por serviço"
labels: ["epic:M7-observabilidade", "tipo:infra", "prioridade:P1"]
milestone: "M7 — Observabilidade"
---
## Contexto

Com logs (M7-01) e métricas (M7-02) no ar, precisamos de **visualização operacional permanente**: durante o canary do M9 a decisão de avançar 5%→25%→50%→100% por rota será tomada olhando esses dashboards, comparando Lambda × EC2. O entregável de aceite do epic M7 no [PLANO-MIGRACAO](../PLANO-MIGRACAO.md) é literalmente "dashboard por serviço; alarmes DLQ/erro/latência ativos".

Esta issue cria **1 dashboard geral ("AdServerless-Overview-{stage}") + 1 dashboard por serviço** (9), todos **como código** (recursos CloudFormation no `serverless.yml` ou arquivos JSON versionados aplicados no deploy) — nunca criados à mão no console, para sobreviverem a recriação de stack e serem revisáveis em PR.

## Especificação detalhada

### 1. Dashboard geral `AdServerless-Overview-{stage}`

Linha de visão única da saúde do sistema (o que se olha primeiro num incidente):

| Widget | Métrica/fonte | Observação |
|---|---|---|
| Invocações por função (stacked) | `AWS/Lambda Invocations` por FunctionName | as 9 funções |
| Erros por função | `AWS/Lambda Errors` + `% = Errors/Invocations` (math) | |
| Latência p50/p99 agregada | `AWS/Lambda Duration` (p50, p99) | |
| Throttles | `AWS/Lambda Throttles` | qualquer valor > 0 é anômalo |
| **DLQ depth** | `AWS/SQS ApproximateNumberOfMessagesVisible` da `tracking-dlq` | deve ser 0 |
| **Idade da mensagem mais antiga no SQS** | `AWS/SQS ApproximateAgeOfOldestMessage` da `tracking-queue` | atraso de escrita de eventos |
| **Conexões RDS Proxy** | `AWS/RDS DatabaseConnections` / `ClientConnections` (dimensão ProxyName) | proteger o MySQL compartilhado (ADR-002) |
| Erros 4xx/5xx do API Gateway | `AWS/ApiGateway 4xx/5xx` (HTTP API: `4XXError`/`5XXError` por ApiId) | visão de borda |
| Eventos de tracking recebidos × escritos | EMF `TrackingEventReceived` vs `TrackingEventWritten` | divergência = perda na fila |
| Erros upstream por host parceiro | EMF `UpstreamError` por UpstreamHost (top N) | parceiro fora do ar |

### 2. Dashboards por serviço (9)

Padrão `AdServerless-{service}-{stage}` com: invocações, duração p50/p99, erros/throttles, memória máxima usada (Logs Insights widget sobre o relatório REPORT), concurrent executions, e os widgets ESPECÍFICOS:

- **track-handler:** `TrackingEventReceived` por EventType; latência por rota (adtrack/vasttrack/trackingpixel); falhas de `SendMessage` SQS.
- **tracker-writer:** `SQSMessagesProcessed`/`Failed`; `ApproximateAgeOfOldestMessage`; DLQ depth; `TrackingEventWritten` por EventType; duração de batch.
- **vast-handler:** `VastFlow` (A/B/C); `UpstreamLatency`/`UpstreamError` por host; `CacheHit/Miss` (hotspots/override); contagem 404/400/502/504.
- **proxy-handler:** `UpstreamLatency`/`UpstreamError` por host; tamanho de resposta (propriedade EMF → Logs Insights); contagem de VAST errors detectados (`/errors?`, `&error=`).
- **postback-handler:** `PostbackUpstreamSent/Failed` por Source (modatta/prezao_claro); 401/404/422.
- **ad-handler:** latência /ad vs /GAM; `CacheHit/Miss` hotspots; 404 rate (hid inexistente).
- **media-handler:** hits S3 vs cache-miss (download); bytes servidos.
- **report-handler:** duração (timeout 120s — destacar p100); invocações (baixo volume).
- **redirect-handler:** contagem 400 (URL inválida) vs 200.

### 3. Implementação como código

- Preferência: recurso `AWS::CloudWatch::Dashboard` em `resources:` do serverless.yml, com o JSON do corpo gerado a partir de templates em `infra/dashboards/*.json` (script `make dashboards` valida JSON e injeta stage/region/nomes de função).
- Alternativa aceitável (documentar a escolha): plugin `serverless-plugin-cloudwatch-dashboard` — avaliar manutenção/compatibilidade antes.
- Nomes de funções/filas/proxy parametrizados — NUNCA hardcoded por stage.
- Period default dos widgets: 1 min (5 min nos de baixo volume). Janela default: 3h.

### 4. Queries Logs Insights salvas

Criar (via `AWS::Logs::QueryDefinition`) queries salvas reutilizáveis: "requests por cid", "erros 5xx com request_id", "latência por rota acima de 500ms", "mensagens DLQ com corpo" — usadas pelos runbooks (M8-04).

## Arquivos a criar/alterar

- `infra/dashboards/overview.json` + 9 × `infra/dashboards/{service}.json`
- `serverless.yml` (`resources:` com `AWS::CloudWatch::Dashboard` e `AWS::Logs::QueryDefinition`)
- `Makefile` (target `dashboards` — validação dos JSON)
- `docs/runbooks/` referenciará os dashboards (M8-04, não nesta issue)

## Critérios de aceite

- [ ] `serverless deploy --stage dev` cria/atualiza os 10 dashboards sem passos manuais
- [ ] Dashboard geral contém TODOS os widgets da tabela §1, incluindo DLQ depth, idade da mensagem SQS e conexões RDS Proxy
- [ ] Cada um dos 9 dashboards de serviço tem os widgets específicos listados em §2
- [ ] Nenhum nome de recurso hardcoded — recriar a stack em outra conta/região não quebra os dashboards (usar Ref/GetAtt/variáveis do framework)
- [ ] Widgets EMF apontam para o namespace `AdServerless/{stage}` com as dimensões corretas do M7-02
- [ ] Queries Logs Insights salvas e funcionais (evidência no PR)
- [ ] Screenshot de cada dashboard em dev com tráfego sintético anexado ao PR
- [ ] JSONs lintados (`make dashboards` no CI)

## Dependências

Bloqueada por: M7-02 (métricas EMF + X-Ray — os dashboards consomem essas métricas)

## Referências

- [docs/issues/M7-02-metricas-emf-xray.md](M7-02-metricas-emf-xray.md) (catálogo de métricas e dimensões)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (9 Lambdas), ADR-002 (RDS Proxy)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) (entregável de aceite do M7; canary M9 usa estes dashboards)
- [Estrutura de dashboard body](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/CloudWatch-Dashboard-Body-Structure.html) · [Métricas SQS](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-available-cloudwatch-metrics.html) · [Métricas RDS Proxy](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/rds-proxy.monitoring.html)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M7-03] Dashboards CloudWatch seguindo
docs/issues/M7-03-dashboards-cloudwatch.md e CLAUDE.md. Criar 1 dashboard
geral (invocações, p50/p99, erros, throttles, DLQ depth, idade SQS,
conexões RDS Proxy, eventos recebidos×escritos, erros upstream por host) +
9 dashboards por serviço com widgets específicos, tudo como código em
infra/dashboards/*.json + resources do serverless.yml, sem nomes
hardcoded, com queries Logs Insights salvas. Validar em dev com tráfego
sintético e anexar screenshots ao PR.
```
