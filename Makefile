# Makefile do ad-serverless — build, teste, lint e deploy das Lambdas Go.
#
# Por que estas flags de build (ver docs/arquitetura/ARQUITETURA-ALVO.md §1 e §4):
#   - O runtime provided.al2023 exige que o binário se chame "bootstrap".
#   - GOOS=linux GOARCH=arm64: as Lambdas rodam em Graviton (arm64).
#   - CGO_ENABLED=0: binário estático, sem dependência de libc do host.
#   - -tags lambda.norpc: remove o modo RPC legado do aws-lambda-go
#     (binário menor, cold start mais rápido).
#   - -ldflags="-s -w": remove símbolos de debug (binário menor).
#
# Ferramentas esperadas:
#   - Go 1.24+
#   - golangci-lint v1.64.8 (série v1.x — o formato do .golangci.yml deste
#     repo é o da v1; a série v2 mudou o formato e NÃO é compatível)
#   - Node.js + npx (Serverless Framework, somente para deploy)

# A lista de serviços é derivada das pastas em cmd/ — um novo serviço
# (cmd/<nome>/main.go) entra no build automaticamente, sem editar o Makefile.
SERVICES := $(notdir $(wildcard cmd/*))

# Empacotador de zip da Lambda. Usamos a ferramenta oficial do aws-lambda-go
# em vez do comando `zip` porque: (1) `zip` não existe no Windows/Git Bash;
# (2) mesmo quando existe, não grava o bit de execução POSIX do bootstrap
# dentro do zip — e o runtime provided.al2023 EXIGE bootstrap executável
# (0755), senão o deploy sobe um artefato inválido. A versão é pinada na
# mesma do go.mod.
BUILD_LAMBDA_ZIP := go run github.com/aws/aws-lambda-go/cmd/build-lambda-zip@v1.54.0

.PHONY: build test lint deploy-dev

build: ## Compila e empacota todas as Lambdas (bootstrap + zip por cmd/*)
	@for svc in $(SERVICES); do \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags lambda.norpc -ldflags="-s -w" \
		-o bin/$$svc/bootstrap ./cmd/$$svc; \
		$(BUILD_LAMBDA_ZIP) -o bin/$$svc/$$svc.zip bin/$$svc/bootstrap; \
	done

test: ## Roda todos os testes com cobertura
	go test ./... -coverprofile=coverage.out

lint: ## Lint completo (golangci-lint v1.64.8)
	golangci-lint run ./...

deploy-dev: build ## Deploy no stage dev
	npx serverless deploy --stage dev
