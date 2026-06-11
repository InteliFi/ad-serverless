# Sistema Legado — Configuração, Infraestrutura e Deploy

> **Fonte:** análise de `application*.properties`, classes de config Spring, `pom.xml`, `ci/build.sh`, `ci/deploy.sh` e `Dockerfile` do ad-server (v0.25.0).

## 1. Topologia atual

| Ambiente | Compute | RDS MySQL | DynamoDB | Deploy |
|---|---|---|---|---|
| DEV | EC2 `i-0267248b971ac7cd8` (us-east-1), Docker porta 91→8080, mem 512m | us-east-1 (`dev-mysql.ckkqlpl6ei1d...`) | sa-east-1 ⚠️ cross-region | `ci/build.sh -dev` (scp+rsync+ssh) |
| PROD | 2× EC2 `i-030bd120418d71a9d`, `i-0707c9d77d0420be3` (sa-east-1), Docker porta 80→8080, mem 3072m | sa-east-1 (`prod-mysql.cglsxksyzbur...`) | sa-east-1 | `ci/build.sh -prod` (deploy paralelo nas 2) |

Sem CI/CD automatizado — scripts shell manuais. Health check Docker: `curl -f http://localhost:8080/` (30s/10s/60s start/3 retries).

## 2. ⚠️ SEGREDOS HARDCODED (ação crítica na migração)

| Segredo | Onde | Risco |
|---|---|---|
| AWS access key + secret (mesma p/ dev e prod) | `application*.properties` | **CRÍTICO — rotacionar e mover p/ IAM Role da Lambda** |
| Senha MySQL dev (root) | `application-dev.properties` | rotacionar; mover p/ Secrets Manager/SSM |
| Senhas MySQL prod (`adserver_dml`, `adserver_ddl`) | `application-prod.properties` | rotacionar; mover p/ Secrets Manager/SSM |
| `intv.ad.signaturekey` | `application.properties` | mover p/ SSM Parameter Store (SecureString) |

Na arquitetura serverless: **IAM Roles por função** (sem access keys), segredos via SSM/Secrets Manager com cache na inicialização do container.

## 3. Configurações relevantes (valores exatos a preservar/adaptar)

### HTTP / Tomcat (→ limites do API Gateway + Lambda)
- Max HTTP header 128KB; max POST 20MB; max connections 10.000; max threads 200; connection timeout 60s; URI UTF-8.
- `relaxed-query-chars` / `relaxed-path-chars`: `| { } [ ] ^ \` < > \ ; : / ? : @ & = + $ # %` — URLs malformadas de players DEVEM continuar aceitas (API Gateway HTTP API é mais permissivo que REST API, mas validar com testes reais).

### HikariCP (→ dimensionamento de conexões Lambda)
- Pool máx 10, min idle 2–5, idle timeout 120s, max lifetime 300s, leak detection 60s, validation `SELECT 1`.
- **Implicação Lambda:** 1 conexão por container; com pico de containers concorrentes, usar **RDS Proxy** para multiplexar e proteger o MySQL compartilhado.

### HTTP client (RestTemplate)
- Connect/read timeout globais: 60s (proxies usam 10s/30s específicos no proxy-audit).
- Em Go: `http.Client` compartilhado por handler com timeouts equivalentes e `Transport` com keep-alive.

### Async (→ SQS)
- ThreadPool core 2 / max 4 / queue 100, prefixo `PostbackLog-` — substituído por SQS + Lambda consumer.

### Flyway
- `baseline-on-migrate=true`, retries 10×10s; prod usa usuário DDL separado (`adserver_ddl`).
- **Fase 1: Flyway NÃO roda nas Lambdas** — schema é gerido pelo ad-server legado até o cutover do banco.

### Jobs agendados (legado — desnecessários em Lambda)
- `checkConnectionHealth` a cada 5 min (`SELECT 1`) e `validateConnectionPool` a cada 1h — keep-alive de pool; sem equivalente necessário (RDS Proxy resolve). Documentado para não "perder feature": a *função* era manter conexões saudáveis.

### Logging
- Nível ERROR global (Flyway INFO em prod), apenas stdout.
- Em Go: `log/slog` JSON em stdout → CloudWatch Logs; nível por env var.

### Proxy Audit (prod)
```
PROXY_AUDIT_ALLOWED_FETCH_HOSTS=cdn.00px.net,cdn.vendor.com,admotion.digital,servedby.metrike.com.br,nsp.admotion.digital
PROXY_AUDIT_CONNECT_TIMEOUT=10000
PROXY_AUDIT_READ_TIMEOUT=30000
PROXY_AUDIT_MAX_RESPONSE_SIZE=2097152
PROXY_AUDIT_CACHE_TTL_MINUTES=10
```

### Video cache
```
video.cache.directory=/tmp/adserver_video_cache[_prod]
video.cache.whitelist.domains=gcdn.2mdn.net,googlevideo.com
```

### Timezone
- Container roda com `TZ=GMT-3`; conexão MySQL `serverTimezone=GMT-3`; DynamoDB event_date em `America/Sao_Paulo`.
- Em Go: **nunca depender do TZ do ambiente** — usar `time.LoadLocation("America/Sao_Paulo")` explícito (embarcar tzdata via `import _ "time/tzdata"`).

## 4. Dependências Maven → equivalentes Go

| Java | Uso | Go |
|---|---|---|
| Spring Boot Web 3.4.13 | HTTP server | `aws-lambda-go` + API Gateway (payload v2) |
| Spring Data JPA + mysql-connector | ORM MySQL | `database/sql` + `go-sql-driver/mysql` |
| HikariCP | pool | `sql.DB` (pool nativo) + RDS Proxy |
| Flyway | migrations | `golang-migrate` (só na fase de banco) |
| Caffeine | cache local | cache TTL próprio (`internal/cache`) ou `hashicorp/golang-lru/v2/expirable` |
| AWS SDK v2 (dynamodb-enhanced 2.20.162) | DynamoDB | `aws-sdk-go-v2/service/dynamodb` + `attributevalue` |
| Apache POI 3.15 | Excel | `xuri/excelize/v2` |
| commons-io / commons-lang3 / commons-codec | utils | stdlib (`io`, `strings`, `encoding/hex`, `crypto/md5`) |
| spring-retry | retry | `aws-sdk-go-v2` retryer + wrapper próprio |
| Lombok | boilerplate | desnecessário |
| JUnit5/Mockito/RestAssured/H2 | testes | `testing` + `testify` + `httptest` + MySQL em container (testcontainers-go) |

## 5. Versionamento e branches do legado

- ad-server `0.25.0`, branch `develop` @ `74748d2`; ad-commons `1.4.4`.
- Commits recentes relevantes (regras que JÁ estão nas specs): Caffeine cache hotspots/override (`f459e85`), bypass DoubleClick (`1f8ce34`), skip video cache não-mp4 (`52936fa`), proxy só do VPAID JS AdForce (`c26eba4`), click URL direto em campaign VAST (`74748d2`).

## 6. Requisitos não-funcionais (metas da migração)

| Métrica | Hoje (EC2) | Meta (Lambda Go) |
|---|---|---|
| Volume | ~2M req/dia (~23 rps médio, picos estimados 10×) | suportar ≥10× sem ação manual |
| Latência ad/tracking | dezenas de ms + GC Java | p50 < 20ms, p99 < 150ms (sem upstream) |
| Cold start | N/A (sempre quente) | < 100ms (Go/arm64); provisioned concurrency no vast-handler se necessário |
| Disponibilidade | 2 EC2 prod, sem auto-healing além do Docker restart | multi-AZ implícito da Lambda; DLQ p/ tracking |
| Deploy | shell manual, sem rollback | GitHub Actions, deploy por stage, rollback automático |
| Segredos | hardcoded | IAM Roles + SSM/Secrets Manager |
