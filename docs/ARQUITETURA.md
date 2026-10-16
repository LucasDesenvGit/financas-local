# Arquitetura

```
Navegador (Angular)  ──HTTPS 127.0.0.1:5443──►  Financas.Api (.NET 10, Kestrel)
                                                   │
                                                   ├─► SQLite (dados/sql/FinancasLocal.db, arquivo local)
                                                   ├─► api.pluggy.ai          (Open Finance)
                                                   ├─► api.bcb.gov.br         (Selic)
                                                   ├─► olinda.bcb.gov.br      (câmbio — PTAX)
                                                   └─► Yahoo Finance/brapi    (cotações)
```

A API sobe direto pelo Kestrel (sem IIS nem proxy) e roda como serviço do sistema: `UseSystemd` no Linux e `UseWindowsService` no Windows, com os mesmos scripts bash nos dois.

O Angular é publicado dentro da própria API (`wwwroot`), então site e API têm a mesma origem: sem CORS, cookies `__Host-` e proteção CSRF simples.

## Back-end — `src/backend/Financas.Api`

| Pasta | Responsabilidade |
|---|---|
| `Controllers/` | Endpoints REST (`/api/...`). Finos: validam a entrada e chamam os serviços. `BaseController` padroniza erros. Um controller pode reunir recursos relacionados (ex.: `PlanejamentoControllers.cs` com orçamentos, metas e contas a pagar). |
| `Servicos/` | Regras de negócio. `RelatorioService` (painel, orçamentos, projeção, visão anual), `CategorizacaoService` (regras automáticas), `SugestaoCategoriaService` (sugestões pelo histórico), `ContaService` (saldos), `EmprestimoService` (empréstimos e financiamentos), `ConfiguracaoService`, `AuditoriaService`, `Relogio` (fuso de Brasília). |
| `Servicos/Pluggy/` | Cliente HTTP da Pluggy (sem SDK), modelos e sincronização (agendada pelo `SincronizacaoHostedService`). |
| `Servicos/Investimentos/` | Carteira de renda variável, cotações (`CotacaoService`), Selic (`SelicService`), FIIs (`FiiService`), importação pelo extrato (`ExtratoCarteiraService`) e reservas estimadas pelo extrato (`ReservaService`). |
| `Servicos/Cambio/` | Cotação PTAX de moedas estrangeiras (`CambioService`), guardada no banco para uso offline e atualizada pelo `CambioHostedService`. |
| `Entidades/` | Classes mapeadas no EF Core. |
| `Data/FinancasDbContext.cs` | Mapeamento das tabelas (nomes, tamanhos, precisão, índices) — em sincronia manual com `banco/01-estrutura.sql`. |
| `Dtos/` | Contratos de entrada e saída da API (arquivo único `Dtos.cs`). |
| `Seguranca/` | Cabeçalhos de segurança (CSP), validação de sessão, constantes de cookies. |

Decisões (o porquê de cada uma, com alternativas e custos, está nas [ADRs](adr/README.md)):
- **SQLite + Kestrel**, sem SQL Server nem IIS ([ADR 0002](adr/0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md)).
- **Sem migrations.** O banco é criado e atualizado por scripts SQL idempotentes ([ADR 0003](adr/0003-esquema-por-scripts-sql-idempotentes.md); passo a passo em [BANCO-DE-DADOS.md](BANCO-DE-DADOS.md)).
- **Front-end no `wwwroot` da API**, mesma origem ([ADR 0004](adr/0004-front-end-publicado-no-wwwroot-da-api.md)).
- **Cliente HTTP próprio** para a Pluggy e as demais integrações ([ADR 0006](adr/0006-cliente-http-proprio-para-a-pluggy.md)).
- **Câmbio pela PTAX** do Banco Central, guardado no banco para uso offline ([ADR 0007](adr/0007-cotacao-de-cambio-pela-ptax-do-banco-central.md)). Só investimentos respeitam moeda estrangeira; o resto é em reais ([ADR 0008](adr/0008-moeda-estrangeira-somente-em-investimentos.md)).
- **Reservas sem saldo no Open Finance** (cofrinhos do Mercado Pago) são estimadas pelos lançamentos "Dinheiro reservado/retirado" do extrato, com conferência manual opcional do saldo real ([ADR 0010](adr/0010-reservas-estimadas-pelo-extrato.md)).
- **Consultas só com LINQ/lambda** sobre entidades mapeadas; atualizações em massa com `ExecuteUpdate`/`ExecuteDelete`.
- **Segredos** (Client Secret da Pluggy, token da brapi) ficam criptografados na tabela `Configuracao` com Data Protection. Chaves em `dados/chaves`: protegidas por DPAPI no Windows e por permissão de arquivo no Linux ([ADR 0005](adr/0005-segredos-criptografados-com-data-protection.md)).
- **Datas** sempre no fuso de Brasília (`Relogio`).
- **Acesso ao banco** é só permissão de arquivo do sistema sobre `dados/`: o SQLite não tem login nem usuário; quem lê e grava é o processo da API, rodando com a conta do serviço. Não há senha de banco em lugar nenhum.

## Front-end — `src/frontend`

| Pasta | Responsabilidade |
|---|---|
| `src/app/core/` | `ApiService` (todas as chamadas HTTP), modelos (interfaces TypeScript), autenticação, tema, privacidade. |
| `src/app/compartilhado/` | Componentes reutilizáveis: gráfico (Chart.js), modal, paginação, calculadora, pipe de moeda que respeita "ocultar valores". |
| `src/app/layout/` | Estrutura da tela (menu lateral). |
| `src/app/paginas/` | Uma página por rota. |
| `src/styles.css` | Tema (tokens de cor claros/escuros) e componentes visuais globais. |

## Ferramentas — `src/backend/Financas.MigrarSqlite`

Migra os dados de uma instalação antiga (SQL Server/IIS) para o SQLite. É seguro rodar sem SQL Server disponível: sem nada para migrar, termina sem erro. O Client Secret da Pluggy e o token da brapi são limpos na migração (foram criptografados com as chaves da instalação antiga) e precisam ser informados de novo.

```bash
dotnet run --project src/backend/Financas.MigrarSqlite -- "<connection string SQL Server origem>" "<caminho .db destino>"
```

## Scripts — `scripts/`

Em bash, para Linux, macOS e Windows (Git Bash/MSYS ou WSL). No Windows, a instalação também tem uma versão em PowerShell que dispensa o bash ([ADR 0012](adr/0012-instalador-do-windows-em-powershell.md)).

| Script | Função |
|---|---|
| `instalar.sh` | Roda as etapas 1, 2 e 3 em sequência e para no primeiro erro. |
| `instalar-windows.ps1` | Etapas 1, 2 e 3 no Windows PowerShell 5.1, sem bash; chamado por `instalar-windows.cmd` (na raiz), que pede elevação. Lê a lista de colunas novas do `2-criar-banco.sh`. Mudanças nas etapas 1 e 3 do bash precisam ser refletidas aqui. |
| `1-preparar-ambiente.sh` | Confere as ferramentas básicas do terminal; instala Node.js LTS, .NET 10 SDK e, se o sistema não tiver, `sqlite3` portáteis em `ferramentas/` (reaproveita um `dotnet` 10.x e um `sqlite3` do PATH). Não depende de Python ([ADR 0011](adr/0011-ferramentas-de-instalacao-portateis-sem-python.md)). |
| `2-criar-banco.sh` | Cria as pastas de `dados/` com permissão restrita e aplica os arquivos de `banco/` em ordem. |
| `3-publicar-atualizar.sh` | Compila front e API, publica em `publicado/api`, gera/reaproveita o certificado HTTPS local e registra o serviço (systemd `--user` no Linux, `sc.exe` no Windows). |
| `4-backup.sh` | Backup online do banco (`sqlite3 .backup`), com retenção e opção de agendar. |
| `comum.sh` | Funções compartilhadas pelos scripts (detecção de SO, pastas, `sqlite3` portátil no PATH, caminho nativo no Windows, extração de `.zip`, conferência de hash, helpers de coluna do banco). |
| `ci/verificar-sql.py` | Usado no CI: confere que `banco/*.sql` é idempotente e não tem sintaxe de T-SQL. |
| `git/configurar-github.ps1` | Cria o repositório no GitHub, envia as branches e aplica as proteções. |
