---
title: "[M3-05] track-handler: GET /trackingpixel"
labels: ["epic:M3-tracking", "tipo:port", "prioridade:P1"]
milestone: "M3 — Tracking"
---
## Contexto

`GET /trackingpixel?cid={campaignId}` é o pixel de e-mail marketing/tracking do legado (`TrackingPixelService.java` + `TrackingPixelComponent` — [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §10 e [02-logica-negocio.md](../legado/02-logica-negocio.md) §1.7). O comportamento legado tem uma característica deliberadamente "estranha" que é **feature em produção** e NÃO pode ser otimizada (CLAUDE.md, "O que NUNCA fazer"): a imagem do pixel é **baixada da URL cadastrada A CADA request** (`FileUtils.copyURLToFile`), sem cache. Esta issue adiciona a rota ao `track-handler` (mesma Lambda do M3-03).

## Especificação detalhada

### 1. Fluxo (portado de `TrackingPixelService.java` / `TrackingPixelComponentImpl.java`)

1. **Validação do `cid`:** ausente ou não-inteiro → **`400 Bad Request`** (paridade com `ServiceUtils.buildBadRequest()`).
2. **Busca da URL do pixel:** `mysql.FindTrackingPixelURL(ctx, cid)` (M3-01) — `SELECT url FROM tracking_pixels WHERE campaign_id = ?`.
   - Pixel inexistente para a campanha → **`500`** (paridade: no legado o download de URL nula estourava IO/NPE e caía no catch `IOException → 500`; NÃO "melhorar" para 404).
3. **Download da imagem** da URL a cada request (paridade do `FileUtils.copyURLToFile`): em Lambda, baixar para memória (`[]byte`) via client HTTP compartilhado (`internal/httpx`, M1-08) com timeout default 60s. Falha de download (DNS, timeout, status != 2xx, corpo vazio) → **`500`**.
4. **Registro do evento `TRACKING_PIXEL`:** publicar `TrackingEvent` no SQS (publisher do M3-03) com:
   - `campaign_id` = cid; `event_type` = `"TRACKING_PIXEL"`;
   - `hotspot_id` = `""` com `source` = `"trackingpixel"` → o tracker-writer (M3-04) converte para **NULL** no MySQL e omite no DynamoDB (paridade: `hotspot = null` deliberado — 02 §1.7);
   - `event_time` = relógio injetado (America/Sao_Paulo) formatado `yyyyMMddHHmmssSSS` — o legado usava o instante do request;
   - `event_id` = UUID v4.
   - Falha no publish SQS → **`500`** (evento não pode ser perdido silenciosamente — mesma regra do M3-03).
5. **Resposta `200 OK`** com os bytes baixados e headers EXATOS do legado:
   - `Content-Type: image/png` (mesmo que a imagem de origem não seja PNG — paridade);
   - `Content-Disposition: attachment; filename="tracking_pixel_{cid}.png"` (filename ENTRE ASPAS, ex.: `attachment; filename="tracking_pixel_42.png"` — o Java monta `String.format("\"tracking_pixel_%s.png\"", campaignId)`);
   - `Content-Description: File Transfer`.
   - Corpo binário: em API Gateway HTTP API responder com `IsBase64Encoded: true` e corpo base64.

### 2. Ordem das operações — atenção de paridade

No legado, o `TrackingPixelComponent.getTrackingPixel` baixa a imagem E registra o evento ANTES de responder; se o download falha, o evento **não** é registrado (a exceção interrompe o fluxo). Preservar essa ordem: download → publish SQS → resposta. Testar que falha de download NÃO publica evento.

### 3. Estrutura

- Rota `GET /trackingpixel` adicionada ao router do `cmd/track/main.go` (M3-03), com middlewares CORS/validação/recover (M1-07).
- Lógica em `internal/tracking/pixel.go` com interfaces para repositório MySQL, downloader HTTP e publisher SQS (testabilidade).
- Logs slog JSON: `service=track-handler`, `route=/trackingpixel`, `cid`, e a URL do pixel em DEBUG.
- `serverless.yml`: adicionar rota `GET /trackingpixel` à function `track-handler` (256MB / 10s já definidos); IAM já cobre `sqs:SendMessage`; adicionar acesso de leitura MySQL via RDS Proxy (já coberto pela rede/secret da M3-01).

### 4. Testes

- `cid` ausente → 400; `cid` não numérico → 400.
- Pixel encontrado + download OK → 200 com os 3 headers exatos (incluindo aspas no filename) e corpo = bytes do mock; evento publicado com `event_type=TRACKING_PIXEL`, `hotspot_id=""`, `source=trackingpixel`.
- Pixel inexistente → 500 sem publicar evento.
- Download falha (status 500 do upstream / timeout simulado) → 500 sem publicar evento.
- Publish SQS falha → 500.
- Servidor HTTP de teste (`httptest`) como origem da imagem; nenhum acesso à rede real nos testes.

## Arquivos a criar/alterar

- `internal/tracking/pixel.go` (`// Portado de: TrackingPixelService.java / TrackingPixelComponentImpl.java`)
- `internal/tracking/pixel_test.go`
- `cmd/track/main.go` (registrar rota `GET /trackingpixel`)
- `serverless.yml` (rota adicional na function `track-handler`)
- `docs/MATRIZ-PARIDADE.md`

## Critérios de aceite

- [ ] `GET /trackingpixel?cid=42` com pixel cadastrado → 200, `Content-Type: image/png`, `Content-Disposition: attachment; filename="tracking_pixel_42.png"` (com aspas), `Content-Description: File Transfer`, corpo = imagem baixada
- [ ] A imagem é baixada da URL **a cada request** — NENHUM cache de imagem (comentário no código explicando que é paridade deliberada)
- [ ] Evento `TRACKING_PIXEL` publicado no SQS com hotspot vazio (`source=trackingpixel` → NULL no writer) e timestamp do request
- [ ] `cid` ausente/inválido → 400; pixel inexistente ou falha de download/SQS → 500 (testes cobrem cada caso)
- [ ] Falha de download NÃO publica evento (ordem do legado preservada, teste cobre)
- [ ] Resposta binária correta via API Gateway (IsBase64Encoded)
- [ ] `// Portado de:` presente; godoc em português; `make lint && make test` verdes; MATRIZ-PARIDADE atualizada

## Dependências

Bloqueada por: M3-01 (FindTrackingPixelURL), M3-03 (track-handler + publisher SQS).

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §10
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.7
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (track-handler)
- Java: `ad-server/.../presentation/service/TrackingPixelService.java`, `business/component/impl/TrackingPixelComponentImpl.java`

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M3-05] GET /trackingpixel no repo InteliFi/ad-serverless
seguindo docs/issues/M3-05-track-handler-trackingpixel.md e CLAUDE.md: rota no
track-handler que busca tracking_pixels.url por cid (M3-01), BAIXA a imagem da URL
a cada request (comportamento legado, sem cache), publica evento TRACKING_PIXEL com
hotspot vazio no SQS e devolve image/png com Content-Disposition attachment
filename="tracking_pixel_{cid}.png" (com aspas) e Content-Description File Transfer.
cid nulo→400, pixel inexistente/falha IO→500, falha de download não publica evento.
Código comentado em português. Testes verdes. Abrir PR ao final.
```
