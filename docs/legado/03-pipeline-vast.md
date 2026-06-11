# Sistema Legado — Pipeline VAST Completo (Especificação Funcional)

> **Fonte:** análise do `VastService.java` (~1279 linhas), `ProxyAuditService`, `ProxyTrackerService`, `SafeFrameService`, `VideoCacheService` e templates do [ad-server](https://github.com/InteliFi/ad-server).
> Este é o componente **mais crítico** da migração (~40% do tráfego). A reimplementação em Go deve reproduzir cada regra abaixo. Recomenda-se dividir em 3–4 módulos Go (fetch, rewrite, partner-rules, video-cache).

## 1. Entrada — `GET /vast`

| Param | Tipo | Default | Descrição |
|---|---|---|---|
| `cid` | int? | — | Campaign ID (ativa fluxo de Campaign Override) |
| `hid` | string | `""` | Hotspot ID (ativa fluxo hardcoded por hotspot) |
| `vcurl` | string? | — | URL VAST externa codificada em **base64** |
| `gdpr` | string | `"0"` | Flag GDPR |
| `gdpr_consent` | string | `""` | TCF 2.0 consent string |
| `refOrigin` | string | `https://ads.inteli.fi` | Origem do player; validar protocolo http/https (inválido → `400 "Invalid refOrigin"`) |

## 2. Decisão de fluxo (ordem exata)

```
1. Validar/normalizar refOrigin (default https://ads.inteli.fi)
2. Se cid != null:
     override = campaignVastOverrideService.findValidOverride(cid)   // cache 5min
     // valida enabled=true E hoje ∈ [start_date, end_date]
     se inválido → 404
     se override válido E vcurl vazio → FLUXO A (Campaign Direct VAST)
3. Se hid não vazio → FLUXO B (hotspots hardcoded — switch/case)
4. Se vcurl não vazio → FLUXO C (fetch dinâmico de VAST externo)
5. Nenhum → 404
```

### FLUXO A — Campaign Direct VAST (VAST 4.2 gerado localmente)

Query no banco: `campaigns c LEFT JOIN creatives cr` validando `c.enabled=true` e datas; retorna `name`, `url_click`, `impression_tracker`, `url_portrait`. Gera:

```xml
<VAST version="4.2">
  <Ad id="{cid}"><InLine>
    <Impression id="Impression-ID"><![CDATA[ {proxy-tracker(impression_tracker)} ]]></Impression>
    <Impression id="Impression-PV"><![CDATA[ /vasttrack?cid={cid}&et=PAGE_VIEW&hid={hid}&time={ts} ]]></Impression>
    <Creatives><Creative id="{cid}"><Linear>
      <TrackingEvents>
        <Tracking event="start"><![CDATA[ /vasttrack?et=VIDEO_STARTED... ]]></Tracking>
        <Tracking event="firstQuartile"><![CDATA[ /vasttrack?et=25_PER_PLAYED... ]]></Tracking>
        <Tracking event="midpoint"><![CDATA[ /vasttrack?et=50_PER_PLAYED... ]]></Tracking>
        <Tracking event="thirdQuartile"><![CDATA[ /vasttrack?et=75_PER_PLAYED... ]]></Tracking>
        <Tracking event="complete"><![CDATA[ /vasttrack?et=VIDEO_ENDED... ]]></Tracking>
      </TrackingEvents>
      <MediaFiles><MediaFile type="video/mp4"><![CDATA[ {url_portrait} ]]></MediaFile></MediaFiles>
      <VideoClicks><ClickThrough><![CDATA[ {url_click} ]]></ClickThrough></VideoClicks>
    </Linear></Creative></Creatives>
  </InLine></Ad>
</VAST>
```

Regra recente (commit `74748d2`): manter a **URL de clique direta da campanha** (não proxiar ClickThrough do campaign direct) e alinhar o fluxo de start tracking do AdForce.

### FLUXO B — Hotspots hardcoded (switch por `hid`)

| Hotspot | Tipo | Comportamento |
|---|---|---|
| `SESTSENAT_1`, `SESTSENAT_2` | Inline | VAST 4.2 fixo com vídeo/tracking Sest Senat |
| `CLARO_WIFI` | Misto | Random entre 4 opções: 3 hardcoded (inclui VPAID 00px.net) + 1 default SmartAdServer |
| `CLARO_RECOMPENSAS` | URL | Fetch SmartAdServer `videoapi.smartadserver.com/ac?siteid=596893...` |
| `CLARO_PREZAOFREE` | URL | SmartAdServer com pgname=CLARO_PREZAOFREE |
| `TV_COINS_TESTE` | Inline | VAST 4.2 fixo (vídeo Santander) |
| `OPOVO_PREROLL` / `OPOVO_MIDROLL` | URL | SmartAdServer pgname=TVCOINS_OPOVO |
| `TVCULTURA_PREROLL` / `TVCULTURA_MIDROLL` | URL | SmartAdServer |
| `INTELIFI_TEST` | URL | Metrike `servedby.metrike.com.br/vast.spark?setID=61348` |
| (outro) | — | `404` |

> Na migração, esses hardcodes devem virar **configuração em dados** (tabela/SSM/DynamoDB), mantendo o comportamento byte-a-byte na Fase 1.

### FLUXO C — Fetch dinâmico

1. Decodifica `vcurl` de base64 (variante segura p/ URL).
2. **Expansão de macros** na URL:
   - `${GDPR}` → param `gdpr`
   - `${GDPR_CONSENT_755}` → param `gdpr_consent`
   - `%%CACHEBUSTER%%` → `System.currentTimeMillis()`
   - `%%REFERRER_URL_ESC%%` → refOrigin URL-encoded
   - `%%REFERRER_URL%%` → refOrigin
3. **Params garantidos** (add ou replace): `t={cachebuster}`, `h={refOrigin URL-encoded}`.
4. Fetch HTTP GET com headers upstream:
   - `X-Forwarded-For`, `X-Real-IP` = IP real do cliente
   - `X-Forwarded-Proto/Host/Port`, `Forwarded: for=IP;proto=...;host=...`
   - `Origin: {refOrigin}`, `Referer: {refOrigin}/`
   - Headers de geolocalização repassados (CF-IPCountry etc.)
5. Resposta vazia → `500 "Empty VAST response"`; sem tag `<VAST>` → WARN; `ResourceAccessException` → `504`.

## 3. Rewrite do XML (pós-processamento) ⚠️ CORAÇÃO DO SISTEMA

Para **cada categoria de tag**, decide bypass × proxy e reescreve via regex (CDATA e não-CDATA):

| Tag | Reescrita | Observação |
|---|---|---|
| `<Tracking>` | `https://ads.inteli.fi/proxy-tracker?u={base64(url)}&refOrigin={enc}` | bypass Space/AdForce |
| `<ClickTracking>` | idem | bypass Space/AdForce |
| `<Impression>` | idem + rewrite de placeholders GDPR antes | bypass `vast-logger-js-*` e URLs locais inteli.fi |
| `<ViewableImpression>` (Viewable/NotViewable/ViewUndetermined) | idem | bypass Space |
| `<Error>` | idem | bypass Space |
| `<MediaFile type="video/mp4">` | cache local → `https://ads.inteli.fi/media/{md5}.mp4` | NUNCA para Google DoubleClick; falha de cache mantém original; substitui `ip/0.0.0.0` → IP real |
| `<JavaScriptResource>` | `https://ads.inteli.fi/proxy-audit?src={base64(url)}` | NUNCA para Google DoubleClick |
| `<AdParameters>` | URLs internas reescritas p/ proxy-tracker | regras especiais §3.2 |

**Regexes de referência (copiar semântica):**
- Tracking CDATA: `(<Tracking[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)`
- Tracking plain: `(<Tracking[^>]*>)([^<\[][^<]*)</Tracking>` → embrulha em CDATA + proxy
- MediaFile CDATA: `(<MediaFile([^>]*)>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)`
- AdParameters CDATA: `(<AdParameters[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)`

### 3.1 Regras de BYPASS (não proxiar) — condições literais

```java
// Google DoubleClick (VAST inteiro marcado como "google"):
host.endsWith("doubleclick.net") || host.endsWith("googlesyndication.com") || host.endsWith("googletagservices.com")
// Efeito: MediaFile, JavaScriptResource e AdParameters NÃO são reescritos (URLs assinadas do Google expiram se modificadas)

// Impression bypass:
host.startsWith("vast-logger-js-")           // e URLs locais inteli.fi

// Space/00px (VPAID) bypass:
host.endsWith("00px.net")

// AdForce bypass:
host.endsWith("adftech.com.br")
```

### 3.2 AdParameters — reescrita de URLs em JSON/config

```
se shouldBypass(url)        → mantém
se isMediaAssetUrl(url)     → mantém (.mp4, .webm, ...)
se isBareDomain(url)        → proxyTracker + url + refOriginSuffix      (SEM base64)
senão                       → proxyTracker + base64(url) + refOriginSuffix
```

### 3.3 Impression → Tracking event="start" (regra AdForce/viewability)

1. Coleta URLs de `<Impression>` e `<Viewable>`; extrai `viewable_impression` do JSON de AdParameters (AdForce: `{"$":{"event":"viewable_impression"},"_":"https://..."}`).
2. **Remove** os blocos `<Impression>` e `<ViewableImpression>` do XML.
3. **Injeta** as URLs como `<Tracking event="start">` antes de `</TrackingEvents>`.

**Resultado:** a impressão dispara no *start* do vídeo, não no load do XML.

## 4. Cache de Vídeo (VideoCacheService)

- Diretório: `/tmp/adserver_video_cache` (dev) / `/tmp/adserver_video_cache_prod` (prod).
- Whitelist: `gcdn.2mdn.net`, `googlevideo.com`.
- Bypass URLs assinadas: `gcdn.2mdn.net` + `/videoplayback/`; `googlevideo.com` + `/manifest/`.
- Nome do arquivo: `MD5(url).mp4`; mapa em memória `url → filename`.
- Download com headers do cliente + `Referer: https://ads.inteli.fi/` + `X-Forwarded-For` real; placeholder `ip/0.0.0.0` substituído.
- Servido por `GET /media/{filename}` como `video/mp4`.
- **Migração → S3 + CloudFront** (Lambda não tem disco compartilhado entre containers).
- Regra recente (commit `52936fa`): **pular cache para mídia não-mp4**.

## 5. Cabeçalhos e CORS da resposta `/vast`

- `Content-Type: text/xml`.
- CORS do CorsFilter global (reflete Origin, credentials).

## 6. Serviços de Proxy

### 6.1 `/proxy-tracker` — ver [01-endpoints-http.md](01-endpoints-http.md) §12

### 6.2 `/proxy-audit` — reescritas de JavaScript (literais)

Pipeline: decode base64 (`src`) → whitelist de hosts → fetch (UA `InteliFi-ProxyAudit/1.0`, 10s/30s, máx 2MB) → **cirurgia de regex** → resposta `text/javascript` com `Cache-Control: public, max-age=3600`.

Substituições aplicadas (todas forçam o refOrigin configurado):

```javascript
// a) ADXSPACE pageUrl:
encodeURIComponent(ADXSPACE.pageUrl)        → encodeURIComponent("{refOrigin}")

// b) Space/00px:
this.origin = o()                            → this.origin = "{refOrigin}"
this.referrer = n(this.macro, "&pn")        → this.referrer = "{refOrigin}"

// c) Space VPAID:
function getReferrer() {...}                 → function getReferrer(){return "{refOrigin}"}
function getOrigins() {...}                  → function getOrigins(){return "{refOrigin}"}

// d) Metrike/AdButler:
app.sourceURL = app.getReferrer()            → app.sourceURL = "{refOrigin}"
return referrer;                             → return "{refOrigin}";

// e) Rewrite de CDNs para passar pelo próprio proxy (base64 fixo):
https://servedby.metrike.com.br/app.js       → https://ads.inteli.fi/proxy-audit?src=aHR0cHM6Ly9zZXJ2ZWRieS5tZXRyaWtlLmNvbS5ici9hcHAuanM=
https://sdk.adftech.com.br/sdk.js            → proxied (base64)
https://sdk.adftech.com.br/sdk-standard-extension.js → proxied (base64)

// f) Admotion Digital:
window.top.location.href                     → "{refOrigin}"
window.parent.location.href                  → "{refOrigin}"

// g) loadPixel() — injeta proxy no início da função:
o = "https://ads.inteli.fi/proxy-tracker?u=" + btoa(o) + "&refOrigin={enc}"
```

Regra recente (commit `c26eba4`): **proxiar apenas o JS VPAID do AdForce** via proxy-audit; eventos de tracker AdForce têm bypass.
Regra recente (commit `1f8ce34`): **bypass total de media e JS proxy para VAST do Google DoubleClick**.

### 6.3 `/safeframe/proxy-safeframe` — ver [01-endpoints-http.md](01-endpoints-http.md) §14

## 7. VAST Tracking — `GET /vasttrack`

Milestones de vídeo mapeados nos templates/VAST gerado:

| Evento VAST | EventType persistido |
|---|---|
| (load da página) | `PAGE_VIEW` |
| `start` | `VIDEO_STARTED` |
| `firstQuartile` | `25_PER_PLAYED` |
| `midpoint` | `50_PER_PLAYED` |
| `thirdQuartile` | `75_PER_PLAYED` |
| `complete` | `VIDEO_ENDED` |

Persistência: MySQL + DynamoDB async (idêntico a `/adtrack`).

## 8. Templates relevantes para VAST

- `vast42.vm` — VAST 4.2 genérico (`${cid}`, `${hid}`, `${ad_tracker_timestamp}`, `${url_portrait}`, `${tracking_redirect_url}`).
- `programaticaVAST.vm` — JS com objeto `Tracker` (monta `/adtrack?cid=&et=&hid=&time=`), integração Google IMA SDK, loadPixel/printPixel.
- `space/vpaid/videoVpaidSpace.vm` — VPAID Space/00px.
- `bannerAd.vm`, `videoAd.vm` — inline básicos.

## 9. Caches do pipeline

| O quê | Onde | TTL | Chave |
|---|---|---|---|
| Campaign VAST Override | Caffeine `campaignVastOverride` | 5 min / 500 | `campaignId` |
| Hotspots | Caffeine `hotspots` | 5 min / 500 | `code` |
| Vídeo | disco + map em memória | indefinido | `MD5(url)` |
| VAST fetched | **não há** | — | — |

## 10. Checklist de paridade do vast-handler Go

- [ ] 3 fluxos (A/B/C) na ordem exata de decisão
- [ ] Expansão de macros e params garantidos `t`/`h`
- [ ] Headers upstream completos (IP real, geo, Origin/Referer)
- [ ] Rewrite de 8 categorias de tag (CDATA + plain)
- [ ] 4 regras de bypass (Google, vast-logger-js, 00px, adftech)
- [ ] AdParameters: bare-domain sem base64 / URL completa com base64 / media asset intocado
- [ ] Impression+Viewable → Tracking start (com extração de viewable_impression do AdForce)
- [ ] Cache de vídeo S3 com whitelist + bypass de URLs assinadas + substituição `ip/0.0.0.0`
- [ ] Campaign Direct VAST com click URL direta (não proxiada)
- [ ] Tratamento de erros: 400/404/500/502/504 idênticos
