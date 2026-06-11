---
title: "[M11-02] CPL Tracking — leads com deduplicação e webhooks"
labels: ["epic:M11-backlog", "tipo:feature", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

**Nova feature (não existe no legado).** Recebimento estruturado de leads (Cost Per Lead) com deduplicação e notificação a publishers — hoje `POSTBACK_CPL` é só um evento sem dados do lead.

> Aproveitada do planejamento anterior (issue antiga #32), adaptada para Go. Executar somente após M9.

## Especificação detalhada

1. **Nova tabela DynamoDB `Leads`**: PK `lead_id` (UUID); GSI `gsi_campaign` (`campaign_id`, SK `received_at`). Atributos: `campaign_id`, `lead_data` (map JSON flexível), `lead_status` (new|qualified|converted), `lead_value` (N), `source`, `dedup_key`, `received_at`.
2. **Endpoint `POST /lead/receive`**: validação de campanha ativa + assinatura; corpo JSON com dados do lead.
3. **Deduplicação**: `dedup_key = SHA256(campaign_id + email|telefone normalizado)` com **conditional write** numa tabela auxiliar `LeadDedup` (PK `dedup_key`, TTL 24h) — lead duplicado na janela → `200` com `{"duplicated":true}`, sem criar registro.
4. **Webhook configurável** por campanha (URL em SSM/config): notificar o publisher em lead aceito; entrega assíncrona via SQS própria com retry e DLQ (nunca segurar a resposta HTTP).
5. **Métricas EMF**: `leads_recebidos`, `leads_duplicados`, `webhooks_falhos` ([M7-02]).

## Arquivos a criar/alterar

- `internal/leads/` + handler; tabelas e fila no serverless.yml; testes

## Critérios de aceite

- [ ] Dedup: mesmo email/telefone na janela de 24h não cria segundo lead (teste com TTL simulado)
- [ ] Webhook entregue com retry e cai na DLQ após esgotamento (teste com endpoint mock falhando)
- [ ] Assinatura validada; campanha inativa → 404
- [ ] Métricas publicadas

## Dependências

Bloqueada por: M9 completo; recomenda-se após [M11-01] (compartilha padrões)

## Referências

- Issue antiga #32 (origem)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-02] CPL Tracking seguindo docs/issues/M11-02-cpl-tracking.md e CLAUDE.md. Tabela Leads, endpoint POST /lead/receive com dedup por conditional write + TTL e webhooks assíncronos via SQS. Código comentado em português. Abrir PR ao final.
```
