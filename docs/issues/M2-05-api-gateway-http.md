---
title: "[M2-05] API Gateway HTTP API: rotas, CORS, catch-all, error responses"
labels: ["epic:M2-infra", "tipo:infra", "prioridade:P1"]
milestone: "M2 — Infra AWS"
---
## Contexto

Todos os 16 endpoints do legado ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md)) passam a ser servidos por um **API Gateway HTTP API** (payload v2.0) roteando para as 9 Lambdas ([ARQUITETURA-ALVO](../arquitetura/ARQUITETURA-ALVO.md) §3). Este endpoint substitui o Tomcat das EC2 — incluindo seus comportamentos permissivos com URLs malformadas, que players de vídeo reais enviam (risco mapeado no [PLANO-MIGRACAO](../PLANO-MIGRACAO.md)).

## Especificação detalhada

### Tabela completa de rotas (serverless.yml `httpApi` events)

| Rota | Função |
|---|---|
| `GET /` , `GET /health`, `GET /healthz` (+ HEAD via `ANY`?) | ad-handler |
| `GET /ad` | ad-handler |
| `GET /GAM` | ad-handler |
| `GET /vast` | vast-handler |
| `POST /adtrack` | track-handler |
| `GET /vasttrack` | track-handler |
| `GET /trackingpixel` | track-handler |
| `GET /adtrack` | report-handler |
| `GET /adtrack/xls` | report-handler |
| `GET /adtrack/postback` | postback-handler |
| `GET /redirect` | redirect-handler |
| `GET /proxy-tracker`, `OPTIONS /proxy-tracker` | proxy-handler |
| `GET /proxy-audit` | proxy-handler |
| `GET /safeframe/proxy-safeframe`, `OPTIONS /safeframe` | proxy-handler |
| `GET /media/{filename+}` | media-handler |
| `$default` | resposta 400 (ver abaixo) |

Notas:
- **HEAD**: o HTTP API trata HEAD como GET na mesma rota — validar que os health checks respondem a HEAD (paridade com `ApplicationStatusService`).
- **`GET /adtrack` vs `POST /adtrack`**: mesmo path, métodos distintos, funções distintas — suportado nativamente.
- **`$default`**: rota catch-all apontando para uma mini-função (ou o ad-handler) que responde `400` com corpo `Bad Request: Invalid HTTP format` — paridade com o `/error/400` do legado.

### CORS — decisão

NÃO usar o CORS nativo do HTTP API: o legado **reflete o Origin com `Access-Control-Allow-Credentials: true`** ([docs/legado/01](../legado/01-endpoints-http.md) §16), o que o CORS nativo não faz (não permite credentials com origin dinâmico). O CORS fica 100% no middleware Go ([M1-07]) e o `httpApi.cors` fica **desabilitado** para não conflitar (headers duplicados quebram browsers).

### URLs malformadas (risco crítico)

O Tomcat legado roda com `relaxedQueryChars`/`relaxedPathChars` para `| { } [ ] ^ \` < > \ ; : @ & = + $ # %`. O HTTP API é mais permissivo que o REST API, mas:
1. Capturar do access log das EC2 ~50 URLs reais com caracteres especiais (especialmente `/proxy-tracker?u=` e `/vast?vcurl=`);
2. Testar cada uma contra o stage dev e registrar o resultado em `docs/infra/URLS-MALFORMADAS.md`;
3. Se houver rejeições, avaliar mitigação (encode no edge via CloudFront Function) ANTES do cutover.

### Outras configurações

- Payload format version 2.0 em todas as integrações; timeout de integração = timeout da função.
- Throttling default do stage: 500 rps burst / 200 rps rate em dev; sem limite custom em prod (Lambda escala).
- Logs de acesso do API Gateway habilitados (JSON: requestId, route, status, latency, ip, userAgent).
- Domínio custom fica para o cutover ([M9-03]); até lá usa-se o endpoint padrão `*.execute-api`.

## Arquivos a criar/alterar

- `serverless.yml` — events `httpApi` em todas as funções + `$default`
- `docs/infra/URLS-MALFORMADAS.md` — resultado dos testes com URLs reais

## Critérios de aceite

- [ ] `serverless deploy --stage dev` cria as rotas; `curl` em cada rota retorna o status esperado (mesmo que handler ainda seja stub)
- [ ] `$default` responde 400 com o corpo exato `Bad Request: Invalid HTTP format`
- [ ] HEAD /health responde 200
- [ ] 50 URLs reais malformadas testadas e documentadas, sem rejeição não mitigada
- [ ] CORS nativo desabilitado (resposta sem headers duplicados)

## Dependências

Bloqueada por: [M0-02]

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) (tabela de endpoints e filtros)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 (chars relaxados do Tomcat)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) (risco de URLs malformadas)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M2-05] API Gateway HTTP API seguindo docs/issues/M2-05-api-gateway-http.md e CLAUDE.md. Configurar todas as rotas da tabela no serverless.yml com payload v2.0, $default com 400 de paridade, CORS nativo desabilitado (middleware Go cuida), e documentar o teste de URLs malformadas. Abrir PR ao final.
```
