---
title: "[M0-01] Bootstrap do repositório Go (estrutura, go.mod, Makefile, lint)"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P0"]
milestone: "M0 — Fundação"
---
## Contexto

Esta é a primeira issue executável do projeto. O repositório `ad-serverless` hoje contém apenas documentação (`docs/`, `CLAUDE.md`, `CODE_DOCS_POLICY.md`). Precisamos do esqueleto Go completo para que TODAS as issues seguintes (M1 em diante) tenham onde colocar código, com lint e build padronizados desde o dia 1.

A estrutura de pastas é a definida em [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §4 ("Estrutura do repositório (monorepo Go)") e os comandos `make` são os documentados em [CLAUDE.md](../../CLAUDE.md) §Comandos. O legado Java usava Maven multi-módulo (ad-server + ad-commons); aqui é um monorepo Go único com 1 binário por Lambda.

## Especificação detalhada

### 1. `go.mod`
```
module github.com/InteliFi/ad-serverless

go 1.24
```
Sem dependências ainda — `github.com/aws/aws-lambda-go` entra na M0-02 junto com o hello-handler.

### 2. Estrutura de pastas (criar TODAS, exatamente como ARQUITETURA-ALVO §4)
Pastas vazias recebem `.gitkeep`. Pacotes `internal/` ainda NÃO recebem código (issues M1):
```
cmd/                          # main.go por Lambda — criados nas issues de cada serviço
internal/domain/
internal/frequencycap/
internal/selection/
internal/templates/assets/
internal/vast/
internal/proxy/
internal/tracking/
internal/repository/mysql/
internal/repository/dynamo/
internal/cache/
internal/middleware/
internal/httpx/
internal/platform/
migrations/                   # VAZIO na fase 1 (ADR-006 — schema intocado)
tests/golden/
tests/load/
.github/workflows/
```

### 3. `Makefile`
Targets obrigatórios (binário Lambda `provided.al2023` exige nome `bootstrap`):
```makefile
SERVICES := $(notdir $(wildcard cmd/*))

build: ## Compila todas as Lambdas (1 binário bootstrap por cmd/*)
	@for svc in $(SERVICES); do \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags lambda.norpc -ldflags="-s -w" \
		-o bin/$$svc/bootstrap ./cmd/$$svc; \
	done

test: ## Roda todos os testes com cobertura
	go test ./... -coverprofile=coverage.out

lint: ## Lint completo
	golangci-lint run ./...

deploy-dev: build ## Deploy no stage dev
	npx serverless deploy --stage dev
```
- `lambda.norpc` remove o RPC legado do `aws-lambda-go` (binário menor, start mais rápido).
- `-ldflags="-s -w"` reduz o tamanho do binário (sem símbolos de debug).
- O loop usa `$(wildcard cmd/*)` para que novos serviços entrem no build automaticamente.

### 4. `.golangci.yml`
Deve forçar a política de documentação ([CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) — todo identificador exportado com doc comment em português):
```yaml
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - revive
    - ineffassign
    - misspell
    - gofmt
    - goimports
linters-settings:
  revive:
    rules:
      - name: exported        # OBRIGA doc comment em todo exportado
        severity: error
      - name: package-comments
        severity: error
issues:
  exclude-dirs:
    - tests/load
```

### 5. `.gitignore`
Mínimo: `bin/`, `coverage.out`, `.serverless/`, `node_modules/`, `.env*`, `*.zip`, `.DS_Store`, `dist/`.

### 6. `.editorconfig`
`root = true`; Go com tabs (`indent_style = tab`); YAML/JSON/Markdown com 2 espaços; `end_of_line = lf`; `insert_final_newline = true`; `charset = utf-8`.

## Arquivos a criar/alterar

- `go.mod`
- `Makefile`
- `.golangci.yml`
- `.gitignore`
- `.editorconfig`
- `cmd/.gitkeep`, `internal/**/.gitkeep` (todas as pastas da §4 da arquitetura), `migrations/.gitkeep`, `tests/golden/.gitkeep`, `tests/load/.gitkeep`, `.github/workflows/.gitkeep`

## Critérios de aceite

- [ ] `go build ./...` executa sem erro (mesmo sem pacotes ainda)
- [ ] `make build` roda sem erro (loop vazio é aceitável até existir `cmd/*`)
- [ ] `make lint` roda `golangci-lint` sem erro de configuração (versão do golangci-lint documentada no Makefile ou em comentário do `.golangci.yml`)
- [ ] `make test` executa `go test ./...` sem falha
- [ ] Regra `revive: exported` ativa como `error` — criar um arquivo Go de teste com função exportada sem comentário e confirmar que o lint FALHA (depois remover o arquivo)
- [ ] Estrutura de pastas idêntica à ARQUITETURA-ALVO §4 (validável com `tree`/`Get-ChildItem`)
- [ ] `go.mod` declara `module github.com/InteliFi/ad-serverless` e `go 1.24`
- [ ] Nenhum segredo, credencial ou endpoint de banco commitado

## Dependências

Bloqueada por: nenhuma

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (stack), §4 (estrutura)
- [CLAUDE.md](../../CLAUDE.md) — §Stack e convenções, §Comandos
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) — política de comentários em português
- Legado: `ad-server/pom.xml` e `ad-commons/pom.xml` (estrutura Maven que está sendo substituída) — ver [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §4

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-01] Bootstrap do repositório Go no repo
InteliFi/ad-serverless, seguindo EXATAMENTE a especificação da issue
(estrutura ARQUITETURA-ALVO §4, go.mod com module github.com/InteliFi/ad-serverless
e Go 1.24, Makefile com build GOOS=linux GOARCH=arm64 CGO_ENABLED=0
-tags lambda.norpc, .golangci.yml com revive/exported como error).
Todo código e comentário em português (CODE_DOCS_POLICY.md).
Validar make build/test/lint verdes. Ao final: abrir PR referenciando a issue.
```
