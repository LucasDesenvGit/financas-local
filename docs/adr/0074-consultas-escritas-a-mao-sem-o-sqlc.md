# 0074. Consultas escritas à mão, sem o sqlc

- **Status:** Aceita
- **Data:** 2026-11-22
- **Relacionadas:** substitui em parte a [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (a linha "Consultas" da tabela de bibliotecas); [0056](0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md)

## Contexto

A [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) escolheu o `sqlc` para gerar as consultas, com o esquema como fonte única. Na prática, medido em 18/11/2026 (#771):

- o `sqlc` atende **16 consultas** (configuração, usuário e auditoria); as outras 427 do back-end são escritas à mão, com `database/sql` e `?`;
- o `models.go` gerado tinha 565 linhas e 48 structs, e só a `Usuario` era usada. Ele mudava a cada tabela nova do banco, e uma PR regerava os modelos sem usá-los;
- o `sqlc` compila com cgo, e o `go generate` levava cerca de 1 minuto de cada job Go da CI (53 a 54 s nas três últimas CIs completas do `develop`), além do `gcc` (Linux) ou do MSYS2 (Windows) para quem mexesse numa consulta;
- o `sqlc` não lê o `banco/01-estrutura.sql`. Um gerador próprio reescrevia o esquema num SQLite em memória (`esquema-sqlc.sql`, 559 linhas), e as consultas tinham de ser só ASCII, com todo nome entre aspas duplas. Cada limitação ganhou um teste.

O projeto é local e de um usuário só, e a regra é que ferramenta que não se paga nesse tamanho não entra ([SIMPLICIDADE](../SIMPLICIDADE.md)).

## Alternativas consideradas

- **Manter o `sqlc` como está.** Mantém o que existe, mas cobra o minuto de CI, o cgo, o esquema reescrito e as três limitações por 16 consultas.
- **Migrar as 427 consultas para o `sqlc`.** Daria uma forma só, mas é reescrita enorme, o motor SQLite dele é o menos maduro e não monta bem o filtro opcional, que é o caso comum aqui.
- **Manter o `sqlc` só para as 16, sem gerar os modelos.** Tira as 48 structs, mas não o cgo, o minuto de CI nem o esquema reescrito.
- **Escrever as 16 à mão.** Elas ficam no pacote de quem as usa, com `?`, no mesmo estilo das outras 427.

## Decisão

Tiramos o `sqlc`. As consultas são escritas à mão, com `database/sql` e `?`, no pacote do assunto, como todas as outras (consulta sempre parametrizada, nunca texto do usuário no SQL). O `DBTX` (a interface que `*sql.DB` e `*sql.Tx` cumprem) vai para `internal/plataforma/banco`. O `go generate` continua só para a cópia embutida dos scripts do banco, até a #723.

## Consequências

- **Positivas:** saem o `sqlc.yaml`, o `esquema-sqlc.sql`, o `models.go`, o código gerado, o `GerarEsquemaSqlc` e os testes das limitações (`TestEsquemaDoSqlcEmDia`, `TestConsultasSemAcento`). O job Go perde o minuto do cgo, e o passo "Mudou a estrutura" perde uma etapa. O pacote `internal/plataforma/banco/consultas` deixa de existir, e com ele a exceção na cobertura e na mutação.
- **Negativas:** o compilador não confere mais a consulta contra o esquema: nome de coluna errado só aparece rodando. O teste de cada regra, com SQLite real ([0051](0051-testes-do-back-end-em-go-cada-regra-com-teste.md)), é quem pega, como já era nas 427. Quem escreve a consulta lê as colunas na mão (`Scan`), e o cuidado com `NULL` (ponteiro) e com os tipos do banco (`tipos`) volta a ser de quem escreve.
- **Na prática:** consulta nova vai no pacote do assunto, com `?` e `QueryRowContext`/`QueryContext`/`ExecContext` sobre um `banco.DBTX`. Quem grava junto com a auditoria recebe o `DBTX` (a transação) e o repassa a `auditoria.Registrar`. Mudou a estrutura do banco: o SQL, o `2-criar-banco.sh` e o `go generate`, só para a cópia embutida.
