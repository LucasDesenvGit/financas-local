# Pacote `banco`: o SQLite no formato do EF Core

Abre o banco, dá a transação e cria e atualiza o esquema ao iniciar. Grava no formato que o EF Core usava, para os bancos que já existem continuarem os mesmos ([ADR 0049](../../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)).

| O quê | Onde |
|---|---|
| Abrir o banco e fazer transação | `banco.go`: `Abrir`, `EmTransacao`, `DBTX` |
| Tipos no formato do EF Core (o formato de cada um está no comentário dele) | [`internal/plataforma/tipos`](../tipos): `Decimal`, `Data`, `DataHora`, `Guid` |
| Consultas | à mão, com `?` para todo valor, no pacote de quem as usa, sobre um `DBTX` ([ADR 0074](../../../../../docs/adr/0074-consultas-escritas-a-mao-sem-o-sqlc.md)) |
| Criar e atualizar o banco | `esquema_banco.go`: `AplicarEsquema` ([ADR 0003](../../../../../docs/adr/0003-esquema-por-scripts-sql-idempotentes.md)); mudar a estrutura, em [docs/BANCO-DE-DADOS.md](../../../../../docs/BANCO-DE-DADOS.md#como-alterar-a-estrutura) |
| Relógio no fuso de Brasília | [`internal/plataforma/relogio`](../relogio) |

## Armadilhas

- **Nunca `SUM`, `AVG`, `ORDER BY`, `<` ou `>` do SQL em coluna de dinheiro.** Ela é `TEXT`: o `SUM` passa por ponto flutuante (erra centavos), e o `ORDER BY` ordena como texto (`"10.0" < "9.0"`). Some e compare em Go, com `tipos.Decimal`.
- **Nunca `float64` para dinheiro**, nem no JSON: o `tipos.Decimal` escreve o número sem passar por ele.
- **Arredondamento:** o banco não arredonda (`0.125` fica `0.125`). Use `tipos.Arredondar` (bancário, como o `Math.Round` do C#), nunca o `Round` do shopspring.
- **Divisão:** use `tipos.Dividir`, nunca o `Div` do shopspring, que divide com 16 casas (o `10.0` sairia `10.00` no JSON).
- **Datas:** a de negócio (a `Data` da transação) está no fuso de Brasília; os carimbos (`CriadoEm`), em UTC. O `DataHora` guarda a hora sem fuso: converta antes de gravar (`relogio`).
- **Meia-noite que não existiu** (início do horário de verão, até 2019): para o começo de um dia, use `relogio.InicioDoDiaUtc`, e não o `time.Date` (#372).
- **GUID:** no banco, em maiúsculas; leia com `tipos.LerGuid`.
- **Coluna opcional** é ponteiro (`*tipos.Decimal`, `*string`): `nil` grava e lê `NULL`.
- **Transação curta:** nada de chamada de rede nem espera dentro dela. As rotas e as tarefas em segundo plano gravam no mesmo arquivo e esperam até 30 s pela trava.
- **Não edite `esquema/embutido/`:** é a cópia que vai no executável. Mude o original em `banco/` e rode `go generate ./...` (o `TestCopiasEmDia` falha se ficar velha).

## Testes

```bash
go test -race ./...
# Manual (#74): ler todas as tabelas de uma CÓPIA do banco real, sem mostrar valores
cp ../../dados/sql/FinancasLocal.db /tmp/copia.db
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/plataforma/banco -run CopiaDoBancoReal -v
```

- **O gabarito do .NET** (`testdata/esquema-dotnet.json`, com os scripts de `testdata/entradas-esquema/`) é a regressão do `AplicarEsquema` e não é mais regravado (a sonda saiu na #86). Uma mudança de propósito no jeito de criar ou atualizar o banco muda o teste, com o motivo no PR (ADR 0051).
- **Coluna nova:** os testes que a conferem estão em [docs/BANCO-DE-DADOS.md, "Coluna nova"](../../../../../docs/BANCO-DE-DADOS.md#coluna-nova-numa-tabela-existente).
