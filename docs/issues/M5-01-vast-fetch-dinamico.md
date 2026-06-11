---
title: "[M5-01] internal/vast: fetch dinâmico (macros, params, headers)"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

O `GET /vast` é o endpoint mais crítico do sistema (~40% do tráfego). Esta issue porta o **FLUXO C** (fetch dinâmico de VAST externo via `vcurl`) do `VastService.java` (~1409 linhas) para o pacote `internal/vast`: decodificação do `vcurl` em base64, expansão de macros de parceiro, parâmetros garantidos `t`/`h`, montagem dos headers upstream com IP real e geolocalização, e o mapeamento exato de erros HTTP. O **rewrite do XML** retornado é a issue M5-02; aqui entregamos o fetch que produz o XML cru + os metadados necessários para o rewrite (refOrigin efetivo, sufixo de proxy, flag Google DoubleClick).

Design Go: funções puras e testáveis (entrada string → saída string) para decode/macros/params, e um `Fetcher` que usa o client HTTP compartilhado de `internal/httpx` (M1-08). Nenhum `time.Now()` direto: relógio injetado.

## Especificação detalhada

### 1. Parâmetros de entrada do `GET /vast` (paridade exata)

| Param | Tipo | Default | Uso nesta issue |
|---|---|---|---|
| `cid` | int? | — | fluxo A (M5-05); aqui só repassado |
| `hid` | string | `""` | fluxo B (M5-06); aqui só repassado |
| `vcurl` | string? | — | URL VAST externa em **base64** |
| `gdpr` | string | `"0"` | macro `${GDPR}` |
| `gdpr_consent` | string | `""` | macro `${GDPR_CONSENT_755}` |
| `refOrigin` | string? | `https://ads.inteli.fi` | origem do player |

### 2. Validação do refOrigin (port de `normalizeRefOrigin`)

- Constante: `DEFAULT_REF_ORIGIN = "https://ads.inteli.fi"`.
- Se `refOrigin` ausente/em branco → `effectiveRefOrigin = DEFAULT_REF_ORIGIN` e `proxyRefOriginSuffix = ""` (**vazio — sem `&refOrigin=`**).
- Se presente: parse como URL; protocolo deve ser `http` ou `https` (case-insensitive). Inválido → **`400` com corpo `<error>Invalid refOrigin</error>`** e `Content-Type: text/xml`.
- Se válido: `effectiveRefOrigin = refOrigin` (como veio, sem normalizar barra) e `proxyRefOriginSuffix = "&refOrigin=" + urlencode(effectiveRefOrigin)` — este sufixo é consumido pelo rewrite (M5-02).

### 3. Decode do vcurl (port de `decodeBase64UrlSafely`)

Ordem exata de tentativas:
1. Substituir espaços por `+` (`encodedUrl.replace(' ', '+')` — espaços viram `+` por url-decode do gateway).
2. Decodificar com **base64 padrão** (`base64.StdEncoding`); se o resultado começa com `http://` ou `https://` → usar.
3. Senão, decodificar com **base64 URL-safe** (`base64.URLEncoding`, ainda com espaços→`+`); se começa com `http://`/`https://` → usar.
4. Falhou tudo → **retornar a string original** (sem erro; o fetch tentará a string como URL).

Após o decode, calcular `isGoogleDoubleClickVast = isGoogleDoubleClickUrl(urlDecodificada)` — host termina com `doubleclick.net`, `googlesyndication.com` ou `googletagservices.com` (detalhes/efeitos em M5-03). A flag deve ser devolvida junto com o XML para o pipeline de rewrite.

### 4. Expansão de macros (ordem exata, somente no fluxo vcurl)

```
1. url = replace(url, "${GDPR}", gdpr)
2. url = replace(url, "${GDPR_CONSENT_755}", gdprConsent)
3. cacheBuster = millis do relógio injetado (System.currentTimeMillis())
4. encodedRefOrigin = urlencode(effectiveRefOrigin)
5. url = replace(url, "%%CACHEBUSTER%%", cacheBuster)
6. url = replace(url, "%%REFERRER_URL_ESC%%", encodedRefOrigin)
7. url = replace(url, "%%REFERRER_URL%%", effectiveRefOrigin)
```

⚠️ A expansão NÃO acontece para hotspots do fluxo B nem para o XML inline — apenas quando a URL veio de `vcurl`.

### 5. Params garantidos (port de `addOrReplaceQueryParam`)

Garantir `t={cacheBuster}` e `h={encodedRefOrigin}` (nessa ordem) com a semântica exata:
- Regex `([?&])t=([^&]*)` (case-insensitive, chave com `regexp.QuoteMeta`); se casar → `replaceAll` mantendo o separador (`$1t={valor}`), substituindo TODAS as ocorrências.
- Se não casar → append: separador `&` se a URL já contém `?`, senão `?`.
- O valor de `h` é o refOrigin **já URL-encoded** inserido literalmente (sem re-encodar a URL inteira).

### 6. Sanitização da URI (port de `createSafeUri`)

Antes do fetch, escapar caracteres ilegais nesta ordem: `%%`→`%25%25`, `[`→`%5B`, `]`→`%5D`, `{`→`%7B`, `}`→`%7D`, `$`→`%24`, `|`→`%7C`, `\`→`%5C`, `^`→`%5E`, `` ` ``→`%60`, espaço→`%20`. Se ainda assim o parse falhar, fallback: reconstruir a URL por partes (protocolo/autoridade/path/query/fragment); último recurso: `{protocolo}://{host}`.

### 7. Headers upstream (port literal do bloco de headers do `VastService`)

| Header | Valor |
|---|---|
| `X-Forwarded-For` | header `X-Forwarded-For` do cliente se presente; senão IP real |
| `X-Real-IP` | IP real do cliente |
| `X-Forwarded-Proto` | header homônimo do cliente; senão scheme do request |
| `X-Forwarded-Host` | header `Host` do cliente |
| `X-Forwarded-Port` | porta do servidor |
| `Forwarded` | `for={ip};proto={scheme};host={Host}` |
| Geo headers | repassar TODOS os capturados (lista abaixo) |
| `Origin` | `effectiveRefOrigin` |
| `Referer` | `effectiveRefOrigin` com **barra final garantida** (`ensureTrailingSlash`) |

- **IP real** (port de `GeoLocationHelper.getClientIp`): primeiro header presente entre `X-Forwarded-For`, `X-Real-IP`, `X-Client-IP`, `CF-Connecting-IP`, `True-Client-IP` (comparação case-insensitive); se o valor contém vírgula, usar o primeiro item (trim); fallback: remote addr do request (em Lambda: `requestContext.http.sourceIp`).
- **Geo headers** (port de `GeoLocationHelper.captureGeoHeaders`, case-insensitive, repassados com o nome canônico): `CF-IPCountry`, `CF-IPContinent`, `CloudFront-Viewer-Country`, `CloudFront-Viewer-Country-Name`, `CloudFront-Viewer-Region`, `Akamai-User-Country`, `X-GeoIP-Country`, `X-GeoIP-Country-Code`, `X-GeoIP-Region`, `X-GeoIP-Region-Name`, `X-GeoIP-City`, `X-GeoIP-Country-Name`, `GeoIP-Country`, `GeoIP-Region`, `GeoIP-City`.
- Essas duas funções devem ir para `internal/httpx` (reutilizadas por M5-07, M5-08); se M1-08 já as criou, apenas consumir.

### 8. Fetch e mapeamento de erros (paridade byte-a-byte nos corpos)

GET na URL sanitizada com o client compartilhado (timeout default 60s). Resposta sempre `Content-Type: text/xml`:

| Situação | Status | Corpo |
|---|---|---|
| Corpo vazio | `500` | `<error>Empty VAST response</error>` |
| Sem substring `<VAST` | `200` | corpo segue normal; apenas **log WARN** com os 200 primeiros chars |
| Timeout/erro de conexão (`ResourceAccessException`) | `504` | `<error>Connection timed out after {duration}ms</error>` (duration = millis desde o início do request) |
| Upstream 4xx/5xx | mesmo status | `<error>{mensagem do erro}</error>` |
| Erro inesperado | `500` | `<error>Internal server error</error>` |

### 9. Testes obrigatórios (incluindo golden)

1. Decode: base64 padrão, URL-safe, com espaços no lugar de `+`, payload não-URL (retorna original), string não-base64.
2. Macros: fixture de URL real de parceiro com todas as 5 macros → saída exata (golden em `tests/golden/vast/fetch/`).
3. `addOrReplaceQueryParam`: URL sem query, com query, com `t`/`h` existentes (substitui TODAS as ocorrências e preserva separador), chave case-insensitive.
4. refOrigin: ausente → default + sufixo vazio; `ftp://x` → 400 `<error>Invalid refOrigin</error>`; válido → sufixo `&refOrigin={enc}`.
5. Headers: golden test do mapa completo de headers upstream dado um request sintético com geo headers e `X-Forwarded-For` com lista de IPs.
6. Erros: servidor de teste (`httptest`) devolvendo vazio/timeout/500 → status e corpos exatos da tabela.

## Arquivos a criar/alterar

- `internal/vast/doc.go` — visão geral do pacote (em português).
- `internal/vast/fetch.go` — `// Portado de: VastService.java (fluxo vcurl)`: decode, macros, params, safe URI, fetch.
- `internal/vast/reforigin.go` — `normalizeRefOrigin` + sufixo de proxy.
- `internal/httpx/clientip.go`, `internal/httpx/geoheaders.go` — `// Portado de: GeoLocationHelper.java` (se ainda não existirem do M1-08).
- `internal/vast/fetch_test.go`, `reforigin_test.go`.
- `tests/golden/vast/fetch/` — fixtures de URL antes/depois das macros e do mapa de headers.
- `docs/MATRIZ-PARIDADE.md` — linhas do fluxo C.

## Critérios de aceite

- [ ] Decode do `vcurl` com as 4 etapas exatas (espaço→`+`, std, URL-safe, fallback original).
- [ ] 5 macros expandidas na ordem exata; expansão SOMENTE no fluxo vcurl.
- [ ] `t`/`h` adicionados ou substituídos com a semântica regex do legado (case-insensitive, todas as ocorrências, separador preservado).
- [ ] refOrigin: default `https://ads.inteli.fi`; protocolo != http/https → `400 <error>Invalid refOrigin</error>`; sufixo `&refOrigin=` SÓ quando o param foi enviado.
- [ ] Headers upstream completos: X-Forwarded-For/X-Real-IP/X-Forwarded-Proto/Host/Port/Forwarded, geo headers, `Origin`, `Referer` com barra final.
- [ ] Erros: `500 Empty VAST response`, `504 Connection timed out after Nms`, propagação de status upstream, WARN sem `<VAST`.
- [ ] `isGoogleDoubleClickVast` calculada sobre a URL decodificada e exposta ao chamador.
- [ ] Golden tests verdes em `tests/golden/vast/fetch/`; relógio injetado (sem `time.Now()` na lógica).
- [ ] Código 100% comentado em português com `// Portado de: VastService.java` / `GeoLocationHelper.java`; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada.

## Dependências

Bloqueada por: M1-08 (internal/httpx + platform).
Bloqueia: M5-02 (rewrite XML), M5-06 (hotspots fluxo B).

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §1, §2 (FLUXO C), §10.
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §4.
- Java: `ad-server/src/main/java/br/com/intv/adserver/presentation/service/VastService.java` (linhas ~63–555 e helpers `createSafeUri`, `addOrReplaceQueryParam`, `decodeBase64UrlSafely`, `normalizeRefOrigin`, `ensureTrailingSlash`) e `presentation/service/util/GeoLocationHelper.java`.
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (vast-handler 512MB/29s).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-01] internal/vast: fetch dinâmico (macros, params, headers) seguindo docs/issues/M5-01-vast-fetch-dinamico.md e CLAUDE.md. Portar de VastService.java: decode base64 do vcurl (std→URL-safe→fallback), expansão das 5 macros (${GDPR}, ${GDPR_CONSENT_755}, %%CACHEBUSTER%%, %%REFERRER_URL_ESC%%, %%REFERRER_URL%%), params garantidos t/h, headers upstream completos com IP real e geo headers, validação de refOrigin (400 "Invalid refOrigin") e erros 500/504 exatos. Código comentado em português. Testes verdes (incluindo golden tests em tests/golden/vast/fetch/). Abrir PR ao final.
```
