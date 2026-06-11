---
title: "[M11-06] Event-count frequency cap com DynamoDB conditional writes"
labels: ["epic:M11-backlog", "tipo:feature", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

**Implementa uma feature que o legado declarou mas NUNCA implementou:** as colunas `event_cap`, `event_cap_limit` e `event_cap_hours_limit` existem em `campaigns` desde a migration V11, sem nenhuma lógica por trás ([docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.3). Esta feature limita a quantidade de eventos (ex.: impressões) de uma campanha por janela de horas — controle real de budget/pacing.

> Aproveitada do planejamento anterior (issue antiga #29 — conditional writes), redesenhada para não tocar no MySQL.

## Especificação detalhada

1. **Nova tabela DynamoDB `FrequencyCapCounters`**: PK `campaign_id#janela` (ex.: `42#2026-06-11T15`), atributo `counter` (N), TTL = fim da janela + 1h. Sem tocar no schema MySQL — os limites são lidos das colunas já existentes em `campaigns`.
2. **Verificação atômica** na seleção de campanha ([M1-04]):
   ```
   UpdateItem com ConditionExpression:
     attribute_not_exists(counter) OR counter < :limite
   UpdateExpression: ADD counter :um
   ```
   - Condição falhou → campanha NÃO elegível nesta janela (filtrada antes do sorteio).
   - Campanha sem `event_cap_limit` → comportamento atual (sem verificação, sem custo).
3. **Janela**: derivada de `event_cap_hours_limit` (ex.: 24 → janela diária; 1 → horária), truncada em America/Sao_Paulo.
4. **Performance**: a verificação adiciona 1 write DynamoDB no hot path do `/ad`/`/vast` apenas para campanhas COM cap; medir impacto de latência (meta: +<10ms p99) e proteger com flag de desativação.
5. **Decisão de contagem**: contar no momento da SELEÇÃO (não no tracking) — documentar o trade-off (overcount se o anúncio não renderizar vs. atraso do tracking) num mini-ADR dentro do PR.

## Arquivos a criar/alterar

- `internal/frequencycap/eventcap.go` + integração em `internal/selection`; tabela no serverless.yml; testes (incluindo concorrência: N goroutines simultâneas não excedem o limite)

## Critérios de aceite

- [ ] Limite respeitado sob concorrência (teste com 100 incrementos paralelos e limite 50 → exatamente 50 passam)
- [ ] Campanha sem cap: zero chamadas DynamoDB extras
- [ ] TTL limpa os contadores; flag desliga a feature
- [ ] Latência adicional medida e documentada

## Dependências

Bloqueada por: M9 completo

## Referências

- Issue antiga #29 (origem); [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2.1 (colunas event_cap*)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-06] Event-count frequency cap seguindo docs/issues/M11-06-event-cap-dynamodb.md e CLAUDE.md. Tabela de contadores com conditional write atômico, janelas em America/Sao_Paulo, teste de concorrência e flag de desativação. Código comentado em português. Abrir PR ao final.
```
