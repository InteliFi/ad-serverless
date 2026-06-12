# Runbook — Deploy do ad-serverless (CodePipeline, ADR-009)

> Fluxo: **merge na `main` → deploy dev automático (us-east-1) + smoke test →
> aprovação manual → deploy prod (sa-east-1) + smoke test.**
> Sem chaves estáticas: CodeBuild usa service role; segredos no SSM.
> Decisão registrada em [ADR-009](../arquitetura/ADR-009-deploy-codepipeline-ssm.md).

## 1. Pré-requisitos (1× — antes do primeiro deploy)

1. Parâmetros SSM cadastrados — ver
   [ssm-parametros.md](ssm-parametros.md) (mysql-dsn e signature-key por
   stage; obrigatórios a partir do M2/M3 — o hello-handler não os usa).
2. Conexão GitHub criada: console AWS → Developer Tools → **Connections** →
   Create connection → GitHub → autorizar a org `InteliFi` (handshake).
   Status precisa ficar `Available`. Registrar o ARN:
   ```bash
   aws ssm put-parameter --region us-east-1 --type String \
     --name /ad-serverless/pipeline/github-connection-arn --value '<ARN>'
   ```

## 2. Criar/atualizar o pipeline (1× e a cada mudança no infra/pipeline.yml)

```bash
aws cloudformation deploy \
  --region us-east-1 \
  --stack-name ad-serverless-pipeline \
  --template-file infra/pipeline.yml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides GithubConnectionArn=$(aws ssm get-parameter \
      --region us-east-1 \
      --name /ad-serverless/pipeline/github-connection-arn \
      --query Parameter.Value --output text)
```

A URL do console sai nos Outputs do stack (`ConsoleDoPipeline`).

## 3. Operação normal

- **Deploy dev:** automático a cada push/merge na `main`. O estágio
  `DeployDev` roda `make build` + `serverless deploy --stage dev` + smoke
  test (`curl .../hello` validando `"status":"UP"`).
- **Aprovar prod:** console do pipeline → estágio `ApprovalProd` →
  **Review** → conferir o smoke de dev e o commit no Source → **Approve**.
  (Quem aprova: engenheiro-chefe — restringir por IAM
  `codepipeline:PutApprovalResult` quando houver mais operadores.)
- **Acompanhar:** cada estágio linka o build do CodeBuild (log completo);
  o stack da aplicação é `ad-serverless-<stage>` no CloudFormation da
  região do stage (dev us-east-1, prod sa-east-1).

## 4. Se o smoke test falhar

O estágio fica vermelho e (em prod) o tráfego JÁ está no código novo — agir:

1. Abrir o log do CodeBuild (link no estágio) — a falha é o `curl`/`grep`.
2. Checar a Lambda: CloudWatch Logs `/aws/lambda/ad-serverless-<stage>-*`.
3. Decidir: corrigir à frente (novo merge) ou **rollback** (abaixo).

## 5. Rollback

Opção A — `serverless rollback` (volta o stack para um deploy anterior):

```bash
npx serverless deploy list --stage prod      # lista timestamps
npx serverless rollback --timestamp <ts> --stage prod
```

Requer credencial com as permissões da BuildRole — na prática, rodar de uma
máquina com perfil admin OU re-executar via pipeline (opção B, preferida).

Opção B — revert no Git (auditável, preferida):

```bash
git revert <commit-ruim> && git push origin main
```

O pipeline redeploya dev automaticamente; aprovar prod normalmente.

## 6. Disparo manual (re-rodar sem novo commit)

Console do pipeline → **Release change** (reprocessa o último commit da
`main` por todos os estágios).

## 7. Smoke test — referência

Validação atual: `GET /hello` → `200` com `"status":"UP"` (hello-handler da
M0-02). **TODO(M4-03):** quando o `ad-handler` entrar com `GET /health`,
trocar o path no `buildspec.yml` (comentário TODO já marcado lá).
