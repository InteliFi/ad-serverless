---
title: "[M8-05] Security review completo"
labels: ["epic:M8-qualidade", "tipo:seguranca", "prioridade:P1"]
milestone: "M8 — Qualidade"
---
## Contexto

O sistema legado acumulou problemas graves de segurança: chaves AWS hardcoded e compartilhadas entre ambientes, senhas de banco no repositório, signature key exposta ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2). Antes do cutover (M9), uma revisão de segurança formal confirma que a nova arquitetura fecha cada lacuna e que nenhuma regressão foi introduzida.

## Especificação detalhada

Checklist executável — cada item com evidência anexada ao relatório:

### Segredos e identidade
- [ ] Confirmação da rotação das chaves AWS expostas ([M0-05] concluída: chave antiga desativada, monitorada e deletada)
- [ ] Varredura **gitleaks** no repositório completo (histórico incluso) integrada ao CI — zero findings
- [ ] Nenhum segredo em env vars do serverless.yml (apenas referências SSM); nenhum segredo em logs (amostragem de CloudWatch)
- [ ] OIDC do GitHub Actions com trust policy restrita ao repo/branch (sem chaves estáticas no CI)

### IAM e infraestrutura
- [ ] Roles por função sem wildcard de recurso ([M2-04]) — validação com IAM Access Analyzer, zero findings de acesso não intencional
- [ ] Buckets S3 privados com OAC (acesso só via CloudFront); SSM parameters como SecureString
- [ ] WAF promovido COUNT→BLOCK conforme análise de [M2-06] (ou justificativa documentada do que permaneceu em COUNT)
- [ ] Security groups do RDS Proxy: ingress apenas das Lambdas (⚠️ sem afetar o acesso dos outros projetos ao RDS — banco compartilhado)

### Aplicação
- [ ] Middleware de validação portado ([M1-07]) com testes cobrindo TODOS os padrões de ataque do legado (PHP/SQL/command injection, encodados proibidos, params perigosos — [docs/legado/01](../legado/01-endpoints-http.md) §16)
- [ ] **govulncheck** no CI — zero vulnerabilidades conhecidas nas dependências
- [ ] Proxies (proxy-tracker/audit/safeframe): validação de protocolo http/https, rejeição de file://, whitelist de hosts do proxy-audit ativa em prod (testes de SSRF: metadata endpoint 169.254.169.254 bloqueado — adicionar à validação se o legado não cobre, como melhoria de segurança JUSTIFICADA)
- [ ] CORS: reflexão de origin documentada como comportamento exigido pelo negócio (players cross-origin); avaliar e registrar o risco aceito
- [ ] Headers de resposta: sem vazamento de stack traces/versões em erros 4xx/5xx

### Relatório
Consolidar em `docs/security/RELATORIO.md`: item × evidência × status × risco residual aceito (com aprovador). Itens reprovados viram issues `tipo:seguranca` bloqueantes do cutover.

## Arquivos a criar/alterar

- `docs/security/RELATORIO.md`
- `.github/workflows/ci.yml` — jobs gitleaks + govulncheck (se ainda ausentes)
- Issues novas para findings (se houver)

## Critérios de aceite

- [ ] Checklist 100% executado com evidências
- [ ] gitleaks + govulncheck rodando no CI em todo PR
- [ ] Teste de SSRF nos 3 proxies documentado
- [ ] Zero findings bloqueantes abertos ao final (ou aceitos formalmente no relatório)

## Dependências

Bloqueada por: epics M2 e M3 implantados

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §2
- [docs/legado/01-endpoints-http.md](../legado/01-endpoints-http.md) §16
- Issues [M0-05], [M2-04], [M2-06], [M1-07]

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M8-05] Security review seguindo docs/issues/M8-05-security-review.md e CLAUDE.md. Executar o checklist completo com evidências, integrar gitleaks e govulncheck ao CI, produzir docs/security/RELATORIO.md e abrir issues para findings. Abrir PR ao final.
```
