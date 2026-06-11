---
title: "[M0-06] Governança: templates de PR/issue, branch protection, CODEOWNERS"
labels: ["epic:M0-fundacao", "tipo:infra", "prioridade:P1"]
milestone: "M0 — Fundação"
---
## Contexto

Todo o desenvolvimento será feito por IA (Claude Code) com revisão humana, no fluxo "1 issue = 1 branch = 1 PR" ([docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Regras de execução e [CLAUDE.md](../../CLAUDE.md) §Fluxo de trabalho). Para que a revisão humana seja eficaz e nenhuma regra inegociável passe batida (paridade, matriz, golden tests, português, referência ao Java de origem), o repositório precisa de guard-rails: template de PR com checklist, templates de issue, CODEOWNERS e branch protection no `main`.

## Especificação detalhada

### 1. `.github/PULL_REQUEST_TEMPLATE.md`
Template em português com as seções:
```markdown
## O que este PR faz
<!-- resumo + "Closes #N" -->

## Origem no legado
<!-- arquivos Java portados, ex.: AdComponentImpl.java#getHotSpotAdScript -->

## Como testar

## Checklist (obrigatório)
- [ ] Código e comentários 100% em português (CODE_DOCS_POLICY.md)
- [ ] Toda lógica portada referencia o arquivo Java de origem em comentário
- [ ] docs/MATRIZ-PARIDADE.md atualizada (linhas desta issue)
- [ ] Golden tests incluídos/atualizados quando o PR altera saída (template/VAST/JS)
- [ ] make lint && make test verdes localmente
- [ ] Nenhum segredo/credencial/endpoint com senha commitado
- [ ] Sem mudança de schema MySQL (regra inegociável — Epic M10)
- [ ] Melhoria NÃO misturada com portagem (issues `melhoria` separadas)
```

### 2. `.github/ISSUE_TEMPLATE/`
- `config.yml` com `blank_issues_enabled: false` e link para `docs/issues/README.md` ("issues planejadas nascem dos arquivos versionados, não manualmente").
- `bug.yml` (form): campos rota/Lambda afetada, stage (dev/prod), comportamento esperado (com link para `docs/legado/*`), comportamento observado, request de reprodução (curl).
- `melhoria.yml` (form): descrição, justificativa, confirmação explícita "não é portagem de paridade" (checkbox), label automática `melhoria`.

### 3. `.github/CODEOWNERS`
```
# Revisão humana obrigatória em tudo (desenvolvimento é feito por IA)
*               @fabio-intelifi
# Mudanças sensíveis exigem também o engenheiro-chefe
/serverless.yml @fabio-intelifi @engenheiro-chefe
/migrations/    @fabio-intelifi @engenheiro-chefe
/docs/legado/   @fabio-intelifi @engenheiro-chefe
```
(Substituir pelos usernames reais do GitHub do time antes do merge — confirmar com o usuário.)

### 4. Branch protection do `main`
Aplicar via `gh api` (documentar no runbook de governança):
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
Regras: PR obrigatório (sem push direto), 1 aprovação + code owners, status checks `lint`/`test`/`build` do ci.yml (M0-03) verdes e atualizados com o main (`strict`), sem force-push/delete.

### 5. `docs/runbooks/governanca.md`
Documentar: regras ativas, como ajustá-las, convenção de branches (`feat/issue-N-slug`, `infra/issue-N-slug`) e de commits (convencionais em português).

## Arquivos a criar/alterar

- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/ISSUE_TEMPLATE/config.yml`
- `.github/ISSUE_TEMPLATE/bug.yml`
- `.github/ISSUE_TEMPLATE/melhoria.yml`
- `.github/CODEOWNERS`
- `docs/runbooks/governanca.md`
- Branch protection no GitHub (via `gh api`, documentada)

## Critérios de aceite

- [ ] Abrir um PR de teste exibe o template com o checklist completo em português
- [ ] Criar issue manual só oferece os forms `bug`/`melhoria` (blank desabilitado)
- [ ] CODEOWNERS validado pelo GitHub (aba Settings → sem erro de sintaxe; usernames reais)
- [ ] push direto no `main` é rejeitado
- [ ] PR sem os checks `lint`/`test`/`build` verdes não pode ser mergeado
- [ ] PR sem aprovação de code owner não pode ser mergeado
- [ ] Runbook de governança escrito em português

## Dependências

Bloqueada por: #M0-01 (o ci.yml da M0-03 precisa existir antes de exigir os checks — se M0-03 ainda não mergeou, aplicar a proteção sem `required_status_checks` e completar depois)

## Referências

- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Regras de execução com Claude Code
- [CLAUDE.md](../../CLAUDE.md) §Regras inegociáveis, §Fluxo de trabalho por issue
- [docs/MATRIZ-PARIDADE.md](../MATRIZ-PARIDADE.md) (item do checklist do PR)
- [docs/issues/README.md](README.md) (labels e fluxo de issues versionadas)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M0-06] Governança do repositório no repo
InteliFi/ad-serverless: PULL_REQUEST_TEMPLATE.md com o checklist da
especificação, ISSUE_TEMPLATE (bug.yml, melhoria.yml, config.yml com
blank desabilitado), CODEOWNERS (confirmar usernames reais com o usuário
antes), branch protection do main via gh api (PR obrigatório, checks
lint/test/build, code owners) e docs/runbooks/governanca.md. Tudo em
português (CODE_DOCS_POLICY.md). Ao final: abrir PR referenciando a issue.
```
