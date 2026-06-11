---
title: "[M0-02] serverless.yml base + hello-handler + stages dev/prod"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P0"]
milestone: "M0 — Fundação"
---
## Contexto

O entregável de aceite do epic M0 ([docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Sequência de Epics) é: "`serverless deploy --stage dev` publica um hello-handler com sucesso". Esta issue cria o `serverless.yml` que será a espinha dorsal de TODA a infraestrutura (as 9 Lambdas e os recursos do M2 serão adicionados a ele).

As regiões espelham o legado ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1): **dev em `us-east-1`** e **prod em `sa-east-1`** — obrigatório porque o MySQL de cada stage está nessas regiões e a latência Lambda↔RDS precisa ser mínima.

⚠️ Inclui uma **DECISÃO a documentar**: Serverless Framework v3 OSS vs v4. A v4 exige licença paga para empresas com receita anual > US$ 2M (risco mapeado em PLANO-MIGRACAO §Riscos: "Serverless Framework v4 exigir licença → usar v3 OSS; decidir em M0").

## Especificação detalhada

### 1. Decisão Serverless Framework v3 vs v4 (fazer PRIMEIRO)
Escrever mini-ADR em `docs/arquitetura/ADR-008-serverless-framework-versao.md` cobrindo:
- v4: licença obrigatória acima de US$ 2M de receita/ano, exige `SERVERLESS_LICENSE_KEY` ou login no CI; v3 é OSS (MIT) porém em manutenção (sem features novas).
- Recomendação default: **v3 (3.40.x) pinado** via `package.json` (`"serverless": "~3.40.0"`) e `frameworkVersion: '3'` no yml. Se o time confirmar elegibilidade/licença v4, registrar no ADR e ajustar.
- A escolha afeta M0-03/M0-04 (instalação no CI). Registrar a decisão final no ADR antes do merge.

### 2. `serverless.yml`
```yaml
service: ad-serverless
frameworkVersion: '3'

provider:
  name: aws
  runtime: provided.al2023
  architecture: arm64
  region: ${self:custom.regions.${sls:stage}}
  stage: ${opt:stage, 'dev'}
  memorySize: 256
  timeout: 10
  httpApi:
    payload: '2.0'          # payload v2.0 — contrato dos handlers Go (ADR-001)
  environment:
    STAGE: ${sls:stage}
  stackTags:
    projeto: ad-serverless
    stage: ${sls:stage}

custom:
  regions:
    dev: us-east-1          # mesmo do legado (EC2 dev + dev-mysql)
    prod: sa-east-1         # mesmo do legado (EC2 prod + prod-mysql + DynamoDB)

package:
  individually: true        # 1 zip por função — deploy independente, zip mínimo

functions:
  hello:
    handler: bootstrap
    package:
      artifact: bin/hello/hello.zip
    events:
      - httpApi:
          method: GET
          path: /hello
```
Notas:
- `sls:stage` só aceita `dev`/`prod` — validar com `custom.regions` (stage inexistente quebra o deploy, o que é o comportamento desejado).
- O empacotamento usa zip pré-construído pelo Makefile (artifact) contendo apenas `bootstrap` — adicionar ao `Makefile` o passo de zip por serviço (`cd bin/$$svc && zip -q $$svc.zip bootstrap`).

### 3. `cmd/hello/main.go` (provisório — será removido quando `ad-handler` existir)
- Usa `github.com/aws/aws-lambda-go/lambda` + `events.APIGatewayV2HTTPRequest/Response`.
- Responde `200` com JSON `{"service":"ad-serverless","stage":"<STAGE>","status":"UP"}` — mesmo formato de status do health legado (`{"version":..., "status":"UP"}`, [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §1) para já validar o contrato payload v2.0.
- Comentários godoc em português; marcar claramente `// PROVISÓRIO: removido na issue do ad-handler (M4-04)`.

### 4. `package.json`
Somente devDependency do Serverless Framework pinado conforme o ADR. Adicionar `node_modules/` já está no `.gitignore` (M0-01).

## Arquivos a criar/alterar

- `serverless.yml`
- `cmd/hello/main.go`
- `package.json` (+ `package-lock.json`)
- `Makefile` (adicionar passo de zip por serviço no target `build`)
- `go.mod` / `go.sum` (adicionar `github.com/aws/aws-lambda-go`)
- `docs/arquitetura/ADR-008-serverless-framework-versao.md`

## Critérios de aceite

- [ ] ADR-008 escrito com a decisão v3/v4 justificada e a versão pinada
- [ ] `make build` gera `bin/hello/bootstrap` (linux/arm64, estático) e o zip
- [ ] `npx serverless print --stage dev` resolve região `us-east-1`; `--stage prod` resolve `sa-east-1`
- [ ] `npx serverless deploy --stage dev` conclui com sucesso (executar com credenciais locais de dev; em CI virá na M0-04)
- [ ] `curl https://<api-id>.execute-api.us-east-1.amazonaws.com/hello` retorna `200` com o JSON especificado
- [ ] `package.individually: true` e `architecture: arm64` presentes
- [ ] Função `hello` documentada como provisória no código e no serverless.yml (comentário)
- [ ] `make lint && make test` verdes

## Dependências

Bloqueada por: #M0-01

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (runtime/arm64/HTTP API), ADR-001
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (regiões dev/prod do legado)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — risco "Serverless Framework v4 exigir licença"
- https://github.com/aws/aws-lambda-go (runtime `provided.al2023` + `lambda.norpc`)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-02] serverless.yml base + hello-handler +
stages dev/prod no repo InteliFi/ad-serverless. Seguir exatamente a
especificação: decidir e documentar ADR-008 (Serverless v3 OSS vs v4),
serverless.yml com provider aws, runtime provided.al2023, arm64, região
por stage (dev=us-east-1, prod=sa-east-1), package.individually, httpApi
payload 2.0 e hello-handler provisório em cmd/hello. Código comentado em
português (CODE_DOCS_POLICY.md). make lint/test verdes e deploy dev
validado. Ao final: abrir PR referenciando a issue.
```
