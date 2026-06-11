---
title: "[M3-03] track-handler: POST /adtrack + GET /vasttrack → SQS"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

`POST /adtrack` e `GET /vasttrack` são os endpoints de registro de eventos (page view, impressão, clique, progresso de vídeo). No Java (`AdTrackService.java`, `VastTrackService.java`) a escrita era MySQL síncrono + DynamoDB `@Async`. Em Lambda, goroutines não sobrevivem ao freeze do container (ADR-003), então o `track-handler` apenas **valida e publica o evento no SQS**, respondendo imediatamente (~5ms). A escrita dupla acontece no `tracker-writer` (M3-04).

Espec funcional: [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §5 e §9.

## Especificação detalhada

### 1. `POST /adtrack` (portado de `AdTrackService.java`)

- Query params **todos obrigatórios**: `cid` (int), `et` (event type string), `hid` (string), `time` (timestamp `yyyyMMddHHmmssSSS` — 17 dígitos, sem separadores).
- Parse do `time`: implementar parse custom de 17 dígitos em America/Sao_Paulo (`DateUtils.DATE_FORMAT` legado; em Go o layout `20060102150405` + 3 dígitos de millis — ver nota de [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §5).
- **Variante `failureInfo`:** se o query param `failureInfo` está presente, logar o body JSON como `Tracking failure: {...}` (slog WARN com o map) e **seguir o fluxo normal** de tracking (paridade).
- Sucesso: publica `TrackingEvent` no SQS e responde **`201 Created`** com header **`Location: /adtrack/{uuid}`** (UUID v4 gerado no handler).
- Erros: `cid`/`et`/`hid`/`time` ausentes ou `time` não parseável → **`500`** (paridade exata com o `ParseException → 500` do legado; NÃO "melhorar" para 400).

### 2. `GET /vasttrack` (portado de `VastTrackService.java`)

- Query params: `cid` (obrigatório int), `et` (obrigatório), `hid` (opcional, default `""`), `time` (obrigatório, mesmo formato).
- Eventos típicos: `PAGE_VIEW`, `VIDEO_STARTED`, `25_PER_PLAYED`, `50_PER_PLAYED`, `75_PER_PLAYED`, `VIDEO_ENDED`.
- Sucesso: publica o mesmo `TrackingEvent` no SQS e responde **`200 OK` com corpo vazio**.
- Erro de parse/validação → **`500`**, com log detalhado contendo `cid`, `hid`, `et`, `time` (paridade com o log do legado).

### 3. Formato da mensagem SQS (contrato com M3-04 — documentar no código)

```go
// TrackingEvent é o contrato JSON publicado na tracking-queue.
// O timestamp é o do REQUEST ORIGINAL (param time), nunca o do processamento (ADR-003).
type TrackingEvent struct {
    EventID    string `json:"event_id"`    // UUID v4 — o mesmo devolvido no Location
    CampaignID int    `json:"campaign_id"`
    HotspotID  string `json:"hotspot_id"`  // vazio = sem hotspot (pixel usa "")
    EventType  string `json:"event_type"`  // valor string persistido (ex. "25_PER_PLAYED")
    EventTime  string `json:"event_time"`  // param time original, formato yyyyMMddHHmmssSSS
    Source     string `json:"source"`      // "adtrack" | "vasttrack" | "trackingpixel"
}
```

- Publisher em `internal/tracking/publisher.go` (SQS `SendMessage`, fila da issue M2-01 via env var `TRACKING_QUEUE_URL`).
- Falha no `SendMessage` → `500` (o evento NÃO pode ser perdido silenciosamente).

### 4. ⚠️ ATENÇÃO de paridade — header Location

O Java retornava `Location: /adtrack/{id}` com o **ID do MySQL** (auto increment). Com a escrita assíncrona o ID não existe no momento da resposta — passamos a retornar **UUID**. Análise registrada: nenhum template/JS do legado lê o `Location` (XHR fire-and-forget), mas a confirmação formal de que NENHUM consumidor externo usa esse valor é a issue **M9-01 (verificação de contratos externos)** — referenciar este risco no PR e na MATRIZ-PARIDADE como "paridade parcial documentada".

### 5. Estrutura

- `cmd/track/main.go` — roteia `POST /adtrack`, `GET /vasttrack` (e futuramente `GET /trackingpixel`, M3-05) via router mínimo (ADR-001), com middlewares CORS/validação/recover (M1-07).
- `internal/tracking/handler.go` — validações + montagem do evento; `internal/tracking/publisher.go` — SQS.
- Logs slog JSON com campos `service`, `route`, `cid`, `hid`.

## Arquivos a criar/alterar

- `cmd/track/main.go`
- `internal/tracking/handler.go`, `internal/tracking/publisher.go`, `internal/tracking/event.go` (struct `TrackingEvent`)
- `internal/tracking/*_test.go`
- `serverless.yml` (function `track-handler`: rotas POST /adtrack, GET /vasttrack; 256MB; timeout 10s; permissão `sqs:SendMessage`)
- `docs/MATRIZ-PARIDADE.md`

## Critérios de aceite

- [ ] `POST /adtrack` válido → `201` + `Location: /adtrack/{uuid}` e mensagem no SQS com o `time` ORIGINAL do request
- [ ] `GET /vasttrack` válido → `200` corpo vazio + mensagem no SQS
- [ ] Param obrigatório ausente ou `time` inválido → `500` (paridade; testes cobrindo cada param)
- [ ] Variante `failureInfo`: body logado como `Tracking failure: {...}` e fluxo de tracking continua (teste)
- [ ] Parse de `yyyyMMddHHmmssSSS` (17 dígitos) correto, em America/Sao_Paulo, com teste de roundtrip
- [ ] Struct `TrackingEvent` documentada como contrato (godoc em português) e estável para o M3-04
- [ ] Falha de publish no SQS → `500` (teste com publisher mockado)
- [ ] Comentário no código + PR referenciando M9-01 sobre a mudança ID→UUID no Location
- [ ] `// Portado de: AdTrackService.java` / `VastTrackService.java`; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M2-01 (SQS tracking-queue), M1-06 (validação de eventos), M1-07 (middleware)

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §5, §9
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §5 (conversões de data)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (decisão tracking assíncrono), ADR-003
- Java: `ad-server/src/main/java/br/com/intv/adserver/presentation/service/AdTrackService.java` e `VastTrackService.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-03] track-handler no repo InteliFi/ad-serverless:
cmd/track/main.go com POST /adtrack (params obrigatórios cid/et/hid/time, time no
formato yyyyMMddHHmmssSSS de 17 dígitos, variante failureInfo logada como
"Tracking failure" seguindo o fluxo, resposta 201 com Location: /adtrack/{uuid},
erros de parse → 500) e GET /vasttrack (200 vazio). Publicar TrackingEvent JSON no
SQS (internal/tracking) com o timestamp ORIGINAL do request. Documentar a mudança
Location ID→UUID referenciando M9-01. Testes unitários de todas as validações.
Código em português, // Portado de:, atualizar MATRIZ-PARIDADE e serverless.yml,
abrir PR.
```
