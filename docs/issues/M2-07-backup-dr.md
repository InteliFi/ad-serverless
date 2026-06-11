---
title: "[M2-07] Backup & disaster recovery (PITR, versioning, plano de DR)"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P1"]
milestone: "M2 — Infra AWS"
---
## Contexto

O legado não tem estratégia formal de backup além dos snapshots automáticos do RDS. Os dados de tracking (a matéria-prima do faturamento) ficam em DynamoDB e MySQL sem plano de recuperação documentado. Esta issue estabelece backup e DR para os recursos da nova arquitetura — **sem tocar no RDS compartilhado** (os backups do RDS continuam como estão; mudanças lá são escopo do Epic M10).

> Ideia aproveitada do planejamento anterior (issue antiga #30), adaptada à arquitetura Go/fase 1.

## Especificação detalhada

1. **DynamoDB `AdTrackers` e `PostbackLogs`**: habilitar **Point-in-Time Recovery (PITR)** (janela 35 dias) — operação aditiva e segura em tabelas existentes; declarar via IaC se possível (tabelas são unmanaged — usar `aws dynamodb update-continuous-backups` documentado + verificação no CI) e documentar custo (~$0,20/GB/mês).
2. **S3 bucket de mídia**: versioning habilitado + lifecycle (versões antigas → expiração em 30 dias; objetos são cache reconstituível, não precisam de Glacier).
3. **SQS**: DLQ com retenção 14 dias (já em [M2-01] — validar aqui).
4. **IaC e código**: o repositório GitHub é a fonte; confirmar que `serverless.yml` reconstrói o ambiente do zero (`serverless deploy` em conta limpa de teste = critério).
5. **Objetivos documentados** em `docs/infra/DR.md`:
   - **RPO**: ≤ 5 min para tracking (PITR contínuo); cache de mídia: sem RPO (reconstituível).
   - **RTO**: ≤ 1h para redeploy completo das Lambdas+API em região alternativa (runbook passo a passo: `serverless deploy --region`, SSM params, DNS).
   - Cenários cobertos: deleção acidental de tabela, corrupção de dados (restore PITR), indisponibilidade regional (decisão consciente: aceitar downtime regional na fase 1 — multi-região é melhoria futura; registrar o risco).
6. **Teste de restore**: executar 1 restore PITR real da `AdTrackers` para tabela temporária em dev, validar integridade e documentar o tempo medido no DR.md.

## Arquivos a criar/alterar

- `docs/infra/DR.md` — objetivos, procedimentos, resultados do teste
- `serverless.yml` — versioning/lifecycle do S3
- Script/documentação do PITR nas tabelas existentes

## Critérios de aceite

- [ ] PITR ativo nas duas tabelas DynamoDB (evidência via `describe-continuous-backups`)
- [ ] S3 com versioning + lifecycle via IaC
- [ ] Restore PITR testado em dev com tempo registrado
- [ ] DR.md com RPO/RTO e runbook de recuperação regional
- [ ] Nenhuma alteração no RDS compartilhado

## Dependências

Bloqueada por: [M2-01], [M2-02]

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §4 (tabelas DynamoDB existentes)
- Issue antiga #30 do planejamento Java+CDK (origem da ideia)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-07] Backup & DR seguindo docs/issues/M2-07-backup-dr.md e CLAUDE.md. PITR nas tabelas DynamoDB existentes, versioning S3, teste real de restore em dev e docs/infra/DR.md com RPO/RTO. Sem tocar no RDS compartilhado. Abrir PR ao final.
```
