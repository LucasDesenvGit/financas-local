# Revisão de código Go

Use ao revisar um PR com Go e no fim do seu próprio trabalho. Cada achado sai com `arquivo:linha`, o que acontece de errado (entrada → resultado) e a fonte da regra; o que for gosto, sem fonte, diga que é julgamento.

## O que a CI já pega (não gaste a revisão nisso)

`gofmt`, `go vet`, `go test -race` (e `go test` no Windows), as regras de ouro do `verificar-regras-go.py` (#457: `float64`, `time.Now()` fora do `relogio`, `time.Sleep` em teste fora do `synctest`), piso de cobertura (80% para pacote novo), pacote novo sem lugar nas listas de mutação, trava de tempo, mutação nos pacotes mudados, `go generate` em dia, build do Windows, govulncheck, CodeQL (`go/sql-injection`), contrato dos assuntos ligados, versão subiu, CHANGELOG, aviso do Manual. Se um destes falhou na CI, a revisão aponta o job; não reexplica a regra.

## O que só a revisão pega

**Regra e teste (ADR 0051)**
- [ ] Cada regra nova tem uma linha numa tabela de teste, e o PR traz a tabela regra → teste.
- [ ] Os casos-limite que se aplicam estão lá (a lista está na ADR 0051 e no `CONTRIBUINDO.md`); os que não se aplicam estão ditos no PR.
- [ ] O teste prova a regra: a mutação à mão listada no PR, com o teste que a pegou. Mutação em condição de `case` só pega à mão (ponto cego do gremlins).
- [ ] Bug corrigido com teste que falhava antes.

**Dinheiro, hora e banco** (o `float64`, o `time.Now()` e o `time.Sleep` a CI já pega)
- [ ] Nenhum `SUM`/`ORDER BY`/`<`/`>` no SQL em coluna de dinheiro (é `TEXT`): somar e comparar em Go, com `tipos.Decimal`.
- [ ] Exceção nova em `scripts/ci/regras-go-excecoes.txt` tem um motivo que convence.
- [ ] Gravação no formato do EF Core (decimal com escala, data, GUID em maiúsculas); atualização em massa dentro de `banco.EmTransacao`.
- [ ] Lista de saída é `[]T{}`, não `nil`, quando o assunto ainda não tem caso de contrato que pegaria.

**Forma do código (ADR 0056)**
- [ ] O teste de arquitetura passa (`go test -tags arquitetura ./internal/arquitetura/`). **É só local**: rode você; a CI não roda.
- [ ] Dependência nova entre assuntos tem a linha em `dependencias` com um motivo que convence. Na dúvida, pergunte se a regra comum não deveria ir para um pacote de regra.
- [ ] Pacote de regra sem `net/http`, `internal/respostas` nem `internal/servidor`.
- [ ] Interface nova tem duas implementações de verdade e é declarada por quem usa.
- [ ] Goroutine nova tem dono que a encerra pelo `context`.

**Erro, log e segurança**
- [ ] Erro não é engolido: ou volta embrulhado com `%w`, ou é registrado com `"origem"` e vira o status certo.
- [ ] Segredo nunca na resposta nem no log; comparação de segredo com `crypto/subtle`.
- [ ] Saída de rede nova tem ADR, linha em `docs/SEGURANCA.md` e revisão do `docs/MODELO-DE-AMEACAS.md`.

**Documentação junto, no mesmo PR** (`CLAUDE.md`)
- [ ] Assunto passou para o Go ou mudou de fronteira: `docs/ARQUITETURA.md` (mapa de contextos, diagramas), `docs/arquitetura/fluxos.md` e `docs/arquitetura/dominio.md` (ADR 0055).
- [ ] Dado pessoal novo: `docs/DADOS-PESSOAIS.md`. Cópia, WAL, `dados/` ou chaves: `docs/OPERACAO.md`.
- [ ] PR do épico #70: a "🎓 Aula rápida", com link nas afirmações técnicas principais ([`links.md`](links.md)).
