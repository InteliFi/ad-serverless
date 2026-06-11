---
title: "[M1-03] internal/cache: cache TTL em memória (semântica Caffeine)"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P1"]
milestone: "M1 — Commons Go"
---
## Contexto

O legado usa Caffeine via `@Cacheable` em dois pontos do hot path: cache `hotspots` (busca por code no `/ad` e `/vast`) e cache `campaignVastOverride` (override de VAST por campanha) — ambos com `expireAfterWrite` de **5 minutos** e `maximumSize` **500**. Em Lambda, containers reusam memória entre invocações, então um cache em memória por container replica esse comportamento com custo zero (decisão da ARQUITETURA-ALVO §1). Esta issue cria o cache genérico reutilizável em `internal/cache`.

## Especificação detalhada

### Semântica (paridade com Caffeine)

- **expireAfterWrite:** a entrada expira `ttl` após o `Set` (não renova no `Get` — NÃO é expireAfterAccess).
- **maximumSize:** ao atingir o limite, novas inserções provocam descarte. Caffeine usa W-TinyLFU; aqui é aceitável **LRU simples OU descarte de uma entrada qualquer/mais antiga** — documentar a escolha no godoc. O ponto inegociável é nunca crescer sem limite (memória de Lambda é 128–512MB).
- **Eviction lazy na leitura:** `Get` de entrada expirada retorna miss e remove a entrada (sem goroutine de limpeza em background — goroutines não sobrevivem ao freeze do container, ver ADR-003).

### API pública (Go generics)

```go
// New cria um cache com TTL (expireAfterWrite) e tamanho máximo.
func New[T any](ttl time.Duration, maxSize int) *Cache[T]

// Get retorna o valor e true em hit válido (não expirado); zero value e false caso contrário.
func (c *Cache[T]) Get(key string) (T, bool)

// Set grava/sobrescreve o valor, registrando o instante de escrita para o TTL.
func (c *Cache[T]) Set(key string, value T)

// GetOrLoad retorna o valor em hit válido; em miss/expiração executa loader,
// grava o resultado (Set) e o retorna. Erro do loader é propagado e NÃO é
// cacheado (próxima chamada tenta de novo) — paridade com a semântica do
// @Cacheable do Spring (exceção não popula o cache).
func (c *Cache[T]) GetOrLoad(key string, loader func() (T, error)) (T, error)
```

- Thread-safe: `sync.RWMutex` (preferido — permite leituras concorrentes no hot path) ou `sync.Map`; justificar a escolha no godoc.
- O relógio interno deve ser injetável (campo `now func() time.Time` privado, default `time.Now`) para testar expiração sem `time.Sleep`.
- Defaults de conveniência exportados: `TTLPadrao = 5 * time.Minute`, `TamanhoPadrao = 500` (paridade com a config Caffeine do legado: `expireAfterWrite=5m, maximumSize=500`).

### Comportamentos a testar

1. Hit antes do TTL; miss exatamente após o TTL (avançar o clock injetado).
2. `Set` sobrescrevendo renova o TTL da chave.
3. Estouro de `maxSize`: inserir `maxSize+1` chaves → tamanho interno nunca excede `maxSize`.
4. Concorrência: `go test -race` com N goroutines fazendo Get/Set simultâneos (sem data race).
5. `Get` de chave inexistente retorna zero value e `false`.
6. Eviction lazy: entrada expirada é removida do mapa no `Get` (verificar tamanho interno).
7. `GetOrLoad`: hit não chama o loader; miss chama 1× e popula o cache; erro do loader é propagado e a chave NÃO entra no cache.

### Benchmarks básicos

`BenchmarkGet_Hit`, `BenchmarkGet_Miss`, `BenchmarkSet` — apenas para garantir que não há contention patológica (alvo: Get hit < 200ns em paralelo). Não otimizar prematuramente.

## Arquivos a criar/alterar

- `internal/cache/doc.go` — godoc: por que cache por container substitui o Caffeine (`// Portado de: configuração Caffeine do CacheConfig.java — expireAfterWrite 5min, maximumSize 500`).
- `internal/cache/cache.go` — implementação genérica.
- `internal/cache/cache_test.go` — testes da lista acima.
- `internal/cache/cache_bench_test.go` — benchmarks.

## Critérios de aceite

- [ ] API exata: `New[T](ttl, maxSize)`, `Get(key) (T, bool)`, `Set(key, T)`, `GetOrLoad(key, loader) (T, error)`; genérico via type parameter.
- [ ] `GetOrLoad` testado: hit não invoca o loader; erro do loader não é cacheado.
- [ ] Semântica expireAfterWrite (TTL conta do Set; Get não renova) testada com clock injetado.
- [ ] Tamanho nunca excede `maxSize` (LRU simples ou descarte documentado no godoc).
- [ ] Eviction lazy no Get (sem goroutines de background).
- [ ] `go test -race ./internal/cache/` verde (teste de concorrência incluído).
- [ ] Constantes `TTLPadrao` (5min) e `TamanhoPadrao` (500) exportadas e documentadas com referência ao legado.
- [ ] Benchmarks compilam e rodam (`go test -bench=. ./internal/cache/`).
- [ ] `make lint && make test` verdes; doc comments em português em todo símbolo exportado.

## Dependências

Bloqueada por: M0-01 (bootstrap do repositório Go).
Bloqueia: M3-01 (cache de hotspots nas queries), M5-05 (cache campaignVastOverride).

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.1 (cache `hotspots` 5min/500), §1.11 (cache `campaignVastOverride` 5min/500), §5 (tabela de padrões transversais).
- [docs/arquitetura/ARQUITETURA-ALVO.md](../arquitetura/ARQUITETURA-ALVO.md) §1 (linha "Cache de dados") e §4 (`internal/cache`).
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md).

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-03 (internal/cache) do ad-serverless: cache genérico em memória com semântica Caffeine expireAfterWrite (TTL 5min default) e maximumSize (500 default), API New[T](ttl, maxSize) / Get(key) (T, bool) / Set(key, T) / GetOrLoad(key, loader) (T, error) sem cachear erro do loader, thread-safe com sync.RWMutex, eviction lazy na leitura, limite por LRU simples documentado, clock injetável para testes. Seguir docs/issues/M1-03-cache-ttl.md: testes com -race, benchmarks básicos, código 100% comentado em português com referência à config Caffeine do legado, make lint && make test verdes, abrir PR feat/issue-M1-03-cache-ttl com Closes na issue.
```
