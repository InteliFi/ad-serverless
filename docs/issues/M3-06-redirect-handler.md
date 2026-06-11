---
title: "[M3-06] redirect-handler: GET /redirect"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

`GET /redirect` devolve uma página HTML com JavaScript que registra o evento `REDIRECT_CAMPAIGN` (via XHR `POST /adtrack`) e então redireciona o navegador para a URL de destino. É usado nas URLs de clique geradas pelos templates (`${tracking_redirect_url}` — M1-05). O port é do HTML+JS **byte a byte** a partir de `RedirectService.java` ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §11): qualquer divergência quebra golden tests e o comportamento de tracking nos navegadores em produção.

Lambda dedicada `redirect-handler` (128MB / 5s — só gera HTML, sem upstream).

## Especificação detalhada

### 1. Query params (paridade Spring)

| Param | Obrigatório | Default | Uso |
|---|---|---|---|
| `url` | sim | — | destino (texto plano ou base64) |
| `cid` | sim (int) | — | campaign ID (vai no XHR e no lookup do spot) |
| `hid` | sim | — | hotspot code (vai no XHR e no lookup do spot) |
| `enc` | não | `false` | `true` → decodificar `url` de base64 (decoder PADRÃO, com padding) |
| `source` | não | — | substitui `{source}` na URL |
| `aff_sub` | não | — | substitui `{aff_sub}` na URL |
| `click_id` | não | — | substitui `{click_id}` na URL |

Param obrigatório ausente → `400` (no legado o Spring rejeitava antes do método; replicar com validação explícita).

### 2. Pipeline de validação/transformação da URL (ordem EXATA do legado)

1. **Se `enc=true`:** decodificar base64. Base64 malformado → `400` com corpo EXATO `<html><body>Invalid URL encoding</body></html>`. URL decodificada inválida → `400` com corpo `<html><body>Invalid URL format</body></html>`.
2. **Se `enc=false`:** validar a URL crua; inválida → `400` `<html><body>Invalid URL format</body></html>`.
3. **Validação `isValidUrl` (portar exata):** não-nula/não-vazia; parseável como URL absoluta (equivalente Go: `url.Parse` + exigir scheme e host, semântica do `new URL()` do Java); e **rejeitar caracteres de controle** — regex Java `.*[\s\x00-\x1F\x7F].*` (qualquer whitespace, 0x00–0x1F ou 0x7F invalida).
4. **Substituição de placeholders:** para cada um de `source`, `aff_sub`, `click_id` presente e não-vazio, substituir `{source}`/`{aff_sub}`/`{click_id}` pelo valor **URL-encoded** (`URLEncoder.encode` UTF-8 ≈ `url.QueryEscape`; atenção: o Java codifica espaço como `+`).
5. **Revalidação:** URL pós-substituição inválida → `400` `<html><body>Invalid URL after parameter replacement</body></html>`.
6. **Escaping para JS (`escapeUrlForJavaScript`, portar exato):**
   a. Tentar normalizar reconstruindo: `protocol://host[:porta]` + path (prefixando `/` se faltar) + `?query` (se houver) + `#fragment` (se houver). Falha de parse → segue com a URL original (apenas WARN).
   b. Escapar na ordem: `\` → `\\`, `'` → `\'`, `"` → `\"`, `\n` → `\\n`, `\r` → `\\r`, `\t` → `\\t`.
7. Erro inesperado em qualquer ponto → `500` com corpo `<html><body>Error processing redirect</body></html>`.

### 3. Lookup do `spot` (label do Google Analytics)

- Buscar hotspot por `hid` **como veio** (sem UPPER CASE — paridade com `hotSpotComponent.findByCode(hid)`) e campanha por `cid`.
- `spot = strings.TrimSpace(physicalId + " " + campaignName)` — cada parte vira `""` se a entidade não existir (null-safe, sem erro).
- Requer leitura MySQL (M3-01/cache M1-03); hotspot/campanha inexistentes NÃO geram erro — apenas spot parcial/vazio.

### 4. HTML+JS gerado — estrutura EXATA (golden test contra fixture do Java)

Documento SEM `<body>`: `<html><head><meta charset="UTF-8"><script type="text/javascript">…</script></head></html>`, contendo na ordem:

1. Função `inteliFiSetRedirectCookie()` — cookie `intelifi-redirect=true`, expiração `time + 900000` ms (15 min/900s), `path=/`, via `toGMTString()`.
2. Const `inteliFiCookieTrackerStatus` — detecta cookie existente (`split(';')` + `item.trim().indexOf('intelifi-redirect=') == 0`).
3. **Se cookie presente:** `console.log('Redirect tracker has already been sent!');` + `window.location = '<escapedUrl>';` direto.
4. **Senão:** snippet Google Analytics (analytics.js, objeto global `WiFiAdsGA`), `WiFiAdsGA('create', 'UA-138989897-1', 'auto');`, XHR:
   - `ajax.open('POST', 'https://ads.inteli.fi/adtrack?cid={cid}&et=REDIRECT_CAMPAIGN&hid={hid}&time={agora em yyyyMMddHHmmssSSS}', true);` — `hid` com `valueOrEmpty`; `time` = relógio do SERVIDOR (injetado, America/Sao_Paulo) no formato de 17 dígitos;
   - `ajax.onload`: se `status === 201` → `WiFiAdsGA('send', 'event', {'eventCategory': 'Redirect', 'eventAction': 'RedirectCampaign', 'eventLabel': '<spot>', 'hitCallback': function() { window.location = '<escapedUrl>'; }});`
   - por fim `inteliFiSetRedirectCookie();` + `ajax.send();`.
5. Copiar a concatenação do Java literalmente (incluindo `\n` internos dos blocos e ausência deles nos demais) — a fixture capturada do Java é a referência.

Resposta: `200 OK`, `Content-Type: text/html`.

### 5. Testes

- Golden test: fixture capturada do Java (instrução no PR: `curl -s "https://<host-java-dev>/redirect?url=...&cid=1&hid=X&enc=false"` com relógio mockável → comparar normalizando apenas o campo `time=`).
- Unitários: base64 válido/ inválido (mensagens exatas), URL com caractere de controle → 400, substituição dos 3 placeholders com URL-encoding (espaço→`+`), revalidação pós-substituição, escaping JS (URL com `'` e `\`), spot com hotspot inexistente, param obrigatório ausente → 400.

## Arquivos a criar/alterar

- `cmd/redirect/main.go`
- `internal/redirect/handler.go`, `internal/redirect/html.go` (geração do HTML; `// Portado de: RedirectService.java`), `internal/redirect/url.go` (`isValidUrl`/`escapeUrlForJavaScript`)
- `internal/redirect/*_test.go` + `tests/golden/redirect/`
- `serverless.yml` (function `redirect-handler`: GET /redirect, 128MB, 5s)
- `docs/MATRIZ-PARIDADE.md`

## Critérios de aceite

- [ ] HTML gerado idêntico ao do Java (golden test com fixture; única normalização permitida: `time=` do XHR)
- [ ] Cookie `intelifi-redirect` com TTL 900000 ms; cookie presente → redirect direto sem GA/XHR
- [ ] XHR `POST https://ads.inteli.fi/adtrack` com `et=REDIRECT_CAMPAIGN` e `time` do servidor em `yyyyMMddHHmmssSSS`
- [ ] `enc=true` decodifica base64; erros com corpos EXATOS: `Invalid URL encoding`, `Invalid URL format`, `Invalid URL after parameter replacement` (400) e `Error processing redirect` (500), todos embrulhados em `<html><body>…</body></html>`
- [ ] `isValidUrl` rejeita whitespace e caracteres de controle 0x00–0x1F/0x7F; revalidação após substituição de placeholders
- [ ] Placeholders `{source}`/`{aff_sub}`/`{click_id}` substituídos com URL-encoding (somente quando o param vem não-vazio)
- [ ] Escaping JS na ordem exata (`\`, `'`, `"`, `\n`, `\r`, `\t`) após normalização da URL
- [ ] `spot` = `physical_id + " " + nome_da_campanha` (trim), null-safe
- [ ] Relógio injetado (sem `time.Now()` na lógica); `// Portado de: RedirectService.java`; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M1-07 (middleware), M1-08 (platform/config). Para o lookup do `${spot}` usa M3-01 (FindHotspotByCode/FindCampaignActive) + cache M1-03 — pode ser implementada com stub e ligada quando M3-01 estiver mergeada.

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §11
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (redirect-handler 128MB/5s)
- Java: `ad-server/src/main/java/br/com/intv/adserver/presentation/service/RedirectService.java` (fonte do HTML byte a byte)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-06] redirect-handler no repo InteliFi/ad-serverless
seguindo docs/issues/M3-06-redirect-handler.md e CLAUDE.md: GET /redirect com port
EXATO do HTML+JS de RedirectService.java (cookie intelifi-redirect 900s, GA
UA-138989897-1 com objeto WiFiAdsGA, XHR POST https://ads.inteli.fi/adtrack com
et=REDIRECT_CAMPAIGN e time yyyyMMddHHmmssSSS do servidor, window.location no
hitCallback; cookie presente → redirect direto). Decode base64 quando enc=true,
substituição URL-encoded de {source}/{aff_sub}/{click_id}, isValidUrl rejeitando
0x00-0x1F/0x7F com revalidação, escaping JS na ordem exata, erros 400/500 com os
corpos literais do legado. Golden test contra fixture do Java. Código comentado em
português. Testes verdes. Abrir PR ao final.
```
