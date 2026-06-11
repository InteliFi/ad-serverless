# Sistema Legado — Lógica de Negócio e Integrações

> **Fonte:** análise exaustiva dos pacotes `business/` e `integration/` do [ad-server](https://github.com/InteliFi/ad-server) (commit `74748d2`).
> Especificação de referência para reimplementação em Go — os algoritmos descritos aqui devem ser portados com paridade exata.

## 1. Componentes de Negócio

### 1.1 AdComponent — seleção de anúncio (núcleo do `/ad`)

Arquivo Java: `business/component/impl/AdComponentImpl.java`

Algoritmo de `getHotSpotAdScript(request)`:
1. Valida request (null → null).
2. Busca hotspot por código **UPPER CASE**: `hotSpotRepository.findByCode(...)` — com cache (`hotspots`, TTL 5 min, máx 500 itens).
3. Hotspot inexistente → retorna null (vira 404).
4. **Elegibilidade de campanhas:** filtra campanhas do hotspot com `enabled=true` E frequency cap satisfeito (hora + dia da semana — ver §1.3).
5. **Seleção aleatória uniforme** da campanha: `random.nextInt(size)`.
6. Nenhuma elegível → `NullCampaign` (Null Object; gera anúncio vazio).
7. **Seleção aleatória uniforme** do creative: `campaign.eligeCreative()`.
8. Decide template pelo `CreativeType` (enum com 40+ tipos → caminho do template).
9. Renderiza template substituindo placeholders `${key}` (ver §1.8).

Resiliência: `@Transactional(READ_COMMITTED, timeout=30)` + `@Retryable(CannotAcquireLockException, 3 tentativas, backoff 1s ×2)`.

### 1.2 CampaignComponent

- CRUD básico de campanhas (save persiste creatives primeiro, depois campaign).
- `campaignExistsAndIsActive(campaignId)`: busca por ID; valida `FrequencyCap` contra `LocalDateTime.now(clock)` — usado pelo postback.

### 1.3 FrequencyCapComponent — algoritmo de elegibilidade ⚠️ CRÍTICO

`isEligibleFor(dateTime, frequencyCap)`:
1. Parse de `hourCap` (string) → `Set<Hour>` (0–23).
2. Parse de `weekdayCap` (string) → `Set<Weekday>` (1=SEG … 7=DOM, padrão ISO).
3. Regra: `(hours vazio OU hours.contains(horaAtual)) E (weekdays vazio OU weekdays.contains(diaAtual))`.

**Formato das strings (DigitExtractor):**
- Separador de fragmentos: `;`
- Dígito único: `"5"` → {5}
- Range **exclusivo** `>`: `"5>10"` → {5,6,7,8,9}
- Range **inclusivo** `>>`: `"15>>23"` → {15,…,23}
- Fragmento inválido → `NullDigit` (conjunto vazio, ignorado)
- Exemplo completo: `"0;5>10;15>>"` → horas {0, 5–9, 15–23}

Campos reservados **não implementados** no legado: `eventCap`, `eventCapLimit`, `eventCapHoursLimit` (existem nas colunas, sem lógica).

### 1.4 AdTrackComponent — tracking duplo ⚠️ CRÍTICO

`save(adTracker)`:
1. Persiste em **MySQL** `ad_trackers` (síncrono, transacional) → obtém ID.
2. Replica em **DynamoDB** `AdTrackers` via `@Async` (fire-and-forget; erro apenas logado).

`savePostback(campaignId, event, timestamp)`:
1. Valida event type contra `EventType.Values` (inválido → `AdException("Invalid event type")`).
2. Valida assinatura — **comentado no código (TODO)**.
3. Valida campanha ativa (`campaignExistsAndIsActive`) → senão `CampaignNotFoundException`.
4. Persiste AdTracker com `hotspot = postbackHotspotCode` — **apenas MySQL** (sem DynamoDB).

### 1.5 AdTrackerDynamoComponent — réplica DynamoDB

- Tabela `AdTrackers` (config `aws.dynamodb.table.ad-trackers`), classe `STANDARD_INFREQUENT_ACCESS`, billing `PAY_PER_REQUEST`.
- **PK:** `campaign_id` (String). **SK:** `created_at_id` = `<ISO8601>#<rds_id>` (ex.: `2024-06-10T15:30:45.123Z#12345`) — ordenação cronológica + unicidade.
- Atributos: `hotspot_id`, `event_type`, `event_date` (formato `yyyy-MM-dd`, timezone **America/Sao_Paulo**), `rds_id` (ID do MySQL).
- Retry: `DynamoDbException`, 3 tentativas, backoff 1s ×2.

### 1.6 TrackerReportComponent — relatórios

- `generateReport(null)` pagina a tabela inteira em batches de **1000** (`while hasMore`).
- Agrupamento: campanha → eventDate → hotspot; contagem por tipo de evento (12 contadores: PAGE_VIEW, IMPRESSION_PRE_ROLL, CLICK_PRE_ROLL, IMPRESSION_CAMPAIGN, CLICK_CAMPAIGN, VIDEO_STARTED, 25/50/75_PER_PLAYED, VIDEO_ENDED, REDIRECT, TRACKING_PIXEL).
- `generateXls(report)`: HSSFWorkbook (POI 3.15), sheet `report_sheet`, temp file.
- ⚠️ **Anti-pattern conhecido:** agrega em memória aplicação; com ~14M linhas é lento. Na migração: `GROUP BY` no banco (ou consulta no DynamoDB) em Lambda dedicada.

### 1.7 TrackingPixelComponent

1. Busca `tracking_pixels.url` da campanha.
2. **Download da imagem** da URL para temp file.
3. Registra evento `TRACKING_PIXel` (`hotspot = null` deliberadamente) via AdTrackComponent (MySQL + DynamoDB).
4. Retorna o arquivo (servido como PNG).

### 1.8 TemplateComponent — renderização ⚠️ CRÍTICO

- Engine = **substituição literal de placeholders** `${key}` (NÃO é Velocity real — facilita port para Go com `strings.Replacer`).
- Templates `.vm` em `src/main/resources/templates/` (~45 arquivos): genéricos (emptyAd, bannerAd, videoAd, bannerQuestionAd, appInstallAd, justBannerAd, bannerButtonClose…), MSP WiConnect (`msps/wiconnect/*`), programática (25+: programaticaVAST, IMA, IMAProgramatica, programaticaClaro, programaticaWebMotors, programaticaBMC, betanoPrezao, smartClaroApp, smartAdRSS, googleAdUnit, redirectPostback…), space VPAID (`space/vpaid/videoVpaidSpace.vm`).
- Cache de template em `ConcurrentHashMap<String,String>` estático (carrega resource 1×) — em Go: `go:embed` + map.
- Falha de renderização → retorna null (nunca lança exceção).

**Placeholders produzidos por `CreativeTemplateRequest`:**
- `${cid}` — campaign ID; `${hid}` — hotspot code; `${spot}` — `physicalId + " " + campaignName`
- `${url_redirect}` — redirect URL do request
- `${z_server_timestamp}` — `System.currentTimeMillis()`
- `${ad_tracker_timestamp}` — timestamp `yyyyMMddHHmmssSSS`
- `${tracking_redirect_url}` — `https://ads.inteli.fi/redirect?hid={hid}&cid={cid}&enc=true&url={BASE64(urlTracking)}`
- Todos os campos de `Creative.toMap()`: `${url_portrait}`, `${url_bg}`, `${url_bg_mobile}`, `${url_preroll}`, `${url_preroll_mobile}`, `${url_video}`, `${url_video_mobile}`, `${url_banner_campaign}`, `${url_banner_campaign_mobile}`, `${url_redirect_mobile}`, `${url_install_google}`, `${url_install_apple}` (+mobile), `${title_color}`, `${button_color}` (+mobile), `${page_view_tracker}`, `${impression_tracker}`, `${click_campaign_tracker}`, `${video_started_tracker}`, `${played_25_per_tracker}`, `${played_50_per_tracker}`, `${played_75_per_tracker}`, `${video_end_tracker}`, `${title_literals}`, `${prebid_code}`
- `uniqueId` = hash(hotspotId, campaignId, creativeHashCode, redirectUrl)

### 1.9 PostbackLogComponent — log assíncrono no DynamoDB

`logPostback(campaignId, event, transactionId, affSub, clickId, source, payout, currency, saleAmount)` — `@Async`:
- `transaction_id` vazio → fallback para `clickId`.
- `payout`/`saleAmount`: parse seguro para BigDecimal (inválido → WARN, segue sem o campo).
- Tabela `PostbackLogs`: **PK** `transaction_id`, **SK** `logged_at` (ISO8601, setado no construtor). Atributos: `campaign_id`, `event`, `aff_sub`, `click_id`, `source`, `payout`, `currency`, `sale_amount`.
- Retry DynamoDbException 3× backoff exponencial; erro final apenas logado.

### 1.10 VideoCacheService — cache local de vídeo ⚠️ muda de arquitetura

`getCachedVideoUrl(originalUrl, clientRequest)`:
1. **Bypass URLs assinadas Google:** host `gcdn.2mdn.net` + path com `/videoplayback/` → `""`; host `googlevideo.com` + path com `/manifest/` → `""`.
2. **Whitelist de domínios** (`video.cache.whitelist.domains` = `gcdn.2mdn.net,googlevideo.com`): host fora → `""` (não cacheia).
3. Cache em memória `ConcurrentHashMap<url, filename>`; nome do arquivo = `MD5(url).mp4`.
4. Hit em disco → `/media/{filename}`. Miss → download (headers do cliente copiados, `Referer: https://ads.inteli.fi/`, `X-Forwarded-For` real, placeholder `ip/0.0.0.0` substituído pelo IP do cliente) → grava em `${video.cache.directory}` → `/media/{filename}`.
5. Qualquer erro → `""` (mantém URL original no VAST; nunca lança).

**Migração:** S3 (bucket de mídia) + CloudFront; chave = `MD5(url).mp4`; download via Lambda apenas em cache-miss; HEAD no S3 para verificar existência.

### 1.11 CampaignVastOverrideService

- Busca override de VAST por campanha (`findValidOverride(cid)`) com cache `campaignVastOverride` (5 min / 500).
- Validação: `enabled=true`, hoje entre `start_date` e `end_date`.
- Query junta `campaigns` + `creatives`: retorna `name`, `url_click`, `impression_tracker`, `url_portrait`.

## 2. Integrações / Repositórios

| Repositório | Backend | Detalhes |
|---|---|---|
| `CampaignRepository` | MySQL `campaigns` | CrudRepository |
| `HotSpotRepository` | MySQL `hotspots` | `findByCode` com `@Cacheable("hotspots")` |
| `CreativeRepository` | MySQL `creatives` | CrudRepository |
| `AdTrackRepository` | MySQL `ad_trackers` | save unitário; named native query `SELECT * FROM ad_trackers WHERE campaign_id = :campaign` |
| `TrackingPixelRepository` | MySQL `tracking_pixels` + HTTP | JPQL `FROM TrackingPixel tp WHERE tp.campaign = :campaignId`; download via `FileUtils.copyURLToFile` |
| `PostbackLogRepository` | DynamoDB `PostbackLogs` | Enhanced Client; retry 3× |
| `AdTrackerDynamoRepository` | DynamoDB `AdTrackers` | Enhanced Client; putItem (upsert); retry 3× |

### DynamoDbMigrationRunner (versionamento DynamoDB)
- Roda no startup; lê `classpath:db/dynamodb/V*.json` ordenado por nome.
- JSON declara: version, description, tableName (com placeholder de config), keySchema (HASH+RANGE), attributeDefinitions, tableClass.
- Cria tabela `PAY_PER_REQUEST` + `waitUntilTableExists`; registra na tabela MySQL `dynamodb_migrations`.
- **Migração:** substituído por recursos do Serverless Framework/CloudFormation (infra declarativa).

## 3. Exceções de Domínio

- `AdException` (base) → HTTP 422 no postback.
- `CampaignNotFoundException` → 404.
- `NotAuthorizedException` → 401.

## 4. Enums de Domínio

### EventType.Values (17)
`PAGE_VIEW`, `IMPRESSION_PRE_ROLL`, `CLICK_PRE_ROLL`, `IMPRESSION_CAMPAIGN`, `CLICK_CAMPAIGN`, `VIDEO_STARTED`, `PLAYED_25_PER("25_PER_PLAYED")`, `PLAYED_50_PER("50_PER_PLAYED")`, `PLAYED_75_PER("75_PER_PLAYED")`, `VIDEO_END("VIDEO_ENDED")`, `TRACKING_PIXEL`, `REDIRECT("REDIRECT_CAMPAIGN")`, `POSTBACK_CLICK`, `POSTBACK_CPL`, `POSTBACK_CPA`, `POSTBACK_INSTALL_ANDROID`, `POSTBACK_INSTALL_IOS`

⚠️ Atenção: o **nome** do enum difere do **valor** persistido em alguns casos (`PLAYED_25_PER` → `"25_PER_PLAYED"`, `REDIRECT` → `"REDIRECT_CAMPAIGN"`). Em Go usar constantes string com os **valores persistidos**.

### CreativeType.Values (45+)
`BANNER, VIDEO, BANNER_QUESTION, BANNER_QUESTION_NO_AUTH, APP_INSTALL, BANNER_BUS_APP_INSTALL, BANNER_BUTTONCLOSE, JUST_BANNER, VAST, VIDEO_VPAID_SPACE, BANNER_SPACE, WICONNECT_VIDEO, WICONNECT_BANNER, SMARTAD_RSS, GOOGLE_AD_UNIT, REDIRECT_POSTBACK, GAM_AERO_VIX, GAM_SPTRANS_NARDELLI, PROGRAMATICA_VAST, PROGRAMATICA_SELFCLOSE, PROGRAMATICA, PROGRAMATICA_SMART, PROGRAMATICA_CLARO, PROGRAMATICA_WEBMOTORS, PROGRAMATICA_BMC, BANNER_WEBMOTORS, BANNER_BMC, PROGRAMATICA_CLARO_PREROLL_CLICK, CAMPAIGN_PROGRAMATICA_CLARO_PREROLL_CLICK, VIDEO_CAMPAIGN_PROGRAMATICA, BETANO_PREZAO, SMART_CLARO_APP, ADFORCE_DISPLAY_BANNER, VIDEO_VPAID_SPACE_WICO, SMARTAD_SPTRANS, SMARTAD_AER_BSB, SMARTAD_AER_VCP, IMA, IMA_PROGRAMATICA, CAMPAIGN_PROGRAMATICA, NOAD_BANNER_PROGRAMATICA, VAST420, SPTRANS_BANNER, PIXEL_TRACKING_SERASA, UNDEF`

### ContentType (7)
`BANNER_PRE_ROLL_MOBILE(BPRM), BANNER_CAMPAIGN_MOBILE(BCM), BACKGROUND_MOBILE(BGM), BANNER_PRE_ROLL_TABLET_DESKTOP(BPRTD), BANNER_CAMPAIGN_TABLET_DESKTOP(BCTD), BACKGROUND_TABLET_DESKTOP(BGTD), VIDEO(VID)`

### TrackerType (2)
`PIXEL` (tracker `<img>`), `SCRIPT` (tracker `<script>`)

## 5. Padrões transversais a portar

| Padrão Java | Equivalente Go |
|---|---|
| `@Cacheable` Caffeine (5min/500) | cache TTL em memória de container (`internal/cache`) |
| `@Async` ThreadPool (core 2, max 4, queue 100, prefixo `PostbackLog-`) | SQS + Lambda consumer (goroutine não sobrevive ao freeze) |
| `@Retryable` backoff exponencial | retry com `backoff` no SDK AWS / wrapper próprio |
| `@Transactional` | `database/sql` Tx explícita |
| Null Object (NullCampaign/NullCreative) | structs zero-value com método `IsNull()` ou ponteiro nil documentado |
| ThreadLocal DateFormat | `time.Parse`/`Format` (já thread-safe) |
| Random uniforme `nextInt(n)` | `math/rand/v2.IntN(n)` |
