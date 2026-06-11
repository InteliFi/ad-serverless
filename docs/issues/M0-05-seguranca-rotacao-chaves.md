---
title: "[M0-05] 🔴 Segurança: rotação das chaves AWS expostas + SSM bootstrap"
labels: ["epic:M0-fundacao", "tipo:infra", "tipo:seguranca", "prioridade:P0"]
milestone: "M0 — Fundação"
---
## Contexto

🔴 **A access key AWS `AKIAVR67P7UR7PR2J6QC` está em texto plano no `application.properties` do ad-server Java — e a MESMA chave é usada em dev E prod.** Junto dela estão hardcoded as senhas do MySQL (root em dev; `adserver_dml`/`adserver_ddl` em prod) e a `intv.ad.signaturekey`. Ver [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2 ("SEGREDOS HARDCODED") e [ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §7.1 ("Rotacionar IMEDIATAMENTE").

⚠️ **NÃO desativar a chave de imediato:** as EC2 de produção do legado a usam (DynamoDB `AdTrackers`/`PostbackLogs` em sa-east-1). A rotação tem que ser coordenada com o time para não derrubar o tracking em produção.

Esta issue também faz o **bootstrap do SSM Parameter Store** — pré-requisito da M2-03 (RDS Proxy) e de todo handler que precisa de segredo.

## Especificação detalhada

### Fase 1 — Inventário (sem mudar nada)
1. Levantar onde a chave é usada:
```bash
aws iam get-access-key-last-used --access-key-id AKIAVR67P7UR7PR2J6QC
aws iam list-access-keys --user-name <usuario-dono-da-chave>
# CloudTrail (últimos 90 dias) para mapear serviços/regiões chamados:
aws cloudtrail lookup-events --lookup-attributes AttributeKey=AccessKeyId,AttributeValue=AKIAVR67P7UR7PR2J6QC --max-results 50 --region sa-east-1
```
2. Confirmar consumidores conhecidos: EC2 dev `i-0267248b971ac7cd8` (us-east-1) e EC2 prod `i-030bd120418d71a9d` + `i-0707c9d77d0420be3` (sa-east-1) via `application*.properties`. Documentar qualquer uso INESPERADO encontrado no CloudTrail antes de prosseguir.

### Fase 2 — Nova credencial de escopo mínimo SÓ para o legado
3. Criar usuário IAM dedicado `adserver-legacy-ec2` com policy mínima (somente o que o Java usa — DynamoDB nas 2 tabelas):
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["dynamodb:PutItem", "dynamodb:GetItem", "dynamodb:Query", "dynamodb:BatchWriteItem"],
    "Resource": [
      "arn:aws:dynamodb:sa-east-1:<ACCOUNT_ID>:table/AdTrackers",
      "arn:aws:dynamodb:sa-east-1:<ACCOUNT_ID>:table/PostbackLogs"
    ]
  }]
}
```
(Ajustar a lista de actions ao que o CloudTrail da Fase 1 mostrar — mínimo real.)
4. `aws iam create-access-key --user-name adserver-legacy-ec2` e **entregar ao responsável pelo deploy do legado** atualizar `application*.properties` nas EC2 (dev primeiro, prod depois) e validar tracking funcionando (item DynamoDB novo em `AdTrackers`).

### Fase 3 — Rotação da chave exposta (COORDENAR COM O TIME)
5. Após confirmação de que dev e prod rodam com a chave nova: `aws iam update-access-key --access-key-id AKIAVR67P7UR7PR2J6QC --status Inactive`. Observar por 48h (alarme/erros no legado). Só então `aws iam delete-access-key`.
6. Rotacionar também as senhas MySQL hardcoded (root dev, `adserver_dml`/`adserver_ddl` prod) **com o DBA** — mesma janela coordenada, pois exigem atualizar as EC2.

### Fase 4 — SSM Parameter Store bootstrap (para as Lambdas)
7. Criar SecureStrings nas DUAS regiões/stages (dev em us-east-1, prod em sa-east-1):
```bash
aws ssm put-parameter --name /ad-serverless/dev/mysql-dsn  --type SecureString --value '<DSN dev — credencial NOVA, nunca a exposta>' --region us-east-1
aws ssm put-parameter --name /ad-serverless/dev/signature-key --type SecureString --value '<nova signature key>' --region us-east-1
aws ssm put-parameter --name /ad-serverless/prod/mysql-dsn --type SecureString --value '<DSN prod>' --region sa-east-1
aws ssm put-parameter --name /ad-serverless/prod/signature-key --type SecureString --value '<nova signature key>' --region sa-east-1
```
- O DSN de prod aguarda o usuário `adserverless_app` (M2-03); preencher placeholder e atualizar depois.
- A `signature-key` legada está comprometida → gerar valor NOVO (a validação está comentada no Java, ver [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §8, então trocar não quebra nada).
8. As Lambdas usarão **IAM Roles** (M2-04) — nenhuma access key será criada para o ad-serverless.

### Fase 5 — Runbook
9. Escrever `docs/runbooks/rotacao-chaves.md`: cronologia executada, datas, quem aprovou, como repetir uma rotação futura, e checklist de verificação pós-rotação.

## Arquivos a criar/alterar

- `docs/runbooks/rotacao-chaves.md`
- Parâmetros SSM `/ad-serverless/{dev,prod}/mysql-dsn` e `/ad-serverless/{dev,prod}/signature-key` (infra, via aws cli)
- Usuário IAM `adserver-legacy-ec2` + policy mínima (infra)
- NENHUM código Go nesta issue

## Critérios de aceite

- [ ] Inventário CloudTrail da chave `AKIAVR67P7UR7PR2J6QC` documentado no runbook (serviços, regiões, origens)
- [ ] Usuário `adserver-legacy-ec2` criado com policy restrita às tabelas `AdTrackers` e `PostbackLogs` (sa-east-1)
- [ ] EC2 dev e prod do legado operando com a chave nova (tracking validado no DynamoDB)
- [ ] Chave `AKIAVR67P7UR7PR2J6QC` com status `Inactive` e, após 48h sem incidentes, deletada
- [ ] 4 parâmetros SSM SecureString criados (dev/us-east-1 e prod/sa-east-1)
- [ ] Signature key NOVA gerada (valor legado descartado)
- [ ] Runbook completo em português com aprovações registradas
- [ ] Confirmado: nenhuma access key criada para o ad-serverless (somente IAM Roles)

## Dependências

Bloqueada por: nenhuma (⚠️ executar o quanto antes; M2-03 depende desta issue)

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (EC2/IDs), §2 (segredos hardcoded)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §7 (segurança)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — risco "Chave AWS exposta no repo legado"
- Legado: `ad-server/src/main/resources/application.properties`, `application-dev.properties`, `application-prod.properties`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-05] Segurança: rotação das chaves AWS
expostas + SSM bootstrap no repo InteliFi/ad-serverless. Seguir as 5
fases da especificação NA ORDEM (inventário CloudTrail, credencial
mínima para o legado, rotação coordenada — NUNCA desativar a chave
antiga antes da confirmação do time, SSM SecureStrings
/ad-serverless/{stage}/mysql-dsn e /signature-key, runbook).
Documentação em português (CODE_DOCS_POLICY.md). Ao final: abrir PR
com o runbook referenciando a issue e listar as ações de infra
executadas/pendentes de aprovação humana.
```
