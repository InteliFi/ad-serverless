# Matriz de Paridade de Features — ad-server/ad-commons → ad-serverless

> Cada comportamento do sistema legado mapeado para o serviço Go que o implementa e a issue que o rastreia.
> **Regra:** nenhuma linha pode ficar sem destino. Status: ⬜ pendente · 🟨 em desenvolvimento · ✅ portado com golden test · 🚫 descontinuado (com justificativa).

## Endpoints HTTP

| # | Feature legada | Spec | Serviço Go | Epic | Status |
|---|---|---|---|---|---|
| E01 | GET/HEAD `/`,`/health`,`/healthz` (JSON estático version/UP) | [01](legado/01-endpoints-http.md)§1 | ad-handler | M4 | ⬜ |
| E02 | GET `/ad` — script de anúncio (hid com pipe, retry, 404s) | [01](legado/01-endpoints-http.md)§2 | ad-handler | M4 | ⬜ |
| E03 | GET `/GAM` — HTML via CloudFront d26ykw0gs9fv5u | [01](legado/01-endpoints-http.md)§3 | ad-handler | M4 | ⬜ |
| E04 | GET `/vast` — pipeline completo 3 fluxos | [03](legado/03-pipeline-vast.md) | vast-handler | M5 | ⬜ |
| E05 | POST `/adtrack` (+ variante failureInfo) | [01](legado/01-endpoints-http.md)§5 | track-handler | M3 | ⬜ |
| E06 | GET `/adtrack` — relatório JSON agregado | [01](legado/01-endpoints-http.md)§6 | report-handler | M6 | ⬜ |
| E07 | GET `/adtrack/xls` — relatório Excel | [01](legado/01-endpoints-http.md)§7 | report-handler | M6 | ⬜ |
| E08 | GET `/adtrack/postback` — afiliados + upstreams modatta/prezao | [01](legado/01-endpoints-http.md)§8 | postback-handler | M3 | ⬜ |
| E09 | GET `/vasttrack` — milestones de vídeo | [01](legado/01-endpoints-http.md)§9 | track-handler | M3 | ⬜ |
| E10 | GET `/trackingpixel` — pixel com download + evento | [01](legado/01-endpoints-http.md)§10 | track-handler | M3 | ⬜ |
| E11 | GET `/redirect` — HTML+JS com cookie 15min, GA, placeholders | [01](legado/01-endpoints-http.md)§11 | redirect-handler | M3 | ⬜ |
| E12 | GET/OPTIONS `/proxy-tracker` — proxy genérico + VAST error log | [01](legado/01-endpoints-http.md)§12 | proxy-handler | M5 | ⬜ |
| E13 | GET `/proxy-audit` — proxy JS com 7 famílias de rewrite | [03](legado/03-pipeline-vast.md)§6.2 | proxy-handler | M5 | ⬜ |
| E14 | GET `/safeframe/proxy-safeframe` + OPTIONS | [01](legado/01-endpoints-http.md)§14 | proxy-handler | M5 | ⬜ |
| E15 | GET `/media/{filename}` — vídeo mp4 cacheado | [01](legado/01-endpoints-http.md)§15 | media-handler | M5 | ⬜ |
| E16 | `/error/400` handler | [01](legado/01-endpoints-http.md)§16 | API GW responses | M2 | ⬜ |

## Middleware / Filtros globais

| # | Feature | Spec | Destino | Epic | Status |
|---|---|---|---|---|---|
| F01 | CorsFilter (reflexão de origin + credentials) | [01](legado/01-endpoints-http.md)§16 | internal/middleware/cors | M1 | ⬜ |
| F02 | RequestValidationFilter (anti-injection, bypasses proxy-tracker, heurística base64) | [01](legado/01-endpoints-http.md)§16 | internal/middleware/validation | M1 | ⬜ |
| F03 | Chars relaxados em path/query (Tomcat relaxed) | [05](legado/05-config-infra-deploy.md)§3 | validação com URLs reais no API GW | M8 | ⬜ |

## Lógica de negócio

| # | Feature | Spec | Destino | Epic | Status |
|---|---|---|---|---|---|
| N01 | Seleção de campanha: enabled + frequency cap + random uniforme | [02](legado/02-logica-negocio.md)§1.1 | internal/selection | M1 | ⬜ |
| N02 | Seleção de creative: random uniforme + NullCreative | [02](legado/02-logica-negocio.md)§1.1 | internal/selection | M1 | ⬜ |
| N03 | FrequencyCap: parser `;`/`>`/`>>` + matching hora/dia | [02](legado/02-logica-negocio.md)§1.3 | internal/frequencycap | M1 | ⬜ |
| N04 | TemplateDecider: CreativeType → template (40+) | [02](legado/02-logica-negocio.md)§1.8 | internal/templates | M4 | ⬜ |
| N05 | Engine `${key}` + cache de template | [02](legado/02-logica-negocio.md)§1.8 | internal/templates | M1 | ⬜ |
| N06 | CreativeTemplateRequest: 30+ placeholders, tracking_redirect_url base64 | [02](legado/02-logica-negocio.md)§1.8 | internal/templates | M1 | ⬜ |
| N07 | Tracking duplo MySQL+DynamoDB com SK `ISO8601#rdsId` | [02](legado/02-logica-negocio.md)§1.4–1.5 | tracker-writer | M3 | ⬜ |
| N08 | Validação postback: event type + campanha ativa + exceções 401/404/422 | [02](legado/02-logica-negocio.md)§1.4 | postback-handler | M3 | ⬜ |
| N09 | PostbackLog DynamoDB (transaction_id fallback, payout parse seguro) | [02](legado/02-logica-negocio.md)§1.9 | postback-handler | M3 | ⬜ |
| N10 | Postbacks upstream por source (modatta base64, prezao_claro) | [01](legado/01-endpoints-http.md)§8 | postback-handler | M3 | ⬜ |
| N11 | PostbackSignature MD5 (existente, validação desativada) | [04](legado/04-modelo-dados.md)§5 | internal/tracking (port + flag) | M1 | ⬜ |
| N12 | VideoCache: whitelist, bypass assinadas, MD5, ip/0.0.0.0 | [02](legado/02-logica-negocio.md)§1.10 | internal/vast/videocache → S3 | M5 | ⬜ |
| N13 | CampaignVastOverride: cache 5min + validação datas | [02](legado/02-logica-negocio.md)§1.11 | vast-handler | M5 | ⬜ |
| N14 | Relatório agregado (12 contadores, batch 1000) | [02](legado/02-logica-negocio.md)§1.6 | report-handler (GROUP BY) | M6 | ⬜ |
| N15 | Export XLS | [02](legado/02-logica-negocio.md)§1.6 | report-handler (excelize) | M6 | ⬜ |
| N16 | Pixel: download de URL + evento TRACKING_PIXEL hotspot null | [02](legado/02-logica-negocio.md)§1.7 | track-handler | M3 | ⬜ |
| N17 | Caches Caffeine hotspots/override (5min/500) | [02](legado/02-logica-negocio.md)§5 | internal/cache | M1 | ⬜ |
| N18 | Retries: DynamoDB 3× backoff, DB locks 3× | [02](legado/02-logica-negocio.md)§5 | SDK retryer + wrapper | M1 | ⬜ |

## Pipeline VAST (detalhe — o mais crítico)

| # | Feature | Spec | Epic | Status |
|---|---|---|---|---|
| V01 | Fluxo A: Campaign Direct VAST 4.2 (click URL direta) | [03](legado/03-pipeline-vast.md)§2 | M5 | ⬜ |
| V02 | Fluxo B: 12+ hotspots hardcoded (SESTSENAT, CLARO_*, OPOVO, TVCULTURA, TV_COINS, INTELIFI_TEST) | [03](legado/03-pipeline-vast.md)§2 | M5 | ⬜ |
| V03 | Fluxo C: fetch dinâmico com macros + params t/h + headers upstream | [03](legado/03-pipeline-vast.md)§2 | M5 | ⬜ |
| V04 | Rewrite 8 categorias de tag (CDATA+plain) p/ proxy-tracker | [03](legado/03-pipeline-vast.md)§3 | M5 | ⬜ |
| V05 | Bypass Google DoubleClick (media+JS+AdParameters intocados) | [03](legado/03-pipeline-vast.md)§3.1 | M5 | ⬜ |
| V06 | Bypass Space 00px.net / AdForce adftech.com.br / vast-logger-js | [03](legado/03-pipeline-vast.md)§3.1 | M5 | ⬜ |
| V07 | AdParameters: bare-domain/base64/media-asset | [03](legado/03-pipeline-vast.md)§3.2 | M5 | ⬜ |
| V08 | Impression+Viewable → Tracking start (extração AdForce viewable_impression) | [03](legado/03-pipeline-vast.md)§3.3 | M5 | ⬜ |
| V09 | Cache de vídeo mp4-only → /media (S3) | [03](legado/03-pipeline-vast.md)§4 | M5 | ⬜ |
| V10 | GDPR macros + consent passthrough | [03](legado/03-pipeline-vast.md)§2 | M5 | ⬜ |
| V11 | Reescritas JS proxy-audit (7 famílias: ADXSPACE, Space, VPAID, Metrike, CDN rewrite, Admotion, loadPixel) | [03](legado/03-pipeline-vast.md)§6.2 | M5 | ⬜ |
| V12 | VAST error tracking no proxy-tracker (`/errors?`, `&error=`) | [01](legado/01-endpoints-http.md)§12 | M5 | ⬜ |

## Dados e operação

| # | Feature | Spec | Destino | Epic | Status |
|---|---|---|---|---|---|
| D01 | Leitura MySQL schema atual (sem mudanças) | [04](legado/04-modelo-dados.md) | RDS Proxy + database/sql | M2 | ⬜ |
| D02 | DynamoDB AdTrackers (chaves/formato preservados) | [04](legado/04-modelo-dados.md)§4.1 | repository/dynamo | M3 | ⬜ |
| D03 | DynamoDB PostbackLogs (chaves/formato preservados) | [04](legado/04-modelo-dados.md)§4.2 | repository/dynamo | M3 | ⬜ |
| D04 | Timezone America/Sao_Paulo em todas as datas | [05](legado/05-config-infra-deploy.md)§3 | time.LoadLocation explícito | M1 | ⬜ |
| D05 | Formato `yyyyMMddHHmmssSSS` do param time | [04](legado/04-modelo-dados.md)§5 | parser custom 17 dígitos | M1 | ⬜ |
| D06 | Segredos: eliminação de hardcode + rotação de chaves | [05](legado/05-config-infra-deploy.md)§2 | IAM Roles + SSM | M0 | ⬜ |
| D07 | Health check p/ balanceador (sem DB) | [01](legado/01-endpoints-http.md)§1 | ad-handler | M4 | ⬜ |
| D08 | Keep-alive de conexões MySQL (jobs @Scheduled) | [05](legado/05-config-infra-deploy.md)§3 | 🚫 obsoleto — RDS Proxy assume | — | 🚫 |
| D09 | DynamoDbMigrationRunner (V*.json) | [02](legado/02-logica-negocio.md)§2 | 🚫 obsoleto — recursos no serverless.yml | — | 🚫 |
| D10 | Flyway migrations nas Lambdas | [05](legado/05-config-infra-deploy.md)§3 | 🚫 desligado na fase 1 (M10 decide) | M10 | ⬜ |

## Como atualizar esta matriz

Ao fechar uma issue de portagem: marcar a(s) linha(s) correspondente(s) como ✅ no PR, com link para o golden test. A revisão de PR DEVE verificar a atualização da matriz.
