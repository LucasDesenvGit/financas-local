# Banco (scripts SQL)

A estrutura do banco SQLite, sem migrations ([ADR 0003](../docs/adr/0003-esquema-por-scripts-sql-idempotentes.md)). O modelo de dados e o passo a passo de cada mudança estão em [docs/BANCO-DE-DADOS.md](../docs/BANCO-DE-DADOS.md).

| Arquivo | O que tem |
|---|---|
| `01-estrutura.sql` | tabelas e índices, todos com `IF NOT EXISTS`: roda de novo sem estragar nada |
| `02-dados-iniciais.sql` | as categorias, configurações e regras padrão de um banco novo |

O site aplica os dois ao iniciar (`-aplicar-esquema`), com uma cópia embutida no executável, e o `scripts/geral/2-criar-banco.sh` os roda pelo `sqlite3`.

**Não mexer sem ler o guia:**
- tabela nova vai no **fim** do `01-estrutura.sql` (a CI confere com o `scripts/ci/banco/verificar-sql.py`);
- coluna nova numa tabela que já existe vai no `2-criar-banco.sh` (`adicionar_coluna_se_ausente`), e não aqui;
- nunca apagar nem renomear coluna em uso;
- depois de mudar, `go generate ./...` em `src/backend`, para a cópia embutida e o `sqlc` ficarem em dia.
