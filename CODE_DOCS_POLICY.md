# Política de Documentação do Projeto Ad-Serverless

## Regra Fundamental

**Todo o código deve ser documentado em português.**

Este projeto tem histórico de problemas com documentação insuficiente. A partir de agora, a documentação é obrigatória e não negociável.

## Requisitos de Documentação

### 1. Classes Públicas
Cada classe pública deve ter um bloco JavaDoc com:
- `@summary` — explicando a responsabilidade da classe no contexto do ad server
- Exemplo de uso quando aplicável

```java
/**
 * Serviço responsável pela geração e proxy de VAST XML.
 *
 * Este serviço é o mais crítico do sistema — serve respostas VAST para players de vídeo
 * em pontos WiFi (hotspots). Processa ~40% das requisições totais do ad server.
 *
 * @author Fabio Santos
 * @since 1.0
 */
public class VastService { ... }
```

### 2. Métodos Públicos
Cada método público deve ter:
- Descrição do que faz e POR QUÊ existe (não apenas o quê)
- `@param` para cada parâmetro com significado de negócio
- `@return` explicando o valor retornado
- `@throws` documentando quando e por que a exceção é lançada

```java
/**
 * Seleciona uma campanha elegível para o hotspot informado.
 *
 * Uma campanha é considerada elegível se:
 * - Está habilitada (enabled=true)
 * - Data atual está entre startDate e endDate
 * - Frequency cap não foi excedido para este usuário
 *
 * @param hotspotCode Código único do ponto WiFi (ex: "CLARO_WIFI")
 * @return Campanha selecionada aleatoriamente, ou NullCampaign se nenhuma for válida
 * @throws CampaignNotFoundException Se o hotspot não existir no banco de dados
 */
public Campaign eligeCampaign(String hotspotCode) { ... }
```

### 3. Blocos Complexos de Lógica
Comentários inline explicando o PORQUÊ, não apenas descrevendo o código:

```java
// Usar conditional write para atomicidade: incrementa o counter e verifica
// se ainda está dentro do limite em uma única operação no DynamoDB.
// Isso evita race conditions quando múltiplas instâncias Lambda processam
// requests simultâneos para a mesma campanha/usuário.
ddbClient.updateItem(req -> req...
```

### 4. Enums
Cada valor deve ser documentado com seu significado de negócio:

```java
/**
 * Tipos de eventos rastreados pelo ad server.
 */
public enum EventType {
    /** Visualização de página no hotspot WiFi */
    PAGE_VIEW,
    /** Impressão do criativo (banner ou vídeo) */
    IMPRESSION_PRE_ROLL,
    /** Clique na campanha publicitária */
    CLICK_CAMPAIGN,
    // ...
}
```

### 5. Interfaces
Documentar o contrato esperado:

```java
/**
 * Repositório para operações com campanhas.
 *
 * Implementações devem ser thread-safe pois são compartilhadas entre
 * múltiplas invocações Lambda dentro do mesmo container.
 */
public interface CampaignRepository {
    // ...
}
```

## Prioridade de Documentação

1. **Módulo commons** (entities + utilities) — usado por todos os serviços
2. **Handlers Lambda** (entry points de cada microserviço)
3. **Services e Components** (lógica de negócio)
4. **Config classes** (configurações Spring Boot, CDK constructs)
5. **Tests** (documentar o que está sendo testado e por quê)

## Ferramentas de Verificação

- JaCoCo para cobertura de testes (mínimo 80%)
- Checkstyle com regras de documentação obrigatória
- Code review com checklist de documentação em cada PR
