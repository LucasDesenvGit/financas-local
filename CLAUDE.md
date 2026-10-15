# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## O que é o projeto

Sistema de gerenciamento financeiro pessoal que roda inteiro localmente, em Windows ou Linux com os mesmos scripts bash: API em C#/.NET 10 hospedada via Kestrel (sem IIS — `UseWindowsService`/`UseSystemd` em `Program.cs`), front-end Angular 20, banco SQLite local (`dados/sql/FinancasLocal.db`). Dados bancários chegam via Open Finance através do **Meu Pluggy**. Veja [README.md](README.md) para a visão funcional completa (painel, transações, orçamentos, metas, investimentos, relatórios) e [docs/ARQUITETURA.md](docs/ARQUITETURA.md) para o diagrama de camadas.

## Comandos

```bash
# Build
dotnet build FinancasLocal.sln -c Release      # back-end (Financas.Api + Financas.MigrarSqlite)
cd src/frontend && npm run build                # front-end (produção)
cd src/frontend && npm start                    # front-end em dev (http://localhost:4200, proxy /api -> API)

# CI roda também:
python3 scripts/ci/verificar-sql.py             # confere que banco/01-estrutura.sql é idempotente

# Banco (sem migrations)
scripts/2-criar-banco.sh                        # roda os scripts de banco/ em ordem + ALTER TABLE pendentes (PRAGMA table_info)

# Publicação
scripts/3-publicar.sh                           # compila front+API, publica, cuida de certificado e do serviço do sistema

# Migrar dados de uma instalação antiga (SQL Server/IIS) para o SQLite novo — seguro rodar mesmo sem SQL Server disponível
dotnet run --project src/backend/Financas.MigrarSqlite -- "<connection string SQL Server origem>" "<caminho .db destino>"
```

Não há projeto de testes automatizados no repositório (sem `*.Tests.csproj`, sem Jasmine/Karma configurado). Validação é feita por build + checklist manual do PR (ver [CONTRIBUINDO.md](CONTRIBUINDO.md)).

Em desenvolvimento no Visual Studio, o perfil *Desenvolvimento (Kestrel)* sobe a API em `https://localhost:7443`; as chaves do Data Protection ficam em `dados\chaves-dev` (Client Secret da Pluggy precisa ser reinformado).

## Arquitetura (resumo — detalhes em docs/ARQUITETURA.md)

Angular é publicado dentro do `wwwroot` da própria API: mesma origem, sem CORS, cookies `__Host-`.

**Back-end** (`src/backend/Financas.Api`):
- `Controllers/` — endpoints finos (`BaseController` padroniza erros); um controller pode cobrir múltiplos recursos relacionados (ex.: `PlanejamentoControllers.cs` reúne orçamentos/metas/contas a pagar).
- `Servicos/` — regra de negócio: `RelatorioService` (painel/orçamentos/projeção/visão anual), `CategorizacaoService` (regras automáticas), `SugestaoCategoriaService`, `ContaService`, `ConfiguracaoService`, `AuditoriaService`, `Relogio` (hora sempre em fuso de Brasília).
- `Servicos/Pluggy/` — cliente HTTP próprio (sem SDK) para Open Finance; `SincronizacaoHostedService` agenda `SincronizacaoService` periodicamente (endpoint `/v2/transactions`, paginação por cursor).
- `Servicos/Investimentos/` — carteira de renda variável: `CarteiraService`, `CotacaoService` (Yahoo Finance/brapi), `SelicService` (api.bcb.gov.br), `FiiService`, `ExtratoCarteiraService` (importa compras/vendas a partir da descrição de transações).
- `Entidades/` — classes mapeadas no EF Core (`Financeiro.cs`, `Carteira.cs`, `OpenFinance.cs`, `Seguranca.cs`, `Enums.cs`).
- `Data/FinancasDbContext.cs` — único lugar com nomes/tamanhos/precisão/índices das tabelas; deve ficar em sincronia manual com `banco/01-estrutura.sql`.
- `Dtos/Dtos.cs` — todos os DTOs num arquivo único.
- `Seguranca/` — cabeçalhos de segurança, validação de sessão, constantes de cookies.

**Front-end** (`src/frontend/src/app`):
- `core/` — `ApiService` (único ponto de chamadas HTTP), modelos TS, autenticação, tema, privacidade (ocultar valores).
- `compartilhado/` — componentes reutilizáveis (gráfico Chart.js, modal, paginação, calculadora flutuante, pipe de moeda).
- `layout/` — casca da tela (menu lateral).
- `paginas/` — uma página por rota.
- Componentes standalone, signals e control flow (`@if`/`@for`).

**Banco** (`banco/`) — **sem EF Core Migrations**, de propósito, em SQLite:
- `01-estrutura.sql` — tabelas/índices/restrições com `CREATE TABLE/INDEX IF NOT EXISTS` (idempotência nativa do SQLite), organizado em blocos sequenciais por versão (`v4`, `v5`, ...) — verificado pelo CI via `scripts/ci/verificar-sql.py` e aplicado de fato a um banco SQLite descartável no CI. Nunca apagar/renomear coluna em uso sem bloco de transição.
- `02-dados-iniciais.sql` — categorias/config/regras padrão.
- Coluna nova numa tabela existente: SQLite não tem `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, então essa idempotência fica em `scripts/2-criar-banco.sh` (checagem via `PRAGMA table_info`), não no SQL.
- Ao mudar a estrutura: atualizar `Entidades/` + `FinancasDbContext.cs`, rodar `2-criar-banco.sh`, e usar `feat(banco)!:` no commit. Passos detalhados em [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md).

## Convenções de código

- Nomes em português em todo o código (`XxxService`, `XxxController`, `XxxDto`, `SalvarXxxDto`).
- Consultas ao banco **só via LINQ/lambda** sobre entidades mapeadas (nunca SQL montado a partir de texto do usuário); atualizações em massa com `ExecuteUpdate`/`ExecuteDelete`.
- Segredos (Client Secret da Pluggy, token da brapi) ficam **criptografados na tabela `Configuracao`** via ASP.NET Data Protection — nunca em arquivo de config nem devolvidos pela API.
- Front-end chama a API só pelo `ApiService` (não usar `HttpClient` direto nos componentes).
- Formatação: `.editorconfig`.

## Git / commits

Conventional Commits em português (ver [CONTRIBUINDO.md](CONTRIBUINDO.md) para fluxo de branches completo). Escopos usados: `api`, `web`, `banco`, `scripts`, `open-finance`, `investimentos`, `relatorios`. Branch `main` é protegida (só recebe merge de `develop`/`hotfix/*`); trabalho novo parte de `develop` em `feature/<assunto>` ou `fix/<assunto>`.

## Segurança (específico deste projeto — não genérico)

Tudo roda local, sem exposição de rede (bind só em `127.0.0.1:5443`, direto pelo Kestrel — sem IIS/proxy). Pontos que afetam como escrever código aqui:
- Acesso ao banco é só permissão de arquivo do SO sobre `dados/` — SQLite não tem login/usuário de banco; quem lê/grava é o processo da API, rodando como a conta do serviço do sistema (systemd no Linux, Serviço do Windows no Windows). Sem senha de banco em lugar nenhum.
- Segredos (Client Secret da Pluggy, token da brapi) via ASP.NET Data Protection, chaves em `dados/chaves`: DPAPI no Windows (`OperatingSystem.IsWindows()` em `Program.cs`), permissão de arquivo no Linux.
- CSP restritiva: nada de scripts/recursos de terceiros carregados do front-end.
- Saídas de rede permitidas e já implementadas: `api.pluggy.ai`, `api.bcb.gov.br`, `query1.finance.yahoo.com`/`brapi.dev` — qualquer integração externa nova deve seguir o mesmo padrão (HTTPS, dado mínimo enviado, ver tabela em [README.md](README.md#segurança)).
