# Sistema Legado — Modelo de Dados Completo (MySQL + DynamoDB)

> **Fonte:** análise das entities JPA do [ad-commons](https://github.com/InteliFi/ad-commons) (v1.4.4) e das 30 migrations Flyway do ad-server.
> ⚠️ **ATENÇÃO (diretriz do engenheiro-chefe):** o banco MySQL é **compartilhado com outros projetos** e não há CI/CD para atualizar produção. **NENHUMA mudança de schema** na Fase 1 da migração — as Lambdas Go leem/escrevem o schema existente como está. Mudanças de banco ficam para a fase final, coordenadas (ver Epic de Banco de Dados).

## 1. Visão geral

| Banco | Uso | Tabelas |
|---|---|---|
| MySQL RDS (`adserver`) | Entidades de negócio + log de eventos | 28+ tabelas |
| DynamoDB (sa-east-1) | Réplica de tracking + logs de postback | `AdTrackers`, `PostbackLogs` |

Endpoints RDS: dev `dev-mysql.ckkqlpl6ei1d.us-east-1.rds.amazonaws.com`, prod `prod-mysql.cglsxksyzbur.sa-east-1.rds.amazonaws.com`. Timezone de conexão: `GMT-3`.

## 2. Tabelas principais (MySQL)

### 2.1 `campaigns` (~90 registros)

| Coluna | Tipo | Notas |
|---|---|---|
| id | INT PK AUTO | |
| name | VARCHAR(50) | |
| enabled | TINYINT(1) NOT NULL default 0 | flag de ativação |
| start_date / end_date | DATE | janela de veiculação |
| hour_cap | VARCHAR(50) | FrequencyCap embarcado: `"0;5>10;15>>"` |
| weekday_cap | VARCHAR(50) | `"1;2>5"` (1=SEG…7=DOM) |
| event_cap | VARCHAR(50) | reservado, sem lógica |
| event_cap_limit | INT(11) | reservado |
| event_cap_hours_limit | INT(5) | reservado |
| cpe | DECIMAL(15,2) default 0.00 | custo por evento |
| campaign_deal | DECIMAL(15,2) default 0.00 | valor do contrato |
| advertiser | VARCHAR(100) | |
| agency_id | INT FK→agencies | |
| frequency_cap | INT NULL | (V10) |

Relacionamentos: 1:N creatives (cascade REMOVE), N:M hotspots (via `hotspots_campaigns`), 1:N campaign_user (LAZY), N:1 agency.

### 2.2 `creatives`

PK `id`; FKs `campaign_id` (LAZY), `creative_type_id`.
**Deprecated (legado):** `url_portrait`, `url_postroll`, `url_answer_no`, `url_click`, `url_tracking` VARCHAR(300).
**Ativos:** `url_bg`, `url_bg_mobile`, `url_preroll`, `url_preroll_mobile`, `url_video`, `url_video_mobile`, `url_banner_campaign`, `url_banner_campaign_mobile`, `url_redirect`, `url_redirect_mobile`, `url_install_google`, `url_install_apple`, `url_install_google_mobile`, `url_install_apple_mobile` — todos VARCHAR(200).
**Cores (V12):** `title_color`, `title_color_mobile`, `button_color`, `button_color_mobile` VARCHAR(7).
**Trackers (V12):** `tracker_type` VARCHAR(50) enum PIXEL|SCRIPT; `page_view_tracker`, `impression_tracker`, `click_campaign_tracker`, `video_started_tracker`, `played_25_per_tracker`, `played_50_per_tracker`, `played_75_per_tracker`, `video_end_tracker` VARCHAR(1024); `title_literals` VARCHAR(1024); `prebid_code` VARCHAR(1024).

### 2.3 `hotspots` (~928 registros)

PK `id`; `code` VARCHAR(100) NOT NULL (**chave de cache**, único na prática); `description` VARCHAR(50); `physical_id` VARCHAR(100); `local_name` VARCHAR(255) (V19); `mac_address` VARCHAR(255) (V24); `data_plan_renew_month_day` INT (V23); `msp_monthly_fee` INT (V28).
Strings legadas: `segment`, `partner` VARCHAR(100).
FKs de enriquecimento (V14–V29, maioria NULL): `segment_id`→segments, `partner_id`→partners, `country`→countries, `msp`→msps, `city`→cities, `state`→states, `ssid`→ssid, `operator_id`→operator, `data_plan`→data_plan, `modem`→modem, `manufacture`→manufacture, `carrier`→carrier, `msp_fee_currency`→msp_fee_currency, `os`→os.
N:M campaigns via `hotspots_campaigns(hotspot_id, campaign_id)`.

### 2.4 `ad_trackers` (~14M linhas — WRITE-HEAVY) ⚠️

| Coluna | Tipo | Notas |
|---|---|---|
| id | INT PK AUTO | |
| campaign_id | INT | referência simples, sem FK ORM |
| hotspot_id | VARCHAR(50) | NULL para tracking pixel/postback |
| event_type | VARCHAR(50) | valor string do EventType |
| creation_date | DATETIME default CURRENT_TIMESTAMP | timestamp exato |
| event_date | DATE | para agrupar por dia |

Índices: `campaign_evdate_idx(campaign_id, event_date)` (V2), `overview_idx(campaign_id, event_date, event_type)` (V4).

### 2.5 Demais tabelas

- `tracking_pixels(id, campaign_id FK, url VARCHAR(200))`
- `agencies(id, name, logo_url, cnpj, business_name, email, phone)` + índice `agency_name_idx`
- `agency_contacts(id, agency_id FK LAZY, name, email, phone)`
- `campaign_user(id PK AUTO (V5), user_id VARCHAR(50), campaign_id FK)` + índice `campaign_id_idx`
- `google_demand(id, date, demand_channel VARCHAR(16) enum[AD_SERVER, AD_SENSE, AD_EXCHANGE, EXCHANGE_BIDDING, MEDIATION], ad_unit_code, unfilled_impressions, total_impressions, total_clicks, total_revenue DECIMAL(12,6), total_avg_ecpm DECIMAL(12,6), total_ctr DECIMAL(5,4))` + UNIQUE(date, demand_channel, ad_unit_code)
- Lookups: `creative_types(id, name, enabled)`, `event_types(id, name)`, `content_types(id, code)`
- Lookups de hotspot (V14–V29): `segments`, `partners`, `countries`, `msps`, `cities`, `states`, `ssid`, `operator`, `data_plan`*, `modem(id, model, brand)`, `manufacture(id, model, brand)`, `carrier(id, brand)`, `msp_fee_currency(id, acronym, description)`, `os(id, name, version)`
- `dynamodb_migrations(version, description, table_name)` (V30) — tracking de migrations DynamoDB
- Tabela de override VAST de campanha (migration recente, commit `f459e85`) — campos `enabled`, `start_date`, `end_date`, URL de vídeo/click/impression tracker por campanha

## 3. Histórico de migrations Flyway (V1–V30)

| V | Arquivo | O que faz |
|---|---|---|
| V1 | base_version_20180126 | Tabelas base: hotspots, campaigns, hotspots_campaigns, creative_types, creatives, content_types, event_types, ad_trackers, tracking_pixels |
| V2 | create_campaign-user_table | campaign_user (PK composto) + índice campaign_evdate_idx |
| V3 | create_new_campaign_fields | cpe, campaign_deal |
| V4 | create_index_overview | overview_idx em ad_trackers |
| V5 | change_campaign_user | PK→id auto, remove is_adm, índice campaign_id_idx |
| V6 | add_hotspots_partner | hotspots.partner |
| V7 | add_creative_type_status | creative_types.enabled |
| V8 | review_creative_urls | remove url_landscape, +11 colunas URL em creatives |
| V9 | create_agencies_table | agencies + agency_contacts |
| V10 | add_frequency_cap_to_campaigns / create_google_demand_table | frequency_cap; google_demand |
| V11 | new_campaign_fields | agency_id, advertiser, event_cap* |
| V12 | new_creative_fields | cores, tracker_type, 8 trackers, title_literals |
| V13 | change_creatives_url_tracking | url_tracking → VARCHAR(300) |
| V14–V29 | enriquecimento hotspots | segments, partners, countries, msps, cities, local_name, states, ssid, operator, data_plan, mac_address, modem, manufacture, carrier, msp_fee, os |
| V30 | create_dynamodb_migrations_table | dynamodb_migrations |

## 4. DynamoDB

### 4.1 `AdTrackers`
- **PK** `campaign_id` (S), **SK** `created_at_id` (S, `<ISO8601>#<rds_id>`)
- Atributos: `hotspot_id` (S), `event_type` (S), `event_date` (S `yyyy-MM-dd` em America/Sao_Paulo), `rds_id` (N)
- Tabela classe `STANDARD_INFREQUENT_ACCESS`, billing `PAY_PER_REQUEST`
- Padrão: putItem (upsert); particionado por campanha → range queries cronológicas por campanha

### 4.2 `PostbackLogs`
- **PK** `transaction_id` (S — fallback para click_id), **SK** `logged_at` (S, ISO8601)
- Atributos: `campaign_id` (S), `event`, `aff_sub`, `click_id`, `source`, `payout` (N), `currency`, `sale_amount` (N)

## 5. Algoritmos de dados críticos (paridade obrigatória)

### FrequencyCap parsing (DigitExtractor)
```
Input:  "0;5>10;15>>"
Split:  ";" → ["0", "5>10", "15>>"]
"0"     → Digit{0}
"5>10"  → DigitRange exclusivo {5,6,7,8,9}      (min até max-1; valida min<=max)
"15>>"  → DigitRange inclusivo {15..23}          (">>"; sem max = até limite do domínio)
Inválido → NullDigit {} (ignorado silenciosamente)
Output: {0,5,6,7,8,9,15,...,23}
```

### PostbackSignature (MD5)
```
assinatura = hex(MD5(campaignId + event + key))          // key = config intv.ad.signaturekey
válida se signature.toLowerCase() == gerada
```

### Seleção aleatória (NumberUtils.getPositiveIndex)
```
índice = random.nextInt(tamanhoDaLista)   // uniforme, 0..n-1
```

### Conversões de data
- `DateUtils.DATE_FORMAT = "yyyyMMddHHmmssSSS"` (param `time` de /adtrack e /vasttrack) → Go layout: `20060102150405.000` **sem o ponto** — implementar parse custom de 17 dígitos.
- `event_date` DynamoDB: `yyyy-MM-dd` em `America/Sao_Paulo`.
- Postback: ZonedDateTime São Paulo truncado a segundos, mantendo offset (`-03:00`).

## 6. Mapeamento para Go (Fase 1 — schema intocado)

| Entity Java | Struct Go (internal/domain) | Acesso |
|---|---|---|
| Campaign | `Campaign` | `database/sql` SELECT (read-only no hot path) |
| Creative | `Creative` | SELECT junto com campaign |
| HotSpot | `HotSpot` | SELECT por code (cache 5min) |
| AdTracker | `AdTracker` | INSERT (hot path de escrita) |
| TrackingPixel | `TrackingPixel` | SELECT por campaign |
| Agency/AgencyContact/CampaignUser/GoogleDemand | structs | sem uso no hot path — portar para relatórios/admin futuros |
| PostbackLog | `PostbackLog` | DynamoDB PutItem |
| AdTrackerDynamo | `AdTrackerEvent` | DynamoDB PutItem |

Queries SQL do hot path (escrever à mão, sem ORM):
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
