// Stack principal do Ad Serverless
// Agrega todos os constructs de infraestrutura em uma única stack CloudFormation
// Cada componente (DynamoDB, Aurora, ElastiCache, SQS, etc.) é importado e configurado aqui

import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { DynamoDbTablesConstruct } from './constructs/dynamodb-tables';
import { AuroraClusterConstruct } from './constructs/aurora-cluster';
import { ElastiCacheClusterConstruct } from './constructs/elasticache-cluster';
import { SqsQueuesConstruct } from './constructs/sqs-queues';
import { CloudFrontWafConstruct } from './constructs/cloudfront-waf';
import { ObservabilityConstruct } from './constructs/observability';

/**
 * Stack principal do Ad Serverless
 *
 * Esta stack orquestra todos os componentes de infraestrutura:
 * - DynamoDB: tabelas para tracking em tempo real (hot path)
 * - Aurora Serverless v2: banco relacional para analytics e reporting
 * - ElastiCache Redis: cache distribuído (Tier 3 da estratégia de cache)
 * - SQS: filas assíncronas para processamento de eventos de tracking
 * - CloudFront + WAF: CDN e proteção contra ataques
 * - Observability: ADOT Lambda Layer, X-Ray, CloudWatch dashboards
 */
export class AdServerlessStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // ============================================================
    // LAYER 1 - Observabilidade (base para todos os componentes)
    // ============================================================
    const observability = new ObservabilityConstruct(this, 'Observability');

    // ============================================================
    // LAYER 2 - Filas e Mensageria (async tracking)
    // ============================================================
    const sqsQueues = new SqsQueuesConstruct(this, 'SqsQueues');

    // ============================================================
    // LAYER 3 - Banco de Dados
    // ============================================================

    // DynamoDB: hot path para tracking events (escrita alta, leitura baixa)
    const dynamoDbTables = new DynamoDbTablesConstruct(this, 'DynamoDbTables');

    // Aurora Serverless v2 + RDS Proxy: analytics e reporting relacional
    const auroraCluster = new AuroraClusterConstruct(this, 'AuroraCluster');

    // ElastiCache Redis: cache distribuído (Tier 3)
    const elasticacheCluster = new ElastiCacheClusterConstruct(this, 'ElastiCacheCluster');

    // ============================================================
    // LAYER 4 - CDN e Segurança
    // ============================================================
    const cloudFrontWaf = new CloudFrontWafConstruct(this, 'CloudFrontWaf');

    // ============================================================
    // OUTPUTS - Valores exportados para uso nos Lambda functions
    // ============================================================

    // Outputs das tabelas DynamoDB
    new cdk.CfnOutput(this, 'TrackingTableArn', {
      value: dynamoDbTables.trackingTable.tableArn,
      description: 'ARN da tabela de tracking principal no DynamoDB',
    });

    // Output das filas SQS
    new cdk.CfnOutput(this, 'TrackingQueueUrl', {
      value: sqsQueues.trackingQueue.queueUrl,
      description: 'URL da fila principal de tracking events',
    });
  }
}
