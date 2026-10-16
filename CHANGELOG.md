# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e [versionamento semântico](https://semver.org/lang/pt-BR/).

## [Não publicado]

### Funcionalidades
- Transações: filtro de **várias categorias** de uma vez, com caixas de marcar (cada categoria inclui as subcategorias, e dá para somar "Sem categoria"). Um chip por categoria em "Filtrando:", cada um com o seu ✕; na URL, `?categoria=12,20,sem` (links antigos com uma categoria continuam valendo). A API aceita `categoriaIds` repetido, mantendo `categoriaId`.
- Empréstimos e financiamentos importados pelo Open Finance (`GET /loans` da Pluggy): saldo devedor, parcelas, taxa, CET e histórico de pagamentos de cada contrato.
- Nova página **Empréstimos e financiamentos**, com financiamentos (imóvel, veículo, bens) e empréstimos sempre separados: totais, contratos e pagamentos por mês.
- Painel: patrimônio líquido passa a descontar o saldo devedor de financiamentos e de empréstimos (cada um em seu cartão), com alerta de parcelas em atraso.
- Câmbio: cotação PTAX oficial do Banco Central (`GET /api/cambio/moedas` e `GET /api/cambio/cotacao?moeda=USD&data=2026-10-15`), guardada no banco para uso offline (`GET /api/cambio/cotacoes`) e atualizada automaticamente uma vez por dia e de novo depois do fechamento do Banco Central (~13h30).
- Investimentos em moeda estrangeira (Open Finance) entram no patrimônio, na reserva e nos totais da carteira convertidos para reais; a tela de investimentos mostra o valor original e o convertido. Moeda sem cotação fica fora dos totais, com aviso.
- Ativos no exterior na carteira (ex.: AAPL, VOO): em **+ Lançar compra/venda**, escolha a moeda; o câmbio vem preenchido com a PTAX do dia e pode ser trocado pelo da corretora. Custo e resultado em reais por operação, valor atual pelo câmbio de hoje e proventos pelo câmbio da data-com.
- Visão anual: o comprometimento da renda soma as parcelas de cartão, financiamentos e empréstimos, mostradas separadamente.
- Configurações em abas (**Open Finance**, **Parametrização**, **Segurança**), com link direto por `?aba=`.
- Nova aba **Parametrização**: sincronização automática, intervalo, dias relidos e histórico da primeira carga; dias de próximos vencimentos e de aviso de consentimento no painel; tentativas de login e tempo de bloqueio. Cada parâmetro mostra a faixa válida e o padrão, vale na hora (sem reiniciar) e é validado pela API (`GET/PUT /api/parametros`). Bloco **Sistema** somente leitura com versão, caminhos, tamanho do banco e totais (`GET /api/parametros/sistema`).
- Tentativas de login e tempo de bloqueio saem do `appsettings.json` para a Parametrização; o valor do `appsettings.json`, se existir, vira o padrão.
- Reservas do Mercado Pago ("cofrinhos"), cujo saldo não vem pelo Open Finance: estimadas pelos lançamentos "Dinheiro reservado/retirado" do extrato, com conferência opcional do saldo real na nova aba **Investimentos › Reservas**. Entram no total investido, no patrimônio e na reserva de emergência. Reservas que precisam ser conferidas (possível saldo anterior à janela importada ou conferência com mais de 30 dias) são destacadas e geram alerta no Painel (#25, ADR 0010).
- Nova aba **Aparência**: paletas prontas (Lavanda, Oceano, Floresta, Grafite, Pôr do sol), cada uma com versão clara e escura, ou uma cor personalizada (tons calculados para manter contraste de 4,5:1), com prévia. Verde, vermelho, laranja de emergência e as cores de financiamento/empréstimo não mudam. Fica salva no navegador.
- Ícones do [Lucide](https://lucide.dev) (licença ISC, empacotados no build) no menu, nos botões de sincronizar, ocultar valores e tema, e nos cartões do painel.
- Novo logo e ícone do site (cofrinho), com versões PNG para atalho no celular e na área de trabalho e `manifest.webmanifest`.
- **Contas a pagar** sugere cadastrar os pagamentos que se repetem todo mês no extrato (boleto, débito automático, Pix), com vencimento estimado pelo dia habitual e valor pela mediana; cada sugestão pode ser cadastrada (formulário preenchido), conferida em Transações ou ignorada (`GET /api/contas-agendadas/sugestoes`, #62).

### Correções (layout)
- Avisos de lista vazia (ex.: "Nenhum empréstimo ou financiamento encontrado.") ficavam fora do meio do cartão em telas largas: o texto era centralizado dentro de uma caixa de 78 caracteres encostada à esquerda. Agora a caixa também é centralizada, em todas as telas (#46).
- Contraste da cor principal da paleta Lavanda (texto dos botões) passa de 3,5:1 para 4,7:1.
- Arquivos estáticos sem hash no nome (`favicon.svg`, ícones, manifesto) deixam de ficar em cache por um ano: antes, uma troca de ícone não aparecia no navegador.

### Banco
- Novas tabelas `Emprestimo` e `PagamentoEmprestimo` em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh` antes de publicar.
- Nova tabela `CotacaoMoeda` (cache das cotações de câmbio) em `banco/01-estrutura.sql`.
- Novas colunas `Ativo.Moeda` e `OperacaoAtivo.Cambio`: rode `scripts/2-criar-banco.sh` (primeiro uso de `adicionar_coluna_se_ausente`).
- Nova tabela `ReservaConferencia` (saldo conferido das reservas) em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh`.

### Correções
- Janelas sobrepostas (ex.: editar uma operação a partir da lista de operações): a aberta por último fica por cima, e o Esc fecha só ela — antes, a edição abria atrás e um Esc fechava todas.
- Investimentos: a tela não carregava (erro 500) quando o Banco Central ou outra fonte demorava mais que o tempo limite; agora segue sem o dado, como já fazia sem internet. Mesmo ajuste nas cotações, no câmbio, na importação pelo extrato e na sincronização do Open Finance.
- Publicação no Linux: `scripts/3-publicar.sh` passa a se chamar `scripts/3-publicar-atualizar.sh` e reinicia o serviço quando ele já está rodando (antes, continuava com a versão antiga em memória).
- Instalação num Windows novo: o `1-preparar-ambiente.sh` deixa de depender de Python (que no Windows costuma ser só o atalho da Microsoft Store) e de `unzip`, baixa o `sqlite3` portátil oficial quando falta, confere as ferramentas do terminal antes de começar e mostra as versões instaladas no final. O `3-publicar-atualizar.sh` também não usa mais Python, funciona com `--pular-build` sem o .NET no PATH e avisa logo no início quando o Git Bash não está como Administrador (ADR 0011).
- Instalação em um passo: `scripts/instalar.sh` roda as três etapas em sequência. No Windows, basta dar dois cliques em `instalar-windows.cmd`, que pede Administrador e roda `scripts/instalar-windows.ps1` — só PowerShell, sem Git, bash nem Python (ADR 0012).
- Publicação no Windows: o serviço é parado antes de copiar os arquivos novos (antes, a cópia falhava com as DLLs em uso).
- Instalador do Windows: o log guarda também as mensagens de erro do npm/dotnet e os eventos do Windows quando a API não sobe; o erro final diz em que etapa parou; API que não responde em 30 s reprova a instalação (antes, terminava em verde); certificado confiável para todas as contas do computador, sem janela de confirmação; clique na janela não congela mais a instalação (QuickEdit desligado); no fim, abre o site e fecha sozinho (só fica aberto se der erro); o log acumula as execuções.
- Atualização mais segura (Windows e bash): cópia do banco em `backups/FinancasLocal-antes-instalacao-*.db` antes de mudar a estrutura (ficam as 5 mais recentes), `sqlite3` espera em vez de falhar com `database is locked` se a API estiver gravando; no Windows, se a versão nova não subir, a anterior volta sozinha.
- Backup e criação do banco funcionam em pastas com apóstrofo no caminho (ex.: `C:\Users\D'Avila`).
- Backup agendado no Windows (`4-backup.sh --agendar`): a tarefa era criada com caminhos do Git Bash (`/usr/bin/bash`, `/c/...`), que o Agendador de Tarefas não entende, e não rodava. Agora usa caminhos do Windows e o `bash.exe` do Git for Windows.
- Validação do código da B3 volta ao formato estrito (ex.: PETR4, MXRF11); ativos de fora da B3, como AOK, entram escolhendo a moeda (ADR 0009).

### Documentação
- Pasta `docs/` restaurada (fora apagada por engano na migração para SQLite) e atualizada para SQLite/Kestrel e scripts bash.
- Decisões de arquitetura passam a ser registradas como ADRs em `docs/adr/`, com as decisões já tomadas registradas retroativamente.

### Infraestrutura
- Testes automatizados do front-end com **Jest** (`npm test` em `src/frontend`): 84 testes de regressão (filtros de Transações na URL, balão do glossário dentro da tela, utilidades de datas e links) e de usabilidade (nome e estado acessível, teclado e Esc, "Mais análises" que só cria o conteúdo ao abrir, filtros ativos visíveis, status da sincronização anunciado). Rodam na CI (ADR 0013).
- Novo workflow **Teste de instalação no Linux**: roda `scripts/instalar.sh` num runner Ubuntu sem dotnet, node, sqlite3 e Python no PATH, confere o serviço `systemd --user` e a API, testa a atualização e o `4-backup.sh`.
- Novo workflow **Teste de instalação no Windows**: instala do zero num runner Windows (sem as ferramentas do runner no PATH), confere o serviço e a API, e testa a atualização pelo `instalar-windows.cmd`.
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
