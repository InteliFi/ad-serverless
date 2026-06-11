---
title: "[M3-02] repository/dynamo: AdTrackers + PostbackLogs (formatos exatos)"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

O legado replica cada evento de tracking no DynamoDB `AdTrackers` e registra postbacks na tabela `PostbackLogs` — ambas **tabelas EXISTENTES em produção (sa-east-1)**, que serão REUSADAS sem nenhuma alteração (ARQUITETURA-ALVO §1). Esta issue cria o pacote `internal/repository/dynamo` com os dois writers, preservando **byte a byte** os nomes de atributos (snake_case) e os formatos de chave, pois consultas existentes dependem deles.

Origem Java: `AdTrackerDynamoComponent`/`AdTrackerDynamoRepository` (Enhanced Client, putItem) e `PostbackLogComponent`/`PostbackLogRepository` — ver [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.5 e §1.9.

## Especificação detalhada

### 1. `PutAdTracker` — tabela `AdTrackers`

Portado de: `AdTrackerDynamoComponent.java` / `AdTrackerDynamo.java`.

| Atributo | Tipo | Formato EXATO |
|---|---|---|
| `campaign_id` (**PK**) | S | campaign ID inteiro convertido para string (ex.: `"42"`) |
| `created_at_id` (**SK**) | S | `<ISO8601 com millis UTC>#<rds_id>` — ex.: `2024-06-10T15:30:45.123Z#12345` |
| `hotspot_id` | S | código do hotspot (omitir/null quando pixel — ver M3-04/M3-05) |
| `event_type` | S | valor string persistido do EventType (ex.: `"25_PER_PLAYED"`, `"REDIRECT_CAMPAIGN"`) |
| `event_date` | S | `yyyy-MM-dd` calculado em **America/Sao_Paulo** |
| `rds_id` | N | ID retornado pelo `LastInsertId` do INSERT MySQL |

- A SK garante ordenação cronológica + unicidade por campanha. O `rds_id` vem do MySQL — por isso o tracker-writer (M3-04) SEMPRE insere no MySQL ANTES do PutItem.
- Formato do timestamp da SK: ISO8601 com milissegundos e sufixo `Z` (UTC), idêntico ao `Instant.toString()` do Java com 3 casas de millis — usar `time.Time.UTC().Format("2006-01-02T15:04:05.000Z")`.
- Operação: `PutItem` (upsert, como o Enhanced Client) — sem condição de existência.

### 2. `PutPostbackLog` — tabela `PostbackLogs`

Portado de: `PostbackLogComponent.java` / `PostbackLog.java`.

| Atributo | Tipo | Regra |
|---|---|---|
| `transaction_id` (**PK**) | S | se vazio/ausente → **fallback para `click_id`** (paridade exata) |
| `logged_at` (**SK**) | S | ISO8601 do momento do log (recebido por parâmetro, não `time.Now()` interno) |
| `campaign_id` | S | string do int |
| `event` | S | evento do postback |
| `aff_sub` | S | |
| `click_id` | S | |
| `source` | S | |
| `payout` | N | parse seguro: valor inválido → **OMITE o atributo + log WARN** (segue sem o campo) |
| `currency` | S | |
| `sale_amount` | N | mesmo parse seguro do payout |

### 3. Retry — paridade com `@Retryable`

- O Java usava `@Retryable(DynamoDbException, 3 tentativas, backoff 1s ×2)` (1s, 2s, 4s).
- Em Go: configurar o **retryer do `aws-sdk-go-v2`** (`retry.NewStandard` com `MaxAttempts(3)` e backoff exponencial base 1s) no client DynamoDB do pacote — documentar em comentário a equivalência com o `@Retryable` legado.
- Erro após as tentativas: retornar o erro embrulhado (`fmt.Errorf("putitem AdTrackers: %w", err)`) — quem decide logar ou falhar o batch é o chamador (M3-04 usa partial batch failure; M3-07 apenas loga).

### 4. Configuração

- Nomes das tabelas via env vars `DYNAMO_TABLE_AD_TRACKERS` (default `AdTrackers`) e `DYNAMO_TABLE_POSTBACK_LOGS` (default `PostbackLogs`); região `sa-east-1` via config padrão do SDK.
- NENHUM `CreateTable` no código (o `DynamoDbMigrationRunner` do legado NÃO é portado — tabelas já existem; infra declarativa fica no serverless.yml apenas como referência de IAM).

### 5. Testes

- Unitários com a interface do client DynamoDB mockada (ou `smithy` middleware de captura): validar item completo gerado — nomes de atributos EXATOS, formato da SK com regex `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z#\d+$`, fallback de `transaction_id`→`click_id`, omissão de `payout`/`sale_amount` inválidos (com WARN), `event_date` calculado em America/Sao_Paulo (testar virada de dia: 2024-06-10T02:30:00Z → `2024-06-09`).

## Arquivos a criar/alterar

- `internal/repository/dynamo/client.go` (client + retryer)
- `internal/repository/dynamo/adtracker.go` (`PutAdTracker`)
- `internal/repository/dynamo/postbacklog.go` (`PutPostbackLog`)
- `internal/repository/dynamo/*_test.go`
- `go.mod` (+ `aws-sdk-go-v2` dynamodb)
- `docs/MATRIZ-PARIDADE.md`

## Critérios de aceite

- [ ] `PutAdTracker` gera item com PK `campaign_id` (S) e SK `created_at_id` = `<ISO8601 com millis>#<rds_id>` (ex. `2024-06-10T15:30:45.123Z#12345`)
- [ ] Atributos EXATOS em snake_case: `hotspot_id`, `event_type`, `event_date` (`yyyy-MM-dd` em America/Sao_Paulo), `rds_id` (Number)
- [ ] `PutPostbackLog`: PK `transaction_id` com fallback para `click_id`; SK `logged_at` ISO8601; `payout`/`sale_amount` Number com parse seguro (inválido → omite + WARN)
- [ ] Retryer aws-sdk-go-v2 com 3 tentativas e backoff exponencial (paridade `@Retryable` 1s ×2), documentado em comentário
- [ ] Nenhuma criação/alteração de tabela no código; nomes de tabela via env var
- [ ] Testes unitários cobrindo formatos de chave, fallbacks e parse seguro; `make lint && make test` verdes
- [ ] `// Portado de: AdTrackerDynamoComponent.java` / `PostbackLogComponent.java`; godoc em português; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M1-01 (internal/domain)

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §4 (DynamoDB: chaves e atributos)
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.5, §1.9
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (tabelas existentes reusadas)
- Java: `ad-server/src/main/java/br/com/intv/adserver/business/component/AdTrackerDynamoComponent.java`, `business/entity/AdTrackerDynamo.java`, `business/entity/PostbackLog.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-02] repository/dynamo no repo InteliFi/ad-serverless:
pacote internal/repository/dynamo com aws-sdk-go-v2. PutAdTracker na tabela
existente AdTrackers (PK campaign_id string, SK created_at_id =
"<ISO8601 com millis>#<rds_id>" ex. "2024-06-10T15:30:45.123Z#12345", atributos
hotspot_id, event_type, event_date yyyy-MM-dd em America/Sao_Paulo, rds_id N) e
PutPostbackLog na tabela PostbackLogs (PK transaction_id com fallback click_id,
SK logged_at ISO8601, payout/sale_amount Number com parse seguro que omite + WARN).
Retryer com 3 tentativas/backoff exponencial (paridade @Retryable 1s×2). Nomes de
atributos EXATOS em snake_case. Testes unitários dos formatos. Código em português,
// Portado de:, atualizar MATRIZ-PARIDADE, abrir PR.
```
