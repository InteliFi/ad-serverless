---
title: "[M4-01] Migração dos 45 templates .vm + fixtures golden do Java"
labels: ["epic:M4-adserving", "tipo:port", "prioridade:P1"]
milestone: "M4 — Ad Serving"
---
## Contexto

A issue [M1-05] criou a infraestrutura de templates (`internal/templates`): engine de substituição literal `${key}` via `strings.NewReplacer`, embedding com `go:embed` e cache em memória — com apenas 2–3 templates de exemplo. Esta issue completa o trabalho: **copiar TODOS os templates `.vm` reais do legado, byte a byte, sem nenhuma modificação**, e capturar as fixtures golden do Java que servirão de referência de paridade para o `ad-handler` ([M4-03]).

Apesar da extensão `.vm`, os arquivos NÃO são Velocity: são texto puro com placeholders `${key}` substituídos literalmente (ver `docs/legado/02-logica-negocio.md` §1.8). Qualquer alteração de conteúdo (encoding, quebra de linha, BOM) quebra a paridade byte a byte exigida pelos golden tests — por isso a cópia deve ser tratada como cópia de binário.

Origem (repo Java local): `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/templates/`.

## Especificação detalhada

### 1. Inventário REAL dos templates (44 arquivos `.vm` encontrados no commit `74748d2`)

> Nota de paridade: a documentação fala em "~45 templates"; o inventário real do diretório `src/main/resources/templates/` contém **44 arquivos** `.vm`. O código Java referencia ainda `templates/programatica/sizmekFull.vm` (método `getSizmekFull` de `TemplateComponentImpl.java`), **que NÃO existe em disco** — é código morto; não criar esse arquivo.

Lista completa (caminho relativo a `src/main/resources/templates/`):

| # | Arquivo Java | Destino em `internal/templates/assets/` |
|---|---|---|
| 1 | `appInstallAd.vm` | `appInstallAd.vm` |
| 2 | `bannerAd.vm` | `bannerAd.vm` |
| 3 | `bannerBusAppInstallAd.vm` | `bannerBusAppInstallAd.vm` |
| 4 | `bannerButtonClose.vm` | `bannerButtonClose.vm` |
| 5 | `bannerQuestionAd.vm` | `bannerQuestionAd.vm` |
| 6 | `bannerQuestionAdNoAuth.vm` | `bannerQuestionAdNoAuth.vm` |
| 7 | `emptyAd.vm` | `emptyAd.vm` |
| 8 | `justBannerAd.vm` | `justBannerAd.vm` |
| 9 | `pixelTracking.vm` | `pixelTracking.vm` |
| 10 | `spTransBanner.vm` | `spTransBanner.vm` |
| 11 | `vast42.vm` | `vast42.vm` |
| 12 | `videoAd.vm` | `videoAd.vm` |
| 13 | `videoAdSkyfi.vm` | `videoAdSkyfi.vm` |
| 14 | `ima/ima.vm` | `ima/ima.vm` |
| 15 | `ima/imaProgramatica.vm` | `ima/imaProgramatica.vm` |
| 16 | `msps/wiconnect/bannerAd.vm` | `msps/wiconnect/bannerAd.vm` |
| 17 | `msps/wiconnect/videoAd.vm` | `msps/wiconnect/videoAd.vm` |
| 18 | `adservers/space/banner/bannerSpace.vm` | `space/banner/bannerSpace.vm` |
| 19 | `adservers/space/vpaid/videoVpaidSpace.vm` | `space/vpaid/videoVpaidSpace.vm` |
| 20 | `adservers/space/vpaid/videoVpaidSpaceWico.vm` | `space/vpaid/videoVpaidSpaceWico.vm` |
| 21 | `programatica/bannerBMC.vm` | `programatica/bannerBMC.vm` |
| 22 | `programatica/bannerWebMotors.vm` | `programatica/bannerWebMotors.vm` |
| 23 | `programatica/betanoPrezao.vm` | `programatica/betanoPrezao.vm` |
| 24 | `programatica/campaignProgramatica.vm` | `programatica/campaignProgramatica.vm` |
| 25 | `programatica/campaignProgramaticaPreRollClick.vm` | `programatica/campaignProgramaticaPreRollClick.vm` |
| 26 | `programatica/gamAeroVix.vm` | `programatica/gamAeroVix.vm` |
| 27 | `programatica/gamSPTransNardelli.vm` | `programatica/gamSPTransNardelli.vm` |
| 28 | `programatica/googleAdUnit.vm` | `programatica/googleAdUnit.vm` |
| 29 | `programatica/noAdBannerProgramatica.vm` | `programatica/noAdBannerProgramatica.vm` |
| 30 | `programatica/programatica.vm` | `programatica/programatica.vm` |
| 31 | `programatica/programaticaBMC.vm` | `programatica/programaticaBMC.vm` |
| 32 | `programatica/programaticaClaro.vm` | `programatica/programaticaClaro.vm` |
| 33 | `programatica/programaticaClaroPreRollClick.vm` | `programatica/programaticaClaroPreRollClick.vm` |
| 34 | `programatica/programaticaSelfClose.vm` | `programatica/programaticaSelfClose.vm` |
| 35 | `programatica/programaticaSmart.vm` | `programatica/programaticaSmart.vm` |
| 36 | `programatica/programaticaVAST.vm` | `programatica/programaticaVAST.vm` |
| 37 | `programatica/programaticaVideoAd.vm` | `programatica/programaticaVideoAd.vm` |
| 38 | `programatica/programaticaWebMotors.vm` | `programatica/programaticaWebMotors.vm` |
| 39 | `programatica/redirect_postback.vm` | `programatica/redirect_postback.vm` |
| 40 | `programatica/smartAdAerBSB.vm` | `programatica/smartAdAerBSB.vm` |
| 41 | `programatica/smartAdAerVCP.vm` | `programatica/smartAdAerVCP.vm` |
| 42 | `programatica/smartAdRSS.vm` | `programatica/smartAdRSS.vm` |
| 43 | `programatica/smartAdSptrans.vm` | `programatica/smartAdSptrans.vm` |
| 44 | `programatica/smartClaroApp.vm` | `programatica/smartClaroApp.vm` |

### 2. Regras de cópia (byte a byte)

- Copiar cada arquivo **SEM MODIFICAR** (conteúdo idêntico, byte a byte): sem reformatar, sem trocar encoding, sem normalizar quebras de linha (CRLF/LF como estiver no original), sem adicionar/remover BOM.
- **Estrutura de pastas:** preservar as subpastas `msps/`, `ima/`, `programatica/`; a única transformação de caminho é `adservers/space/**` → `space/**`, seguindo a convenção de nomes já estabelecida em [M1-05] (`space/vpaid/videoVpaidSpace.vm`). O conteúdo permanece intocado.
- Adicionar regra no `.gitattributes` para impedir que o git altere quebras de linha dos assets: `internal/templates/assets/** -text`.
- Garantir que o `//go:embed assets` existente em `internal/templates` cobre todas as subpastas (embed recursivo). Se [M1-05] criou templates de exemplo com nomes que colidem com os reais (ex.: `emptyAd.vm`), os reais SUBSTITUEM os de exemplo.
- Verificação obrigatória pós-cópia: comparar hash MD5/SHA-256 de cada arquivo origem×destino (ex.: script `make` ou teste Go que itera o `embed.FS` — ver testes abaixo).

### 3. Teste de inventário embarcado

Criar `internal/templates/assets_test.go` com:

1. Teste que percorre o `embed.FS` e compara a lista de arquivos com a lista canônica dos 44 caminhos da tabela acima (falha se faltar ou sobrar arquivo).
2. Teste de integridade: para os templates usados nos golden tests, validar tamanho em bytes > 0 e ausência de `\r` APENAS se o original não tiver `\r` (ou seja: o teste fixa o hash SHA-256 de cada asset, gerado uma vez na migração e gravado em `internal/templates/assets_checksums.txt`).

### 4. Captura de fixtures golden do Java (ambiente dev EC2)

As fixtures alimentam os golden tests do [M4-03] e o harness do [M8-01]. Procedimento:

1. Identificar, no MySQL de dev, ao menos **1 hotspot representativo por `CreativeType` em uso** (consulta sugerida — adaptar se necessário; somente `SELECT`):
   ```sql
   SELECT h.code AS hid, c.id AS cid, cr.id AS creative_id, ct.name AS creative_type
   FROM hotspots h
   JOIN hotspots_campaigns hc ON hc.hotspot_id = h.id
   JOIN campaigns c ON c.id = hc.campaign_id AND c.enabled = 1
   JOIN creatives cr ON cr.campaign_id = c.id
   JOIN creative_types ct ON ct.id = cr.creative_type_id
   ORDER BY ct.name;
   ```
2. Para cada tipo, executar contra o ambiente dev EC2 do legado (host informado pelo operador; NUNCA produção):
   ```bash
   curl -sS "https://<dev-ec2-legado>/ad?hid=<HID>" \
     -D "tests/golden/ad/<creative_type>.headers.txt" \
     -o "tests/golden/ad/<creative_type>.js"
   ```
3. Salvar também 1 caso com `red` (param de redirect): `.../ad?hid=<HID>&red=https%3A%2F%2Fexemplo.com` → `tests/golden/ad/<creative_type>_red.js`.
4. Registrar em `tests/golden/ad/FIXTURES.md`: data da captura, hid, cid, creative_id, tipo e URL usada — sem credenciais.
5. ⚠️ A seleção de campanha/creative é **aleatória**: para hotspots com mais de 1 campanha/creative elegível, repetir o curl até capturar o creative desejado (conferir `${cid}` no corpo) ou usar hotspot de dev com 1 única campanha/creative. Os campos dinâmicos (`${z_server_timestamp}`, `${ad_tracker_timestamp}`) variam por request — os golden tests do [M4-03] devem normalizá-los (regex) antes de comparar.

## Arquivos a criar/alterar

- `internal/templates/assets/**` — os 44 `.vm` copiados (tabela acima).
- `internal/templates/assets_test.go` — teste de inventário + checksums.
- `internal/templates/assets_checksums.txt` — SHA-256 de cada asset (gerado na migração).
- `.gitattributes` — `internal/templates/assets/** -text`.
- `tests/golden/ad/` — fixtures capturadas (`<creative_type>.js`, `.headers.txt`, `FIXTURES.md`).
- `docs/MATRIZ-PARIDADE.md` — marcar linha dos templates.

## Critérios de aceite

- [ ] Os 44 arquivos `.vm` presentes em `internal/templates/assets/` com conteúdo byte-idêntico ao original (checksums conferidos e versionados).
- [ ] Estrutura de pastas preservada (`msps/wiconnect/`, `ima/`, `programatica/`, `space/banner/`, `space/vpaid/`).
- [ ] `sizmekFull.vm` NÃO criado (não existe no legado); `videoAdSkyfi.vm` copiado mesmo sem mapeamento no decider (paridade de inventário).
- [ ] Teste de inventário falha se qualquer um dos 44 caminhos faltar no `embed.FS`.
- [ ] `.gitattributes` impede normalização de EOL nos assets.
- [ ] Fixtures golden capturadas para os tipos de creative ativos em dev, com `FIXTURES.md` documentando cada captura.
- [ ] Código/testes comentados em português; `make lint && make test` verdes.

## Dependências

Bloqueada por: [M1-05]

## Referências

- `docs/legado/02-logica-negocio.md` §1.8 (TemplateComponent — engine `${key}`, cache, falha → null).
- `docs/legado/01-endpoints-http.md` §2 (`GET /ad`).
- Java de origem: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/templates/**` e `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/br/com/intv/adserver/business/component/impl/TemplateComponentImpl.java`.
- `docs/arquitetura/ARQUITETURA-ALVO.md` ADR-004 (go:embed + strings.Replacer).
- Issue [M8-01] (harness de golden tests que consumirá `tests/golden/ad/`).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M4-01] do ad-serverless conforme docs/issues/M4-01-templates-migracao.md e CLAUDE.md: copiar byte a byte os 44 templates .vm de c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/templates/ para internal/templates/assets/ (preservando subpastas msps/, ima/, programatica/ e mapeando adservers/space/** → space/**), SEM modificar conteúdo; versionar checksums SHA-256; adicionar .gitattributes -text; criar teste de inventário do embed.FS com a lista canônica dos 44 caminhos; preparar tests/golden/ad/ com FIXTURES.md e instruções de captura via curl no dev EC2 do legado. Código e testes 100% comentados em português, make lint && make test verdes, abrir PR feat/issue-M4-01-templates-migracao com Closes na issue.
```
