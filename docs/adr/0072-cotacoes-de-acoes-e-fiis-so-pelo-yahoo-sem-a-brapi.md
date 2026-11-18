# 0072. Cotações de ações e FIIs só pelo Yahoo Finance, sem a brapi

- **Status:** Aceita
- **Data:** 2026-11-18
- **Relacionadas:** [0005](0005-segredos-criptografados-com-data-protection.md), [0048](0048-cotacao-de-criptomoedas-pelo-mercado-bitcoin.md), [0056](0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md)

## Contexto

As cotações de ações e FIIs (`internal/patrimonio/cotacoes`) tinham duas fontes:
- o **Yahoo Finance** (`query1.finance.yahoo.com`, sem chave), a principal: preço, histórico mensal, proventos, o IFIX e o fechamento diário do dia da compra;
- a **brapi.dev**, de reserva: só o preço, só para ativo da B3, só quando o Yahoo não trazia preço e só se a pessoa tivesse informado um token.

A reserva custava bastante para o pouco que fazia:
- uma saída de rede a mais (`brapi.dev`), que recebia o token e os códigos dos ativos;
- um segredo guardado (`Cotacao.BrapiToken` na `Configuracao`);
- um campo na tela de Investimentos e o endpoint `GET/PUT /api/carteira/configuracao`;
- casos de teste e várias páginas de documentação.

Na troca para o Go (#85, ADR 0049), o token gravado pelo .NET deixou de abrir. Quem tinha token ficou sem a reserva e com o aviso "O token da brapi precisa ser informado de novo." em Configurações › Erros (18/11/2026, versão 3.27.0). Na base de quem pediu a mudança, todos os ativos estavam com a cotação do Yahoo (*inferido* de uma base só).

## Alternativas consideradas

- **Manter as duas fontes.** A reserva cobre o Yahoo fora do ar. Contra: só vale para quem cria uma conta na brapi e informa o token, e não traz histórico nem proventos. Também mantém o segredo, a saída de rede e a tela.
- **Trocar a principal pela brapi.** Ela é feita para a B3. Contra: exige token, e o plano gratuito tem limite de pedidos e não traz histórico e proventos no formato de que a carteira precisa. Também não atende os ativos no exterior (ADR 0009), que o Yahoo atende.
- **Só o Yahoo (escolhida).**

## Decisão

As cotações, os proventos e os históricos de ações, FIIs, ativos no exterior e do IFIX vêm **só do Yahoo Finance**. A brapi saiu do código, da tela, do banco e da documentação (#763).

## Consequências

- **Positivas:**
  - uma saída de rede e um segredo a menos;
  - a tela de Investimentos não pede mais nada;
  - o aviso do token antigo some.
- **Negativas:**
  - o Yahoo é uma fonte não oficial e sem contrato. Se ele mudar o formato ou bloquear o pedido (como o Mercado Bitcoin fez na #762), as cotações de ações e FIIs param até a correção. Nesse meio-tempo, a tela mostra a última cotação guardada;
  - quem usava a brapi perde a reserva. Na prática, o token antigo não abria desde a troca para o Go.
- **Na prática:**
  - o `cotacoes.Cliente` só fala com o Yahoo, e o `NovoCliente` recebe uma URL só;
  - o `cotacoes.NovoServico` e o `carteira.NovoServico` não recebem mais o cofre;
  - uma rotina de partida nova, a sétima (`carteira.ApagarTokenDaBrapi`, em `internal/partida`), apaga a linha `Cotacao.BrapiToken` da `Configuracao` a cada subida. Ela não pode ir no `02-dados-iniciais.sql`, que só roda em banco novo;
  - uma segunda fonte, se voltar a ser preciso, segue a ADR 0056 ("Fonte de cotação"): uma interface declarada pelo `cotacoes`, com a troca quando uma falha, e uma ADR nova para a saída de rede.
