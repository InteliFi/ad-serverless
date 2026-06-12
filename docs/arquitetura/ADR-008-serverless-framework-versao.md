# ADR-008 — Versão do Serverless Framework: v3 OSS (pinada) vs v4

- **Status:** aceito (M0-02, 2026-06-12)
- **Decisores:** time ad-serverless (decisão default da issue M0-02; revisão humana no PR)
- **Contexto de origem:** risco mapeado em [PLANO-MIGRACAO.md](../PLANO-MIGRACAO.md) §Riscos —
  "Serverless Framework v4 exigir licença → bloqueio de deploy → usar v3 OSS ou compose; decidir em M0".

## Contexto

O `serverless.yml` criado na issue M0-02 é a espinha dorsal de TODA a infraestrutura
do projeto (9 Lambdas + recursos do M2 serão adicionados a ele). A versão do
Serverless Framework precisa ser decidida **antes** do primeiro deploy porque:

1. **v4 mudou o modelo de licenciamento:** é gratuita apenas para organizações com
   receita anual abaixo de US$ 2 milhões. Acima disso, exige assinatura paga e
   autenticação no CLI (`SERVERLESS_LICENSE_KEY` ou login na Serverless Inc.) —
   inclusive no CI. Um deploy de emergência com licença expirada/indisponível
   seria bloqueado.
2. **v3 é OSS (licença MIT), sem exigência de chave/login**, porém está em modo
   manutenção desde o lançamento da v4 (sem features novas; correções esporádicas).
3. A escolha afeta diretamente **M0-03/M0-04** (instalação do framework no GitHub
   Actions) e qualquer automação futura de deploy.

## Decisão

**Usar Serverless Framework v3 OSS, série 3.40.x, pinada.**

- `package.json`: `"serverless": "~3.40.0"` (devDependency; `~` aceita apenas patches da 3.40)
- `serverless.yml`: `frameworkVersion: '3'`
- Execução sempre via `npx serverless` (binário local do projeto, nunca global),
  garantindo a mesma versão em qualquer máquina e no CI.

## Justificativa

1. **Zero risco de licença:** v3 é MIT. Não há verificação de receita, chave de
   licença nem login — nada que possa bloquear um deploy (local ou CI).
2. **O que o projeto usa da ferramenta é 100% coberto pela v3:** `provider: aws`,
   runtime `provided.al2023` + `arm64`, HTTP API com payload 2.0,
   `package.individually` com `artifact` pré-construído pelo Makefile, stages e
   recursos CloudFormation embutidos. Não usamos nenhuma feature exclusiva da v4
   (Compose v4, suporte a outros vendors, AI etc.).
3. **"Em manutenção" é risco aceitável aqui:** o framework só orquestra
   CloudFormation no momento do deploy — não roda em produção. O hot path é 100%
   Go + AWS. Se a v3 um dia quebrar com uma mudança de API da AWS, a migração
   (v4 com licença, OpenTofu/Terraform, SAM ou CDK) é uma troca de ferramenta de
   deploy, não de arquitetura.
4. **Reversibilidade barata:** o `serverless.yml` da v3 é amplamente compatível
   com a v4. Se o time confirmar elegibilidade (receita < US$ 2M) ou contratar
   licença, a migração é trocar o pin do `package.json`, atualizar
   `frameworkVersion` e configurar `SERVERLESS_LICENSE_KEY` no CI.

## Consequências

- **M0-03/M0-04 (CI):** instalar via `npm ci` (lockfile) e invocar `npx serverless`.
  NÃO usar `npm i -g serverless` (instalaria a v4 mais recente).
- A versão só muda por PR que atualize o pin no `package.json` (e este ADR, se
  mudar de major).
- Sem dashboard da Serverless Inc. (feature de conta v4) — observabilidade fica
  com CloudWatch (M7), que já era o plano.

## Revisão futura

Reavaliar este ADR se ocorrer QUALQUER um:
- v3 incompatível com uma mudança de API da AWS/CloudFormation necessária ao projeto;
- decisão de negócio de contratar a v4 (registrar elegibilidade/licença aqui);
- migração de IaC para outra ferramenta (exigiria novo ADR).
