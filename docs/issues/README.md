# Issues do Projeto — Fonte Versionada

> Cada arquivo `.md` neste diretório vira uma issue no GitHub via workflow [sync-issues](../../.github/workflows/sync-issues.yml) (push em `main`).
> O workflow é **idempotente** (casa por título): editar um arquivo atualiza a issue; criar um arquivo cria a issue. Issues antigas do planejamento Java+CDK são fechadas automaticamente como obsoletas.

## Formato do arquivo

```markdown
---
title: "[M0-01] Título da issue"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P0"]
milestone: "M0 — Fundação"
---
(corpo da issue em markdown)
```

## Inventário completo (69 issues, 12 milestones)

### M0 — Fundação (6)
| ID | Título | Depende de |
|---|---|---|
| M0-01 | Bootstrap do repositório Go (estrutura, go.mod, Makefile, lint) | — |
| M0-02 | serverless.yml base + hello-handler + stages dev/prod | M0-01 |
| M0-03 | CI GitHub Actions (lint, test, build) + OIDC AWS | M0-01 |
| M0-04 | Pipeline de deploy (dev automático, prod com aprovação) | M0-02, M0-03 |
| M0-05 | 🔴 P0 Segurança: rotação das chaves AWS expostas + SSM bootstrap | — |
| M0-06 | Governança: templates de PR/issue, branch protection, CODEOWNERS | M0-01 |

### M1 — Commons Go (8)
| ID | Título | Depende de |
|---|---|---|
| M1-01 | internal/domain: structs e enums do domínio | M0-01 |
| M1-02 | internal/frequencycap: parser de ranges + elegibilidade | M1-01 |
| M1-03 | internal/cache: cache TTL em memória (semântica Caffeine) | M0-01 |
| M1-04 | internal/selection: seleção aleatória + Null Object | M1-01, M1-02 |
| M1-05 | internal/templates: engine ${key} + go:embed + placeholders | M1-01 |
| M1-06 | internal/tracking: PostbackSignature MD5 + validação de eventos | M1-01 |
| M1-07 | internal/middleware: CORS + RequestValidation + recover | M0-01 |
| M1-08 | internal/httpx + platform: HTTP client, IP/geo, config SSM, slog | M0-01 |

### M2 — Infra AWS (7)
| ID | Título | Depende de |
|---|---|---|
| M2-01 | SQS tracking-queue + DLQ | M0-02 |
| M2-02 | S3 bucket de mídia + CloudFront | M0-02 |
| M2-03 | RDS Proxy na frente do MySQL existente (dev/prod) | M0-05 |
| M2-04 | IAM roles por função (least privilege) | M0-02 |
| M2-05 | API Gateway HTTP API: rotas, CORS, catch-all, error responses | M0-02 |
| M2-06 | WAF + rate limiting no CloudFront | M2-02 |
| M2-07 | Backup & disaster recovery (PITR, versioning, plano de DR) | M2-01, M2-02 |

### M3 — Tracking (7)
| ID | Título | Depende de |
|---|---|---|
| M3-01 | repository/mysql: conexão RDS Proxy + queries do hot path | M2-03, M1-08 |
| M3-02 | repository/dynamo: AdTrackers + PostbackLogs (formatos exatos) | M1-01 |
| M3-03 | track-handler: POST /adtrack + GET /vasttrack → SQS | M2-01, M1-06, M1-07 |
| M3-04 | tracker-writer: consumidor SQS → MySQL + DynamoDB | M3-01, M3-02 |
| M3-05 | track-handler: GET /trackingpixel | M3-01, M3-03 |
| M3-06 | redirect-handler: GET /redirect | M1-07, M1-08 |
| M3-07 | postback-handler: GET /adtrack/postback | M3-01, M3-02 |

### M4 — Ad Serving (4)
| ID | Título | Depende de |
|---|---|---|
| M4-01 | Migração dos 45 templates .vm + fixtures golden do Java | M1-05 |
| M4-02 | TemplateDecider: CreativeType → template | M4-01 |
| M4-03 | ad-handler: GET /ad (pipeline completo) | M4-02, M3-01, M1-04 |
| M4-04 | ad-handler: GET /GAM + health checks | M1-08 |

### M5 — VAST & Proxies (9)
| ID | Título | Depende de |
|---|---|---|
| M5-01 | internal/vast: fetch dinâmico (macros, params, headers) | M1-08 |
| M5-02 | internal/vast: rewrite XML (8 categorias de tag) | M5-01 |
| M5-03 | internal/vast: regras por parceiro (bypasses) + AdParameters | M5-02 |
| M5-04 | internal/vast: Impression→Tracking start (AdForce) | M5-03 |
| M5-05 | vast-handler: fluxo A (Campaign Direct + override) | M5-02, M3-01 |
| M5-06 | vast-handler: fluxo B (hotspots hardcoded) | M5-01 |
| M5-07 | Video cache S3 + media-handler | M2-02, M5-02 |
| M5-08 | proxy-handler: /proxy-tracker | M1-07, M1-08 |
| M5-09 | proxy-handler: /proxy-audit (rewrites JS) + /safeframe | M5-08 |

### M6 — Relatórios (2)
| ID | Título | Depende de |
|---|---|---|
| M6-01 | report-handler: GET /adtrack (JSON, agregação no banco) | M3-01 |
| M6-02 | report-handler: GET /adtrack/xls (excelize) | M6-01 |

### M7 — Observabilidade (4)
| ID | Título | Depende de |
|---|---|---|
| M7-01 | Logging estruturado padrão (slog JSON) em todos os serviços | M1-08 |
| M7-02 | Métricas EMF + X-Ray tracing | M7-01 |
| M7-03 | Dashboards CloudWatch por serviço | M7-02 |
| M7-04 | Alarmes (DLQ, erros, latência, RDS Proxy) + SNS | M7-02 |

### M8 — Qualidade (5)
| ID | Título | Depende de |
|---|---|---|
| M8-01 | Harness de golden tests + captura de fixtures do Java | M1-05 |
| M8-02 | Testes de carga k6 (10× volume) | M3-*, M4-*, M5-* |
| M8-03 | Especificação OpenAPI 3 dos 16 endpoints | — |
| M8-04 | Runbooks operacionais | M7-* |
| M8-05 | Security review completo | M2-*, M3-* |

### M9 — Cutover (5)
| ID | Título | Depende de |
|---|---|---|
| M9-01 | Verificação de contratos externos (Location header, consumidores) | — |
| M9-02 | Shadow traffic + comparação automática | M8-01 |
| M9-03 | Canary por rota (CloudFront/Route53) | M9-02 |
| M9-04 | Operação assistida + reconciliação diária de eventos | M9-03 |
| M9-05 | Descomissionamento das EC2 + arquivamento dos repos Java | M9-04 |

### M10 — Banco de Dados ⚠️ FINAL, COM CUIDADO (4)
| ID | Título | Depende de |
|---|---|---|
| M10-01 | Inventário de consumidores do MySQL compartilhado | M9-05 |
| M10-02 | ADR: decisão de destino do banco | M10-01 |
| M10-03 | CI/CD de migrations + staging de banco | M10-02 |
| M10-04 | Migração coordenada sem downtime | M10-03 |

### M11 — Backlog Pós-Cutover (8) — melhorias e novas features, NUNCA antes do M9
> Ideias aproveitadas do planejamento anterior (Qwen3.6) que valiam ser mantidas, adaptadas à arquitetura Go e às diretrizes do projeto.

| ID | Título | Origem |
|---|---|---|
| M11-01 | CPA Tracking — conversões com attribution window | issue antiga #31 |
| M11-02 | CPL Tracking — leads com deduplicação e webhooks | issue antiga #32 |
| M11-03 | Analytics avançado — dashboards de negócio e exports | issues antigas #33, #37 |
| M11-04 | Real-time streaming — Kinesis Firehose → S3 + OpenSearch | issue antiga #34 |
| M11-05 | Circuit breaker + fallbacks para upstreams de parceiros | issue antiga #35 |
| M11-06 | Event-count frequency cap com DynamoDB conditional writes | issue antiga #29 |
| M11-07 | Deploy gradual contínuo — aliases/CodeDeploy + feature flags | issue antiga #47 |
| M11-08 | Avaliação de cache compartilhado (DAX/ElastiCache) + cache VAST | issues antigas #7, #26 |

Ideias do planejamento anterior **rejeitadas conscientemente** (e por quê): migração imediata de campaigns/creatives/hotspots para DynamoDB e Flyway→PostgreSQL (#5, #6, #12, #27, #28 — violam a diretriz do banco compartilhado; viram opções do ADR em M10-02); cookie dedup em DynamoDB (#17 — o cookie é client-side e funciona em Lambda); novo esquema de chaves no tracking (#19 — quebraria as tabelas DynamoDB compartilhadas com o legado durante a transição); SnapStart/provisioned concurrency Java (#24 — não se aplica a Go).

## Labels padronizadas

- `epic:M0-fundacao` … `epic:M10-banco` · `epic:M11-backlog`
- `tipo:infra` · `tipo:port` (paridade) · `tipo:feature` · `tipo:seguranca` · `tipo:teste` · `tipo:docs` · `tipo:decisao`
- `prioridade:P0` (bloqueante/segurança) · `P1` (caminho crítico) · `P2` (normal) · `P3` (pós-cutover)
- `melhoria` — mudanças além da paridade (NUNCA misturar com `tipo:port`)
