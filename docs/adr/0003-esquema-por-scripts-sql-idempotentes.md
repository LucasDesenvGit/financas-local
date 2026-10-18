# 0003. Esquema do banco por scripts SQL idempotentes, sem migrations

- **Status:** Aceita
- **Data:** 2026-10-15 (atualizada em 2026-10-17: a API também aplica os scripts ao iniciar, #119)
- **Relacionadas:** [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md)

## Contexto

O banco é criado e atualizado na máquina do usuário, pelos scripts de instalação, por alguém que não necessariamente tem o SDK do .NET à mão nem conhece EF Core. O mesmo script precisa servir para criar um banco novo e para atualizar um existente, podendo ser rodado de novo sem perder dados.

*Inferido:* a escolha também mantém o esquema legível e revisável em SQL puro no pull request, em vez de espalhado em classes de migration geradas.

## Alternativas consideradas

- **EF Core Migrations** — gera o SQL a partir do modelo e guarda o histórico; mas aplicar exige a ferramenta do EF ou código de migração na subida da API, e as migrations geradas para SQLite recriam tabelas em várias alterações.
- **Ferramenta de versionamento de esquema (DbUp, Flyway)** — histórico explícito; mas é mais uma dependência para um banco de um usuário.
- **Scripts SQL idempotentes** — `CREATE TABLE/INDEX IF NOT EXISTS` em `banco/01-estrutura.sql` e dados iniciais em `banco/02-dados-iniciais.sql`, aplicados pelo `scripts/2-criar-banco.sh`.

## Decisão

Não usamos EF Core Migrations. O esquema vive em scripts SQL idempotentes, e o `FinancasDbContext` é mantido em sincronia manual com eles.

## Consequências

- **Positivas:** um único caminho para criar e atualizar; o script pode ser rodado quantas vezes for preciso; o esquema é lido direto em SQL.
- **Negativas:**
  - entidade, `FinancasDbContext` e SQL podem divergir sem nenhum erro de compilação — a divergência só aparece em tempo de execução;
  - coluna nova em tabela existente não é idempotente em SQL puro no SQLite: a checagem fica no bash (`adicionar_coluna_se_ausente` em `scripts/comum.sh`, via `PRAGMA table_info`);
  - não há histórico automático de qual versão do esquema cada banco tem.
- **Atualização de 2026-10-17 (#119, instalador .msi):** a própria API aplica os mesmos scripts ao iniciar (`Data/EsquemaBanco.cs`), para que a instalação pelo .msi não precise de `sqlite3` nem de bash.
  - **Fontes, embutidas no build:** `01-estrutura.sql`, as colunas novas das linhas `adicionar_coluna_se_ausente` do `2-criar-banco.sh` (o mesmo formato que o `instalar-windows.ps1` lê; a lista continua num lugar só) e `02-dados-iniciais.sql`. Este último só entra no banco novo, porque rodá-lo de novo recriaria categorias apagadas.
  - **`PRAGMA user_version`** guarda um número tirado do conteúdo da estrutura e da lista de colunas. Quando ele é igual, a API não faz nada. Quando muda, ela copia o banco em `dados/backups` (`FinancasLocal-antes-atualizacao-*.db`, guarda as 5 mais recentes) e aplica tudo numa transação.
  - Continua sem histórico de versões; esse número diz só "mesmos scripts da última vez".
  - Os scripts de instalação seguem funcionando como antes: rodar os dois caminhos não atrapalha, porque tudo é idempotente.
  - Custo aceito: um erro de SQL nos scripts agora também impede a API de subir, com o erro no log. É melhor do que atender com o esquema pela metade.
- **Na prática:** toda mudança de estrutura atualiza `Entidades/`, `FinancasDbContext.cs` e `banco/01-estrutura.sql` juntos, roda `scripts/ci/verificar-sql.py` e usa `feat(banco)!:` no commit. Passo a passo em [BANCO-DE-DADOS.md](../BANCO-DE-DADOS.md).
