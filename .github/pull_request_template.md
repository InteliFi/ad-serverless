# Pull Request

## O que este PR faz

<!-- resumo + "Closes #N" -->

## Origem no legado

<!-- arquivos Java portados, ex.: AdComponentImpl.java#getHotSpotAdScript -->

## Como testar

<!-- comandos e passos para validar -->

## Checklist (obrigatório)

- [ ] Código e comentários 100% em português ([CODE_DOCS_POLICY.md](../CODE_DOCS_POLICY.md))
- [ ] Toda lógica portada referencia o arquivo Java de origem em comentário (`// Portado de: ...`)
- [ ] [docs/MATRIZ-PARIDADE.md](../docs/MATRIZ-PARIDADE.md) atualizada (linhas desta issue)
- [ ] Golden tests incluídos/atualizados quando o PR altera saída (template/VAST/JS)
- [ ] `make lint` e `make test` verdes localmente
- [ ] Nenhum segredo/credencial/endpoint com senha commitado
- [ ] Sem mudança de schema MySQL (regra inegociável — Epic M10)
- [ ] Melhoria NÃO misturada com portagem (issues `melhoria` separadas)
