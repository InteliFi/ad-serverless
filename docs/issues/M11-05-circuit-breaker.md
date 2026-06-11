---
title: "[M11-05] Circuit breaker + fallbacks para upstreams de parceiros"
labels: ["epic:M11-backlog", "tipo:feature", "melhoria", "prioridade:P3"]
milestone: "M11 — Backlog Pós-Cutover"
---
## Contexto

**Melhoria de resiliência (o legado não tem).** Quando um parceiro upstream degrada (SmartAdServer lento, modatta fora), hoje cada request paga o timeout inteiro. Circuit breaker corta rápido e aplica fallback — importante no vast-handler (29s de timeout × picos = custo e latência).

> Aproveitada do planejamento anterior (issue antiga #35, incluindo a tabela por parceiro). Executar somente após M9 — durante a paridade, o comportamento de timeout deve ser idêntico ao legado.

## Especificação detalhada

1. **Biblioteca**: `sony/gobreaker` (ou implementação própria simples) por host upstream, estado compartilhado por container.
2. **Configuração por parceiro** (valores iniciais da issue antiga, ajustar com dados reais de M7):

   | Parceiro | Host | Abre com | Janela | Fallback |
   |---|---|---|---|---|
   | Modatta | pb.modatta.org | 50% falhas | 30s | já é fire-and-forget: log WARN (comportamento atual) — breaker só corta o tempo gasto |
   | Prezão Claro | api.prezaofree.com.br | 50% | 30s | idem |
   | SmartAdServer | videoapi.smartadserver.com | 60% | 60s | VAST vazio 404 rápido (player faz fallback) — avaliar campanha default |
   | Space/00px | cdn.00px.net | 70% | 45s | servir JS sem verificação (comentário no corpo) |
   | Metrike | servedby.metrike.com.br | 60% | 60s | idem SmartAd |

3. **Métricas EMF**: estado do breaker por host (aberto/fechado), cortes aplicados ([M7-02]); alarme quando um breaker abre ([M7-04]).
4. **Flag de desativação** por env var (rollback instantâneo para comportamento de paridade).

## Arquivos a criar/alterar

- `internal/httpx/breaker.go` + integração nos clients de vast/proxy/postback; testes com upstream mock falhando

## Critérios de aceite

- [ ] Breaker abre/fecha conforme thresholds (testes determinísticos com clock injetado)
- [ ] Fallback por parceiro correto; flag desliga tudo
- [ ] Métricas e alarme funcionando
- [ ] Sem mudança de comportamento com breaker fechado (golden tests continuam verdes)

## Dependências

Bloqueada por: M9 completo

## Referências

- Issue antiga #35 (origem da tabela)
- Runbook `parceiro-upstream-fora.md` ([M8-04]) — atualizar com o breaker

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M11-05] Circuit breaker seguindo docs/issues/M11-05-circuit-breaker.md e CLAUDE.md. Breaker por host com fallbacks da tabela, flag de desativação, métricas/alarme e golden tests intactos. Código comentado em português. Abrir PR ao final.
```
