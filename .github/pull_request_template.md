# Pull Request

## Issue

Closes #<!-- número da issue -->

## O que foi feito

<!-- Resumo do que foi implementado/portado e decisões tomadas -->

## Como testar

<!-- Comandos e passos para validar -->

## Checklist (obrigatório)

- [ ] Código 100% comentado em **português** conforme [CODE_DOCS_POLICY.md](../CODE_DOCS_POLICY.md)
- [ ] Lógica portada referencia o arquivo Java de origem (`// Portado de: ...`)
- [ ] `make lint` e `make test` verdes
- [ ] Golden tests incluídos/atualizados (quando a issue exige)
- [ ] [docs/MATRIZ-PARIDADE.md](../docs/MATRIZ-PARIDADE.md) atualizada (status das linhas desta issue)
- [ ] Nenhum segredo/credencial no código (config via env/SSM)
- [ ] Nenhuma mudança de schema no MySQL (proibido até o Epic M10)
