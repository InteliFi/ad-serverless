# Arquitetura Alvo — ad-serverless (Go + AWS Lambda + Serverless Framework)

> Decisões de arquitetura para a migração do ad-server/ad-commons (Java/EC2) para microserviços serverless em Go.
> Volume: 2M+ req/dia com tracking timestampado. Metas: alta disponibilidade, escala automática, latência mínima, custo otimizado.

## 1. Stack Tecnológica

| Decisão | Escolha | Justificativa |
|---|---|---|
| **Linguagem** | Go 1.24+ | Cold start < 100ms (vs. segundos em JVM), binário único, footprint mínimo de memória (128–256MB), concorrência nativa para proxies I/O-bound |
| **Runtime Lambda** | `provided.al2023` + **arm64 (Graviton)** | Runtime custom Go oficial; arm64 ≈ 20% mais barato e mais rápido |
| **IaC / Deploy** | **Serverless Framework** (serverless.yml único, `package.individually`) | Requisito do projeto; DX simples, plugins maduros, stages dev/prod |
| **API** | API Gateway **HTTP API** (payload v2.0) | 3–4× mais barato e menor latência que REST API; suporta CORS nativo + rotas catch-all |
| **Banco fase 1** | **MySQL existente via RDS Proxy** (leitura hot path + escrita ad_trackers) | ⚠️ Banco compartilhado com outros projetos — NENHUMA mudança de schema; RDS Proxy multiplexa conexões de N containers Lambda |
| **Banco tracking** | DynamoDB `AdTrackers` + `PostbackLogs` (**tabelas existentes, reusadas**) | Já em produção; on-demand; padrão de chaves preservado |
| **Fila de tracking** | SQS Standard + DLQ → Lambda `tracker-writer` | Substitui @Async; absorve picos; batch write; retry confiável |
| **Mídia (vídeo cache)** | S3 + CloudFront | Substitui `/tmp/adserver_video_cache`; cache global de borda |
| **Cache de dados** | In-memory por container com TTL (5min/500, semântica Caffeine) | Containers Lambda reusam memória entre invocações; zero custo extra |
| **Segredos** | IAM Roles (AWS) + SSM Parameter Store SecureString (MySQL, signature key) | Elimina credenciais hardcoded; chaves expostas DEVEM ser rotacionadas |
| **Observabilidade** | CloudWatch Logs (slog JSON) + métricas EMF + X-Ray + alarmes | Tracing end-to-end; dashboards por serviço |
| **CI/CD** | GitHub Actions (lint → test → build → deploy dev → smoke → deploy prod manual) | Repo já no GitHub; OIDC para AWS (sem secrets estáticos) |
| **Testes** | `testing` + testify + httptest + **golden tests** comparando saída Java×Go | Paridade byte-a-byte nos templates/VAST |

## 2. Diagrama

```
                        ┌──────────────────────────────────────────────┐
                        │                 CloudFront                    │
                        │   (ads.inteli.fi · cache de borda · WAF)      │
                        └───────┬──────────────────────────┬───────────┘
                                │                          │
                     ┌──────────▼───────────┐   ┌──────────▼──────────┐
                     │ API Gateway HTTP API │   │  S3 bucket media     │
                     │  (stage dev/prod)    │   │  (vídeos MD5(url))   │
                     └──────────┬───────────┘   └─────────────────────┘
        ┌───────────┬───────────┼────────────┬──────────────┬─────────────┐
        │           │           │            │              │             │
  ┌─────▼────┐ ┌────▼─────┐ ┌───▼──────┐ ┌───▼───────┐ ┌────▼──────┐ ┌────▼─────┐
  │ad-handler│ │vast-     │ │track-    │ │redirect-  │ │postback-  │ │proxy-    │
  │/ad /GAM  │ │handler   │ │handler   │ │handler    │ │handler    │ │handler   │
  │          │ │/vast     │ │/adtrack  │ │/redirect  │ │/adtrack/  │ │/proxy-*  │
  │          │ │          │ │/vasttrack│ │           │ │postback   │ │/safeframe│
  └────┬─────┘ └──┬───┬───┘ │/tracking-│ └─────┬─────┘ └──┬───┬────┘ └────┬─────┘
       │          │   │     │pixel     │       │          │   │           │
       │          │   │     └──┬───┬───┘       │          │   │           │
       │      upstream │        │   │          │      upstream│       upstream
       │      VAST     │        │   └────┐     │      modatta/│       (JS/HTML/
       │      partners │        │        │     │      prezao  │        trackers)
       │               │        │        │     │              │
  ┌────▼───────────────▼────────▼──┐ ┌───▼─────▼──────────────▼───┐
  │   RDS Proxy → MySQL (existente)│ │      SQS tracking-queue     │
  │   leitura: hotspots/campaigns/ │ │       (+ DLQ)               │
  │   creatives/pixels/override    │ └──────────┬─────────────────┘
  │   escrita: ad_trackers (fase 1)│            │ batch(25)
  └────────────────────────────────┘ ┌──────────▼─────────────────┐
                                     │  tracker-writer (Lambda)    │
  ┌─────────────────────────────┐    │  MySQL ad_trackers INSERT   │
  │ report-handler              │    │  + DynamoDB AdTrackers      │
  │ GET /adtrack, /adtrack/xls  │    └──────────┬─────────────────┘
  │ (timeout maior, fora do hot │    ┌──────────▼─────────────────┐
  │  path)                      │    │ DynamoDB (existentes)       │
  └─────────────────────────────┘    │ AdTrackers · PostbackLogs   │
                                     └────────────────────────────┘
```

## 3. Microserviços (9 Lambdas)

| Lambda | Rotas | Memória | Timeout | Notas |
|---|---|---|---|---|
| `ad-handler` | GET /ad, GET /GAM, GET /health(z) | 256MB | 10s | hot path; cache hotspots |
| `vast-handler` | GET /vast | 512MB | 29s | mais crítica; upstream fetch + rewrite |
| `track-handler` | POST /adtrack, GET /vasttrack, GET /trackingpixel | 256MB | 10s | publica no SQS; pixel busca MySQL+download |
| `redirect-handler` | GET /redirect | 128MB | 5s | só gera HTML/JS |
| `postback-handler` | GET /adtrack/postback | 256MB | 29s | upstreams modatta/prezao (10s/30s) |
| `proxy-handler` | GET/OPTIONS /proxy-tracker, GET /proxy-audit, GET /safeframe/proxy-safeframe | 512MB | 29s | I/O-bound; resposta máx 2MB |
| `media-handler` | GET /media/{filename} | 256MB | 29s | HEAD S3 → redirect 302 p/ CloudFront ou stream; cache-miss = download p/ S3 |
| `report-handler` | GET /adtrack, GET /adtrack/xls | 1024MB | 120s | fora do hot path; agregação SQL GROUP BY |
| `tracker-writer` | (SQS event source, batch 25) | 256MB | 60s | INSERT MySQL + PutItem DynamoDB; partial batch failure |

**Decisão — tracking assíncrono:** o `track-handler` valida o evento e publica no SQS, retornando `201` imediatamente (latência ~5ms). O `tracker-writer` consome em batch e faz a dupla escrita (MySQL `ad_trackers` + DynamoDB `AdTrackers`) preservando o comportamento atual. O header `Location: /adtrack/{id}` passa a usar UUID (verificar que nenhum consumidor usa o ID — ver issue de validação).

**Decisão — escrita MySQL mantida na fase 1:** outros projetos leem `ad_trackers` no mesmo banco; a dupla escrita continua até a fase de banco de dados (Epic DB) decidir o destino final com coordenação entre projetos.

## 4. Estrutura do repositório (monorepo Go)

```
ad-serverless/
├── serverless.yml              # Serverless Framework — todas as functions + recursos
├── go.mod / go.sum
├── Makefile                    # build (GOOS=linux GOARCH=arm64), test, lint, deploy
├── cmd/                        # 1 main.go por Lambda (binário individual)
│   ├── ad/main.go
│   ├── vast/main.go
│   ├── track/main.go
│   ├── redirect/main.go
│   ├── postback/main.go
│   ├── proxy/main.go
│   ├── media/main.go
│   ├── report/main.go
│   └── trackerwriter/main.go
├── internal/
│   ├── domain/                 # structs: Campaign, Creative, HotSpot, AdTracker, enums
│   ├── frequencycap/           # parser "0;5>10;15>>" + elegibilidade
│   ├── selection/              # seleção aleatória campanha/creative
│   ├── templates/              # go:embed dos .vm + engine ${key}
│   ├── vast/                   # fetch, rewrite XML, partner rules, macros
│   ├── proxy/                  # proxy-tracker/audit/safeframe (reescritas JS)
│   ├── tracking/               # eventos, SQS publisher, validações
│   ├── repository/
│   │   ├── mysql/              # queries hot path (database/sql)
│   │   └── dynamo/             # AdTrackers, PostbackLogs
│   ├── cache/                  # TTL cache genérico (semântica Caffeine 5min/500)
│   ├── middleware/             # CORS, RequestValidation, logging, recover
│   ├── httpx/                  # client HTTP com timeouts, headers builder, IP real/geo
│   └── platform/               # config (env+SSM), slog, xray, response helpers
├── migrations/                 # vazio na fase 1 (schema intocado) — golang-migrate depois
├── docs/                       # este diretório
├── tests/
│   ├── golden/                 # fixtures Java→Go (saídas de template/VAST esperadas)
│   └── load/                   # k6
└── .github/workflows/          # ci.yml, deploy.yml, sync-issues.yml
```

## 5. Decisões de design (mini-ADRs)

### ADR-001 — Go puro com `aws-lambda-go`, sem framework HTTP
Cada Lambda recebe `events.APIGatewayV2HTTPRequest` e responde `events.APIGatewayV2HTTPResponse`. Um roteador mínimo interno (`internal/platform/router`) trata múltiplas rotas por função. Sem Gin/Echo: menos dependências, menos latência, sem camada de adaptação.

### ADR-002 — RDS Proxy na frente do MySQL existente
Lambdas escalam para N containers; cada um abre 1–2 conexões. Sem proxy, picos esgotariam `max_connections` do RDS compartilhado e derrubariam OUTROS projetos. RDS Proxy é **infra aditiva** (não muda schema nem afeta consumidores existentes). Pool por container: `SetMaxOpenConns(2)`, `SetConnMaxLifetime(5m)`.

### ADR-003 — SQS entre handler e escrita de tracking
Goroutines não sobrevivem ao freeze do container → fire-and-forget em memória perderia eventos. SQS garante durabilidade, absorve picos e habilita batch write (25 itens) no DynamoDB e multi-row INSERT no MySQL. DLQ com alarme para eventos não processáveis. Timestamp do evento é o do **request original** (param `time`), não o do processamento — paridade preservada.

### ADR-004 — Templates com `go:embed`
Os ~45 templates `.vm` são copiados como assets e embarcados no binário. Engine = `strings.NewReplacer` com os pares `${key}`→valor (mesma semântica do legado). Golden tests comparam saída Go × saída Java capturada.

### ADR-005 — Paridade primeiro, melhoria depois
Fase 1 replica comportamento byte-a-byte (incluindo hardcodes de hotspot do VAST). Melhorias (hotspots VAST em config dinâmica, validação de assinatura de postback, relatórios agregados no banco) são issues separadas marcadas `melhoria`, NUNCA misturadas com a portagem.

### ADR-006 — Sem mudança de banco na fase 1 (diretriz do engenheiro-chefe)
O MySQL é compartilhado e sem CI/CD de produção. As Lambdas usam o schema como está. O Epic "Banco de Dados" (fase final) fará: inventário de consumidores do banco, decisão de destino (Aurora/derivados/DynamoDB-only), plano de migração coordenado e CI/CD de migrations. Flyway permanece DESLIGADO nas Lambdas.

### ADR-007 — Domínio e cutover gradual
`ads.inteli.fi` aponta hoje para as EC2. O cutover usa CloudFront/Route53 com pesos (canary por rota): começa com rotas de menor risco (/health, /redirect), depois tracking, por último /vast. Rollback = voltar peso para EC2. As URLs `https://ads.inteli.fi/...` geradas dentro de VAST/templates funcionam em ambos os mundos durante a transição.

## 6. Performance e custo (estimativa)

- 2M req/dia ≈ 61M invocações/mês distribuídas em 9 funções.
- Lambda arm64 256MB, média 15ms (sem upstream): ~61M × 0,25GB×0,015s ≈ 230k GB-s ≈ **$4** + requests $12 ≈ **$16/mês** de compute base; proxies/vast com upstream elevam para ~$60–120/mês.
- HTTP API: $1,00/M → ~$61/mês. SQS: ~$25/mês (2M/dia). DynamoDB on-demand: mantém custo atual. RDS Proxy: ~$25/mês. CloudFront/S3 media: depende do tráfego de vídeo (~$20–80/mês).
- **Total estimado: US$ 200–350/mês** (vs. ~US$ 900–1.100 da proposta Java SnapStart + provisioned concurrency) + economia das 3 EC2 desligadas no cutover.
- Cold start Go arm64 medido tipicamente 40–80ms; sem necessidade de provisioned concurrency na carga atual (reavaliar com métricas reais no vast-handler).

## 7. Segurança

1. **Rotacionar IMEDIATAMENTE** a access key AWS exposta no repositório Java (issue P0).
2. IAM Role por função com least privilege (tabela DynamoDB específica, fila específica, bucket específico, SSM path específico).
3. Middleware `RequestValidationFilter` portado (anti-injection) + AWS WAF no CloudFront (rate limiting, SQLi/XSS managed rules).
4. OIDC GitHub Actions → AWS (sem chaves estáticas no CI).
5. Segredos só em SSM SecureString; carregados 1× por container e cacheados.
