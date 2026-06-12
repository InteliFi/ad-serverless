# Runbook — Governança do repositório ad-serverless

> **Objetivo:** manter guard-rails automáticos que impeçam violação das regras
> inegociáveis (paridade, matriz, português, referência ao Java, sem segredos,
> sem mudança de schema) em um fluxo onde o desenvolvimento é feito por IA
> (Claude Code) com revisão humana.

## Índice

1. [Regras ativas](#1-regras-ativas)
2. [Branch protection do `main`](#2-branch-protection-do-main)
3. [Convenções de branch e commit](#3-convenções-de-branch-e-commit)
4. [Como ajustar as regras](#4-como-ajustar-as-regras)

---

## 1. Regras ativas

| Regra | Onde está definida | O que faz |
|---|---|---|
| Template de PR | `.github/PULL_REQUEST_TEMPLATE.md` | Checklist obrigatório: português, referência Java, matriz paridade, golden tests, lint/test verdes, sem segredos, sem schema change, melhoria separada |
| Templates de issue | `.github/ISSUE_TEMPLATE/` | `bug.yml` e `melhoria.yml` com campos estruturados; blank desabilitado (`config.yml`) |
| CODEOWNERS | `.github/CODEOWNERS` | Revisão humana obrigatória em todo PR (desenvolvimento por IA); infra exige revisão extra |
| Branch protection | GitHub API (aplicada via comando abaixo) | Sem push direto no `main`; PR obrigatório; 1 aprovação + code owners; checks `lint`/`test`/`build` verdes e atualizados com main (`strict`) |

### Fluxo de issues versionadas

Issues planejadas nascem dos arquivos em `docs/issues/`, não manualmente:
1. Criar/editar arquivo em `docs/issues/MX-NN-titulo.md`
2. Fazer push na branch → merge no `main`
3. O workflow `sync-issues.yml` (CI) cria/atualiza a issue no GitHub automaticamente

Para bugs e melhorias não planejadas, use os templates (`New Issue` → `Bug` ou `Melhoria`).

---

## 2. Branch protection do `main`

### Aplicar via GitHub API

```bash
gh api -X PUT repos/InteliFi/ad-serverless/branches/main/protection \
  -F required_status_checks[strict]=true \
  -F "required_status_checks[contexts][]=lint" \
  -F "required_status_checks[contexts][]=test" \
  -F "required_status_checks[contexts][]=build" \
  -F enforce_admins=false \
  -F required_pull_request_reviews[required_approving_review_count]=1 \
  -F required_pull_request_reviews[require_code_owner_reviews]=true \
  -F restrictions=null \
  -F allow_force_pushes=false \
  -F allow_deletions=false
```

### Regras aplicadas

| Configuração | Valor | Por quê |
|---|---|---|
| `required_status_checks.strict` | `true` | PR precisa estar atualizado com main (evita merge de código desatualizado) |
| `contexts: lint, test, build` | checks do `ci.yml` (M0-03) | Código não compila ou falha em teste → não entra no main |
| `required_approving_review_count` | `1` | Pelo menos 1 aprovação humana obrigatória |
| `require_code_owner_reviews` | `true` | CODEOWNERS são notificados e precisam aprovar (revisão IA → humano) |
| `allow_force_pushes` | `false` | Impede reescrita de história no main |
| `allow_deletions` | `false` | Branch main não pode ser deletada acidentalmente |
| `enforce_admins` | `false` | Admins podem bypass em emergência (rollback rápido) |

### Se o CI ainda não existe

Se a M0-03 (`ci.yml`) ainda não mergeou, aplicar sem `required_status_checks`:

```bash
gh api -X PUT repos/InteliFi/ad-serverless/branches/main/protection \
  -F enforce_admins=false \
  -F required_pull_request_reviews[required_approving_review_count]=1 \
  -F required_pull_request_reviews[require_code_owner_reviews]=true \
  -F restrictions=null \
  -F allow_force_pushes=false \
  -F allow_deletions=false
```

Completar os checks depois do merge da M0-03 (rodar o comando completo acima).

### Verificar a proteção aplicada

```bash
gh api repos/InteliFi/ad-serverless/branches/main/protection | jq '{
  status_checks: .required_status_checks,
  pr_reviews: .required_pull_request_reviews,
  force_pushes: .allow_force_pushes.enabled,
  deletions: .allow_deletions.enabled
}'
```

---

## 3. Convenções de branch e commit

### Branch

Formato: `<tipo>/issue-N-slug` onde `tipo` é um dos:

| Tipo | Uso | Exemplo |
|---|---|---|
| `feat` | Portagem de funcionalidade do Java ou feature nova | `feat/issue-23-vast-impression-rewrite` |
| `docs` | Documentação, runbooks, ADRs | `docs/issue-54-governanca-repo` |
| `infra` | Infraestrutura (pipeline, CI, CloudFormation) | `infra/issue-51-ci-codepipeline` |
| `fix` | Correção de bug identificado | `fix/issue-67-postback-signature-timestamp` |

Sempre criar a partir de `main`:
```bash
git checkout -b feat/issue-N-slug origin/main
```

### Commit

Formato convencional em português: `<tipo>(<escopo>?): descrição curta`

| Tipo | Uso |
|---|---|
| `feat` | Nova funcionalidade portada do Java ou feature original |
| `docs` | Documentação, comentários, runbooks |
| `fix` | Correção de bug |
| `refactor` | Refatoração sem mudança de comportamento |
| `test` | Testes unitários, golden tests, fixtures |
| `chore` | Configuração, CI, dependências |

Exemplos:
```
feat(vast): porta rewrite de Impression (issue #23)
docs(seg): runbook de rotação de chaves AWS expostas (issue #53)
fix(postback): corrige timestamp da assinatura em fuso SPT (issue #67)
test(golden): fixture VAST impression do Java (issue #23)
```

---

## 4. Como ajustar as regras

### Adicionar um check obrigatório no CI

1. Adicionar o job ao `.github/workflows/ci.yml` com o nome desejado
2. Atualizar a branch protection para incluir o contexto:
   ```bash
   gh api -X PUT repos/InteliFi/ad-serverless/branches/main/protection \
     -F required_status_checks[strict]=true \
     -F "required_status_checks[contexts][]=lint" \
     -F "required_status_checks[contexts][]=test" \
     -F "required_status_checks[contexts][]=build" \
     -F "required_status_checks[contexts][]=<novo-job>" \
     ... (resto dos parâmetros)
   ```

### Adicionar um code owner para uma pasta nova

1. Adicionar linha ao `.github/CODEOWNERS`:
   ```
   /nova-pasta/    @usuario-github
   ```
2. Fazer PR com a mudança (o próprio PR será revisado pelo owner existente)

### Remover branch protection (emergência)

```bash
gh api -X DELETE repos/InteliFi/ad-serverless/branches/main/protection
```

**Importante:** re-aplicar assim que possível — sem proteção, push direto no main é permitido.

### Atualizar template de PR

Editar `.github/PULL_REQUEST_TEMPLATE.md` e fazer PR. O template é aplicado
automaticamente a novos PRs abertos pelo GitHub (não precisa de deploy).

---

## Referências

- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Regras de execução com Claude Code
- [CLAUDE.md](../../CLAUDE.md) §Reguas inegociáveis, §Fluxo de trabalho por issue
- [docs/MATRIZ-PARIDADE.md](../MATRIZ-PARIDADE.md) (item do checklist do PR)
