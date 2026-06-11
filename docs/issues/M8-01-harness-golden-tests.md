---
title: "[M8-01] Harness de golden tests + captura de fixtures do Java"
labels: ["epic:M8-qualidade", "tipo:teste", "prioridade:P1"]
milestone: "M8 — Qualidade"
---
## Contexto

A regra nº 1 do projeto é **paridade byte-a-byte** com o Java (CLAUDE.md; ADR-005). O mecanismo que prova isso são os **golden tests**: capturamos a saída REAL do ad-server Java no ambiente dev EC2, gravamos como fixture em `tests/golden/`, e o teste Go compara sua saída com a fixture após normalizar campos dinâmicos. Issues M4/M5 já criaram golden tests pontuais; esta issue entrega o **harness unificado** (runner, normalizadores, layout de fixtures, integração CI) e o **procedimento de captura completo e reprodutível** — qualquer pessoa (ou IA) deve conseguir recapturar fixtures seguindo este documento.

Ambiente de captura: EC2 dev `i-0267248b971ac7cd8` (us-east-1), Docker porta **91→8080** ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1). Endpoint base: `http://<IP-EC2-DEV>:91` (IP via console/SSM; NUNCA commitar IP/credenciais — usar env var `LEGACY_DEV_BASE_URL` no script de captura).

## Especificação detalhada

### 1. Layout de fixtures

```
tests/golden/
├── fixtures/
│   ├── ad/                  # <caso>.req.json (método, path, query, headers) + <caso>.golden (corpo) + <caso>.meta.json (status, content-type, headers relevantes)
│   ├── vast/{google,space,adforce,smartad,metrike,campaign-direct,hotspots}/
│   ├── redirect/
│   ├── proxy-audit/{adxspace,space,space-vpaid,metrike,cdn-rewrite,admotion,loadpixel}/
│   └── proxy-tracker/  safeframe/
├── runner.go                # carrega fixtures, executa handler Go in-process, normaliza, compara
├── normalize.go             # normalizadores (ver §3)
└── capture/capture.sh       # script de captura (curl) parametrizado por LEGACY_DEV_BASE_URL
```

### 2. Procedimento de captura (curls exatos — gravar request E resposta)

Todo curl salva corpo (`-o`), status e headers (`-D`). Template: `curl -sS -D "$caso.headers" -o "$caso.golden" "$LEGACY_DEV_BASE_URL/..."`.

**a) `/ad` — variar hid pelos CreativeTypes em uso (consultar `creatives`/`hotspots_campaigns` no MySQL dev para escolher hotspots que exercitem templates distintos):**
```bash
curl -sS -D ad/banner.headers -o ad/banner.golden "$LEGACY_DEV_BASE_URL/ad?hid=<HID_BANNER>"
curl -sS -D ad/video.headers  -o ad/video.golden  "$LEGACY_DEV_BASE_URL/ad?hid=<HID_VIDEO>"
curl -sS "$LEGACY_DEV_BASE_URL/ad?hid=<HID>|<OUTRO>"        # regra do pipe: usa a 1ª ocorrência
curl -sS "$LEGACY_DEV_BASE_URL/ad?hid=<HID>&red=https%3A%2F%2Fexemplo.com%2Flp"   # param red
curl -sS -D ad/404-vazio.headers "$LEGACY_DEV_BASE_URL/ad?hid="                    # 404
curl -sS -D ad/404-inexistente.headers "$LEGACY_DEV_BASE_URL/ad?hid=NAO_EXISTE"    # 404
```
⚠️ A seleção campanha/creative é ALEATÓRIA — para fixture determinística usar hotspot com exatamente 1 campanha/1 creative elegível em dev, ou capturar N vezes e fixar o par (documentar `cid`/creative no `.meta.json`).

**b) `/vast` — um conjunto por parceiro (vcurl = base64 da URL VAST do parceiro) + fluxos A e B:**
```bash
b64() { printf '%s' "$1" | base64 -w0; }
# Fluxo C por parceiro (capturar TAMBÉM o XML upstream cru para o teste de rewrite ser hermético):
curl -sS -o vast/google/resp.golden   "$LEGACY_DEV_BASE_URL/vast?vcurl=$(b64 'https://pubads.g.doubleclick.net/gampad/ads?...&output=vast')"
curl -sS -o vast/space/resp.golden    "$LEGACY_DEV_BASE_URL/vast?vcurl=$(b64 '<URL VAST 00px.net usada em prod>')"
curl -sS -o vast/adforce/resp.golden  "$LEGACY_DEV_BASE_URL/vast?vcurl=$(b64 '<URL VAST adftech.com.br>')"
curl -sS -o vast/smartad/resp.golden  "$LEGACY_DEV_BASE_URL/vast?vcurl=$(b64 'https://videoapi.smartadserver.com/ac?siteid=596893&...')"
curl -sS -o vast/metrike/resp.golden  "$LEGACY_DEV_BASE_URL/vast?vcurl=$(b64 'https://servedby.metrike.com.br/vast.spark?setID=61348')"
# GDPR passthrough:
curl -sS "$LEGACY_DEV_BASE_URL/vast?vcurl=...&gdpr=1&gdpr_consent=CO_TESTE&refOrigin=https%3A%2F%2Fplayer.exemplo.com"
# Fluxo A (campaign direct — cid com override válido em dev):
curl -sS -o vast/campaign-direct/resp.golden "$LEGACY_DEV_BASE_URL/vast?cid=<CID_OVERRIDE>&hid=TESTE"
# Fluxo B (TODOS os hotspots hardcoded):
for h in SESTSENAT_1 SESTSENAT_2 CLARO_WIFI CLARO_RECOMPENSAS CLARO_PREZAOFREE TV_COINS_TESTE OPOVO_PREROLL OPOVO_MIDROLL TVCULTURA_PREROLL TVCULTURA_MIDROLL INTELIFI_TEST; do
  curl -sS -o "vast/hotspots/$h.golden" "$LEGACY_DEV_BASE_URL/vast?hid=$h"; done
```
⚠️ Upstreams são vivos: as fixtures de Fluxo C devem guardar o PAR (XML upstream cru capturado direto do parceiro no mesmo instante + saída do legado); o teste Go alimenta o XML cru num servidor `httptest` e compara o rewrite — assim o golden não depende do parceiro estar no ar. ⚠️ `CLARO_WIFI` é random entre 4 opções — capturar até obter as 4 e gravar todas (`claro-wifi-1..4.golden`); o teste Go valida que a saída ∈ conjunto.

**c) `/redirect`:**
```bash
curl -sS -o redirect/plain.golden  "$LEGACY_DEV_BASE_URL/redirect?url=https%3A%2F%2Fexemplo.com%2Flp&cid=1&hid=TESTE"
curl -sS -o redirect/base64.golden "$LEGACY_DEV_BASE_URL/redirect?url=$(b64 'https://exemplo.com/lp')&enc=true&cid=1&hid=TESTE"
curl -sS -o redirect/placeholders.golden "$LEGACY_DEV_BASE_URL/redirect?url=$(b64 'https://ex.com/?s={source}&a={aff_sub}&c={click_id}')&enc=true&cid=1&hid=T&source=modatta&aff_sub=a1&click_id=c1"
curl -sS -D redirect/400-invalida.headers "$LEGACY_DEV_BASE_URL/redirect?url=notaurl&cid=1&hid=T"      # 400 "Invalid URL format"
curl -sS -D redirect/400-base64.headers "$LEGACY_DEV_BASE_URL/redirect?url=%%%inv&enc=true&cid=1&hid=T" # 400 "Invalid URL encoding"
```

**d) `/proxy-audit` — 1 fixture por família de rewrite JS ([03-pipeline-vast.md](../legado/03-pipeline-vast.md) §6.2):** capturar o JS upstream cru de cada família (a/ADXSPACE, b/Space 00px, c/Space VPAID, d/Metrike-AdButler, e/CDN rewrite metrike+adftech, f/Admotion, g/loadPixel) e a saída do legado: `curl -sS -o proxy-audit/metrike.golden "$LEGACY_DEV_BASE_URL/proxy-audit?src=$(b64 'https://servedby.metrike.com.br/app.js')&refOrigin=https%3A%2F%2Fplayer.exemplo.com"` (repetir por família; para famílias sem URL pública estável, criar JS sintético contendo o padrão literal, hospedar em host da whitelist de dev e capturar).

**e) `/proxy-tracker` e `/safeframe`:** casos `u` base64 e url-encoded, `type=js`, OPTIONS (headers CORS no `.meta.json`), detecção de VAST error (`/errors?`, `&error=`).

### 3. Normalização antes do diff (em `normalize.go`, por tipo de fixture)

- **Timestamps:** `time=\d{17}` → `time=<TS>`; ISO8601 → `<ISO>`.
- **Cachebusters:** `t=\d{13}` → `t=<CB>`; `%%CACHEBUSTER%%` expandido → `<CB>`; `ord=\d+`, `correlator=\d+` → placeholder.
- **UUIDs** → `<UUID>`; **IPs** → `<IP>`; hash MD5 de mídia em `/media/{md5}.mp4` → `<MD5>.mp4` (validar formato, não o valor).
- **Base64 dinâmico** em `u=`/`src=`: decodificar, normalizar a URL interna recursivamente, re-comparar decodificado.
- **Hosts de ambiente:** `ads.inteli.fi` × host de dev → `<BASE>`.
- XML: comparar após normalização de whitespace ENTRE tags apenas (conteúdo de CDATA é byte-exato pós-placeholders).

### 4. Runner e CI

- `runner.go`: descobre fixtures por glob, monta `events.APIGatewayV2HTTPRequest` do `.req.json`, invoca o handler Go in-process (upstreams mockados com o XML/JS cru gravado), normaliza ambos os lados, diff legível (linha a linha) em caso de falha. Flag `-update` proibida no CI.
- CI (`.github/workflows/ci.yml`): job `golden` rodando `go test ./tests/golden/... -v` em TODO PR; falha bloqueia merge.

## Arquivos a criar/alterar

- `tests/golden/runner.go`, `normalize.go`, `runner_test.go`, `normalize_test.go`
- `tests/golden/capture/capture.sh` + `tests/golden/capture/README.md` (procedimento §2 completo)
- `tests/golden/fixtures/**` (fixtures capturadas)
- `.github/workflows/ci.yml` (job golden em todo PR)
- `docs/MATRIZ-PARIDADE.md` (linhas E02/E04/E11/E13 etc. → ✅ com link p/ golden)

## Critérios de aceite

- [ ] `capture.sh` reproduz a captura completa só com `LEGACY_DEV_BASE_URL` definida; nenhum IP/segredo commitado
- [ ] Fixtures versionadas para: /ad (≥4 templates + pipe + red + 404s), /vast (5 parceiros + fluxo A + 11 hotspots hardcoded + GDPR), /redirect (5 casos), /proxy-audit (7 famílias), /proxy-tracker + /safeframe
- [ ] Fluxo C testado hermeticamente (XML upstream cru servido por httptest — sem rede no CI)
- [ ] Normalizadores com testes próprios (timestamp, cachebuster, UUID, base64 recursivo, host)
- [ ] CLARO_WIFI validado como pertencimento ao conjunto de 4 saídas
- [ ] CI roda golden em todo PR e o diff de falha é legível (evidência: PR com falha proposital)
- [ ] `tests/golden/capture/README.md` permite recaptura por qualquer pessoa sem conhecimento prévio
- [ ] MATRIZ-PARIDADE atualizada com links para os golden tests

## Dependências

Bloqueada por: M1-05 (template engine — handlers precisam existir para o runner invocar; na prática requer M4/M5 mergeadas para cobertura completa)

## Referências

- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) (parâmetros exatos por endpoint)
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §2–§6 (fluxos, bypasses, 7 famílias JS)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (EC2 dev porta 91)
- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) (regra de captura antes de portar)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M8-01] Harness de golden tests seguindo
docs/issues/M8-01-harness-golden-tests.md e CLAUDE.md. Criar
tests/golden/{runner.go,normalize.go,capture/capture.sh,capture/README.md}
com layout de fixtures da issue, normalizadores (timestamps 17 dígitos,
cachebusters, UUID, base64 recursivo, hosts) testados, runner in-process
com upstreams mockados (httptest) e diff legível, job golden no CI em todo
PR. Documentar a captura com os curls exatos da issue (env
LEGACY_DEV_BASE_URL, sem segredos). Atualizar MATRIZ-PARIDADE.
```
