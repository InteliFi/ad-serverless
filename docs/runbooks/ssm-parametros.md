# Runbook — Inventário de credenciais e parâmetros SSM do ad-serverless

> **Objetivo:** lista COMPLETA do que o ad-serverless precisa no SSM Parameter
> Store, para o responsável (Fabio) cadastrar os valores. As credenciais do
> ad-server legado não seguem o padrão SSM da InteliFi — serão levantadas dos
> `application*.properties` e cadastradas aqui no padrão novo.
>
> ⚠️ **NUNCA** colar valores de segredo em issue, PR, chat ou neste arquivo.
> Depois de cadastrar, basta confirmar os NOMES criados (e o ARN da
> CodeConnection) para o código/pipeline referenciá-los.

## Convenção de nomes

- Padrão: `/ad-serverless/<stage>/<nome>` (alinhado à issue M0-05).
- Parâmetros do pipeline (sem stage): `/ad-serverless/pipeline/<nome>`.
- **Região importa:** dev em `us-east-1`, prod em `sa-east-1` — a Lambda lê o
  SSM da própria região (mesma decisão de regiões do serverless.yml/ADR).
- Tipo `SecureString` (KMS `alias/aws/ssm` é suficiente; chave dedicada pode
  vir depois sem mudar os nomes).

## 1. Segredos OBRIGATÓRIOS (SecureString)

| Parâmetro | Região | Conteúdo | Origem no legado | Consumidor |
|---|---|---|---|---|
| `/ad-serverless/dev/mysql-dsn` | us-east-1 | DSN Go do MySQL dev (formato abaixo) | `application-dev.properties` (host `dev-mysql.ckkqlpl6ei1d.us-east-1.rds.amazonaws.com`, db `adserver`, hoje usuário `root` ⚠️) | `internal/repository/mysql` (M3) via RDS Proxy (M2-03) |
| `/ad-serverless/prod/mysql-dsn` | sa-east-1 | DSN Go do MySQL prod | `application-prod.properties` (host `prod-mysql.cglsxksyzbur.sa-east-1.rds.amazonaws.com`, db `adserver`, usuário `adserver_dml`) | idem |
| `/ad-serverless/dev/signature-key` | us-east-1 | Chave de assinatura de postback — **gerar valor NOVO** | `intv.ad.signaturekey` do `application.properties` (⚠️ comprometida e idêntica em dev/prod; a validação está desativada no Java, então trocar não quebra nada) | `internal/tracking` (N11, port + flag) |
| `/ad-serverless/prod/signature-key` | sa-east-1 | idem (valor próprio de prod, diferente do dev) | idem | idem |

**Formato do DSN Go** (`go-sql-driver/mysql`):

```
<usuario>:<senha>@tcp(<host>:3306)/adserver?parseTime=true&loc=America%2FSao_Paulo&timeout=5s&readTimeout=30s&writeTimeout=30s
```

Notas:
- O formato definitivo dos query params será fixado na M3-01 (paridade com o
  `serverTimezone=GMT-3` do legado) — o que importa cadastrar agora é
  usuário/senha/host corretos; ajustar params é mudança barata.
- Em prod, quando o usuário dedicado `adserverless_app` for criado (M2-03),
  atualizar este parâmetro (nova VERSÃO do mesmo nome — o SSM versiona).
- Em dev, evitar `root`: se possível já criar usuário de aplicação no MySQL
  dev. Senhas legadas serão rotacionadas na M0-05.

## 2. Parâmetros do pipeline (CodePipeline/CodeBuild — ADR-009)

| Parâmetro | Região | Tipo | Conteúdo |
|---|---|---|---|
| `/ad-serverless/pipeline/github-connection-arn` | us-east-1 | String | ARN da conexão AWS CodeConnections com o GitHub (criada 1× no console — handshake com a org InteliFi; **não é um token**, não há segredo de GitHub a armazenar) |

- Serverless Framework v3 OSS **não tem** license key (ADR-008) — não há
  token de deploy a guardar. Se um dia migrar para v4:
  `/ad-serverless/pipeline/serverless-license-key` (SecureString).

## 3. O que NÃO migra para o SSM (importante)

| Item do legado | Destino |
|---|---|
| AWS access key/secret hardcoded no legado (ID registrado na issue M0-05) | **NÃO migra.** Lambdas usam IAM Role por função (M2-04); CodeBuild usa service role. A chave exposta será rotacionada/desativada na M0-05 (coordenado — as EC2 legadas ainda a usam) |
| Senha do usuário DDL (`adserver_ddl`) | Só será necessária no Epic M10 (migrations). Quando for: `/ad-serverless/prod/mysql-ddl-dsn` (SecureString). NÃO cadastrar antes |
| Credenciais de teste (`application-test.properties`) | Não migram — testes Go usam container local (testcontainers) |

## 4. Configuração NÃO-secreta (vai por env var no serverless.yml — não SSM)

Valores de comportamento (sem sigilo) ficam versionados no `serverless.yml`
de cada função (M2+), preservando paridade com o legado:

- Tabelas DynamoDB: `AdTrackers`, `PostbackLogs`; região DynamoDB `sa-east-1`
  **inclusive em dev** (cross-region — paridade com o legado).
- `PROXY_AUDIT_ALLOWED_FETCH_HOSTS`, `PROXY_AUDIT_CONNECT_TIMEOUT=10000`,
  `PROXY_AUDIT_READ_TIMEOUT=30000`, `PROXY_AUDIT_MAX_RESPONSE_SIZE=2097152`,
  `PROXY_AUDIT_CACHE_TTL_MINUTES=10`.
- `video.cache.whitelist.domains=gcdn.2mdn.net,googlevideo.com`.
- Código do hotspot de postback: `POSTBACK_HOTSPOT`.
- `STAGE` (já existe desde a M0-02).

## 5. Comandos prontos (preencher `<...>` na hora de executar)

```bash
# DEV (us-east-1)
aws ssm put-parameter --region us-east-1 --type SecureString \
  --name /ad-serverless/dev/mysql-dsn --value '<DSN dev>'
aws ssm put-parameter --region us-east-1 --type SecureString \
  --name /ad-serverless/dev/signature-key --value '<chave NOVA dev>'

# PROD (sa-east-1)
aws ssm put-parameter --region sa-east-1 --type SecureString \
  --name /ad-serverless/prod/mysql-dsn --value '<DSN prod>'
aws ssm put-parameter --region sa-east-1 --type SecureString \
  --name /ad-serverless/prod/signature-key --value '<chave NOVA prod>'

# Pipeline (após criar a conexão CodeConnections no console)
aws ssm put-parameter --region us-east-1 --type String \
  --name /ad-serverless/pipeline/github-connection-arn --value '<ARN da conexão>'

# Gerar chaves novas de assinatura (exemplo):
openssl rand -hex 16
```

## 6. Checklist pós-cadastro

- [ ] 4 SecureStrings criadas (2 por stage, na região certa)
- [ ] Conexão CodeConnections criada e ARN registrado
- [ ] Confirmar para o time APENAS os nomes/ARN (sem valores)
- [ ] As policies de leitura (`ssm:GetParameter` por prefixo
      `/ad-serverless/<stage>/*` + `kms:Decrypt`) entram nas IAM Roles das
      Lambdas na M2-04 e na service role do CodeBuild na M0-04
