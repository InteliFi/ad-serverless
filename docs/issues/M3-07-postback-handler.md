---
title: "[M3-07] postback-handler: GET /adtrack/postback"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

`GET /adtrack/postback` recebe conversões de redes de afiliados (Modatta, Prezão Claro etc.), registra o evento como `POSTBACK_*` e dispara postbacks upstream para parceiros específicos. Port de `AdTrackService.insertPostback` + `AdTrackComponent.savePostback` + `PostbackLogComponent.logPostback` ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §8; [02-logica-negocio.md](../legado/02-logica-negocio.md) §1.4 e §1.9). Lambda dedicada `postback-handler` (256MB / 29s — os upstreams podem demorar até 30s de read timeout).

⚠️ Particularidade de paridade: diferente do `/adtrack` comum, o postback grava o AdTracker **SOMENTE no MySQL** (sem réplica na tabela DynamoDB `AdTrackers`) — e registra um item na tabela DynamoDB `PostbackLogs`. Não usar SQS aqui: a escrita é síncrona como no legado (endpoint fora do hot path).

## Especificação detalhada

### 1. Query params

| Param | Obrigatório | Notas |
|---|---|---|
| `cid` | sim (int) | campaign ID |
| `event` | sim | sufixo do event type (ex.: `CPL`) |
| `aff_sub` | sim | |
| `click_id` | sim | |
| `source` | sim | decide postback upstream |
| `transaction_id` | não | PK do PostbackLogs (fallback `click_id`) |
| `payout` | não | Number com parse seguro |
| `currency` | não | |
| `sale_amount` | não | Number com parse seguro |

Param obrigatório ausente → `400` (comportamento do binding do Spring; validar explicitamente).

### 2. Fluxo (ordem EXATA do legado)

1. **Timestamp do servidor:** agora em `America/Sao_Paulo`, **truncado a segundos**, normalizado para manter apenas o **offset** (sem o nome da zona) — equivalente do `ZonedDateTime.now(SP).truncatedTo(SECONDS).withZoneSameInstant(offset)`: ex. `2026-06-10T15:30:45-03:00`. Relógio injetado (nunca `time.Now()` na lógica).
2. **`savePostback` (portar de `AdTrackComponentImpl`):**
   a. Montar `eventType = "POSTBACK_" + event` e validar contra os valores persistidos do EventType (M1-06). Válidos: `POSTBACK_CLICK`, `POSTBACK_CPL`, `POSTBACK_CPA`, `POSTBACK_INSTALL_ANDROID`, `POSTBACK_INSTALL_IOS`. Inválido → `AdException("Invalid event type")` → **`422`**.
   b. A validação de assinatura (`PostbackSignature` MD5, M1-06) está **COMENTADA no legado (TODO)** — manter DESLIGADA (paridade); reativação é issue `melhoria` separada. Manter no handler o branch de `NotAuthorizedException → 401` (existe no código e na espec).
   c. Validar campanha existe E está ativa: `FindCampaignActive` (M3-01) + elegibilidade `internal/frequencycap` (M1-02) contra o relógio injetado. Inexistente/inativa → `CampaignNotFoundException` → **`404`**.
   d. **INSERT MySQL `ad_trackers`** (M3-01): `campaign_id=cid`, `hotspot_id = POSTBACK_HOTSPOT` (config `POSTBACK_HOTSPOT_CODE`, default `"POSTBACK_HOTSPOT"` — paridade com `intv.ad.postback.hotspot.code`), `event_type = "POSTBACK_"+event`, `creation_date`/`event_date` derivados do timestamp do passo 1. **NÃO gravar na tabela DynamoDB AdTrackers** (paridade!).
3. **Log de auditoria:** `slog.Info("Postback received", cid, event, transaction_id, aff_sub, click_id, source, payout, currency, sale_amount)` (paridade com o log de stdout do legado).
4. **`PutPostbackLog`** (M3-02) na tabela `PostbackLogs`: PK `transaction_id` (vazio → fallback `click_id`), SK `logged_at` = timestamp do passo 1 em ISO8601, atributos `campaign_id`/`event` (sem o prefixo `POSTBACK_` — o legado loga o `event` cru)/`aff_sub`/`click_id`/`source`/`payout` (N, parse seguro)/`currency`/`sale_amount` (N, parse seguro). No legado era `@Async` fire-and-forget: em Go executar síncrono ANTES da resposta, mas **falha apenas loga ERROR — não muda a resposta HTTP**.
5. **Postback upstream por `source`** (portar de `processSourceSpecificPostback`; client HTTP com connect timeout **10s** e read timeout **30s**):
   - `source == "bW9kYXR0YQ"` (base64 de "modatta") → `GET https://pb.modatta.org/external/affiliates/pb/9A061369-320A-4C1E-9451-E1BA9991E193?modid={click_id}` (click_id URL-encoded via query builder);
   - `source == "prezao_claro"` → `GET https://api.prezaofree.com.br/event/postback?partnerId=425701220616215160&event=register&clickId={click_id}` (concatenação direta — paridade);
   - qualquer outro `source` → nada (log DEBUG `No special postback handling for source: {source}`);
   - **QUALQUER falha upstream (HTTP 4xx/5xx, timeout, rede) → apenas `slog.Warn` — NUNCA afeta a resposta** (o postback já foi persistido). Logar status+corpo no sucesso (INFO) como o legado.
6. **Resposta `202 Accepted`** com corpo vazio.

### 3. Mapa de respostas

| Condição | Status |
|---|---|
| Sucesso | `202` |
| `NotAuthorizedException` (branch preservado) | `401` |
| `CampaignNotFoundException` (campanha inexistente/inativa) | `404` |
| `AdException`/erro genérico (incl. event type inválido, falha MySQL) | `422` |
| Param obrigatório ausente | `400` |

### 4. Estrutura e configuração

- `cmd/postback/main.go` — rota `GET /adtrack/postback`, middlewares M1-07.
- `internal/postback/handler.go` (+ `upstream.go` para os parceiros), interfaces para MySQL, Dynamo e HTTP client (testabilidade).
- `serverless.yml`: function `postback-handler`, 256MB, timeout 29s, IAM `dynamodb:PutItem` apenas na tabela PostbackLogs; env `POSTBACK_HOTSPOT_CODE`.
- URLs/IDs dos parceiros podem ficar como constantes documentadas (hardcode = paridade), com comentário apontando o legado.

### 5. Testes

- Event type válido → 202 + INSERT MySQL com `hotspot_id=POSTBACK_HOTSPOT` e `event_type=POSTBACK_CPL` + PutPostbackLog (mocks verificam argumentos exatos, incluindo o timestamp truncado com offset).
- Event inválido → 422 sem INSERT; campanha inexistente/inativa (frequency cap fora da janela) → 404 sem INSERT.
- `transaction_id` vazio → PostbackLogs com PK = `click_id`.
- `payout`/`sale_amount` inválidos → atributo omitido + WARN (comportamento do M3-02, teste de integração dos mocks).
- `source=bW9kYXR0YQ` → GET na URL Modatta exata com `modid`; `source=prezao_claro` → URL Prezão exata; upstream 500/timeout → resposta continua 202 (httptest).
- Falha do PutPostbackLog → ERROR logado, resposta continua 202.
- Verificação de que NENHUMA escrita na tabela AdTrackers do DynamoDB acontece (mock garante zero chamadas).

## Arquivos a criar/alterar

- `cmd/postback/main.go`
- `internal/postback/handler.go` (`// Portado de: AdTrackService.java (insertPostback) / AdTrackComponentImpl.java (savePostback)`)
- `internal/postback/upstream.go` (`// Portado de: AdTrackService.java (processSourceSpecificPostback)`)
- `internal/postback/*_test.go`
- `serverless.yml` (function `postback-handler`)
- `docs/MATRIZ-PARIDADE.md`

## Critérios de aceite

- [ ] `GET /adtrack/postback` com params obrigatórios cid/event/aff_sub/click_id/source; ausentes → 400
- [ ] Timestamp São Paulo truncado a segundos com offset preservado (ex. `-03:00`), relógio injetado
- [ ] Event type inválido → 422; campanha inexistente/inativa → 404; sucesso → 202; branch 401 preservado
- [ ] INSERT `ad_trackers` com `hotspot_id=POSTBACK_HOTSPOT` e `event_type="POSTBACK_"+event` — **somente MySQL**, zero escrita na tabela DynamoDB AdTrackers (teste garante)
- [ ] `PutPostbackLog` com fallback transaction_id→click_id e parse seguro de payout/sale_amount; falha só loga, não muda resposta
- [ ] Upstreams Modatta e Prezão Claro com URLs EXATAS, timeouts 10s connect / 30s read; falha upstream → apenas WARN, resposta 202
- [ ] Validação de assinatura permanece desligada (comentário referenciando o TODO do legado e a futura issue `melhoria`)
- [ ] `// Portado de:` presentes; godoc em português; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M3-01 (InsertAdTracker/FindCampaignActive), M3-02 (PutPostbackLog). Usa M1-02 (frequencycap), M1-06 (event types), M1-07/M1-08.

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §8
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.4, §1.9, §3 (exceções), §4 (EventType)
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §4.2 (PostbackLogs), §5 (conversões de data)
- Java: `ad-server/.../presentation/service/AdTrackService.java` (insertPostback, processSourceSpecificPostback), `business/component/impl/AdTrackComponentImpl.java`, `PostbackLogComponent.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-07] postback-handler no repo InteliFi/ad-serverless
seguindo docs/issues/M3-07-postback-handler.md e CLAUDE.md: GET /adtrack/postback
com params obrigatórios cid/event/aff_sub/click_id/source e opcionais
transaction_id/payout/currency/sale_amount; timestamp São Paulo truncado a segundos
com offset; valida event type (inválido→422) e campanha ativa (404); INSERT
ad_trackers com hotspot=POSTBACK_HOTSPOT e eventType="POSTBACK_"+event SOMENTE
MySQL (sem DynamoDB AdTrackers); PutItem PostbackLogs com fallback
transaction_id→click_id; postbacks upstream Modatta (source bW9kYXR0YQ) e Prezão
Claro (source prezao_claro) com URLs exatas e timeouts 10s/30s, falha upstream
apenas WARN; respostas 202/401/404/422. Código comentado em português. Testes
verdes. Abrir PR ao final.
```
