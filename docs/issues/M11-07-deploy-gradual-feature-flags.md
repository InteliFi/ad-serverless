---
title: "[M11-07] Deploy gradual contínuo — Lambda aliases/CodeDeploy + feature flags"
labels: ["epic:M11-backlog", "tipo:infra", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

O canary do cutover ([M9-03]) é um mecanismo pontual de migração. Esta melhoria estabelece deploy gradual **permanente** para o dia a dia pós-migração: cada release nova de Lambda recebe tráfego progressivamente com rollback automático por métrica.

> Aproveitada do planejamento anterior (issue antiga #47 — Blue/Green + Feature Flags).

## Especificação detalhada

1. **Lambda aliases + CodeDeploy**: publicar versões e usar `AWS::CodeDeploy` com configuração `Canary10Percent5Minutes` (ou `Linear10PercentEvery1Minute`) nas funções de hot path (vast, ad, track); rollback automático acionado por alarmes do CloudWatch ([M7-04]: error rate, p99) durante a janela de shift.
2. **Integração com Serverless Framework**: plugin `serverless-plugin-canary-deployments` (avaliar manutenção do plugin; alternativa: recursos CodeDeploy crus no `resources`). Documentar a escolha.
3. **Feature flags via AWS AppConfig** (avaliar vs. SSM simples — custo/complexidade):
   - Flags iniciais: `validacao-assinatura-postback` ([M1-06]), `circuit-breaker` ([M11-05]), `event-cap` ([M11-06]).
   - Cache local de flags por container com TTL 60s; mudança de flag SEM redeploy.
4. **Pipeline**: o deploy de prod ([M0-04]) passa a publicar versão + shift gradual em vez de update direto; smoke test roda contra o alias `live` durante o shift.

## Arquivos a criar/alterar

- `serverless.yml` (aliases/CodeDeploy), `.github/workflows/deploy.yml`, `internal/platform/flags.go` + testes, `docs/infra/DEPLOY-GRADUAL.md`

## Critérios de aceite

- [ ] Deploy em dev com shift gradual observável e rollback automático testado (injetar erro proposital e ver o CodeDeploy reverter)
- [ ] Flag alterada no AppConfig reflete nas Lambdas em ≤60s sem redeploy
- [ ] Documentação da mecânica e do procedimento de emergência

## Dependências

Bloqueada por: M9 completo; usa [M7-04]

## Referências

- Issue antiga #47 (origem); [M0-04] (pipeline atual); [M9-03] (canary do cutover)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-07] Deploy gradual + feature flags seguindo docs/issues/M11-07-deploy-gradual-feature-flags.md e CLAUDE.md. Aliases com CodeDeploy e rollback por alarme, flags via AppConfig com cache de 60s, pipeline atualizado e teste de rollback real em dev. Abrir PR ao final.
```
