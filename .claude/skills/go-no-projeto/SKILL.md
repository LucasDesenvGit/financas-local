---
name: go-no-projeto
description: Como se escreve Go no Finanças Local (src/backend, o back-end inteiro desde a #86) — funcionalidade nova, mudança em código Go que já existe e revisão de PR com Go, com as regras do projeto e a fonte de cada uma. Usar ao pedirem "faz isso em Go", "rota nova", "pacote novo", "revisa o Go", "mexe no back-end" e ao pegar issue que mexe em src/backend.
model: sonnet
---

# Go no Finanças Local

Você escreve e revisa o Go do projeto, que é o back-end inteiro desde a #86 (o .NET saiu e está na tag `v-ultimo-dotnet`). O que vale **sempre** está aqui. A skill `migrar-para-go` é a da migração, que terminou: serve só para consultar como um assunto veio do C#.

**Cada regra mora num lugar só.** Esta skill diz o caminho e aponta a fonte; não copie a regra para cá. Achou uma regra aqui que diverge da fonte, vale a fonte: corrija a skill no mesmo PR.

| Assunto | Fonte (a regra está lá) |
|---|---|
| Regras de ouro (SQL, relógio, estilo) e bibliotecas | [ADR 0049](../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), "Convenções do código Go" e a tabela de bibliotecas; o dinheiro e a funcionalidade nova em Go também no `CLAUDE.md`, "Código Go" (já está no seu contexto) |
| Como o .NET saiu (a história da troca), "Aula rápida" | [ADR 0049](../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) |
| Camadas por assunto, regra sem HTTP, mapa de contextos, interfaces | [ADR 0056](../../../docs/adr/0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md) |
| Testes (onde mora cada um, pronto quando, proibido, mutação, fuzz, cobertura) | [ADR 0051](../../../docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md) e `CONTRIBUINDO.md`, "Testes do back-end (Go)" |
| Apoio dos testes (banco temporário, fábrica, relógio, `Chamar`, serviço de fora) | [`internal/plataforma/testeapoio/README.md`](../../../src/backend/internal/plataforma/testeapoio/README.md) |
| Rota nova, ligar no site, rodar, antes de subir | [`src/backend/README.md`](../../../src/backend/README.md) |
| Banco no formato do EF Core, `sqlc`, esquema | [`internal/plataforma/banco/README.md`](../../../src/backend/internal/plataforma/banco/README.md) e `docs/BANCO-DE-DADOS.md` |
| Gerenciador em Go (`src/gerenciador-go`) | [ADR 0052](../../../docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md): outro módulo, com bibliotecas próprias; o estilo daqui vale |

## Referências (leia quando o passo pedir)

| Arquivo | Quando ler |
|---|---|
| [`referencias/estilo.md`](referencias/estilo.md) | antes de escrever código Go, e sempre que um guia de fora (Effective Go, Google, Uber) parecer dizer outra coisa |
| [`referencias/exemplos.md`](referencias/exemplos.md) | antes de criar pacote, rota, consulta, cliente externo ou tarefa: copie a forma de um pacote que já existe |
| [`referencias/revisao.md`](referencias/revisao.md) | ao revisar um PR com Go, e no fim do seu próprio trabalho |
| [`referencias/links.md`](referencias/links.md) | ao escrever a "🎓 Aula rápida" ou citar o Go como fato: só links daqui ou conferidos na hora |

## Funcionalidade nova em Go

Todo o back-end é Go: assunto novo ganha um pacote, e mudança num assunto fica no pacote dele.

1. **Issue e branch** como em todo PR (`CLAUDE.md`, "Git / commits").
2. **ADR antes do código** quando a funcionalidade trouxer dependência, saída de rede, tabela além da rotina ou padrão novo (`CLAUDE.md`, "Decisões de arquitetura").
3. **Banco:** tabela nova em `banco/01-estrutura.sql` (o Go só cria tabela por esses scripts, no `-aplicar-esquema`), `go generate ./...` e `docs/BANCO-DE-DADOS.md`. Consulta pelo `sqlc` (`internal/plataforma/banco/consultas/sql`, depois `go generate ./...`) ou com `?`.
4. **Pacote `internal/<area>/<assunto>`** com HTTP → regra → banco dentro (ADR 0056), a linha do assunto em `dependencias` do teste de arquitetura, com o motivo, o pacote na tabela de `docs/arquitetura/componentes.md` (o mesmo teste confere) e o termo novo em `docs/arquitetura/dominio.md`.
5. **Rotas** pelo `Registrar(mux, ...)` do pacote e uma linha em `internal/api/api.go` (README do Go, "Rota nova"), com os casos de contrato num número livre de `20` a `89`.
6. **Testes** pela ADR 0051: tabela por regra, SQLite real, `httptest` e, se tiver cliente externo, `testeapoio.NovoServicoDeFora`. Pacote novo nasce com 80% de cobertura (`verificar-cobertura-go.py`) e entra em `scripts/ci/go/mutacao-go.txt` (com regra de negócio ou dinheiro) **ou** em `scripts/ci/go/mutacao-go-fora.txt`, com o motivo: a CI falha com pacote que não está em nenhum dos dois (#456).
7. **Front-end, Manual, diagramas** (ADR 0055) e `docs/DADOS-PESSOAIS.md` se houver dado pessoal novo: no mesmo PR, como manda o `CLAUDE.md`.

## Mudança em Go que já existe

- Leia o pacote inteiro e os testes dele antes de mexer; siga a forma dele, mesmo que outro pacote faça diferente.
- O assunto ainda não segue a ADR 0056 (ela vale para os já migrados quando alguém mexer neles)? Ajuste só o que a mudança toca e diga no PR o que ficou.
- Bug corrigido ganha teste que falha antes da correção (ADR 0051).

## Antes de dizer que terminou

Em `src/backend`, os comandos de "Antes de subir" do README do Go, mais:

```bash
go test -tags arquitetura ./internal/arquitetura/      # só local (#476)
python3 ../../scripts/ci/go/mutacao-go.py internal/<area>/<assunto>   # pacote com regra; a CI mede no PR (#456)
```

- **Go não instalado** (`go version` falha; nesta máquina pode ser o sandbox do Flatpak: `flatpak-spawn --host`): diga isso e não afirme que passou.
- **O que a CI já pega** (formatação, `vet`, `-race`, cobertura, tempo, mutação no pacote mudado, govulncheck, CodeQL): rode antes, mas não gaste texto do PR explicando. O que a CI **não** pega é onde está o valor da revisão: [`referencias/revisao.md`](referencias/revisao.md).

## Desempenho: medir, não presumir

- Compare com as metas de `docs/REQUISITOS-NAO-FUNCIONAIS.md` e diga como mediu (`python3 scripts/dev/medir-desempenho.py`).
- Código quente (relatórios, sincronização): `go test -bench . -benchmem` e `pprof` antes de otimizar. Otimização que complica o código precisa de número que a justifique.

## Como entregar

Separe o que conferiu no código (com `arquivo:linha`) do que é julgamento seu, diga o que rodou e o resultado (inclusive o que falhou ou não rodou), e o que ficou de fora, com o motivo. PR: o checklist do `CONTRIBUINDO.md` e do modelo de PR; merge só com o "pode" de quem pediu, para aquele PR.
