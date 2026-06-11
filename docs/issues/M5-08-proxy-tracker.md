---
title: "[M5-08] proxy-handler: /proxy-tracker"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

O `/proxy-tracker` é o destino de TODAS as URLs de tracking reescritas pelo pipeline VAST (M5-02): `<Tracking>`, `<ClickTracking>`, `<Impression>`, `<ViewableImpression>`, `<Error>` e `<AdParameters>` apontam para `https://ads.inteli.fi/proxy-tracker?u={base64(url)}&refOrigin={enc}`. Ele atua como proxy reverso "limpador": busca a URL original do parceiro com headers controlados (User-Agent fixo, IP real do cliente, Origin/Referer forçados para o `refOrigin`) e devolve a resposta ao player com CORS local — escondendo o contexto real da página onde o anúncio roda. Também é o ponto de observabilidade de **VAST Errors** (códigos de erro reportados pelos players).

No legado é o `ProxyTrackerService.java` (Spring `@RestController` em `/proxy-tracker`, métodos `preflight` OPTIONS e `proxyTracker` GET). Na arquitetura alvo vira parte da Lambda **proxy-handler** (512MB / 29s, I/O-bound — ver ARQUITETURA-ALVO §3), rotas `GET /proxy-tracker` e `OPTIONS /proxy-tracker`, com a lógica em `internal/proxy`.

⚠️ O `RequestValidationFilter` (M1-07) tem **bypass específico** para a query string de `/proxy-tracker` (permite base64 bruto) e para o param `u` (apenas checagem de non-printable) — conferir que o middleware está encadeado com esses bypasses ativos.

## Especificação detalhada

Portar de `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/br/com/intv/adserver/presentation/service/ProxyTrackerService.java` (e helper `presentation/service/util/GeoLocationHelper.java`).

### 1. OPTIONS /proxy-tracker (preflight) → `204 No Content`

Headers EXATOS (doc legado 01 §12):
- `Access-Control-Allow-Headers`: eco do header `Access-Control-Request-Headers` do request se presente e não vazio; senão default `Content-Type, Authorization, Accept, Origin`.
- `Access-Control-Allow-Methods: GET, POST, OPTIONS`
- `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires` (constante `EXPOSE_HEADERS`)
- `Cross-Origin-Embedder-Policy: credentialless`

O middleware CORS global (M1-07) NÃO deve curto-circuitar este OPTIONS com `200` — usar o opt-out previsto em M1-07 para que o handler responda `204` com os headers acima (o `Access-Control-Allow-Origin` refletindo o `Origin` continua vindo do middleware, como o `CorsFilter` fazia no legado).

### 2. GET /proxy-tracker — parâmetros

- `u` (obrigatório): URL em **base64 OU url-encoded**. Ausente/vazio → `400` corpo `Missing url parameter`.
- `refOrigin` (opcional): default `https://ads.inteli.fi` (constante `DEFAULT_REF_ORIGIN`). Se presente, deve ser URL válida com protocolo http/https; inválido → erro de argumento → `400` (mensagem do legado: `refOrigin inválido: {valor}` / `refOrigin precisa usar http ou https`).

### 3. Decode do param `u` (ordem EXATA do legado — `decodeUrlParam`)

1. Tenta decodificar como **base64 padrão** (`base64.StdEncoding`). Se decodificar SEM erro **e** o resultado começa com `http://` ou `https://` → usa o resultado.
2. Senão (erro de base64 OU resultado sem prefixo http) → **url-decode** do valor cru (`url.QueryUnescape`). Falha no url-decode → `400` `Invalid request - {msg}`.
3. URL decodificada é validada com parse de URL; protocolo diferente de http/https (case-insensitive) → `400` corpo `Only http/https are allowed`.

### 4. Headers do request upstream (montar do ZERO, nunca repassar todos)

| Header | Valor |
|---|---|
| `Accept`, `Accept-Encoding`, `Accept-Language` | copiados do request do cliente, SOMENTE se presentes (primeiro valor) |
| `User-Agent` | fixo: `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36` |
| `X-Forwarded-For` | IP real do cliente via `httpx.IPRealDoCliente` (M1-08): busca case-insensitive em `X-Forwarded-For`, `X-Real-IP`, `X-Client-IP`, `CF-Connecting-IP`, `True-Client-IP`; se valor contém vírgula, usar o primeiro IP (trim); fallback = sourceIP do request |
| headers geo | TODOS os capturados via `httpx.HeadersGeo` (M1-08), case-insensitive, repassados com nome canônico: `CF-IPCountry`, `CF-IPContinent`, `CloudFront-Viewer-Country`, `CloudFront-Viewer-Country-Name`, `CloudFront-Viewer-Region`, `Akamai-User-Country`, `X-GeoIP-Country`, `X-GeoIP-Country-Code`, `X-GeoIP-Region`, `X-GeoIP-Region-Name`, `X-GeoIP-City`, `X-GeoIP-Country-Name`, `GeoIP-Country`, `GeoIP-Region`, `GeoIP-City` (log INFO com a contagem quando não vazio, como no legado) |
| `Origin` | `effectiveOrigin` (refOrigin resolvido) |
| `Referer` | `effectiveOrigin` com `/` final garantido (`ensureTrailingSlash`: só adiciona se não termina em `/`) |

Fetch `GET` com o cliente HTTP compartilhado (M1-08, timeout default 60s), corpo como `[]byte` (a resposta pode ser binária — pixel GIF).

### 5. Resposta ao cliente (propagação)

1. Copiar **status, corpo e headers** do upstream.
2. **Remover** dos headers copiados: todos cujo nome (lowercase) começa com `access-control-` E o header `Vary` (`stripUpstreamCors` do legado).
3. Adicionar: `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires` e `Cross-Origin-Embedder-Policy: credentialless`.
4. CORS local refletindo o `Origin` do request via middleware M1-07 (`Access-Control-Allow-Origin: {origin}` + `Access-Control-Allow-Credentials: true` + `Vary: Origin`).
5. Se a URL decodificada contém a substring `type=js` → **forçar** `Content-Type: application/javascript` (sobrescreve o do upstream).
6. Corpo binário → resposta APIGatewayV2 com `IsBase64Encoded: true` (helper `resp.Binario` de M1-08).

### 6. Logging especial de VAST Error (paridade observacional)

- `isVastError = decodedUrl contém "/errors?" OU "&error="` (avaliado logo após o decode).
- Antes do fetch, se `isVastError`: log INFO `VAST ERROR TAG invoked` com URL decodificada, refOrigin e `User-Agent` do cliente.
- Depois do fetch com sucesso, se `isVastError`: log ERROR `VAST ERROR TRACKING` com URL, `ErrorCode` extraído, status do upstream e tamanho do corpo.
- `extractErrorParam(url)`: procura `?error=` e, se ausente, `&error=`; extrai o valor de `índice+7` até o próximo `&` (ou fim da string); ausente → `NOT_FOUND`; exceção → `ERROR_EXTRACTING`.

### 7. Erros

- Argumento inválido (decode/URL/refOrigin) → `400` corpo `Invalid request - {mensagem}` (ou as mensagens específicas das seções 2–3).
- Falha de fetch upstream (rede, timeout) → `502` corpo `Proxy error`; se `isVastError`, log ERROR `VAST ERROR TRACKING FAILED`, senão `PROXY TRACKER ERROR`.
- Nunca panic: middleware `Recover` (M1-07) encadeado.

### 8. Testes obrigatórios (unit + golden)

Upstream simulado com `httptest.Server`; fixtures reais em `tests/golden/proxy/tracker/`:
1. OPTIONS: 204 com os 4 headers exatos; eco de `Access-Control-Request-Headers`.
2. Decode: `u` base64 de URL http → usa base64; `u` base64 válido de string não-http (ex.: `aGVsbG8=`) → cai no url-decode; `u` url-encoded; `u` ausente → 400 `Missing url parameter`; protocolo `ftp://` → 400 `Only http/https are allowed`.
3. Headers upstream: User-Agent fixo, X-Forwarded-For com primeiro IP da lista, geo headers repassados, `Origin`/`Referer` (com barra) do refOrigin custom e default.
4. Propagação: status 302 do upstream preservado; headers `access-control-*` e `Vary` do upstream removidos; `Cross-Origin-Embedder-Policy` presente; corpo binário (GIF 1x1) intacto via base64.
5. `type=js` na URL → `Content-Type: application/javascript` mesmo com upstream `text/plain`.
6. VAST error: URL com `&error=303` → log capturado com `ErrorCode: 303`; URL com `/errors?` sem param → `NOT_FOUND`.
7. Upstream derrubado → 502 `Proxy error`.

## Arquivos a criar/alterar

- `internal/proxy/tracker.go` — `// Portado de: ProxyTrackerService.java`: decode de `u`, resolveRefOrigin, montagem de headers, propagação, logging VAST error.
- `internal/proxy/tracker_test.go` — todos os casos da seção 8.
- `cmd/proxy/main.go` — main da Lambda proxy-handler com roteador (ADR-001): `GET /proxy-tracker`, `OPTIONS /proxy-tracker` (M5-09 adiciona as demais rotas).
- `serverless.yml` — function `proxy-handler` (512MB, timeout 29s, arm64) com os 2 eventos httpApi.
- `tests/golden/proxy/tracker/` — fixtures de upstream (GIF, JS, XML de erro).
- `docs/MATRIZ-PARIDADE.md` — linha `/proxy-tracker` → status.

## Critérios de aceite

- [ ] OPTIONS retorna `204` com `Access-Control-Allow-Headers` (eco ou default), `Access-Control-Allow-Methods: GET, POST, OPTIONS`, `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires`, `Cross-Origin-Embedder-Policy: credentialless`.
- [ ] Decode de `u`: base64 padrão primeiro (aceito só se resultado começa com http(s)), fallback url-decode; falha → 400; protocolo ≠ http/https → 400 `Only http/https are allowed`.
- [ ] `refOrigin` default `https://ads.inteli.fi`; inválido → 400.
- [ ] Headers upstream EXATOS: Accept/Accept-Encoding/Accept-Language copiados, User-Agent fixo Chrome/91 literal, X-Forwarded-For com IP real (5 headers candidatos, primeiro IP), geo headers repassados, `Origin`=refOrigin, `Referer`=refOrigin+`/`.
- [ ] Resposta propaga status/corpo/headers; remove `access-control-*` e `Vary` do upstream; adiciona CORS local refletindo origin + COEP credentialless; `type=js` força `Content-Type: application/javascript`.
- [ ] Logging de VAST error (`/errors?` ou `&error=`) com extração do código (`NOT_FOUND`/`ERROR_EXTRACTING` nos casos de borda).
- [ ] Falha upstream → `502` corpo `Proxy error`.
- [ ] Corpo binário propagado corretamente (base64 no payload v2).
- [ ] Código 100% comentado em português com `// Portado de: ProxyTrackerService.java`; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada.

## Dependências

Bloqueada por: [M1-07] (middleware CORS/validação com bypass de /proxy-tracker), [M1-08] (httpx: client, IP real, geo headers; platform: resp helpers).
Bloqueia: [M5-09] (proxy-audit/safeframe na mesma Lambda).

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §12 (Proxy Tracker), §16 (filtros globais e bypass).
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3 (URLs reescritas apontam para cá), §6.1.
- Java: `c:/Users/Fabio/Documents/Dev/ad-server/.../presentation/service/ProxyTrackerService.java` (íntegra) e `.../presentation/service/util/GeoLocationHelper.java` (IP real + geo headers).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (proxy-handler 512MB/29s), ADR-001.

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-08] proxy-handler: /proxy-tracker seguindo docs/issues/M5-08-proxy-tracker.md e CLAUDE.md. Portar ProxyTrackerService.java (repo c:/Users/Fabio/Documents/Dev/ad-server) para internal/proxy/tracker.go + cmd/proxy/main.go: OPTIONS 204 com headers exatos; GET com decode de u (base64 primeiro, fallback url-decode), validação http/https, headers upstream (Accept* copiados, User-Agent fixo Chrome/91, X-Forwarded-For com IP real, geo headers, Origin=refOrigin, Referer=refOrigin+/), propagação de status/corpo/headers removendo access-control-* e Vary do upstream, COEP credentialless, Content-Type forçado para application/javascript quando a URL contém type=js, logging de VAST error (/errors? ou &error=) e 502 "Proxy error" em falha. Código 100% comentado em português com "// Portado de: ...". Testes unit + golden (tests/golden/proxy/tracker/) verdes com make lint && make test. Atualizar MATRIZ-PARIDADE e abrir PR ao final.
```
