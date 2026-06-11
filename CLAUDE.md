# CLAUDE.md — Guia de Desenvolvimento com IA (ad-serverless)

Este repositório é a migração do **ad-server** + **ad-commons** (Java/Spring Boot em EC2) para **microserviços Go em AWS Lambda** com **Serverless Framework**. Todo o desenvolvimento é executado via Claude Code, issue por issue.

## Contexto essencial (leia antes de qualquer tarefa)

1. [docs/PLANO-MIGRACAO.md](docs/PLANO-MIGRACAO.md) — plano mestre, sequência de epics, regras de execução.
2. [docs/arquitetura/ARQUITETURA-ALVO.md](docs/arquitetura/ARQUITETURA-ALVO.md) — stack, 9 Lambdas, estrutura do repo, ADRs.
3. [docs/legado/](docs/legado/) — especificações funcionais EXATAS do sistema Java (5 documentos). **A fonte da verdade de comportamento.**
4. [docs/MATRIZ-PARIDADE.md](docs/MATRIZ-PARIDADE.md) — rastreio de cada feature; atualizar a cada PR.
5. Código-fonte Java original (referência local): `c:\Users\Fabio\Documents\Dev\ad-server` e `c:\Users\Fabio\Documents\Dev\ad-commons`.

## Regras inegociáveis

1. **Paridade primeiro.** Fase 1 replica o comportamento Java byte-a-byte (até os hardcodes). Melhorias só em issues marcadas `melhoria`.
2. **Banco de dados intocável.** O MySQL é compartilhado com outros projetos. NENHUM `CREATE/ALTER/DROP`, nenhuma migration, nenhum Flyway. Apenas `SELECT` e `INSERT` no schema existente. Mudanças de banco = Epic M10, no final, com aprovação humana.
3. **Código 100% comentado em português.** Ver [CODE_DOCS_POLICY.md](CODE_DOCS_POLICY.md). Toda função exportada tem doc comment godoc em português explicando O QUE faz e POR QUÊ existe no contexto do ad server.
4. **Toda lógica portada referencia a origem:** `// Portado de: VastService.java (rewrite de <Impression>)`.
5. **1 issue = 1 branch = 1 PR.** Branch `feat/issue-N-slug`. PR referencia `Closes #N` e atualiza a Matriz de Paridade.
6. **Testes obrigatórios.** Unitários para lógica; golden tests para saídas (templates/VAST/JS rewrites) comparando com fixtures capturadas do Java em `tests/golden/`.
7. **Sem segredos no código.** Configuração via env vars (serverless.yml) e SSM Parameter Store. Nunca copiar as credenciais do repo Java legado (estão comprometidas e serão rotacionadas).

## Stack e convenções

- **Go 1.24+**, módulos: `github.com/InteliFi/ad-serverless`.
- Lambdas em `cmd/<serviço>/main.go`; lógica compartilhada em `internal/` (ver estrutura na arquitetura).
- Handler: `aws-lambda-go` com `events.APIGatewayV2HTTPRequest/Response` (HTTP API payload v2).
- Erros: embrulhar com `fmt.Errorf("contexto: %w", err)`; nunca panic em handler (middleware recover).
- Logs: `log/slog` JSON em stdout; campos padrão `service`, `route`, `cid`, `hid` quando aplicável.
- Datas: SEMPRE `time.LoadLocation("America/Sao_Paulo")` explícito; importar `_ "time/tzdata"` nos mains.
- HTTP client: compartilhado por container, timeouts explícitos (default 60s; proxy-audit 10s connect/30s read).
- Aleatoriedade: `math/rand/v2` (`rand.IntN(n)` — uniforme, como `Random.nextInt`).
- MySQL: `database/sql` + `go-sql-driver/mysql`, queries à mão (sem ORM), `SetMaxOpenConns(2)`, `SetConnMaxLifetime(5*time.Minute)`, DSN via SSM.
- DynamoDB: `aws-sdk-go-v2`; preservar nomes de atributos e formatos de chave EXATOS (`created_at_id` = `<ISO8601>#<rds_id>`).
- Templates: assets em `internal/templates/assets/` com `go:embed`; engine = substituição `${key}` literal.

## Comandos

```bash
make build          # compila todas as Lambdas (GOOS=linux GOARCH=arm64, -tags lambda.norpc)
make test           # go test ./... (unit + golden)
make lint           # golangci-lint run
make deploy-dev     # serverless deploy --stage dev
make deploy-prod    # somente via GitHub Actions com aprovação manual
go test ./internal/vast/ -run TestGolden -v   # golden tests do VAST
```

## Fluxo de trabalho por issue

1. Ler a issue completa (`docs/issues/<arquivo>.md` ou GitHub) e os docs de legado referenciados nela.
2. Conferir dependências (`Bloqueada por`) — não iniciar se a dependência não foi mergeada.
3. Criar branch a partir de `main`: `git checkout -b feat/issue-N-slug`.
4. Implementar com TDD onde possível; capturar fixtures do Java quando a issue pedir golden test.
5. `make lint && make test` verdes.
6. Atualizar [docs/MATRIZ-PARIDADE.md](docs/MATRIZ-PARIDADE.md) (status da(s) linha(s) da issue).
7. Commit convencional em português: `feat(vast): porta rewrite de Impression (issue #23)`.
8. Abrir PR com descrição: o que foi portado, decisões tomadas, como testar, `Closes #N`.

## O que NUNCA fazer

- Tocar no schema do MySQL (nem em dev — espelha produção de outros projetos).
- Misturar melhoria com portagem no mesmo PR.
- Remover comportamento "estranho" do legado sem issue `melhoria` aprovada (ex.: hotspots hardcoded do VAST, download do pixel a cada request — são features em produção).
- Usar `time.Now()` direto na lógica de negócio — receber `time.Time`/`clock` injetado (testabilidade, como o `Clock` do Java).
- Commitar credenciais, endpoints RDS com senha, ou as chaves AWS do legado.
