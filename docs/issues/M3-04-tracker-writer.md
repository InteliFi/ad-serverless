---
title: "[M3-04] tracker-writer: consumidor SQS → MySQL + DynamoDB"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

O `tracker-writer` é a Lambda que materializa a escrita dupla de tracking do legado (`AdTrackComponent.save`: MySQL síncrono + DynamoDB `@Async` — ver [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.4 e §1.5). Ela consome a `tracking-queue` (SQS, M2-01) em batch de até 25 mensagens publicadas pelo `track-handler` (M3-03) e, para cada evento, executa **na ordem**: INSERT no MySQL `ad_trackers` → captura o `LastInsertId` → `PutItem` no DynamoDB `AdTrackers` com o `rds_id`. A ordem é INEGOCIÁVEL: a sort key `created_at_id` do DynamoDB embute o ID do MySQL (ver M3-02).

Decisão de arquitetura: ADR-003 ([ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md)) — goroutines não sobrevivem ao freeze do container, então o `@Async` do Java vira fila durável + consumidor. O **timestamp do evento é o do request original** (campo `event_time` da mensagem), nunca o momento do processamento.

## Especificação detalhada

### 1. Event source e estrutura

- `cmd/trackerwriter/main.go` — handler de `events.SQSEvent` (NÃO é HTTP; sem API Gateway).
- `serverless.yml`: function `tracker-writer`, 256MB, timeout 60s, event source SQS da `tracking-queue` com `batchSize: 25` e `functionResponseType: ReportBatchItemFailures`.
- Mensagem consumida = struct `TrackingEvent` (contrato JSON definido em M3-03, pacote `internal/tracking`):

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "campaign_id": 42,
  "hotspot_id": "HOTSPOT_ABC",
  "event_type": "VIDEO_STARTED",
  "event_time": "20240610153045123",
  "source": "adtrack"
}
```

### 2. Processamento de cada mensagem (porta `AdTrackComponent.save`)

1. **Unmarshal** do JSON; falha de parse → mensagem vai para `batchItemFailures` (após esgotar retries cai na DLQ — mensagem malformada nunca trava o batch inteiro).
2. **Parse do `event_time`** (`yyyyMMddHHmmssSSS`, 17 dígitos, parser custom do M3-03/M1-06) em `America/Sao_Paulo` — este é o timestamp do EVENTO.
3. **INSERT MySQL** via `mysql.InsertAdTracker` (M3-01): colunas `campaign_id`, `hotspot_id`, `event_type`, `creation_date` (DATETIME = event_time), `event_date` (DATE = event_time truncado ao dia em America/Sao_Paulo). Captura o **`LastInsertId`**.
   - Regra do `hotspot_id`: se `source == "trackingpixel"` → gravar **NULL** (paridade: o pixel registra hotspot null deliberadamente — 02 §1.7); caso contrário gravar o valor literal recebido, **inclusive string vazia** (paridade com o `hid` default `""` do `/vasttrack`).
4. **PutItem DynamoDB** via `dynamo.PutAdTracker` (M3-02): PK `campaign_id` (string), SK `created_at_id` = `<event_time em UTC ISO8601 com millis>#<rds_id>`, atributos `hotspot_id` (omitido quando NULL), `event_type`, `event_date` (`yyyy-MM-dd` America/Sao_Paulo), `rds_id` (N) = `LastInsertId`.

### 3. Partial batch failure (ReportBatchItemFailures) — semântica de erro

| Falha | Comportamento | Justificativa de paridade |
|---|---|---|
| JSON inválido / `event_time` não parseável | adiciona `messageId` em `batchItemFailures` | redelivery → DLQ após 5 tentativas |
| INSERT MySQL falha | adiciona em `batchItemFailures` (mensagem será reentregue) | no legado o save MySQL era transacional e propagava erro (500) |
| PutItem DynamoDB falha (após os 3 retries do SDK, M3-02) | **apenas log ERROR — NÃO marca falha** | paridade: no legado a réplica DynamoDB era fire-and-forget (`@Async`, erro apenas logado) — reprocessar duplicaria a linha MySQL |

- O handler retorna `events.SQSEventResponse{BatchItemFailures: [...]}` somente com as mensagens que falharam — as demais são removidas da fila (nunca falhar o batch inteiro por 1 mensagem).
- Processamento sequencial mensagem a mensagem (sem multi-row INSERT): o `LastInsertId` individual é obrigatório para a SK do DynamoDB. Otimização de batch é `melhoria` futura, fora desta issue.

### 4. Idempotência básica

- SQS Standard é at-least-once: a mesma mensagem pode chegar 2×. Manter um **cache em memória por container** (map `event_id` → struct{}, com limite ~10k entradas, semântica do `internal/cache` M1-03) e pular mensagens já processadas no mesmo container (log INFO `evento duplicado ignorado`).
- Documentar em comentário a LIMITAÇÃO: o cache não sobrevive entre containers — duplicatas residuais são aceitas (o legado também não tinha deduplicação; a reconciliação é a issue M9-04).

### 5. DLQ e observabilidade

- A `tracking-queue` (M2-01) tem redrive policy `maxReceiveCount: 5` → após 5 tentativas a mensagem vai para a DLQ (alarme na M7-04).
- Logs slog JSON com campos `service=tracker-writer`, `event_id`, `cid`, `hid`, `event_type`, `rds_id` (quando obtido).
- Sem `time.Now()` na lógica: o timestamp do evento vem da mensagem; relógio injetável onde necessário.

### 6. Testes

- Unitários com repositórios MySQL/Dynamo mockados (interfaces): sucesso completo (INSERT→ID→PutItem na ordem certa), falha MySQL → `batchItemFailures` contém o `messageId`, falha Dynamo → SEM falha no batch + log ERROR, JSON inválido → falha individual sem afetar as demais, `source=trackingpixel` → hotspot NULL, `hid=""` de vasttrack → string vazia gravada, duplicata de `event_id` ignorada.
- Teste de conversão do `event_time`: `"20240610153045123"` → `creation_date` 2024-06-10 15:30:45.123 (São Paulo), `event_date` `2024-06-10`, SK Dynamo `2024-06-10T18:30:45.123Z#<id>` (UTC = São Paulo + 3h).

## Arquivos a criar/alterar

- `cmd/trackerwriter/main.go`
- `internal/tracking/writer.go` (lógica de processamento do batch; `// Portado de: AdTrackComponent.java (save)`)
- `internal/tracking/writer_test.go`
- `serverless.yml` (function `tracker-writer`: event source SQS batch 25, `ReportBatchItemFailures`, IAM `sqs:ReceiveMessage/DeleteMessage/GetQueueAttributes` + `dynamodb:PutItem` na tabela AdTrackers)
- `docs/MATRIZ-PARIDADE.md`

## Critérios de aceite

- [ ] Consome a tracking-queue em batch de até 25 com `ReportBatchItemFailures` configurado no serverless.yml
- [ ] Para cada mensagem: INSERT MySQL `ad_trackers` → `LastInsertId` → PutItem DynamoDB com `rds_id` (ordem garantida, testada)
- [ ] `creation_date`/`event_date` derivados do `event_time` ORIGINAL da mensagem (nunca do momento do processamento)
- [ ] Falha MySQL → mensagem em `batchItemFailures`; falha DynamoDB → apenas log ERROR (paridade fire-and-forget; teste cobre os dois casos)
- [ ] `source=trackingpixel` → `hotspot_id` NULL no MySQL e omitido no DynamoDB; demais sources gravam o valor literal (incl. `""`)
- [ ] Idempotência best-effort por `event_id` em cache de container, com limitação documentada
- [ ] DLQ após 5 tentativas (redrive policy da M2-01 referenciada); nenhuma mensagem perdida silenciosamente
- [ ] `// Portado de: AdTrackComponent.java / AdTrackerDynamoComponent.java`; godoc em português; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M3-01 (repository/mysql), M3-02 (repository/dynamo). Consome o contrato `TrackingEvent` da M3-03 e a fila da M2-01.

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.4 (tracking duplo), §1.5 (réplica DynamoDB)
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2.4 (`ad_trackers`), §4.1 (`AdTrackers`), §5 (conversões de data)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-003, §3 (tracker-writer)
- Java: `ad-server/.../business/component/impl/AdTrackComponentImpl.java`, `AdTrackerDynamoComponent.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-04] tracker-writer no repo InteliFi/ad-serverless
seguindo docs/issues/M3-04-tracker-writer.md e CLAUDE.md: Lambda consumidora da
tracking-queue (SQS batch 25, ReportBatchItemFailures) que, para cada TrackingEvent,
faz INSERT em ad_trackers via M3-01, captura LastInsertId e faz PutItem no DynamoDB
AdTrackers via M3-02 com rds_id na SK. Timestamp = event_time original da mensagem.
Falha MySQL → batchItemFailures; falha Dynamo → só log (paridade fire-and-forget).
source=trackingpixel → hotspot NULL. Idempotência best-effort por event_id. DLQ após
5 tentativas. Código comentado em português. Testes verdes. Abrir PR ao final.
```
