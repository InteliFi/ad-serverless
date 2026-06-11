---
title: "[M1-05] internal/templates: engine ${key} + go:embed + placeholders do CreativeTemplateRequest"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P1"]
milestone: "M1 — Commons Go"
---
## Contexto

O legado renderiza ~45 templates `.vm` (genéricos, MSP WiConnect, programática, space VPAID) por **substituição literal** de placeholders `${key}` — apesar da extensão `.vm`, **NÃO é Velocity**: não há condicionais, loops nem expressões. Isso torna o port trivial e seguro: `strings.NewReplacer` em Go (ADR-004). Esta issue cria a engine, o embedding dos assets e a construção do mapa de placeholders (port do `CreativeTemplateRequest`). A cópia física dos 45 templates + golden fixtures é a issue M4-01; aqui criamos a infraestrutura com 2–3 templates de exemplo.

## Especificação detalhada

### Engine de substituição (port de `TemplateComponent`)

```go
// Renderiza substitui cada ${key} do template pelo valor do mapa.
func Renderiza(nomeTemplate string, valores map[string]string) string
```

- Substituição literal de `${key}` → valor, via `strings.NewReplacer` construído a partir do mapa (pares `"${key}", valor`).
- **Placeholder sem valor no mapa → substituído por string vazia** (paridade com `StringUtils.valueOrEmpty` do Java). Implementação: o mapa de placeholders SEMPRE contém todas as chaves conhecidas, com `""` quando o campo é nulo — nunca deixar `${key}` cru na saída para chaves conhecidas.
- **Falha de render (template inexistente, asset corrompido) → string vazia + log de erro via slog. NUNCA panic** (paridade: TemplateComponent retorna null e nunca lança exceção; o handler converte em 404).

### Carga e cache dos templates

- Assets em `internal/templates/assets/**` embarcados com `//go:embed assets` (`embed.FS`).
- Cache em memória: cada template é lido do FS embarcado **1 única vez** e guardado em `map[string]string` protegido por `sync.Once`/`sync.RWMutex` (paridade com o `ConcurrentHashMap<String,String>` estático do Java).
- Nome do template = caminho relativo dentro de `assets/` (ex.: `emptyAd.vm`, `msps/wiconnect/videoAd.vm`, `space/vpaid/videoVpaidSpace.vm`).

### Mapa de placeholders — port COMPLETO do `CreativeTemplateRequest`

```go
// NovoCreativeTemplateRequest monta o mapa completo de placeholders do anúncio.
func NovoCreativeTemplateRequest(h *domain.HotSpot, c *domain.Campaign, cr *domain.Creative, redirectURL string, agora time.Time) map[string]string
```

Placeholders OBRIGATÓRIOS (lista completa do docs/legado/02 §1.8):

| Placeholder | Valor |
|---|---|
| `${cid}` | ID da campanha |
| `${hid}` | code do hotspot |
| `${spot}` | `physicalId + " " + campaignName` (concatenação com espaço — EXATO) |
| `${url_redirect}` | redirect URL vinda do request (param `red`) |
| `${z_server_timestamp}` | millis Unix do servidor (`System.currentTimeMillis()` → `agora.UnixMilli()`) |
| `${ad_tracker_timestamp}` | `agora` formatado `yyyyMMddHHmmssSSS` (17 dígitos, sem separadores — layout Go `20060102150405` + milissegundos concatenados manualmente; ver M1-06) |
| `${tracking_redirect_url}` | `https://ads.inteli.fi/redirect?hid={hid}&cid={cid}&enc=true&url={BASE64(urlTracking)}` — base64 padrão do campo `url_tracking` do creative |
| `${url_portrait}` | creatives.url_portrait |
| `${url_bg}` / `${url_bg_mobile}` | creatives.url_bg / url_bg_mobile |
| `${url_preroll}` / `${url_preroll_mobile}` | creatives.url_preroll / url_preroll_mobile |
| `${url_video}` / `${url_video_mobile}` | creatives.url_video / url_video_mobile |
| `${url_banner_campaign}` / `${url_banner_campaign_mobile}` | colunas homônimas |
| `${url_redirect_mobile}` | creatives.url_redirect_mobile |
| `${url_install_google}` / `${url_install_google_mobile}` | colunas homônimas |
| `${url_install_apple}` / `${url_install_apple_mobile}` | colunas homônimas |
| `${title_color}` / `${title_color_mobile}` | colunas homônimas |
| `${button_color}` / `${button_color_mobile}` | colunas homônimas |
| `${page_view_tracker}` | creatives.page_view_tracker |
| `${impression_tracker}` | creatives.impression_tracker |
| `${click_campaign_tracker}` | creatives.click_campaign_tracker |
| `${video_started_tracker}` | creatives.video_started_tracker |
| `${played_25_per_tracker}` / `${played_50_per_tracker}` / `${played_75_per_tracker}` | colunas homônimas |
| `${video_end_tracker}` | creatives.video_end_tracker |
| `${title_literals}` | creatives.title_literals |
| `${prebid_code}` | creatives.prebid_code |

- Campos NULL/vazios do creative entram no mapa como `""` (nunca omitidos).
- `uniqueId` do legado = hash(hotspotId, campaignId, creativeHashCode, redirectUrl) — portar como função auxiliar determinística e documentar onde é usado.
- `agora` injetado (sem `time.Now()` no pacote), em America/Sao_Paulo.

### Testes obrigatórios

1. Substituição básica: template `"a ${cid} b ${hid}"` com mapa → saída exata.
2. Placeholder com valor vazio → some da saída (`"x${url_bg}y"` → `"xy"`).
3. Template inexistente → `""` + sem panic.
4. `${tracking_redirect_url}` montada byte a byte (fixture com `url_tracking` conhecida e base64 esperado pré-calculado).
5. `${ad_tracker_timestamp}` com `agora` fixo → exatamente 17 dígitos esperados; `${z_server_timestamp}` = millis do `agora`.
6. `${spot}` = `"PHY123 Campanha Teste"` (espaço único).
7. Cache: segunda chamada não relê o embed.FS (testável com contador interno ou hook).

## Arquivos a criar/alterar

- `internal/templates/doc.go`, `engine.go` (`// Portado de: TemplateComponent.java`), `request.go` (`// Portado de: CreativeTemplateRequest.java`), `assets/` (emptyAd.vm + 1–2 exemplos para teste), `engine_test.go`, `request_test.go`.
- `tests/golden/templates/` — fixtures dos testes 4–6 (base para o harness do M8-01).

## Critérios de aceite

- [ ] Engine = `strings.NewReplacer` com substituição literal `${key}`; sem nenhuma dependência de template engine externa.
- [ ] Assets via `go:embed`; cada template lido 1× e cacheado (paridade ConcurrentHashMap).
- [ ] Mapa de placeholders contém TODAS as ~30 chaves da tabela acima, com `""` para nulos (valueOrEmpty).
- [ ] `${tracking_redirect_url}`, `${ad_tracker_timestamp}` (yyyyMMddHHmmssSSS), `${z_server_timestamp}` (millis) e `${spot}` validados por golden test com valores exatos.
- [ ] Falha de render → `""` + log slog; nenhum panic possível (teste cobre).
- [ ] `// Portado de: TemplateComponent.java / CreativeTemplateRequest.java` presentes; código comentado em português.
- [ ] `make lint && make test` verdes.

## Dependências

Bloqueada por: M1-01 (structs de domínio).
Bloqueia: M4-01 (migração dos 45 templates), M4-02 (TemplateDecider), M8-01 (harness golden).

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.8 (TemplateComponent + lista completa de placeholders).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-004 (go:embed + strings.Replacer).
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-05 (internal/templates) do ad-serverless: engine de substituição literal ${key} com strings.NewReplacer (NÃO é Velocity), assets via go:embed com cache em memória carregado 1×, e port completo do CreativeTemplateRequest com TODOS os placeholders da tabela em docs/issues/M1-05-template-engine.md (incluindo tracking_redirect_url com BASE64 de url_tracking, ad_tracker_timestamp yyyyMMddHHmmssSSS, z_server_timestamp em millis, spot = physicalId+" "+campaignName). Falha de render → string vazia + log, nunca panic; placeholder sem valor → vazio (valueOrEmpty). Golden tests com valores exatos, código 100% comentado em português com "// Portado de: TemplateComponent.java / CreativeTemplateRequest.java", make lint && make test verdes, abrir PR feat/issue-M1-05-template-engine com Closes na issue.
```
