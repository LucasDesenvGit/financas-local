# 0022. Erros do sistema gravados no banco e vistos no site

- **Status:** Aceita
- **Data:** 2026-10-16 (PR da issue #11)
- **Relacionadas:** [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (SQLite, serviço do sistema), [0003](0003-esquema-por-scripts-sql-idempotentes.md) (tabela nova por script), [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) (prazo de retenção editável), [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (filtros na URL)

## Contexto

Para investigar uma falha (sincronização da Pluggy, cotações, câmbio, erro 500 da API), era preciso abrir o Visualizador de Eventos do Windows ou o `journalctl` no Linux, como Administrador. O usuário não via que algo tinha falhado, a menos que a tela quebrasse (Nielsen 1, visibilidade do estado do sistema). Os erros do navegador só iam para o console.

Restrições:
- tudo roda local, sem serviço externo;
- os logs podem carregar dado sensível (o token da brapi vai em query string, as exceções trazem URLs e textos do extrato);
- o banco é um arquivo SQLite único, que não deve crescer sem controle;
- o mesmo código roda em Windows e Linux.

## Alternativas consideradas

- **Continuar só no log do sistema (Event Log/journald).**
  - Prós: nada a fazer.
  - Contras: invisível para o usuário e trabalhoso de ler.
- **Serilog com sink para SQLite ou arquivo, e uma tela que lê o arquivo.**
  - Prós: biblioteca madura.
  - Contras:
    - dependência nova só para isso;
    - arquivo de log fora do banco (backup e permissões à parte);
    - não agrupa repetições;
    - a limpeza de dados sensíveis teria que ser feita do mesmo jeito.
- **Serviço de observabilidade (Seq, OpenTelemetry, Sentry).**
  - Prós: busca e alertas prontos.
  - Contras: mais um processo ou um envio de dados para fora, o que contraria o "tudo local".
- **Provedor de log próprio que grava numa tabela, com limpeza dos textos e agrupamento.**
  - Prós:
    - usa o `ILogger` que o código já chama;
    - fica no mesmo banco e no mesmo backup;
    - sem dependência nova.
  - Contras: código próprio para manter.

## Decisão

Usamos um `ILoggerProvider` próprio (`Servicos/Diagnostico/RegistroErrosLoggerProvider`) que captura **avisos, erros e críticos**:
- do código do sistema (categorias `Financas.*`);
- do tratador de exceções não tratadas da API (categoria `Erro`, em `Program.cs`);
- da queda de serviços em segundo plano (`Microsoft.Extensions.Hosting`).

O resto do framework (EF Core, Kestrel) fica de fora. Os registros passam por uma fila em memória com limite (`FilaErros`) e são gravados na tabela `ErroSistema` pelo `GravadorErrosHostedService`. Antes de gravar, o `Sanitizador` troca por `[removido]`:
- query strings de URLs;
- pares `token=`/`senha=`/`client_secret:`…;
- tokens Bearer e JWT;
- CPF;
- valores em R$.

A mesma ocorrência (assinatura = categoria + mensagem + tipo da exceção) ainda em aberto e vista nas últimas 24 h só soma em `Ocorrencias`. Os registros mais antigos que o prazo da Parametrização (`Diagnostico.DiasRetencaoErros`, padrão 90 dias) são apagados uma vez por dia.

O front-end manda seus erros não tratados por `POST /api/erros/front-end`, a partir de um `ErrorHandler` do Angular. Os limites são:
- só com login;
- sem erros de chamada HTTP;
- sem repetir;
- até 10 por carga da página;
- até 30 por hora na API.

A tela fica em **Configurações › Erros**, com filtros na URL. O Painel ganha um alerta quando há erros (não avisos) em aberto na última semana.

O porquê principal: o usuário passa a ver e resolver as falhas no próprio site, sem nada novo instalado nem dado saindo da máquina.

## Consequências

- **Positivas:**
  - falhas aparecem no site e no Painel;
  - repetições não poluem a lista;
  - o log do sistema continua recebendo tudo, porque os outros provedores não mudaram;
  - o backup do banco leva junto o histórico de erros.
- **Negativas:**
  - **A limpeza é por padrão de texto, não garantia.** Um segredo num formato que os padrões não reconhecem iria para a tabela. A regra continua sendo não colocar segredo em log. Novos formatos entram no `Sanitizador`.
  - Erros antes de o contêiner de serviços existir, ou com o banco inacessível, não chegam à tabela. Ficam só no log do sistema.
  - Com a fila cheia (1000), os registros novos são descartados. Isso também fica só no log do sistema.
  - O banco cresce com os erros. O limite vem da retenção e do agrupamento, e a tabela fica pequena no uso normal.
  - Em ambiente de desenvolvimento, a página de exceções do ASP.NET trata o erro antes do tratador da categoria `Erro`. Por isso, exceções não tratadas da API só são gravadas em produção.
  - Código próprio de log para manter (provedor, fila, gravador, limpeza).
- **Na prática:**
  - basta logar com `ILogger` como já se faz: `LogWarning`/`LogError` em classes `Financas.*` aparecem na tela;
  - a pasta do serviço define a origem (Pluggy → Open Finance, Investimentos, Cambio → Câmbio); pasta nova com origem própria entra em `RegistroErrosLoggerProvider.Origem`;
  - não logar valores, tokens nem dados pessoais nas mensagens;
  - o `GravadorErrosHostedService` não pode ser capturado por esse provedor, para que uma falha ao gravar não gere outro erro a gravar.
