---
title: "[M0-04] Pipeline de deploy (dev automático, prod com aprovação manual)"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P0"]
milestone: "M0 — Fundação"
---
## Contexto

> ♻️ **Reescrita em 2026-06-12 (ADR-009):** o deploy NÃO usa GitHub Actions.
> Decisão do engenheiro-chefe registrada em
> [ADR-009](../arquitetura/ADR-009-deploy-codepipeline-ssm.md): entrega via
> **AWS CodePipeline + CodeBuild**, segredos no **SSM Parameter Store**,
> fonte GitHub via **CodeConnections**. O CI de integração (lint/test/build)
> permanece no GitHub Actions (M0-03). A versão anterior desta issue
> (deploy.yml + OIDC) está no histórico do Git.

Com CI (M0-03) e `serverless.yml` (M0-02) prontos, falta o pipeline de
**entrega**: deploy automático em dev após merge e deploy em prod gateado por
aprovação humana. Substitui o fluxo legado de `ci/build.sh -dev|-prod` via
scp/ssh sem rollback ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 e §6).

Esta issue também cumpre a pendência herdada da M0-02: o **primeiro deploy
real do stage dev** (hello-handler) acontece pelo primeiro run do pipeline,
com smoke test `curl /hello` validando `"status":"UP"`.

## Especificação detalhada

### 1. `buildspec.yml` (raiz do repo)
Um único buildspec parametrizado por `STAGE` (dev|prod): instala Go pinado +
Node 20, `npm ci` (Serverless v3 do lockfile — ADR-008), `make build`,
`npx serverless deploy --stage $STAGE` e smoke test
(`curl .../hello` + grep `"status":"UP"`). Comentários em português.

### 2. `infra/pipeline.yml` (CloudFormation, criado 1× em us-east-1)
- **Source:** GitHub `InteliFi/ad-serverless` branch `main` via
  CodeConnections (`DetectChanges: true`) — ARN da conexão como parâmetro,
  lido de `/ad-serverless/pipeline/github-connection-arn`.
- **DeployDev:** CodeBuild `ad-serverless-deploy-dev` (ARM small,
  `STAGE=dev`).
- **ApprovalProd:** aprovação manual no console (gate de produção).
- **DeployProd:** CodeBuild `ad-serverless-deploy-prod` (`STAGE=prod`;
  região do stack vem do serverless.yml — sa-east-1).
- Roles dedicadas: `ad-serverless-pipeline-role` (orquestração) e
  `ad-serverless-codebuild-deploy-role` (permissões do Serverless Framework
  com escopo `ad-serverless-*` + leitura SSM `/ad-serverless/*`).
- Bucket de artefatos com bloqueio público e expiração 30d.

### 3. Runbook `docs/runbooks/deploy.md`
Pré-requisitos (SSM + conexão GitHub), criação do pipeline
(`aws cloudformation deploy`), como aprovar prod, smoke test falhou,
rollback (`serverless rollback` e revert via Git), disparo manual.

## Arquivos a criar/alterar

- `buildspec.yml`
- `infra/pipeline.yml`
- `docs/runbooks/deploy.md`
- Infra manual (1×, documentada no runbook): conexão CodeConnections +
  `aws cloudformation deploy` do pipeline

## Critérios de aceite

- [ ] `buildspec.yml` parametrizado por STAGE com build, deploy e smoke test
- [ ] `infra/pipeline.yml` com os 4 estágios (Source → DeployDev →
      ApprovalProd → DeployProd) e roles de escopo mínimo
- [ ] Pipeline criado na conta (us-east-1) e conectado ao repo via
      CodeConnections
- [ ] Merge em `main` dispara deploy automático em dev e o smoke test passa
      (cumpre a pendência da M0-02: `curl /hello` → `200` `"status":"UP"`)
- [ ] Estágio `ApprovalProd` segura o deploy de prod até aprovação manual
- [ ] Após aprovação, deploy prod conclui e smoke test passa
- [ ] Zero chaves estáticas: service roles + SSM (ADR-009); nenhum secret
      AWS no GitHub
- [ ] Runbook de deploy/rollback escrito em português

## Dependências

Bloqueada por: #M0-02, #M0-03. Infra manual prévia: conexão CodeConnections
autorizada e parâmetro `/ad-serverless/pipeline/github-connection-arn`
(runbook [ssm-parametros.md](../runbooks/ssm-parametros.md)).

## Referências

- [docs/arquitetura/ADR-009-deploy-codepipeline-ssm.md](../arquitetura/ADR-009-deploy-codepipeline-ssm.md)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (pipeline CI/CD)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (deploy legado), §6 (metas)
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §1 (formato do health para o smoke)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-04] Pipeline de deploy (modelo ADR-009,
CodePipeline) no repo InteliFi/ad-serverless: buildspec.yml parametrizado
por STAGE (build, serverless deploy, smoke test "status":"UP"),
infra/pipeline.yml (Source CodeConnections → DeployDev → ApprovalProd →
DeployProd, roles de escopo mínimo) e docs/runbooks/deploy.md (criação,
aprovação, rollback). Comentários em português (CODE_DOCS_POLICY.md).
CI verde. Ao final: abrir PR referenciando a issue.
```
