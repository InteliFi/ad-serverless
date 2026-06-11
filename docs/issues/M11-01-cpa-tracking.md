---
title: "[M11-01] CPA Tracking — conversões com attribution window"
labels: ["epic:M11-backlog", "tipo:feature", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

**Nova feature (não existe no legado).** Hoje os postbacks `POSTBACK_CPA` são apenas gravados como eventos; não há entidade de conversão, valor, nem janela de atribuição. Esta feature cria o tracking completo de CPA (Cost Per Acquisition).

> Aproveitada do planejamento anterior (issue antiga #31), adaptada para Go. Executar SOMENTE após o cutover completo (M9) — nunca misturar com a portagem de paridade.

## Especificação detalhada

1. **Nova tabela DynamoDB `Conversions`** (recurso novo, sem impacto no legado):
   - PK `conversion_id` (UUID); GSI `gsi_campaign` (PK `campaign_id`, SK `converted_at`).
   - Atributos: `campaign_id`, `click_id`, `transaction_id`, `conversion_value` (N), `currency`, `conversion_type` (CPA|INSTALL), `source`, `attribution_window` (7d|30d), `converted_at` (ISO8601 São Paulo), `attributed` (bool).
2. **Endpoint `POST /conversion/receive`** (postback-handler ou Lambda nova):
   - Validar campanha ativa (mesma validação do postback atual) e assinatura ([M1-06] — aqui a validação NASCE ativa).
   - **Attribution window**: localizar o clique original (PostbackLogs por `click_id`) e marcar `attributed=true` somente se `converted_at - clicked_at ≤ janela` configurada por campanha (SSM ou coluna futura — decidir no design).
   - Idempotência por `transaction_id` (conditional write `attribute_not_exists`).
3. **Métricas**: EMF `conversoes_recebidas`, `conversoes_atribuidas`, `valor_conversoes` por campanha ([M7-02]).
4. **Relatórios**: incluir contadores de conversão no report ([M6-01]) como colunas novas — sem alterar as existentes.

## Arquivos a criar/alterar

- `internal/conversions/` + handler; recurso DynamoDB no serverless.yml; testes

## Critérios de aceite

- [ ] Conversão registrada com atribuição correta dentro/fora da janela (testes com clock injetado)
- [ ] Idempotência por transaction_id comprovada
- [ ] Assinatura validada (rejeita inválida com 401)
- [ ] Métricas e relatório atualizados

## Dependências

Bloqueada por: M9 completo (cutover)

## Referências

- Issue antiga #31 (origem); [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §8 (postback atual)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-01] CPA Tracking seguindo docs/issues/M11-01-cpa-tracking.md e CLAUDE.md. Nova tabela Conversions, endpoint POST /conversion/receive com attribution window e idempotência, métricas e relatório. Código comentado em português. Abrir PR ao final.
```
