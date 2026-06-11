---
title: "[M1-07] internal/middleware: CORS + RequestValidation + recover como middleware encadeável"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P2"]
milestone: "M1 — Commons Go"
---
## Contexto

O legado aplica DOIS filtros servlet a TODAS as requisições: `CorsFilter` (reflexão de Origin com credentials) e `RequestValidationFilter` (anti-injection: PHP wrappers, SQL injection, command injection, params perigosos). Na arquitetura alvo, esses filtros viram **middleware Go encadeável** em `internal/middleware`, reutilizado por todos os 8 handlers HTTP (ad, vast, track, redirect, postback, proxy, media, report). O CORS do API Gateway NÃO substitui o `CorsFilter`: a reflexão de origin com `Access-Control-Allow-Credentials: true` exige lógica na Lambda (nota 1 do docs/legado/01). Inclui também o middleware `Recover` exigido pelo CLAUDE.md ("nunca panic em handler").

⚠️ Paridade: as listas de padrões de ataque, params perigosos e bypasses abaixo vêm do `RequestValidationFilter.java` — ao implementar, **conferir as listas completas no código Java** (`c:\Users\Fabio\Documents\Dev\ad-server`, pacote `presentation/filter/`) e copiá-las integralmente (as specs trazem exemplos representativos com "...").

## Especificação detalhada

### Tipo de middleware (contrato comum a todos os handlers)

```go
// Handler é a assinatura padrão de um handler Lambda HTTP (payload v2).
type Handler func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

// Middleware envolve um Handler adicionando comportamento antes/depois.
type Middleware func(Handler) Handler

// Encadeia aplica os middlewares na ordem dada (o primeiro é o mais externo).
func Encadeia(h Handler, ms ...Middleware) Handler
```

Ordem canônica de encadeamento (documentar no godoc): `Recover` → `CORS` → `RequestValidation` → handler. Recover é o mais externo para capturar panics inclusive dos outros middlewares.

### Middleware `CORS` — port de `CorsFilter.java`

| Condição | Headers de resposta |
|---|---|
| Request COM header `Origin` | `Access-Control-Allow-Origin: <origin refletido>`, `Access-Control-Allow-Credentials: true`, `Vary: Origin` |
| Request SEM `Origin` | `Access-Control-Allow-Origin: *` e SEM credentials (a combinação `*` + credentials é inválida na spec CORS — o legado também não a produz) |

Sempre (ambos os casos):
- `Access-Control-Allow-Methods: GET, POST, OPTIONS, PUT, DELETE, HEAD`
- `Access-Control-Allow-Headers: Content-Type, Authorization, X-Requested-With, Accept, Origin`
- `Access-Control-Max-Age: 3600`
- **OPTIONS → responde `200` imediato** sem chamar o handler interno (preflight curto-circuitado).
- Força charset UTF-8 no `Content-Type` da resposta quando aplicável (paridade com `setCharacterEncoding("UTF-8")`).
- Loga o request completo em nível DEBUG (slog).

Nota: `/proxy-tracker` e `/safeframe` têm respostas OPTIONS PRÓPRIAS mais específicas (204, headers extras) — esses handlers (M5-08/M5-09) substituem o curto-circuito; o middleware deve permitir opt-out do tratamento de OPTIONS (ex.: `CORSComOpcoes(TratarPreflight bool)`).

### Middleware `RequestValidation` — port de `RequestValidationFilter.java` (TODAS as regras)

Qualquer violação → `400 Bad Request` com corpo curto de texto (copiar as mensagens exatas do Java, ex.: `"Invalid HTTP method"`).

1. **Método HTTP:** permitidos `GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH, TRACE`; qualquer outro → `400 "Invalid HTTP method"`. Header `X-Invalid-Method` presente → `400` (gancho usado pelos testes do legado — manter).
2. **Path — sequências encodadas proibidas** (case-insensitive): `%7C %3C %3E %5C %5E %60 %7B %7D %00 %0D %0A %25 %09` (pipe, `<`, `>`, `\`, `^`, backtick, `{`, `}`, NUL, CR, LF, `%`, TAB).
3. **Path — charset permitido** após decode: apenas `[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=\-]*`; rejeita caracteres não-imprimíveis (controle 0x00–0x1F, 0x7F).
4. **Padrões de ataque PHP** (path e valores): `php://`, `allow_url_include`, `data:`, `file:`, `glob:` (e demais wrappers listados no Java).
5. **Padrões de SQL injection:** `UNION SELECT`, `INSERT INTO`, `DELETE FROM`, `DROP TABLE`, `UPDATE ... SET` etc. (case-insensitive; lista completa no Java).
6. **Padrões de command injection:** `; rm`, `&& bash`, pipes para shell, backticks, `$(...)`, `/dev/tcp`, `wget `, `curl ` etc. (lista completa no Java).
7. **Query string:** mesmas validações do path, **com bypass total para o path `/proxy-tracker`** (a query carrega base64 bruto que colidiria com as regras).
8. **Nomes de parâmetro perigosos** (rejeitar se presentes): `cmd, command, exec, execute, run, script, shell, system, opt, mdb, mdc, sys`.
9. **Valores de parâmetro:** validados contra os padrões 4–6; **bypass para o param `u` quando o path é `/proxy-tracker`** (somente checagem de não-imprimíveis nesse caso).
10. **Heurística base64:** valor com `len > 20` casando `^[A-Za-z0-9+/]*={0,2}$` é ISENTO dos padrões de ataque (4–6) — evita falso positivo em payloads base64 legítimos (VAST/tracking).
11. **Combinações conhecidas de ataque** (rejeitar mesmo isoladamente válidas): `opt=sys`, `cmd=___S_O_S_T_R_E_A_MAX___`, `mdb=sos` (lista completa no Java).
12. **`Content-Type` > 1000 caracteres** → `400`.

Implementação: regexes pré-compiladas em `var` de pacote (compilação 1× por container — hot path!); funções pequenas e testáveis por regra (`pathValido`, `queryValida`, `paramPerigoso`, `valorComAtaque`, `pareceBase64`).

### Middleware `Recover`

- Captura `panic` do handler/middlewares internos, loga stack trace via slog (ERROR, campos `service`, `route`) e responde `500` com corpo genérico `{"message":"Internal server error"}` — NUNCA vaza stack/erro interno na resposta.
- Regra do CLAUDE.md: "nunca panic em handler (middleware recover)".

### Tabela de casos de teste OBRIGATÓRIOS

| Caso | Entrada | Esperado |
|---|---|---|
| CORS com Origin | `Origin: https://player.example.com` | origin refletido + credentials true + `Vary: Origin` |
| CORS sem Origin | sem header | `*`, sem credentials |
| Preflight | `OPTIONS /ad` | `200` imediato, handler interno NÃO chamado |
| Método inválido | `FOO /ad` | `400 "Invalid HTTP method"` |
| Encodado proibido | path com `%3C` | `400` |
| SQL injection | `?q=1 UNION SELECT password` | `400` |
| Command injection | `?x=;rm -rf /` e `` ?x=`id` `` | `400` |
| PHP wrapper | `?f=php://input` | `400` |
| Param perigoso | `?cmd=ls` | `400` |
| Combinação | `?opt=sys` | `400` |
| Bypass proxy-tracker | `/proxy-tracker?u=<base64 com '/' e '+'>` | `200` (passa) |
| Heurística base64 | valor de 24 chars `^[A-Za-z0-9+/]*={0,2}$` contendo "data" | passa (isento) |
| Content-Type longo | header com 1001 chars | `400` |
| Recover | handler que dá `panic("boom")` | `500` JSON genérico + log com stack |
| Request limpo | `GET /ad?hid=ABC&red=https%3A%2F%2Fx.com` | passa intocado |

## Arquivos a criar/alterar

- `internal/middleware/doc.go` — godoc do pacote (ordem canônica de encadeamento).
- `internal/middleware/middleware.go` — tipos `Handler`/`Middleware` + `Encadeia`.
- `internal/middleware/cors.go` — (`// Portado de: CorsFilter.java`).
- `internal/middleware/validation.go` — (`// Portado de: RequestValidationFilter.java`), regexes pré-compiladas.
- `internal/middleware/recover.go` — middleware de recover.
- `internal/middleware/cors_test.go`, `validation_test.go`, `recover_test.go` — todos os casos da tabela.

## Critérios de aceite

- [ ] Tipos `Handler`/`Middleware`/`Encadeia` definidos sobre `events.APIGatewayV2HTTPRequest/Response` (payload v2), sem framework HTTP (ADR-001).
- [ ] CORS: reflexão de origin + credentials + `Vary: Origin` quando `Origin` presente; `*` sem credentials caso contrário; OPTIONS curto-circuitado com opt-out configurável.
- [ ] RequestValidation implementa as 12 regras acima, com as listas COMPLETAS copiadas do `RequestValidationFilter.java` (não apenas os exemplos da issue) e comentário por bloco citando a regra de origem.
- [ ] Bypasses do `/proxy-tracker` (query string e param `u`) e heurística base64 (len>20) funcionando — testes cobrem.
- [ ] Regexes compiladas 1× em nível de pacote (nenhum `regexp.MustCompile` dentro de função chamada por request).
- [ ] `Recover` responde 500 genérico e loga stack; nenhum panic escapa em teste.
- [ ] Todos os casos da tabela de testes implementados e verdes; `make lint && make test` ok.
- [ ] Código 100% comentado em português; `// Portado de: CorsFilter.java / RequestValidationFilter.java` presentes.

## Dependências

Bloqueada por: M0-01 (bootstrap do repositório Go).
Bloqueia: M3-03 (track-handler), M3-06 (redirect-handler), M5-08 (proxy-handler) — todos os handlers HTTP consomem este pacote.

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §16 (especificação dos dois filtros) e "Notas de migração" 1–2.
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §4 (`internal/middleware`), §7 item 3 (WAF complementa, não substitui).
- Código Java de referência: `c:\Users\Fabio\Documents\Dev\ad-server` (`CorsFilter.java`, `RequestValidationFilter.java` — fonte das listas completas de padrões).
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) §3 (comentar o PORQUÊ de cada bloco de validação).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-07 (internal/middleware) do ad-serverless: middleware encadeável (Handler/Middleware/Encadeia sobre APIGatewayV2) com (1) CORS portado do CorsFilter.java — reflexão de Origin com Access-Control-Allow-Credentials: true e Vary: Origin quando Origin presente, senão "*" sem credentials, OPTIONS 200 imediato com opt-out, Max-Age 3600; (2) RequestValidation portado do RequestValidationFilter.java com TODAS as regras (métodos válidos, encodados proibidos %7C %3C %3E..., charset permitido do path, padrões PHP/SQL/command injection copiados INTEGRALMENTE do Java em c:\Users\Fabio\Documents\Dev\ad-server, params perigosos cmd/exec/opt/mdb/sys..., bypasses para /proxy-tracker e param u, heurística base64 len>20, combinações opt=sys etc., Content-Type>1000); (3) Recover que converte panic em 500 genérico com log de stack. Seguir docs/issues/M1-07-middleware-cors-validation.md com a tabela de testes completa, código 100% comentado em português, make lint && make test verdes, abrir PR feat/issue-M1-07-middleware com Closes na issue.
```
