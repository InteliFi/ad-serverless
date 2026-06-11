---
title: "[M0-03] CI GitHub Actions (lint, test, build) + OIDC AWS"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P0"]
milestone: "M0 — Fundação"
---
## Contexto

O legado NÃO tem CI/CD: deploy é `ci/build.sh` manual via scp/ssh ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1). A meta não-funcional da migração (§6 do mesmo doc) é "GitHub Actions, deploy por stage, rollback automático". Esta issue cria o pipeline de **integração** (lint → test → build); o pipeline de **deploy** é a M0-04.

Decisão de segurança da arquitetura ([ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 e §7.4): autenticação AWS via **OIDC do GitHub Actions** — nenhuma access key estática em secrets do repo (o projeto nasce justamente para eliminar a chave exposta do legado, ver M0-05).

## Especificação detalhada

### 1. `.github/workflows/ci.yml`
Disparo: `pull_request` (qualquer branch) e `push` em `main`.
```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - uses: golangci/golangci-lint-action@v6
        with: { version: v1.64 }   # pinar versão compatível com o .golangci.yml

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: go test ./... -coverprofile=coverage.out -covermode=atomic
      - run: go tool cover -func=coverage.out | tail -1   # imprime cobertura total no log
      - uses: actions/upload-artifact@v4
        with: { name: coverage, path: coverage.out }

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: make build      # GOOS=linux GOARCH=arm64 CGO_ENABLED=0 -tags lambda.norpc, todos os cmd/*
```

### 2. OIDC AWS (preparação para a M0-04 — criar agora)
Documentar em `docs/runbooks/oidc-github-aws.md` e executar (humano com admin) os passos:

a) Criar o Identity Provider OIDC (1× por conta AWS):
```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com
```

b) Criar 1 role por stage — `gha-ad-serverless-dev` e `gha-ad-serverless-prod` — com trust policy restrita ao repo (e, para prod, ao environment do GitHub):
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "Federated": "arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com" },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": { "token.actions.githubusercontent.com:aud": "sts.amazonaws.com" },
      "StringLike": { "token.actions.githubusercontent.com:sub": "repo:InteliFi/ad-serverless:*" }
    }
  }]
}
```
Para `gha-ad-serverless-prod`, trocar o `sub` por `repo:InteliFi/ad-serverless:environment:prod`.

c) Permissions policy das roles: o necessário para o Serverless Framework v3 (CloudFormation, S3 do deployment bucket, Lambda, API Gateway, IAM PassRole/criação de roles do stack, Logs, e os serviços dos recursos do M2: SQS, S3, CloudFront, SSM, Secrets Manager, RDS — escopo por prefixo `ad-serverless-*` onde possível). Documentar a policy completa no runbook.

d) Uso no workflow (referência para a M0-04):
```yaml
permissions: { id-token: write, contents: read }
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: arn:aws:iam::<ACCOUNT_ID>:role/gha-ad-serverless-dev
    aws-region: us-east-1
```

### 3. Sem chaves estáticas
- PROIBIDO criar secrets `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` no repo. O ci.yml desta issue nem precisa de AWS (só lint/test/build).

## Arquivos a criar/alterar

- `.github/workflows/ci.yml`
- `docs/runbooks/oidc-github-aws.md` (passo a passo + policies completas)

## Critérios de aceite

- [ ] `ci.yml` roda nos PRs e em push no main, com os 3 jobs (lint, test, build) verdes
- [ ] Job de build compila TODOS os binários de `cmd/*` (via `make build`)
- [ ] Cobertura impressa no log do job test e artefato `coverage.out` publicado
- [ ] Versões pinadas: Go 1.24, golangci-lint compatível com `.golangci.yml`
- [ ] OIDC provider criado na conta AWS; roles `gha-ad-serverless-dev` e `gha-ad-serverless-prod` criadas com trust policy restrita a `repo:InteliFi/ad-serverless`
- [ ] Runbook `oidc-github-aws.md` documenta provider, trust policies e permissions policies
- [ ] Nenhum secret de chave AWS estática criado no repositório

## Dependências

Bloqueada por: #M0-01

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (CI/CD), §7.4 (OIDC)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (deploy manual legado), §6 (metas)
- Legado: `ad-server/ci/build.sh`, `ad-server/ci/deploy.sh` (processo substituído)
- https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-03] CI GitHub Actions + OIDC AWS no repo
InteliFi/ad-serverless. Criar .github/workflows/ci.yml (golangci-lint,
go test com cobertura, make build de todos os binários arm64) e o runbook
docs/runbooks/oidc-github-aws.md com comandos aws cli para o provider OIDC
e as roles gha-ad-serverless-dev/prod (trust policy restrita ao repo, sem
chaves estáticas). Código/comentários em português (CODE_DOCS_POLICY.md).
CI verde no PR. Ao final: abrir PR referenciando a issue.
```
