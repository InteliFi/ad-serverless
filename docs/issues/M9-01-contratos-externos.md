---
title: "[M9-01] Verificação de contratos externos (Location header, consumidores)"
labels: ["epic:M9-cutover", "tipo:decisao", "prioridade:P1"]
milestone: "M9 — Cutover"
---
## Contexto

Antes de QUALQUER canary (M9-03), precisamos provar que nenhum sistema externo quebra silenciosamente com a troca EC2→Lambda. O [PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) lista explicitamente o risco: *"Header `Location` de /adtrack com ID consumido por alguém → quebra silenciosa → issue de verificação nos consumidores antes do canary de tracking"*. Esta é essa issue.

São **3 contratos externos** a verificar:

1. **Header `Location` do `POST /adtrack`.** O legado responde `201 Created` + `Location: /adtrack/{id}`, onde `{id}` é a PK auto-increment do MySQL `ad_trackers` ([docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §5). Na arquitetura nova, o `track-handler` publica no SQS e responde `201` imediatamente — o ID do MySQL ainda não existe nesse momento, então o `Location` passa a usar **UUID** ([ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3, decisão "tracking assíncrono"). Se algum consumidor parseia esse ID (ex.: para correlacionar eventos), ele quebra sem erro visível.
2. **Quem chama `ads.inteli.fi` e com quais URLs exatas.** Os testes até aqui usaram fixtures sintéticas e capturas pontuais; o cutover precisa do inventário REAL de chamadores (players, apps, parceiros, scripts server-to-server) extraído dos access logs de produção.
3. **Allowlist de IP de origem nos parceiros de postback/VAST.** Hoje as chamadas de saída (modatta, prezao, SmartAdServer) partem dos IPs das EC2 de produção. Nas Lambdas, o tráfego de saída sai pelos IPs do **NAT Gateway** (ou IPs dinâmicos da Lambda, se fora de VPC) — se algum parceiro filtra por IP de origem, o postback passa a falhar silenciosamente no canary.

Esta issue é **investigativa/decisória** (`tipo:decisao`): não produz código Go, produz o documento `docs/cutover/CONTRATOS.md` com evidências e decisões que liberam (ou bloqueiam) o canary. **Não depende de nada — pode (e deve) rodar cedo, em paralelo a M1–M8.**

## Especificação detalhada

### 1. Verificação do header `Location` de `POST /adtrack`

1. **Buscar consumidores no código da empresa:** grep nos demais repositórios da organização InteliFi (via `gh search code` ou clone local) pelos padrões:
   - `adtrack` (chamadas POST e leitura de resposta);
   - `Location` próximo a chamadas para `ads.inteli.fi`;
   - `/adtrack/` seguido de interpolação de ID.
   Documentar CADA repositório verificado (mesmo os negativos) com data e método de busca.
2. **Verificar os templates/JS do próprio ad-server:** o JS gerado por `/redirect` faz `POST /adtrack` via XHR ([01-endpoints-http.md](../legado/01-endpoints-http.md) §11) e os templates `.vm` disparam tracking — confirmar (em `c:/Users/Fabio/Documents/Dev/ad-server/src/main/resources/templates/` e nos JS dos templates) que NENHUM lê o header `Location` da resposta. Registrar os trechos verificados.
3. **Constatação a registrar:** no legado NÃO existe rota `GET /adtrack/{id}` — `GET /adtrack` é o relatório agregado ([01-endpoints-http.md](../legado/01-endpoints-http.md) §6). Ou seja, a URL do `Location` nunca foi "seguível"; se ninguém parseia o ID, trocar para UUID é seguro. Formalizar essa decisão (manter UUID OU, se um consumidor for encontrado, abrir issue para preservar o contrato).

### 2. Inventário de chamadores de `ads.inteli.fi` (access logs das EC2 de produção)

4. Fonte: as 2 EC2 de produção `i-030bd120418d71a9d` e `i-0707c9d77d0420be3` em **sa-east-1**, Docker porta 80→8080 ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1).
   - ⚠️ O logging do legado é nível ERROR, apenas stdout ([05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3) — **verificar primeiro se access log existe** (`docker logs`, access log do Tomcat dentro do container, logs de ALB/CloudFront se houver camada na frente). Se não existir, habilitar temporariamente o access log do Tomcat (`server.tomcat.accesslog.enabled=true` via env/properties + redeploy coordenado) e coletar **≥ 7 dias corridos** (cobrir fim de semana — sazonalidade).
5. Extrair dos logs e tabular por rota (as 16 rotas de [01-endpoints-http.md](../legado/01-endpoints-http.md) §1):
   - volume por rota/dia e por hora (baseline para o M9-04);
   - **URLs exatas** mais frequentes (com query strings) — entrada para o replay do M9-02 e para o risco "API Gateway rejeitar URLs malformadas que o Tomcat relaxado aceitava" ([PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md), riscos);
   - `User-Agent` distintos (players, SDKs, bots, chamadas server-to-server);
   - `Referer`/`Origin` distintos (sites parceiros que embedam os scripts);
   - IPs de origem recorrentes de chamadas server-to-server (candidatos a parceiros que precisariam ser avisados);
   - amostras de URLs com caracteres relaxados (`| { } [ ] ^ < >` etc., [05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3) para o teste F03 da [MATRIZ-PARIDADE](../MATRIZ-PARIDADE.md).
6. Guardar os logs brutos sanitizados (sem IPs de usuários finais — anonimizar último octeto) em local interno (S3 privado), NUNCA commitados.

### 3. Confirmação de allowlist de IP com parceiros

7. Upstreams de saída em produção (URLs literais do legado, [01-endpoints-http.md](../legado/01-endpoints-http.md) §8 e [03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3):
   - **modatta:** `GET https://pb.modatta.org/external/affiliates/pb/9A061369-320A-4C1E-9451-E1BA9991E193?modid={click_id}` (source `bW9kYXR0YQ`);
   - **prezao:** `GET https://api.prezaofree.com.br/event/postback?partnerId=425701220616215160&event=register&clickId={click_id}` (source `prezao_claro`);
   - **SmartAdServer:** `https://videoapi.smartadserver.com/ac?siteid=596893&...` (hotspots `CLARO_RECOMPENSAS`, `CLARO_PREZAOFREE`, `OPOVO_*`, `TVCULTURA_*`).
8. Perguntar formalmente a cada parceiro (e registrar a resposta com data/contato): *"Existe allowlist de IP de origem para as chamadas que recebemos de ads.inteli.fi? Os IPs de saída vão mudar de <IPs das EC2> para <IPs do NAT Gateway das Lambdas>."*
   - Levantar antes os IPs atuais de saída das EC2 (`curl ifconfig.me` de dentro de cada instância ou Elastic IPs no console) e os IPs do NAT Gateway do stack serverless (output do CloudFormation/serverless info).
   - Se houver allowlist: solicitar inclusão dos novos IPs ANTES do degrau de canary da rota correspondente (postback → modatta/prezao; vast → SmartAdServer) e registrar a confirmação.
9. Verificar também upstreams secundários que recebem fetch das Lambdas: hosts da whitelist do proxy-audit (`cdn.00px.net`, `cdn.vendor.com`, `admotion.digital`, `servedby.metrike.com.br`, `nsp.admotion.digital` — [05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §3) e domínios do video cache (`gcdn.2mdn.net`, `googlevideo.com`). Para esses, basta registrar que são CDNs públicas (sem allowlist) ou tratar como os parceiros acima.

### 4. Entregável: `docs/cutover/CONTRATOS.md`

Estrutura mínima do documento (em português):

```markdown
# Contratos Externos — Verificação pré-cutover
## 1. Header Location de POST /adtrack
   (repos verificados, evidências, DECISÃO: UUID ok / contrato a preservar)
## 2. Inventário de chamadores de ads.inteli.fi
   (período coletado, volume por rota, UAs/Referers/IPs server-to-server, URLs exóticas)
## 3. Allowlist de IP nos parceiros
   (modatta / prezao / SmartAdServer: resposta, data, contato, ação)
## 4. Riscos bloqueantes para o canary
   (lista objetiva: o que precisa estar resolvido antes de cada rota do M9-03)
## 5. Sign-off
   (quem aprovou, data)
```

## Arquivos a criar/alterar

- `docs/cutover/CONTRATOS.md` (entregável principal — criar diretório `docs/cutover/`)
- NENHUM código Go nesta issue
- NENHUM log bruto ou IP de usuário final commitado (somente agregados/anonimizados no documento)

## Critérios de aceite

- [ ] Todos os repositórios da organização verificados quanto a consumo do header `Location` (lista nominal, incl. negativos, com método de busca)
- [ ] Templates `.vm` e JS do `/redirect` do legado verificados (nenhuma leitura de `Location` — ou consumidor documentado com plano)
- [ ] Decisão registrada: `Location: /adtrack/{uuid}` aprovado OU issue de preservação de contrato aberta
- [ ] Access logs de produção coletados por ≥ 7 dias corridos das instâncias `i-030bd120418d71a9d` e `i-0707c9d77d0420be3` (ou habilitação documentada + coleta)
- [ ] Inventário por rota: volume, URLs exatas top-N, User-Agents, Referers/Origins, IPs server-to-server
- [ ] Amostra de URLs com caracteres relaxados do Tomcat separada para o M9-02/F03
- [ ] Resposta formal de modatta, prezao e SmartAdServer sobre allowlist de IP registrada (data + contato)
- [ ] IPs de saída atuais (EC2) e futuros (NAT Gateway) documentados
- [ ] `docs/cutover/CONTRATOS.md` completo com a seção "Riscos bloqueantes para o canary" e sign-off
- [ ] Nenhum dado sensível (logs brutos, IPs de usuários) commitado

## Dependências

Bloqueada por: nenhuma (executar cedo — o M9-03 NÃO inicia o canary de tracking/postback sem as confirmações desta issue)

## Referências

- [docs/PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) — riscos "Header Location de /adtrack" e "API Gateway rejeitar URLs malformadas"
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §3 — decisão do tracking assíncrono (Location com UUID)
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §5 (POST /adtrack), §6 (GET /adtrack é relatório), §8 (URLs literais modatta/prezao), §11 (/redirect)
- [docs/legado/03-pipeline-vast.md](../legado/03-pipeline-vast.md) §3 (hotspots SmartAdServer)
- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (EC2 prod sa-east-1), §3 (logging ERROR-only, proxy-audit whitelist, video cache whitelist)
- Legado: `c:/Users/Fabio/Documents/Dev/ad-server/src/main/java/.../AdTrackService.java` (geração do Location), `src/main/resources/templates/` (JS dos templates)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M9-01] Verificação de contratos externos
seguindo docs/issues/M9-01-contratos-externos.md e CLAUDE.md. Executar
as 3 verificações NA ORDEM (consumidores do Location header nos repos
da organização e nos templates do legado; inventário de chamadores via
access logs das EC2 prod i-030bd120418d71a9d e i-0707c9d77d0420be3;
confirmação de allowlist de IP com modatta/prezao/SmartAdServer) e
produzir docs/cutover/CONTRATOS.md em português com evidências,
decisões e a lista de riscos bloqueantes para o canary. Itens que
exigem ação humana (contato com parceiros, acesso SSH às EC2) devem
ser listados como pendências explícitas no documento. Sem código Go,
sem dados sensíveis commitados. Ao final: abrir PR referenciando a
issue.
```
