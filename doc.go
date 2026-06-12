// Package adserverless é a raiz do monorepo Go da migração do ad-server +
// ad-commons (Java/Spring Boot em EC2) para microserviços em AWS Lambda.
//
// Este pacote NÃO contém lógica de negócio: existe para documentar o módulo
// e para que as ferramentas padrão (go test ./..., golangci-lint run ./...)
// tenham ao menos um pacote Go para analisar enquanto os pacotes de produção
// (cmd/, internal/) ainda não foram criados pelas issues dos milestones M1+.
//
// A estrutura de pastas deste repositório está definida em
// docs/arquitetura/ARQUITETURA-ALVO.md §4; o fluxo de trabalho por issue,
// em CLAUDE.md; a política de documentação em português, em
// CODE_DOCS_POLICY.md.
package adserverless
