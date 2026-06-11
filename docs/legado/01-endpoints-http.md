# Sistema Legado — Inventário Completo de Endpoints HTTP

> **Fonte:** análise exaustiva do pacote `presentation/` do [ad-server](https://github.com/InteliFi/ad-server) (branch `develop`, commit `74748d2`, versão 0.25.0).
> Este documento é a **especificação funcional de referência** para a reimplementação em Go. Nenhum comportamento aqui descrito pode ser perdido na migração.

## Visão Geral

| # | Endpoint | Método | Classe Java | Lambda Go alvo |
|---|----------|--------|-------------|----------------|
| 1 | `/`, `/health`, `/healthz` | GET/HEAD | `ApplicationStatusService` | API Gateway (mock) / `ad-handler` |
| 2 | `/ad` | GET | `AdService` | `ad-handler` |
| 3 | `/GAM` | GET | `GamService` | `ad-handler` |
| 4 | `/vast` | GET | `VastService` | `vast-handler` |
| 5 | `/adtrack` | POST | `AdTrackService` | `track-handler` |
| 6 | `/adtrack` | GET | `AdTrackService` | `report-handler` |
| 7 | `/adtrack/xls` | GET | `AdTrackService` | `report-handler` |
| 8 | `/adtrack/postback` | GET | `AdTrackService` | `postback-handler` |
| 9 | `/vasttrack` | GET | `VastTrackService` | `track-handler` |
| 10 | `/trackingpixel` | GET | `TrackingPixelService` | `track-handler` |
| 11 | `/redirect` | GET | `RedirectService` | `redirect-handler` |
| 12 | `/proxy-tracker` | GET/OPTIONS | `ProxyTrackerService` | `proxy-handler` |
| 13 | `/proxy-audit` | GET | `ProxyAuditController/Service` | `proxy-handler` |
| 14 | `/safeframe/proxy-safeframe` | GET/OPTIONS | `SafeFrameService` | `proxy-handler` |
| 15 | `/media/{filename}` | GET | `MediaController` | `media-handler` (S3/CloudFront) |
| 16 | `/error/400` | * | `ErrorController` | API Gateway responses |

Filtros globais (aplicam a TODAS as requisições): `CorsFilter` e `RequestValidationFilter` — migram para middleware Go compartilhado.

---

## 1. Health Check — `GET|HEAD /`, `/health`, `/healthz`

- **Resposta:** `200 OK`, `application/json`, corpo `{"version":"<api.version>","status":"UP"}`.
- **Não toca o banco** — resposta estática (usada por load balancer / Docker healthcheck).
- Propriedade `api.version` da configuração (fallback `"unknown"`).

## 2. Ad Script — `GET /ad`

- **Query params:**
  - `hid` (opcional, default `""`) — código do hotspot. Suporta múltiplos valores separados por `|`; usa apenas a **primeira** ocorrência.
  - `red` (opcional) — URL de redirect repassada ao template.
- **Resposta 200:** `text/javascript` com headers `Content-Description: File Transfer` e `Content-Disposition: attachment; filename="adscript.js"`.
- **Resposta 404:** `hid` vazio, hotspot inexistente ou script renderizado vazio.
- **Lógica:**
  1. Normaliza `hid` para UPPER CASE e busca hotspot (cache 5 min).
  2. Seleciona campanha elegível (enabled + frequency cap hora/dia) **aleatoriamente** (uniforme).
  3. Seleciona creative da campanha **aleatoriamente**.
  4. Decide template pelo `CreativeType` (40+ templates) e renderiza com substituição `${key}`.
- **Resiliência:** retry em `CannotAcquireLockException` — 3 tentativas, backoff 1s ×2 (1s, 2s, 4s). Transação READ_COMMITTED, timeout 30s.

## 3. GAM HTML — `GET /GAM`

- **Query params:** `hid` (obrigatório — nome do arquivo HTML), `red` (opcional).
- **Lógica:** fetch HTTP de `https://d26ykw0gs9fv5u.cloudfront.net/public/gam/{hid}.html` e devolve o HTML como `text/html`.
- **Erros:** `hid` vazio, falha de fetch ou resposta vazia → `404`. Sem timeout específico configurado (usa default).

## 4. VAST XML — `GET /vast`

Endpoint mais crítico (~40% do tráfego). Especificação completa em [03-pipeline-vast.md](03-pipeline-vast.md).

- **Query params:** `cid` (opcional, int), `hid` (opcional, default `""`), `vcurl` (opcional, URL base64), `gdpr` (default `"0"`), `gdpr_consent` (default `""`), `refOrigin` (default `https://ads.inteli.fi`).
- **Resposta:** `text/xml` VAST 4.2 | `404` (campanha/hotspot inválido) | `400` (refOrigin inválido) | `500/502/504` (upstream).

## 5. Registro de Evento — `POST /adtrack`

- **Query params (todos obrigatórios):** `cid` (int), `et` (event type string), `hid` (string), `time` (timestamp no formato `yyyyMMddHHmmssSSS` — `DateUtils.DATE_FORMAT`).
- **Variante:** se o param `failureInfo` está presente, o body JSON (Map) é logado como `Tracking failure: {...}` e o fluxo normal continua.
- **Lógica:** cria `AdTracker{campaign, hotspot, eventTypeId, eventDate, creationDate}` e persiste:
  1. **MySQL** `ad_trackers` (síncrono);
  2. **DynamoDB** `AdTrackers` (assíncrono, fire-and-forget, com retry 3× backoff exponencial).
- **Resposta:** `201 Created` + header `Location: /adtrack/{id}`. ParseException/erro → `500`.

## 6. Relatório JSON — `GET /adtrack`

- Gera relatório agregado **de toda a tabela** `ad_trackers` em batches de 1000 (paginação para evitar OOM).
- Agrupa por campanha → data do evento → hotspot; conta 12+ tipos de evento (PAGE_VIEW, IMPRESSION_*, CLICK_*, VIDEO_*, REDIRECT, TRACKING_PIXEL).
- **Resposta:** JSON `AdTrackerReport{items:[AdTrackerReportItem]}`.
- ⚠️ Com ~14M de linhas, esse endpoint é pesado — na migração, mover para consulta agregada (ver Epic de Relatórios).

## 7. Relatório Excel — `GET /adtrack/xls`

- Mesmo relatório do item 6, exportado como Excel (formato HSSF legado via Apache POI 3.15, arquivo temp `.xlsx`).
- Header: `EventDate, CampaignId, PAGE_VIEW, IMPRESSION_PRE_ROLL, CLICK_PRE_ROLL, IMPRESSION_CAMPAIGN, CLICK_CAMPAIGN, VIDEO_STARTED, PLAYED_25_PER, PLAYED_50_PER, PLAYED_75_PER, VIDEO_END, REDIRECT`.
- Em Go: usar `excelize/v2` com streaming.

## 8. Postback de Afiliados — `GET /adtrack/postback`

- **Query params obrigatórios:** `cid` (int), `event`, `aff_sub`, `click_id`, `source`.
- **Opcionais:** `transaction_id`, `payout`, `currency`, `sale_amount`.
- **Lógica:**
  1. Timestamp do servidor em `America/Sao_Paulo`, truncado a segundos, normalizado para offset (ex.: `-03:00`).
  2. Valida event type contra enum `EventType.Values` (inválido → `AdException`).
  3. Valida campanha existe e está ativa (frequency cap) → senão `CampaignNotFoundException` → `404`.
  4. Persiste `AdTracker` com `hotspot = POSTBACK_HOTSPOT` (config `intv.ad.postback.hotspot.code`), `eventTypeId = "POSTBACK_" + event` — **somente MySQL** (sem réplica DynamoDB).
  5. Log assíncrono no DynamoDB `PostbackLogs` (`PostbackLogComponent.logPostback`, @Async).
  6. **Postbacks upstream por source** (RestTemplate, connect 10s / read 30s, falha apenas loga WARN):
     - `source == "bW9kYXR0YQ"` (base64 de "modatta") → `GET https://pb.modatta.org/external/affiliates/pb/9A061369-320A-4C1E-9451-E1BA9991E193?modid={click_id}`
     - `source == "prezao_claro"` → `GET https://api.prezaofree.com.br/event/postback?partnerId=425701220616215160&event=register&clickId={click_id}`
- **Respostas:** `202 Accepted` | `401` (NotAuthorized) | `404` (campanha) | `422` (erro genérico).
- Existe `PostbackSignature` (MD5 de `campaignId + event + key` em hex) implementado porém **validação comentada no código (TODO)** — considerar reativar na migração.

## 9. VAST Tracking — `GET /vasttrack`

- **Query params:** `cid` (obrigatório int), `et` (obrigatório), `hid` (opcional, default `""`), `time` (obrigatório, formato `yyyyMMddHHmmssSSS`).
- Mesma persistência do `POST /adtrack` (MySQL + DynamoDB async).
- Eventos típicos: `PAGE_VIEW`, `VIDEO_STARTED`, `25_PER_PLAYED`, `50_PER_PLAYED`, `75_PER_PLAYED`, `VIDEO_ENDED`.
- **Resposta:** `200 OK` corpo vazio | `500` (parse/erro, com log detalhado de cid/hid/et/time).

## 10. Tracking Pixel — `GET /trackingpixel`

- **Query param:** `cid` (obrigatório int; null → `400`).
- **Lógica:**
  1. Busca `tracking_pixels.url` por campanha (named query `findTrackingPixelByCampaignId`).
  2. **Baixa a imagem da URL** para arquivo temporário (`FileUtils.copyURLToFile`).
  3. Registra evento `TRACKING_PIXEL` com `hotspot = null` (MySQL + DynamoDB async).
  4. Devolve os bytes como `image/png` com `Content-Disposition: attachment; filename="tracking_pixel_{cid}.png"`.
- **Erros:** IO/Parse → `500`.

## 11. Redirect com Tracking — `GET /redirect`

- **Query params:** `url` (obrigatório; texto plano ou base64), `cid` (obrigatório int), `hid` (obrigatório), `enc` (opcional bool, default false — se true decodifica `url` de base64), `source`, `aff_sub`, `click_id` (opcionais — substituem placeholders `{source}`, `{aff_sub}`, `{click_id}` na URL com URL-encoding).
- **Validações:** URL válida via `new URL()`; rejeita caracteres de controle (0x00–0x1F, 0x7F); revalida após substituição de placeholders.
- **Resposta `200` `text/html`** com JavaScript que:
  1. Verifica cookie `intelifi-redirect` (TTL 15 min / 900s) para evitar tracking duplicado;
  2. Se cookie ausente: carrega Google Analytics, faz `POST /adtrack` com `et=REDIRECT_CAMPAIGN` via XHR e redireciona via `window.location` no callback;
  3. Se cookie presente: redireciona imediatamente;
  4. URL final escapada para JS (`\\`, `'`, `"`, `\n`, `\r`, `\t`).
- **Erros:** URL inválida → `400 "Invalid URL format"`; base64 malformado → `400 "Invalid URL encoding"`; erro inesperado → `500`.

## 12. Proxy Tracker — `GET|OPTIONS /proxy-tracker`

- **OPTIONS (preflight):** `204` com `Access-Control-Allow-Headers` (eco do request ou default), `Access-Control-Allow-Methods: GET, POST, OPTIONS`, `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires`, `Cross-Origin-Embedder-Policy: credentialless`.
- **GET params:** `u` (obrigatório — URL em base64 OU url-encoded; tenta base64 primeiro), `refOrigin` (opcional, default `https://ads.inteli.fi`).
- **Lógica:**
  1. Decodifica `u`; valida protocolo http/https (senão `400`).
  2. Headers upstream: copia `Accept`, `Accept-Encoding`, `Accept-Language` do cliente; `User-Agent` fixo (`Mozilla/5.0 ... Chrome/91...`); `X-Forwarded-For` = IP real do cliente (considera `X-Forwarded-For`, `CF-Connecting-IP`, `True-Client-IP`); headers de geolocalização capturados (`CF-IPCountry`, `CloudFront-Viewer-Country`, etc.); `Origin` = refOrigin; `Referer` = refOrigin + `/`.
  3. Propaga status, corpo e headers do upstream; remove headers CORS do upstream e `Vary`; adiciona CORS local (reflete origin); se URL contém `type=js`, força `Content-Type: application/javascript`.
  4. **Detecção de VAST Error:** URLs contendo `/errors?` ou `&error=` recebem log especial (código de erro extraído do param `error=`).
- **Erros:** decodificação falha / URL inválida / refOrigin inválido → `400`; falha upstream → `502 "Proxy error"`.

## 13. Proxy Audit (JS de verificação) — `GET /proxy-audit`

Especificação completa das reescritas de JS em [03-pipeline-vast.md](03-pipeline-vast.md) §6.2.

- **Params:** `src` (obrigatório, base64 — aceita variante URL-safe e padding ausente), `refOrigin` (opcional, default `https://ads.inteli.fi`).
- **Whitelist de hosts:** config `PROXY_AUDIT_ALLOWED_FETCH_HOSTS` (prod: `cdn.00px.net,cdn.vendor.com,admotion.digital,servedby.metrike.com.br,nsp.admotion.digital`). Vazia = permissivo. Suporta wildcard por sufixo.
- **Fetch upstream:** `User-Agent: InteliFi-ProxyAudit/1.0`, `Accept: application/javascript, text/javascript, */*`; connect timeout 10s, read 30s, resposta máx 2MB. Falha de fetch → devolve JS vazio com comentário de erro (nunca quebra o player).
- **Resposta:** `text/javascript; charset=utf-8` com `Cache-Control: public, max-age=3600` e `Vary: Accept-Encoding`. Erros de validação → `400` com comentário JS.

## 14. SafeFrame Proxy — `GET /safeframe/proxy-safeframe` (+ `OPTIONS /safeframe`)

- **Param:** `u` (base64 ou url-encoded). Rejeita explicitamente `file://`.
- **Lógica:** fetch da URL; reescreve **todas** as URLs do conteúdo (regex `https?://[^"'\s)]+`) para `https://ads.inteli.fi/proxy-tracker?u={base64}` — exceto URLs contendo `inteli.fi` ou que casem com o Origin do request; envolve o resultado em HTML SafeFrame (viewport 300x250 hardcoded, stub `window.$sf.ext.register()`, stub `AdSDK.withSize().build()`, meta UTF-8, init no `DOMContentLoaded`).
- **Resposta:** `text/html` com CORS refletindo origin + `Vary: Origin`; copia `Cache-Control`, `Expires`, `Last-Modified`, `Etag` do upstream.
- **Erros:** `400` (URL inválida) | `502` (upstream).

## 15. Media — `GET /media/{filename:.+}`

- Serve vídeos MP4 do cache local `${video.cache.directory}` (`/tmp/adserver_video_cache[_prod]`).
- Path normalizado (`normalize()`) contra path traversal.
- **Resposta:** `200` `video/mp4` | `404` (não existe/não legível) | `500`.
- **Na migração:** substituir por S3 + CloudFront (cache local de container não é viável em Lambda).

## 16. Filtros Globais (migram para middleware compartilhado em Go)

### CorsFilter
- Se `Origin` presente: reflete origem, `Access-Control-Allow-Credentials: true`, `Vary: Origin`. Senão: `*` e credentials false.
- Métodos: GET, POST, OPTIONS, PUT, DELETE, HEAD. Headers: Content-Type, Authorization, X-Requested-With, Accept, Origin. Max-Age 3600.
- OPTIONS → `200` imediato. Força UTF-8. Loga request completo em DEBUG.

### RequestValidationFilter
- **Métodos válidos:** GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH, TRACE — senão `400 "Invalid HTTP method"`.
- **Path:** rejeita encodados proibidos (`%7C %3C %3E %5C %5E %60 %7B %7D %00 %0D %0A %25 %09`); após decode, permite apenas `[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=\-]*`; rejeita não-imprimíveis; bloqueia padrões de ataque PHP (`php://`, `allow_url_include`, `data:`, `file:`, `glob:`), SQL injection (`UNION SELECT`, `INSERT INTO`, `DELETE FROM`...) e command injection (`; rm`, `&& bash`, pipes, backticks, `$(...)`, `/dev/tcp`...).
- **Query string:** mesmas validações, **com bypass para `/proxy-tracker`** (permite base64 bruto).
- **Params:** rejeita nomes perigosos (`cmd, command, exec, execute, run, script, shell, system, opt, mdb, mdc, sys`); valida valores contra os padrões de ataque (bypass para param `u` de proxy-tracker: apenas non-printable check); bloqueia combinações conhecidas (`opt=sys`, `cmd=___S_O_S_T_R_E_A_MAX___`, `mdb=sos`).
- **Heurística base64:** strings com len > 20 casando `^[A-Za-z0-9+/]*={0,2}$` são isentas dos padrões de ataque.
- `Content-Type` > 1000 chars → `400`. Header `X-Invalid-Method` presente → `400` (para testes).

---

## Notas de migração

1. CORS pode ser parcialmente delegado ao API Gateway HTTP API, mas o comportamento de *reflexão de origin com credentials* exige middleware na Lambda.
2. `RequestValidationFilter` vira pacote `internal/middleware/validation` reutilizado por todos os handlers.
3. `ThreadLocal DateFormat` → em Go, `time.Parse` com layout `20060102150405.000` derivado de `yyyyMMddHHmmssSSS` (atenção: sem separadores).
4. Tracking assíncrono (@Async) → goroutines não sobrevivem ao freeze da Lambda; usar **SQS** (fire-and-forget confiável) ou escrita síncrona antes do return.
5. Endpoints de relatório são candidatos a saírem do hot path (Lambda separada com timeout maior).
