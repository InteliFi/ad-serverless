---
title: "[M5-02] internal/vast: rewrite XML (8 categorias de tag)"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

Este é o **coração do sistema**: depois de buscar o VAST do parceiro (M5-01), o `VastService.java` reescreve as URLs do XML para que os trackers, impressões, mídia e scripts passem pelos proxies do InteliFi (`/proxy-tracker`, `/proxy-audit`, `/media`). A reescrita é feita por **regex**, em duas variantes para cada tag (conteúdo embrulhado em `<![CDATA[...]]>` e conteúdo "plain" sem CDATA — neste caso o resultado é embrulhado em CDATA para manter o XML válido na presença de `&`).

Esta issue porta as **8 categorias de tag** e a infraestrutura de pipeline. As regras de **bypass por parceiro** (Google/00px/AdForce/vast-logger) e o tratamento especial de `<AdParameters>` ficam em M5-03; aqui deixamos os ganchos `shouldBypass...`/`proxyUrlForAdParam` como dependências injetadas (interfaces/funções), com stubs que NÃO bypassam, para os golden tests desta issue funcionarem isoladamente. O cache de vídeo do `<MediaFile>` é M5-07 — aqui o gancho de cache é uma função injetada que por padrão retorna `""` (mantém URL original).

**Design Go (obrigatório):** pipeline de **funções puras** `func(xml string, ctx RewriteCtx) string`, encadeadas na ordem exata do legado. Cada função recebe e devolve string — perfeitamente testável por golden test. `RewriteCtx` carrega `proxyRefOriginSuffix`, `effectiveRefOrigin`, `isGoogleDoubleClickVast`, `clientIP` e os ganchos injetados.

## Especificação detalhada

### Constantes

```
proxyTracker = "https://ads.inteli.fi/proxy-tracker?u="
proxyAudit   = "https://ads.inteli.fi/proxy-audit?src="
base64Url(u) = base64.StdEncoding.EncodeToString([]byte(u))   // encoder PADRÃO, não URL-safe
```

A reescrita padrão de uma URL de tracker é:
`{proxyTracker}{base64Url(url)}{proxyRefOriginSuffix}` (o sufixo já inclui `&refOrigin=...` ou é vazio — vem do M5-01).
A reescrita de JS é: `{proxyAudit}{base64Url(url)}{proxyRefOriginSuffix}`.

### Ordem exata do pipeline (a ordem importa — replica o `VastService`)

1. **Tracking** (CDATA, depois plain)
2. **ClickTracking** (CDATA, depois plain)
3. **Impression** (CDATA, depois plain) — com `rewriteImpressionUrl` antes (M5-03)
4. **ViewableImpression** — `Viewable`/`NotViewable`/`ViewUndetermined` (CDATA, depois plain)
5. **Error** (CDATA, depois plain)
6. **MediaFile** (CDATA, depois plain) — cache de vídeo (M5-07)
7. **JavaScriptResource** (CDATA, depois plain)
8. **AdParameters** (CDATA, depois plain) — regras especiais (M5-03)
9. **Impression→Tracking start** (M5-04)
10. **applyCampaignOverride** se houver override (M5-05)

### Regexes de referência (copiar literalmente — sintaxe RE2/Go)

| Tag | Padrão CDATA | Padrão plain |
|---|---|---|
| Tracking | `(<Tracking[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<Tracking[^>]*>)([^<\[][^<]*)</Tracking>` |
| ClickTracking | `(<ClickTracking[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<ClickTracking[^>]*>)([^<\[][^<]*)</ClickTracking>` |
| Impression | `(<Impression[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<Impression[^>]*>)([^<\[][^<]*)</Impression>` |
| Viewable* | `(<(Viewable\|NotViewable\|ViewUndetermined)[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<(Viewable\|NotViewable\|ViewUndetermined)[^>]*>)([^<\[][^<]*)</\2>` |
| Error | `(<Error[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<Error[^>]*>)(?!\s*<!\[CDATA\[)([\s\S]*?)(</Error>)` |
| MediaFile | `(<MediaFile([^>]*)>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<MediaFile([^>]*)>)([^<\[][^<]*)</MediaFile>` |
| JavaScriptResource | `(<JavaScriptResource[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<JavaScriptResource[^>]*>)([^<\[][^<]*)</JavaScriptResource>` |
| AdParameters | `(<AdParameters[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>)` | `(<AdParameters[^>]*>)([^<\[][^<]*)</AdParameters>` |

⚠️ **Atenção Go/RE2:** o backreference `\2` do padrão plain de ViewableImpression NÃO existe em RE2. Portar como: rodar o regex sem backreference capturando o nome da tag no grupo 2 e validar, na função de substituição, que a tag de fechamento corresponde — ou compilar 3 regexes (um por nome de tag). Documentar a decisão em comentário. O grupo "plain" do Error usa lookahead negativo `(?!...)` que também não existe em RE2 — portar com verificação manual (ignorar matches cujo conteúdo já começa com `<![CDATA[`).

### Lógica de substituição (idêntica para Tracking/ClickTracking/Viewable/Error)

Para cada match, `url = trim(grupo do conteúdo)`:
- Se `url` **não** começa com `http` → manter o match original intocado (`group(0)`).
- Se começa com `http`:
  - **bypass** (M5-03: `shouldBypassProxyForSpaceTracker` OU `shouldBypassProxyForAdForceTracker`) → manter a URL original (mas, no caso plain, embrulhar em CDATA: `{abertura}<![CDATA[{url}]]>{fechamento}`).
  - senão → `{abertura}{proxyTracker}{base64Url(url)}{proxyRefOriginSuffix}{fechamento}` (variante plain embrulha em CDATA).
- Usar substituição **literal** (equivalente a `Matcher.quoteReplacement`): nunca interpretar `$`/`\` do conteúdo como referência de grupo.

**Impression** adiciona um passo: aplicar `rewriteImpressionUrl(url, effectiveRefOrigin)` (M5-03) ANTES de decidir, e o conjunto de bypass inclui também `shouldBypassProxyForImpression` (vast-logger-js-* / inteli.fi).

**MediaFile** (detalhe nesta issue, cache em M5-07):
- Se `isGoogleDoubleClickVast` → manter intocado (`continue`).
- Substituir `ip/0.0.0.0` por `ip/{clientIP}` na URL.
- `cacheableVideo` = atributos do MediaFile contêm a substring literal `type="video/mp4"`.
- Se `cacheableVideo`: chamar gancho de cache (M5-07). Retorno não-vazio → `{abertura}https://ads.inteli.fi{cachedPath}{fechamento}`. Vazio → manter original + log de erro `Video cache failed`.
- Se não cacheável → manter original.

**JavaScriptResource**:
- Se `isGoogleDoubleClickVast` → manter intocado.
- Senão → `{abertura}{proxyAudit}{base64Url(url)}{proxyRefOriginSuffix}{fechamento}` (variante plain só reescreve se começa com `http`; senão mantém).

### Headers e Content-Type da resposta `/vast`

- `Content-Type: text/xml`.
- CORS via middleware global (M1-07): reflete `Origin`, `Access-Control-Allow-Credentials: true`, `Vary: Origin`.

### Testes obrigatórios (golden)

Fixtures de VAST reais por parceiro em `tests/golden/vast/rewrite/` (entrada e saída esperada lado a lado):
1. Tracking CDATA e plain (com `&` no meio) → proxiados; conteúdo não-http intocado.
2. ClickTracking dentro de `<VideoClicks>`.
3. Impression CDATA + plain.
4. Viewable/NotViewable/ViewUndetermined nas duas variantes.
5. Error CDATA + plain (sem dupla embrulhagem de CDATA).
6. MediaFile `type="video/mp4"` com gancho de cache stub (retorna `/media/abc.mp4` → reescreve; retorna `""` → mantém) e substituição `ip/0.0.0.0`.
7. JavaScriptResource CDATA + plain.
8. `isGoogleDoubleClickVast=true`: MediaFile/JavaScriptResource/AdParameters intocados (validação completa em M5-03).
9. Idempotência: rodar o pipeline 2× não duplica `proxy-tracker?u=proxy-tracker?u=` (garantida pela checagem `startsWith(proxyTracker)` no AdParameters de M5-03; documentar para as demais tags).

## Arquivos a criar/alterar

- `internal/vast/rewrite.go` — `// Portado de: VastService.java (rewrite de 8 categorias)`: pipeline de funções puras + `RewriteCtx`.
- `internal/vast/rewrite_tags.go` — uma função por categoria (CDATA + plain).
- `internal/vast/rewrite_test.go`.
- `tests/golden/vast/rewrite/*.in.xml` + `*.out.xml`.
- `docs/MATRIZ-PARIDADE.md`.

## Critérios de aceite

- [ ] 8 categorias reescritas, cada uma nas variantes CDATA e plain, na ordem exata do legado.
- [ ] Regexes equivalentes às de referência; backreference `\2` e lookahead `(?!...)` portados corretamente para RE2 (sem regex inválida em Go).
- [ ] Substituição literal (sem interpretação de `$`/`\`); conteúdo não-http preservado.
- [ ] Encoder base64 PADRÃO (não URL-safe) para `u`/`src`; sufixo `proxyRefOriginSuffix` aplicado.
- [ ] MediaFile: bypass Google; `ip/0.0.0.0`→IP real; cacheável = `type="video/mp4"`; gancho de cache injetado (default `""`).
- [ ] JavaScriptResource → proxy-audit; bypass Google.
- [ ] Pipeline de funções PURAS (string→string), encadeáveis e testáveis isoladamente.
- [ ] Golden tests por parceiro em `tests/golden/vast/rewrite/`; `make lint && make test` verdes.
- [ ] Código comentado em português com `// Portado de: VastService.java`; MATRIZ-PARIDADE atualizada.

## Dependências

Bloqueada por: M5-01 (fetch dinâmico).
Bloqueia: M5-03 (regras por parceiro), M5-04 (Impression→start), M5-05 (fluxo A), M5-07 (video cache).

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3 (rewrite), §3 tabela de regexes.
- Java: `VastService.java` linhas ~557–955 (blocos `sb1`..`sb9`) e helper `getBase64UrlEncoder`.
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-005 (paridade), §3.

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-02] internal/vast: rewrite XML (8 categorias de tag) seguindo docs/issues/M5-02-vast-rewrite-xml.md e CLAUDE.md. Portar de VastService.java o pipeline de rewrite como funções puras string→string (Tracking, ClickTracking, Impression, ViewableImpression Viewable/NotViewable/ViewUndetermined, Error, MediaFile, JavaScriptResource, AdParameters) nas variantes CDATA e plain, na ordem exata, reescrevendo para https://ads.inteli.fi/proxy-tracker?u={base64}&refOrigin={enc} e /proxy-audit?src={base64}. Adaptar backreference \2 e lookahead negativo para RE2. Ganchos de bypass e cache injetados. Código comentado em português. Golden tests por parceiro em tests/golden/vast/rewrite/. Abrir PR ao final.
```
