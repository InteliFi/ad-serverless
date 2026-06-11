---
title: "[M11-04] Real-time streaming — Kinesis Firehose → S3 + OpenSearch"
labels: ["epic:M11-backlog", "tipo:feature", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

**Nova feature.** Dashboards em tempo real e arquivo histórico bruto dos eventos — hoje a análise depende de queries no MySQL (14M+ linhas) com defasagem.

> Aproveitada do planejamento anterior (issue antiga #34). Executar somente após M9.

## Especificação detalhada

1. **Bifurcação no `tracker-writer`**: além da escrita atual (MySQL+DynamoDB), publicar o evento no **Kinesis Data Firehose** (PutRecord assíncrono; falha no Firehose NUNCA falha o processamento principal — log WARN).
2. **Firehose Delivery Stream**: buffer 60s/5MB; destinos:
   - **S3** (arquivo histórico): prefixo particionado `year=/month=/day=/` em formato Parquet (conversão nativa do Firehose) — base futura para Athena.
   - **OpenSearch Service** (ou OpenSearch Serverless): índice diário `ad-events-YYYY-MM-DD`, retenção 7 dias (ILM).
3. **Dashboards OpenSearch**: eventos por minuto (tempo real), heatmap de hotspots, top campanhas últimas 24h, funil de vídeo (start→25→50→75→complete).
4. **Custo**: estimar e documentar antes de ativar em prod (Firehose + OpenSearch ≈ maior item novo de custo; avaliar OpenSearch Serverless vs. t3.small.search).

## Arquivos a criar/alterar

- `internal/tracking/firehose.go` + integração no tracker-writer; recursos no serverless.yml; dashboards exportados em `docs/observabilidade/`

## Critérios de aceite

- [ ] Evento aparece no OpenSearch em <2 min após o request
- [ ] Falha do Firehose não afeta a escrita principal (teste de caos)
- [ ] Parquet no S3 consultável via Athena (query de validação documentada)
- [ ] Estimativa de custo aprovada antes do deploy em prod

## Dependências

Bloqueada por: M9 completo; usa [M3-04]

## Referências

- Issue antiga #34 (origem)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-04] Real-time streaming seguindo docs/issues/M11-04-realtime-streaming.md e CLAUDE.md. Firehose com destinos S3 Parquet + OpenSearch, bifurcação não-bloqueante no tracker-writer e dashboards. Código comentado em português. Abrir PR ao final.
```
