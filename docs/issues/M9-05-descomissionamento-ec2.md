---
title: "[M9-05] Descomissionamento das EC2 + arquivamento"
labels: ["epic:M9-cutover", "tipo:infra", "prioridade:P1"]
milestone: "M9 — Cutover"
---
## Contexto

Com 2 semanas de operação assistida limpa ([M9-04]), o legado é desligado de forma **reversível primeiro, definitiva depois**. As 3 EC2 ([docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1: dev `i-0267248b971ac7cd8` us-east-1; prod `i-030bd120418d71a9d` e `i-0707c9d77d0420be3` sa-east-1) saem de operação e os repositórios Java entram em modo arquivo.

## Especificação detalhada

### Sequência (cada passo com validação antes do próximo)

1. **DNS/CDN definitivo**: remover o origin legado do CloudFront/Route53 (pesos já a 100% nas Lambdas); aguardar 48h monitorando que NENHUM tráfego chega às EC2 (access logs zerados).
2. **Stop (não terminate) das EC2** + criação de AMI/snapshot de cada uma; etiquetar com data e motivo. Janela de arrependimento: **30 dias** paradas (custo só de EBS).
3. ⚠️ **NÃO desligar o RDS MySQL** — continua servindo as Lambdas e os outros projetos (banco compartilhado; destino é decisão do Epic M10).
4. **Chaves AWS antigas**: executar a fase final da sequência de [M0-05] — confirmar 30 dias sem uso da chave antiga (CloudTrail), desativar → 7 dias → deletar.
5. **Arquivamento dos repositórios** [ad-server](https://github.com/InteliFi/ad-server) e [ad-commons](https://github.com/InteliFi/ad-commons): commit final no README apontando para o ad-serverless ("Projeto migrado — ver github.com/InteliFi/ad-serverless"), depois Archive no GitHub (somente leitura). ⚠️ Antes: confirmar que nenhum pipeline/dependência externa ainda consome o ad-commons como biblioteca (verificação de [M9-01]).
6. **Limpeza**: security groups/EIPs/ALBs órfãos do legado (inventariar antes de deletar); manter os access logs históricos das EC2 em S3 (compactados) por 1 ano.
7. **Encerramento**: atualizar [docs/MATRIZ-PARIDADE.md](../MATRIZ-PARIDADE.md) (todas as linhas ✅), registrar no LOG.md a data de descomissionamento e terminar as EC2 após os 30 dias.

## Arquivos a criar/alterar

- `docs/cutover/DESCOMISSIONAMENTO.md` — checklist executado com datas e evidências
- READMEs finais nos repos ad-server e ad-commons
- `docs/MATRIZ-PARIDADE.md` — fechamento

## Critérios de aceite

- [ ] 48h sem tráfego nas EC2 antes do stop (evidência de logs)
- [ ] AMIs/snapshots criados e etiquetados; instâncias paradas 30 dias antes do terminate
- [ ] RDS intocado e funcional para os outros projetos
- [ ] Chaves antigas deletadas após janela do CloudTrail
- [ ] Repos Java arquivados com README de redirecionamento
- [ ] Matriz de paridade 100% ✅ e DESCOMISSIONAMENTO.md completo

## Dependências

Bloqueada por: [M9-04]

## Referências

- [docs/legado/05-config-infra-deploy.md](../legado/05-config-infra-deploy.md) §1 (instâncias e topologia)
- Issues [M0-05] (sequência segura de chaves), [M9-01] (consumidores do ad-commons), [M10-01] (banco fica)

## Comando sugerido (Claude Code)

```
/goal Implementar a issue [M9-05] Descomissionamento seguindo docs/issues/M9-05-descomissionamento-ec2.md e CLAUDE.md. Executar a sequência reversível (DNS→stop+AMI→chaves→arquivamento→limpeza) com validação por passo e evidências no DESCOMISSIONAMENTO.md. Ações destrutivas exigem confirmação humana. Abrir PR ao final.
```
