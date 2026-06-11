---
title: "[M4-02] TemplateDecider: CreativeType → template"
labels: ["epic:M4-adserving", "tipo:port", "prioridade:P1"]
milestone: "M4 — Ad Serving"
---
## Contexto

No legado, o `AdComponentImpl` decide qual template renderizar a partir do `CreativeType` do creative sorteado — um mapeamento de 40+ tipos para caminhos de template implementado no `TemplateComponentImpl` ([docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.8). Esta issue porta esse mapeamento como uma tabela explícita em Go.

## Especificação detalhada

1. **Extrair o mapeamento REAL do Java** — abrir `c:\Users\Fabio\Documents\Dev\ad-server\src\main\java\br\com\intv\adserver\business\component\impl\TemplateComponentImpl.java` e a lógica de decisão associada (enum `TemplateDecider` ou switch no `AdComponentImpl`) e transcrever TODAS as entradas CreativeType→template. Entradas conhecidas (validar contra o código — o código Java é a fonte da verdade):

   | CreativeType | Template |
   |---|---|
   | BANNER | templates/bannerAd.vm |
   | VIDEO | templates/videoAd.vm |
   | BANNER_QUESTION | templates/bannerQuestionAd.vm |
   | BANNER_QUESTION_NO_AUTH | templates/bannerQuestionAdNoAuth.vm |
   | APP_INSTALL | templates/appInstallAd.vm |
   | BANNER_BUS_APP_INSTALL | templates/bannerBusAppInstallAd.vm |
   | BANNER_BUTTONCLOSE | templates/bannerButtonClose.vm |
   | JUST_BANNER | templates/justBannerAd.vm |
   | WICONNECT_VIDEO | templates/msps/wiconnect/videoAd.vm |
   | WICONNECT_BANNER | templates/msps/wiconnect/bannerAd.vm |
   | VIDEO_VPAID_SPACE | templates/space/vpaid/videoVpaidSpace.vm |
   | PROGRAMATICA_VAST | templates/programaticaVAST.vm |
   | (… completar com TODOS os 45+ tipos, incluindo IMA, IMA_PROGRAMATICA, PROGRAMATICA_CLARO, PROGRAMATICA_WEBMOTORS, PROGRAMATICA_BMC, BETANO_PREZAO, SMART_CLARO_APP, SMARTAD_RSS, GOOGLE_AD_UNIT, REDIRECT_POSTBACK, GAM_*, SMARTAD_* …) |

2. **Implementação Go** (`internal/templates/decider.go`):
   ```go
   // TemplateParaCreativeType devolve o caminho do template para o tipo de
   // creative. Tipo desconhecido devolve o template de anúncio vazio
   // (emptyAd.vm) — paridade com o comportamento do legado de nunca falhar.
   // Portado de: TemplateComponentImpl.java.
   func TemplateParaCreativeType(tipo string) string
   ```
   Mapa `map[string]string` package-level; default `templates/emptyAd.vm`.
3. Cada entrada do mapa comentada se o template tiver particularidade (ex.: tipos GAM usam fetch externo, não template local — verificar no Java como são tratados e documentar).

## Arquivos a criar/alterar

- `internal/templates/decider.go` + `decider_test.go`

## Critérios de aceite

- [ ] Mapa cobre 100% dos valores de `CreativeType` do domínio ([M1-01]) — teste itera sobre todos e garante que nenhum cai no default sem registro explícito de que é intencional
- [ ] Tipo desconhecido → emptyAd.vm (teste)
- [ ] Cada entrada confere com o `TemplateComponentImpl.java` (revisão lado a lado no PR, com link para as linhas)
- [ ] `make lint && make test` verdes

## Dependências

Bloqueada por: [M4-01]

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.8
- Java de origem: `ad-server/src/main/java/br/com/intv/adserver/business/component/impl/TemplateComponentImpl.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M4-02] TemplateDecider seguindo docs/issues/M4-02-template-decider.md e CLAUDE.md. Extrair o mapeamento completo do TemplateComponentImpl.java (repo local ad-server), implementar internal/templates/decider.go com default emptyAd, testes cobrindo todos os CreativeTypes. Código comentado em português. Abrir PR ao final.
```
