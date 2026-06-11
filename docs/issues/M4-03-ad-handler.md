---
title: "[M4-03] ad-handler: GET /ad (pipeline completo)"
labels: ["epic:M4-adserving", "tipo:port", "prioridade:P1"]
milestone: "M4 — Ad Serving"
---
## Contexto

`GET /ad` é o endpoint de serving de banners/scripts: dado um hotspot, sorteia uma campanha elegível, sorteia um creative e devolve o script JavaScript renderizado. Port de `AdService.java` + `AdComponentImpl.java` ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §2 e [02-logica-negocio.md](../legado/02-logica-negocio.md) §1.1). Esta issue monta o pipeline completo na Lambda `ad-handler` usando os blocos já portados (seleção [M1-04], templates [M4-01]/[M4-02], MySQL [M3-01]).

## Especificação detalhada

### Contrato HTTP (paridade exata)

- **Params:** `hid` (opcional, default `""`), `red` (opcional — redirect URL repassada ao template).
- `hid` vazio → **404**.
- `hid` com múltiplos valores separados por `|` → usar apenas a **primeira** ocorrência (`getFirstSplitOccurence`).
- Resposta **200**: `Content-Type: text/javascript` + `Content-Description: File Transfer` + `Content-Disposition: attachment; filename="adscript.js"`.
- Script renderizado vazio/nulo → **404**.

### Pipeline (cmd/ad/main.go + internal/)

1. Normalizar `hid`: primeira ocorrência do split por `|`, depois `strings.ToUpper`.
2. Buscar hotspot por code via repositório MySQL com cache TTL 5min/500 ([M1-03] + [M3-01]). Não encontrado → 404.
3. Carregar campanhas do hotspot (`enabled=1`) + creatives ([M3-01]).
4. Filtrar elegíveis por frequency cap hora/dia em America/Sao_Paulo ([M1-02]) e sortear campanha uniformemente ([M1-04]); sem elegível → renderizar `emptyAd.vm` (paridade com NullCampaign).
5. Sortear creative; montar `CreativeTemplateRequest` com os 30+ placeholders ([M1-05]), incluindo `url_redirect` = param `red`.
6. Decidir template ([M4-02]) e renderizar; devolver com os headers exatos.

### Resiliência

O legado usa `@Retryable(CannotAcquireLockException, 3×, backoff 1s×2)` — em Go, retry de query somente para erros transitórios de conexão (wrapper de [M3-01]); NÃO replicar o backoff de 1s+2s+4s no hot path da Lambda (timeout total 10s) — usar 100ms/200ms/400ms e documentar a divergência consciente no código e no PR.

## Arquivos a criar/alterar

- `cmd/ad/main.go` (handler + roteamento /ad, /GAM, health — /GAM e health vêm de [M4-04])
- `internal/adserving/pipeline.go` + testes
- `tests/golden/ad/*` — fixtures por tipo de creative (captura conforme [M4-01])

## Critérios de aceite

- [ ] Golden tests: para os mesmos inputs (hotspot/campanha/creative fixos via fixtures de DB), a saída Go é **byte-idêntica** à saída Java capturada (após normalizar `${ad_tracker_timestamp}`, `${z_server_timestamp}` e `uniqueId`)
- [ ] Casos de erro: hid vazio→404, hotspot inexistente→404, sem campanha elegível→script emptyAd (200), template vazio→404
- [ ] Headers de resposta exatamente como o legado (teste de integração com httptest)
- [ ] `make lint && make test` verdes; código comentado em português com referências `// Portado de:`

## Dependências

Bloqueada por: [M4-02], [M3-01], [M1-04]

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §2
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.1, §1.8
- Java de origem: `AdService.java`, `AdComponentImpl.java`, `CreativeTemplateRequest.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M4-03] ad-handler GET /ad seguindo docs/issues/M4-03-ad-handler.md e CLAUDE.md. Pipeline completo hotspot→campanha→creative→template com paridade exata de contrato HTTP e golden tests contra fixtures do Java. Código comentado em português. Abrir PR ao final.
```
