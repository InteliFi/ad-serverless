# Migrations — ad-serverless

> ⚠️ **Esta pasta fica SEM NENHUM SQL até o Epic M10.** Regra inegociável
> nº 2 do [CLAUDE.md](../CLAUDE.md) e ADR-006: o MySQL `adserver` é
> compartilhado com outros projetos em produção — nenhum
> `CREATE/ALTER/DROP`, nenhuma migration executada, Flyway desligado.
> Qualquer mudança de schema exige o processo do M10 (inventário de
> consumidores M10-01 → ADR de destino M10-02 → CI/CD de migrations M10-03 →
> migração coordenada M10-04) com aprovação humana.

Esta estrutura existe (decisão do engenheiro-chefe, 2026-06-12) para o
projeto **já nascer com a ferramenta de migrations definida** — quando o M10
chegar, só entra conteúdo, não há setup novo.

## Ferramenta: golang-migrate

[golang-migrate](https://github.com/golang-migrate/migrate) — substituto do
Flyway do legado (ver tabela de equivalências em
[docs/legado/05-config-infra-deploy.md](../docs/legado/05-config-infra-deploy.md) §4).

Instalação do CLI (somente quem for trabalhar no M10):

```bash
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## Convenções (definidas agora, usadas no M10)

1. **Nome dos arquivos:** `NNNNNN_descricao_curta.up.sql` e
   `NNNNNN_descricao_curta.down.sql` (sequencial com 6 dígitos; toda
   migration TEM down — rollback obrigatório).
   Exemplo: `000001_baseline_schema_adserver.up.sql`.
2. **Baseline primeiro:** a migration `000001` do M10 será o snapshot do
   schema existente marcado como aplicado (`migrate ... force 1`), espelhando
   o `baseline-on-migrate=true` do Flyway legado — NUNCA recriar tabelas que
   já existem.
3. **Usuário DDL separado** (paridade com o legado: `adserver_ddl`): as
   migrations rodam com o usuário DDL via DSN próprio
   (`/ad-serverless/prod/mysql-ddl-dsn`, criado SÓ no M10 — ver
   [docs/runbooks/ssm-parametros.md](../docs/runbooks/ssm-parametros.md) §3);
   as Lambdas continuam com o usuário DML, sem permissão de DDL.
4. **Execução nunca dentro de Lambda de request:** migrations rodam no
   pipeline (estágio dedicado do CodePipeline, M10-03), jamais no cold start.
5. **Comando de referência** (NÃO executar antes do M10):

```bash
migrate -path ./migrations -database 'mysql://<DSN do usuário DDL>' up
```
