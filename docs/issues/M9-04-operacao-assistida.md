---
title: "[M9-04] Operação assistida + reconciliação diária"
labels: ["epic:M9-cutover", "tipo:teste", "prioridade:P1"]
milestone: "M9 — Cutover"
---
## Contexto

Após o canary chegar a 100% em todas as rotas ([M9-03]), o sistema novo opera por **2 semanas em regime assistido** antes do descomissionamento das EC2 ([M9-05]). O objetivo é provar com dados que nenhum evento de tracking se perde e que a receita por parceiro se mantém — os dois danos silenciosos possíveis desta migração.

## Especificação detalhada

### Script de reconciliação diária (automatizado)

`tools/reconciliacao/main.go` (ou script) executado 1×/dia (EventBridge Schedule → Lambda dedicada ou execução manual documentada):

1. **Contagem por fonte** para o dia anterior (D-1), agrupada por `event_type` e `campaign_id`:
   - MySQL: `SELECT event_type, campaign_id, COUNT(*) FROM ad_trackers WHERE event_date = ? GROUP BY 1,2`
   - DynamoDB `AdTrackers`: contagem via Query por campanha (atributo `event_date`)
   - CloudWatch: métrica EMF de eventos publicados/processados ([M7-02]) + mensagens SQS processadas
2. **Comparações com alarme em desvio > 2%**:
   - MySQL × DynamoDB (paridade da dupla escrita)
   - Publicados (track-handler) × persistidos (tracker-writer) — perda na fila
   - D-1 × mesmo dia da semana pré-migração (baseline capturada antes do canary) — queda de volume por rota
3. **Receita/entrega por parceiro**: impressões e starts de vídeo por parceiro (SmartAdServer, Metrike, AdForce, Space) comparados com a média das 4 semanas anteriores; desvio > 5% → investigar com o runbook `parceiro-upstream-fora.md`.
4. Saída: relatório diário em `docs/cutover/reconciliacao/AAAA-MM-DD.md` + notificação no SNS de alarmes ([M7-04]) quando houver desvio.

### Rotina das 2 semanas

- Revisão diária do relatório (humano) com registro de OK/ação no `docs/cutover/LOG.md`.
- DLQ inspecionada diariamente (deve permanecer vazia).
- Critério de saída para [M9-05]: **14 dias consecutivos** sem desvio não explicado e sem rollback.

## Arquivos a criar/alterar

- `tools/reconciliacao/` — script + agendamento (EventBridge no serverless.yml)
- `docs/cutover/reconciliacao/` — relatórios diários
- `docs/cutover/BASELINE.md` — contagens de referência pré-migração (capturar ANTES do primeiro canary — tarefa desta issue, executável cedo)

## Critérios de aceite

- [ ] Baseline pré-migração capturada e versionada
- [ ] Script de reconciliação rodando diariamente em prod com notificação de desvio
- [ ] 14 dias consecutivos limpos registrados no LOG.md
- [ ] Aprovação explícita do engenheiro-chefe para iniciar [M9-05]

## Dependências

Bloqueada por: [M9-03] (a baseline pode ser capturada antes)

## Referências

- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) (risco de perda de eventos)
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §4 (chaves DynamoDB)
- Issues [M7-02], [M7-04], [M9-03]

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M9-04] Operação assistida seguindo docs/issues/M9-04-operacao-assistida.md e CLAUDE.md. Criar o script de reconciliação diária (MySQL × DynamoDB × CloudWatch × baseline), agendamento e relatórios; capturar a baseline pré-migração. Código comentado em português. Abrir PR ao final.
```
