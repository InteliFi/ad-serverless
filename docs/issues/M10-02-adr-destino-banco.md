---
title: "[M10-02] ADR: decisão de destino do banco"
labels: ["epic:M10-banco", "tipo:decisao", "prioridade:P2"]
milestone: "M10 — Banco de Dados"
---
## Contexto

⚠️ **Diretriz do engenheiro-chefe:** o MySQL `adserver` é **compartilhado com outros projetos** e **não há CI/CD para atualizar produção** — mudanças de banco ficam para o FINAL e devem ser feitas **com muito cuidado** ([docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md), [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) aviso no topo, ADR-006).

Com o cutover concluído ([M9-05]) e o inventário de consumidores entregue ([M10-01]), chegou o momento previsto no ADR-006: "decisão de destino (Aurora/derivados/DynamoDB-only)". Esta issue **NÃO executa nada no banco** — ela produz o **ADR-008** (`docs/arquitetura/ADR-008-destino-banco.md`) comparando as opções e termina com uma recomendação que **EXIGE aprovação explícita e registrada do engenheiro-chefe antes de qualquer execução** ([M10-03] e [M10-04] ficam bloqueadas até essa assinatura).

Estado atual a considerar (fase 1 já entregue):
- Lambdas Go acessam o RDS MySQL existente via **RDS Proxy** ([M2-03], ADR-002), usuário `adserverless_app` (SELECT geral + INSERT só em `ad_trackers`), `MaxConnectionsPercent=25`.
- **Dupla escrita** de tracking já em produção: MySQL `ad_trackers` (~14M linhas, write-heavy) + DynamoDB `AdTrackers`; postbacks logados em DynamoDB `PostbackLogs` ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2.4, §4; [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3).
- Leitura do hot path: `hotspots` (~928), `campaigns` (~90), `creatives`, `tracking_pixels`, `hotspots_campaigns`, tabela de override VAST — dados de **cadastro**, pequenos, cacheados 5min ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §2, §6).
- Relatórios (`report-handler`) fazem agregação SQL `GROUP BY` em `ad_trackers` ([M6-01]).
- Prod em sa-east-1; em dev há o legado cross-region (RDS us-east-1 × DynamoDB sa-east-1) ⚠️ ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1).
- Flyway está DESLIGADO desde a fase 1; o schema está congelado nas migrations V1–V30 do legado ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §3).

## Especificação detalhada

### 1. Formato do ADR (docs/arquitetura/ADR-008-destino-banco.md)

Seguir o estilo dos mini-ADRs existentes (ADR-001…007 em [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §5), porém como arquivo próprio e mais extenso, com as seções: **Status** (`Proposto` → `Aprovado pelo engenheiro-chefe em <data>` — só muda com a assinatura), **Contexto**, **Opções consideradas**, **Análise por consumidor**, **Decisão**, **Consequências**, **Plano de reversão**, **Aprovação**.

### 2. Opções a comparar (obrigatórias, nesta ordem)

Para CADA opção: descrição técnica, prós, contras, estimativa de custo mensal (instância/proxy/IO/transferência — comparar com a estimativa atual de ~US$ 25/mês do RDS Proxy + custo da instância RDS atual), risco de migração (baixo/médio/alto com justificativa), esforço estimado e impacto nos consumidores do inventário.

**(a) Manter RDS MySQL como está (baseline / não fazer nada)**
- Prós: risco zero de migração; consumidores intocados; RDS Proxy já protege conexões.
- Contras: permanece sem CI/CD de migrations (lacuna apontada pelo engenheiro-chefe — mitigável com [M10-03] sobre o banco atual); instância única, escalabilidade vertical; `ad_trackers` cresce ~indefinidamente no MySQL.
- Esta opção é o BASELINE: qualquer outra precisa vencê-la em benefício líquido por consumidor.

**(b) Aurora MySQL Serverless v2 (compatível, menor risco de migração de engine)**
- Wire-compatible com MySQL → consumidores existentes em tese só trocam endpoint (validar versão do engine atual, coletada na Fase 0 de [M2-03], contra a compatibilidade Aurora).
- Caminhos de migração a avaliar: snapshot-restore para cluster Aurora (downtime na troca de endpoint) vs. Aurora Read Replica do RDS + promoção (near-zero downtime) vs. AWS DMS.
- Prós: auto-scaling de capacidade (ACUs), failover multi-AZ, até 15 réplicas de leitura (relatórios), storage que cresce sozinho; RDS Proxy suporta Aurora.
- Contras: custo mínimo de ACU contínuo (estimar com métricas reais de CPU/conexões pós-cutover); migração exige janela coordenada com TODOS os consumidores (troca de endpoint/DNS).

**(c) DynamoDB-only para tracking + MySQL para cadastro (split por workload)**
- `ad_trackers` (write-heavy, ~14M linhas) deixa de ser escrito no MySQL; DynamoDB `AdTrackers` vira a fonte única de tracking; MySQL mantém apenas cadastro (campaigns/creatives/hotspots/pixels/override).
- ⚠️ PRÉ-CONDIÇÃO DURA: o inventário [M10-01] precisa provar que NENHUM outro projeto lê `ad_trackers` no MySQL — a decisão de manter a dupla escrita na fase 1 existiu exatamente porque "outros projetos leem `ad_trackers` no mesmo banco" ([docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3). Se houver leitor: ou ele migra junto (custo no plano), ou a opção cai.
- Impacto interno: `report-handler` ([M6-01]/[M6-02]) hoje agrega via SQL `GROUP BY` — reescrever agregação sobre DynamoDB (Query por `campaign_id` + agregação em código, ou export S3+Athena) e o `tracker-writer` perde o `LastInsertId()` do MySQL que compõe a SK `created_at_id` = `<ISO8601>#<rds_id>` ([docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §4.1) — definir novo gerador de unicidade (ULID/UUID) e o impacto de QUEBRA DE FORMATO da SK.
- Prós: elimina o maior volume de escrita do MySQL; DynamoDB já está em produção; custo on-demand já conhecido.
- Contras: dois modelos de dados; relatórios mais complexos; mudança de formato de chave exige decisão explícita (não é paridade).

**(d) PostgreSQL (RDS ou Aurora PostgreSQL)**
- Avaliar por completude (foi pedido na comparação), mas com ceticismo: exige conversão de schema/tipos/SQL e migração via DMS, e TODOS os consumidores do inventário teriam que portar drivers e queries simultaneamente.
- Prós: features (particionamento declarativo para `ad_trackers`, índices melhores); ecossistema.
- Contras: risco MÁXIMO num banco compartilhado — esforço multiplicado pelo número de consumidores; sem ganho funcional identificado nos docs que justifique a troca de dialeto.

### 3. Análise POR CONSUMIDOR (núcleo da issue)

Para cada linha da matriz de `docs/db/CONSUMIDORES.md` ([M10-01]), preencher a tabela:

| Consumidor | Tabelas/operações | Impacto em (a) | Impacto em (b) | Impacto em (c) | Impacto em (d) | Esforço de adaptação | Veto? |
|---|---|---|---|---|---|---|---|

- "Veto?" = o consumidor inviabiliza a opção (ex.: sistema sem dono ativo que não pode ser alterado → veta (c) e (d) se tocar nas tabelas dele).
- Consumidores de criticidade ALTA têm peso de veto; BAIXA pode aceitar adaptação planejada.
- A recomendação final DEVE decorrer desta tabela (rastreabilidade: citar as linhas que decidem).

### 4. Critérios de decisão (pesos explícitos no ADR)

1. **Risco para os outros projetos** (peso máximo — diretriz do engenheiro-chefe);
2. Existência de caminho de rollback testável;
3. Custo mensal total (comparar com §6 da arquitetura: meta de manter o total US$ 200–350/mês);
4. Esforço de execução ([M10-04]) e de manutenção contínua;
5. Habilitação do CI/CD de migrations ([M10-03]) — todas as opções devem descrever como ficam as migrations depois.

### 5. Aprovação (gate humano obrigatório)

- Apresentar o ADR ao engenheiro-chefe (reunião ou revisão assíncrona) e registrar na seção **Aprovação**: nome, data, decisão (aprovada/aprovada com ressalvas/rejeitada) e ressalvas literais.
- O PR desta issue só pode ser mergeado com o ADR em `Status: Aprovado` OU com o status `Proposto` + comentário do engenheiro-chefe no PR confirmando que a análise está completa e a decisão fica agendada. **Nenhuma execução de banco ocorre nesta issue em hipótese alguma.**

## Arquivos a criar/alterar

- `docs/arquitetura/ADR-008-destino-banco.md` (entregável principal)
- `docs/arquitetura/ARQUITETURA-ALVO.md` (§5: linha apontando para o ADR-008; atualizar nota do ADR-006 indicando que a decisão foi tomada)
- `docs/db/CONSUMIDORES.md` (se a análise descobrir lacunas do inventário, registrar adendo datado — nunca sobrescrever evidência)
- `docs/PLANO-MIGRACAO.md` (status do M10 se aplicável)

## Critérios de aceite

- [ ] `docs/arquitetura/ADR-008-destino-banco.md` criado com as seções: Status, Contexto, Opções consideradas, Análise por consumidor, Decisão, Consequências, Plano de reversão, Aprovação
- [ ] As 4 opções (a)–(d) comparadas com prós/contras/custo mensal estimado/risco/esforço, usando dados reais (volumetria de [M10-01], métricas pós-cutover, preços AWS sa-east-1 vigentes)
- [ ] Tabela de análise POR CONSUMIDOR cobrindo 100% das linhas de `docs/db/CONSUMIDORES.md`, com coluna de veto e rastreabilidade na decisão final
- [ ] Opção (c) com a pré-condição sobre leitores externos de `ad_trackers` verificada contra o inventário, e o impacto na SK `created_at_id` e no `report-handler` analisado por escrito
- [ ] Critérios de decisão com pesos explícitos, sendo "risco para os outros projetos" o de maior peso, citando a diretriz do engenheiro-chefe (banco compartilhado, sem CI/CD de produção)
- [ ] Recomendação única e justificada + plano de reversão da opção recomendada
- [ ] Seção de Aprovação preenchida (ou processo de aprovação agendado e registrado no PR) — SEM aprovação explícita do engenheiro-chefe, [M10-03]/[M10-04] permanecem bloqueadas
- [ ] Nenhum comando executado contra banco/RDS nesta issue (issue 100% documental)

## Dependências

Bloqueada por: [M10-01] (inventário de consumidores — a análise por consumidor depende da matriz)

## Referências

- `docs/db/CONSUMIDORES.md` (entregável de [M10-01] — insumo principal)
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) — aviso do engenheiro-chefe, §2.4 (`ad_trackers` ~14M), §3 (migrations V1–V30), §4 (DynamoDB `AdTrackers`/`PostbackLogs`, formato `created_at_id`)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (topologia/regiões), §3 (Flyway/usuário DDL)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-002, ADR-006, §3 (decisão de dupla escrita), §6 (custos)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — diretriz do engenheiro-chefe; entregável M10 "plano aprovado pelo engenheiro-chefe"
- Issues: [M10-01] (insumo), [M10-03]/[M10-04] (consumidores desta decisão), [M6-01]/[M6-02] (relatórios afetados pela opção c), [M3-04] (tracker-writer/dupla escrita)
- Java (referência): `c:/Users/Fabio/Documents/Dev/ad-commons` (entities JPA), `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/db/migration` (30 migrations Flyway)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M10-02] ADR: decisão de destino do banco no repo
InteliFi/ad-serverless seguindo docs/issues/M10-02-adr-destino-banco.md e
CLAUDE.md. Issue 100% documental — NENHUM comando contra o banco (diretriz do
engenheiro-chefe: banco compartilhado com outros projetos, sem CI/CD de
produção, mudanças no final e com cuidado). Produzir
docs/arquitetura/ADR-008-destino-banco.md comparando (a) manter RDS MySQL,
(b) Aurora MySQL Serverless v2, (c) DynamoDB-only para tracking + MySQL para
cadastro, (d) PostgreSQL — com prós/contras/custo/risco POR CONSUMIDOR de
docs/db/CONSUMIDORES.md, critérios com pesos (risco aos outros projetos em
primeiro), recomendação única com plano de reversão e seção de Aprovação que
EXIGE assinatura explícita do engenheiro-chefe antes de qualquer execução.
Documento em português, abrir PR destacando que M10-03/M10-04 ficam bloqueadas
até a aprovação.
```
