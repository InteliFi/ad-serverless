---
title: "[M8-02] Testes de carga k6 (10× volume)"
labels: ["epic:M8-qualidade", "tipo:teste", "prioridade:P1"]
milestone: "M8 — Qualidade"
---
## Contexto

O sistema legado atende **~2M req/dia (~23 rps de média, com picos estimados em 10×)** e a meta não-funcional da migração é **suportar ≥10× o volume sem ação manual**, com **p50 < 20ms e p99 < 150ms** nas rotas sem upstream ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §6). Antes do cutover (M9), precisamos PROVAR essas metas com testes de carga reprodutíveis: esta issue cria os cenários **k6** em `tests/load/` (diretório já previsto na estrutura do repo — [ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §4), com mix realista de tráfego, rampa até **250 rps** (10× a média de ~23 rps), thresholds que falham o teste automaticamente, execução contra o **stage dev** e comparação dos resultados com a tabela de metas do legado.

Dois modos de execução: **smoke de 30s no CI** (valida que os cenários e o ambiente funcionam, baixa taxa) e **carga completa manual** (rampa até 250 rps, sob supervisão — gera custo e escreve tracking real em dev).

⚠️ Riscos a controlar durante o teste (ver [PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Riscos):
- **MySQL compartilhado:** monitorar `ClientConnections`/`DatabaseConnections` do RDS Proxy (alarme A4 da M7-04) durante toda a execução — abortar se > 80%.
- **Parceiros upstream vivos:** o teste NÃO pode bombardear parceiros reais (Google, Space, AdForce, SmartAd, Metrike). O cenário de `/vast` usa o **fluxo B (hotspots hardcoded)** e/ou `vcurl` apontando para um **mock upstream** controlado por nós (ver §3) — nunca URLs de parceiros de produção.
- **Tracking real:** os eventos gerados entram na fila SQS e são gravados no MySQL/DynamoDB de dev. Usar `cid`/`hid` de teste dedicados (documentar quais) para permitir limpeza/identificação posterior.

## Especificação detalhada

### 1. Mix de tráfego (paridade com a distribuição real)

`/vast` é ~40% do tráfego ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §4). Mix alvo dos cenários:

| Grupo | Rotas | % do mix | Observações |
|---|---|---|---|
| **vast** | `GET /vast` | ~40% | fluxo B (hotspots hardcoded) + fluxo C com `vcurl` → mock upstream |
| **tracking** | `POST /adtrack` (~20%), `GET /vasttrack` (~15%), `GET /trackingpixel` (~5%) | ~40% | `time` no formato `yyyyMMddHHmmssSSS` gerado por request; sequência realista de milestones de vídeo (PAGE_VIEW → VIDEO_STARTED → 25/50/75_PER_PLAYED → VIDEO_ENDED) |
| **demais** | `GET /ad` (~8%), `GET /redirect` (~5%), `GET /proxy-tracker` (~4%), `GET /health` (~3%) | ~20% | proxy-tracker com `u` = base64 de URL do mock upstream |

Percentuais implementados como **cenários k6 separados** (executor `ramping-arrival-rate`, um por grupo) com `preAllocatedVUs` dimensionados; tags `route` em cada request para thresholds e breakdown por rota.

### 2. Perfis de carga

- **smoke** (`tests/load/smoke.js`): 30s, taxa fixa baixa (ex.: 5 rps total no mesmo mix), thresholds idênticos aos da carga completa. Roda no CI após o deploy de dev.
- **carga completa** (`tests/load/mix.js`): rampa em estágios — 0→25 rps (2 min, aquecimento/cold starts), 25→100 rps (5 min), 100→250 rps (5 min), platô 250 rps (10 min), rampa de descida (1 min). Total ~23 min. Execução MANUAL, nunca no CI.
- **(opcional, documentar)** perfil `spike.js`: salto 25→250 rps em 10s para medir comportamento de cold start em burst (meta < 100ms — docs/legado/05 §6).

### 3. Mock upstream para vast/proxy

Para não depender (nem abusar) de parceiros reais: subir um endpoint estático servindo um XML VAST representativo (fixture de `tests/golden/fixtures/vast/` da M8-01) — opções aceitas: Lambda+Function URL temporária no stage dev, bucket S3 com website, ou container local exposto, desde que documentado em `tests/load/README.md` e parametrizado por env var `MOCK_UPSTREAM_URL`. O cenário vast usa `vcurl=base64(MOCK_UPSTREAM_URL/vast.xml)`.

### 4. Thresholds (falham a execução automaticamente)

Conforme metas de [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §6:

```javascript
thresholds: {
  // Rotas SEM upstream: p99 < 150ms e p50 < 20ms (medidos no lado servidor; ver nota de latência abaixo)
  'http_req_duration{route:adtrack}':       ['p(99)<150', 'p(50)<20'],
  'http_req_duration{route:vasttrack}':     ['p(99)<150', 'p(50)<20'],
  'http_req_duration{route:redirect}':      ['p(99)<150', 'p(50)<20'],
  'http_req_duration{route:ad}':            ['p(99)<150', 'p(50)<20'],
  'http_req_duration{route:health}':        ['p(99)<150'],
  // Rotas COM upstream (vast fluxo C, proxy-tracker, trackingpixel): threshold relaxado, documentado
  'http_req_duration{route:vast}':          ['p(99)<1000'],
  'http_req_duration{route:proxy-tracker}': ['p(99)<1000'],
  'http_req_duration{route:trackingpixel}': ['p(99)<1000'],   // legado baixa a imagem a cada request
  // Taxa de erro global < 0,1%
  'http_req_failed': ['rate<0.001'],
  'checks':          ['rate>0.999'],
}
```

Nota de latência: `http_req_duration` do k6 inclui a rede até o API Gateway. Registrar TAMBÉM a latência server-side (métrica EMF `RequestLatency` da M7-02 / CloudWatch `Duration`) no relatório — a meta p99<150ms do legado é avaliada primariamente server-side; o threshold k6 é o guard-rail automatizado.

`checks` por rota: status esperado (`201` adtrack, `200` vast/vasttrack/redirect/ad/health), content-type (`text/xml` no vast, `text/javascript` no ad, `text/html` no redirect) e corpo não-vazio.

### 5. Execução e relatório

- `make load-smoke` → `k6 run tests/load/smoke.js -e BASE_URL=... -e MOCK_UPSTREAM_URL=... -e HID_TEST=... -e CID_TEST=...`
- `make load-full` → idem com `mix.js`; exige confirmação explícita (variável `CONFIRMO_CARGA=sim`) para evitar disparo acidental.
- `handleSummary()` exporta `tests/load/results/<data>-summary.json` + resumo markdown comparando cada métrica medida com a meta do docs/legado/05 §6 (tabela: métrica | meta | medido | veredito). Resultados NÃO são commitados (gitignore em `tests/load/results/`), mas o resumo markdown da execução de aceite vai no PR.
- Durante a carga completa, observar os dashboards M7-03 e registrar no relatório: throttles Lambda, `ApproximateAgeOfOldestMessage` da tracking-queue, conexões do RDS Proxy, erros 5xx do API Gateway.

### 6. CI

- Job `load-smoke` no workflow de deploy dev (`.github/workflows/deploy.yml`), após o deploy + smoke funcional existente: instala k6 (action `grafana/setup-k6-action` ou binário fixado por versão), roda `smoke.js` por 30s contra o stage dev; thresholds violados falham o job.
- A carga completa NUNCA roda no CI (custo + escrita massiva em dev) — somente via `make load-full` com supervisão humana.

## Arquivos a criar/alterar

- `tests/load/mix.js` — cenários de carga completa (4 grupos, rampa até 250 rps, thresholds §4)
- `tests/load/smoke.js` — smoke 30s para o CI (mesmo mix, taxa baixa)
- `tests/load/lib/routes.js` — builders de URL/payload por rota (base64 helper, gerador de `time` 17 dígitos, sequência de eventos de vídeo) — comentado em português
- `tests/load/lib/config.js` — leitura das env vars (`BASE_URL`, `MOCK_UPSTREAM_URL`, `HID_TEST`, `CID_TEST`), validação de presença
- `tests/load/README.md` — como rodar, pré-requisitos, mock upstream, interpretação do relatório, checklist de monitoramento durante a carga
- `tests/load/results/.gitignore` — ignora resultados
- `Makefile` — targets `load-smoke` e `load-full` (com trava `CONFIRMO_CARGA`)
- `.github/workflows/deploy.yml` — job `load-smoke` pós-deploy dev

## Critérios de aceite

- [ ] Mix implementado com cenários separados e tags por rota: vast ~40%, tracking ~40% (adtrack/vasttrack/trackingpixel), demais ~20%
- [ ] Rampa até 250 rps sustentados por 10 min contra o stage dev SEM violar thresholds (evidência: summary JSON + resumo markdown no PR)
- [ ] Thresholds automáticos: p99<150ms e p50<20ms nas rotas sem upstream; `http_req_failed rate<0.001`; execução falha sozinha se violados
- [ ] Cenário vast NÃO chama parceiros reais (fluxo B + mock upstream parametrizado) — verificável no código
- [ ] Relatório compara cada métrica com a tabela de metas do docs/legado/05 §6 (inclui latência server-side via CloudWatch/EMF, throttles, idade SQS, conexões RDS Proxy)
- [ ] Smoke de 30s rodando no CI após deploy dev, com thresholds ativos (evidência: run verde no Actions)
- [ ] `make load-full` exige `CONFIRMO_CARGA=sim`; nenhum segredo/IP hardcoded (tudo via env vars)
- [ ] `tests/load/README.md` permite a qualquer pessoa repetir a execução completa
- [ ] Scripts k6 comentados em português (CODE_DOCS_POLICY.md)

## Dependências

Bloqueada por: M3-* (tracking), M4-* (ad serving) e M5-* (VAST & proxies) — todos os handlers do mix implantados e funcionais no stage dev. Recomendado: M7-02/M7-03 (métricas EMF e dashboards) para a leitura server-side do relatório.

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §6 (metas: ~23 rps médio, ≥10×, p50<20ms, p99<150ms, cold start <100ms)
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) (parâmetros exatos por rota; §4 — vast ≈40% do tráfego)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §4 (`tests/load/`), §6 (estimativas de performance)
- [docs/issues/M7-04-alarmes-sns.md](M7-04-alarmes-sns.md) (alarme A4 — RDS Proxy; A5 — throttles; A7 — idade SQS)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Riscos (conexões do MySQL compartilhado)
- [k6 — ramping-arrival-rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ramping-arrival-rate/) · [Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M8-02] Testes de carga k6 (10× volume) seguindo
docs/issues/M8-02-k6-load-tests.md e CLAUDE.md. Criar tests/load/{mix.js,
smoke.js,lib/routes.js,lib/config.js,README.md} com o mix vast ~40% /
tracking ~40% / demais ~20% em cenários ramping-arrival-rate com tags por
rota, rampa até 250 rps, thresholds p99<150ms e p50<20ms nas rotas sem
upstream e http_req_failed rate<0.001, mock upstream para vast/proxy (sem
parceiros reais), handleSummary comparando com as metas do
docs/legado/05 §6, targets make load-smoke/load-full (trava
CONFIRMO_CARGA) e job load-smoke de 30s no deploy dev. Scripts comentados
em português, executar smoke contra dev com evidência, abrir PR
referenciando a issue.
```
