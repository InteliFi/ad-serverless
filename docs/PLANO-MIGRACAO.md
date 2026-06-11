# Plano Mestre de Migração — ad-server + ad-commons → ad-serverless (Go)

> **Status:** planejamento aprovado para execução. Issues criadas no [GitHub](https://github.com/InteliFi/ad-serverless/issues) e sincronizadas a partir de `docs/issues/`.
> **Diretriz inegociável:** nenhuma feature do sistema atual pode ser perdida. A [Matriz de Paridade](MATRIZ-PARIDADE.md) rastreia cada comportamento.
> **Diretriz do engenheiro-chefe:** mudanças no banco de dados ficam para o FINAL, com muito cuidado — o MySQL é compartilhado com outros projetos e não há CI/CD de produção configurado.

## Sequência de Epics (Milestones no GitHub)

```
M0 Fundação ──► M1 Commons Go ──► M2 Infra AWS ──► M3 Tracking ──► M4 Ad Serving ──► M5 VAST & Proxies
                                                        │                                   │
                                                        └────────────► M6 Relatórios ◄─────┘
M7 Observabilidade (paralelo a M3–M6) ──► M8 Qualidade & Golden Tests ──► M9 Cutover ──► M10 Banco de Dados (FINAL, cuidadoso)
```

| Epic | Objetivo | Entregável de aceite |
|---|---|---|
| **M0 — Fundação** | Repo Go funcional com CI e deploy dev | `serverless deploy --stage dev` publica um hello-handler com sucesso via GitHub Actions |
| **M1 — Commons Go** | Port do ad-commons: domain, frequency cap, digit parser, utils, signature, template engine, cache TTL | 100% testes unitários verdes, incluindo casos extraídos do Java |
| **M2 — Infra AWS** | SQS+DLQ, S3 media, CloudFront, SSM, RDS Proxy, IAM roles, WAF | recursos provisionados nos 2 stages; conexão MySQL via proxy validada |
| **M3 — Tracking** | track-handler, tracker-writer, redirect, postback, pixel | eventos fluem fim-a-fim (API→SQS→MySQL+DynamoDB) em dev, com paridade de dados validada |
| **M4 — Ad Serving** | ad-handler (/ad, /GAM) + 45 templates embarcados | golden tests: saída idêntica ao Java para os mesmos inputs |
| **M5 — VAST & Proxies** | vast-handler (3 fluxos + rewrite + partner rules), proxy-handler (3 proxies), media-handler (S3) | golden tests de VAST; players reais validados em dev |
| **M6 — Relatórios** | report-handler (JSON + XLS) com agregação no banco | mesmos números do relatório legado para um período de teste |
| **M7 — Observabilidade** | slog JSON, métricas EMF, X-Ray, dashboards, alarmes | dashboard por serviço; alarmes DLQ/erro/latência ativos |
| **M8 — Qualidade** | golden tests completos, k6 (10× carga), OpenAPI, runbooks, security review | k6 sem erros a 250 rps; checklist de segurança fechado |
| **M9 — Cutover** | shadow traffic, canary por rota, rollback, desligamento EC2 | 100% do tráfego nas Lambdas por 2 semanas sem regressão; EC2 desligadas |
| **M10 — Banco de Dados** ⚠️ | inventário de consumidores, decisão de destino, migração coordenada, CI/CD de migrations | plano aprovado pelo engenheiro-chefe; execução sem downtime |

## Fases de risco do cutover (M9)

1. **Shadow:** CloudFront duplica % do tráfego para Lambdas (resposta descartada); compara-se métricas/logs.
2. **Canary por rota**, da menor para a maior criticidade: `/health` → `/redirect` → `/trackingpixel` → `/adtrack`+`/vasttrack` → `/adtrack/postback` → `/ad`+`/GAM` → `/proxy-*` → `/media` → `/vast`.
3. Cada rota: 5% → 25% → 50% → 100%, com janela de observação de 24–48h e rollback de 1 clique (peso DNS/CloudFront).
4. EC2 permanecem quentes por 2 semanas após 100%.

## Regras de execução com Claude Code

Cada issue é autossuficiente: contém contexto, referências às specs (`docs/legado/*`), arquivos a criar, critérios de aceite e o comando sugerido. Fluxo padrão por issue:

```
1. Abrir sessão no repo ad-serverless
2. /goal Implementar a issue #N do GitHub (InteliFi/ad-serverless) seguindo
   exatamente a especificação da issue e docs/legado referenciados. Código
   100% comentado em português (CODE_DOCS_POLICY.md). Testes incluídos e
   verdes. Ao final: PR aberto referenciando a issue.
3. Revisar PR (humano) → merge → próxima issue
```

- **1 issue = 1 branch = 1 PR.** Branches: `feat/issue-N-slug` ou `infra/issue-N-slug`.
- Issues marcadas `bloqueada-por:#N` só começam após o merge da dependência.
- Toda lógica portada referencia o arquivo Java original em comentário (`// Portado de: AdComponentImpl.java#getHotSpotAdScript`).
- Golden tests: antes de portar um componente de saída (template/VAST), capturar a saída Java real (curl no ambiente dev EC2) e gravar em `tests/golden/`.

## Riscos e mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Esgotar conexões do MySQL compartilhado | derruba outros projetos | RDS Proxy (M2) + `SetMaxOpenConns(2)` + alarme de conexões |
| Diferença sutil no rewrite de VAST | perda de receita/tracking de parceiros | golden tests por parceiro (Google, Space, AdForce, SmartAd, Metrike) + shadow traffic |
| Perda de eventos no tracking assíncrono | métricas furadas | SQS com DLQ + alarme; comparação diária de contagens MySQL×DynamoDB no canary |
| API Gateway rejeitar URLs malformadas que o Tomcat relaxado aceitava | players quebrados | testes com URLs reais capturadas dos logs; rota catch-all + decode manual |
| Chave AWS exposta no repo legado | comprometimento da conta | rotação imediata (issue P0 em M0) |
| Header `Location` de /adtrack com ID consumido por alguém | quebra silenciosa | issue de verificação nos consumidores antes do canary de tracking |
| Serverless Framework v4 exigir licença | bloqueio de deploy | usar v3 OSS ou compose; decidir em M0 |

## Novas features (pós-paridade — backlog)

Registradas no milestone **M11 — Backlog Pós-Cutover** (8 issues), executam **somente após M9**: CPA tracking com attribution window, CPL com deduplicação e webhooks, analytics avançado, streaming real-time (Kinesis→OpenSearch), circuit breaker por parceiro, event-count frequency cap (implementa as colunas `event_cap*` que o legado nunca usou), deploy gradual contínuo com feature flags e avaliação de cache compartilhado. Ver [docs/issues/README.md](issues/README.md) §M11 — inclui o registro das ideias do planejamento anterior aproveitadas e das rejeitadas (com justificativa).
