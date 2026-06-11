---
title: "[M2-04] IAM roles por função (least privilege)"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P1"]
milestone: "M2 — Infra AWS"
---
## Contexto

O sistema legado usa **uma única access key AWS hardcoded, compartilhada entre dev e prod** ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2) — o anti-padrão que a issue [M0-05] elimina. Na arquitetura serverless, cada uma das 9 Lambdas recebe **sua própria IAM Role** com o mínimo de permissões necessário ([ARQUITETURA-ALVO](../arquitetura/ARQUITETURA-ALVO.md) §7). Nenhuma função deve conseguir tocar um recurso que não usa: se o `redirect-handler` for comprometido, ele não pode ler o banco nem escrever no DynamoDB.

## Especificação detalhada

Configurar no `serverless.yml` uma role por função (`iam.role` por function — NÃO usar a role compartilhada do provider) com as seguintes permissões, sempre por ARN específico (placeholders de stage/região resolvidos pelo framework):

| Função | Permissões |
|---|---|
| `ad-handler` | SSM `GetParameter` em `/ad-serverless/${stage}/*`; VPC (acessa MySQL) |
| `vast-handler` | SSM idem; VPC; S3 `HeadObject/PutObject/GetObject` no bucket de mídia |
| `track-handler` | SQS `SendMessage` apenas na `tracking-queue`; SSM; VPC (pixel busca MySQL) |
| `redirect-handler` | nenhuma além de logs (só gera HTML) |
| `postback-handler` | DynamoDB `PutItem` apenas em `PostbackLogs`; SSM; VPC |
| `proxy-handler` | SSM (config de whitelist) |
| `media-handler` | S3 `HeadObject/GetObject/PutObject` no bucket de mídia |
| `report-handler` | SSM; VPC |
| `tracker-writer` | SQS `ReceiveMessage/DeleteMessage/GetQueueAttributes` na `tracking-queue`; DynamoDB `PutItem` em `AdTrackers`; SSM; VPC |

Permissões comuns a todas: `logs:CreateLogStream/PutLogEvents` (log group próprio), `xray:PutTraceSegments/PutTelemetryRecords`.

Regras:
1. **Proibido** `Resource: "*"` em qualquer statement (exceto X-Ray, que não suporta ARN de recurso).
2. Permissões de VPC (`ec2:CreateNetworkInterface`, `DescribeNetworkInterfaces`, `DeleteNetworkInterface`) **apenas** nas funções marcadas com VPC (as que acessam o MySQL via RDS Proxy — [M2-03]).
3. As tabelas DynamoDB `AdTrackers` e `PostbackLogs` **já existem** (criadas pelo legado) — referenciar ARNs por nome via variável, não criar recursos.
4. Nomes de role determinísticos: `ad-serverless-${stage}-<função>`.

## Arquivos a criar/alterar

- `serverless.yml` — bloco `iam.role` em cada function + `resources` com as policies gerenciadas se necessário
- `docs/arquitetura/IAM.md` — tabela função × permissões (manter sincronizada)

## Critérios de aceite

- [ ] `serverless package` gera CloudFormation com 9 roles distintas, sem `Resource: "*"` (validar com script de grep no template gerado)
- [ ] IAM Access Analyzer sem findings de acesso externo nas roles
- [ ] Teste negativo: invocar `redirect-handler` com código que tenta `PutItem` no DynamoDB → AccessDenied
- [ ] `docs/arquitetura/IAM.md` criado e consistente com o serverless.yml

## Dependências

Bloqueada por: [M0-02]

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §7 (Segurança)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2 (segredos hardcoded do legado)
- Issues relacionadas: [M0-05] (rotação de chaves), [M2-03] (RDS Proxy/VPC)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-04] IAM roles por função seguindo docs/issues/M2-04-iam-roles.md e CLAUDE.md. Uma role por Lambda no serverless.yml, permissões por ARN específico, sem wildcards de recurso. Validar com serverless package + grep no template gerado. Documentar em docs/arquitetura/IAM.md. Abrir PR ao final.
```
