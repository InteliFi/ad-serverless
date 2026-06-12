# Runbook — OIDC GitHub Actions ↔ AWS (provider + roles)

> ⚠️ **Status: REFERÊNCIA — execução OPCIONAL e adiada (ADR-009).**
> A decisão [ADR-009](../arquitetura/ADR-009-deploy-codepipeline-ssm.md)
> moveu o DEPLOY para CodeBuild/CodePipeline dentro da AWS; o CI do GitHub
> (lint/test/build) **não usa credencial AWS nenhuma**. Este runbook cumpre a
> documentação pedida na issue M0-03 e fica como referência caso algum job de
> CI precise, no futuro, de acesso AWS (ex.: publicar relatório em S3).
> **Não execute estes passos como pré-requisito de deploy.**

## O que é

Federação OIDC permite que um job do GitHub Actions assuma uma role IAM por
token de curta duração — **zero access keys estáticas** (o projeto nasce para
eliminar a chave exposta do legado, ver M0-05). Quem executa os comandos
precisa de permissão IAM administrativa na conta.

## 1. Criar o Identity Provider OIDC (1× por conta AWS)

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com
```

Se a conta já tiver o provider (outro projeto criou), reutilizar — o ARN é
`arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com`.

## 2. Trust policies (restritas ao repositório)

### `gha-ad-serverless-dev` — qualquer ref do repo

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "Federated": "arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com" },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": { "token.actions.githubusercontent.com:aud": "sts.amazonaws.com" },
      "StringLike":   { "token.actions.githubusercontent.com:sub": "repo:InteliFi/ad-serverless:*" }
    }
  }]
}
```

### `gha-ad-serverless-prod` — somente o environment `prod` do GitHub

Igual à de dev, trocando a condição `sub`:

```json
"StringLike": { "token.actions.githubusercontent.com:sub": "repo:InteliFi/ad-serverless:environment:prod" }
```

O `sub` restrito impede que forks, outros repos ou branches sem o environment
protegido assumam a role.

### Criar as roles

```bash
aws iam create-role --role-name gha-ad-serverless-dev \
  --assume-role-policy-document file://trust-dev.json
aws iam create-role --role-name gha-ad-serverless-prod \
  --assume-role-policy-document file://trust-prod.json
```

## 3. Permissions policy (o que o Serverless Framework v3 precisa)

Anexar a cada role (ajustar `<ACCOUNT_ID>`; a região de dev é us-east-1 e a
de prod é sa-east-1). Escopo por prefixo `ad-serverless-*` onde o serviço
permite:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "CloudFormationStack",
      "Effect": "Allow",
      "Action": ["cloudformation:*"],
      "Resource": "arn:aws:cloudformation:*:<ACCOUNT_ID>:stack/ad-serverless-*/*"
    },
    {
      "Sid": "CloudFormationRead",
      "Effect": "Allow",
      "Action": ["cloudformation:ValidateTemplate"],
      "Resource": "*"
    },
    {
      "Sid": "DeploymentBucket",
      "Effect": "Allow",
      "Action": ["s3:*"],
      "Resource": ["arn:aws:s3:::ad-serverless-*", "arn:aws:s3:::ad-serverless-*/*"]
    },
    {
      "Sid": "Lambda",
      "Effect": "Allow",
      "Action": ["lambda:*"],
      "Resource": "arn:aws:lambda:*:<ACCOUNT_ID>:function:ad-serverless-*"
    },
    {
      "Sid": "HttpApi",
      "Effect": "Allow",
      "Action": ["apigateway:*"],
      "Resource": "arn:aws:apigateway:*::/*"
    },
    {
      "Sid": "IamRolesDoStack",
      "Effect": "Allow",
      "Action": [
        "iam:GetRole", "iam:CreateRole", "iam:DeleteRole", "iam:TagRole",
        "iam:PutRolePolicy", "iam:DeleteRolePolicy", "iam:GetRolePolicy",
        "iam:AttachRolePolicy", "iam:DetachRolePolicy", "iam:PassRole"
      ],
      "Resource": "arn:aws:iam::<ACCOUNT_ID>:role/ad-serverless-*"
    },
    {
      "Sid": "Logs",
      "Effect": "Allow",
      "Action": ["logs:*"],
      "Resource": "arn:aws:logs:*:<ACCOUNT_ID>:log-group:/aws/lambda/ad-serverless-*"
    },
    {
      "Sid": "RecursosM2",
      "Effect": "Allow",
      "Action": [
        "sqs:*", "events:*", "cloudfront:*",
        "ssm:GetParameter", "ssm:GetParameters",
        "secretsmanager:GetSecretValue", "rds:Describe*"
      ],
      "Resource": "*"
    }
  ]
}
```

> Nota: `RecursosM2` é deliberadamente amplo para não quebrar os recursos do
> M2 (SQS, CloudFront, RDS Proxy); ao executar de verdade, restringir aos
> ARNs reais criados — o runbook deve ser atualizado na mesma hora.

## 4. Uso no workflow (referência)

```yaml
permissions:
  id-token: write   # OIDC — emite o token do job
  contents: read

steps:
  - uses: aws-actions/configure-aws-credentials@v4
    with:
      role-to-assume: arn:aws:iam::<ACCOUNT_ID>:role/gha-ad-serverless-dev
      aws-region: us-east-1
```

## 5. Regras permanentes (valem mesmo com ADR-009)

- **PROIBIDO** criar secrets `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` no
  repositório — em qualquer workflow, para qualquer fim.
- O `ci.yml` atual não tem etapa com AWS; revisar este runbook ANTES de
  adicionar qualquer passo de CI que toque a AWS.
- Deploy é exclusivo do CodePipeline (ADR-009) — não criar workflow de
  deploy no GitHub.
