---
title: "[M10-03] CI/CD de migrations + staging de banco"
labels: ["epic:M10-banco", "tipo:infra", "prioridade:P2"]
milestone: "M10 — Banco de Dados"
---
## Contexto

> ⚠️ **Diretriz do engenheiro-chefe (inegociável):** o banco MySQL é **compartilhado com outros projetos** e **não há CI/CD para atualizar produção** hoje. Mudanças de banco ficam para o FINAL da migração e devem ser feitas **com muito cuidado**. Esta issue existe exatamente para fechar essa lacuna ANTES de qualquer mudança de schema (ver [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) e o aviso no topo de [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md)).

No sistema legado (Java/Spring Boot), as migrations eram aplicadas pelo **Flyway embutido na aplicação** no boot do Tomcat — sem pipeline, sem aprovação, sem staging e com `validate-on-migrate=false` e `repair()` automático antes de cada migrate (ver `FlywayRetryConfig.java`). Esse modelo é inaceitável num banco compartilhado: qualquer deploy da aplicação podia alterar o schema que outros projetos consomem, sem revisão nem rollback planejado. Evidências do legado:

- 30 migrations Flyway em `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/db/migration/` (V1–V30), incluindo uma **colisão de versão real**: existem DOIS arquivos `V10__*` (`V10__add_frequency_cap_to_campaigns.sql` e `V10__create_google_demand_table.sql`) — o novo pipeline deve impedir esse tipo de erro por construção.
- Usuário DDL dedicado já existia: `adserver_ddl`, criado em `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/db/init/bootstrap.sql` com grants `SELECT, INSERT, CREATE, DROP, ALTER, DELETE, REFERENCES, INDEX ON adserver.*`, separado do usuário de runtime `adserver_dml` (`SELECT, INSERT, DELETE, UPDATE`). Em produção o Flyway usava `spring.flyway.user=adserver_ddl` (`application-prod.properties`, linha 48) enquanto a aplicação rodava com `adserver_dml`.
- Conforme o ADR-006 de [ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md): "O MySQL é compartilhado e sem CI/CD de produção. [...] O Epic 'Banco de Dados' (fase final) fará: inventário de consumidores do banco, decisão de destino, plano de migração coordenado e **CI/CD de migrations**. Flyway permanece DESLIGADO nas Lambdas."

Esta issue cria a esteira de migrations com **golang-migrate** (substituto Go-nativo do Flyway), o diretório `migrations/` versionado (já reservado na estrutura do repo — hoje vazio por decisão da Fase 1) e um pipeline GitHub Actions com 4 estágios: validação em banco efêmero → staging (clone do prod) → aprovação manual → produção. Nada desta issue aplica mudança ao banco de produção por si só — ela entrega a ESTEIRA; o conteúdo das migrations virá das decisões do ADR de [M10-02] e será executado em [M10-04].

## Especificação detalhada

### 1. Ferramenta e convenções (golang-migrate)

- Adotar [golang-migrate/migrate](https://github.com/golang-migrate/migrate) v4 (CLI no CI; **não** embutir auto-migrate nas Lambdas — paridade com a diretriz "Flyway DESLIGADO").
- Diretório `migrations/` na raiz do repo (já previsto na estrutura do monorepo, seção 4 de ARQUITETURA-ALVO.md), com pares obrigatórios:
  - `migrations/000001_<slug_em_portugues>.up.sql`
  - `migrations/000001_<slug_em_portugues>.down.sql`
- **Toda migration `up` DEVE ter `down` funcional e testado** (plano de rollback por migration). Migrations irreversíveis (ex.: `DROP COLUMN` com perda de dado) exigem: backup prévio explícito no script ou em step do pipeline + justificativa no PR + aprovação do engenheiro-chefe.
- Tabela de controle: `schema_migrations` (default do golang-migrate). NÃO tocar na tabela `flyway_schema_history` legada — ela permanece como registro histórico (V1–V30 já aplicadas viram o "baseline"; documentar isso em `migrations/README.md`).
- Versões numéricas sequenciais com zero-padding (`000001`, `000002`, ...) — o golang-migrate rejeita versões duplicadas, eliminando por construção a colisão tipo "dois V10" do legado.
- Lint de SQL no CI (ex.: `sqlfluff` dialect mysql) + verificação de que todo `.up.sql` tem `.down.sql` correspondente (script `scripts/check-migrations.sh`).

### 2. Usuários e credenciais (paridade com o legado, sem repetir os erros)

- **Usuário DDL separado** `adserver_ddl` (paridade com `db/init/bootstrap.sql` do legado): usado SOMENTE pelo pipeline de migrations. As Lambdas continuam com usuário DML sem privilégios de DDL.
- Senhas NUNCA no repo (as do legado estão comprometidas e foram/serão rotacionadas em M0-05). Credenciais do pipeline:
  - `/<stage>/adserverless/db/ddl-dsn` em SSM Parameter Store (SecureString), lidas pelo workflow via OIDC (role IAM `migrations-runner-<stage>` com permissão apenas de `ssm:GetParameter` nesse path).
- Rotacionar a senha do `adserver_ddl` (a atual está commitada em `application-prod.properties` do repo legado) como pré-requisito do primeiro run em produção.

### 3. Pipeline GitHub Actions (`.github/workflows/db-migrations.yml`)

Disparo: PRs que tocam `migrations/**` (estágio 1) e `workflow_dispatch`/push em `main` com mudanças em `migrations/**` (estágios 2–4).

**Estágio 1 — Validação em banco efêmero (todo PR):**
1. Sobe container `mysql:8` (service container) com schema vazio + carga do dump de estrutura do schema `adserver` atual (snapshot de estrutura versionado em `migrations/baseline/schema-baseline.sql`, SEM dados de produção).
2. `migrate up` de todas as migrations novas.
3. `migrate down N` e `migrate up` novamente (teste de reversibilidade automatizado de TODA migration nova).
4. Smoke de compatibilidade: executa as queries do hot path documentadas em `docs/legado/04-modelo-dados.md` §6 (SELECT de hotspots/campaigns/creatives, INSERT em `ad_trackers`, SELECT de `tracking_pixels`) contra o schema migrado — garante que as Lambdas em produção continuam funcionando com o schema novo.

**Estágio 2 — Staging (clone/snapshot do prod):**
1. Cria (ou reaproveita, com refresh) uma instância RDS de staging a partir do **snapshot mais recente do prod** (`aws rds restore-db-instance-from-db-snapshot`), em subnet isolada, sem acesso público.
2. Aplica `migrate up` no staging com o usuário DDL.
3. Roda a suíte de validação por consumidor (a mesma do checklist de [M10-01]) + mede duração de cada migration (DDL em tabelas grandes como `ad_trackers` ~14M linhas pode travar — registrar tempo e locks; se necessário, anotar exigência de ferramenta online tipo `gh-ost`/`pt-online-schema-change` no plano de M10-04).
4. Publica relatório como artifact do workflow (duração, locks observados, diff de schema antes/depois via `mysqldump --no-data`).

**Estágio 3 — Aprovação manual:**
- Environment `prod-db` do GitHub com **required reviewers** (engenheiro-chefe obrigatório). O job de prod só roda após aprovação explícita no ambiente, com o relatório de staging anexado.

**Estágio 4 — Produção:**
1. Snapshot manual do RDS prod IMEDIATAMENTE antes (`aws rds create-db-snapshot` com tag da versão da migration) — rollback de último recurso.
2. `migrate up` com usuário `adserver_ddl` via DSN do SSM.
3. Verificação pós-aplicação: versão em `schema_migrations`, smoke das queries do hot path, alarme de erro das Lambdas (M7-04) sem disparos por 15 minutos.
4. Em falha: executar o `down` da migration que falhou (testado nos estágios 1–2); se o `down` falhar, restaurar do snapshot (procedimento documentado no runbook).

### 4. Documentação

- `migrations/README.md`: convenções, fluxo do pipeline, como escrever up/down, política de migrations irreversíveis, baseline V1–V30 do Flyway legado.
- `docs/runbooks/db-migrations.md`: runbook operacional (como aprovar, como acompanhar, como reverter, como restaurar snapshot), seguindo o padrão dos runbooks de M8-04.

## Arquivos a criar/alterar

| Arquivo | Ação |
|---|---|
| `migrations/README.md` | criar — convenções, baseline, política de rollback |
| `migrations/baseline/schema-baseline.sql` | criar — dump de ESTRUTURA do schema `adserver` (sem dados, sem credenciais) |
| `migrations/.gitkeep` | remover quando o primeiro par up/down for criado (se existir) |
| `scripts/check-migrations.sh` | criar — valida pares up/down + numeração sequencial |
| `.github/workflows/db-migrations.yml` | criar — pipeline de 4 estágios descrito acima |
| `serverless.yml` (ou IaC equivalente) | alterar — role IAM `migrations-runner-<stage>` (SSM read no path do DSN DDL) se for gerida aqui |
| `docs/runbooks/db-migrations.md` | criar — runbook operacional |
| `docs/MATRIZ-PARIDADE.md` | alterar — registrar substituição Flyway→golang-migrate (linha do Epic M10) |

## Critérios de aceite

- [ ] `migrations/` versionado com convenção up/down documentada; `scripts/check-migrations.sh` falha o CI se faltar `.down.sql` ou houver versão duplicada (cenário "dois V10" do legado coberto por teste do script).
- [ ] Estágio 1 roda em todo PR que toca `migrations/**`: MySQL efêmero + baseline + `up` → `down` → `up` verde.
- [ ] Smoke de compatibilidade executa as queries do hot path de `docs/legado/04-modelo-dados.md` §6 contra o schema migrado.
- [ ] Estágio 2 aplica em staging restaurado de snapshot do prod e publica relatório (duração por migration + diff de schema) como artifact.
- [ ] Deploy em produção SÓ ocorre após aprovação manual no environment `prod-db` (required reviewer configurado); evidência: screenshot/log do gate no PR desta issue.
- [ ] Estágio 4 cria snapshot do prod antes de aplicar e tem passo de rollback documentado e referenciando os down scripts testados.
- [ ] Pipeline usa usuário DDL dedicado (`adserver_ddl`) com DSN em SSM SecureString; NENHUMA credencial no repo nem no log do workflow; usuário das Lambdas permanece sem privilégio de DDL.
- [ ] Nenhuma migration real aplicada em produção como parte desta issue (a esteira é validada com uma migration de teste reversível aplicada e revertida APENAS no banco efêmero e em staging).
- [ ] `migrations/README.md` e `docs/runbooks/db-migrations.md` escritos em português; ambos citam a diretriz do engenheiro-chefe (banco compartilhado, sem CI/CD de produção até esta issue — fazer depois e com cuidado).
- [ ] `make lint && make test` verdes; PR aberto com `Closes #<n>` e Matriz de Paridade atualizada.

## Dependências

Bloqueada por: [M10-02]

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) — ⚠️ aviso do engenheiro-chefe no topo; §3 histórico Flyway V1–V30; §6 queries do hot path
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — diretriz do engenheiro-chefe; entregável de aceite do M10
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) — ADR-006 (sem mudança de banco na fase 1; CI/CD de migrations no epic final); estrutura do repo (`migrations/`)
- Legado Java: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/db/migration/` (30 arquivos V1–V30, com colisão dupla de V10)
- Legado Java: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/db/init/bootstrap.sql` (usuários `adserver_ddl` e `adserver_dml` com grants distintos)
- Legado Java: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/application-prod.properties` (Flyway com `spring.flyway.user=adserver_ddl`; ⚠️ credenciais expostas — NÃO copiar)
- Legado Java: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/br/com/intv/adserver/config/FlywayRetryConfig.java` (repair+baseline+migrate automáticos no boot — o antipadrão que esta issue elimina)
- [golang-migrate/migrate](https://github.com/golang-migrate/migrate) — documentação da ferramenta

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M10-03] "CI/CD de migrations + staging de banco" conforme
a especificação em docs/issues/M10-03-cicd-migrations.md, respeitando CLAUDE.md
(em especial: banco compartilhado — NENHUMA migration aplicada em produção nesta
issue; apenas a esteira). Criar migrations/ com convenções golang-migrate,
scripts/check-migrations.sh, o workflow .github/workflows/db-migrations.yml com os
4 estágios (banco efêmero → staging de snapshot → aprovação manual → prod com
rollback por down script) e o runbook. Todo código e documentação comentados em
português. make lint && make test verdes. Ao final: abrir PR referenciando a issue
e atualizando docs/MATRIZ-PARIDADE.md.
```
