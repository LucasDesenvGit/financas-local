# Banco de dados

O projeto **não usa migrations**. A estrutura completa vive em scripts SQL idempotentes, que podem ser executados quantas vezes for preciso:

| Arquivo | Conteúdo |
|---|---|
| `banco/01-estrutura.sql` | Tabelas, índices e restrições. Organizado em blocos por versão (`v4`, `v5`, `v6`, `v7`...). |
| `banco/02-dados-iniciais.sql` | Categorias padrão, configurações e regras de categorização iniciais. |
| `banco/03-permissoes.sql` | Login e permissões do pool do IIS (somente leitura/escrita de dados). |

`scripts/2-criar-banco.ps1` executa os três, em ordem.

## Como alterar a estrutura

1. Adicione **no fim** de `01-estrutura.sql` um bloco novo, com cabeçalho da versão e sempre protegido por verificação de existência:

   ```sql
   /* ------------------------------------------------------------------ */
   /* Assunto (v8): descrição curta                                       */
   /* ------------------------------------------------------------------ */
   IF COL_LENGTH(N'dbo.Tabela', N'NovaColuna') IS NULL
       ALTER TABLE dbo.Tabela ADD NovaColuna BIT NOT NULL CONSTRAINT DF_Tabela_NovaColuna DEFAULT 0;
   GO

   IF OBJECT_ID(N'dbo.NovaTabela', N'U') IS NULL
   CREATE TABLE dbo.NovaTabela ( ... );
   GO
   ```

2. Atualize a entidade em `Entidades/` e o mapeamento em `Data/FinancasDbContext.cs` (tamanhos e precisão iguais ao SQL).
3. Rode `scripts\2-criar-banco.ps1` e depois `scripts\3-publicar.ps1`.
4. No commit, use `feat(banco)!:` para indicar que é preciso rodar o script do banco.

Regras:
- Nunca apague ou renomeie colunas em uso sem um bloco de transição.
- `GO` separa os lotes (o script de instalação divide o arquivo por ele).
- Restrições e defaults sempre com nome explícito (`DF_`, `CK_`, `FK_`, `UX_`, `IX_`).
