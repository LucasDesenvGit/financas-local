# Pacote `banco`: o SQLite do jeito que o .NET usa

O Go e o .NET usam **o mesmo arquivo** de banco durante a migração ([ADR 0049](../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Tudo o que o Go grava tem de estar no formato do EF Core, para o .NET ler (e vice-versa).

| O quê | Onde |
|---|---|
| Abrir o banco e fazer transação | `banco.go`: `Abrir`, `EmTransacao` |
| Tipos no formato do EF Core | [`internal/tipos`](../tipos): `Decimal`, `Data`, `DataHora`, `Guid` |
| Consultas | `consultas/sql/*.sql`, geradas pelo `sqlc` em `consultas/` |
| Esquema para o `sqlc` | `esquema/esquema-sqlc.sql`, gerado a partir do `banco/` |
| Criar e atualizar o banco (o `EsquemaBanco` do .NET) | `esquema_banco.go`: `AplicarEsquema`; scripts em `esquema/embutido/` |
| Relógio no fuso de Brasília | [`internal/relogio`](../relogio) |

## Formatos (confirmados pela sonda do EF Core)

A sonda em [`internal/tipos/testdata/sonda-efcore`](../tipos/testdata/sonda-efcore) grava valores pelo mesmo EF Core da API e guarda o resultado em `efcore.json`, que é o gabarito dos testes. Rode de novo quando o EF Core mudar de versão.

| C# | No banco (`TEXT`, salvo indicação) | No JSON da API |
|---|---|---|
| `decimal` | `"0.0"`, `"-5000.0"`, `"1.5"` (pelo menos uma casa, sem zero à direita) | número com a escala lida: `-5000.0`, `1.5` |
| `DateTime` | `"2026-10-15 22:55:31.3481471"` (fração some quando é zero) | `"2026-10-15T22:55:31.3481471"`, **sem `Z`** |
| `DateOnly` | `"2026-10-15"` | `"2026-10-15"` |
| `Guid` | **MAIÚSCULAS** | minúsculas |
| `bool`, `enum` | `INTEGER` (0/1, número) | `true`/`false`, número |

- `HasPrecision(18, 2)` **não arredonda** no SQLite: `0.125` fica `0.125`.
- Conexão: o EF Core liga `foreign_keys`; o `Abrir` também. O `journal_mode` é do arquivo, e o `Abrir` não o muda: o `EsquemaBanco` do .NET passa o banco para **WAL** ao iniciar, antes de subir o Go (#231). `ModoDoDiario` diz o modo.
- Banco ocupado: o `Abrir` espera até 30 s (`busy_timeout`), o mesmo tempo padrão do Microsoft.Data.Sqlite, e começa as transações com `BEGIN IMMEDIATE` (`_txlock=immediate`). Com o .NET e o Go no mesmo arquivo, transação curta: nada de chamada de rede nem espera dentro dela.
- Dois processos gravando ao mesmo tempo: `TestDoisProcessosGravandoEmWAL` (o próprio teste chamado num segundo processo). Com `BEGIN` comum no lugar do `IMMEDIATE`, ele falha (conferido).

## ⚠️ Armadilhas

- **Nunca `SUM`, `AVG`, `ORDER BY`, `<` ou `>` do SQL em coluna de dinheiro.** Ela é `TEXT`: o `SUM` converte para ponto flutuante (erra centavos), e o `ORDER BY` ordena como texto (`"10.0" < "9.0"`). Some e compare em Go, com `tipos.Decimal`. Confira como o .NET faz hoje em cada consulta.
- **Nunca `float64` para dinheiro.** Nem no JSON: o `tipos.Decimal` escreve o número sem passar por `float64`. O `shopspring/decimal` sozinho escreveria **entre aspas**.
- **Arredondamento:** o `Math.Round` do C# arredonda pelo método bancário (0,125 → 0,12). Use `RoundBank`, e não `Round`, onde o C# não diz o `MidpointRounding`.
- **Datas:** a de negócio (a `Data` da transação) está no fuso de Brasília; os carimbos (`CriadoEm`) estão em UTC. O `DataHora` guarda a hora "de parede" sem fuso, como o EF Core: converta antes de gravar (`relogio`).
- **Coluna opcional** é ponteiro (`*tipos.Decimal`, `*string`): `nil` grava e lê `NULL`.

## Esquema ao iniciar: `AplicarEsquema` (#297)

O porte do `Data/EsquemaBanco.cs` ([ADR 0003](../../../../docs/adr/0003-esquema-por-scripts-sql-idempotentes.md)), passo a passo igual:

1. Lê as linhas `adicionar_coluna_se_ausente` do `2-criar-banco.sh` com a mesma expressão (com o `\s` do .NET, que pega mais espaços que o do Go) e calcula a **mesma assinatura** do .NET.
2. Banco que já existe com a mesma assinatura no `PRAGMA user_version`: não faz nada, nem cópia.
3. Assinatura diferente: copia o banco com `VACUUM INTO` para `dados/backups/FinancasLocal-antes-atualizacao-AAAAMMDD_HHMMSS.db` (hora de Brasília; o .NET usa a do computador) e guarda as 5 mais recentes.
4. Numa transação: o `01-estrutura.sql`, as colunas que faltam e, **só no banco novo**, o `02-dados-iniciais.sql`. Depois grava a assinatura.

**Até a inversão (#85, [ADR 0049](../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)), quem aplica o esquema ao iniciar é o .NET.** O executável Go só aplica com `-aplicar-esquema`. Com a mesma assinatura, um lado não refaz o trabalho do outro.

Os scripts vão dentro do executável. Como o `//go:embed` não sai do módulo (`src/backend-go`), o `go generate ./...` copia os três arquivos para `esquema/embutido/`. **Não edite a cópia:** mude o original e rode o `go generate`. O `TestCopiasEmDia` falha se a cópia ficar velha.

### O gabarito do .NET

A sonda em [`testdata/sonda-esquema`](testdata/sonda-esquema) compila o **mesmo** `EsquemaBanco.cs` da API, com os mesmos scripts embutidos, roda os cenários (banco novo, sem mudança, arquivo vazio, banco antigo sem duas colunas, rodízio das cópias, pasta da cópia) e grava:

- `testdata/esquema-dotnet.json`: as assinaturas, as linhas lidas do `.sh` e a estrutura e os dados de cada banco (tabela por tabela, coluna por coluna, índices e chaves estrangeiras);
- `testdata/entradas-esquema/*.gz`: os scripts com que o gabarito foi feito. O teste do Go aplica **esses**, e não os de agora, para não ficar velho a cada mudança no `banco/`.

Rode de novo quando o `EsquemaBanco.cs` mudar, ou para conferir o Go com os scripts de agora:

```bash
~/.dotnet/dotnet run --project src/backend-go/internal/banco/testdata/sonda-esquema   # ou dotnet, se estiver no PATH
```

## `sqlc`: como gerar e o que ele não entende

```bash
cd src/backend-go
go generate ./...     # copia os scripts para esquema/embutido/, reescreve esquema/esquema-sqlc.sql e gera consultas/ (sqlc v1.31.1 pelo go run)
```

O `sqlc` compila com cgo: precisa do `gcc` no Linux ou do MSYS2 no Windows. Só precisa quem muda uma consulta ou o esquema, porque o código gerado vai no commit. O teste `TestEsquemaDoSqlcEmDia` falha quando o `banco/` ou o `2-criar-banco.sh` mudam e alguém esquece de gerar.

O motor SQLite do `sqlc` (v1.31.1) tem três limitações, contornadas assim:

1. **`TEXT NULL` vira `interface{}`** e **os nomes do `CREATE TABLE` viram minúsculas** (`CriadoEm` vira o campo `Criadoem`). Por isso o `sqlc` não lê o `01-estrutura.sql`: o gerador em `esquema/` aplica a estrutura e as colunas novas num SQLite em memória e reescreve o esquema como o próprio SQLite o vê, com os nomes entre aspas.
2. **Nome sem aspas na consulta vira minúsculo** e não acha a tabela. Escreva **todos os nomes entre aspas duplas**: `SELECT "Chave" FROM "Configuracao"`.
3. **Acento antes da consulta quebra o recorte do SQL**: a consulta gerada sai cortada no lugar errado. Os `.sql` de `consultas/sql/` são **só ASCII** (o `TestConsultasSemAcento` confere). Texto com acento vai por parâmetro (`?`).

Filtro opcional, que o `sqlc` não monta bem, vai para `database/sql` com `?` e uma lista de argumentos. **Nunca** concatene texto do usuário no SQL.

## Testes

```bash
go test -race ./...
# Manual (#74): ler todas as tabelas de uma CÓPIA do banco real, sem mostrar valores
cp ../../dados/sql/FinancasLocal.db /tmp/copia.db
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/banco -run CopiaDoBancoReal -v
```
