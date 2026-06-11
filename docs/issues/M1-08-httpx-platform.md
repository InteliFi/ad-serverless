---
title: "[M1-08] internal/httpx + internal/platform: HTTP client, IP real/geo, config SSM, slog e response helpers"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P2"]
milestone: "M1 — Commons Go"
---
## Contexto

Vários handlers fazem fetch de upstreams (VAST partners, GAM, proxies, postbacks modatta/prezão) e TODOS precisam de: cliente HTTP com timeouts corretos, headers upstream padronizados (User-Agent fixo, IP real do cliente, geo headers), configuração via env+SSM e logging estruturado. Esta issue cria a fundação transversal em `internal/httpx` (HTTP de saída + extração de IP/geo) e `internal/platform` (config, slog, helpers de resposta API Gateway v2). Sem ela, cada handler reinventaria a roda — e divergiria do legado, que centraliza isso no `RestTemplate` configurado e nos utilitários de request.

## Especificação detalhada

### 1. `internal/httpx` — cliente HTTP compartilhado

```go
// NovoCliente cria um http.Client com timeouts explícitos e keep-alive,
// para ser criado 1× por container (var de pacote no main) e reusado.
// Portado de: configuração do RestTemplate (connect/read 60s globais).
func NovoCliente(timeoutTotal time.Duration) *http.Client

// NovoClienteProxyAudit cria o cliente específico do proxy-audit:
// connect 10s, read 30s (PROXY_AUDIT_CONNECT_TIMEOUT/READ_TIMEOUT do legado).
func NovoClienteProxyAudit() *http.Client
```

- Default global: **60s** (paridade com o RestTemplate); proxy-audit: **10s connect / 30s read** via `net.Dialer{Timeout}` no `Transport` + `Client.Timeout` total.
- `Transport` com keep-alive habilitado e `MaxIdleConnsPerHost` razoável (containers Lambda reusam conexões entre invocações — documentar).
- NUNCA usar `http.DefaultClient` (sem timeout) — proibir via comentário no godoc e teste de lint manual.

### 2. `internal/httpx` — extração de IP real e geo headers

```go
// IPRealDoCliente extrai o IP verdadeiro do cliente atrás de CDNs/proxies.
// Ordem de precedência (paridade com ProxyTrackerService.java):
// 1. X-Forwarded-For (primeiro IP da lista, antes da primeira vírgula);
// 2. CF-Connecting-IP (Cloudflare);
// 3. True-Client-IP (Akamai/Cloudflare Enterprise);
// 4. fallback: sourceIP do requestContext (API Gateway).
func IPRealDoCliente(req events.APIGatewayV2HTTPRequest) string

// HeadersGeo retorna os headers de geolocalização presentes no request,
// preservando nome e valor originais para repasse ao upstream.
// Lista mínima: CF-IPCountry, CloudFront-Viewer-Country,
// CloudFront-Viewer-Country-Region, CloudFront-Viewer-City
// (conferir a lista EXATA em ProxyTrackerService.java/VastService.java do legado).
func HeadersGeo(req events.APIGatewayV2HTTPRequest) map[string]string
```

- Headers do payload v2 chegam em **minúsculas** no map `req.Headers` — a busca deve ser case-insensitive e o repasse deve usar a grafia canônica (ex.: `CF-IPCountry`).
- `X-Forwarded-For` com múltiplos IPs (`"203.0.113.7, 70.41.3.18, 150.172.238.178"`) → usar o **primeiro** (`203.0.113.7`), com trim de espaços.

### 3. `internal/httpx` — builder de headers upstream

```go
// HeadersUpstream monta os headers padrão para fetch de upstreams
// (VAST partners, proxy-tracker), copiando o que o legado copia.
// Portado de: ProxyTrackerService.java + VastService.java (fetch dinâmico).
func HeadersUpstream(req events.APIGatewayV2HTTPRequest, refOrigin string) http.Header
```

Conteúdo (paridade com docs/legado/01 §12 e docs/legado/03 §2):
- Copia do cliente quando presentes: `Accept`, `Accept-Encoding`, `Accept-Language`.
- `User-Agent` **fixo** (copiar a string EXATA do Java, padrão `Mozilla/5.0 ... Chrome/91...` — conferir em `ProxyTrackerService.java`).
- `X-Forwarded-For` e `X-Real-IP` = `IPRealDoCliente(req)`.
- `X-Forwarded-Proto/Host/Port` e `Forwarded: for=IP;proto=...;host=...` (usados no fetch de VAST — docs/legado/03 §2 passo 4).
- Headers de geolocalização (`HeadersGeo`) repassados.
- `Origin: {refOrigin}` e `Referer: {refOrigin}/` (barra final no Referer — paridade).

### 4. `internal/platform/config` — loader env + SSM com cache

```go
// Config concentra toda a configuração do serviço, carregada 1× por container.
type Config struct {
    Stage               string // dev | prod (env STAGE)
    MySQLDSN            string // SSM SecureString /ad-serverless/{stage}/mysql-dsn
    SignatureKey        string // SSM SecureString /ad-serverless/{stage}/signature-key
    PostbackHotspotCode string // env POSTBACK_HOTSPOT_CODE (legado intv.ad.postback.hotspot.code)
    // ... demais chaves adicionadas pelas issues que as consomem
}

// Carrega lê env vars e parâmetros SSM (GetParameters com WithDecryption),
// cacheando o resultado em variável de pacote — chamadas subsequentes na
// mesma instância do container NÃO vão ao SSM (custo e latência).
func Carrega(ctx context.Context) (*Config, error)
```

- Env vars simples vêm do `serverless.yml`; segredos SEMPRE de SSM SecureString (regra 7 do CLAUDE.md — nunca copiar credenciais do repo Java, estão comprometidas).
- Cache de container: `sync.Once` ou checagem de ponteiro; expor `CarregaParaTeste(overrides)` para os testes não tocarem SSM (interface `ssmClient` mínima mockável).
- Falha de SSM em parâmetro obrigatório → erro fatal no cold start (fail fast), com mensagem clara SEM ecoar valores.

### 5. `internal/platform/log` — slog JSON padrão

```go
// Novo cria o slog.Logger JSON padrão do projeto, escrevendo em stdout
// (CloudWatch Logs) com os campos fixos service e stage.
func Novo(service, stage string) *slog.Logger
```

- Handler `slog.NewJSONHandler(os.Stdout, ...)`; nível via env `LOG_LEVEL` (default INFO; legado roda ERROR global — documentar a diferença como decisão operacional, não de paridade de negócio).
- Convenção de campos (CLAUDE.md): `service`, `route`, `cid`, `hid` quando aplicável — helpers `ComRota(l, route)`, `ComCampanha(l, cid, hid)` retornando `*slog.Logger` com os atributos.

### 6. `internal/platform/resp` — helpers de resposta APIGatewayV2

```go
// JSON monta resposta application/json;charset=UTF-8 com status e corpo serializado.
func JSON(status int, corpo any) (events.APIGatewayV2HTTPResponse, error)

// Texto monta resposta text/plain (mensagens de erro curtas do legado, ex.: 400).
func Texto(status int, corpo string) (events.APIGatewayV2HTTPResponse, error)

// XML, JS, HTML — content types text/xml, text/javascript, text/html (charset UTF-8).
// Binario monta resposta com isBase64Encoded=true (XLS do M6-02, pixel do M3-05).
func Binario(status int, contentType string, dados []byte, headersExtras map[string]string) (events.APIGatewayV2HTTPResponse, error)

// Erro padroniza erros HTTP sem vazar detalhes internos (mensagem fixa + log).
func Erro(status int, mensagem string) (events.APIGatewayV2HTTPResponse, error)
```

- Charset UTF-8 explícito em todos os content types textuais (paridade com o force-UTF-8 do CorsFilter).
- `Binario` é pré-requisito do tracking pixel (`image/png` + Content-Disposition) e do XLS (M6-02) — incluir suporte a headers extras (`Content-Disposition`, `Content-Description`).

### Testes obrigatórios

1. `IPRealDoCliente`: tabela com X-Forwarded-For simples, X-Forwarded-For com lista, CF-Connecting-IP, True-Client-IP, nenhum header (fallback sourceIP), precedência entre eles.
2. `HeadersUpstream`: request com Accept/Accept-Language → copiados; sem eles → ausentes; User-Agent SEMPRE o fixo; Origin/Referer derivados de refOrigin (com a barra no Referer).
3. `HeadersGeo`: headers em minúsculas no map de entrada → repassados com grafia canônica.
4. Clientes HTTP: timeouts configurados (inspecionar `Client.Timeout`/`Transport`); proxy-audit ≠ default.
5. Config: loader com SSM mockado; segunda chamada não invoca o mock (cache); parâmetro obrigatório ausente → erro.
6. Resp: golden dos headers/content types; `Binario` com `isBase64Encoded=true` e round-trip base64.

## Arquivos a criar/alterar

- `internal/httpx/doc.go`, `client.go`, `ip.go` (`// Portado de: ProxyTrackerService.java (extração de IP real)`), `headers.go`.
- `internal/platform/config/config.go` + `config_test.go`.
- `internal/platform/log/log.go` + `log_test.go`.
- `internal/platform/resp/resp.go` + `resp_test.go`.
- `internal/httpx/client_test.go`, `ip_test.go`, `headers_test.go`.

## Critérios de aceite

- [ ] `NovoCliente` (60s) e `NovoClienteProxyAudit` (10s connect/30s read) com timeouts explícitos e keep-alive; nenhum uso de `http.DefaultClient` no repo.
- [ ] `IPRealDoCliente` com a precedência X-Forwarded-For (primeiro IP) → CF-Connecting-IP → True-Client-IP → sourceIP, busca case-insensitive, testada por tabela.
- [ ] `HeadersUpstream` reproduz o conjunto do legado (cópia seletiva, UA fixo EXATO do Java, X-Forwarded-For/X-Real-IP, Forwarded, geo, Origin/Referer com `/`).
- [ ] `HeadersGeo` cobre no mínimo CF-IPCountry e CloudFront-Viewer-Country, com a lista final conferida no código Java (citar arquivo/linha no comentário).
- [ ] Config: env + SSM SecureString com cache por container; zero segredos hardcoded; mock de SSM nos testes.
- [ ] slog JSON com campos padrão `service`/`stage` e helpers de `route`/`cid`/`hid`.
- [ ] Helpers de resposta com charset UTF-8 e suporte binário base64 (headers extras incluídos).
- [ ] Código 100% comentado em português com `// Portado de: ...`; `make lint && make test` verdes.

## Dependências

Bloqueada por: M0-01 (bootstrap do repositório Go).
Bloqueia: M3-01 (DSN via config), M3-06, M4-04, M5-01 (headers upstream), M6-01, M7-01 (padrão de logging).

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §12 (headers upstream do proxy-tracker, IP real, geo).
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2 passo 4 (headers do fetch de VAST: X-Forwarded-For, X-Real-IP, Forwarded, geo).
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2 (segredos → SSM), §3 (timeouts do RestTemplate e do proxy-audit, logging).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (segredos), §4 (`internal/httpx` e `internal/platform`), ADR-001.
- Código Java de referência: `c:\Users\Fabio\Documents\Dev\ad-server` (`ProxyTrackerService.java`, `VastService.java`, `application*.properties`).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-08 (internal/httpx + internal/platform) do ad-serverless: (1) http.Client compartilhado com timeout 60s e variante proxy-audit 10s connect/30s read, keep-alive, sem http.DefaultClient; (2) IPRealDoCliente com precedência X-Forwarded-For (primeiro IP)/CF-Connecting-IP/True-Client-IP/sourceIP e HeadersGeo (CF-IPCountry, CloudFront-Viewer-Country... conferir lista exata no Java em c:\Users\Fabio\Documents\Dev\ad-server); (3) HeadersUpstream com cópia de Accept/Accept-Encoding/Accept-Language, User-Agent fixo EXATO do legado, Forwarded/X-Real-IP, Origin=refOrigin e Referer=refOrigin+"/"; (4) config loader env+SSM SecureString com cache por container e mock nos testes; (5) slog JSON com campos service/route/cid/hid; (6) helpers de resposta APIGatewayV2 (JSON/Texto/XML/JS/HTML/Binario base64/Erro) com UTF-8. Seguir docs/issues/M1-08-httpx-platform.md, código 100% comentado em português com "// Portado de: ...", make lint && make test verdes, abrir PR feat/issue-M1-08-httpx-platform com Closes na issue.
```
