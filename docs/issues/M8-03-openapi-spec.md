---
title: "[M8-03] Especificação OpenAPI 3 dos 16 endpoints"
labels: ["epic:M8-qualidade", "tipo:docs", "prioridade:P1"]
milestone: "M8 — Qualidade"
---
## Contexto

O contrato HTTP do ad server existe hoje apenas como prosa em [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) e como código Java. Esta issue formaliza o contrato em **OpenAPI 3** (`docs/api/openapi.yaml`), cobrindo os **16 endpoints** da tabela "Visão Geral" do docs/legado/01, com TODOS os parâmetros, códigos de status, content-types e headers de resposta documentados. A spec serve para: (1) verificação de contrato no cutover (M9-01), (2) referência para consumidores externos (players, afiliados), (3) lint automático no CI impedindo que a documentação apodreça.

Regra de ouro: a spec descreve o **comportamento de paridade** (o que o legado faz e o Go replica). Onde a arquitetura serverless muda o transporte (ex.: `/media` pode responder `302` para CloudFront; `Location` do POST /adtrack passa a UUID), documentar AMBOS com `description` explicando a transição. **Não inventar nada**: cada parâmetro/código sai do docs/legado/01 ou do código Java de referência (`c:\Users\Fabio\Documents\Dev\ad-server`, pacote `presentation/`).

## Especificação detalhada

### 1. Estrutura do arquivo

- `docs/api/openapi.yaml`, OpenAPI **3.0.3** (compatibilidade ampla de ferramentas), `info` com versão do projeto e descrição em português, `servers`: `https://ads.inteli.fi` (prod) e a URL do API Gateway dev (placeholder `{api-id}`).
- `tags` por Lambda alvo: `ad`, `vast`, `tracking`, `redirect`, `postback`, `proxy`, `media`, `report`, `health` — espelhando a coluna "Lambda Go alvo" da tabela do docs/legado/01.
- `components/parameters` para parâmetros repetidos (`cid`, `hid`, `time`, `refOrigin`, `u`, `gdpr`...), `components/responses` para erros comuns (400 validação, 404, 500, 502) e `components/headers` para os headers CORS (reflexão de Origin + credentials — docs/legado/01 §16).
- Todas as `description` em **português**, citando a seção da spec legada (ex.: `"Ver docs/legado/01-endpoints-http.md §11"`).

### 2. Inventário obrigatório (16 endpoints — nenhum pode faltar)

| # | Path/Métodos | Pontos de atenção a documentar |
|---|---|---|
| 1 | `GET/HEAD /`, `/health`, `/healthz` | `200` `application/json` corpo `{"version":"...","status":"UP"}`; sem acesso a banco |
| 2 | `GET /ad` | `hid` (suporta múltiplos valores separados por `\|`, usa o 1º), `red`; `200` **`text/javascript`** + headers `Content-Description: File Transfer` e `Content-Disposition: attachment; filename="adscript.js"`; `404` (hid vazio/inexistente/script vazio) |
| 3 | `GET /GAM` | `hid` obrigatório (nome do HTML), `red`; `200` `text/html`; `404` (hid vazio, fetch falho, resposta vazia) |
| 4 | `GET /vast` | `cid` (int), `hid`, `vcurl` (URL base64), `gdpr` (default `"0"`), `gdpr_consent` (default `""`), `refOrigin` (default `https://ads.inteli.fi`); `200` **`text/xml`** (VAST 4.2), `404`, `400` (refOrigin inválido), `500/502/504` (upstream) |
| 5 | `POST /adtrack` | `cid` (int), `et`, `hid`, `time` (**`yyyyMMddHHmmssSSS`**, 17 dígitos) — todos obrigatórios; variante com param `failureInfo` + body JSON (Map) logado; `201` + header **`Location: /adtrack/{id}`**; `500` (parse) |
| 6 | `GET /adtrack` | relatório JSON agregado `AdTrackerReport{items:[...]}`; `200` `application/json` |
| 7 | `GET /adtrack/xls` | mesmo relatório em Excel; documentar o content-type EXATO observado no `AdTrackService.java` (POI/HSSF legado) e header de download |
| 8 | `GET /adtrack/postback` | obrigatórios: `cid` (int), `event`, `aff_sub`, `click_id`, `source`; opcionais: `transaction_id`, `payout`, `currency`, `sale_amount`; `202` | `401` | `404` (campanha) | `422` |
| 9 | `GET /vasttrack` | `cid` (int, obrig.), `et` (obrig.), `hid` (opc.), `time` (obrig., 17 dígitos); eventos típicos como `enum` documentado (`PAGE_VIEW`, `VIDEO_STARTED`, `25_PER_PLAYED`, `50_PER_PLAYED`, `75_PER_PLAYED`, `VIDEO_ENDED`); `200` corpo vazio | `500` |
| 10 | `GET /trackingpixel` | `cid` (int, obrig.; ausente → `400`); `200` **`image/png`** + `Content-Disposition: attachment; filename="tracking_pixel_{cid}.png"`; `500` (IO/parse) |
| 11 | `GET /redirect` | `url` (obrig., texto plano ou base64), `cid` (int, obrig.), `hid` (obrig.), `enc` (bool, default false), `source`, `aff_sub`, `click_id` (substituem `{source}`/`{aff_sub}`/`{click_id}`); `200` `text/html`; `400` com corpos literais `"Invalid URL format"` e `"Invalid URL encoding"`; `500` |
| 12 | `GET /proxy-tracker` + **`OPTIONS /proxy-tracker`** | `u` (obrig., base64 OU url-encoded), `refOrigin`; OPTIONS → `204` com `Access-Control-Allow-Methods: GET, POST, OPTIONS`, `Access-Control-Expose-Headers: Content-Type, Cache-Control, Expires`, `Cross-Origin-Embedder-Policy: credentialless`; GET propaga status/corpo/headers do upstream (documentar como resposta genérica); se URL contém `type=js` força `Content-Type: application/javascript`; `400` | `502 "Proxy error"` |
| 13 | `GET /proxy-audit` | `src` (obrig., base64 — aceita URL-safe e sem padding), `refOrigin`; `200` **`text/javascript; charset=utf-8`** + `Cache-Control: public, max-age=3600` + `Vary: Accept-Encoding`; erros de validação → `400` com comentário JS no corpo |
| 14 | `GET /safeframe/proxy-safeframe` + **`OPTIONS /safeframe`** | `u` (base64 ou url-encoded; rejeita `file://`); `200` `text/html` com CORS refletindo origin + `Vary: Origin`; copia `Cache-Control`, `Expires`, `Last-Modified`, `Etag` do upstream; `400` | `502` |
| 15 | `GET /media/{filename}` | path param `filename` (vídeo mp4); `200` **`video/mp4`** | `404` | `500`; na arquitetura serverless adicionar `302` (redirect para CloudFront) com `description` explicando |
| 16 | `/error/400` | resposta de erro padrão (vira API Gateway responses na migração — documentar a resposta `400` default) |

### 3. Detalhes transversais

- **Content-types não triviais são obrigatórios na spec:** `text/xml` (vast), `text/javascript` (ad, proxy-audit), `text/html` (GAM, redirect, safeframe), `image/png` (trackingpixel), `video/mp4` (media), `application/json` (health, relatório), Excel (xls).
- **OPTIONS:** além dos preflights próprios de `/proxy-tracker` e `/safeframe` (item 12/14), documentar em `description` global que o middleware CORS responde OPTIONS `200` imediato nas demais rotas (docs/legado/01 §16 — CorsFilter).
- **Headers CORS** como `components/headers` reutilizados: reflexão de `Origin` + `Access-Control-Allow-Credentials: true` + `Vary: Origin` quando Origin presente; `*` sem credentials caso contrário.
- **Formato `time`:** definir `components/schemas/TrackingTime` (string, `pattern: '^\d{17}$'`, exemplo `20260611143000123`, descrição citando `yyyyMMddHHmmssSSS` / `DateUtils.DATE_FORMAT`).
- Validar parâmetros/códigos contra as classes Java em caso de dúvida: `ApplicationStatusService.java`, `AdService.java`, `GamService.java`, `VastService.java`, `AdTrackService.java`, `VastTrackService.java`, `TrackingPixelService.java`, `RedirectService.java`, `ProxyTrackerService.java`, `ProxyAuditController.java`, `SafeFrameService.java`, `MediaController.java`, `ErrorController.java` (em `c:\Users\Fabio\Documents\Dev\ad-server\src\main\java\br\com\intv\adserver\presentation\`).

### 4. Lint no CI

- Job `openapi-lint` no `.github/workflows/ci.yml`: `npx @redocly/cli@<versão fixada> lint docs/api/openapi.yaml` — falha bloqueia merge.
- Configuração `docs/api/redocly.yaml` com ruleset `recommended`; regras desabilitadas documentadas com justificativa em comentário (ex.: `operation-4xx-response` se algum endpoint legitimamente não tiver 4xx).
- O job roda apenas quando `docs/api/**` muda (path filter) + sempre no `main`.

## Arquivos a criar/alterar

- `docs/api/openapi.yaml` — a especificação completa (16 endpoints)
- `docs/api/redocly.yaml` — configuração do lint
- `docs/api/README.md` — como visualizar (ex.: `npx @redocly/cli preview-docs`), como manter sincronizada (regra: PR que muda contrato DEVE atualizar a spec)
- `.github/workflows/ci.yml` — job `openapi-lint`

## Critérios de aceite

- [ ] `docs/api/openapi.yaml` válido (OpenAPI 3.0.3) cobrindo os 16 endpoints da tabela do docs/legado/01 — nenhum path/método ausente (incluindo HEAD no health e os dois OPTIONS de proxy-tracker/safeframe)
- [ ] Todos os parâmetros com tipo, obrigatoriedade, default e descrição em português extraídos do docs/legado/01 (sem invenções)
- [ ] Todos os códigos de status documentados por endpoint (incluindo 201+Location, 202/401/404/422 do postback, 400 com corpos literais do redirect, 502/504)
- [ ] Content-types exatos: text/xml, text/javascript, text/html, image/png, video/mp4, application/json e o content-type do XLS verificado no `AdTrackService.java`
- [ ] Headers de resposta documentados: Content-Disposition (ad, trackingpixel), Location (adtrack), Cache-Control/Vary (proxy-audit), headers CORS como components reutilizados
- [ ] Schema `TrackingTime` com pattern `^\d{17}$` referenciado por /adtrack e /vasttrack
- [ ] `npx @redocly/cli lint` passa sem erros; job `openapi-lint` no CI verde (evidência: run no Actions)
- [ ] `docs/api/README.md` define a regra de manutenção da spec em PRs futuros

## Dependências

Bloqueada por: nenhuma (a fonte é a documentação legada + código Java de referência; pode ser executada em paralelo a qualquer epic)

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) — tabela "Visão Geral" + §1–§16 (fonte da verdade de params/códigos/content-types)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3 (chars relaxados — citar como nota na spec)
- [docs/MATRIZ-PARIDADE.md](../MATRIZ-PARIDADE.md) (linhas E01–E16)
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (mapa rota → Lambda, decisão do Location UUID)
- Código Java de referência: `c:\Users\Fabio\Documents\Dev\ad-server\src\main\java\br\com\intv\adserver\presentation\` (services, controllers e filtros)
- [OpenAPI 3.0.3](https://spec.openapis.org/oas/v3.0.3) · [Redocly CLI lint](https://redocly.com/docs/cli/commands/lint/)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M8-03] Especificação OpenAPI 3 dos 16 endpoints
seguindo docs/issues/M8-03-openapi-spec.md e CLAUDE.md. Criar
docs/api/openapi.yaml (OpenAPI 3.0.3) com os 16 endpoints da tabela de
docs/legado/01-endpoints-http.md — todos os parâmetros, códigos de status,
content-types (text/xml, text/javascript, text/html, image/png, video/mp4,
JSON, XLS) e headers (Location, Content-Disposition, CORS como components),
incluindo os OPTIONS de /proxy-tracker e /safeframe e o schema TrackingTime
^\d{17}$. Conferir detalhes no código Java em
c:\Users\Fabio\Documents\Dev\ad-server (pacote presentation/) sem inventar
nada. Adicionar docs/api/redocly.yaml, docs/api/README.md e job
openapi-lint (redocly) no ci.yml. Descrições em português, lint verde,
abrir PR referenciando a issue.
```
