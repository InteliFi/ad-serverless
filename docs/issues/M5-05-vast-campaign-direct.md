---
title: "[M5-05] vast-handler: fluxo A (Campaign Direct + override)"
labels: ["epic:M5-vast", "tipo:port", "prioridade:P1"]
milestone: "M5 — VAST & Proxies"
---
## Contexto

Quando `GET /vast` recebe `cid` sem `vcurl`, o sistema gera um **VAST 4.2 localmente** a partir dos dados da campanha (Campaign Direct) — sem fetch externo. É o "fluxo A" da decisão de fluxos ([docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2). Port do `CampaignVastOverrideService` + trecho correspondente do `VastService.java`.

## Especificação detalhada

### Decisão de fluxo (ordem exata — paridade)

```
1. validar refOrigin (default https://ads.inteli.fi; protocolo inválido → 400 "Invalid refOrigin")
2. se cid != null:
     override = findValidOverride(cid)        // cache "campaignVastOverride" TTL 5min/500
     se inválido → 404
     se override válido E vcurl vazio → FLUXO A (esta issue)
3. hid não vazio → fluxo B ([M5-06]) ; vcurl não vazio → fluxo C ([M5-01]) ; senão 404
```

### findValidOverride (query [M3-01])

`campaigns c LEFT JOIN creatives cr` validando `c.enabled=1` e data atual ∈ `[start_date, end_date]` (America/Sao_Paulo); retorna `name`, `url_click`, `impression_tracker`, `url_portrait` ([docs/legado/02](../legado/02-logica-negocio.md) §1.11). Campanha inexistente/inativa/fora da janela → **404**.

### Montagem do VAST 4.2 (estrutura exata do doc 03 §2, fluxo A)

- `<Impression id="Impression-ID">` = `impression_tracker` **proxiado** via proxy-tracker ([M5-02]).
- `<Impression id="Impression-PV">` = `https://ads.inteli.fi/vasttrack?cid={cid}&et=PAGE_VIEW&hid={hid}&time={ts}`.
- `<TrackingEvents>` com os 5 eventos mapeados para `/vasttrack`:
  | VAST event | et |
  |---|---|
  | start | VIDEO_STARTED |
  | firstQuartile | 25_PER_PLAYED |
  | midpoint | 50_PER_PLAYED |
  | thirdQuartile | 75_PER_PLAYED |
  | complete | VIDEO_ENDED |
- `<MediaFiles><MediaFile type="video/mp4">` = `url_portrait`.
- `<VideoClicks><ClickThrough>` = `url_click` **DIRETO, sem proxy** — regra do commit `74748d2` do legado ("keep direct campaign click URL"). NÃO passar pelo rewrite de ClickTracking.
- `time` no formato `yyyyMMddHHmmssSSS` ([M1-05]/[M1-08]).
- Template do XML em `internal/templates/assets/` (usar o `vast42.vm` migrado em [M4-01] se for o mesmo; senão template Go dedicado com comentário da origem).

### Resposta

`200` `Content-Type: text/xml`. Erros: 400 (refOrigin), 404 (campanha).

## Arquivos a criar/alterar

- `internal/vast/campaigndirect.go` + testes
- `internal/vast/override.go` (findValidOverride + cache) + testes
- `cmd/vast/main.go` — decisão de fluxo (integra A/B/C)
- `tests/golden/vast/campaign-direct/*.xml`

## Critérios de aceite

- [ ] Golden test: VAST gerado byte-idêntico ao do Java para a mesma campanha (normalizar timestamp/cachebuster)
- [ ] ClickThrough NÃO proxiado (assert explícito no teste)
- [ ] Cache do override: 2ª chamada no mesmo container não consulta MySQL (teste com mock contando queries)
- [ ] 404 para: cid inexistente, enabled=0, fora da janela de datas
- [ ] Ordem de decisão de fluxos coberta por testes (cid+vcurl → fluxo C, não A)
- [ ] `make lint && make test` verdes; comentários em português com `// Portado de: VastService.java / CampaignVastOverrideService.java`

## Dependências

Bloqueada por: [M5-02], [M3-01]

## Referências

- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2 (fluxo A e estrutura XML)
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.11
- Java de origem: `VastService.java`, `CampaignVastOverrideService.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M5-05] fluxo A Campaign Direct seguindo docs/issues/M5-05-vast-campaign-direct.md e CLAUDE.md. Decisão de fluxo exata, query de override com cache 5min, VAST 4.2 com ClickThrough direto (sem proxy) e golden tests. Código comentado em português. Abrir PR ao final.
```
