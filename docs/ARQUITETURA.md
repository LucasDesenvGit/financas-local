# Arquitetura

```
Navegador (Angular)  ──HTTPS 127.0.0.1:5443──►  IIS (in-process)  ──►  Financas.Api (.NET 10)
                                                                         │
                                                                         ├─► SQL Server Express (.\FINANCAS, só local)
                                                                         ├─► api.pluggy.ai        (Open Finance)
                                                                         ├─► api.bcb.gov.br       (Selic)
                                                                         └─► Yahoo Finance/brapi  (cotações)
```

O Angular é publicado dentro da própria API (`wwwroot`), então site e API têm a mesma origem: sem CORS, cookies `__Host-` e proteção CSRF simples.

## Back-end — `src/backend/Financas.Api`

| Pasta | Responsabilidade |
|---|---|
| `Controllers/` | Endpoints REST (`/api/...`). Finos: validam a entrada e chamam os serviços. `BaseController` padroniza erros. |
| `Servicos/` | Regras de negócio. `RelatorioService` (painel, orçamentos, projeção, visão anual), `CategorizacaoService` (regras automáticas), `SugestaoCategoriaService` (sugestões pelo histórico), `ContaService` (saldos). |
| `Servicos/Pluggy/` | Cliente HTTP da Pluggy (sem SDK), modelos e sincronização (agendada pelo `SincronizacaoHostedService`). |
| `Servicos/Investimentos/` | Carteira de renda variável, cotações (`CotacaoService`), Selic (`SelicService`), FIIs (`FiiService`) e importação pelo extrato. |
| `Entidades/` | Classes mapeadas no EF Core. |
| `Data/FinancasDbContext.cs` | Mapeamento das tabelas (nomes, tamanhos, precisão, índices). |
| `Dtos/` | Contratos de entrada e saída da API. |
| `Seguranca/` | Cabeçalhos de segurança, validação de sessão, constantes de cookies. |

Decisões:
- **Sem migrations.** O banco é criado e atualizado por scripts SQL idempotentes (ver [BANCO-DE-DADOS.md](BANCO-DE-DADOS.md)).
- **Consultas só com LINQ/lambda** sobre entidades mapeadas; atualizações em massa com `ExecuteUpdate`/`ExecuteDelete`.
- **Segredos** (Client Secret da Pluggy, token da brapi) ficam criptografados na tabela `Configuracao` com Data Protection (chaves protegidas por DPAPI em `dados\chaves`).
- **Datas** sempre no fuso de Brasília (`Relogio`).
- O acesso ao banco usa a identidade do pool do IIS (autenticação do Windows); não há senha de banco em arquivo.

## Front-end — `src/frontend`

| Pasta | Responsabilidade |
|---|---|
| `src/app/core/` | `ApiService` (todas as chamadas HTTP), modelos (interfaces TypeScript), autenticação, tema, privacidade. |
| `src/app/compartilhado/` | Componentes reutilizáveis: gráfico (Chart.js), modal, paginação, calculadora, pipe de moeda que respeita "ocultar valores". |
| `src/app/layout/` | Estrutura da tela (menu lateral). |
| `src/app/paginas/` | Uma página por rota. |
| `src/styles.css` | Tema (tokens de cor claros/escuros) e componentes visuais globais. |

## Scripts — `scripts/`

| Script | Função |
|---|---|
| `1-instalar-ambiente.ps1` | IIS, .NET (Hosting Bundle e SDK), Node.js portátil e SQL Server Express — baixados de fontes oficiais com verificação de hash/assinatura. |
| `2-criar-banco.ps1` | Cria/atualiza o banco rodando os arquivos de `banco/` em ordem. |
| `3-publicar.ps1` | Compila front e API, publica no IIS, configura certificado e bindings. |
| `4-backup.ps1` | Backup do banco (com opção de agendar). |
| `git/configurar-github.ps1` | Cria o repositório no GitHub, envia as branches e aplica as proteções. |
