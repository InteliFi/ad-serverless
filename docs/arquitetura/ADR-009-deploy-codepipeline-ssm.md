# ADR-009 — Deploy via CodeBuild/CodePipeline na AWS + segredos no SSM

- **Status:** aceito (decisão do engenheiro-chefe, 2026-06-12)
- **Decisores:** Fabio (engenheiro-chefe)
- **Relação com outros ADRs:** complementa o ADR-008 (Serverless v3 OSS);
  **supersede a parte de DEPLOY** das issues M0-03/M0-04 conforme escritas
  (deploy via GitHub Actions + OIDC). O CI de integração (lint/test/build)
  permanece no GitHub Actions.

## Contexto

As issues M0-03/M0-04 planejavam o deploy via GitHub Actions autenticado por
OIDC (roles `gha-ad-serverless-{dev,prod}`). O engenheiro-chefe decidiu que a
**entrega (deploy) não será feita pelo GitHub**:

1. **Segredos e tokens ficam dentro da AWS**, no SSM Parameter Store —
   versionados (histórico de versões de parâmetro), criptografados (KMS) e
   auditáveis (CloudTrail), sem depender de secrets/environments de um
   sistema externo.
2. As credenciais dos projetos da InteliFi estão sendo **padronizadas no
   SSM** (as do ad-server legado ainda não seguem o padrão e serão
   levantadas e cadastradas — ver runbook
   [ssm-parametros.md](../runbooks/ssm-parametros.md)).
3. O pipeline roda **na mesma conta/regiões dos recursos**, com service
   roles do CodeBuild/CodePipeline (sem chaves estáticas e sem federação
   externa).

## Decisão

1. **CI (integração) continua no GitHub Actions** — `.github/workflows/ci.yml`
   roda lint, testes com cobertura e build em todo PR e push na main.
   Esse workflow **não tem credencial AWS nenhuma** (não precisa).
2. **CD (entrega) será AWS CodePipeline + CodeBuild** (implementação na
   M0-04, que deve ser reescrita):
   - **Source:** GitHub via AWS CodeConnections (handshake único no console;
     gera um ARN de conexão — nenhum token de GitHub em texto plano).
   - **Build/Deploy dev:** projeto CodeBuild (`buildspec.yml` versionado no
     repo) executando `make build` + `npx serverless deploy --stage dev` com
     a service role do CodeBuild (us-east-1), disparado por merge na `main`.
   - **Aprovação manual** (estágio Approval do CodePipeline) antes do
     deploy prod (sa-east-1).
3. **Todos os segredos/tokens em SSM Parameter Store** (SecureString),
   convenção `/ad-serverless/<stage>/<nome>` — inventário completo e
   comandos no runbook [ssm-parametros.md](../runbooks/ssm-parametros.md).
4. As roles OIDC do GitHub (`gha-ad-serverless-{dev,prod}`) **deixam de ser
   pré-requisito de deploy**. O runbook
   [oidc-github-aws.md](../runbooks/oidc-github-aws.md) permanece como
   referência documentada (exigência da issue M0-03) caso algum job de CI
   precise um dia de acesso AWS — sua execução é OPCIONAL e está adiada.

## Consequências

- **M0-04 reescrita:** `buildspec.yml` + definição do pipeline (estágios
  Source → BuildDeployDev → ApprovalProd → DeployProd) substituem o
  `deploy.yml` do GitHub Actions. Os critérios "environments do GitHub" e
  "OIDC" daquela issue caem; entram service roles e CodeConnections.
- **Nenhum secret AWS no GitHub** (nem chave estática, nem role OIDC para
  deploy) — o repo só compila e testa.
- O pipeline fica versionado: `buildspec.yml` no repo; a definição do
  CodePipeline em IaC (incluí-la no `serverless.yml`/CloudFormation na M0-04
  ou criada via runbook com aws cli documentado).
- A pendência herdada da M0-02 (primeiro deploy dev + `curl /hello` validado)
  passa a ser cumprida pelo primeiro run do CodePipeline na M0-04.
- Custo: CodeBuild ~U$0,005/min de build (compute small arm) — desprezível
  no volume do projeto; CodePipeline V2 cobra por execução de pipeline.

## Revisão futura

Reavaliar se: o time passar a usar GitHub Enterprise com exigência de
deploy centralizado no GitHub; ou a InteliFi padronizar outra ferramenta de
CD entre os projetos.
