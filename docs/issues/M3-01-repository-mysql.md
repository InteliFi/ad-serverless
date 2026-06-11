---
title: "[M3-01] repository/mysql: conexão RDS Proxy + queries do hot path"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

Todas as Lambdas do hot path (ad, track, postback, vast, tracker-writer) precisam ler/escrever no MySQL **existente e compartilhado** através do RDS Proxy (ADR-002). Esta issue cria o pacote `internal/repository/mysql` com a conexão e TODAS as queries do hot path escritas à mão — **SEM ORM** (diretriz do CLAUDE.md). O schema é INTOCÁVEL (ADR-006): apenas `SELECT` e `INSERT`, nenhum DDL, nenhuma migration.

No Java, o acesso era via Spring Data JPA (`HotSpotRepository`, `CampaignRepository`, `CreativeRepository`, `AdTrackRepository`, `TrackingPixelRepository` — ver [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §2). Em Go usamos `database/sql` + `go-sql-driver/mysql` puro.

## Especificação detalhada

### 1. Conexão (`internal/repository/mysql/conn.go`)

- DSN lido do **SSM Parameter Store** (SecureString, path informado por env var ex.: `MYSQL_DSN_SSM_PARAM`) — carregado 1× por container e cacheado. NUNCA hardcoded, NUNCA copiado do repo Java (credenciais comprometidas, ver M0-05).
- Host do DSN = endpoint do **RDS Proxy** (issue M2-03), não o RDS direto.
- Parâmetros obrigatórios do DSN (`go-sql-driver/mysql`): `parseTime=true`, `loc=America%2FSao_Paulo` (paridade com o timezone `GMT-3` da conexão legada).
- Pool por container (ADR-002): `db.SetMaxOpenConns(2)`, `db.SetConnMaxLifetime(5 * time.Minute)`, `db.SetMaxIdleConns(2)`.
- Construtor `New(ctx, cfg)` retorna `*Repository` embrulhando `*sql.DB`; erros com `fmt.Errorf("contexto: %w", err)`.

### 2. Queries (copiar de [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §6 — escrever à mão)

```sql
-- hotspot + campanhas + creatives elegíveis (substitui o grafo JPA):
SELECT h.id, h.code, h.physical_id, ... FROM hotspots h WHERE h.code = ?;
SELECT c.* FROM campaigns c
  JOIN hotspots_campaigns hc ON hc.campaign_id = c.id
 WHERE hc.hotspot_id = ? AND c.enabled = 1;
SELECT cr.* FROM creatives cr WHERE cr.campaign_id IN (...);
-- tracking:
INSERT INTO ad_trackers (campaign_id, hotspot_id, event_type, creation_date, event_date) VALUES (?,?,?,?,?);
-- pixel:
SELECT url FROM tracking_pixels WHERE campaign_id = ?;
```

Métodos públicos do repositório (todos com `context.Context` como primeiro parâmetro):

| Método | Comportamento | Portado de |
|---|---|---|
| `FindHotspotByCode(ctx, code)` | Executa as 3 primeiras queries acima e monta `domain.HotSpot` com campanhas (`enabled=1`) e creatives associados. Code já chega em UPPER CASE (responsabilidade do handler). Não encontrado → `(nil, nil)` (vira 404 no handler). | `HotSpotRepository.findByCode` |
| `InsertAdTracker(ctx, t)` | INSERT em `ad_trackers`; **retorna `LastInsertId()`** — o ID é OBRIGATÓRIO pois compõe a SK `created_at_id` do DynamoDB (issue M3-02). Colunas: `campaign_id`, `hotspot_id` (NULL permitido — pixel/postback), `event_type`, `creation_date`, `event_date`. | `AdTrackRepository.save` |
| `FindTrackingPixelURL(ctx, campaignID)` | `SELECT url FROM tracking_pixels WHERE campaign_id = ?`; não encontrado → `("", nil)`. | `TrackingPixelRepository` (JPQL `findTrackingPixelByCampaignId`) |
| `FindCampaignActive(ctx, campaignID)` | Busca campanha por ID com `hour_cap`/`weekday_cap` para o postback validar elegibilidade (`campaignExistsAndIsActive`) via `internal/frequencycap`. Retorna a campanha ou `(nil, nil)`. | `CampaignComponentImpl.campaignExistsAndIsActive` |
| `FindCampaignVastOverride(ctx, campaignID)` | Override de VAST: junta `campaigns` + `creatives`, retorna `name`, `url_click`, `impression_tracker`, `url_portrait` quando `enabled=1` e hoje entre `start_date` e `end_date` (ver 02 §1.11). Consumido pelo vast-handler (M5-05). | `CampaignVastOverrideService.findValidOverride` |

### 3. Regras de implementação

- Tipos de data: `creation_date` é `DATETIME` e `event_date` é `DATE` — gravar ambos no timezone `America/Sao_Paulo` (nunca `time.Now()` direto na lógica: receber `time.Time` do chamador).
- Scan tolerante a NULL (`sql.NullString`/`sql.NullInt64`) nas colunas opcionais de `creatives` e `hotspots`.
- Comentário de origem em CADA método: `// Portado de: HotSpotRepository.java (findByCode)` etc.
- Todo identificador exportado com doc comment em português (CODE_DOCS_POLICY.md).

### 4. Testes — testcontainers-go (MySQL 8)

- `conn_test.go` + `queries_test.go` usando `testcontainers-go` com imagem `mysql:8`.
- O teste carrega um **schema mínimo de teste local** (arquivo `internal/repository/mysql/testdata/schema_test.sql`) contendo APENAS o DDL das tabelas usadas: `hotspots`, `campaigns`, `hotspots_campaigns`, `creatives`, `ad_trackers`, `tracking_pixels` (+ tabela de override VAST), derivado de [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2.
- ⚠️ **DEIXAR EXPLÍCITO em comentário no schema e no teste: este DDL existe SOMENTE dentro do container efêmero de teste. Os testes NUNCA rodam contra banco real (nem dev — espelha produção de outros projetos).** Nenhuma config de teste pode apontar para endpoint RDS.
- Casos: hotspot encontrado/não encontrado, campanhas filtradas por `enabled`, `InsertAdTracker` retorna ID crescente, pixel inexistente, override fora da janela de datas.

## Arquivos a criar/alterar

- `internal/repository/mysql/conn.go`
- `internal/repository/mysql/queries.go` (ou 1 arquivo por agregado: `hotspot.go`, `adtracker.go`, `pixel.go`, `campaign.go`)
- `internal/repository/mysql/testdata/schema_test.sql`
- `internal/repository/mysql/*_test.go`
- `go.mod` (+ `go-sql-driver/mysql`, `testcontainers-go`)
- `docs/MATRIZ-PARIDADE.md` (linhas dos repositórios MySQL)

## Critérios de aceite

- [ ] DSN carregado de SSM SecureString; nenhum segredo no código ou em testes
- [ ] `SetMaxOpenConns(2)` e `SetConnMaxLifetime(5m)` configurados; DSN com `parseTime=true` e `loc=America/Sao_Paulo`
- [ ] 5 métodos implementados com as queries SQL à mão (sem ORM, sem query builder)
- [ ] `InsertAdTracker` retorna o `LastInsertId` do MySQL
- [ ] Testes testcontainers-go (MySQL 8) verdes com schema mínimo local; comentário explícito de que NUNCA rodam contra banco real
- [ ] Nenhum `CREATE/ALTER/DROP` fora do `testdata/schema_test.sql` de container efêmero
- [ ] Comentários `// Portado de: <Classe>.java` em toda lógica portada; godoc em português
- [ ] `make lint && make test` verdes; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M2-03 (RDS Proxy), M1-08 (config SSM/platform)

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2 (tabelas), §6 (queries do hot path)
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.11, §2 (repositórios Java)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-002, ADR-006
- Java: `ad-server/src/main/java/br/com/intv/adserver/integration/` e entities do `ad-commons`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-01] repository/mysql no repo InteliFi/ad-serverless:
pacote internal/repository/mysql com database/sql + go-sql-driver/mysql (SEM ORM),
DSN de SSM apontando para RDS Proxy, SetMaxOpenConns(2)/SetConnMaxLifetime(5m),
parseTime=true e loc=America/Sao_Paulo. Implementar FindHotspotByCode,
InsertAdTracker (retorna LastInsertId), FindTrackingPixelURL, FindCampaignActive
e FindCampaignVastOverride com as queries de docs/legado/04-modelo-dados.md §6.
Testes com testcontainers-go (mysql:8) e schema mínimo em testdata — NUNCA contra
banco real. Código comentado em português, // Portado de: nas lógicas, atualizar
MATRIZ-PARIDADE, abrir PR.
```
