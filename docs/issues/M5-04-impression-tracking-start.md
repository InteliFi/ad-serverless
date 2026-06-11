---
title: "[M5-04] internal/vast: Impression→Tracking start (AdForce)"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

Última etapa do pipeline de rewrite (passo 9 da ordem definida em M5-02): o `VastService.java` **move** as URLs de `<Impression>` e `<ViewableImpression>` para `<Tracking event="start">`, de modo que a impressão e a viewability disparem no **start do vídeo**, e não no load do XML / 50%-visível. É a regra de viewability do AdForce: o JSON de `<AdParameters>` do AdForce carrega um evento `viewable_impression` que também precisa ser extraído, removido do JSON e disparado no start (alinhamento do commit `74748d2` do legado — "align AdForce start tracking flow").

Especificação de origem: [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3.3:

1. Coleta URLs de `<Impression>` e `<Viewable>`; extrai `viewable_impression` do JSON de AdParameters (AdForce: `{"$":{"event":"viewable_impression"},"_":"https://..."}`).
2. **Remove** os blocos `<Impression>` e `<ViewableImpression>` do XML.
3. **Injeta** as URLs como `<Tracking event="start">` antes de `</TrackingEvents>`.

**Resultado:** a impressão dispara no *start* do vídeo, não no load do XML.

⚠️ Esta etapa roda **DEPOIS** do rewrite das 8 categorias (M5-02/M5-03): as URLs de Impression coletadas já estão proxiadas (`proxy-tracker?u=...`) ou em bypass (ex.: AdForce `adftech.com.br` mantida original — é exatamente assim que o tracker de impressão do AdForce dispara direto no start, sem proxy).

**Design Go (obrigatório):** função pura `func MoveImpressionsToStartTracking(vastXml string) string` em `internal/vast`, encadeada no pipeline da M5-02 na posição exata (após AdParameters, antes do `applyCampaignOverride` da M5-05).

## Especificação detalhada

### 1. Coleta das URLs (port de `VastService.java` linhas ~957–973)

Acumular em uma lista ordenada `startUrls` (a ordem de inserção é preservada na injeção):

1. Para cada match de `<Impression[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</Impression>`: `u = trim(grupo 1)`; adicionar **somente se** `u` começa com `http`.
2. Para cada match de `<Viewable[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</Viewable>`: idem.

⚠️ Só a variante CDATA é coletada (as variantes plain já foram embrulhadas em CDATA pelo rewrite da M5-02). `<NotViewable>`/`<ViewUndetermined>` NÃO são coletadas (apenas `<Viewable>`). Atenção ao regex: `<Viewable[^>]*>` também casaria `<ViewableImpression>`, mas o fechamento exigido `</Viewable>` restringe o match — reproduzir essa semântica.

### 2. Extração do `viewable_impression` do AdParameters do AdForce (linhas ~975–988)

1. Localizar o **PRIMEIRO** (e somente o primeiro) bloco `(<AdParameters[^>]*>\s*<!\[CDATA\[)([\s\S]*?)(\]\]>\s*</AdParameters>)`.
2. No conteúdo (JSON do AdForce), procurar o **primeiro** match do padrão literal:

```
,?\{"\$":\{"event":"viewable_impression"\},"_":"(https?://[^"]+)"\}
```

Formato do JSON do AdForce (doc §3.3): `{"$":{"event":"viewable_impression"},"_":"https://..."}` — o `,?` opcional no início remove também a vírgula que separa a entrada do item anterior na lista JSON.

3. Se encontrou: adicionar a URL (grupo 1) a `startUrls`; **remover a entrada** do JSON (`replaceFirst` por string vazia); substituir o bloco AdParameters original pelo bloco com o JSON limpo (substituição **literal**, preservando abertura/fechamento dos grupos 1 e 3).
4. Se não encontrou: XML inalterado nesta sub-etapa.

### 3. Remoção e injeção (linhas ~990–1000) — SOMENTE se `startUrls` não vazia

Se `startUrls` está **vazia**, o XML sai intocado (os blocos `<Impression>` permanecem). Caso contrário:

1. Remover TODOS os blocos `<Impression[^>]*>\s*<!\[CDATA\[[\s\S]*?\]\]>\s*</Impression>` (replaceAll por `""`).
2. Remover TODOS os blocos `<ViewableImpression[^>]*>[\s\S]*?</ViewableImpression>` (replaceAll por `""` — leva junto Viewable/NotViewable/ViewUndetermined internos).
3. Montar o bloco de injeção, **uma linha por URL, com a indentação literal do legado** (14 espaços antes de `<Tracking`, `\n` ao final):

```
              <Tracking event="start"><![CDATA[{url}]]></Tracking>
```

4. Substituir a **PRIMEIRA** ocorrência de `</TrackingEvents>` por `{blocoDeInjeção}            </TrackingEvents>` (12 espaços antes do fechamento). Em Go: `strings.Replace(xml, "</TrackingEvents>", bloco+"            </TrackingEvents>", 1)` — o `replaceFirst` do Java usa regex, mas o padrão `</TrackingEvents>` não tem metacaracteres, então a substituição literal é equivalente (documentar em comentário).

⚠️ Se o VAST tem múltiplos `<Creative>`, apenas o primeiro `</TrackingEvents>` recebe as URLs — paridade exata, não "corrigir".

### 4. Posição no pipeline e interação com M5-05

- Encadear em `internal/vast/rewrite.go` imediatamente após o processamento de AdParameters (blocos `sb8`/`sb9`) e ANTES de `applyCampaignOverride` (M5-05) — o override injeta seu `impression_tracker` pelo mesmo mecanismo (`<Tracking event="start">` antes de `</TrackingEvents>`), contando que os `<Impression>` já foram removidos por esta etapa.
- A função NÃO depende de `RewriteCtx` (não usa refOrigin nem bypass): assinatura `string → string` pura.

### 5. Testes obrigatórios (golden, fixtures por parceiro)

Fixtures em `tests/golden/vast/start-tracking/` (entrada = saída da etapa AdParameters; saída esperada capturada do Java):

1. **AdForce completo** (`adforce-viewable-impression`): VAST com Impression proxiada + Impression adftech em bypass + AdParameters JSON contendo `{"$":{"event":"viewable_impression"},"_":"https://ev.adftech.com.br/..."}` no meio da lista (com vírgula) → 3 `<Tracking event="start">` injetados na ordem de coleta; entrada `viewable_impression` removida do JSON (sem vírgula órfã); blocos Impression removidos.
2. **ViewableImpression Space**: `<ViewableImpression>` com `<Viewable>` + `<NotViewable>` → só a URL de `<Viewable>` coletada; o bloco `<ViewableImpression>` inteiro removido.
3. **Sem URLs coletáveis**: VAST sem Impression http (ex.: conteúdo não-http) → XML byte-a-byte intocado.
4. **viewable_impression no início da lista JSON** (sem vírgula antes) → remoção limpa.
5. **Múltiplos `</TrackingEvents>`**: injeção apenas no primeiro.
6. **Idempotência observável**: rodar a função 2× sobre a saída não duplica Tracking (não há mais `<Impression>` para coletar).

## Arquivos a criar/alterar

- `internal/vast/start_tracking.go` — `// Portado de: VastService.java (Impression→Tracking start, linhas ~957–1000)`: `MoveImpressionsToStartTracking`.
- `internal/vast/start_tracking_test.go`.
- `internal/vast/rewrite.go` — encadear a etapa na posição 9 do pipeline.
- `tests/golden/vast/start-tracking/*.in.xml` + `*.out.xml`.
- `docs/MATRIZ-PARIDADE.md` — linha "Impression+Viewable → Tracking start (AdForce viewable_impression)".

## Critérios de aceite

- [ ] Coleta de Impression e Viewable (variante CDATA, trim, somente URLs `http*`), preservando a ordem.
- [ ] Extração do `viewable_impression` com o regex literal `,?\{"\$":\{"event":"viewable_impression"\},"_":"(https?://[^"]+)"\}` aplicado SOMENTE ao primeiro bloco AdParameters; entrada removida do JSON sem corromper a lista.
- [ ] `startUrls` vazia → XML intocado; não vazia → remoção de TODOS os blocos Impression/ViewableImpression e injeção antes do PRIMEIRO `</TrackingEvents>` com a indentação literal (14/12 espaços).
- [ ] Etapa posicionada após AdParameters e antes do `applyCampaignOverride` (M5-05); função pura `string → string`.
- [ ] URLs em bypass do AdForce (adftech.com.br) chegam originais ao Tracking start — alinhado ao fluxo do commit `74748d2`.
- [ ] Golden tests verdes em `tests/golden/vast/start-tracking/` (fixtures AdForce e Space); `make lint && make test` verdes.
- [ ] Código 100% comentado em português com `// Portado de: VastService.java`; MATRIZ-PARIDADE atualizada.

## Dependências

Bloqueada por: [M5-03]
Bloqueia: [M5-05] (o `applyCampaignOverride` injeta o impression_tracker pelo mesmo mecanismo).

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3.3 (Impression → Tracking event="start") e §10 (checklist de paridade).
- Java: `ad-server/src/main/java/br/com/intv/adserver/presentation/service/VastService.java` linhas ~957–1000 (coleta, extração do viewable_impression, remoção e injeção).
- Commit do legado: `74748d2` — "fix(vast): keep direct campaign click URL and align AdForce start tracking flow".
- Issues [M5-02] (pipeline e regexes), [M5-03] (bypass AdForce que preserva as URLs adftech originais).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-04] internal/vast: Impression→Tracking start (AdForce) seguindo docs/issues/M5-04-impression-tracking-start.md e CLAUDE.md. Portar de VastService.java (linhas ~957–1000) a função pura MoveImpressionsToStartTracking: coletar URLs http de <Impression> e <Viewable> (CDATA), extrair viewable_impression do JSON de AdParameters do AdForce com o regex literal da issue, remover os blocos Impression/ViewableImpression e injetar tudo como <Tracking event="start"> antes do primeiro </TrackingEvents> com a indentação exata. Encadear como passo 9 do pipeline de rewrite. Código comentado em português, golden tests em tests/golden/vast/start-tracking/ e make lint && make test verdes. Abrir PR ao final.
```
