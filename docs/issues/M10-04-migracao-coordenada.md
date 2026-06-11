---
title: "[M10-04] Migração coordenada sem downtime"
labels: ["epic:M10-banco", "tipo:infra", "prioridade:P2"]
milestone: "M10 — Banco de Dados"
---
## Contexto

> ⚠️ **Diretriz do engenheiro-chefe (inegociável):** o banco MySQL é **compartilhado com outros projetos** e **não havia CI/CD para atualizar produção** antes de [M10-03]. Mudanças de banco ficam para o FINAL da migração e devem ser feitas **com muito cuidado** (ver [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) e o aviso no topo de [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md)).

Esta é a ÚLTIMA issue do plano de migração (60/60). Neste ponto:

- O cutover M9 está completo: 100% do tráfego nas Lambdas Go, EC2 desligadas, repos Java arquivados ([M9-05]).
- [M10-01] produziu o **inventário de consumidores** do MySQL compartilhado: quem lê/escreve cada tabela (outros projetos leem `ad_trackers` — ver decisão na seção 3 de [ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md): "outros projetos leem ad_trackers no mesmo banco; a dupla escrita continua até a fase de banco de dados decidir o destino final com coordenação entre projetos").
- [M10-02] produziu o **ADR aprovado pelo engenheiro-chefe** com o destino do banco (ex.: permanecer no MySQL atual com schema evoluído, Aurora MySQL, separação do schema `adserver` em instância própria, ou DynamoDB-only para tracking) — esta issue NÃO rediscute a decisão, apenas a executa.
- [M10-03] entregou a esteira: golang-migrate + pipeline GitHub Actions com banco efêmero, staging restaurado de snapshot do prod, aprovação manual e rollback por migration.

O que falta — e é o escopo desta issue — é o **plano de execução coordenado** e a **execução em si**, com critério central: **zero downtime percebido pelos consumidores** (Lambdas do ad-serverless E os outros projetos que usam o mesmo banco). O entregável de aceite do epic, conforme o PLANO-MIGRACAO.md: "plano aprovado pelo engenheiro-chefe; execução sem downtime".

Dados de referência do banco (de `docs/legado/04-modelo-dados.md`): schema `adserver` com 28+ tabelas; `ad_trackers` com ~14M linhas e write-heavy; `hotspots` ~928 registros; `campaigns` ~90; DynamoDB `AdTrackers`/`PostbackLogs` em sa-east-1; endpoint prod `prod-mysql.cglsxksyzbur.sa-east-1.rds.amazonaws.com`, timezone de conexão `GMT-3`.

## Especificação detalhada

O escopo concreto desta issue depende do ADR de [M10-02]. A especificação abaixo define o **framework de execução obrigatório**, com ramificações explícitas para os dois cenários possíveis (mesma engine × mudança de engine/instância). Tudo que for plano/documentação vai para `docs/plano-migracao-banco/`.

### 1. Plano de janela e comunicação (`docs/plano-migracao-banco/01-janela-e-comunicacao.md`)

1. **Mapa de stakeholders** a partir do inventário de [M10-01]: para cada consumidor do banco (projeto, time, contato, tabelas usadas, padrão de acesso leitura/escrita, tolerância a latência), definir o impacto da mudança e o canal de comunicação.
2. **Cronograma com 3 comunicações mínimas** por time consumidor: T-2 semanas (anúncio + pedido de validação do checklist), T-2 dias (confirmação de janela + freeze de mudanças de schema por parte deles), T-0 (início/fim da janela em tempo real, com canal de guerra — Slack/Meet).
3. **Janela escolhida** com base nas métricas reais de tráfego (menor RPS observado nos dashboards de M7-03), mesmo que a meta seja zero downtime — a janela limita o raio de dano de um rollback.
4. **Freeze**: durante a janela, nenhum deploy de Lambdas nem migrations de outros projetos (acordado com os times na comunicação T-2 semanas).

### 2. Estratégia de dados — dual-write/backfill SE houver mudança de engine/instância (`docs/plano-migracao-banco/02-estrategia-dados.md`)

**Cenário A — destino é o próprio MySQL atual (mudanças de schema in-place):**
- Sem dual-write. Toda mudança via migrations expand/contract pelo pipeline de [M10-03]:
  - *Expand*: adicionar coluna/tabela/índice novos SEM remover os antigos (compatível com consumidores atuais).
  - *Migrate*: atualizar Lambdas e (coordenadamente) outros projetos para o schema novo.
  - *Contract*: remover o legado SOMENTE após todos os consumidores confirmarem (checklist da seção 4).
- Para DDL em tabelas grandes (`ad_trackers` ~14M linhas): usar a medição de duração/locks coletada pelo estágio de staging de [M10-03]; se um `ALTER TABLE` bloquear escrita acima do tolerado, usar `gh-ost` ou `pt-online-schema-change` (decidir com base no relatório de staging e documentar).

**Cenário B — mudança de engine/instância (ex.: Aurora, instância separada):**
1. **Dual-write**: as Lambdas de escrita (`tracker-writer` — INSERT em `ad_trackers`; e qualquer outra escrita identificada em M10-01) passam a escrever no banco antigo E no novo, com o antigo como fonte de verdade. Implementar como camada no `internal/repository/mysql` com flag de configuração via SSM (`/prod/adserverless/db/dual-write-enabled`), comentada em português, com métrica EMF de divergência (falha em um dos dois destinos → alarme, nunca perda silenciosa).
2. **Backfill**: cópia dos dados históricos para o destino (AWS DMS com full load + CDC, ou snapshot/restore se a engine for compatível), com janela de catch-up do CDC até lag ~0.
3. **Validação de consistência**: job de reconciliação (mesmo padrão da reconciliação diária de M9-04) comparando contagens por `campaign_id` + `event_date` entre origem e destino, além de checksums por chunk (`pt-table-checksum` ou query de hash por faixa de id).
4. **Cutover de leitura**: consumidores migram a leitura para o novo endpoint um a um (RDS Proxy de M2-03 facilita: trocar o target do proxy é transparente para as Lambdas; consumidores externos trocam DSN no ritmo deles dentro da janela combinada).
5. **Inversão da fonte de verdade** e, após período de observação acordado, desligamento do dual-write e descomissionamento do banco antigo (com snapshot final retido).

### 3. Ensaio completo em staging (obrigatório nos dois cenários)

- Executar o plano INTEIRO (migrations + dual-write/backfill se cenário B + validações + rollback) no ambiente de staging de [M10-03] (restaurado de snapshot do prod), de ponta a ponta, medindo o tempo de cada etapa.
- O ensaio gera o **cronograma minuto-a-minuto** da janela real (`docs/plano-migracao-banco/03-cronograma-janela.md`), com responsável por etapa, comando exato, duração esperada e critério go/no-go.
- **Rollback testado ANTES da janela**: para cada etapa do cronograma, o procedimento de reversão correspondente (down scripts do golang-migrate, religar leitura no banco antigo, restaurar snapshot) é executado no ensaio e cronometrado. Nenhuma etapa sem rollback ensaiado entra no cronograma.

### 4. Validação por consumidor (checklist de [M10-01])

- Para CADA consumidor do inventário de [M10-01], um checklist verificável em `docs/plano-migracao-banco/04-checklist-consumidores.md`:
  - [ ] queries representativas do consumidor executadas com sucesso contra o staging migrado (no ensaio) e contra o prod (pós-janela);
  - [ ] latência p95 das queries dentro da banda histórica (±20%);
  - [ ] contagens de linhas/agregações batem com o esperado (para `ad_trackers`: reconciliação MySQL×DynamoDB do M9-04 continua verde);
  - [ ] assinatura do responsável do time consumidor (nome + data) confirmando operação normal.
- As Lambdas do ad-serverless são consumidores também: smoke das queries do hot path (`docs/legado/04-modelo-dados.md` §6) + dashboards M7-03 sem anomalia + alarmes M7-04 silenciosos por 24h.

### 5. Execução e encerramento

- Execução na janela seguindo o cronograma minuto-a-minuto, pelo pipeline de [M10-03] (estágios staging → aprovação manual → prod), NUNCA por comandos manuais ad-hoc.
- Snapshot do prod imediatamente antes (passo já obrigatório do estágio 4 de [M10-03]).
- Relatório pós-execução (`docs/plano-migracao-banco/05-relatorio-execucao.md`): o que rodou, durações reais vs. ensaiadas, incidentes, confirmações dos consumidores, e — se cenário B — data planejada do desligamento do dual-write.

## Arquivos a criar/alterar

| Arquivo | Ação |
|---|---|
| `docs/plano-migracao-banco/01-janela-e-comunicacao.md` | criar — stakeholders, cronograma de comunicação, janela, freeze |
| `docs/plano-migracao-banco/02-estrategia-dados.md` | criar — expand/contract (cenário A) ou dual-write/backfill/CDC (cenário B), conforme ADR de M10-02 |
| `docs/plano-migracao-banco/03-cronograma-janela.md` | criar — minuto-a-minuto gerado pelo ensaio em staging, com go/no-go e rollback por etapa |
| `docs/plano-migracao-banco/04-checklist-consumidores.md` | criar — checklist por consumidor do inventário de M10-01, com assinaturas |
| `docs/plano-migracao-banco/05-relatorio-execucao.md` | criar — relatório pós-janela |
| `migrations/NNNNNN_*.up.sql` / `.down.sql` | criar — as migrations do ADR de M10-02, via pipeline de M10-03 |
| `internal/repository/mysql/dualwrite.go` (+ `_test.go`) | criar SOMENTE se cenário B — camada de dual-write com flag SSM e métricas, comentada em português |
| `cmd/trackerwriter/main.go` | alterar SOMENTE se cenário B — ligar dual-write via config |
| `docs/runbooks/db-migrations.md` | alterar — anexar lições aprendidas da execução |
| `docs/MATRIZ-PARIDADE.md` | alterar — fechar a linha do Epic M10 |

## Critérios de aceite

- [ ] Plano completo (`docs/plano-migracao-banco/01..04`) escrito em português e **aprovado por escrito pelo engenheiro-chefe** antes da janela (entregável de aceite do M10 no PLANO-MIGRACAO.md).
- [ ] Todos os times consumidores do inventário de [M10-01] comunicados nos marcos T-2 semanas, T-2 dias e T-0, com registro (link/ata) no documento 01.
- [ ] Ensaio completo executado em staging restaurado de snapshot do prod, incluindo rollback de CADA etapa, com durações registradas no documento 03 — nenhuma etapa entra no cronograma sem rollback ensaiado.
- [ ] Se o ADR de [M10-02] implicar mudança de engine/instância: dual-write implementado com flag SSM, métrica de divergência e alarme; backfill com reconciliação verde (contagens por `campaign_id`+`event_date` e checksum por chunk) antes do cutover de leitura.
- [ ] Execução em produção feita exclusivamente pelo pipeline de [M10-03] (staging → aprovação manual no environment `prod-db` → prod com snapshot prévio); nenhum comando manual ad-hoc no banco.
- [ ] **Zero downtime percebido pelos consumidores**: nenhuma janela de indisponibilidade de leitura/escrita reportada por nenhum consumidor; dashboards M7-03 sem queda de tráfego/sucesso; alarmes M7-04 silenciosos durante e nas 24h após a janela.
- [ ] Checklist do documento 04 100% assinado pelos responsáveis de cada projeto consumidor após a janela.
- [ ] Reconciliação diária MySQL×DynamoDB (M9-04) verde no dia da janela e nos 7 dias seguintes.
- [ ] Relatório pós-execução (documento 05) publicado e revisado pelo engenheiro-chefe.
- [ ] Código novo (se houver, cenário B) 100% comentado em português, com testes unitários verdes (`make lint && make test`); PR aberto com `Closes #<n>` e Matriz de Paridade atualizada.

## Dependências

Bloqueada por: [M10-03]

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) — ⚠️ aviso do engenheiro-chefe no topo; §1 visão geral (28+ tabelas, endpoints, GMT-3); §2.4 `ad_trackers` ~14M linhas write-heavy; §6 queries do hot path
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — diretriz do engenheiro-chefe; entregável M10: "plano aprovado pelo engenheiro-chefe; execução sem downtime"
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) — ADR-002 (RDS Proxy), ADR-006 (banco intocado até o epic final, coordenação entre projetos), decisão "escrita MySQL mantida na fase 1"
- Issue [M10-01] — inventário de consumidores do MySQL compartilhado (base do checklist)
- Issue [M10-02] — ADR com a decisão de destino do banco (define cenário A ou B)
- Issue [M10-03] — pipeline de migrations + staging usados nesta execução
- Issue [M9-04] — reconciliação diária de eventos (reaproveitada como validação de consistência)
- Legado Java: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/application-prod.properties` (endpoint RDS prod e timezone de conexão GMT-3 — ⚠️ credenciais expostas, NÃO copiar)
- Legado Java: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/db/init/bootstrap.sql` (separação DDL/DML de usuários a preservar no destino)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M10-04] "Migração coordenada sem downtime" conforme a
especificação em docs/issues/M10-04-migracao-coordenada.md, respeitando CLAUDE.md
e o ADR aprovado em M10-02 (não rediscutir a decisão). Produzir o plano completo
em docs/plano-migracao-banco/ (janela e comunicação com os times consumidores,
estratégia de dados com dual-write/backfill se houver mudança de engine,
cronograma minuto-a-minuto derivado do ensaio em staging, checklist por consumidor
do inventário de M10-01 e rollback testado antes da janela). Toda execução em
produção via pipeline de M10-03 com aprovação manual. Código e documentação 100%
em português; make lint && make test verdes. Ao final: abrir PR referenciando a
issue e atualizando docs/MATRIZ-PARIDADE.md.
```
