# 0002. SQLite e Kestrel no lugar de SQL Server Express e IIS

- **Status:** Aceita (em parte substituída pela [0012](0012-instalador-do-windows-em-powershell.md): no Windows, a instalação é em PowerShell)
- **Data:** 2026-10-15
- **Relacionadas:** [0003](0003-esquema-por-scripts-sql-idempotentes.md)

## Contexto

A primeira versão (v1.0.0) rodava só no Windows: API hospedada no IIS (in-process) e banco no SQL Server Express (instância `.\FINANCAS`), com acesso ao banco pela identidade do pool do IIS. O objetivo passou a ser rodar o mesmo sistema também no Linux, com os mesmos scripts, sem depender de instalar e administrar uma instância de banco nem um servidor web.

O sistema tem um único usuário, roda numa única máquina e só escuta em `127.0.0.1` — não há concorrência de escrita relevante nem acesso remoto ao banco.

## Alternativas consideradas

- **Manter SQL Server + IIS** — nada a migrar; mas IIS não existe no Linux e o SQL Server para Linux é pesado e exige administração de instância para um sistema de um usuário.
- **PostgreSQL + Kestrel** — multiplataforma e robusto; mas continua sendo um serviço de banco para instalar, atualizar, fazer backup e proteger com senha.
- **SQLite + Kestrel** — o banco é um arquivo; o Kestrel já vem no .NET e roda como serviço com `UseSystemd`/`UseWindowsService`.

## Decisão

Usamos SQLite (arquivo `dados/sql/FinancasLocal.db`) e hospedamos a API direto no Kestrel, registrada como serviço do sistema (systemd `--user` no Linux, `sc.exe` no Windows), com a automação em scripts bash.

## Consequências

- **Positivas:** mesma instalação em Windows e Linux; nada de instância de banco nem senha de banco — o acesso é a permissão de arquivo sobre `dados/`; backup é copiar o arquivo com segurança (`sqlite3 .backup`).
- **Negativas:**
  - o SQLite não tem `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, `INCLUDE` em índice nem tipos decimais nativos — decimais e datas são gravados como `TEXT`, e a precisão vale pelo mapeamento do EF Core;
  - as chaves estrangeiras dependem de `PRAGMA foreign_keys = ON` em cada conexão (o EF Core já envia);
  - escrita é serializada no arquivo: suficiente para um usuário, não para um cenário multiusuário;
  - instalações antigas precisam migrar os dados com `Financas.MigrarSqlite`, e os segredos têm de ser informados de novo (ver [0005](0005-segredos-criptografados-com-data-protection.md)).
- **Na prática:** nada de sintaxe de T-SQL em `banco/*.sql` (o CI recusa); colunas novas seguem o processo de [BANCO-DE-DADOS.md](../BANCO-DE-DADOS.md).
