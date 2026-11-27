# Banco (scripts SQL)

A estrutura do banco SQLite, sem migrations ([ADR 0003](../docs/adr/0003-esquema-por-scripts-sql-idempotentes.md)). O modelo de dados e o passo a passo de cada mudança estão em [docs/BANCO-DE-DADOS.md](../docs/BANCO-DE-DADOS.md).

| Arquivo | O que tem |
|---|---|
| `01-estrutura.sql` | tabelas e índices, todos com `IF NOT EXISTS`: roda de novo sem estragar nada |
| `02-dados-iniciais.sql` | as categorias, configurações e regras padrão, só num banco novo |
| `03-tabelas-oficiais.sql` | as tabelas do INSS e do IRRF por vigência, aplicadas a cada partida do site (ADR 0065) |

O site aplica os três ao iniciar (`-aplicar-esquema`), com uma cópia embutida no executável, e os scripts de instalação chamam o mesmo executável (`aplicar-esquema`, #723).

**Não mexer sem ler o guia:** [como alterar a estrutura](../docs/BANCO-DE-DADOS.md#como-alterar-a-estrutura) (tabela nova, coluna nova, que vai também no Go, e o que rodar depois).
