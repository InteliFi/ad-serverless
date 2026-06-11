---
title: "[M11-03] Analytics avançado — dashboards de negócio e export multi-formato"
labels: ["epic:M11-backlog", "tipo:feature", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

**Nova feature.** O relatório atual ([M6-01]/[M6-02]) replica o legado (agregado fixo + XLS). Esta feature adiciona analytics de negócio: visão por período, por campanha, por hotspot, tendências e exports com filtros.

> Aproveitada do planejamento anterior (issues antigas #33 e #37 — dashboards de negócio), adaptada para Go. Executar somente após M9.

## Especificação detalhada

1. **Novos endpoints** (report-handler ou Lambda `analytics`):
   - `GET /analytics/overview?from&to` — impressions, clicks, CTR, video starts/completes, conversões, por dia.
   - `GET /analytics/campaigns/{id}?from&to` — série temporal da campanha + breakdown por hotspot.
   - `GET /analytics/hotspots/{code}?from&to` — performance do ponto WiFi.
   - `GET /analytics/export/csv` e `/export/xls` — com filtros campaign/date/event_type.
2. **Fonte de dados**: MySQL `ad_trackers` com GROUP BY indexado (`overview_idx`) na fase inicial; reavaliar fonte (DynamoDB/derivados) conforme decisão do Epic M10 — a implementação isola a fonte atrás de interface `AnalyticsRepository`.
3. **Dashboard CloudWatch de negócio** (complementa [M7-03]): impressions/dia, CTR, fill rate de VAST (% de /vast com anúncio válido), top 10 campanhas — widgets sobre as métricas EMF.
4. **Proteção**: endpoints novos exigem autenticação (API key via API Gateway authorizer simples) — diferente dos endpoints públicos de serving; documentar.

## Arquivos a criar/alterar

- `internal/analytics/` + handler + testes; rotas e authorizer no serverless.yml; widgets de negócio no dashboard

## Critérios de aceite

- [ ] 5 endpoints com testes (dados de fixture) e autenticação
- [ ] Fonte de dados isolada atrás de interface
- [ ] Dashboard de negócio publicado
- [ ] Sem impacto de performance no hot path (queries só nas réplicas/índices, timeout próprio)

## Dependências

Bloqueada por: M9 completo; usa [M6-01] e [M7-02]

## Referências

- Issues antigas #33 e #37 (origem)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-03] Analytics avançado seguindo docs/issues/M11-03-analytics-avancado.md e CLAUDE.md. Endpoints de overview/campanha/hotspot/export com autenticação, fonte isolada por interface e dashboard de negócio. Código comentado em português. Abrir PR ao final.
```
