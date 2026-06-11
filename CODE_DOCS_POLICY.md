# Política de Documentação de Código — ad-serverless (Go)

## Regra Fundamental

**Todo o código deve ser documentado em português.**

Este projeto nasce da migração de um sistema com histórico grave de falta de documentação. Aqui, documentação é obrigatória e não negociável — é critério de aceite de todo PR.

## 1. Pacotes

Todo pacote tem um `doc.go` (ou comentário no arquivo principal) explicando sua responsabilidade no contexto do ad server:

```go
// Package frequencycap implementa a validação de elegibilidade de campanhas
// por horário e dia da semana.
//
// As campanhas armazenam restrições como strings no formato legado do MySQL
// (ex.: "0;5>10;15>>"), onde ";" separa fragmentos, ">" define range exclusivo
// e ">>" range inclusivo. Este pacote faz o parse e o matching contra a data
// atual em America/Sao_Paulo.
//
// Portado de: FrequencyCapComponentImpl.java + DigitExtractor.java (ad-server).
package frequencycap
```

## 2. Funções e métodos exportados

Descrever O QUE faz e POR QUÊ existe (significado de negócio), parâmetros com semântica, retorno e erros:

```go
// SelecionaCampanha escolhe aleatoriamente (distribuição uniforme) uma campanha
// elegível para o hotspot informado.
//
// Uma campanha é elegível quando:
//   - enabled = true no banco;
//   - o frequency cap de hora/dia da semana permite o momento atual.
//
// A seleção aleatória uniforme é uma regra de negócio: distribui as impressões
// igualmente entre campanhas concorrentes no mesmo hotspot (paridade com
// NumberUtils.getPositiveIndex do legado).
//
// Retorna nil quando nenhuma campanha é elegível — o chamador deve responder
// com anúncio vazio (template emptyAd), nunca com erro.
func SelecionaCampanha(hotspot *domain.HotSpot, agora time.Time) *domain.Campaign {
```

## 3. Blocos complexos

Comentário inline explicando o PORQUÊ (não o que a linha faz):

```go
// O Google invalida URLs assinadas se qualquer parâmetro for alterado.
// Por isso, VAST vindo de doubleclick.net/googlesyndication.com NÃO passa
// pelo proxy: MediaFile, JavaScriptResource e AdParameters ficam intocados.
// Portado de: VastService.java (isGoogleDoubleClickUrl).
if ehGoogleDoubleClick(host) {
    return urlOriginal
}
```

## 4. Constantes e enums

Cada valor com seu significado de negócio:

```go
// Tipos de evento rastreados pelo ad server. Os valores são as strings
// persistidas em ad_trackers.event_type — NUNCA renomear (paridade com
// EventType.Values do ad-commons; note que o nome difere do valor em
// PLAYED_25_PER → "25_PER_PLAYED" e REDIRECT → "REDIRECT_CAMPAIGN").
const (
    // EventoPageView registra a visualização da página no hotspot WiFi.
    EventoPageView = "PAGE_VIEW"
    // EventoVideoStarted registra o início da reprodução do vídeo (VAST start).
    EventoVideoStarted = "VIDEO_STARTED"
    // ...
)
```

## 5. Structs de domínio

Cada campo com a coluna/atributo de origem e semântica:

```go
// AdTracker representa um evento de tracking (linha da tabela MySQL ad_trackers,
// replicada de forma assíncrona para a tabela DynamoDB AdTrackers).
type AdTracker struct {
    // CampaignID é a campanha associada ao evento (coluna campaign_id).
    CampaignID int
    // HotspotID identifica o ponto WiFi de origem (coluna hotspot_id).
    // É vazio/NULL em eventos de tracking pixel e postback — comportamento legado.
    HotspotID string
    // ...
}
```

## 6. Referência à origem (obrigatório em código portado)

Toda lógica portada do Java referencia a classe/método de origem:

```go
// Portado de: AdTrackerDynamoComponent.java#save — a sort key concatena o
// timestamp ISO8601 com o ID do MySQL para garantir ordenação cronológica
// e unicidade: "2024-06-10T15:30:45.123Z#12345".
```

## 7. Testes

Documentar o que está sendo testado e por quê:

```go
// TestFrequencyCap_RangeExclusivo garante a paridade com DigitRange do Java:
// "5>10" deve permitir as horas 5..9 e NEGAR a hora 10 (range exclusivo).
func TestFrequencyCap_RangeExclusivo(t *testing.T) {
```

## Prioridade de documentação

1. `internal/domain` + `internal/frequencycap` (usados por todos)
2. Handlers Lambda (`cmd/*`) — entry points
3. `internal/vast` + `internal/proxy` (lógica mais complexa)
4. Repositórios e middleware
5. Testes

## Verificação

- `golangci-lint` com `revive` (regra `exported` — doc comment obrigatório em todo símbolo exportado).
- Code review com checklist de documentação em cada PR (item do template de PR).
- Golden tests servem também como documentação executável do comportamento legado.
