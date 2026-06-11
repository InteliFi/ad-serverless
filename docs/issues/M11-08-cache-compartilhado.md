---
title: "[M11-08] Avaliação de cache compartilhado (DAX/ElastiCache) + cache de VAST upstream"
labels: ["epic:M11-backlog", "tipo:decisao", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

A fase 1 usa apenas cache em memória por container ([M1-03], paridade com o Caffeine do legado). Com N containers Lambda, cada um aquece o próprio cache — sob escala, isso multiplica leituras no MySQL/RDS Proxy. Esta issue **avalia com dados reais** se um tier de cache compartilhado se justifica, e se vale cachear resposta de VAST upstream (que o legado nunca cacheou).

> Aproveitada do planejamento anterior (issues antigas #7 — ElastiCache 3 tiers — e #26 — cache de VAST upstream 5s). É uma issue de DECISÃO: produz ADR, não implementação direta.

## Especificação detalhada

1. **Coletar dados** (2+ semanas de produção pós-cutover, métricas de [M7-02]):
   - Taxa de hit do cache local por tipo (hotspots, override);
   - QPS no RDS Proxy e % da capacidade;
   - Cardinalidade real de containers simultâneos;
   - Repetição de URLs de VAST upstream (mesma URL em janela de 5s?).
2. **Avaliar opções** com custo mensal estimado:
   - Status quo (cache local) — baseline;
   - **DAX** na frente do DynamoDB (se leituras de DynamoDB virarem gargalo);
   - **ElastiCache Redis/Valkey serverless** para hotspots/campanhas compartilhados;
   - **Cache de VAST upstream** (TTL 5s, chave = URL final com macros expandidas) — ⚠️ atenção: cachebuster `t={ms}` torna a URL única; só faz sentido cachear normalizando a chave SEM os params dinâmicos, e isso muda contadores de impressão dos parceiros — analisar risco de discrepância de billing ANTES.
3. **Entregável**: `docs/arquitetura/ADR-009-cache-compartilhado.md` com decisão, dados e plano de implementação (se aprovada, vira issues novas).

## Critérios de aceite

- [ ] ADR-009 com dados reais de produção, análise de custo e decisão fundamentada
- [ ] Risco de billing de parceiros no cache de VAST analisado explicitamente
- [ ] Se decisão = implementar: issues de implementação criadas e linkadas

## Dependências

Bloqueada por: M9 completo + 2 semanas de métricas ([M7-02])

## Referências

- Issues antigas #7 e #26 (origem das ideias)
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §9 (caches atuais — VAST nunca foi cacheado)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-08] seguindo docs/issues/M11-08-cache-compartilhado.md e CLAUDE.md. Coletar as métricas listadas, comparar as opções com custos e produzir o ADR-009 com a decisão. Abrir PR ao final.
```
