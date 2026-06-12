# Runbook — Rotação de chaves AWS expostas + SSM bootstrap (M0-05)

> **Objetivo:** rotacionar a access key AWS exposta no repositório Java legado do
> ad-server e bootstrapear o SSM Parameter Store para as Lambdas do novo
> ad-serverless. Sem esta issue, nenhuma Lambda consegue acessar MySQL nem
> validar assinaturas de postback.
>
> ⚠️ **REGRA DE OURO:** NUNCA desativar a chave antiga antes da confirmação
> explícita do time — as EC2 de produção ainda dependem dela para DynamoDB.

## Índice

1. [Contexto e risco](#1-contexto-e-risco)
2. [Fase 1 — Inventário CloudTrail](#2-fase-1--inventário-cloudtrail)
3. [Fase 2 — Nova credencial mínima para o legado](#3-fase-2--nova-credencial-mínima-para-o-legado)
4. [Fase 3 — Rotação coordenada (NUNCA sem confirmação)](#4-fase-3--rotação-coordenada-nunca-sem-confirmação)
5. [Fase 4 — SSM Parameter Store bootstrap](#5-fase-4--ssm-parameter-store-bootstrap)
6. [Checklist pós-rotação](#6-checklist-pós-rotação)
7. [Como repetir uma rotação futura](#7-como-repetir-uma-rotação-futura)

---

## 1. Contexto e risco

A access key AWS exposta (ID registrado na issue #53) está em texto plano no
`application.properties` do ad-server Java — a **mesma** chave é usada em dev E
prod, junto com senhas MySQL hardcoded e a signature key de postback.

**Consumidores conhecidos:**

| Ambiente | EC2 | Região | Uso da chave |
|---|---|---|---|
| Dev | `i-0267248b971ac7cd8` | us-east-1 | DynamoDB `AdTrackers`, `PostbackLogs` (sa-east-1, cross-region) |
| Prod | `i-030bd120418d71a9d` | sa-east-1 | idem |
| Prod | `i-0707c9d77d0420be3` | sa-east-1 | idem |

**Risco:** qualquer um com acesso ao repo legado pode usar a chave para acessar
DynamoDB na conta. A rotação elimina o risco sem derrubar o tracking em
produção.

---

## 2. Fase 1 — Inventário CloudTrail

**Objetivo:** mapear TODO uso da chave antes de qualquer mudança. Não altere
nada nesta fase.

### 2.1. Último uso da chave

```bash
aws iam get-access-key-last-used --access-key-id <CHAVE_EXPOSTA_ID>
```

Registrar: data/hora do último uso, serviço chamado, região.

### 2.2. Listar chaves do usuário

```bash
# Descubra o dono da chave (se não souber):
aws iam list-access-keys --user-name <usuario-dono>
```

Confirmar que a chave exposta é a única ativa; registrar se houver outra.

### 2.3. Eventos CloudTrail (últimos 90 dias)

```bash
# sa-east-1 (onde estão as tabelas DynamoDB consumidas):
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=AccessKeyId,AttributeValue=<CHAVE_EXPOSTA_ID> \
  --max-results 50 \
  --region sa-east-1

# us-east-1 (onde está a EC2 dev que faz cross-region):
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=AccessKeyId,AttributeValue=<CHAVE_EXPOSTA_ID> \
  --max-results 50 \
  --region us-east-1
```

### 2.4. Documentar no runbook

Registrar os resultados nesta seção (editar após execução):

| Campo | Valor encontrado |
|---|---|
| Último uso (IAM) | |
| Serviços chamados | |
| Regiões ativas | |
| Origens (EC2/IPs) | |
| Uso inesperado? | Não (esperado: DynamoDB via EC2 legado) |

**Decisão:** se houver uso inesperado (serviço não-DynamoDB, IP externo),
parar e investigar antes de prosseguir.

---

## 3. Fase 2 — Nova credencial mínima para o legado

**Objetivo:** criar um usuário IAM dedicado com permissão SOMENTE para as
operações DynamoDB que o Java precisa. As EC2 legadas migrarão para esta chave
antes de desativar a antiga.

### 3.1. Policy de escopo mínimo

Baseada no inventário da Fase 1, a policy permite apenas as ações observadas:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:BatchWriteItem"
      ],
      "Resource": [
        "arn:aws:dynamodb:sa-east-1:<ACCOUNT_ID>:table/AdTrackers",
        "arn:aws:dynamodb:sa-east-1:<ACCOUNT_ID>:table/PostbackLogs"
      ]
    }
  ]
}
```

> **Nota:** ajustar a lista de actions ao que o CloudTrail mostrou — se o Java
> só usa `PutItem` e `Query`, remover `GetItem` e `BatchWriteItem`. O mínimo
> real é melhor.

### 3.2. Criar usuário + policy + chave

```bash
# Criar o usuário IAM dedicado
aws iam create-user --user-name adserver-legacy-ec2

# Anexar a policy (salvar o JSON acima em legacy-dynamodb-policy.json)
aws iam put-user-policy \
  --user-name adserver-legacy-ec2 \
  --policy-name DynamoDBAdTablesOnly \
  --policy-document file://legacy-dynamodb-policy.json

# Gerar access key nova
aws iam create-access-key --user-name adserver-legacy-ec2
```

Registrar a nova chave (Access Key ID e Secret Access Key) em local seguro —
**NUNCA** no repo, chat ou issue.

### 3.3. Migrar as EC2 legadas

1. **Dev primeiro:** atualizar `application-dev.properties` na EC2
   `i-0267248b971ac7cd8` com a nova chave → reiniciar app → validar tracking
   (item novo em `AdTrackers`).
2. **Prod depois:** atualizar `application*.properties` nas EC2 prod
   (`i-030bd120418d71a9d`, `i-0707c9d77d0420be3`) → reiniciar → validar.

**Validação:** verificar CloudWatch Logs do Java em cada EC2 — sem erro de
`UnrecognizedClientException` ou `AccessDeniedException`. Confirmar com o time
que tracking está gerando itens no DynamoDB normalmente.

---

## 4. Fase 3 — Rotação coordenada (NUNCA sem confirmação)

**Objetivo:** desativar e deletar a chave exposta + rotacionar senhas MySQL.

### 4.1. Pré-requisitos

- [ ] Dev operando com nova chave (validado DynamoDB OK)
- [ ] Prod operando com nova chave (validado DynamoDB OK)
- [ ] Confirmação explícita do time: "podemos desativar a chave antiga"

### 4.2. Desativar a chave exposta

```bash
aws iam update-access-key \
  --access-key-id <CHAVE_EXPOSTA_ID> \
  --status Inactive
```

**Observar por 48h:** monitorar erros no legado (CloudWatch, alarmes DynamoDB).
Se aparecer erro de credencial na EC2: reativar imediatamente.

```bash
# Reativar EMERGÊNCIA (se erro nas 48h):
aws iam update-access-key \
  --access-key-id <CHAVE_EXPOSTA_ID> \
  --status Active
```

### 4.3. Deletar a chave (após 48h sem incidentes)

```bash
aws iam delete-access-key \
  --access-key-id <CHAVE_EXPOSTA_ID>
```

### 4.4. Rotacionar senhas MySQL (com DBA)

Mesma janela coordenada, pois exigem atualizar as EC2:

| Usuário | Ambiente | Ação |
|---|---|---|
| `root` | Dev (us-east-1) | Nova senha no RDS + update `application-dev.properties` na EC2 dev |
| `adserver_dml` | Prod (sa-east-1) | Nova senha no RDS + update `application-prod.properties` nas EC2 prod |
| `adserver_ddl` | Prod (sa-east-1) | Nova senha no RDS (usuário DDL/Flyway, só Epic M10 usa) |

**Importante:** as Lambdas do ad-serverless usarão um usuário dedicado
(`adserverless_app`, criado na M2-03) — não precisam da senha do `adserver_dml`.

---

## 5. Fase 4 — SSM Parameter Store bootstrap

**Objetivo:** criar os parâmetros SecureString que as Lambdas vão ler em
runtime. As Lambdas usam IAM Role (M2-04) — nenhuma access key é criada para o
ad-serverless.

### 5.1. Gerar nova signature key

A chave legada está comprometida (texto plano no repo). A validação de
assinatura está desativada no Java, então trocar não quebra nada:

```bash
openssl rand -hex 16
# Exemplo de saída: a3f8b2c9d4e5f60718293a4b5c6d7e8f
```

Gerar **dois valores diferentes** — um para dev, outro para prod.

### 5.2. Criar parâmetros SSM

```bash
# DEV (us-east-1)
aws ssm put-parameter \
  --region us-east-1 \
  --type SecureString \
  --name /ad-serverless/dev/mysql-dsn \
  --value '<DSN dev com credencial NOVA — formato: user:pass@tcp(host)/adserver?parseTime=true>'

aws ssm put-parameter \
  --region us-east-1 \
  --type SecureString \
  --name /ad-serverless/dev/signature-key \
  --value '<nova signature key dev>'

# PROD (sa-east-1)
aws ssm put-parameter \
  --region sa-east-1 \
  --type SecureString \
  --name /ad-serverless/prod/mysql-dsn \
  --value '<DSN prod — placeholder até M2-03 criar adserverless_app>'

aws ssm put-parameter \
  --region sa-east-1 \
  --type SecureString \
  --name /ad-serverless/prod/signature-key \
  --value '<nova signature key prod>'
```

**Notas:**
- O DSN de prod é placeholder até o M2-03 criar `adserverless_app` — atualizar
  com `put-parameter` (isso cria uma nova versão, não substitui).
- KMS: usar a chave gerenciada por AWS (`alias/aws/ssm`) — suficiente para
  SecureString. Chave dedicada pode vir depois sem mudar os nomes.

### 5.3. Verificar criação

```bash
aws ssm get-parameter --region us-east-1 --name /ad-serverless/dev/signature-key --with-decryption
aws ssm get-parameter --region sa-east-1 --name /ad-serverless/prod/signature-key --with-decryption
```

Confirmar que o `Type` é `SecureString` e o valor está correto.

---

## 6. Checklist pós-rotação

- [ ] Inventário CloudTrail documentado (Fase 1, tabela preenchida)
- [ ] Usuário `adserver-legacy-ec2` criado com policy restrita DynamoDB
- [ ] EC2 dev migrada para nova chave + tracking validado
- [ ] EC2s prod migradas para nova chave + tracking validado
- [ ] Chave exposta desativada (`Inactive`) — data: ___/___/______
- [ ] 48h de observação sem incidentes
- [ ] Chave exposta deletada — data: ___/___/______
- [ ] Senhas MySQL rotacionadas (root dev, adserver_dml prod, adserver_ddl prod)
- [ ] 4 parâmetros SSM SecureString criados (2 dev/us-east-1 + 2 prod/sa-east-1)
- [ ] Signature key nova gerada (valor legado descartado)
- [ ] Confirmado: nenhuma access key criada para o ad-serverless (IAM Roles apenas)

### Aprovações

| Fase | Quem aprovou | Data |
|---|---|---|
| Desativar chave antiga | | |
| Deletar chave antiga (pós 48h) | | |
| Rotacionar senhas MySQL | | |

---

## 7. Como repetir uma rotação futura

1. Gerar nova credencial (SSM ou IAM, conforme o segredo).
2. Atualizar TODOS os consumidores (EC2, Lambda via SSM, pipeline) com a nova
   credencial — dev primeiro, prod depois.
3. Validar cada consumidor operando normalmente.
4. Se for access key IAM: desativar (`Inactive`) → observar 48h → deletar.
5. Se for SSM SecureString: `put-parameter` cria nova versão automaticamente;
   as Lambdas leem a versão mais recente no próximo cold start.
6. Registrar data, aprovador e qualquer incidente neste runbook (editar seção 6).

---

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (EC2/IDs), §2 (segredos hardcoded)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §7 (segurança)
- [docs/runbooks/ssm-parametros.md](ssm-parametros.md) — inventário completo de parâmetros SSM
