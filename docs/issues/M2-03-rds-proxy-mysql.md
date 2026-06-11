---
title: "[M2-03] RDS Proxy na frente do MySQL existente (dev/prod)"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P0"]
milestone: "M2 — Infra AWS"
---
## Contexto

⚠️ **ISSUE DE MÁXIMO CUIDADO.** O MySQL RDS é **compartilhado com outros projetos** e não tem CI/CD de produção ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) aviso inicial, ADR-006). O ad-server legado nas EC2 continua usando esses bancos em produção DURANTE toda a migração.

Endpoints existentes ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §1):
- **dev:** `dev-mysql.ckkqlpl6ei1d.us-east-1.rds.amazonaws.com` (us-east-1), schema `adserver`
- **prod:** `prod-mysql.cglsxksyzbur.sa-east-1.rds.amazonaws.com` (sa-east-1), schema `adserver`

Por que o RDS Proxy (ADR-002): Lambdas escalam para N containers e cada um abre 1–2 conexões; o HikariCP legado usava pool máx 10 ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3). Sem proxy, um pico de containers esgotaria `max_connections` do RDS compartilhado e **derrubaria OUTROS projetos**. O RDS Proxy multiplexa conexões e é **infra 100% ADITIVA**: nada muda no RDS, no schema ou nos consumidores existentes.

### Regras invioláveis desta issue

1. **NENHUM** `modify-db-instance`, mudança de parameter group, reboot, upgrade ou alteração de configuração do RDS.
2. **NENHUM** `CREATE/ALTER/DROP` de tabela, índice ou schema. Flyway permanece desligado.
3. A ÚNICA mudança em recurso existente permitida: **adicionar** (nunca remover/editar) UMA regra de ingress no security group do RDS, liberando 3306 a partir do SG do proxy.
4. Credenciais legadas expostas (root dev, `adserver_dml`/`adserver_ddl` prod) NUNCA são copiadas (M0-05).

## Especificação detalhada

### Fase 0 — Descoberta (somente leitura, registrar no runbook)

```bash
aws rds describe-db-instances --region us-east-1 \
  --query "DBInstances[?contains(Endpoint.Address,'dev-mysql')].{id:DBInstanceIdentifier,vpc:DBSubnetGroup.VpcId,subnets:DBSubnetGroup.Subnets[].SubnetIdentifier,sg:VpcSecurityGroups[].VpcSecurityGroupId,engine:EngineVersion}"
aws rds describe-db-instances --region sa-east-1 \
  --query "DBInstances[?contains(Endpoint.Address,'prod-mysql')].{...}"   # idem
# Verificar se a VPC já tem NAT Gateway e quais route tables servem as subnets privadas:
aws ec2 describe-nat-gateways --region <região> --filter Name=vpc-id,Values=<vpc-do-rds>
aws ec2 describe-route-tables --region <região> --filters Name=vpc-id,Values=<vpc-do-rds>
```

Registrar: `DBInstanceIdentifier`, VPC, subnets, SGs do RDS, versão do engine (RDS Proxy exige MySQL 5.7+/8.x) e existência de NAT. **Snapshot textual do "antes"** para o diff de validação da Fase 7.

### Fase 1 — Usuário MySQL `adserverless_app` (tarefa MANUAL coordenada) 🔴

Criar usuário **só-DML** é PREFERÍVEL a reusar `root`/`adserver_dml`. Porém `CREATE USER`/`GRANT` exigem privilégio administrativo no banco compartilhado → **tarefa manual executada pelo engenheiro-chefe (ou DBA por ele indicado)**, agendada e registrada no runbook. SQL exato a entregar:

```sql
-- Cria usuário de aplicação do ad-serverless (least privilege: SELECT geral + INSERT só em ad_trackers)
-- NÃO é mudança de schema: nenhuma tabela/índice/coluna é tocada.
CREATE USER 'adserverless_app'@'%' IDENTIFIED BY '<senha forte gerada na hora, ex. openssl rand -base64 24>';
GRANT SELECT ON adserver.* TO 'adserverless_app'@'%';
GRANT INSERT ON adserver.ad_trackers TO 'adserverless_app'@'%';
FLUSH PRIVILEGES;
```

- Executar primeiro em dev, validar, depois em prod (janela combinada).
- A senha vai DIRETO para o Secrets Manager (Fase 2) — nunca em chat/e-mail/repo.
- **Fallback documentado** (se a criação em prod atrasar): usar `adserver_dml` TEMPORARIAMENTE, com senha JÁ ROTACIONADA (M0-05) — registrar a decisão e criar follow-up para trocar por `adserverless_app`. Nunca usar root.

### Fase 2 — Secrets Manager (exigência do RDS Proxy)

O RDS Proxy autentica targets via **Secrets Manager** (não SSM). Criar em cada região/stage:

```bash
aws secretsmanager create-secret --region us-east-1 \
  --name ad-serverless/dev/mysql-proxy-credentials \
  --secret-string '{"username":"adserverless_app","password":"<senha-dev>"}'
aws secretsmanager create-secret --region sa-east-1 \
  --name ad-serverless/prod/mysql-proxy-credentials \
  --secret-string '{"username":"adserverless_app","password":"<senha-prod>"}'
```

### Fase 3 — Infra como código em stack SEPARADA (`infra/rds-proxy.yml`)

**Decisão:** o proxy NÃO entra no `serverless.yml` — um `serverless remove` acidental do stack da aplicação não pode arrastar infraestrutura ligada ao banco compartilhado. Criar template CloudFormation dedicado `infra/rds-proxy.yml` (parâmetros: `Stage`, `VpcId`, `SubnetIds`, `DbInstanceIdentifier`, `RdsSecurityGroupId`, `SecretArn`), com `DeletionPolicy: Retain` no proxy, contendo:

a) **Role do proxy** — `ad-serverless-rds-proxy-role-{stage}`: trust `rds.amazonaws.com`; policy `secretsmanager:GetSecretValue` APENAS no ARN do secret da Fase 2.

b) **Security groups (novos, aditivos):**
- `ad-serverless-lambda-sg-{stage}` — para as Lambdas; sem ingress; egress TCP 3306 → SG do proxy e TCP 443 → 0.0.0.0/0 (endpoints/NAT).
- `ad-serverless-rdsproxy-sg-{stage}` — ingress TCP 3306 do `ad-serverless-lambda-sg-{stage}`; egress TCP 3306 → SG do RDS.

c) **Regra ADITIVA no SG existente do RDS** (`AWS::EC2::SecurityGroupIngress` standalone, NUNCA inline para não reescrever o SG): ingress TCP 3306 com source = `ad-serverless-rdsproxy-sg-{stage}`, description `ad-serverless RDS Proxy`. ⚠️ Único toque em recurso existente — aprovar com o engenheiro-chefe antes do deploy.

d) **RDS Proxy:**

```yaml
DBProxy:
  Type: AWS::RDS::DBProxy
  DeletionPolicy: Retain
  Properties:
    DBProxyName: ad-serverless-mysql-proxy-${Stage}
    EngineFamily: MYSQL
    RequireTLS: true
    IdleClientTimeout: 1800
    Auth:
      - { AuthScheme: SECRETS, SecretArn: !Ref SecretArn, IAMAuth: DISABLED }
    RoleArn: !GetAtt ProxyRole.Arn
    VpcSubnetIds: !Ref SubnetIds                      # mesmas subnets do RDS
    VpcSecurityGroupIds: [!Ref ProxySecurityGroup]

DBProxyTargetGroup:
  Type: AWS::RDS::DBProxyTargetGroup
  Properties:
    DBProxyName: !Ref DBProxy
    TargetGroupName: default
    DBInstanceIdentifiers: [!Ref DbInstanceIdentifier]   # instância EXISTENTE — registrada, jamais alterada
    ConnectionPoolConfigurationInfo:
      MaxConnectionsPercent: 25        # proxy usa NO MÁXIMO 25% do max_connections — protege os outros projetos
      MaxIdleConnectionsPercent: 10
      ConnectionBorrowTimeout: 120
```

Deploy manual documentado: `aws cloudformation deploy --template-file infra/rds-proxy.yml --stack-name ad-serverless-rds-proxy-dev --parameter-overrides Stage=dev ... --capabilities CAPABILITY_NAMED_IAM --region us-east-1` (idem prod em sa-east-1).

### Fase 4 — VPC nas Lambdas que acessam MySQL (`serverless.yml`)

Somente as funções que tocam MySQL entram na VPC do RDS: `ad`, `vast`, `track` (trackingpixel), `postback`, `report`, `tracker-writer`. **Ficam FORA da VPC:** `redirect`, `proxy`, `media` (não usam MySQL e precisam de internet direta/baixa latência).

```yaml
custom:
  vpc:
    dev:
      securityGroupIds: [<ad-serverless-lambda-sg-dev>]
      subnetIds: [<subnets privadas da VPC do dev-mysql>]
    prod: { ... }   # idem com valores de prod
# por função: vpc: ${self:custom.vpc.${sls:stage}}
```

### Fase 5 — Conectividade de Lambdas em VPC (endpoints + NAT)

Lambda dentro de VPC perde acesso à internet e aos serviços AWS públicos. Provisionar na VPC do RDS (também em `infra/rds-proxy.yml` ou `infra/vpc-endpoints.yml`):
- **Gateway endpoints (grátis):** S3 e DynamoDB, associados às route tables das subnets privadas (tracker-writer escreve DynamoDB; atenção: em dev o DynamoDB fica em sa-east-1 — gateway endpoint NÃO atende cross-region; nesse caso o tráfego sai via NAT, paridade com o cross-region ⚠️ do legado).
- **Interface endpoints:** `com.amazonaws.{região}.ssm` (DSN via SSM) e `com.amazonaws.{região}.sqs` (track-handler publica na fila) — ~US$ 7/mês/AZ cada, registrar custo.
- **NAT Gateway:** obrigatório para upstreams externos de funções em VPC (`vast` faz fetch de parceiros; `postback` chama modatta/prezão). **Reusar NAT existente da VPC se houver (Fase 0)**; senão criar 1 NAT Gateway + rota `0.0.0.0/0` nas route tables privadas (~US$ 32/mês + tráfego — já contemplado na estimativa de custo da arquitetura).

### Fase 6 — Atualizar o DSN no SSM (consumido pela M3-01)

```bash
aws ssm put-parameter --overwrite --type SecureString --region us-east-1 \
  --name /ad-serverless/dev/mysql-dsn \
  --value 'adserverless_app:<senha-dev>@tcp(<endpoint-do-proxy-dev>:3306)/adserver?parseTime=true&loc=America%2FSao_Paulo&tls=true'
# idem prod em sa-east-1 com o endpoint do proxy prod
```

`tls=true` é obrigatório (`RequireTLS: true` no proxy). Host = endpoint do PROXY, nunca o RDS direto.

### Fase 7 — Validação (sem tocar no banco além de SELECT)

1. Lambda descartável de validação (ou `cmd/hello` temporariamente na VPC) executando: `SELECT 1`, `SELECT COUNT(*) FROM hotspots`, `SELECT COUNT(*) FROM campaigns` via proxy → valores plausíveis (~928 hotspots, ~90 campanhas).
2. Confirmar que `adserverless_app` NÃO consegue DDL: `CREATE TABLE teste_ddl (id INT)` deve falhar com `Access denied` (registrar evidência no runbook).
3. Diff do "antes/depois" do `describe-db-instances`: ÚNICA diferença aceitável é a regra nova no SG do RDS. Sem reboot (`aws rds describe-events --source-identifier <id> --source-type db-instance --duration 1440` sem eventos de modificação).
4. Conferir métricas `DatabaseConnections` do RDS antes/depois — sem salto anormal.

## Arquivos a criar/alterar

- `infra/rds-proxy.yml` (CloudFormation: role, SGs, ingress aditivo, DBProxy, TargetGroup, endpoints/NAT ou `infra/vpc-endpoints.yml` separado)
- `serverless.yml` (`custom.vpc.{dev,prod}` + `vpc:` nas funções que usam MySQL — com comentário de quais ficam fora e por quê)
- `docs/runbooks/rds-proxy.md` (descoberta da Fase 0, SQL do usuário, sequência executada, evidências de validação, custos NAT/endpoints, rollback = deletar proxy/SGs novos e remover a regra de ingress — zero impacto no RDS)
- `docs/MATRIZ-PARIDADE.md` (linha "HikariCP pool → RDS Proxy")
- Secrets Manager + SSM (infra via cli, documentada no runbook)

## Critérios de aceite

- [ ] Proxies `ad-serverless-mysql-proxy-dev` (us-east-1) e `-prod` (sa-east-1) `Available`, `RequireTLS=true`, target = instância existente com status `AVAILABLE` (`aws rds describe-db-proxy-targets`)
- [ ] `MaxConnectionsPercent=25` e `MaxIdleConnectionsPercent=10` no target group (proteção do banco compartilhado)
- [ ] Usuário `adserverless_app` criado (dev e prod) com SELECT em `adserver.*` + INSERT apenas em `ad_trackers`; evidência de `Access denied` para DDL no runbook
- [ ] Secrets `ad-serverless/{stage}/mysql-proxy-credentials` criados; senha em NENHUM arquivo do repo
- [ ] SSM `/ad-serverless/{stage}/mysql-dsn` atualizado com endpoint do proxy, `tls=true`, `parseTime=true`, `loc=America%2FSao_Paulo`
- [ ] Lambda de validação na VPC executa `SELECT 1` e os COUNTs via proxy com sucesso nos 2 stages
- [ ] Diff antes/depois do RDS: única mudança é a regra de ingress aditiva no SG; nenhum evento de modificação/reboot na instância
- [ ] `serverless.yml` com `vpc:` SOMENTE nas funções que usam MySQL; redirect/proxy/media fora da VPC (comentário justificando)
- [ ] Endpoints S3/DynamoDB/SSM/SQS e NAT (criado ou reusado) documentados no runbook com custos
- [ ] Stack do proxy SEPARADA do stack da aplicação, com `DeletionPolicy: Retain`
- [ ] Runbook completo em português, com aprovação do engenheiro-chefe registrada para a Fase 1 e para a regra de SG

## Dependências

Bloqueada por: #M0-05 (SSM bootstrap + credenciais legadas nunca reutilizadas)

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §1 (endpoints/schema `adserver`), aviso do engenheiro-chefe
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2 (segredos), §3 (HikariCP/implicação Lambda)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-002, ADR-006, §6 (custos)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) risco "Esgotar conexões do MySQL compartilhado"
- Issues: M0-05 (segredos), M3-01 (consumidor do DSN), M2-04 (permissões SSM das Lambdas), M7-04 (alarme de conexões do proxy)
- Java: `ad-server/src/main/resources/application*.properties` (HikariCP/endpoints — referência, NUNCA copiar credenciais)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-03] RDS Proxy na frente do MySQL existente no
repo InteliFi/ad-serverless seguindo docs/issues/M2-03-rds-proxy-mysql.md e
CLAUDE.md. INFRA ADITIVA APENAS: nenhuma alteração no RDS nem no schema
(banco compartilhado com outros projetos). Executar as fases NA ORDEM:
descoberta read-only, SQL do usuário adserverless_app entregue como tarefa
manual do engenheiro-chefe, Secrets Manager, stack separada
infra/rds-proxy.yml (proxy TLS, MaxConnectionsPercent 25, SGs novos + 1
ingress aditivo no SG do RDS), vpc: nas Lambdas que usam MySQL, endpoints
S3/DynamoDB/SSM/SQS + NAT, atualização do SSM mysql-dsn e validação SELECT
via proxy. Runbook em português com evidências. Abrir PR ao final listando
o que foi executado e o que aguarda aprovação humana.
```
