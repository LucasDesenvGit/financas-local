# Pacote `banco`: o SQLite no formato que o .NET deixou

O banco de quem já usa o site foi criado e gravado pelo .NET, com o EF Core ([ADR 0049](../../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). O Go lê e grava **no mesmo formato**, para os bancos que já existem continuarem os mesmos (e para a tag `v-ultimo-dotnet` ainda abrir um banco gravado pelo Go).

| O quê | Onde |
|---|---|
| Abrir o banco e fazer transação | `banco.go`: `Abrir`, `EmTransacao`, `DBTX` |
| Tipos no formato do EF Core | [`internal/plataforma/tipos`](../tipos): `Decimal`, `Data`, `DataHora`, `Guid` |
| Consultas | escritas à mão, com `?`, no pacote de quem as usa; `DBTX` (um `*sql.DB` ou uma `*sql.Tx`) é a interface que elas recebem ([ADR 0074](../../../../../docs/adr/0074-consultas-escritas-a-mao-sem-o-sqlc.md)) |
| Criar e atualizar o banco (o `EsquemaBanco` do .NET) | `esquema_banco.go`: `AplicarEsquema`; scripts em `esquema/embutido/` |
| Relógio no fuso de Brasília | [`internal/plataforma/relogio`](../relogio) |

## Formatos (confirmados pela sonda do EF Core)

A sonda `internal/plataforma/tipos/testdata/sonda-efcore` gravou valores pelo mesmo EF Core da API e guardou o resultado em `efcore.json`, que é o gabarito dos testes. Ela saiu com o C# na #86 (está na tag `v-ultimo-dotnet`); o gabarito fica, porque é o formato de todos os bancos que já existem.

| C# | No banco (`TEXT`, salvo indicação) | No JSON da API |
|---|---|---|
| `decimal` | `"0.0"`, `"-5000.0"`, `"1.5"` (pelo menos uma casa, sem zero à direita) | número com a escala lida: `-5000.0`, `1.5` |
| `DateTime` | `"2026-10-15 22:55:31.3481471"` (fração some quando é zero) | `"2026-10-15T22:55:31.3481471"`, **sem `Z`** |
| `DateOnly` | `"2026-10-15"` | `"2026-10-15"` |
| `Guid` | **MAIÚSCULAS** | minúsculas |
| `bool`, `enum` | `INTEGER` (0/1, número) | `true`/`false`, número |

- `HasPrecision(18, 2)` **não arredonda** no SQLite: `0.125` fica `0.125`.
- Conexão: o EF Core liga `foreign_keys`; o `Abrir` também. O `journal_mode` é do arquivo, e o `Abrir` não o muda: o `AplicarEsquema` passa o banco para **WAL** ao iniciar (`passarParaWal`, teste `TestAplicarEsquemaPassaParaWal`), como o `EsquemaBanco` do .NET fazia. `ModoDoDiario` diz o modo.
- Banco ocupado: o `Abrir` espera até 30 s (`busy_timeout`), o mesmo tempo padrão do Microsoft.Data.Sqlite, e começa as transações com `BEGIN IMMEDIATE` (`_txlock=immediate`). Transação curta: nada de chamada de rede nem espera dentro dela (as tarefas em segundo plano e as rotas gravam no mesmo arquivo).
- Dois processos gravando ao mesmo tempo: `TestDoisProcessosGravandoEmWAL` (o próprio teste chamado num segundo processo). Com `BEGIN` comum no lugar do `IMMEDIATE`, ele falha (conferido).

## ⚠️ Armadilhas

- **Nunca `SUM`, `AVG`, `ORDER BY`, `<` ou `>` do SQL em coluna de dinheiro.** Ela é `TEXT`: o `SUM` converte para ponto flutuante (erra centavos), e o `ORDER BY` ordena como texto (`"10.0" < "9.0"`). Some e compare em Go, com `tipos.Decimal`. Confira como o .NET faz hoje em cada consulta.
- **Nunca `float64` para dinheiro.** Nem no JSON: o `tipos.Decimal` escreve o número sem passar por `float64`. O `shopspring/decimal` sozinho escreveria **entre aspas**.
- **Arredondamento:** o `Math.Round` do C# arredonda pelo método bancário (0,125 → 0,12) e só mexe quando há mais casas que o pedido (250.0 continua 250.0). Use `tipos.Arredondar`, e não o `Round` do shopspring, onde o C# não diz o `MidpointRounding`.
- **Divisão:** use `tipos.Dividir`, nunca o `Div` do shopspring (#81). O decimal do C# divide até caber em 96 bits, arredonda a última casa pelo método bancário e tira os zeros à direita só até a escala natural (`1.00 / 1 = 1.00`, `10 / 4 = 2.5`); o shopspring divide com 16 casas, e o `10.0` do C# sairia `10.00` no JSON. O gabarito é o `tipos/testdata/csharp.json` (sonda `tipos/testdata/sonda-csharp`).
- **Datas:** a de negócio (a `Data` da transação) está no fuso de Brasília; os carimbos (`CriadoEm`) estão em UTC. O `DataHora` guarda a hora "de parede" sem fuso, como o EF Core: converta antes de gravar (`relogio`).
- **Meia-noite que não existiu:** nos dias em que o horário de verão começava (até 2019, como 04/11/2018), o relógio de Brasília pulava de 23:59:59 para 01:00. Para essa hora, o `time.Date` devolve as 23:00 do dia anterior. Para o começo de um dia, use o `relogio.InicioDoDiaUtc`, que já trata isso (#372).
- **GUID:** o `tipos.LerGuid` lê só os dois formatos que o .NET escreve, `8-4-4-4-12` e os 32 dígitos, como o `Guid.Parse`. Hífen fora do lugar é recusado (#372).
- **Coluna opcional** é ponteiro (`*tipos.Decimal`, `*string`): `nil` grava e lê `NULL`.

## Esquema ao iniciar: `AplicarEsquema` (#297)

O porte do `Data/EsquemaBanco.cs` ([ADR 0003](../../../../../docs/adr/0003-esquema-por-scripts-sql-idempotentes.md)), passo a passo igual:

1. Calcula a **mesma assinatura** do .NET, com a estrutura e as `ColunasNovas` (`esquema/colunas_novas.go`; até a #723 eram as linhas `adicionar_coluna_se_ausente` do `2-criar-banco.sh`).
2. Banco que já existe com a mesma assinatura no `PRAGMA user_version`: não faz nada, nem cópia.
3. Assinatura diferente: copia o banco com `VACUUM INTO` para `dados/backups/FinancasLocal-antes-atualizacao-AAAAMMDD_HHMMSS_mmm.db` (hora de Brasília; o .NET usa a do computador) e guarda as 5 mais recentes.
   - **Milissegundos no nome** (#366): só até o segundo, duas atualizações no mesmo segundo davam o mesmo nome, e o `VACUUM INTO` recusa um destino que já existe.
   - **Ordem pelo nome, byte a byte** (`slices.Sort`; no .NET, `StringComparer.Ordinal`): o `.` vem antes do `_`, e o nome antigo de um segundo (`_HHMMSS.db`) fica antes dos novos do mesmo segundo. A ordem da cultura, padrão do `OrderBy` do .NET, põe o `_` antes do `.` (medido) e apagaria o mais novo.
   - **Cópia antiga que não dá para apagar** (outro dono, só leitura) fica para a próxima, com um aviso no log que diz o arquivo, e a API sobe (#365).
4. Numa transação: o `01-estrutura.sql`, as colunas que faltam e, **só no banco novo**, o `02-dados-iniciais.sql`. Depois grava a assinatura.

**Quem aplica o esquema é o executável**: o site ao iniciar, com `-aplicar-esquema` (a linha do serviço instalado sempre liga), e os scripts de instalação, com o subcomando `aplicar-esquema -banco <caminho>` (`cmd/financas/esquema.go`, #723). A assinatura é a mesma que o .NET calculava: quem atualiza vindo da versão em .NET não ganha uma cópia nem uma atualização à toa.

Os scripts `.sql` vão dentro do executável (a lista de colunas já é código Go). Como o `//go:embed` não sai do módulo (`src/backend`), o `go generate ./...` copia os dois arquivos `.sql` para `esquema/embutido/`. **Não edite a cópia:** mude o original e rode o `go generate`. O `TestCopiasEmDia` falha se a cópia ficar velha.

### O gabarito do .NET

A sonda `testdata/sonda-esquema` (que saiu com o C# na #86 e está na tag `v-ultimo-dotnet`) compilava o **mesmo** `EsquemaBanco.cs` da API, com os mesmos scripts embutidos, rodava os cenários (banco novo, sem mudança, arquivo vazio, banco antigo sem duas colunas, rodízio das cópias, pasta da cópia) e gravou:

- `testdata/esquema-dotnet.json`: as assinaturas e a estrutura e os dados de cada banco (tabela por tabela, coluna por coluna, índices e chaves estrangeiras);
- `testdata/entradas-esquema/*.gz`: os scripts com que o gabarito foi feito. O teste do Go aplica **esses**, e não os de agora, para não ficar velho a cada mudança no `banco/`.

Sem o .NET, o gabarito não é mais regravado: ele é a regressão do `AplicarEsquema`. Uma mudança de propósito no jeito de criar ou atualizar o banco muda o teste, com o motivo no PR (ADR 0051).

## Consultas

Escreva o SQL à mão, com `?` para todo valor, num `QueryRowContext`, `QueryContext` ou `ExecContext` sobre um `DBTX`. **Nunca** concatene texto do usuário no SQL; filtro opcional monta o `WHERE` com pedaços fixos e uma lista de argumentos.

## Testes

```bash
go test -race ./...
# Manual (#74): ler todas as tabelas de uma CÓPIA do banco real, sem mostrar valores
cp ../../dados/sql/FinancasLocal.db /tmp/copia.db
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/plataforma/banco -run CopiaDoBancoReal -v
```

Coluna nova (#367, `colunas_novas_test.go`), com os scripts de agora:
- `TestColunasNovasTambemNoCreateTable`: toda coluna de `ColunasNovas` também está no `CREATE TABLE` do `01-estrutura.sql`;
- `TestBancoAntigoAtualizadoFicaIgualAoNovo`: um banco sem essas colunas, atualizado pelo `AplicarEsquema`, fica com as mesmas colunas do banco novo (tipo, `NOT NULL`, padrão e chave; só a ordem pode mudar).
