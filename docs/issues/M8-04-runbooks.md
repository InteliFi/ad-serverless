---
title: "[M8-04] Runbooks operacionais"
labels: ["epic:M8-qualidade", "tipo:docs", "prioridade:P1"]
milestone: "M8 — Qualidade"
---
## Contexto

O sistema legado não tem documentação operacional — incidentes dependem de memória tribal. Para operar 2M req/dia em produção serverless (e especialmente durante o cutover M9), cada cenário de incidente precisa de um procedimento escrito, testado e apontado pelos alarmes ([M7-04] referencia o runbook na mensagem do alarme).

## Especificação detalhada

Criar `docs/runbooks/` com um arquivo por cenário, todos no formato: **Sintoma → Pré-condições/acessos → Diagnóstico passo a passo → Ação corretiva → Validação → Escalonamento**.

1. **`latencia-alta.md`** — p99 acima da meta: diagnóstico via X-Ray (segmento mais lento: upstream parceiro? RDS Proxy? cold start?); ações por causa (aumentar memória da função, provisioned concurrency temporária no vast-handler, contato com parceiro).
2. **`dlq-redrive.md`** — mensagens na DLQ de tracking: inspecionar amostra (`aws sqs receive-message`), classificar causa (payload inválido × falha de dependência), corrigir e reprocessar com `aws sqs start-message-move-task --source-arn <dlq> --destination-arn <fila>`; validar reconciliação de contagens depois.
3. **`rollback-deploy.md`** — deploy ruim: `serverless rollback --timestamp <ts> --stage prod` (ou redeploy da tag anterior via Actions); durante o cutover, reversão de pesos CloudFront/Route53 ([M9-03]) como rollback de 1 ação; critérios para escolher cada um.
4. **`rds-proxy-degradado.md`** — erros de conexão MySQL: verificar métricas do proxy (conexões, borrowed), conexões dos OUTROS projetos no RDS (⚠️ banco compartilhado — nunca matar conexões alheias), failover do RDS; mitigação: reduzir concorrência das Lambdas com reserved concurrency.
5. **`parceiro-upstream-fora.md`** — SmartAdServer/Metrike/modatta/prezao fora: impacto por rota (tabela rota×parceiro), comportamento esperado (vast 502/504, postback 202 com WARN), quando acionar o parceiro, mitigações (nenhum circuit breaker na fase 1 — documentar como melhoria).
6. **`rotacao-segredos.md`** — rotacionar DSN MySQL/signature key no SSM: ordem (criar versão nova → atualizar parâmetro → containers pegam na próxima reciclagem → forçar com redeploy), validação, rollback.

Cada runbook testado em dev (executar o procedimento real pelo menos 1×) com a data do teste registrada no rodapé.

## Arquivos a criar/alterar

- `docs/runbooks/latencia-alta.md`, `dlq-redrive.md`, `rollback-deploy.md`, `rds-proxy-degradado.md`, `parceiro-upstream-fora.md`, `rotacao-segredos.md`
- `docs/runbooks/README.md` — índice com mapeamento alarme→runbook

## Critérios de aceite

- [ ] 6 runbooks no formato padrão, em português
- [ ] Cada um executado/ensaiado em dev com data registrada
- [ ] Alarmes de [M7-04] linkam o runbook correspondente na descrição
- [ ] Revisão do engenheiro-chefe no runbook de RDS Proxy (banco compartilhado)

## Dependências

Bloqueada por: epic M7 (alarmes e dashboards existentes)

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) (topologia e operação atual)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) (riscos e mitigações)
- Issues [M7-04], [M9-03]

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M8-04] Runbooks operacionais seguindo docs/issues/M8-04-runbooks.md e CLAUDE.md. Criar os 6 runbooks no formato Sintoma→Diagnóstico→Ação→Validação→Escalonamento, com comandos AWS reais, índice com mapeamento alarme→runbook. Tudo em português. Abrir PR ao final.
```
