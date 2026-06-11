---
title: "[M10-01] Inventário de consumidores do MySQL compartilhado"
labels: ["epic:M10-banco", "tipo:docs", "prioridade:P2"]
milestone: "M10 — Banco de Dados"
---
## Contexto

⚠️ **Diretriz do engenheiro-chefe (inegociável, repetida em todos os docs):** o banco MySQL `adserver` é **compartilhado com outros projetos** e **não há CI/CD para atualizar produção** — qualquer mudança de banco fica para o FINAL da migração e deve ser feita **com muito cuidado** ([docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md), [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) aviso no topo, ADR-006).

O Epic M10 é a fase final e só inicia após o descomissionamento das EC2 ([M9-05]). Antes de QUALQUER decisão sobre o destino do banco ([M10-02]), é obrigatório saber **exatamente quem usa o banco hoje**. A frase "compartilhado com outros projetos" nunca foi materializada em uma lista: não sabemos quantos projetos são, quais tabelas cada um lê/escreve, com que frequência, nem o quão críticos eles são. Decidir migração de engine ou schema sem esse inventário = quebrar sistemas de terceiros silenciosamente.

Esta issue produz o inventário completo e verificado: `docs/db/CONSUMIDORES.md`, com matriz **projeto × tabela × operação (R/W) × criticidade**. Ela é 100% **leitura e documentação** — NENHUMA mudança no banco, NENHUMA mudança de parâmetro do RDS sem aprovação explícita (ver regras abaixo).

Dados conhecidos de partida (extraídos dos docs de legado):
- Endpoints RDS ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §1): dev `dev-mysql.ckkqlpl6ei1d.us-east-1.rds.amazonaws.com` (us-east-1), prod `prod-mysql.cglsxksyzbur.sa-east-1.rds.amazonaws.com` (sa-east-1); schema `adserver`; timezone de conexão `GMT-3`.
- Usuários MySQL conhecidos ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2/§3): root (dev), `adserver_dml` e `adserver_ddl` (prod, Flyway usava o DDL), `adserverless_app` (criado em M2-03 para as Lambdas).
- 28+ tabelas; `ad_trackers` com ~14M linhas é a write-heavy ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §1, §2.4).
- Já há suspeita registrada de pelo menos UM consumidor externo: "outros projetos leem `ad_trackers` no mesmo banco" ([docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3, decisão de escrita MySQL mantida) e o risco do header `Location` de `/adtrack` ([M9-01]).

### Regras invioláveis desta issue

1. **Somente leitura.** Nenhum `CREATE/ALTER/DROP`, nenhum INSERT/UPDATE/DELETE, nenhuma migration.
2. **Nenhuma mudança de parameter group ou reboot do RDS.** Se `performance_schema` ou slow query log estiverem desligados, a habilitação é tarefa MANUAL do engenheiro-chefe (ver Fase 1), nunca executada autonomamente.
3. Acesso ao prod apenas com usuário read-only e em horário de baixo tráfego, com consultas leves (as views do `performance_schema` são baratas, mas registrar tudo no relatório).

## Especificação detalhada

### Fase 1 — Telemetria do RDS prod (performance_schema / slow query log)

Objetivo: descobrir **quem se conecta** (usuário × host) e **quais tabelas são tocadas e como** (R/W), a partir do próprio banco — fonte que não mente, ao contrário de grep em código.

1. Verificar disponibilidade da instrumentação (read-only):
```sql
SHOW VARIABLES LIKE 'performance_schema';          -- ON por padrão no MySQL 5.7+/8
SHOW VARIABLES LIKE 'slow_query_log%';
SHOW VARIABLES LIKE 'long_query_time';
```
   - Se `performance_schema=OFF`: habilitar exige parameter group + reboot → **entregar como tarefa manual ao engenheiro-chefe** com janela combinada; documentar a decisão (pode-se seguir só com slow query log + processlist, com cobertura menor).
   - Slow query log pode ser ligado dinamicamente (`slow_query_log=1`, sem reboot), mas é mudança de config de prod → **aprovação explícita do engenheiro-chefe antes**, com `long_query_time` baixo (ex.: 0) por período LIMITADO (24–48h) e rollback do valor anterior registrado.

2. Coletas no `performance_schema` (executar em prod, salvar saída bruta em `docs/db/evidencias/` com data):
```sql
-- Quem conecta: usuários × hosts (origem das aplicações)
SELECT user, host, current_connections, total_connections
  FROM performance_schema.accounts WHERE user IS NOT NULL;

-- Tabelas × volume de leitura/escrita (a matriz nasce daqui)
SELECT object_schema, object_name, count_read, count_write,
       count_fetch, count_insert, count_update, count_delete
  FROM performance_schema.table_io_waits_summary_by_table
 WHERE object_schema = 'adserver' ORDER BY count_write DESC, count_read DESC;

-- Statements normalizados mais frequentes (identifica padrões de acesso por consumidor)
SELECT schema_name, digest_text, count_star, sum_rows_examined, sum_rows_sent
  FROM performance_schema.events_statements_summary_by_digest
 WHERE schema_name = 'adserver' ORDER BY count_star DESC LIMIT 100;

-- Inventário de usuários existentes no servidor (há usuário que não conhecemos?)
SELECT user, host FROM mysql.user ORDER BY user;
```

3. Amostragem de `information_schema.processlist` (ou `SHOW FULL PROCESSLIST`) em horários variados (manhã/tarde/noite/madrugada, dias úteis e fim de semana) durante pelo menos 1 semana — script de coleta pode ser um cron simples documentado, executado de uma máquina com acesso; cada amostra registra `user`, `host`, `db`, `command`, `info`.

4. Cruzar hosts/IPs de origem com inventário de infra da empresa (EC2, ECS, IPs de escritório, VPNs) — cada IP origem deve ser atribuído a um projeto ou marcado como **DESCONHECIDO** (item de follow-up obrigatório na entrevista da Fase 3).

⚠️ Atenção ao viés temporal: como o M10 roda APÓS o cutover ([M9-05]), as EC2 do ad-server legado já estarão desligadas — as conexões observadas serão das Lambdas (`adserverless_app` via RDS Proxy) + **os outros projetos**, o que facilita o isolamento. Consumidores batch (relatórios mensais, jobs de BI) podem não aparecer na janela de coleta → mitigar com a Fase 2 e a Fase 3.

### Fase 2 — Grep nos repositórios da empresa

Procurar em TODOS os repositórios da organização (GitHub `InteliFi` + eventuais repos fora do GitHub indicados pelo engenheiro-chefe) pelos identificadores conhecidos:

```bash
# Via gh CLI (code search na org) — uma busca por termo:
gh search code --owner InteliFi 'cglsxksyzbur'                  # endpoint prod (fragmento único)
gh search code --owner InteliFi 'ckkqlpl6ei1d'                  # endpoint dev (fragmento único)
gh search code --owner InteliFi 'adserver_dml'
gh search code --owner InteliFi 'adserver_ddl'
gh search code --owner InteliFi 'ad_trackers'
gh search code --owner InteliFi 'hotspots_campaigns'
gh search code --owner InteliFi '"adserver"'                    # nome do schema (mais ruidoso, filtrar)
```

- Para repositórios privados não indexados pelo code search: clonar e rodar `git grep -n` pelos mesmos termos (listar no relatório quais repos foram varridos e em qual commit).
- Para cada hit: registrar repo, arquivo, linha, tabela(s) referenciada(s) e operação (R/W) inferida do código. Hits em código morto/branches antigas são registrados com a marcação `possivelmente inativo` e confirmados na Fase 3.
- Incluir os repositórios legados arquivados em [M9-05] (`ad-server`, `ad-commons`) apenas como linha histórica (consumidor desativado) — referência local: `c:/Users/Fabio/Documents/Dev/ad-server` e `c:/Users/Fabio/Documents/Dev/ad-commons`.

### Fase 3 — Entrevista estruturada com o engenheiro-chefe

Roteiro mínimo (anexar respostas como ata em `docs/db/evidencias/entrevista-engenheiro-chefe.md`, com data e revisão dele):

1. Quais projetos/sistemas você sabe que usam este banco hoje? Quem é o dono/contato técnico de cada um?
2. Para cada projeto: quais tabelas lê? Em quais escreve? Com que frequência (contínuo, diário, mensal)?
3. Existe consumo via ferramentas de BI/planilhas/dashboards (Metabase, Grafana, scripts ad-hoc) que não está em repositório?
4. Existe algum consumidor que use o `id` de `ad_trackers` ou o header `Location` de `/adtrack` (pendência de [M9-01])?
5. Algum desses sistemas tem janela de manutenção própria, SLA ou época do ano em que NÃO pode haver mudança (ex.: fechamento de mês)?
6. Há usuários MySQL/IPs da Fase 1 que você não reconhece? (resolver TODOS os "DESCONHECIDO")
7. Quem precisa aprovar/ser comunicado numa eventual migração de banco (lista de stakeholders para [M10-04])?

### Fase 4 — Consolidação: docs/db/CONSUMIDORES.md

Estrutura obrigatória do entregável:

```markdown
# Consumidores do MySQL compartilhado (schema adserver)
> Gerado pela issue M10-01 em <data>. Fontes: performance_schema/slow log (Fase 1),
> grep nos repositórios (Fase 2), entrevista com engenheiro-chefe (Fase 3).

## 1. Matriz consumidor × tabela × operação × criticidade
| Projeto/Sistema | Dono/Contato | Tabela | Operação (R/W) | Frequência | Criticidade | Evidência |
|---|---|---|---|---|---|---|
| ad-serverless (este projeto) | <contato> | ad_trackers | W (INSERT) | contínuo | ALTA | tracker-writer M3-04 |
| ad-serverless (este projeto) | <contato> | hotspots, campaigns, creatives, tracking_pixels, hotspots_campaigns, <override VAST> | R | contínuo | ALTA | M3-01 |
| <projeto X> | ... | ... | ... | ... | ALTA/MÉDIA/BAIXA | link p/ evidência |

## 2. Usuários MySQL × projeto
## 3. Hosts/IPs de origem × projeto
## 4. Tabelas SEM consumidor identificado (candidatas a órfãs — NÃO remover, só registrar)
## 5. Consumidores batch/sazonais e suas janelas
## 6. Pendências e riscos abertos
```

- Criticidade: **ALTA** (receita/produção direta), **MÉDIA** (operação interna), **BAIXA** (relatório eventual) — critério registrado no próprio doc.
- Cada linha da matriz TEM que ter evidência rastreável (saída de query, link de código ou ata da entrevista). Linha sem evidência não entra.
- As tabelas DynamoDB `AdTrackers` e `PostbackLogs` ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §4) entram como apêndice (consumidores conhecidos: ad-serverless), pois a decisão de [M10-02] também as considera.

## Arquivos a criar/alterar

- `docs/db/CONSUMIDORES.md` (entregável principal — matriz + seções acima)
- `docs/db/evidencias/` (saídas brutas das queries do performance_schema, amostras de processlist, resultado do grep por repo, ata da entrevista)
- `docs/db/README.md` (índice curto do diretório e aviso do banco compartilhado)
- `docs/PLANO-MIGRACAO.md` (marcar M10-01 entregue na tabela de epics, se aplicável)

## Critérios de aceite

- [ ] `docs/db/CONSUMIDORES.md` existe com a matriz **projeto × tabela × operação (R/W) × criticidade** preenchida e TODAS as linhas com evidência rastreável
- [ ] Fase 1 executada: saídas de `accounts`, `table_io_waits_summary_by_table`, `events_statements_summary_by_digest` e `mysql.user` do prod salvas em `docs/db/evidencias/` (ou justificativa documentada caso `performance_schema=OFF` + decisão do engenheiro-chefe)
- [ ] Amostragem de processlist cobrindo ≥1 semana, com horários variados, e todos os hosts de origem atribuídos a um projeto (zero "DESCONHECIDO" sem follow-up registrado)
- [ ] Fase 2 executada: lista dos repositórios varridos (org InteliFi + indicados) com commit de referência e hits por endpoint/usuário/tabela
- [ ] Ata da entrevista com o engenheiro-chefe revisada/aprovada por ele, incluindo a lista de stakeholders para [M10-04] e a resposta sobre o `id` de `ad_trackers`/header `Location` ([M9-01])
- [ ] Seção de tabelas sem consumidor identificado preenchida (sem nenhuma ação de remoção — apenas registro)
- [ ] NENHUMA mudança executada no banco/RDS (sem DDL, sem DML, sem parameter group, sem reboot); eventual habilitação de slow log feita SOMENTE com aprovação registrada e revertida ao final
- [ ] Documento cita explicitamente a diretriz do engenheiro-chefe (banco compartilhado, sem CI/CD de produção — mudanças no final e com cuidado)

## Dependências

Bloqueada por: [M9-05] (descomissionamento das EC2 — o legado não pode mais estar conectado durante a coleta)

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) — aviso do engenheiro-chefe no topo, §1 (endpoints/schema), §2 (28+ tabelas), §4 (DynamoDB)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (topologia), §2 (usuários `adserver_dml`/`adserver_ddl`), §3 (Flyway com usuário DDL)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-002, ADR-006, decisão "escrita MySQL mantida na fase 1"
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — diretriz do engenheiro-chefe, linha M10 da tabela de epics
- Issues relacionadas: [M9-01] (verificação do header `Location`), [M2-03] (RDS Proxy/usuário `adserverless_app`), [M10-02] (consome este inventário)
- Java (referência histórica): `c:/Users/Fabio/Documents/Dev/ad-server` (`application*.properties` — endpoints), `c:/Users/Fabio/Documents/Dev/ad-commons` (entities JPA = tabelas)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M10-01] Inventário de consumidores do MySQL
compartilhado no repo InteliFi/ad-serverless seguindo
docs/issues/M10-01-inventario-consumidores.md e CLAUDE.md. SOMENTE LEITURA no
banco (diretriz do engenheiro-chefe: banco compartilhado com outros projetos e
sem CI/CD de produção — nenhuma mudança). Executar as 4 fases: telemetria do
performance_schema/slow log do RDS prod (mudanças de config só como tarefa
manual aprovada), grep nos repositórios da org pelos endpoints/usuários/tabelas
conhecidos, roteiro de entrevista estruturada com o engenheiro-chefe e
consolidação em docs/db/CONSUMIDORES.md (matriz projeto × tabela × R/W ×
criticidade com evidências). Documentação em português, scripts de coleta
comentados em português, abrir PR ao final listando pendências que dependem
de humanos (entrevista, aprovações).
```
