---
title: "[M1-04] internal/selection: seleção aleatória de campanha/creative + Null Object"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P1"]
milestone: "M1 — Commons Go"
---
## Contexto

O núcleo do `/ad` (AdComponentImpl) decide QUAL anúncio servir: filtra as campanhas elegíveis do hotspot e sorteia uma com distribuição **uniforme** — regra de negócio que distribui as impressões igualmente entre campanhas concorrentes no mesmo hotspot. Depois sorteia um creative da campanha. Quando nada é elegível, o legado usa o padrão **Null Object** (`NullCampaign`/`NullCreative`) que resulta no template `emptyAd` (anúncio vazio, nunca erro). Esta issue porta essa lógica para `internal/selection`.

## Especificação detalhada

### Filtro de elegibilidade (port de `AdComponentImpl.getHotSpotAdScript`, passos 4–7)

```go
// CampanhasElegiveis filtra as campanhas do hotspot que podem ser veiculadas agora.
func CampanhasElegiveis(campanhas []domain.Campaign, agora time.Time) []domain.Campaign
```

Uma campanha é elegível quando **(a)** `enabled = true` **E (b)** o frequency cap permite o instante `agora` (`frequencycap.IsEligibleFor` — M1-02). A ordem do slice de entrada deve ser preservada no filtro (o sorteio é que randomiza).

### Seleção aleatória uniforme (paridade com `NumberUtils.getPositiveIndex`)

```go
// SelecionaCampanha sorteia uniformemente uma campanha elegível; nil se não houver.
func SelecionaCampanha(elegiveis []domain.Campaign, r Sorteador) *domain.Campaign

// SelecionaCreative sorteia uniformemente um creative da campanha; nil se não houver.
// Paridade com Campaign.eligeCreative() do Java.
func SelecionaCreative(c *domain.Campaign, r Sorteador) *domain.Creative
```

- Java: `random.nextInt(size)` → Go: **`math/rand/v2`**, `rand.IntN(n)` (uniforme em `0..n-1`). NÃO usar `math/rand` v1 nem `crypto/rand` (não é necessidade criptográfica; paridade estatística com `Random.nextInt`).
- `Sorteador` é uma interface mínima (`IntN(n int) int`) para injetar um gerador determinístico nos testes — mesma técnica do clock injetado.
- Lista com 1 elemento → retorna esse elemento (sem chamar `IntN(1)` é aceitável, mas `IntN(1)`=0 também funciona; documentar).

### Comportamento Null Object (port de `NullCampaign`/`NullCreative`)

- Lista de elegíveis vazia → `SelecionaCampanha` retorna **nil documentado**: o chamador (ad-handler, M4-03) DEVE responder com o template `emptyAd`, **nunca** com erro/404 por falta de campanha (404 é só para hotspot inexistente).
- Campanha sem creatives → `SelecionaCreative` retorna nil com a mesma semântica.
- Decisão de design (registrar no godoc): em vez de structs `NullCampaign` com `IsNull()`, usamos ponteiro nil + contrato documentado — equivalente Go idiomático do Null Object (conforme tabela de padrões do docs/legado/02 §5).

### Injeção de clock

Nenhuma função deste pacote chama `time.Now()`: `agora time.Time` sempre vem de fora (paridade com o `Clock` injetado no Java; regra do CLAUDE.md). O handler é quem cria `time.Now().In(locSaoPaulo)`.

### Testes obrigatórios

1. **Uniformidade:** com `Sorteador` real e 3 campanhas, 30.000 sorteios → cada uma entre ~28% e ~38% (tolerância estatística frouxa, teste não-flaky).
2. **Determinismo:** com sorteador fake que retorna 2, seleciona a campanha de índice 2.
3. **Filtro:** campanha `enabled=false` nunca aparece nos elegíveis; campanha com `hour_cap` fora da hora atual idem (usar caps reais: `"5>10"` testado às 4h e às 7h de São Paulo).
4. **Null Object:** lista vazia → nil; campanha sem creatives → nil.
5. **1 elemento:** sempre selecionado.

## Arquivos a criar/alterar

- `internal/selection/doc.go` — godoc do pacote.
- `internal/selection/selection.go` — filtro + seleções (`// Portado de: AdComponentImpl.java (getHotSpotAdScript passos 4–7) + NumberUtils.getPositiveIndex + Campaign.eligeCreative`).
- `internal/selection/sorteador.go` — interface `Sorteador` + implementação default com `math/rand/v2`.
- `internal/selection/selection_test.go` — os 5 grupos de teste acima.

## Critérios de aceite

- [ ] Filtro: `enabled=true` E `frequencycap.IsEligibleFor(agora, cap)`; reutiliza M1-02 (sem duplicar parsing).
- [ ] Sorteio com `math/rand/v2.IntN` via interface `Sorteador` injetável; nenhum uso de `math/rand` v1.
- [ ] Lista vazia → nil (campanha e creative), com contrato `emptyAd` documentado no godoc.
- [ ] Nenhuma chamada a `time.Now()` no pacote (verificável por grep).
- [ ] Teste estatístico de uniformidade não-flaky + teste determinístico com sorteador fake.
- [ ] Comentários `// Portado de: ...` nas três lógicas (filtro, getPositiveIndex, eligeCreative).
- [ ] `make lint && make test` verdes; doc comments em português.

## Dependências

Bloqueada por: M1-01 (structs), M1-02 (frequencycap).
Bloqueia: M4-03 (ad-handler GET /ad).

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.1 (algoritmo do AdComponent), §5 (Null Object e Random uniforme).
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §5 (NumberUtils.getPositiveIndex).
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) §2 (o exemplo `SelecionaCampanha` é exatamente esta função).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-04 (internal/selection) do ad-serverless: filtro de campanhas elegíveis (enabled=true E frequencycap.IsEligibleFor), seleção aleatória uniforme de campanha e de creative com math/rand/v2 IntN via interface Sorteador injetável (paridade NumberUtils.getPositiveIndex e Campaign.eligeCreative), comportamento Null Object (lista vazia → nil documentado → chamador renderiza emptyAd), clock sempre injetado (zero time.Now() no pacote). Seguir docs/issues/M1-04-selection-random.md: teste estatístico de uniformidade + determinístico com sorteador fake, código 100% comentado em português com "// Portado de: AdComponentImpl.java", make lint && make test verdes, abrir PR feat/issue-M1-04-selection-random com Closes na issue.
```
