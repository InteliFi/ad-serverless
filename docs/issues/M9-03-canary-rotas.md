---
title: "[M9-03] Canary por rota (CloudFront/Route53)"
labels: ["epic:M9-cutover", "tipo:infra", "prioridade:P1"]
milestone: "M9 — Cutover"
---
## Contexto

Com o shadow traffic limpo ([M9-02]), o tráfego real de `ads.inteli.fi` migra das EC2 para as Lambdas **rota por rota, com pesos progressivos** — a fase mais delicada da migração. O plano de ordem e degraus está no [PLANO-MIGRACAO](../PLANO-MIGRACAO.md); esta issue implementa a mecânica e executa.

## Especificação detalhada

### Mecânica de pesos (decidir e documentar em docs/cutover/CANARY.md)

Opção recomendada: **CloudFront com 2 origins** (ALB/EC2 legado + API Gateway) e **behaviors por path**, usando origin groups ou uma CloudFront Function para split percentual por hash de IP (consistência por cliente). Alternativa: Route53 weighted records — rejeitar se o TTL de DNS tornar o rollback lento (>60s). Critério da decisão: rollback deve ser **1 ação com efeito em <1 min**.

### Ordem das rotas (menor → maior criticidade, do PLANO-MIGRACAO)

```
/health → /redirect → /trackingpixel → /adtrack + /vasttrack → /adtrack/postback
→ /ad + /GAM → /proxy-tracker + /proxy-audit + /safeframe/* → /media → /vast
```

### Degraus e critérios

- Degraus por rota: **5% → 25% → 50% → 100%**, janela de observação de **24–48h** por degrau (cobrir picos diários).
- **Critérios de avanço** (todos obrigatórios): taxa de erro da rota < 0,1%; p99 dentro da meta ([docs/legado/05](../legado/05-config-infra-deploy.md) §6); reconciliação de eventos do dia ok (script de [M9-04], desvio < 2%); zero mensagens novas na DLQ sem explicação; nenhuma reclamação de parceiro.
- **Rollback**: reverter peso para 0% (1 ação); registrar causa em docs/cutover/LOG.md; só retomar após correção + shadow da rota limpa.
- Rotas de tracking exigem atenção extra: comparar contagem MySQL×DynamoDB da rota canária vs. controle ANTES de avançar o degrau.

### Registro

`docs/cutover/LOG.md` — diário de bordo: data/hora, rota, degrau, métricas observadas, decisão (avançar/segurar/rollback), assinado por quem decidiu.

## Arquivos a criar/alterar

- `serverless.yml`/IaC — origins, behaviors e mecanismo de peso
- `docs/cutover/CANARY.md` — mecânica, critérios, procedimento de rollback
- `docs/cutover/LOG.md` — diário de execução

## Critérios de aceite

- [ ] Mecânica implantada com rollback testado em dev (simulação completa: subir peso, reverter, medir tempo de efeito < 1 min)
- [ ] CANARY.md aprovado pelo engenheiro-chefe ANTES do primeiro degrau em prod
- [ ] Execução completa: todas as rotas a 100% com critérios atendidos e LOG.md preenchido
- [ ] EC2 ainda quentes ao final (desligamento é [M9-05], após [M9-04])

## Dependências

Bloqueada por: [M9-02]

## Referências

- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) (fases de risco do cutover)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §6 (metas)
- Issues [M9-02], [M9-04], [M7-04]

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M9-03] Canary por rota seguindo docs/issues/M9-03-canary-rotas.md e CLAUDE.md. Implementar a mecânica de pesos com rollback <1min, documentar CANARY.md com critérios de avanço/rollback e preparar o LOG.md. A execução em prod requer aprovação humana por degrau. Abrir PR ao final.
```
