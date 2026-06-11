---
title: "[M6-01] report-handler: GET /adtrack (JSON, agregação no banco)"
labels: ["epic:M6-relatorios", "tipo:port", "prioridade:P1"]
milestone: "M6 — Relatórios"
---
## Contexto

`GET /adtrack` gera o relatório agregado de eventos de tracking: por campanha → data do evento → hotspot, com 12 contadores nomeados por tipo de evento. No legado (`TrackerReportComponent`), a agregação é feita **em memória na aplicação**, paginando a tabela `ad_trackers` inteira em batches de 1000 linhas (`while hasMore`) — um anti-pattern documentado: com **~14M de linhas**, o endpoint é lento e arrisca OOM. A própria especificação do legado (docs/legado/02 §1.6) e a arquitetura alvo (tabela de Lambdas: `report-handler ... agregação SQL GROUP BY`) determinam que a migração mova a agregação para o banco.

**Contrato de paridade:** o que NÃO pode mudar é a **saída JSON** (`AdTrackerReport`/`AdTrackerReportItem`) — mesmos nomes de campo, mesma estrutura de agrupamento, mesmos contadores. A implementação interna (SQL `GROUP BY` em vez de batches em memória) muda por decisão de arquitetura já registrada. O resultado da query agregada é matematicamente idêntico ao do loop em memória (mesma contagem por chave) — validar com golden test contra resposta capturada do Java.

Esta Lambda fica FORA do hot path: memória **1024MB**, timeout **120s** (tabela de Lambdas da ARQUITETURA-ALVO §3).

## Especificação detalhada

### Query agregada (substitui os batches de 1000)

```sql
-- Agregação no banco: 1 linha por (campanha, dia, hotspot, tipo de evento).
-- Usa o índice overview_idx(campaign_id, event_date, event_type) — V4.
SELECT campaign_id,
       event_date,
       hotspot_id,
       event_type,
       COUNT(*) AS total
  FROM ad_trackers
 GROUP BY campaign_id, event_date, hotspot_id, event_type
 ORDER BY campaign_id, event_date, hotspot_id;
```

- **Somente SELECT** — regra inegociável 2 do CLAUDE.md (nenhum DDL, nenhum índice novo; o `overview_idx` já existe).
- Executar com `QueryContext` e o `context.Context` da invocação (timeout da Lambda cancela a query).
- `hotspot_id` é NULL em eventos de tracking pixel/postback → ler com `sql.NullString` e reproduzir o comportamento do Java para a chave de agrupamento (conferir em `TrackerReportComponent.java` se NULL vira string vazia ou chave própria — documentar a decisão com referência à linha do Java).
- Conexão via repositório MySQL de M3-01 (RDS Proxy, `SetMaxOpenConns(2)`); apenas adicionar o método de leitura agregada:

```go
// AgregadoTracking devolve as contagens por (campanha, dia, hotspot, evento).
// Substitui a paginação em batches de 1000 do TrackerReportComponent.java —
// mesma matemática, executada pelo MySQL (índice overview_idx).
func (r *AdTrackerRepository) AgregadoTracking(ctx context.Context) ([]LinhaAgregada, error)

// LinhaAgregada é uma linha do GROUP BY.
type LinhaAgregada struct {
    CampaignID int            // campaign_id
    EventDate  string         // event_date (yyyy-MM-dd)
    HotspotID  sql.NullString // hotspot_id (NULL em pixel/postback)
    EventType  string         // event_type (valor persistido)
    Total      int64          // COUNT(*)
}
```

### Montagem do relatório — port de `TrackerReportComponent.generateReport`

Estrutura de agrupamento do legado: **campanha → eventDate → hotspot**, com um item por combinação e os contadores preenchidos a partir do `event_type`:

```go
// AdTrackerReport é o DTO raiz da resposta (paridade com AdTrackerReport.java).
type AdTrackerReport struct {
    Items []AdTrackerReportItem `json:"items"`
}

// AdTrackerReportItem agrega os contadores de uma chave campanha/dia/hotspot.
// Os NOMES dos campos JSON devem ser idênticos aos do DTO Java — conferir a
// serialização Jackson em AdTrackerReportItem.java e ajustar as tags json.
type AdTrackerReportItem struct {
    CampaignID            int    `json:"campaignId"`
    EventDate             string `json:"eventDate"`
    HotspotID             string `json:"hotspotId"`
    PageView              int64  `json:"pageView"`
    ImpressionPreRoll     int64  `json:"impressionPreRoll"`
    ClickPreRoll          int64  `json:"clickPreRoll"`
    ImpressionCampaign    int64  `json:"impressionCampaign"`
    ClickCampaign         int64  `json:"clickCampaign"`
    VideoStarted          int64  `json:"videoStarted"`
    Played25Per           int64  `json:"played25Per"`
    Played50Per           int64  `json:"played50Per"`
    Played75Per           int64  `json:"played75Per"`
    VideoEnd              int64  `json:"videoEnd"`
    Redirect              int64  `json:"redirect"`
    TrackingPixel         int64  `json:"trackingPixel"`
}
```

**Os 12 contadores nomeados** (mapeamento `event_type` persistido → campo):

| event_type (valor no banco) | Contador |
|---|---|
| `PAGE_VIEW` | PageView |
| `IMPRESSION_PRE_ROLL` | ImpressionPreRoll |
| `CLICK_PRE_ROLL` | ClickPreRoll |
| `IMPRESSION_CAMPAIGN` | ImpressionCampaign |
| `CLICK_CAMPAIGN` | ClickCampaign |
| `VIDEO_STARTED` | VideoStarted |
| `25_PER_PLAYED` | Played25Per |
| `50_PER_PLAYED` | Played50Per |
| `75_PER_PLAYED` | Played75Per |
| `VIDEO_ENDED` | VideoEnd |
| `REDIRECT_CAMPAIGN` | Redirect |
| `TRACKING_PIXEL` | TrackingPixel |

- ⚠️ Usar as constantes de M1-01 (valores persistidos — `25_PER_PLAYED`, não `PLAYED_25_PER`); `event_type` desconhecido (ex.: `POSTBACK_*`) → reproduzir o comportamento do Java (conferir se é ignorado ou contado em algum campo — documentar com referência).
- ⚠️ Nomes/casing dos campos JSON: a issue assume camelCase Jackson default; **conferir no DTO Java real** (`c:\Users\Fabio\Documents\Dev\ad-server`) e, havendo `@JsonProperty`, copiar EXATAMENTE. Capturar uma resposta real do Java como golden fixture (`tests/golden/report/adtrack-report.json`).
- Ordem dos items: reproduzir a ordem do Java (iteração dos agrupamentos); com a query ordenada acima a saída é determinística — golden test garante.

### Handler e roteamento

- `cmd/report/main.go` — roteia `GET /adtrack` (este endpoint) e `GET /adtrack/xls` (M6-02) pelo `rawPath`.
- ⚠️ Desambiguação: `POST /adtrack` é do track-handler (M3-03) e `GET /adtrack/postback` do postback-handler — no API Gateway as rotas são distintas por método/path; documentar no main.
- Resposta: `200` `application/json;charset=UTF-8` com `AdTrackerReport` (helper `resp.JSON` de M1-08). Erro de query → `500` genérico + log slog com o erro real.
- Middleware: `Recover` + `CORS` + `RequestValidation` (M1-07).
- `serverless.yml`: function `report-handler` com `memorySize: 1024`, `timeout: 120`, rotas `GET /adtrack` e `GET /adtrack/xls`, env/SSM do MySQL (mesmo padrão de M3-01).

### Testes obrigatórios

1. **Mapeamento de contadores:** tabela com as 12 linhas acima — cada `event_type` incrementa SOMENTE o campo correto.
2. **Agrupamento:** linhas agregadas de 2 campanhas × 2 dias × 2 hotspots → 8 items com os totais certos.
3. **hotspot NULL:** linha com `hotspot_id` NULL → comportamento documentado (paridade com Java).
4. **event_type desconhecido:** não corrompe o relatório (comportamento igual ao Java).
5. **Golden test:** fixture de linhas agregadas → JSON byte a byte igual a `tests/golden/report/adtrack-report.json` (capturado do Java).
6. **Handler:** repositório mockado; 200 com JSON; erro do repositório → 500 sem vazar detalhes.

## Arquivos a criar/alterar

- `internal/report/doc.go`, `report.go` (DTOs + montagem; `// Portado de: TrackerReportComponent.java (generateReport) — agregação movida para SQL GROUP BY conforme ARQUITETURA-ALVO`), `report_test.go`.
- `internal/repository/mysql/adtracker_report.go` — `AgregadoTracking` + `LinhaAgregada` (+ teste).
- `cmd/report/main.go` — Lambda handler com roteador interno.
- `serverless.yml` — function `report-handler` (1024MB / 120s).
- `tests/golden/report/adtrack-report.json` — fixture capturada do Java.
- `docs/MATRIZ-PARIDADE.md` — atualizar linha do `GET /adtrack`.

## Critérios de aceite

- [ ] Agregação 100% no MySQL via `GROUP BY campaign_id, event_date, hotspot_id, event_type` + `COUNT(*)`; NENHUMA paginação em memória; somente SELECT (zero DDL).
- [ ] DTOs `AdTrackerReport`/`AdTrackerReportItem` com nomes JSON idênticos aos do Java (conferidos no fonte) e os 12 contadores da tabela mapeados pelas constantes de M1-01.
- [ ] Golden test contra resposta capturada do Java passa byte a byte (mesma ordem de items).
- [ ] Comportamentos de borda (hotspot NULL, event_type desconhecido) idênticos ao Java, com comentário citando a linha de origem.
- [ ] `serverless.yml`: 1024MB, timeout 120s, rotas corretas sem conflito com `POST /adtrack`/`GET /adtrack/postback`.
- [ ] Query usa `QueryContext` com o context da invocação; erro → 500 genérico + log estruturado.
- [ ] Código 100% comentado em português; `// Portado de: TrackerReportComponent.java` presente; `make lint && make test` verdes.

## Dependências

Bloqueada por: M3-01 (repositório MySQL/RDS Proxy).
Bloqueia: M6-02 (XLS reusa a mesma agregação).

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §6 (contrato do endpoint e aviso das 14M linhas).
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.6 (TrackerReportComponent: batches de 1000, agrupamento campanha→eventDate→hotspot, 12 contadores, anti-pattern documentado).
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2.4 (tabela ad_trackers + índice overview_idx).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (report-handler 1024MB/120s, "agregação SQL GROUP BY").
- Código Java de referência: `c:\Users\Fabio\Documents\Dev\ad-server` (`TrackerReportComponent.java`, `AdTrackerReport.java`, `AdTrackerReportItem.java`, `AdTrackService.java`).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M6-01 (report-handler GET /adtrack) do ad-serverless: substituir a agregação em memória do TrackerReportComponent.java (batches de 1000 sobre ~14M linhas) por SELECT campaign_id, event_date, hotspot_id, event_type, COUNT(*) FROM ad_trackers GROUP BY campaign_id, event_date, hotspot_id, event_type (somente SELECT, zero DDL), preservando EXATAMENTE a estrutura JSON de saída AdTrackerReport/AdTrackerReportItem com os 12 contadores (PAGE_VIEW, IMPRESSION_PRE_ROLL, CLICK_PRE_ROLL, IMPRESSION_CAMPAIGN, CLICK_CAMPAIGN, VIDEO_STARTED, 25/50/75_PER_PLAYED, VIDEO_ENDED, REDIRECT_CAMPAIGN, TRACKING_PIXEL) — conferir nomes JSON no DTO Java em c:\Users\Fabio\Documents\Dev\ad-server e validar com golden test. Lambda report-handler com 1024MB e timeout 120s no serverless.yml. Seguir docs/issues/M6-01-report-handler-json.md e CLAUDE.md, código 100% comentado em português, atualizar MATRIZ-PARIDADE, make lint && make test verdes, abrir PR feat/issue-M6-01-report-handler-json com Closes na issue.
```
