---
title: "[M1-06] internal/tracking: PostbackSignature MD5 + validação de eventos"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P2"]
milestone: "M1 — Commons Go"
---
## Contexto

O endpoint `GET /adtrack/postback` recebe conversões de redes de afiliados (modatta, prezão etc.). O legado define um mecanismo de assinatura (`PostbackSignature`) para autenticar esses callbacks: `hex(MD5(campaignId + event + key))`, com a chave secreta na config `intv.ad.signaturekey`. **Detalhe crítico de paridade:** no Java a chamada de validação está COMENTADA com um TODO (`AdTrackComponent.savePostback`, passo 2) — ou seja, em produção hoje a assinatura NÃO é validada. Esta issue porta o algoritmo completo e a validação de event types para `internal/tracking`, com a validação de assinatura atrás de uma **feature flag desligada por default** (paridade) que poderá ser ativada depois sem novo deploy de código (issue futura `melhoria`).

A chave do legado está hardcoded em `application.properties` (comprometida — ver M0-05); aqui a chave SEMPRE vem do SSM Parameter Store (SecureString), nunca do código.

## Especificação detalhada

### Algoritmo `PostbackSignature` — port EXATO

Port de `PostbackSignature.java` (ad-server) e do pseudocódigo do docs/legado/04 §5:

```
assinatura = hex(MD5(campaignId + event + key))
válida se strings.ToLower(assinaturaRecebida) == assinaturaGerada
```

- Concatenação **de strings, sem separador**: `campaignId` em decimal sem zeros à esquerda (ex.: `123`, não `0123`), seguido do `event` cru (como veio no query param), seguido da `key`.
- Hash: `crypto/md5` da stdlib; saída em **hexadecimal minúsculo** (`hex.EncodeToString`).
- Validação **case-insensitive na entrada**: o legado faz `signature.toLowerCase().equals(gerada)` — a assinatura gerada já é minúscula; a recebida é normalizada antes da comparação. Comparar com `strings.EqualFold` ou `ToLower` explícito.
- MD5 aqui NÃO é uso criptográfico de senha — é o protocolo combinado com as redes de afiliados (paridade obrigatória; trocar de hash quebraria os parceiros). Documentar isso no godoc e suprimir o linter `gosec G401/G501` com `//nolint` justificado.

### API pública

```go
// GeraAssinatura calcula hex(MD5(campaignId + event + key)) em minúsculas.
// Portado de: PostbackSignature.java (ad-server).
func GeraAssinatura(campaignID int, event, key string) string

// ValidaAssinatura compara a assinatura recebida com a gerada, ignorando
// maiúsculas/minúsculas na recebida (paridade com signature.toLowerCase()).
func ValidaAssinatura(assinaturaRecebida string, campaignID int, event, key string) bool

// ValidaEventType retorna ErrEventTypeInvalido quando a string não é um dos
// 17 valores persistidos de EventType (M1-01). Portado de:
// AdTrackComponent.savePostback passo 1 — AdException("Invalid event type").
func ValidaEventType(event string) error
```

- `ErrEventTypeInvalido` exportado como `errors.New`/erro sentinela com a mensagem do legado (`"Invalid event type"`) — o postback-handler (M3-07) mapeia para HTTP 422.
- `ValidaEventType` usa a função `EventTypeValido` / lista de constantes de M1-01 — **não duplicar a lista de eventos** neste pacote.
- Atenção: a validação recebe o **valor persistido** (ex.: `25_PER_PLAYED`, `REDIRECT_CAMPAIGN`), não o nome do enum Java.

### Feature flag de validação (paridade: DESLIGADA)

```go
// Config controla o comportamento da validação de postback.
type Config struct {
    // SignatureValidationEnabled liga a validação de assinatura.
    // Default FALSE — paridade com o legado, onde a chamada está comentada
    // (TODO em AdTrackComponent.savePostback). Ativar = issue `melhoria`.
    SignatureValidationEnabled bool
    // SignatureKey é a chave secreta (SSM SecureString /ad-serverless/{stage}/signature-key).
    SignatureKey string
}

// ValidaPostback aplica as validações na ordem do legado:
// 1. event type válido (sempre); 2. assinatura (somente se a flag estiver ligada).
func (c Config) ValidaPostback(campaignID int, event, assinatura string) error
```

- Flag lida da env var `POSTBACK_SIGNATURE_VALIDATION_ENABLED` (string `"true"`/`"false"`, default `false`) via config loader (M1-08); este pacote só recebe o `Config` montado — sem `os.Getenv` aqui dentro.
- Com a flag desligada, `ValidaPostback` **ignora completamente** a assinatura (nem exige o parâmetro) — comportamento idêntico ao código comentado.
- Com a flag ligada e assinatura inválida → erro mapeável para `401 NotAuthorized` (exceção `NotAuthorizedException` do legado, docs/legado/02 §3).
- A chave NUNCA aparece em logs (nem em nível DEBUG) — critério de aceite.

### Tabela de casos de teste OBRIGATÓRIOS

| Caso | Entrada | Esperado |
|---|---|---|
| Fixture 1 | `GeraAssinatura(123, "CPA", "chave-teste")` | `806544ee130b7e513d9fdb20fc790692` |
| Fixture 2 | `GeraAssinatura(55, "POSTBACK_CLICK", "segredo")` | `be7942552cec1f67ba881804f0a577d0` |
| Case-insensitive | `ValidaAssinatura("806544EE130B7E513D9FDB20FC790692", 123, "CPA", "chave-teste")` | `true` |
| Assinatura errada | hash de outra campanha | `false` |
| Event válido | `ValidaEventType("25_PER_PLAYED")` | `nil` |
| Event válido (postback) | `ValidaEventType("POSTBACK_INSTALL_ANDROID")` | `nil` |
| Event inválido | `ValidaEventType("PLAYED_25_PER")` (nome Java ≠ valor persistido!) | `ErrEventTypeInvalido` |
| Event inválido | `ValidaEventType("FOO")`, `""` | `ErrEventTypeInvalido` |
| Flag OFF | `Config{false, ""}.ValidaPostback(1, "POSTBACK_CPA", "qualquer-coisa")` | `nil` (assinatura ignorada) |
| Flag ON ok | assinatura correta | `nil` |
| Flag ON inválida | assinatura errada/vazia | erro de não autorizado |

## Arquivos a criar/alterar

- `internal/tracking/doc.go` — godoc do pacote (contexto do postback de afiliados; nota sobre a validação comentada no legado).
- `internal/tracking/signature.go` — `GeraAssinatura` + `ValidaAssinatura` (`// Portado de: PostbackSignature.java`).
- `internal/tracking/eventos.go` — `ValidaEventType` + `ErrEventTypeInvalido` (`// Portado de: AdTrackComponent.savePostback`).
- `internal/tracking/config.go` — `Config` + `ValidaPostback` com a feature flag documentada.
- `internal/tracking/signature_test.go`, `eventos_test.go` — todos os casos da tabela.

## Critérios de aceite

- [ ] `GeraAssinatura` reproduz `hex(MD5(campaignId + event + key))` minúsculo — fixtures 1 e 2 da tabela passam byte a byte.
- [ ] Validação case-insensitive na assinatura recebida (teste com hash em MAIÚSCULAS).
- [ ] `ValidaEventType` aceita exatamente os 17 valores persistidos de M1-01 e rejeita os nomes Java divergentes (`PLAYED_25_PER`, `REDIRECT`).
- [ ] Feature flag `SignatureValidationEnabled` default `false`; com OFF a assinatura é ignorada (paridade com o TODO comentado do Java — citar no comentário).
- [ ] Chave secreta nunca logada; nenhum valor de chave hardcoded em código ou teste de integração (fixtures de teste usam chaves sintéticas como acima).
- [ ] `//nolint:gosec` justificado no uso de MD5 (protocolo legado com parceiros).
- [ ] `// Portado de: PostbackSignature.java / AdTrackComponent.java` presentes; código 100% comentado em português.
- [ ] `make lint && make test` verdes.

## Dependências

Bloqueada por: M1-01 (constantes EventType).
Bloqueia: M3-03 (track-handler valida eventos), M3-07 (postback-handler usa ValidaPostback).

## Referências

- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §5 (pseudocódigo PostbackSignature).
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.4 (savePostback — validação comentada com TODO), §3 (exceções), §4 (EventType.Values).
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §8 (endpoint postback completo).
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2 (`intv.ad.signaturekey` hardcoded — rotacionar, mover p/ SSM).
- Código Java de referência: `c:\Users\Fabio\Documents\Dev\ad-server` (PostbackSignature.java, AdTrackComponentImpl.java).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-06 (internal/tracking) do ad-serverless: portar o PostbackSignature.java — GeraAssinatura(campaignID, event, key) = hex(MD5(campaignId+event+key)) minúsculo, ValidaAssinatura case-insensitive na assinatura recebida — e a validação de event types contra os 17 VALORES PERSISTIDOS de M1-01 (ErrEventTypeInvalido com mensagem "Invalid event type"). Incluir Config com feature flag SignatureValidationEnabled DEFAULT FALSE (paridade: validação comentada com TODO no Java) e chave vinda de SSM (nunca hardcoded, nunca logada). Seguir docs/issues/M1-06-tracking-postback-signature.md à risca, incluindo as fixtures MD5 exatas da tabela de testes. Código 100% comentado em português com "// Portado de: ...", make lint && make test verdes, abrir PR feat/issue-M1-06-tracking-postback-signature com Closes na issue.
```
