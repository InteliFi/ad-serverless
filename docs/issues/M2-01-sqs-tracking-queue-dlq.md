---
title: "[M2-01] SQS tracking-queue + DLQ"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P1"]
milestone: "M2 — Infra AWS"
---
## Contexto

No Java, o tracking era persistido em MySQL síncrono + DynamoDB via `@Async` (ThreadPool core 2 / max 4 / queue 100 — [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 "Async"). Em Lambda, goroutines/threads NÃO sobrevivem ao freeze do container: um fire-and-forget em memória perderia eventos (ADR-003 da [ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md)).

A decisão de arquitetura é: o `track-handler` (M3-03) valida o evento e publica em uma **fila SQS Standard**; o `tracker-writer` (M3-04) consome em batch e faz a dupla escrita (MySQL `ad_trackers` + DynamoDB `AdTrackers`). Esta issue provisiona a fila, a DLQ e o alarme — pré-requisito direto da M3-03.

Por que **Standard** (e não FIFO): o legado não garante ordem nem deduplicação (o `@Async` Java também não garantia); Standard tem throughput ilimitado e custa menos. O timestamp do evento é o do **request original** (param `time`), então reordenação no consumo não altera os dados persistidos.

## Especificação detalhada

### 1. Recursos CloudFormation no `serverless.yml` (seção `resources:`)

Criar nas DUAS stages (dev → us-east-1, prod → sa-east-1; a região já é resolvida por `custom.regions`, M0-02):

```yaml
resources:
  Resources:
    # DLQ primeiro (a fila principal referencia o ARN dela)
    TrackingDLQ:
      Type: AWS::SQS::Queue
      Properties:
        QueueName: ad-serverless-tracking-dlq-${sls:stage}
        MessageRetentionPeriod: 1209600        # 14 dias — tempo máximo para investigar eventos falhos
        SqsManagedSseEnabled: true

    TrackingQueue:
      Type: AWS::SQS::Queue
      Properties:
        QueueName: ad-serverless-tracking-queue-${sls:stage}
        VisibilityTimeout: 90                  # > timeout do tracker-writer (60s) — exigência da AWS p/ event source
        MessageRetentionPeriod: 345600         # 4 dias
        ReceiveMessageWaitTimeSeconds: 20      # long polling (menos receives vazios, menor custo)
        SqsManagedSseEnabled: true
        RedrivePolicy:
          deadLetterTargetArn: !GetAtt TrackingDLQ.Arn
          maxReceiveCount: 5                   # 5 tentativas de processamento antes de ir para a DLQ

    # Tópico de alarmes do projeto (assinaturas de e-mail/Slack entram na M7-04)
    AlarmsTopic:
      Type: AWS::SNS::Topic
      Properties:
        TopicName: ad-serverless-alarms-${sls:stage}

    # Qualquer mensagem na DLQ = evento de tracking perdido após 5 tentativas → investigar SEMPRE
    TrackingDLQAlarm:
      Type: AWS::CloudWatch::Alarm
      Properties:
        AlarmName: ad-serverless-tracking-dlq-messages-${sls:stage}
        AlarmDescription: "Mensagens na DLQ de tracking — eventos de ad tracking falharam 5x e precisam de investigação manual"
        Namespace: AWS/SQS
        MetricName: ApproximateNumberOfMessagesVisible
        Dimensions:
          - Name: QueueName
            Value: !GetAtt TrackingDLQ.QueueName
        Statistic: Maximum
        Period: 60
        EvaluationPeriods: 1
        Threshold: 1
        ComparisonOperator: GreaterThanOrEqualToThreshold
        TreatMissingData: notBreaching
        AlarmActions:
          - !Ref AlarmsTopic

  Outputs:
    TrackingQueueUrl:
      Value: !Ref TrackingQueue
      Export: { Name: ad-serverless-${sls:stage}-tracking-queue-url }
    TrackingQueueArn:
      Value: !GetAtt TrackingQueue.Arn
      Export: { Name: ad-serverless-${sls:stage}-tracking-queue-arn }
    TrackingDLQArn:
      Value: !GetAtt TrackingDLQ.Arn
      Export: { Name: ad-serverless-${sls:stage}-tracking-dlq-arn }
```

### 2. Variável de ambiente para os handlers (contrato com M3-03)

Em `provider.environment` (ou na função `track-handler` quando ela existir), expor:

```yaml
TRACKING_QUEUE_URL: !Ref TrackingQueue
```

O nome `TRACKING_QUEUE_URL` é o contrato já documentado na issue M3-03 (`internal/tracking/publisher.go`) — NÃO renomear.

### 3. Justificativas (registrar como comentários no serverless.yml)

- `VisibilityTimeout: 90` — o `tracker-writer` tem timeout 60s (ARQUITETURA §3); o visibility precisa ser maior que o timeout da função consumidora, senão o deploy do event source mapping falha.
- `maxReceiveCount: 5` — paridade aproximada com o retry 3× + margem do legado (`DynamoDB async com retry 3× backoff exponencial`, [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §5); após 5 falhas a mensagem é preservada na DLQ em vez de descartada (melhor que o legado, que perdia o evento — ganho intrínseco da arquitetura, não é `melhoria` de comportamento de negócio).
- Permissões (`sqs:SendMessage` para o track-handler, `ReceiveMessage/DeleteMessage/GetQueueAttributes` para o tracker-writer) NÃO entram aqui — são da M2-04 (IAM least privilege).

## Arquivos a criar/alterar

- `serverless.yml` (seção `resources:` com TrackingQueue, TrackingDLQ, AlarmsTopic, TrackingDLQAlarm e Outputs; env `TRACKING_QUEUE_URL`)
- `docs/MATRIZ-PARIDADE.md` (linha "tracking assíncrono @Async → SQS+DLQ": infra provisionada)

## Critérios de aceite

- [ ] `npx serverless deploy --stage dev` cria fila, DLQ, tópico SNS e alarme em us-east-1
- [ ] `aws sqs get-queue-attributes --queue-url <url> --attribute-names All --region us-east-1` mostra `VisibilityTimeout=90`, `ReceiveMessageWaitTimeSeconds=20`, `RedrivePolicy` com `maxReceiveCount: 5` apontando para a DLQ
- [ ] DLQ com `MessageRetentionPeriod=1209600` (14 dias) e SSE habilitado nas duas filas
- [ ] Teste do alarme: `aws sqs send-message --queue-url <dlq-url> --message-body teste` coloca o alarme em estado `ALARM` em ~1 min (`aws cloudwatch describe-alarms --alarm-names ad-serverless-tracking-dlq-messages-dev`); depois `purge-queue` na DLQ e alarme volta a `OK`
- [ ] Roundtrip básico: `aws sqs send-message` na fila principal seguido de `receive-message` devolve a mensagem
- [ ] Outputs `TrackingQueueUrl/Arn` e `TrackingDLQArn` exportados no CloudFormation
- [ ] `npx serverless print --stage prod` resolve os mesmos recursos para sa-east-1 (deploy prod ocorrerá pelo pipeline M0-04 com aprovação)
- [ ] Nenhuma credencial/segredo no serverless.yml; comentários do yml em português

## Dependências

Bloqueada por: #M0-02

## Referências

- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (fila de tracking), §3 (tracker-writer), ADR-003
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §5 e §9 (persistência MySQL+DynamoDB do tracking)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 (ThreadPool @Async substituído)
- Issue M3-03 (contrato `TRACKING_QUEUE_URL` + struct `TrackingEvent`), M3-04 (consumidor), M7-04 (assinaturas do SNS)
- Java: `ad-server` — configuração `@EnableAsync`/ThreadPool (`PostbackLog-`) substituída por esta fila

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-01] SQS tracking-queue + DLQ no repo
InteliFi/ad-serverless seguindo docs/issues/M2-01-sqs-tracking-queue-dlq.md
e CLAUDE.md: adicionar à seção resources do serverless.yml a fila standard
ad-serverless-tracking-queue-{stage} (visibility 90s, long polling 20s, SSE),
a DLQ ad-serverless-tracking-dlq-{stage} (retenção 14 dias, maxReceiveCount 5),
o tópico ad-serverless-alarms-{stage} e o alarme de mensagens na DLQ, com
Outputs exportados e env TRACKING_QUEUE_URL. Comentários do yml em português.
Validar deploy em dev e o teste do alarme. Abrir PR ao final.
```
