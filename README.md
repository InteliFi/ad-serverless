# Ad-Serverless

> Migração completa do ad-server Java Spring Boot (desde 2015) + ad-commons para arquitetura serverless em microserviços AWS Lambda.

## 📊 Visão Geral

| Métrica | Valor |
|---------|-------|
| Volume atual | 2M+ requisições/dia |
| Stack atual | Java 17, Spring Boot 3.4.x, MySQL + HikariCP |
| Deploy atual | Scripts shell manuais → EC2 (Dev: us-east-1, Prod: sa-east-1 × 2) |
| Repositórios origem | [ad-server](https://github.com/InteliFi/ad-server) (~60 classes), [ad-commons](https://github.com/InteliFi/ad-commons) (~37 classes) |

## 🎯 Objetivo

Migrar o ad-server e ad-commons para uma arquitetura serverless em microserviços na AWS Lambda, unificados neste repositório `ad-serverless`.

### Novas Features Planejadas
- **CPA Tracking** (Cost Per Acquisition) — tracking completo de conversões com attribution window
- **CPL Tracking** (Cost Per Lead) — recebimento, deduplicação e métricas de leads
- **Analytics Avançado** — dashboards com tendências, comparação A/B, export multi-formato
- **Real-time Streaming** — Kinesis Firehose → OpenSearch para dashboards em tempo real

## 🏗️ Stack Tecnológica Alvo

| Decisão | Escolha | Justificativa |
|---------|---------|---------------|
| Linguagem | Java 21 + Spring Boot 3.4.x | SnapStart elimina cold start, virtual threads para proxy I/O |
| IaC | AWS CDK v2.x (TypeScript) | Type safety, constructs reutilizáveis |
| API Gateway | HTTP API + Response Caching | 3x menos latência, 50% mais barato que REST API |
| DB Hot Path | DynamoDB on-demand → provisioned + auto-scale | Write-heavy, sem gestão de conexão |
| DB Analytics | Aurora Serverless v2 (PostgreSQL) via RDS Proxy | SQL completo para relatórios e JOINs complexos |
| Cache | Caffeine (local) + API Gateway Cache | Sub-ms hit rate, elimina 60-80% invocações Lambda |
| Async Tracking | SQS Standard → Lambda consumer → DynamoDB BatchWrite | Fire-and-forget confiável com DLQ |
| Cold Start | SnapStart + Provisioned Concurrency (min=10 em vast-handler) | <50ms sempre |
| Observability | ADOT Lambda Layer → X-Ray + CloudWatch custom metrics | Auto-instrumentação, tracing end-to-end |

## 💰 Custo Estimado Mensal: ~$900-1.100/mês otimizado

## 📁 Estrutura do Repositório (planejada)

```
ad-serverless/
├── cdk/                    # AWS CDK TypeScript - Infraestrutura
│   ├── lib/
│   │   ├── vast-handler.construct.ts
│   │   ├── tracking-handler.construct.ts
│   │   ├── database.construct.ts
│   │   └── ...
├── services/               # Microserviços Java (Maven multi-module)
│   ├── commons/            # ad-commons migrado e modernizado
│   ├── vast-handler/       # Lambda: VAST XML generation + proxy
│   ├── ad-handler/         # Lambda: Banner/script serving
│   ├── tracking-pixel-handler/  # Lambda: Tracking pixel + events
│   ├── redirect-handler/   # Lambda: Click redirect com GA
│   ├── postback-handler/   # Lambda: CPA/CPL postbacks
│   ├── tracker-writer/     # Lambda: SQS consumer → DynamoDB batch write
│   └── analytics-service/  # Serviço de relatórios e métricas
├── migrations/             # Flyway + AWS DMS scripts
├── docs/                   # Documentação completa (ADRs, OpenAPI, runbooks)
└── .github/workflows/      # CI/CD GitHub Actions
```

## 📋 Planejamento de Migração

O planejamento completo está dividido em **6 Epics** com ~40 issues detalhados:

| Epic | Issues | Foco |
|------|--------|------|
| [Epic 1: Infraestrutura AWS](https://github.com/InteliFi/ad-serverless/milestone/1) | 8 | CDK, CI/CD GitHub Actions, DynamoDB, Aurora, ElastiCache, SQS, CloudFront+WAF |
| [Epic 2: Core Microservices](https://github.com/InteliFi/ad-serverless/milestone/2) | 16 | Maven multi-module, 8 Lambdas (vast, ad, tracking-pixel, redirect, postback, tracker-writer, proxy-audit, proxy-tracker), templates, SnapStart |
| [Epic 3: Database Migration](https://github.com/InteliFi/ad-serverless/milestone/3) | 4 | Flyway → PostgreSQL, DMS MySQL→DynamoDB+Aurora, frequency cap conditional writes, backup+DR |
| [Epic 4: Novas Features](https://github.com/InteliFi/ad-serverless/milestone/4) | 6 | CPA tracking, CPL leads, analytics dashboard, Kinesis Firehose real-time, circuit breaker + retry |
| [Epic 5: Observability](https://github.com/InteliFi/ad-serverless/milestone/5) | 3 | CloudWatch dashboards+alarms, Micrometer custom metrics, structured JSON logging |
| [Epic 6: Documentação & Qualidade](https://github.com/InteliFi/ad-serverless/milestone/6) | 8 | JavaDoc em português, CLAUDE.md, OpenAPI/Swagger, ADRs, testes JUnit5+k6, security review, deploy strategy |

### Todos os Issues
👉 [Ver todos os issues abertos](https://github.com/InteliFi/ad-serverless/issues)

## 🔴 Pontos Críticos Identificados

1. **VastService tem 1279 linhas** — monólito que deve ser dividido em 3-4 serviços menores
2. **ad_trackers com ~14M rows** — precisa de estratégia de partição por data no DynamoDB
3. **AWS credentials hardcoded** no application.properties (AKIAVR67P7UR7PR2J6QC) — CRÍTICO remover e rotacionar chaves expostas
4. **Template engine é substituição simples `${key}`** — não usa Velocity real, facilita migração
5. **Video cache stateful** em `/tmp/adserver_video_cache` — precisa de S3 ou EFS no Lambda

## 📄 Licença

Proprietário — InteliFi/INTV Brasil
