---
title: "[M9-02] Shadow traffic + comparação automática"
labels: ["epic:M9-cutover", "tipo:infra", "prioridade:P1"]
milestone: "M9 — Cutover"
---
## Contexto

Os golden tests (M8-01) provam paridade para fixtures **escolhidas a dedo**. A fase *shadow* do cutover ([PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §"Fases de risco do cutover", item 1) prova paridade para o **tráfego real de produção**: pegamos os access logs das EC2 (coletados na M9-01), fazemos *replay* de cada request contra o ambiente legado de dev E contra o stage `dev` das Lambdas, e comparamos status, headers e corpo de forma automatizada. É a única forma de pegar divergências que nenhuma fixture sintética cobriu: URLs malformadas que o Tomcat relaxado aceitava, combinações de query params inéditas, User-Agents exóticos, encodings duplos.

A comparação direta byte-a-byte não funciona: as respostas contêm campos dinâmicos (timestamps `time=` de 17 dígitos, cachebusters `t=` de 13 dígitos, UUIDs, hosts de ambiente). Por isso o replay **reusa os normalizadores do harness de golden tests** (`tests/golden/normalize.go`, M8-01) antes do diff.

**Critério de saída desta fase (gate do canary M9-03): 0 divergências não explicadas.** Toda divergência restante precisa estar classificada e justificada por escrito (ex.: header `Apigw-Requestid` adicionado pelo API Gateway, ordem de headers, header `Date`).

## Especificação detalhada

### 1. Corpus de replay

1. Entrada: access logs de produção sanitizados coletados na M9-01 (≥ 7 dias, 2 instâncias prod). Ferramenta lê o formato de access log do Tomcat (common/combined — detectar pelo cabeçalho/shape da linha).
2. Construir o corpus em `tools/shadow/corpus/` (NÃO commitado — ver `.gitignore`): 1 arquivo JSONL com `{method, path, query, headers}` por request. Amostragem:
   - **100% das URLs distintas** (deduplicação por método+path+query normalizada) — garante cobertura de formas raras;
   - mais uma amostra aleatória de ≥ 100.000 requests brutos (preserva distribuição real por rota).
3. Incluir obrigatoriamente a amostra de URLs com caracteres relaxados do Tomcat separada na M9-01 (risco do [PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md): "API Gateway rejeitar URLs malformadas que o Tomcat relaxado aceitava").

### 2. Segurança do replay — rotas com efeito colateral

⚠️ O replay NUNCA pode disparar efeitos em produção de terceiros. Classificação por rota:

| Rota | Efeito colateral | Tratamento no replay |
|---|---|---|
| `GET /adtrack/postback` | chama `pb.modatta.org` / `api.prezaofree.com.br` REAIS ([01-endpoints-http.md](../legado/01-endpoints-http.md) §8) | **modo dry-run obrigatório**: replay somente com `source` sintético que NÃO casa `bW9kYXR0YQ`/`prezao_claro`, OU comparar apenas validação/4xx; flag `--allow-upstream` default `false` |
| `POST /adtrack`, `GET /vasttrack`, `GET /trackingpixel` | escreve eventos em MySQL/DynamoDB **de dev** | permitido (ambos os lados escrevem em dev); marcar eventos com `hid` de replay quando possível para limpeza |
| `GET /vast`, `GET /proxy-*`, `GET /media/*` | fetch de upstreams vivos (parceiros, CDNs) | permitido com rate limit (≤ 10 rps por upstream) — são GETs idempotentes; divergências por upstream vivo são classificadas como "ambiente" (ver §4) |
| `GET /ad`, `/GAM`, `/redirect`, `/health` | nenhum | permitido sem restrição |

### 3. Ferramenta de replay + diff (`tools/shadow/`)

4. CLI em Go (código comentado em português, CODE_DOCS_POLICY.md):
```
go run ./tools/shadow \
  -corpus tools/shadow/corpus/replay.jsonl \
  -legacy "$LEGACY_DEV_BASE_URL" \          # EC2 dev i-0267248b971ac7cd8, porta 91
  -target "$LAMBDA_DEV_BASE_URL" \          # API Gateway HTTP API, stage dev
  -rate 25 -concurrency 8 \
  -report tools/shadow/out/report.json
```
5. Para cada request do corpus: envia a MESMA request aos dois ambientes (mesmos headers relevantes: `User-Agent`, `Referer`, `Origin`, `Accept*`; `X-Forwarded-For` sintético fixo para os dois lados — o IP real influencia headers de upstream no VAST, [03-pipeline-vast.md](../legado/03-pipeline-vast.md) §4).
6. **Normalização antes do diff** — importar/reusar os normalizadores do M8-01 (`tests/golden/normalize.go`): timestamps `time=\d{17}` → `<TS>`, cachebusters `t=\d{13}`/`ord`/`correlator` → `<CB>`, UUIDs → `<UUID>`, IPs → `<IP>`, MD5 de mídia → `<MD5>.mp4`, base64 dinâmico decodificado e normalizado recursivamente, hosts de ambiente → `<BASE>`. Se necessário, extrair os normalizadores para pacote importável (`internal/goldenx` ou export no pacote `golden`) SEM duplicar código.
7. Comparar, por request: **status code**; **headers significativos** (allowlist: `Content-Type`, `Cache-Control`, `Location`, `Content-Disposition`, headers CORS — ignorar `Date`, `Server`, `Apigw-Requestid`, `X-Amzn-*`, `Via`, ordem); **corpo normalizado** (XML com normalização de whitespace entre tags, como no golden harness).
8. Repetição com upstream vivo: se uma divergência ocorre em rota com upstream (vast/proxy), re-executar o caso 3× — se o resultado oscila entre execuções no MESMO ambiente, classificar como "upstream dinâmico" automaticamente.

### 4. Relatório de divergências por rota

9. Saída `report.json` + render em markdown `docs/cutover/SHADOW-REPORT.md` (este sim, commitado), com:
   - totais por rota: requests, OK, divergência de status / header / corpo;
   - cada divergência única (agrupada por assinatura do diff) com: 1 exemplo de request reproduzível (curl), diff normalizado legível, e campo `classificacao`: `bug-go` (corrigir antes do canary), `bug-legado` (documentar — paridade mantém o comportamento do legado!), `ambiente` (dados de dev diferentes entre MySQL/Dynamo dos dois lados), `upstream-dinamico`, `infra-aceita` (ex.: headers do API Gateway) — com justificativa em português;
   - seção final "Gate do canary": `divergencias_nao_explicadas: 0` (ou lista do que falta).
10. Toda divergência `bug-go` vira correção nesta issue (se trivial) ou issue própria bloqueando o M9-03 (referenciar no relatório).

## Arquivos a criar/alterar

- `tools/shadow/main.go`, `tools/shadow/replay.go`, `tools/shadow/differ.go`, `tools/shadow/parser.go` (parser de access log), `tools/shadow/README.md`
- `tools/shadow/*_test.go` (unitários: parser de log, allowlist de headers, classificação automática)
- `tests/golden/normalize.go` — exportar normalizadores para reuso (sem quebrar os golden tests)
- `docs/cutover/SHADOW-REPORT.md` (relatório final commitado)
- `.gitignore` — `tools/shadow/corpus/` e `tools/shadow/out/`

## Critérios de aceite

- [ ] Parser lê os access logs reais coletados na M9-01 e gera o corpus JSONL (com teste unitário sobre linhas reais anonimizadas)
- [ ] Corpus cobre 100% das URLs distintas dos logs + amostra ≥ 100k requests, incluindo as URLs com caracteres relaxados do Tomcat
- [ ] Replay roda contra legado dev (porta 91) e stage dev das Lambdas com rate limit e concorrência configuráveis
- [ ] `GET /adtrack/postback` NUNCA dispara upstream real de parceiro no replay (teste provando o bloqueio de `bW9kYXR0YQ`/`prezao_claro` com `--allow-upstream=false`)
- [ ] Normalizadores reusados do harness M8-01 (zero duplicação; golden tests continuam verdes)
- [ ] Diff cobre status + headers da allowlist + corpo normalizado; diffs legíveis linha a linha
- [ ] `docs/cutover/SHADOW-REPORT.md` gerado com totais por rota e TODAS as divergências classificadas e justificadas em português
- [ ] **0 divergências não explicadas** (gate do M9-03); divergências `bug-go` corrigidas ou com issue bloqueante aberta e referenciada
- [ ] Nenhum log bruto/corpus commitado (`.gitignore` cobrindo `corpus/` e `out/`)
- [ ] `make lint && make test` verdes

## Dependências

Bloqueada por: [M8-01] (normalizadores e handlers invocáveis); insumos da [M9-01] (access logs coletados — se M9-01 ainda não tiver os logs, esta issue não inicia)

## Referências

- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — fase 1 do cutover (shadow) e risco das URLs malformadas
- [docs/issues/M8-01-harness-golden-tests.md](M8-01-harness-golden-tests.md) — normalizadores e layout de fixtures
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §8 (upstreams reais do postback — NUNCA chamar no replay)
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §4 (headers que influenciam a resposta)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (EC2 dev porta 91; EC2 prod fonte dos logs), §3 (chars relaxados do Tomcat)
- [docs/cutover/CONTRATOS.md](../cutover/CONTRATOS.md) (M9-01 — inventário de URLs reais)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M9-02] Shadow traffic + comparação automática
seguindo docs/issues/M9-02-shadow-traffic.md e CLAUDE.md. Criar
tools/shadow/ (parser de access log, replay duplo legado dev × stage dev,
diff com normalizadores REUSADOS de tests/golden/normalize.go, relatório
por rota) com código 100% comentado em português e testes unitários
verdes, garantindo que /adtrack/postback jamais chama modatta/prezao
reais no replay. Gerar docs/cutover/SHADOW-REPORT.md com todas as
divergências classificadas e o gate "0 divergências não explicadas".
make lint && make test verdes. Ao final: abrir PR referenciando a issue.
```
