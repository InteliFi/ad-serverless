---
title: "[M0-04] Pipeline de deploy (dev automático, prod com aprovação manual)"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P0"]
milestone: "M0 — Fundação"
---
## Contexto

Com CI (M0-03) e `serverless.yml` (M0-02) prontos, falta o pipeline de **entrega**: deploy automático em dev após merge e deploy em prod gateado por aprovação humana. Substitui o fluxo legado de `ci/build.sh -dev|-prod` via scp/ssh sem rollback ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 e §6).

O fluxo segue a decisão de CI/CD da [ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1: "lint → test → build → deploy dev → smoke → deploy prod manual", autenticado por OIDC (roles criadas na M0-03).

## Especificação detalhada

### 1. Environments no GitHub (configuração do repo, documentar no runbook)
- `dev` — sem proteção; variável `AWS_ROLE_ARN = arn:aws:iam::<ACCOUNT_ID>:role/gha-ad-serverless-dev`, `AWS_REGION = us-east-1`.
- `prod` — **required reviewers** (mínimo 1 — engenheiro-chefe) e branch `main` apenas; `AWS_ROLE_ARN = .../gha-ad-serverless-prod`, `AWS_REGION = sa-east-1`.

### 2. `.github/workflows/deploy.yml`
```yaml
name: Deploy
on:
  push:
    branches: [main]      # merge em main => deploy dev automático
  workflow_dispatch: {}     # disparo manual também permitido

permissions:
  id-token: write           # OIDC
  contents: read

concurrency: deploy-${{ github.ref }}   # nunca 2 deploys simultâneos

jobs:
  deploy-dev:
    runs-on: ubuntu-latest
    environment: dev
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - run: npm ci
      - run: make build
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ vars.AWS_ROLE_ARN }}
          aws-region: ${{ vars.AWS_REGION }}
      - run: npx serverless deploy --stage dev --verbose
      - name: Smoke test
        run: |
          URL=$(npx serverless info --stage dev --verbose | grep -oP 'https://[^ ]+execute-api[^ ]+' | head -1)
          curl -fsS --retry 3 --retry-delay 5 "${URL%/}/hello" | grep '"status":"UP"'

  deploy-prod:
    needs: deploy-dev
    runs-on: ubuntu-latest
    environment: prod        # ⇐ pausa aqui até aprovação manual no GitHub
    steps:
      # mesmos passos, com --stage prod e região sa-east-1 (vars do environment prod)
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - run: npm ci
      - run: make build
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ vars.AWS_ROLE_ARN }}
          aws-region: ${{ vars.AWS_REGION }}
      - run: npx serverless deploy --stage prod --verbose
      - name: Smoke test
        run: |
          URL=$(npx serverless info --stage prod --verbose | grep -oP 'https://[^ ]+execute-api[^ ]+' | head -1)
          curl -fsS --retry 3 --retry-delay 5 "${URL%/}/hello" | grep '"status":"UP"'
```
Notas:
- Smoke test usa `/hello` enquanto o hello-handler existir; quando o `ad-handler` (M4-04) entrar com `GET /health`, trocar para `/health` validando `{"status":"UP"}` (deixar comentário TODO no workflow apontando para isso).
- `npm ci` instala o Serverless Framework pinado (ADR-008/M0-02).
- Falha do smoke test FALHA o job — em dev sinaliza problema antes do gate de prod; rollback do Serverless v3 = `npx serverless rollback --timestamp <ts> --stage <stage>` (documentar no runbook).

### 3. Runbook
Criar `docs/runbooks/deploy.md`: como aprovar prod, como acompanhar o CloudFormation, como fazer rollback (`serverless rollback`), e o que fazer se o smoke test falhar.

## Arquivos a criar/alterar

- `.github/workflows/deploy.yml`
- `docs/runbooks/deploy.md`
- Configuração de environments `dev`/`prod` no GitHub (manual — documentada no runbook)

## Critérios de aceite

- [ ] Merge em `main` dispara deploy automático em dev e o smoke test passa
- [ ] Job `deploy-prod` fica em estado "waiting" até aprovação no environment `prod`
- [ ] Após aprovação, deploy prod conclui e smoke test passa (validar com o hello-handler)
- [ ] `concurrency` impede deploys simultâneos
- [ ] Autenticação 100% OIDC (`id-token: write`, `configure-aws-credentials` com role) — zero secrets de chave AWS
- [ ] Runbook de deploy/rollback escrito em português
- [ ] Environments com `AWS_ROLE_ARN`/`AWS_REGION` corretos (dev=us-east-1, prod=sa-east-1)

## Dependências

Bloqueada por: #M0-02, #M0-03

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (pipeline CI/CD)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (deploy legado), §6 (meta "deploy por stage, rollback")
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §1 (formato do health check para o smoke test)
- M0-03 (roles OIDC `gha-ad-serverless-{dev,prod}`)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-04] Pipeline de deploy no repo
InteliFi/ad-serverless. Criar .github/workflows/deploy.yml com deploy
automático em dev após merge em main (OIDC, make build, serverless
deploy --stage dev, smoke test curl validando "status":"UP") e deploy
prod gateado por environment protection com aprovação manual. Criar
docs/runbooks/deploy.md (aprovação, rollback com serverless rollback).
Comentários em português (CODE_DOCS_POLICY.md). CI verde.
Ao final: abrir PR referenciando a issue.
```
