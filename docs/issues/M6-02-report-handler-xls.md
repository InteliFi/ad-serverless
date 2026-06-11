---
title: "[M6-02] report-handler: GET /adtrack/xls (export Excel com excelize)"
labels: ["epic:M6-relatorios", "tipo:port", "prioridade:P2"]
milestone: "M6 — Relatórios"
---
## Contexto

`GET /adtrack/xls` exporta o MESMO relatório do `GET /adtrack` (M6-01) em planilha Excel para download. No legado, `TrackerReportComponent.generateXls(report)` usa Apache POI 3.15 (`HSSFWorkbook`, formato binário antigo), cria a sheet `report_sheet`, escreve em arquivo temporário e devolve os bytes com `Content-Disposition: attachment`. Na migração, a decisão já registrada (docs/legado/01 §7) é usar **`xuri/excelize/v2` com streaming** (`StreamWriter`) — sem arquivo temporário, memória O(1) por linha, formato `.xlsx` moderno. **A paridade obrigatória é o CONTEÚDO**: mesmas colunas, mesmo header, mesmos valores e mesma ordem de linhas do legado.

Este endpoint roda na mesma Lambda `report-handler` de M6-01 (1024MB / 120s, fora do hot path) e reusa a agregação SQL `GROUP BY` — não reimplementar a consulta.

## Especificação detalhada

### Colunas e header — paridade EXATA com o legado

Header (linha 1), nesta ordem exata (docs/legado/01 §7 — 13 colunas):

```
EventDate | CampaignId | PAGE_VIEW | IMPRESSION_PRE_ROLL | CLICK_PRE_ROLL | IMPRESSION_CAMPAIGN | CLICK_CAMPAIGN | VIDEO_STARTED | PLAYED_25_PER | PLAYED_50_PER | PLAYED_75_PER | VIDEO_END | REDIRECT
```

⚠️ Atenção a três pegadinhas de paridade:
1. Os títulos das colunas de evento usam os **NOMES do enum Java** (`PLAYED_25_PER`, `VIDEO_END`, `REDIRECT`) e NÃO os valores persistidos no banco (`25_PER_PLAYED`, `VIDEO_ENDED`, `REDIRECT_CAMPAIGN`) — é o inverso do que acontece no `event_type` da tabela. Os dados continuam vindo dos valores persistidos; só o TÍTULO da coluna usa o nome.
2. O XLS NÃO tem coluna `TRACKING_PIXEL` nem coluna de hotspot (diferente do JSON de M6-01, que tem ambos) — conferir no `generateXls` do Java como as linhas são achatadas (agregação por campanha+data, somando os hotspots? linha por hotspot?) e reproduzir EXATAMENTE, documentando com referência à linha do Java.
3. Sheet com o nome exato **`report_sheet`**.

Linhas de dados: uma por item do relatório na MESMA ordem do legado; `EventDate` no formato em que o Java escreve a célula (string `yyyy-MM-dd` ou célula de data — conferir no fonte e replicar); contadores como células numéricas.

### Geração com excelize/v2 streaming

```go
// GeraXLS escreve o relatório em formato Excel usando StreamWriter
// (memória constante por linha — o relatório pode ter dezenas de milhares
// de linhas). Retorna os bytes prontos para a resposta HTTP.
// Portado de: TrackerReportComponent.java (generateXls) — POI HSSF trocado
// por excelize/v2 streaming por decisão registrada em docs/legado/01 §7.
func GeraXLS(report *AdTrackerReport) ([]byte, error)
```

- `excelize.NewFile()` → `NewStreamWriter("report_sheet")` (renomear/usar a sheet default para não sobrar `Sheet1` — conferir nome final no arquivo gerado).
- Header via `SetRow("A1", ...)`; dados linha a linha (`SetRow("A2"...)`); `Flush()` obrigatório antes de serializar.
- Serializar com `WriteToBuffer()` (sem arquivo temporário — Lambda tem /tmp limitado e o legado usar temp file era detalhe de implementação, não contrato).
- SEM estilos/cores/larguras além do que o Java produz (HSSF cru) — não "melhorar" a planilha (regra de paridade).

### Resposta HTTP

- Status `200`, corpo binário → `events.APIGatewayV2HTTPResponse` com `IsBase64Encoded: true` (helper `resp.Binario` de M1-08).
- Headers:
  - `Content-Type`: conferir o valor EXATO enviado pelo Java (`AdTrackService.java`) — tipicamente `application/vnd.ms-excel` (HSSF) ou `application/octet-stream`; como o formato físico muda para `.xlsx`, usar `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` e registrar a divergência consciente no comentário e na MATRIZ-PARIDADE (consumidor = humano via navegador/Excel; abrir normalmente é o contrato).
  - `Content-Disposition: attachment; filename="<mesmo nome do legado>"` — copiar o filename EXATO do `AdTrackService.java` (se o legado usa extensão `.xls`, manter o nome e documentar que o conteúdo é xlsx, OU ajustar para `.xlsx` com justificativa — decisão registrada no PR).
- Erro de geração/consulta → `500` genérico + log slog (nunca vazar erro interno).
- ⚠️ API Gateway HTTP API tem limite de payload de resposta de 10MB — calcular o tamanho esperado (~linhas agregadas, não 14M) e registrar no comentário; se um dia estourar, a saída vai para S3 + presigned URL (issue futura `melhoria`, NÃO implementar agora).

### Handler e roteamento

- Mesma Lambda `cmd/report/main.go` de M6-01: rota `GET /adtrack/xls` no roteador interno.
- Fluxo: `AgregadoTracking` (M6-01) → montagem do `AdTrackerReport` (M6-01) → `GeraXLS` → `resp.Binario`. Zero duplicação da query/montagem.
- Middleware: `Recover` + `CORS` + `RequestValidation` (M1-07).

### Testes obrigatórios

1. **Header:** planilha gerada → linha 1 tem EXATAMENTE as 13 colunas na ordem da spec (`EventDate`...`REDIRECT`), sheet `report_sheet`.
2. **Dados:** report com 3 items conhecidos → células conferidas uma a uma (abrir com excelize no teste e ler de volta `GetRows`).
3. **Mapeamento nome×valor:** item com contadores de `25_PER_PLAYED`/`VIDEO_ENDED`/`REDIRECT_CAMPAIGN` → valores aparecem sob os títulos `PLAYED_25_PER`/`VIDEO_END`/`REDIRECT`.
4. **Report vazio:** planilha só com header, sem erro.
5. **Volume:** 50k linhas via StreamWriter sem estourar memória (teste com `-short` skip se lento).
6. **Handler:** resposta com `IsBase64Encoded=true`, Content-Disposition attachment com o filename do legado; round-trip base64 → bytes abrem no excelize.
7. **Golden (recomendado):** capturar o XLS do Java para os mesmos dados e comparar o CONTEÚDO lógico (linhas/células via leitura, não bytes — formatos físicos diferem HSSF×xlsx).

## Arquivos a criar/alterar

- `internal/report/xls.go` — `GeraXLS` (`// Portado de: TrackerReportComponent.java (generateXls)`).
- `internal/report/xls_test.go` — testes 1–5 e 7.
- `cmd/report/main.go` — adicionar a rota `GET /adtrack/xls` (criado em M6-01).
- `go.mod` — dependência `github.com/xuri/excelize/v2`.
- `tests/golden/report/adtrack-report.xls` — fixture capturada do Java (para o teste 7).
- `docs/MATRIZ-PARIDADE.md` — linha do `GET /adtrack/xls` (registrar divergências conscientes: xlsx em vez de xls binário, Content-Type).

## Critérios de aceite

- [ ] Sheet `report_sheet` com header EXATO de 13 colunas: `EventDate, CampaignId, PAGE_VIEW, IMPRESSION_PRE_ROLL, CLICK_PRE_ROLL, IMPRESSION_CAMPAIGN, CLICK_CAMPAIGN, VIDEO_STARTED, PLAYED_25_PER, PLAYED_50_PER, PLAYED_75_PER, VIDEO_END, REDIRECT` (nomes do enum nos títulos, valores persistidos na origem dos dados).
- [ ] Estrutura de linhas (achatamento de hotspot, ordem) conferida no `generateXls` do Java e reproduzida, com comentário citando a origem.
- [ ] Geração via `excelize/v2` `StreamWriter` + `WriteToBuffer` — sem arquivo temporário, teste de 50k linhas passa.
- [ ] Resposta com `IsBase64Encoded=true` e `Content-Disposition: attachment; filename=...` idêntico ao do `AdTrackService.java`.
- [ ] Reusa `AgregadoTracking` e a montagem de M6-01 (zero duplicação de query/lógica).
- [ ] Divergências conscientes (xlsx, Content-Type) documentadas na MATRIZ-PARIDADE e no código.
- [ ] Código 100% comentado em português; `make lint && make test` verdes.

## Dependências

Bloqueada por: M6-01 (agregação + DTOs + Lambda report-handler).

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §7 (contrato do endpoint, header de colunas, decisão excelize/v2 streaming).
- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.6 (generateXls: HSSFWorkbook, sheet `report_sheet`, temp file).
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §4 (Apache POI 3.15 → `xuri/excelize/v2`).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 (report-handler 1024MB/120s).
- Código Java de referência: `c:\Users\Fabio\Documents\Dev\ad-server` (`TrackerReportComponent.java#generateXls`, `AdTrackService.java` — filename e Content-Type exatos).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M6-02 (report-handler GET /adtrack/xls) do ad-serverless: gerar o relatório Excel com excelize/v2 StreamWriter (WriteToBuffer, sem temp file), sheet "report_sheet", header EXATO de 13 colunas EventDate, CampaignId, PAGE_VIEW, IMPRESSION_PRE_ROLL, CLICK_PRE_ROLL, IMPRESSION_CAMPAIGN, CLICK_CAMPAIGN, VIDEO_STARTED, PLAYED_25_PER, PLAYED_50_PER, PLAYED_75_PER, VIDEO_END, REDIRECT (títulos com NOMES do enum; dados vindos dos valores persistidos 25_PER_PLAYED/VIDEO_ENDED/REDIRECT_CAMPAIGN), reusando a agregação SQL e os DTOs de M6-01, resposta APIGatewayV2 com IsBase64Encoded=true e Content-Disposition: attachment com o filename copiado de AdTrackService.java (conferir em c:\Users\Fabio\Documents\Dev\ad-server, junto com o achatamento de hotspot do generateXls). Seguir docs/issues/M6-02-report-handler-xls.md e CLAUDE.md, código 100% comentado em português, registrar divergências conscientes (xlsx/Content-Type) na MATRIZ-PARIDADE, make lint && make test verdes, abrir PR feat/issue-M6-02-report-handler-xls com Closes na issue.
```
