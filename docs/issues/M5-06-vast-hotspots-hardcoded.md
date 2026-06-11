---
title: "[M5-06] vast-handler: fluxo B (hotspots hardcoded)"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

Quando `GET /vast` recebe `hid` (sem override de campanha aplicável), o legado resolve o VAST por um **switch hardcoded de hotspots** no `VastService.java` — alguns devolvem XML inline fixo, outros disparam fetch de SmartAdServer/Metrike ([docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2, fluxo B). Pela regra de paridade (ADR-005), a fase 1 **mantém o hardcode em Go** — mover para configuração em dados é melhoria futura.

## Especificação detalhada

### Tabela de hotspots (comportamento por hid)

| Hotspot | Tipo | Comportamento |
|---|---|---|
| `SESTSENAT_1`, `SESTSENAT_2` | Inline | VAST 4.2 fixo (vídeo/tracking Sest Senat) |
| `CLARO_WIFI` | Misto | Random uniforme entre 4 opções: 3 hardcoded (inclui VPAID 00px.net) + 1 default SmartAdServer |
| `CLARO_RECOMPENSAS` | URL | Fetch SmartAdServer `videoapi.smartadserver.com/ac?siteid=596893...` |
| `CLARO_PREZAOFREE` | URL | SmartAdServer pgname=CLARO_PREZAOFREE |
| `TV_COINS_TESTE` | Inline | VAST 4.2 fixo (vídeo Santander) |
| `OPOVO_PREROLL`, `OPOVO_MIDROLL` | URL | SmartAdServer pgname=TVCOINS_OPOVO |
| `TVCULTURA_PREROLL`, `TVCULTURA_MIDROLL` | URL | SmartAdServer |
| `INTELIFI_TEST` | URL | Metrike `servedby.metrike.com.br/vast.spark?setID=61348` |
| (qualquer outro) | — | **404** |

### Tarefa central: transcrição literal do Java

⚠️ Os XMLs inline e as URLs completas (com todos os parâmetros) NÃO estão nos docs — devem ser copiados **literalmente** de:
`c:\Users\Fabio\Documents\Dev\ad-server\src\main\java\br\com\intv\adserver\presentation\service\VastService.java`
(localizar o switch/if por `hotspotId`). Cada XML inline vira um asset em `internal/vast/assets/hotspots/{HID}.xml` (go:embed); cada URL vira constante comentada com a origem. O random do CLARO_WIFI usa o sorteador de [M1-04] (uniforme, 4 opções, paridade com `NumberUtils.getPositiveIndex`).

### Integração com o pipeline

- Hotspots tipo URL seguem para o **fluxo C** (fetch dinâmico [M5-01] + rewrite [M5-02..04]) com a URL hardcoded como se fosse `vcurl` — mesmo comportamento de macros/params/headers.
- Hotspots inline devolvem o XML **após passar pelo mesmo rewrite** (verificar no Java se os inline são reescritos ou servidos crus — transcrever o comportamento exato e documentar no código).

### Follow-up registrado (NÃO fazer nesta issue)

Criar issue `melhoria` ao final: "Mover config de hotspots VAST para dados (tabela/SSM)" — referenciar esta.

## Arquivos a criar/alterar

- `internal/vast/hotspots.go` + testes
- `internal/vast/assets/hotspots/*.xml` (transcritos do Java)
- `cmd/vast/main.go` — integração do fluxo B
- `tests/golden/vast/hotspots/*.xml`

## Critérios de aceite

- [ ] Cada hid da tabela coberto por teste; hid desconhecido → 404
- [ ] XMLs inline byte-idênticos aos do `VastService.java` (diff manual no PR com link para as linhas do Java)
- [ ] CLARO_WIFI: teste estatístico do random uniforme entre as 4 opções
- [ ] Golden tests contra capturas do ambiente dev legado (1 por hotspot acessível)
- [ ] Issue de melhoria criada (follow-up) e linkada no PR
- [ ] `make lint && make test` verdes; comentários em português

## Dependências

Bloqueada por: [M5-01]

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2 (fluxo B)
- Java de origem: `VastService.java` (switch por hotspot)
- [ARQUITETURA-ALVO](../arquitetura/ARQUITETURA-ALVO.md) ADR-005 (paridade primeiro)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-06] fluxo B hotspots hardcoded seguindo docs/issues/M5-06-vast-hotspots-hardcoded.md e CLAUDE.md. Transcrever literalmente os XMLs e URLs do VastService.java (repo local ad-server), integrar com fluxo C para os de URL, golden tests por hotspot. Código comentado em português. Abrir PR ao final.
```
