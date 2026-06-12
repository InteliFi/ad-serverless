---
title: "[M1-01] internal/domain: structs e enums do domínio"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P1"]
milestone: "M1 — Commons Go"
---

> 📌 **Status (2026-06-12):** Implementado — 7 structs + 4 enums com testes.
> PR #TBD aberto (Closes #55). Nota: CreativeType tem 45 valores no Java
> (a spec dizia 46, mas a fonte real é o enum CreativeType.Values).

## Contexto

O ad-commons (Java v1.4.4) define as entities JPA que modelam o domínio do ad server: campanhas, creatives, hotspots e eventos de tracking. TODA a lógica dos demais pacotes (`frequencycap`, `selection`, `templates`, `tracking`, repositórios) depende dessas structs. Esta issue porta o modelo de dados para `internal/domain` em Go, **sem nenhuma mudança de schema** (o MySQL é compartilhado — ver ADR-006). Cada campo deve citar a coluna de origem em comentário, conforme [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) §5.

## Especificação detalhada

### Struct `Campaign` (tabela `campaigns`, ~90 registros)

Campos (coluna → campo Go, tags `db:"..."`): `id`, `name` VARCHAR(50), `enabled` TINYINT(1), `start_date`/`end_date` DATE, `hour_cap` VARCHAR(50) (ex.: `"0;5>10;15>>"`), `weekday_cap` VARCHAR(50) (ex.: `"1;2>5"`, 1=SEG…7=DOM), `event_cap` VARCHAR(50), `event_cap_limit` INT(11), `event_cap_hours_limit` INT(5) (os três `event_cap*` são **reservados, sem lógica no legado** — documentar no comentário), `cpe` DECIMAL(15,2), `campaign_deal` DECIMAL(15,2), `advertiser` VARCHAR(100), `agency_id` INT FK, `frequency_cap` INT NULL (V10). Relacionamento: slice `Creatives []Creative` (carregado pelas queries do hot path, sem ORM).

### Struct `Creative` (tabela `creatives`) — todos os ~35 campos mapeados

- Chaves: `id`, `campaign_id`, `creative_type_id`.
- **Deprecated (manter por paridade — VARCHAR(300)):** `url_portrait`, `url_postroll`, `url_answer_no`, `url_click`, `url_tracking`.
- **URLs ativas (VARCHAR(200)):** `url_bg`, `url_bg_mobile`, `url_preroll`, `url_preroll_mobile`, `url_video`, `url_video_mobile`, `url_banner_campaign`, `url_banner_campaign_mobile`, `url_redirect`, `url_redirect_mobile`, `url_install_google`, `url_install_apple`, `url_install_google_mobile`, `url_install_apple_mobile`.
- **Cores (V12, VARCHAR(7)):** `title_color`, `title_color_mobile`, `button_color`, `button_color_mobile`.
- **Trackers (V12):** `tracker_type` VARCHAR(50) (enum PIXEL|SCRIPT); `page_view_tracker`, `impression_tracker`, `click_campaign_tracker`, `video_started_tracker`, `played_25_per_tracker`, `played_50_per_tracker`, `played_75_per_tracker`, `video_end_tracker` — todos VARCHAR(1024).
- **Extras (VARCHAR(1024)):** `title_literals`, `prebid_code`.

### Struct `HotSpot` (tabela `hotspots`, ~928 registros)

`id`, `code` VARCHAR(100) NOT NULL (**chave de cache**, buscado em UPPER CASE), `description` VARCHAR(50), `physical_id` VARCHAR(100), `local_name` VARCHAR(255), `mac_address` VARCHAR(255), `data_plan_renew_month_day` INT, `msp_monthly_fee` INT, `segment`/`partner` VARCHAR(100) (strings legadas). FKs de enriquecimento (maioria NULL, declarar como `sql.NullInt64`/ponteiros): `segment_id`, `partner_id`, `country`, `msp`, `city`, `state`, `ssid`, `operator_id`, `data_plan`, `modem`, `manufacture`, `carrier`, `msp_fee_currency`, `os`. N:M com campanhas via `hotspots_campaigns(hotspot_id, campaign_id)`.

### Structs de tracking

- `AdTracker` (tabela `ad_trackers`, ~14M linhas): `id`, `campaign_id` INT (sem FK ORM), `hotspot_id` VARCHAR(50) (**NULL em tracking pixel e postback** — usar `sql.NullString` e documentar), `event_type` VARCHAR(50) (valor string do EventType), `creation_date` DATETIME, `event_date` DATE.
- `TrackingPixel` (tabela `tracking_pixels`): `id`, `campaign_id` FK, `url` VARCHAR(200).
- `AdTrackerEvent` (réplica DynamoDB, tabela `AdTrackers`), tags `dynamodbav:"..."`: PK `campaign_id` (S), SK `created_at_id` (S, formato EXATO `<ISO8601>#<rds_id>`, ex.: `2024-06-10T15:30:45.123Z#12345`), `hotspot_id` (S), `event_type` (S), `event_date` (S `yyyy-MM-dd` em America/Sao_Paulo), `rds_id` (N).
- `PostbackLog` (DynamoDB `PostbackLogs`): PK `transaction_id` (S, fallback para `click_id`), SK `logged_at` (S, ISO8601), `campaign_id` (S), `event`, `aff_sub`, `click_id`, `source`, `payout` (N), `currency`, `sale_amount` (N).

### Constantes `EventType` (17 valores) — ⚠️ valor persistido ≠ nome do enum

| Nome Java | Valor persistido (usar em Go) |
|---|---|
| PAGE_VIEW | `PAGE_VIEW` |
| IMPRESSION_PRE_ROLL | `IMPRESSION_PRE_ROLL` |
| CLICK_PRE_ROLL | `CLICK_PRE_ROLL` |
| IMPRESSION_CAMPAIGN | `IMPRESSION_CAMPAIGN` |
| CLICK_CAMPAIGN | `CLICK_CAMPAIGN` |
| VIDEO_STARTED | `VIDEO_STARTED` |
| PLAYED_25_PER | `25_PER_PLAYED` ⚠️ |
| PLAYED_50_PER | `50_PER_PLAYED` ⚠️ |
| PLAYED_75_PER | `75_PER_PLAYED` ⚠️ |
| VIDEO_END | `VIDEO_ENDED` ⚠️ |
| TRACKING_PIXEL | `TRACKING_PIXEL` |
| REDIRECT | `REDIRECT_CAMPAIGN` ⚠️ |
| POSTBACK_CLICK | `POSTBACK_CLICK` |
| POSTBACK_CPL | `POSTBACK_CPL` |
| POSTBACK_CPA | `POSTBACK_CPA` |
| POSTBACK_INSTALL_ANDROID | `POSTBACK_INSTALL_ANDROID` |
| POSTBACK_INSTALL_IOS | `POSTBACK_INSTALL_IOS` |

Em Go: constantes string com os **valores persistidos** + função `EventTypeValido(s string) bool` (necessária para M1-06). Comentar a divergência nome×valor (paridade com `EventType.Values` do ad-commons).

### Constantes `CreativeType` (46 valores — listar TODOS)

`BANNER`, `VIDEO`, `BANNER_QUESTION`, `BANNER_QUESTION_NO_AUTH`, `APP_INSTALL`, `BANNER_BUS_APP_INSTALL`, `BANNER_BUTTONCLOSE`, `JUST_BANNER`, `VAST`, `VIDEO_VPAID_SPACE`, `BANNER_SPACE`, `WICONNECT_VIDEO`, `WICONNECT_BANNER`, `SMARTAD_RSS`, `GOOGLE_AD_UNIT`, `REDIRECT_POSTBACK`, `GAM_AERO_VIX`, `GAM_SPTRANS_NARDELLI`, `PROGRAMATICA_VAST`, `PROGRAMATICA_SELFCLOSE`, `PROGRAMATICA`, `PROGRAMATICA_SMART`, `PROGRAMATICA_CLARO`, `PROGRAMATICA_WEBMOTORS`, `PROGRAMATICA_BMC`, `BANNER_WEBMOTORS`, `BANNER_BMC`, `PROGRAMATICA_CLARO_PREROLL_CLICK`, `CAMPAIGN_PROGRAMATICA_CLARO_PREROLL_CLICK`, `VIDEO_CAMPAIGN_PROGRAMATICA`, `BETANO_PREZAO`, `SMART_CLARO_APP`, `ADFORCE_DISPLAY_BANNER`, `VIDEO_VPAID_SPACE_WICO`, `SMARTAD_SPTRANS`, `SMARTAD_AER_BSB`, `SMARTAD_AER_VCP`, `IMA`, `IMA_PROGRAMATICA`, `CAMPAIGN_PROGRAMATICA`, `NOAD_BANNER_PROGRAMATICA`, `VAST420`, `SPTRANS_BANNER`, `PIXEL_TRACKING_SERASA`, `UNDEF`.

### Constantes `ContentType` (7) e `TrackerType` (2)

- ContentType (nome → código): `BANNER_PRE_ROLL_MOBILE`→`BPRM`, `BANNER_CAMPAIGN_MOBILE`→`BCM`, `BACKGROUND_MOBILE`→`BGM`, `BANNER_PRE_ROLL_TABLET_DESKTOP`→`BPRTD`, `BANNER_CAMPAIGN_TABLET_DESKTOP`→`BCTD`, `BACKGROUND_TABLET_DESKTOP`→`BGTD`, `VIDEO`→`VID`.
- TrackerType: `PIXEL` (tracker `<img>`), `SCRIPT` (tracker `<script>`).

## Arquivos a criar/alterar

- `internal/domain/doc.go` — godoc do pacote em português.
- `internal/domain/campaign.go`, `creative.go`, `hotspot.go`, `adtracker.go` (AdTracker + AdTrackerEvent), `trackingpixel.go`, `postbacklog.go`.
- `internal/domain/eventtype.go`, `creativetype.go`, `contenttype.go`, `trackertype.go`.
- `internal/domain/domain_test.go` — testes das constantes e helpers.

## Critérios de aceite

- [ ] `Creative` contém os ~35 campos de URL/cores/trackers listados acima, cada um com comentário citando a coluna MySQL de origem.
- [ ] 17 constantes de EventType com os **valores persistidos** (incluindo `25_PER_PLAYED`, `50_PER_PLAYED`, `75_PER_PLAYED`, `VIDEO_ENDED`, `REDIRECT_CAMPAIGN`); teste valida cada string.
- [ ] 46 constantes de CreativeType, 7 de ContentType (com códigos), 2 de TrackerType.
- [ ] `AdTrackerEvent` com tags `dynamodbav` preservando nomes EXATOS (`campaign_id`, `created_at_id`, `hotspot_id`, `event_type`, `event_date`, `rds_id`); `PostbackLog` idem.
- [ ] Campos anuláveis (`hotspot_id`, FKs de hotspot) tipados como `sql.Null*` ou ponteiro, com comentário explicando quando são NULL.
- [ ] Comentário `// Portado de: Campaign.java / Creative.java / HotSpot.java / AdTracker.java / EventType.java ... (ad-commons)` em cada arquivo.
- [ ] `make lint && make test` verdes; todo símbolo exportado com doc comment em português (revive `exported`).

## Dependências

Bloqueada por: M0-01 (bootstrap do repositório Go).
Bloqueia: M1-02, M1-04, M1-05, M1-06, M3-02.

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2 (tabelas), §4 (DynamoDB), §6 (mapeamento Go).
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §4 (enums completos).
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) §4 e §5.

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-01 (internal/domain) do ad-serverless: criar as structs Campaign, Creative (~35 campos de URL/cores/trackers), HotSpot, AdTracker, TrackingPixel, PostbackLog e AdTrackerEvent com tags db/dynamodbav e comentário por campo citando a coluna de origem; criar as constantes EventType (17 — usar os VALORES PERSISTIDOS: 25_PER_PLAYED, 50_PER_PLAYED, 75_PER_PLAYED, VIDEO_ENDED, REDIRECT_CAMPAIGN), CreativeType (46), ContentType (7) e TrackerType (2). Seguir docs/issues/M1-01-domain-structs-enums.md à risca, com TODO o código comentado em português (CODE_DOCS_POLICY.md), comentários "// Portado de: <ClasseJava>", testes unitários verdes (make lint && make test) e abrir PR na branch feat/issue-M1-01-domain-structs-enums com Closes na issue.
```
