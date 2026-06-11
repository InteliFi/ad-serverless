---
title: "[M5-03] internal/vast: regras por parceiro (bypasses) + AdParameters"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

O pipeline de rewrite do VAST (M5-02) deixou como **ganchos injetados** as decisões de bypass por parceiro e o tratamento especial de `<AdParameters>`. Esta issue implementa esses ganchos, portando do `VastService.java` as 4 famílias de regras: **Google DoubleClick** (VAST inteiro marcado como "google" — URLs assinadas do Google expiram se modificadas), **vast-logger-js-\*** (Impression), **Space/00px** (VPAID) e **AdForce/adftech** (commits `c26eba4` e `74748d2` do legado: eventos de tracker do AdForce têm bypass; apenas o JS VPAID passa pelo proxy-audit).

Condições literais de bypass conforme [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3.1:

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

**Design Go (obrigatório):** funções **puras** (`func(url string) bool` / `func(url, refOrigin string) string`) em `internal/vast`, sem estado, perfeitamente testáveis com table tests + golden tests. Elas são plugadas nos ganchos `shouldBypass...`/`proxyUrlForAdParam` definidos no `RewriteCtx` da M5-02 (substituindo os stubs que não bypassam).

## Especificação detalhada

### 1. Predicados de bypass (port literal de `VastService.java` linhas ~1357–1399)

Todos parseiam a URL e extraem o host; **qualquer erro de parse → `false`** (não bypassa). Host em branco → `false`.

| Função Go | Condição literal | Portado de |
|---|---|---|
| `ShouldBypassProxyForImpression(url)` | `host` começa com `vast-logger-js-` | `shouldBypassProxyForImpression` |
| `ShouldBypassProxyForSpaceTracker(url)` | `host` termina com `00px.net` | `shouldBypassProxyForSpaceTracker` |
| `ShouldBypassProxyForAdForceTracker(url)` | `host` termina com `adftech.com.br` | `shouldBypassProxyForAdForceTracker` |
| `IsGoogleDoubleClickURL(url)` | `host` termina com `doubleclick.net` OU `googlesyndication.com` OU `googletagservices.com` | `isGoogleDoubleClickUrl` |

⚠️ `endsWith`/`startsWith` são comparações de **string simples** sobre o host (sem normalizar subdomínios, sem lowercase extra — paridade exata). Em Go: `url.Parse` + `strings.HasSuffix`/`strings.HasPrefix` sobre `u.Hostname()`; documentar em comentário qualquer diferença de comportamento do parser (ex.: Java `new URL(...)` rejeita URLs sem protocolo → `false`; reproduzir).

**Onde cada predicado é aplicado (paridade com a ordem da M5-02):**

- `Tracking`, `ClickTracking`, `Viewable/NotViewable/ViewUndetermined`, `Error`: bypass se `Space OU AdForce` (URL mantida original; variante plain embrulha em CDATA).
- `Impression`: bypass se `Impression OU Space OU AdForce` — com `RewriteImpressionURL` aplicada ANTES (item 2).
- `MediaFile`, `JavaScriptResource`, `AdParameters`: intocados quando `isGoogleDoubleClickVast == true`. ⚠️ A flag é calculada **uma vez** sobre a URL do `vcurl` decodificada (M5-01, `isGoogleDoubleClickUrl(url)` na linha ~473 do Java), NÃO por tag.

### 2. `RewriteImpressionURL(impressionURL, effectiveRefOrigin)` (port de `rewriteImpressionUrl`, linhas ~1401–1407)

```
se NÃO ShouldBypassProxyForImpression(impressionURL) → retorna impressionURL inalterada
senão → substitui o placeholder literal "[PAGEURL]" por urlencode(effectiveRefOrigin)
```

Ou seja: o placeholder `[PAGEURL]` só é expandido nas URLs `vast-logger-js-*` (que ficam sem proxy e precisam do pageUrl real). Encoding = `URLEncoder.encode(..., UTF_8)` do Java → em Go `url.QueryEscape` (documentar a equivalência `+` para espaço).

### 3. `ProxyURLForAdParam` — reescrita de URLs dentro de `<AdParameters>` (port de `proxyUrlForAdParam`, linhas ~1135–1149)

Ordem EXATA das decisões (doc 03 §3.2):

```
1. se originalUrl começa com proxyTracker ("https://ads.inteli.fi/proxy-tracker?u=")
       → retorna original (idempotência: URL local inteli.fi já proxiada não é re-embrulhada)
2. se ShouldBypassProxyForSpaceTracker OU ShouldBypassProxyForAdForceTracker → mantém original
3. se isMediaAssetUrl(originalUrl)     → mantém original
4. se isBareDomain(originalUrl)        → proxyTracker + originalUrl + proxyRefOriginSuffix   (SEM base64)
5. senão                               → proxyTracker + base64(originalUrl) + proxyRefOriginSuffix  (COM base64)
```

Helpers (port literal):

- `isMediaAssetUrl(url)`: lowercase da URL **contém** `.mp4`, `.webm`, `.m3u8`, `.mpd` ou `.mov` (substring, não sufixo).
- `isBareDomain(url)`: parse como URI; `true` se (path nulo/vazio/`"/"`) **E** query nula. Falha de parse → `false` (na dúvida, proxia com base64). Racional do legado: bare domains são proxiados SEM base64 porque o cliente concatena paths depois — base64 corromperia o payload.
- `base64` = encoder **PADRÃO** (`base64.StdEncoding`), não URL-safe; falha → retorna a URL original (port de `getBase64UrlEncoder`).

### 4. Processamento do conteúdo de `<AdParameters>` (port das linhas ~883–955, blocos `sb8`/`sb9`)

Para cada match de AdParameters (CDATA e plain, regexes da M5-02), com `content = trim(conteúdo)`:

```
se isGoogleDoubleClickVast → mantém content intocado (continue)
se content começa com "http" E NÃO contém "{" E NÃO contém "[":
    → caso URL simples: content = ProxyURLForAdParam(content, ...)
senão se content contém "http":
    → JSON/config com URLs embutidas: para cada match do regex (https?://[^\s"'\]\}]+)
      substituir por ProxyURLForAdParam(urlEncontrada, ...)
senão → mantém content intocado
```

⚠️ O regex de extração de URLs embutidas é literalmente `(https?://[^\s"'\]\}]+)` — para o delimitador, exclui espaço, aspas dupla/simples, `]` e `}`. Compatível com RE2. Substituição **literal** (sem interpretar `$`).

### 5. Testes obrigatórios (golden + table tests)

Table tests para cada predicado (hosts exatos, subdomínios, URLs inválidas, host vazio) e golden tests com fixtures reais **por parceiro** em `tests/golden/vast/`:

1. **Google** (`tests/golden/vast/google/`): VAST com `vcurl` de `pubads.g.doubleclick.net` → MediaFile + JavaScriptResource + AdParameters **byte-a-byte intocados**; Tracking/Impression continuam proxiados.
2. **AdForce** (`tests/golden/vast/adforce/`): Tracking/Impression/Viewable com host `*.adftech.com.br` mantidos originais; AdParameters JSON do AdForce com URLs adftech mantidas e URLs de terceiros proxiadas.
3. **Space/00px** (`tests/golden/vast/space/`): trackers `*.00px.net` em bypass; JavaScriptResource 00px ainda vai para proxy-audit (bypass do Space NÃO se aplica a JS).
4. **vast-logger** (`tests/golden/vast/vastlogger/`): Impression `https://vast-logger-js-xyz.example.com/imp?page=[PAGEURL]` → mantida sem proxy COM `[PAGEURL]` substituído pelo refOrigin URL-encoded.
5. **AdParameters**: caso URL simples, caso JSON com múltiplas URLs (mistura bypass + media asset `.mp4` + bare domain `https://cdn.exemplo.com` sem base64 + URL completa com base64), caso já proxiado (`proxy-tracker?u=` não duplica), caso sem `http`.

## Arquivos a criar/alterar

- `internal/vast/partner_rules.go` — `// Portado de: VastService.java (shouldBypass*/isGoogleDoubleClickUrl/rewriteImpressionUrl)`: 4 predicados + `RewriteImpressionURL`.
- `internal/vast/adparams.go` — `// Portado de: VastService.java (proxyUrlForAdParam/isBareDomain/isMediaAssetUrl + blocos sb8/sb9)`: helpers + processamento do conteúdo.
- `internal/vast/partner_rules_test.go`, `internal/vast/adparams_test.go`.
- `internal/vast/rewrite.go` — plugar as funções reais nos ganchos do `RewriteCtx` (remover stubs da M5-02).
- `tests/golden/vast/{google,adforce,space,vastlogger}/*.in.xml` + `*.out.xml`.
- `docs/MATRIZ-PARIDADE.md` — linhas de bypass por parceiro e AdParameters.

## Critérios de aceite

- [ ] 4 predicados com as condições LITERAIS do §3.1 (doubleclick.net/googlesyndication.com/googletagservices.com; vast-logger-js-; 00px.net; adftech.com.br); erro de parse → `false`.
- [ ] Flag `isGoogleDoubleClickVast` calculada 1× sobre a URL do vcurl decodificada e aplicada a MediaFile/JavaScriptResource/AdParameters (intocados).
- [ ] `RewriteImpressionURL`: `[PAGEURL]` substituído por refOrigin URL-encoded SOMENTE quando a URL é `vast-logger-js-*`.
- [ ] `ProxyURLForAdParam` com a ordem exata: já-proxiada → bypass → media asset (.mp4/.webm/.m3u8/.mpd/.mov, substring lowercase) → bare domain SEM base64 → senão COM base64 (encoder padrão).
- [ ] `isBareDomain`: path vazio/`/` E sem query; falha de parse → `false`.
- [ ] Conteúdo de AdParameters: URL simples × JSON com URLs embutidas (regex `(https?://[^\s"'\]\}]+)`) × conteúdo sem http intocado.
- [ ] Funções puras, sem estado, plugadas nos ganchos da M5-02 (stubs removidos).
- [ ] Golden tests por parceiro verdes em `tests/golden/vast/`; `make lint && make test` verdes.
- [ ] Código 100% comentado em português com `// Portado de: VastService.java`; MATRIZ-PARIDADE atualizada.

## Dependências

Bloqueada por: [M5-02]
Bloqueia: [M5-04] (Impression→Tracking start), [M5-05] (fluxo A usa `buildProxyTrackerUrl` com o mesmo base64).

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3.1 (condições literais de bypass), §3.2 (AdParameters).
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §5 (padrões transversais).
- Java: `ad-server/src/main/java/br/com/intv/adserver/presentation/service/VastService.java` — `shouldBypassProxyForImpression` (~1357), `shouldBypassProxyForSpaceTracker` (~1367), `shouldBypassProxyForAdForceTracker` (~1377), `isGoogleDoubleClickUrl` (~1387), `rewriteImpressionUrl` (~1401), `proxyUrlForAdParam` (~1135), `isBareDomain` (~1155), `isMediaAssetUrl` (~1167), blocos `sb8`/`sb9` (~883–955).
- Commits do legado: `c26eba4` (proxy só do JS VPAID AdForce), `1f8ce34` (bypass total Google DoubleClick), `74748d2` (alinhamento AdForce).
- Issue [M5-02] (ganchos `shouldBypass...`/`proxyUrlForAdParam` do `RewriteCtx`).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-03] internal/vast: regras por parceiro (bypasses) + AdParameters seguindo docs/issues/M5-03-partner-rules.md e CLAUDE.md. Portar de VastService.java os 4 predicados de bypass (Google doubleclick.net/googlesyndication.com/googletagservices.com; vast-logger-js-*; 00px.net; adftech.com.br), RewriteImpressionURL ([PAGEURL]→refOrigin encoded) e ProxyURLForAdParam (já-proxiada→bypass→media asset→bare domain SEM base64→senão COM base64 padrão), plugando nos ganchos do RewriteCtx da M5-02. Funções puras, código comentado em português, golden tests por parceiro em tests/golden/vast/ e make lint && make test verdes. Abrir PR ao final.
```
