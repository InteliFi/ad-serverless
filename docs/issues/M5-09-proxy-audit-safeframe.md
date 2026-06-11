---
title: "[M5-09] proxy-handler: /proxy-audit (rewrites JS) + /safeframe"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

O pipeline VAST (M5-02) reescreve toda `<JavaScriptResource>` para `https://ads.inteli.fi/proxy-audit?src={base64(url)}` e o `/safeframe/proxy-safeframe` é o destino de criativos SafeFrame (300x250) que precisam ser embrulhados e ter suas URLs proxiadas. Ambos completam a Lambda **proxy-handler** (512MB / 29s, I/O-bound — ver ARQUITETURA-ALVO §3), iniciada em M5-08, adicionando as rotas `GET /proxy-audit`, `GET /safeframe/proxy-safeframe` e `OPTIONS /safeframe`.

- `/proxy-audit` busca o JS de verificação/VPAID de terceiros (Space/00px, AdForce/adftech, Metrike/AdButler, Admotion) e faz **cirurgia de regex** para forçar o `refOrigin` configurado, escondendo a página real onde o anúncio roda e roteando pixels pelo `/proxy-tracker`. É o que impede que verificadores de viewability detectem o contexto real e quebrem a entrega.
- `/safeframe/proxy-safeframe` busca um criativo HTML, reescreve **todas** as URLs para `/proxy-tracker` (exceto inteli.fi/origin) e embrulha em um HTML SafeFrame com stubs `$sf.ext.register` e `AdSDK`.

No legado são `ProxyAuditController.java` + `ProxyAuditService.java` (`@RestController` em `/proxy-audit`) e `SafeFrameService.java` (`@RestController` em `/safeframe`). A lógica vai para `internal/proxy`.

⚠️ **Paridade byte-a-byte:** o JS de saída é consumido por players reais; qualquer divergência nos rewrites quebra tracking de parceiro (perda de receita). Por isso os golden tests com fixtures de JS reais por família são obrigatórios.

## Especificação detalhada

Portar de:
- `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/br/com/intv/adserver/presentation/service/ProxyAuditService.java`
- `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/br/com/intv/adserver/presentation/controller/ProxyAuditController.java`
- `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/br/com/intv/adserver/presentation/service/SafeFrameService.java`

### A) GET /proxy-audit

**Params:** `src` (obrigatório, base64) e `refOrigin` (opcional, default `https://ads.inteli.fi` — constante `DEFAULT_REF_ORIGIN`).

**A.1 — decode do `src` (`decodeBase64Url`):** aceita variante **URL-safe** (substitui `-`→`+` e `_`→`/`) e **padding opcional** (acrescenta `=` até o comprimento ser múltiplo de 4) e então decodifica base64 padrão como UTF-8. Falha → `IllegalArgumentException("URL Base64 inválida: " + src)`.

**A.2 — whitelist (`isAllowedFetchHost`):** faz parse da URL; protocolo deve ser http/https (senão `false`). Config `PROXY_AUDIT_ALLOWED_FETCH_HOSTS` (prod: `cdn.00px.net,cdn.vendor.com,admotion.digital,servedby.metrike.com.br,nsp.admotion.digital`). **Vazia/null = permissivo** (qualquer host). Caso contrário, split por `,`, trim, casa se `host == allowedHost` **ou** `host` termina com `"." + allowedHost` (wildcard por sufixo). Host não permitido → `IllegalArgumentException("Host não permitido para busca: " + url)`.

**A.3 — `resolveRefOrigin`:** vazio → default; senão parse de URL com protocolo http/https, inválido → `IllegalArgumentException("refOrigin precisa usar http ou https")`; parse falho → `IllegalArgumentException("refOrigin inválido: " + refOrigin)`.

**A.4 — fetch (`fetchUpstreamJs`):** `GET` com `User-Agent: InteliFi-ProxyAudit/1.0` e `Accept: application/javascript, text/javascript, */*`; connect timeout 10s (`PROXY_AUDIT_CONNECT_TIMEOUT=10000`), read 30s (`PROXY_AUDIT_READ_TIMEOUT=30000`), resposta máx 2MB (`PROXY_AUDIT_MAX_RESPONSE_SIZE=2097152`; corpo maior → erro). **Qualquer falha de fetch (rede/timeout/>2MB/vazio) NUNCA quebra o player:** retorna corpo `// proxy audit error - upstream fetch failed` e `contentType` nulo (que vira `text/javascript; charset=utf-8`). Preserva o `Content-Type` do upstream quando presente.

**A.5 — pipeline `processVerifierJs` (ordem EXATA):** decode → whitelist → resolveRefOrigin → fetch → `forceFixedPageUrlInH` (famílias a, b-origin/referrer, d, e, f abaixo) → `forceProxyTrackerOnLoadPixel` (família g; **bypass se for JS da Space**) → `forceSpaceOriginAndReferrer` (família c; **só se for JS da Space**) → se `decodedUrl` contém `admotion.digital`: `forceAdmotionProxy`.

`isSpaceJsContent` = conteúdo contém `00px.net` OU `space.runs` OU `space.ad(` OU `ADXSPACE`.

**As 7 famílias de rewrite (literais — copiar regex/strings exatas da §6.2 do doc 03 e do ProxyAuditService.java).** Em todas, `{refOrigin}` = `effectiveRefOrigin` e `{enc}` = url-encode dele:

```javascript
// a) ADXSPACE.pageUrl  (regex CASE_INSENSITIVE, tolerante a minificação):
//    encodeURIComponent\(\s*ADXSPACE\s*\.\s*pageUrl\s*\)
encodeURIComponent(ADXSPACE.pageUrl)          → encodeURIComponent("{refOrigin}")

// b) Space/00px origin e referrer:
//    this\s*\.\s*origin\s*=\s*o\s*\(\s*\)
this.origin = o()                              → this.origin="{refOrigin}"
//    this\s*\.\s*referrer\s*=\s*n\s*\(\s*this\s*\.\s*macro\s*,\s*"&pn"\s*\)
this.referrer = n(this.macro, "&pn")           → this.referrer="{refOrigin}"

// c) Space VPAID (só quando isSpaceJsContent):
//    function\s+getReferrer\s*\(\s*\)\s*\{[\s\S]*?\}   (CASE_INSENSITIVE)
function getReferrer() {...}                   → function getReferrer(){return "{enc}"}
//    function\s+getOrigins\s*\(\s*\)\s*\{[\s\S]*?\}    (CASE_INSENSITIVE)
function getOrigins() {...}                    → function getOrigins(){return "{refOrigin}"}

// d) Metrike/AdButler:
//    app\s*\.\s*sourceURL\s*=\s*app\s*\.\s*getReferrer\s*\(\s*\)\s*;
app.sourceURL = app.getReferrer();             → app.sourceURL="{refOrigin}";
//    return\s+referrer\s*;   (aplicado SOMENTE se o conteúdo contém "getReferrer")
return referrer;                               → return "{refOrigin}";

// e) Rewrite de CDNs para o próprio proxy (base64 FIXO — copiar literal):
https://servedby.metrike.com.br/app.js
  → https://ads.inteli.fi/proxy-audit?src=aHR0cHM6Ly9zZXJ2ZWRieS5tZXRyaWtlLmNvbS5ici9hcHAuanM=
https://sdk.adftech.com.br/sdk.js
  → https://ads.inteli.fi/proxy-audit?src=aHR0cHM6Ly9zZGsuYWRmdGVjaC5jb20uYnIvc2RrLmpz
https://sdk.adftech.com.br/sdk-standard-extension.js
  → https://ads.inteli.fi/proxy-audit?src=aHR0cHM6Ly9zZGsuYWRmdGVjaC5jb20uYnIvc2RrLXN0YW5kYXJkLWV4dGVuc2lvbi5qcw==

// f) Admotion Digital (em forceFixedPageUrlInH, sempre):
//    window\s*\.\s*top\s*\.\s*location\s*\.\s*href
window.top.location.href                       → "{refOrigin}"
//    window\s*\.\s*parent\s*\.\s*location\s*\.\s*href
window.parent.location.href                    → "{refOrigin}"

// g) loadPixel() — injeção (replaceFirst) logo após o try{ (bypass se isSpaceJsContent):
//    (function\s+loadPixel\s*\(\s*o\s*\)\s*\{\s*try\s*\{)
function loadPixel(o){try{ → function loadPixel(o){try{o="https://ads.inteli.fi/proxy-tracker?u="+btoa(o)+"&refOrigin={enc}";
```

Adicionalmente, `forceAdmotionProxy` (só se `decodedUrl` contém `admotion.digital`): substitui o bloco `const e=(t,e)=>{n("https://nsp.admotion.digital/px?evt="+t+...)}` por uma versão que monta `originalUrl` e chama `n("https://ads.inteli.fi/proxy-tracker?u="+btoa(originalUrl))` — copiar `oldCode`/`newCode` literais do método (linhas 472–473 do Java).

**Regra do commit c26eba4 (paridade):** somente o **JS VPAID do AdForce** é proxiado via proxy-audit; eventos de tracker do AdForce têm bypass (essa decisão é aplicada em M5-02/M5-03 no rewrite do XML, não aqui — apenas documentar na MATRIZ que o `/proxy-audit` recebe o JS VPAID já direcionado pelo pipeline).

**A.6 — resposta (`ProxyAuditController`):**
- Sucesso → `200` `Content-Type` do `ProcessedScript` (default `text/javascript; charset=utf-8`), `Cache-Control: public, max-age=3600`, `Vary: Accept-Encoding`, corpo = JS modificado.
- `IllegalArgumentException` (decode/whitelist/refOrigin) → `400`, `Content-Type: text/javascript; charset=utf-8`, corpo `// proxy audit error - {mensagem}`.
- Qualquer outro erro → `500`, `text/javascript; charset=utf-8`, corpo `// proxy audit error - processing failed`.

### B) GET /safeframe/proxy-safeframe + OPTIONS /safeframe

**B.1 — OPTIONS /safeframe (preflight) → `204`:** `applyCors` (reflete `Origin` + `Vary: Origin`; sem Origin → `Access-Control-Allow-Origin: *`); `Access-Control-Allow-Headers` = eco de `Access-Control-Request-Headers` se presente, senão `Content-Type, Authorization, Accept, Origin`; `Access-Control-Allow-Methods: GET, POST, OPTIONS`; `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires` (constante `EXPOSE_HEADERS`).

**B.2 — GET param `u`** (obrigatório; ausente/vazio → `400` `Missing url parameter`). `decodeUrlParam`: tenta base64 padrão (StdEncoding, UTF-8) e **retorna a string decodificada independentemente do protocolo**; em `IllegalArgumentException` → url-decode (`URLDecoder.decode(raw, UTF-8)`).

**B.3 — validação:** se `decodedUrl` começa com `file:` → `400` `Only http/https are allowed`; parse de URL, protocolo ≠ http/https → `400` `Only http/https are allowed`; argumento inválido → `400` `Invalid url parameter`; URL malformada → se contém `file:` → `400` `Only http/https are allowed`, senão `400` `Invalid URL format`.

**B.4 — fetch:** headers `maybeCopy` de `Accept`, `Accept-Language`, `User-Agent` (só se presentes); `GET` da URL.

**B.5 — `rewriteUrls`:** regex `(https?://[^"'\s)]+)` sobre o corpo; para cada URL: se contém `inteli.fi` **ou** (`origin != null && url.startsWith(origin)`) → mantém; senão → `https://ads.inteli.fi/proxy-tracker?u=` + base64(url). Corpo nulo → string vazia.

**B.6 — `wrapInSafeframeHtml`:** embrulha o conteúdo reescrito no HTML SafeFrame literal (copiar do método, linhas 220–276): `<!DOCTYPE html>`, `<meta charset="UTF-8">`, viewport, `body,html { margin:0; padding:0; overflow:hidden; width:300px; height:250px; }`, `#ad-container { width:100%; height:100% }`, stub `window.$sf.ext.register(width,height)` retornando `geom.self`/`geom.exp`, stub `AdSDK().withSize(w,h).build()` com `console.log`, init em `window.addEventListener('DOMContentLoaded', () => new AdSDK().withSize(300,250).build())`, e o `content` dentro de `<div id="ad-container">`.

**B.7 — resposta:** `200` `Content-Type: text/html`; `copyIfPresent` de `Cache-Control`, `Expires`, `Last-Modified`, `Etag` do upstream; `applyCors` (reflete origin + `Vary: Origin`); `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires`. Argumento inválido → `400` `Invalid url parameter`; qualquer outra exceção (incl. falha upstream) → `502` `SafeFrame proxy error`.

### C) Testes obrigatórios (unit + golden por família)

Upstream simulado com `httptest.Server`. Fixtures de JS reais em `tests/golden/proxy/audit/` (uma por família: `space-vpaid.js`, `adxspace-pageurl.js`, `metrike-app.js`, `adftech-sdk.js`, `admotion.js`, `loadpixel.js`) e HTML SafeFrame em `tests/golden/proxy/safeframe/`. Casos mínimos:
1. decode base64 URL-safe + padding ausente; base64 inválido → 400 `// proxy audit error - URL Base64 inválida: ...`.
2. whitelist: host fora da lista → 400; config vazia → permissivo; wildcard `*.metrike.com.br`.
3. cada uma das 7 famílias: entrada real → saída golden idêntica; confirmar bypass de loadPixel quando Space e aplicação de `forceSpaceOriginAndReferrer` só para Space.
4. CDN base64 fixos presentes na saída (strings literais).
5. fetch falho/timeout/>2MB → 200 com corpo `// proxy audit error - upstream fetch failed` (nunca 5xx por causa do upstream).
6. headers de sucesso: `Cache-Control: public, max-age=3600` + `Vary: Accept-Encoding`.
7. safeframe: `u` base64 e url-encoded; `file://` → 400 `Only http/https are allowed`; rewrite de URLs externas e bypass de inteli.fi/origin; HTML SafeFrame golden; cópia de Cache-Control/Expires/Last-Modified/Etag; CORS reflete origin + `Vary: Origin`; upstream caído → 502 `SafeFrame proxy error`; OPTIONS → 204 com headers.

## Arquivos a criar/alterar

- `internal/proxy/audit.go` — `// Portado de: ProxyAuditService.java`: decodeBase64Url, isAllowedFetchHost, resolveRefOrigin, fetchUpstreamJs, as 7 famílias de rewrite + forceAdmotionProxy + isSpaceJsContent, pipeline processVerifierJs.
- `internal/proxy/audit_test.go` — todos os casos da seção C (proxy-audit).
- `internal/proxy/safeframe.go` — `// Portado de: SafeFrameService.java`: decodeUrlParam, rewriteUrls, wrapInSafeframeHtml, applyCors, preflight.
- `internal/proxy/safeframe_test.go` — todos os casos da seção C (safeframe).
- `cmd/proxy/main.go` — adicionar rotas `GET /proxy-audit`, `GET /safeframe/proxy-safeframe`, `OPTIONS /safeframe` ao roteador iniciado em M5-08 (ADR-001).
- `serverless.yml` — adicionar os 3 eventos httpApi à function `proxy-handler` (já existente de M5-08).
- `internal/platform/config` — chaves `PROXY_AUDIT_ALLOWED_FETCH_HOSTS`, `PROXY_AUDIT_CONNECT_TIMEOUT`, `PROXY_AUDIT_READ_TIMEOUT`, `PROXY_AUDIT_MAX_RESPONSE_SIZE` (defaults idênticos ao Java).
- `tests/golden/proxy/audit/` e `tests/golden/proxy/safeframe/` — fixtures de JS/HTML reais.
- `docs/MATRIZ-PARIDADE.md` — linhas `/proxy-audit` e `/safeframe/proxy-safeframe` → status.

## Critérios de aceite

- [ ] `decodeBase64Url` aceita base64 URL-safe (`-`→`+`, `_`→`/`) e padding ausente; inválido → 400 `// proxy audit error - URL Base64 inválida: ...`.
- [ ] Whitelist `PROXY_AUDIT_ALLOWED_FETCH_HOSTS`: vazia = permissivo; match exato OU sufixo `.host`; host fora → 400.
- [ ] Fetch com UA `InteliFi-ProxyAudit/1.0`, `Accept: application/javascript, text/javascript, */*`, timeouts 10s/30s, máx 2MB; falha → JS `// proxy audit error - upstream fetch failed` (nunca quebra o player).
- [ ] As 7 famílias de rewrite produzem saída golden idêntica ao Java; bypass de loadPixel para Space e `forceSpaceOriginAndReferrer` só para Space respeitados; base64 fixos das CDNs literais.
- [ ] `forceAdmotionProxy` aplicado só quando a URL contém `admotion.digital`.
- [ ] Resposta proxy-audit: `200` `text/javascript; charset=utf-8`, `Cache-Control: public, max-age=3600`, `Vary: Accept-Encoding`; erros de validação → `400` com comentário JS; erro interno → `500`.
- [ ] SafeFrame: `u` decodificado (base64→url-encode fallback); `file:` → `400 Only http/https are allowed`; rewrite de todas as URLs externas para `/proxy-tracker` (exceto inteli.fi/origin); HTML SafeFrame com stubs `$sf.ext.register` e `AdSDK().withSize().build()`, viewport 300x250, init no `DOMContentLoaded`.
- [ ] SafeFrame: copia `Cache-Control/Expires/Last-Modified/Etag` do upstream; CORS reflete origin + `Vary: Origin` + `Access-Control-Expose-Headers`; `400 Invalid url parameter` e `502 SafeFrame proxy error`; OPTIONS `/safeframe` → `204` com headers exatos.
- [ ] Código 100% comentado em português com `// Portado de: ...`; `make lint && make test` verdes (incl. golden por família); MATRIZ-PARIDADE atualizada.

## Dependências

Bloqueada por: [M5-08] (proxy-handler: /proxy-tracker — mesma Lambda, roteador, helpers de CORS/IP/geo e bypass do RequestValidationFilter).

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §13 (Proxy Audit), §14 (SafeFrame Proxy), §16 (filtros globais).
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §6.2 (as 7 famílias de rewrite literais), §6.3, §3 (`<JavaScriptResource>` aponta para `/proxy-audit`).
- Java: `ProxyAuditService.java` (íntegra — regex e base64 fixos), `ProxyAuditController.java` (resposta/erros), `SafeFrameService.java` (decode, rewriteUrls, wrapInSafeframeHtml, CORS).
- Commits `c26eba4` (proxiar só o JS VPAID do AdForce) e `1f8ce34` (bypass total para Google DoubleClick — aplicado no rewrite do XML em M5-02/M5-03).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (proxy-handler 512MB/29s, resposta máx 2MB), ADR-001.

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-09] proxy-handler: /proxy-audit (rewrites JS) + /safeframe seguindo docs/issues/M5-09-proxy-audit-safeframe.md e CLAUDE.md. Portar ProxyAuditService.java/ProxyAuditController.java e SafeFrameService.java (repo c:/Users/Fabio/Documents/Dev/ad-server) para internal/proxy/audit.go, internal/proxy/safeframe.go e as rotas em cmd/proxy/main.go (GET /proxy-audit, GET /safeframe/proxy-safeframe, OPTIONS /safeframe). proxy-audit: decode base64 URL-safe com padding opcional, whitelist PROXY_AUDIT_ALLOWED_FETCH_HOSTS (vazia=permissivo, wildcard sufixo), fetch UA InteliFi-ProxyAudit/1.0 (10s/30s, 2MB) com falha→JS vazio comentado, as 7 famílias de rewrite literais (ADXSPACE.pageUrl, Space origin/referrer, Space VPAID getReferrer/getOrigins, Metrike app.sourceURL/return referrer, CDNs com base64 fixos, Admotion window.top/parent, injeção no loadPixel), forceAdmotionProxy quando admotion.digital, resposta text/javascript com Cache-Control public max-age=3600 e Vary Accept-Encoding, erros 400/500 com comentário JS. safeframe: decode u, rejeitar file://, rewrite de todas as URLs para proxy-tracker exceto inteli.fi/origin, wrapper HTML SafeFrame (300x250, stubs $sf.ext.register e AdSDK, init no DOMContentLoaded), cópia de Cache-Control/Expires/Last-Modified/Etag, CORS refletindo origin com Vary Origin, OPTIONS 204, erros 400/502. Código 100% comentado em português com "// Portado de: ...". Golden tests por família (tests/golden/proxy/audit/ e .../safeframe/) verdes com make lint && make test. Atualizar MATRIZ-PARIDADE e abrir PR ao final.
```
