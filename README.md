# Ad-Serverless

> Migração completa do [ad-server](https://github.com/InteliFi/ad-server) (Java/Spring Boot, desde 2015) + [ad-commons](https://github.com/InteliFi/ad-commons) para **microserviços Go em AWS Lambda** com **Serverless Framework** — unificados neste repositório.

## 📊 Visão Geral

| Métrica | Valor |
|---------|-------|
| Volume atual | 2M+ requisições/dia (tracking timestampado de publicidade) |
| Stack origem | Java 17, Spring Boot 3.4.x, MySQL + DynamoDB, deploy manual em EC2 |
| Stack alvo | **Go 1.24+ · AWS Lambda (arm64) · Serverless Framework · API Gateway HTTP API · SQS · S3+CloudFront** |
| Metas | Alta disponibilidade, escala automática, p99 < 150ms, custo ~US$200–350/mês |

## 🎯 Princípios da migração

1. **Paridade total de features** — nenhum comportamento do sistema atual se perde. Rastreado linha a linha na [Matriz de Paridade](docs/MATRIZ-PARIDADE.md).
2. **Banco de dados por último, com cuidado** — o MySQL é compartilhado com outros projetos; a fase 1 usa o schema existente sem nenhuma alteração (via RDS Proxy). Mudanças de banco são o Epic final (M10), coordenado.
3. **Código 100% documentado em português** — [política obrigatória](CODE_DOCS_POLICY.md), verificada em lint e code review.
4. **Desenvolvimento com IA** — cada issue é autossuficiente e executada via Claude Code ([guia](CLAUDE.md)).

## 📚 Documentação

| Documento | Conteúdo |
|---|---|
| [docs/PLANO-MIGRACAO.md](docs/PLANO-MIGRACAO.md) | Plano mestre: epics, sequência, riscos, cutover |
| [docs/arquitetura/ARQUITETURA-ALVO.md](docs/arquitetura/ARQUITETURA-ALVO.md) | Stack, diagrama, 9 Lambdas, ADRs |
| [docs/MATRIZ-PARIDADE.md](docs/MATRIZ-PARIDADE.md) | Cada feature legada → serviço Go → status |
| [docs/legado/01-endpoints-http.md](docs/legado/01-endpoints-http.md) | Spec dos 16 endpoints HTTP |
| [docs/legado/02-logica-negocio.md](docs/legado/02-logica-negocio.md) | Algoritmos de negócio (seleção, frequency cap, tracking) |
| [docs/legado/03-pipeline-vast.md](docs/legado/03-pipeline-vast.md) | Pipeline VAST completo (o componente mais crítico) |
| [docs/legado/04-modelo-dados.md](docs/legado/04-modelo-dados.md) | Schema MySQL (30 migrations) + tabelas DynamoDB |
| [docs/legado/05-config-infra-deploy.md](docs/legado/05-config-infra-deploy.md) | Configuração operacional e deploy atual |
| [docs/issues/](docs/issues/) | Fonte versionada de todas as issues (sincronizadas via Actions) |

## 🏗️ Microserviços

| Lambda | Rotas | Epic |
|---|---|---|
| `ad-handler` | `GET /ad`, `GET /GAM`, health | M4 |
| `vast-handler` | `GET /vast` (3 fluxos + rewrite + partner rules) | M5 |
| `track-handler` | `POST /adtrack`, `GET /vasttrack`, `GET /trackingpixel` | M3 |
| `redirect-handler` | `GET /redirect` | M3 |
| `postback-handler` | `GET /adtrack/postback` (modatta, prezão) | M3 |
| `proxy-handler` | `/proxy-tracker`, `/proxy-audit`, `/safeframe/proxy-safeframe` | M5 |
| `media-handler` | `GET /media/{filename}` (S3 + CloudFront) | M5 |
| `report-handler` | `GET /adtrack`, `GET /adtrack/xls` | M6 |
| `tracker-writer` | consumidor SQS → MySQL + DynamoDB | M3 |

## 📋 Planejamento e tracking

O roadmap completo são **11 milestones (M0–M10)** com issues extremamente detalhadas:

👉 **[Issues abertas](https://github.com/InteliFi/ad-serverless/issues)** · **[Milestones](https://github.com/InteliFi/ad-serverless/milestones)**

As issues são geradas a partir de [docs/issues/](docs/issues/) pelo workflow [sync-issues](.github/workflows/sync-issues.yml) — editar/criar issues = editar os arquivos e fazer push.

## 🔴 Pontos críticos conhecidos

1. **Credenciais AWS expostas** no repositório Java legado — rotação imediata é a issue P0.
2. **VastService com 1279 linhas** — dividido em módulos Go (fetch, rewrite, partner-rules, video-cache) com golden tests por parceiro.
3. **`ad_trackers` com ~14M linhas** write-heavy — escrita via SQS + batch, leitura de relatório com agregação no banco.
4. **MySQL compartilhado** — RDS Proxy + máx 2 conexões por container para nunca afetar os outros projetos.
5. **Cache de vídeo local** (`/tmp`) — substituído por S3 + CloudFront.

## 📄 Licença

Proprietário — InteliFi / INTV Brasil
