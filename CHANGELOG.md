# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e [versionamento semântico](https://semver.org/lang/pt-BR/).

## [Não publicado]

### Funcionalidades
- Empréstimos e financiamentos importados pelo Open Finance (`GET /loans` da Pluggy): saldo devedor, parcelas, taxa, CET e histórico de pagamentos de cada contrato.
- Nova página **Empréstimos e financiamentos**, com financiamentos (imóvel, veículo, bens) e empréstimos sempre separados: totais, contratos e pagamentos por mês.
- Painel: patrimônio líquido passa a descontar o saldo devedor de financiamentos e de empréstimos (cada um em seu cartão), com alerta de parcelas em atraso.
- Visão anual: o comprometimento da renda soma as parcelas de cartão, financiamentos e empréstimos, mostradas separadamente.

### Banco
- Novas tabelas `Emprestimo` e `PagamentoEmprestimo` em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh` antes de publicar.

### Documentação
- Pasta `docs/` restaurada (fora apagada por engano na migração para SQLite) e atualizada para SQLite/Kestrel e scripts bash.
- Decisões de arquitetura passam a ser registradas como ADRs em `docs/adr/`, com as decisões já tomadas registradas retroativamente.

### Infraestrutura
- Backend migrado de SQL Server Express/IIS para SQLite/Kestrel — o projeto agora roda em Windows e Linux com os mesmos scripts.
- Nova ferramenta `Financas.MigrarSqlite` para migrar os dados de uma instalação antiga com SQL Server para o SQLite novo.
- Scripts `.sh` (bash) substituindo os `.ps1` de instalação, criação do banco, publicação e backup.

## [1.0.0] — 2026-10-15

Primeira versão versionada no Git.

### Funcionalidades
- Painel com patrimônio líquido (com e sem renda variável), saldos, faturas, receitas × despesas, gastos por categoria, orçamentos, vencimentos e alertas.
- Open Finance via Pluggy/Meu Pluggy: contas, cartões, faturas, transações (`/v2/transactions`) e investimentos, com sincronização agendada.
- Transações com filtros, categorização em lote, regras automáticas, sugestões de categoria pelo histórico e marcação de gastos de emergência.
- Contas e cartões (limite efetivo, faturas), orçamentos com destaque laranja para estouro por emergência, metas, contas a pagar.
- Relatórios: fluxo mensal com emergências, categorias, maiores despesas, projeção mês a mês e visão anual com indicadores de saúde financeira.
- Investimentos: carteira de renda variável (preço médio, proventos, DY, yield on cost), comparação com a Selic, aba de FIIs com gráficos, importação pelo Open Finance e pelo extrato.
- Calculadora flutuante, modo escuro, opção de ocultar valores.

### Infraestrutura
- API .NET 10 no IIS (in-process), Angular 20, SQL Server 2025 Express local.
- Scripts de instalação, criação do banco (sem migrations), publicação e backup.
