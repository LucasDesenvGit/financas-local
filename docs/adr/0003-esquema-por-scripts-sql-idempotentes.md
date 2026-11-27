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

Não usamos EF Core Migrations. O esquema vive em scripts SQL idempotentes, que o site aplica ao iniciar (`-aplicar-esquema`; no .NET, o `FinancasDbContext` era mantido em sincronia à mão).

## Consequências

- **Positivas:** um único caminho para criar e atualizar; o script pode ser rodado quantas vezes for preciso; o esquema é lido direto em SQL.
- **Negativas:**
  - entidade, `FinancasDbContext` e SQL podem divergir sem nenhum erro de compilação — a divergência só aparece em tempo de execução;
  - coluna nova em tabela existente não é idempotente em SQL puro no SQLite: a checagem fica no Go (`ColunasNovas` e `AplicarEsquema`, via `PRAGMA table_info`; até a #723 era o `adicionar_coluna_se_ausente` do `2-criar-banco.sh`);
  - não há histórico automático de qual versão do esquema cada banco tem.
- **Atualização de 2026-10-17 (#119, instalador .msi):** a própria API aplica os mesmos scripts ao iniciar (`Data/EsquemaBanco.cs`), para que a instalação pelo .msi não precise de `sqlite3` nem de bash.
  - **Fontes, embutidas no build:** `01-estrutura.sql`, as colunas novas (`esquema.ColunasNovas`, em Go; até a #723, as linhas `adicionar_coluna_se_ausente` do `2-criar-banco.sh`), `02-dados-iniciais.sql` e `03-tabelas-oficiais.sql`. No site, o 02 só entra no banco novo, porque rodá-lo de novo recriaria categorias apagadas; o 03 (tabelas do INSS e do IRRF, ADR 0065) entra a cada partida, também com a assinatura em dia, e fica fora dela: uma vigência nova não gera cópia (#805).
  - **`PRAGMA user_version`** guarda um número tirado do conteúdo da estrutura e da lista de colunas. Quando ele é igual, a API não faz nada. Quando muda, ela copia o banco em `dados/backups` (`FinancasLocal-antes-atualizacao-*.db`, guarda as 5 mais recentes) e aplica tudo numa transação.
  - Continua sem histórico de versões; esse número diz só "mesmos scripts da última vez".
  - Os scripts de instalação aplicam o esquema pelo mesmo executável (abaixo): rodar os dois caminhos não atrapalha, porque tudo é idempotente.
  - Custo aceito: um erro de SQL nos scripts agora também impede a API de subir, com o erro no log. É melhor do que atender com o esquema pela metade.
- **Atualização de 2026-11-26 (#723):** os scripts deixam de aplicar o esquema com `sqlite3` e de ler a lista de colunas: o `2-criar-banco.sh` e o `instalar-windows.ps1` chamam `financas-go aplicar-esquema -banco <caminho>` (o Go, via `go run`, porque a etapa 2 vem antes da publicação). A decisão (sem migrations, scripts idempotentes) é a mesma; muda o "como". Os dados iniciais continuam rodando a cada vez pelos scripts, também em banco existente (o subcomando os aplica depois do esquema); o site, ao iniciar, os aplica só no banco novo. O `.msi` não muda: o serviço já aplicava o esquema ao iniciar.
- **Na prática:** toda mudança de estrutura atualiza `banco/01-estrutura.sql` (e `ColunasNovas`, se for coluna nova), roda `go generate ./...` em `src/backend` e `scripts/ci/banco/verificar-sql.py` e usa `feat(banco)!:` no commit. Passo a passo em [BANCO-DE-DADOS.md](../BANCO-DE-DADOS.md).
