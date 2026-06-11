---
title: "[M7-04] Alarmes (DLQ, erros, latência, RDS Proxy) + SNS"
labels: ["epic:M7-observabilidade", "tipo:infra", "prioridade:P1"]
milestone: "M7 — Observabilidade"
---
## Contexto

Dashboards (M7-03) só servem se alguém estiver olhando. Para o cutover (M9) e a operação assistida (M9-04), o sistema precisa **avisar sozinho** quando algo degrada — especialmente os riscos mapeados no [PLANO-MIGRACAO](../PLANO-MIGRACAO.md): perda de eventos no tracking assíncrono (DLQ), esgotamento de conexões do MySQL compartilhado (derruba OUTROS projetos) e parceiro upstream fora do ar (perda de receita silenciosa).

Esta issue cria os **alarmes CloudWatch como código** + **tópico SNS** com assinaturas de e-mail e Slack (via AWS Chatbot ou Lambda de webhook — decidir e documentar), com severidades distintas e mensagens acionáveis apontando para o runbook correspondente (M8-04).

## Especificação detalhada

### 1. Tópicos SNS

- `adserverless-alarms-critical-{stage}`: pagers — e-mail do time + canal Slack `#adserver-alerts` (integração via AWS Chatbot se disponível na conta; senão Lambda `alarm-notifier` que formata a mensagem do SNS para o webhook do Slack armazenado em SSM SecureString `/adserverless/{stage}/slack-webhook`).
- `adserverless-alarms-warning-{stage}`: e-mail + Slack, sem urgência.
- Assinaturas de e-mail parametrizadas (`${param:alertEmail}`) — confirmação manual da assinatura documentada no PR.

### 2. Catálogo de alarmes (todos como código no serverless.yml)

| # | Alarme | Métrica | Condição | Severidade |
|---|---|---|---|---|
| A1 | **DLQ com mensagens** | `AWS/SQS ApproximateNumberOfMessagesVisible` (tracking-dlq) | `> 0` por 1 datapoint de 1 min | CRITICAL |
| A2 | **Taxa de erro por função** | metric math `Errors/Invocations` (por função, 9 alarmes ou composite) | `> 1%` por 5 min | CRITICAL |
| A3 | **Latência p99 por rota** | `RequestLatency` EMF p99 (Service/Route) nas rotas hot (ad, vast, adtrack, vasttrack, redirect, proxy-tracker) | `> 500ms` por 10 min | WARNING (CRITICAL no canary) |
| A4 | **RDS Proxy conexões** | `AWS/RDS ClientConnections` e `DatabaseConnections` (ProxyName) | `> 80%` do máximo configurado, 5 min | CRITICAL (risco para outros projetos) |
| A5 | **Throttles Lambda** | `AWS/Lambda Throttles` (todas as funções) | `> 0` por 1 datapoint de 1 min | CRITICAL |
| A6 | **Postback upstream anormal** | `PostbackUpstreamFailed` EMF por Source | `>= 5` em 15 min OU `PostbackUpstreamSent == 0` por 6h em horário comercial (anomalia de volume — usar anomaly detection ou alarme de ausência com `TreatMissingData: breaching` documentado) | WARNING |
| A7 | Idade da mensagem SQS | `ApproximateAgeOfOldestMessage` (tracking-queue) | `> 300s` por 5 min | WARNING |
| A8 | Erros 5xx API Gateway | `5XXError` da HTTP API | `> 1%` por 5 min | CRITICAL |
| A9 | Upstream parceiro com erro | `UpstreamError` EMF por UpstreamHost (hosts críticos: smartadserver, 00px, adftech, metrike, doubleclick) | `>= 10` em 5 min | WARNING |

Detalhes obrigatórios:

- `TreatMissingData`: `notBreaching` para contadores de erro (ausência de dado = sem erro), EXCETO no A6-ausência (documentar o racional em comentário no yml).
- Cada alarme com `AlarmDescription` em português contendo: o que significa, impacto, e link relativo para o runbook (`docs/runbooks/...` — criados em M8-04; usar o caminho final combinado).
- Alarmes por função via loop/geração no serverless.yml (evitar copy-paste de 9 blocos divergentes); avaliar **composite alarm** "AdServerless-Unhealthy-{stage}" agregando os CRITICAL para reduzir ruído de notificação.
- Stage dev: mesmos alarmes com action apenas no tópico warning (sem pager), para validar a mecânica sem alert fatigue.

### 3. Validação obrigatória (não fechar sem isso)

Testar CADA alarme crítico em dev, com evidência no PR:

1. **A1:** enviar mensagem venenosa para a tracking-queue (JSON inválido) → tracker-writer falha → DLQ recebe → alarme dispara → Slack/e-mail chegam.
2. **A2/A8:** forçar 5xx (env var de fault injection temporária ou rota de teste) e verificar disparo.
3. **A5:** baixar `reservedConcurrency` de uma função de teste para 1 e gerar burst.
4. Registrar tempo entre violação e notificação (meta: < 3 min para CRITICAL).

### 4. Higiene

- Nenhum webhook/endpoint em texto plano no repo — SSM SecureString.
- Documentar no PR como adicionar/remover destinatários sem redeploy (assinatura SNS).

## Arquivos a criar/alterar

- `serverless.yml` (`resources:` — tópicos SNS, assinaturas, `AWS::CloudWatch::Alarm`, composite alarm)
- `infra/alarms/` (se a geração dos blocos for por script/template — documentar no Makefile)
- `cmd/alarmnotifier/main.go` (apenas se a via Slack for Lambda de webhook; comentado em português)
- `docs/runbooks/` — placeholders de link (conteúdo é M8-04)

## Critérios de aceite

- [ ] Todos os alarmes A1–A9 criados via deploy, visíveis no console, com `AlarmDescription` em português apontando runbook
- [ ] DLQ > 0 dispara CRITICAL em menos de 3 min (teste real com mensagem venenosa — evidência no PR)
- [ ] Taxa de erro > 1%/5min e throttles > 0 testados e notificando e-mail + Slack
- [ ] Alarme de RDS Proxy referencia o proxy via Ref (sem nome hardcoded) e o limiar 80% está parametrizado
- [ ] p99 > 500ms/10min configurado sobre a métrica EMF `RequestLatency` correta (Service/Route)
- [ ] A6 cobre tanto pico de falhas quanto ausência anômala de postbacks (anomaly detection ou alarme de ausência documentado)
- [ ] `TreatMissingData` correto e justificado em comentário para cada grupo de alarme
- [ ] Webhook do Slack em SSM SecureString; nenhum segredo no repo
- [ ] Stage dev sem pager (somente warning) — separação testada

## Dependências

Bloqueada por: M7-02 (métricas EMF — A3/A6/A9 dependem delas)

## Referências

- [docs/issues/M7-02-metricas-emf-xray.md](M7-02-metricas-emf-xray.md) (nomes/dimensões das métricas EMF)
- [docs/issues/M7-03-dashboards-cloudwatch.md](M7-03-dashboards-cloudwatch.md) (mesmas fontes de dados)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Riscos (conexões MySQL, perda de eventos, parceiro fora)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) ADR-002, ADR-003
- [Alarmes CloudWatch](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html) · [Composite alarms](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Create_Composite_Alarm.html)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M7-04] Alarmes + SNS seguindo
docs/issues/M7-04-alarmes-sns.md e CLAUDE.md. Criar tópicos SNS
critical/warning com e-mail + Slack (webhook em SSM SecureString), alarmes
A1–A9 da issue como código no serverless.yml (DLQ>0, erro>1%/5min,
p99>500ms/10min, RDS Proxy>80%, throttles>0, postback upstream anormal,
idade SQS, 5xx API GW, upstream parceiro), TreatMissingData justificado,
composite alarm para CRITICAL, dev sem pager. Testar A1/A2/A5 em dev com
evidências no PR. Descrições em português apontando runbooks.
```
