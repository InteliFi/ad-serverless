---
title: "[M1-02] internal/frequencycap: parser de ranges + elegibilidade"
labels: ["epic:M1-commons", "tipo:port", "prioridade:P1"]
milestone: "M1 — Commons Go"
---
## Contexto

O frequency cap é o mecanismo que restringe QUANDO uma campanha pode ser veiculada (horas do dia e dias da semana). É avaliado em TODO request de `/ad` e na validação de campanha ativa do postback — algoritmo ⚠️ CRÍTICO com paridade obrigatória. O formato é uma mini-linguagem de ranges armazenada como string nas colunas `campaigns.hour_cap` e `campaigns.weekday_cap` (ex.: `"0;5>10;15>>"`). Esta issue porta o `DigitExtractor` e o `FrequencyCapComponentImpl` do Java para `internal/frequencycap`, de forma EXATA.

## Especificação detalhada

### Parser `DigitExtractor` — port EXATO

Gramática da string (separador de fragmentos `;`):

| Fragmento | Semântica | Exemplo |
|---|---|---|
| `"5"` | dígito único | `{5}` |
| `"5>10"` | range **exclusivo** no max (min até max−1; valida min<=max) | `{5,6,7,8,9}` |
| `"15>>"` | range **inclusivo** com `>>`; sem max = vai até o limite do domínio | horas: `{15..23}` |
| `"15>>20"` | range inclusivo com max explícito | `{15..20}` |
| inválido | `NullDigit` — conjunto vazio, **ignorado silenciosamente** (sem erro, sem log de erro) | `"abc"` → `{}` |

Exemplo completo (horas): `"0;5>10;15>>"` → `{0, 5, 6, 7, 8, 9, 15, 16, 17, 18, 19, 20, 21, 22, 23}`.

Domínios:
- **Hours:** 0–23 (limite superior do `>>` sem max = 23).
- **Weekdays:** 1–7, padrão ISO: 1=SEG, 2=TER, 3=QUA, 4=QUI, 5=SEX, 6=SÁB, 7=DOM (igual a `time.Weekday` NÃO é — em Go, `time.Sunday == 0`; converter com `int(t.Weekday())` ajustado: domingo → 7).

### Regra de elegibilidade `IsEligibleFor(t time.Time, cap FrequencyCap) bool`

Port de `FrequencyCapComponentImpl.isEligibleFor(dateTime, frequencyCap)`:

```
elegível = (hours vazio OU hours.contains(hora(t)))
         E (weekdays vazio OU weekdays.contains(diaISO(t)))
```

- String vazia ou NULL no banco = conjunto vazio = **sem restrição** (sempre elegível naquela dimensão).
- ⚠️ Timezone: o `t` avaliado DEVE estar em **America/Sao_Paulo** explícito (`time.LoadLocation`), nunca UTC do Lambda. O chamador injeta o `time.Time` (regra do CLAUDE.md: nunca `time.Now()` direto na lógica — injeção de clock).

### Campos reservados SEM lógica (documentar, NÃO implementar)

`campaigns.event_cap`, `event_cap_limit`, `event_cap_hours_limit` existem nas colunas desde a V11 mas **não têm nenhuma lógica no legado**. Registrar isso no godoc do pacote para evitar que alguém "complete" a feature por engano (seria `melhoria`, nunca `tipo:port`).

### Tabela de casos de teste OBRIGATÓRIOS

| Caso | Entrada | Esperado |
|---|---|---|
| Range misto (horas) | `"0;5>10;15>>"` | `{0, 5..9, 15..23}` |
| Range exclusivo (weekdays) | `"1;3>5"` | `{1, 3, 4}` (SEG, QUA, QUI) |
| String vazia | `""` | conjunto vazio → sem restrição (elegível) |
| NULL no banco | `sql.NullString{Valid:false}` | idem acima |
| Range invertido | `"5>3"` | inválido → `{}` (ignorado em silêncio) |
| Fragmento lixo | `"abc;7"` | `{7}` (só o fragmento válido) |
| Hora limite | `"15>>"` às 23h00 (São Paulo) | elegível; `"5>10"` às 10h00 | NÃO elegível |
| Combinação E | hora ok + weekday fora | NÃO elegível (as duas dimensões precisam passar) |
| Domingo ISO | `t` = domingo, cap `"7"` | elegível (domingo = 7, não 0) |

## Arquivos a criar/alterar

- `internal/frequencycap/doc.go` — godoc do pacote (usar o exemplo do CODE_DOCS_POLICY.md §1 como base).
- `internal/frequencycap/digitextractor.go` — parser dos fragmentos (`// Portado de: DigitExtractor.java`).
- `internal/frequencycap/frequencycap.go` — tipo `FrequencyCap{HourCap, WeekdayCap string}` + `IsEligibleFor` (`// Portado de: FrequencyCapComponentImpl.java`).
- `internal/frequencycap/frequencycap_test.go` — TODOS os casos da tabela acima, nomeados por comportamento (ex.: `TestDigitExtractor_RangeExclusivo`).

## Critérios de aceite

- [ ] Parser reproduz EXATAMENTE a semântica: `;` separa, `>` exclusivo, `>>` inclusivo (sem max = limite do domínio), fragmento inválido ignorado silenciosamente.
- [ ] Domínio Hours = 0–23; Weekdays = 1–7 ISO com conversão correta de `time.Weekday` (domingo → 7) testada.
- [ ] `IsEligibleFor` implementa `(hours vazio OU contém) E (weekdays vazio OU contém)`.
- [ ] Timezone America/Sao_Paulo explícito nos testes; nenhuma chamada a `time.Now()` dentro do pacote.
- [ ] Godoc do pacote documenta os campos `event_cap*` como reservados sem lógica.
- [ ] Todos os casos da tabela de testes implementados e verdes; `make lint && make test` ok.
- [ ] Comentários `// Portado de: DigitExtractor.java` / `FrequencyCapComponentImpl.java` presentes.

## Dependências

Bloqueada por: M1-01 (structs de domínio).
Bloqueia: M1-04 (seleção), M3-07 (postback usa `campaignExistsAndIsActive`).

## Referências

- [docs/legado/02-logica-negocio.md](../legado/02-logica-negocio.md) §1.3 (algoritmo completo).
- [docs/legado/04-modelo-dados.md](../legado/04-modelo-dados.md) §5 (pseudocódigo do DigitExtractor) e §2.1 (colunas hour_cap/weekday_cap).
- [CODE_DOCS_POLICY.md](../../CODE_DOCS_POLICY.md) §1 e §7.

## Comando sugerido (Claude Code)

```
/goal Implementar a issue M1-02 (internal/frequencycap) do ad-serverless: portar EXATAMENTE o DigitExtractor.java (separador ";", range exclusivo ">", range inclusivo ">>", fragmento inválido ignorado em silêncio) e o FrequencyCapComponentImpl.isEligibleFor com a regra (hours vazio OU contém hora) E (weekdays vazio OU contém dia ISO 1=SEG..7=DOM), timezone America/Sao_Paulo explícito e clock injetado. Implementar TODOS os casos de teste da tabela em docs/issues/M1-02-frequencycap-parser.md (incluindo "0;5>10;15>>", "5>3" inválido, string vazia = sem restrição, hora 23, domingo=7). Código 100% comentado em português com "// Portado de: ...", make lint && make test verdes, abrir PR feat/issue-M1-02-frequencycap com Closes na issue.
```
