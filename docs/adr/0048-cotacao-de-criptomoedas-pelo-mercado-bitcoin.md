# 0048. Cotação de criptomoedas pela API pública do Mercado Bitcoin

- **Status:** Aceita
- **Data:** 2026-10-21
- **Relacionadas:** [0009](0009-ativos-no-exterior-na-carteira-manual.md) (cotações da carteira); a PTAX do câmbio (#247) é o modelo seguido; issue #279

## Contexto

O usuário quer acompanhar o preço de criptomoedas (Bitcoin, Ethereum e outras) em reais, perto do bloco de Câmbio em Investimentos (#279, fase 1: só a cotação, sem dado da pessoa).

- **O Banco Central não publica cripto na PTAX.** É preciso uma fonte nova, e com ela um **domínio novo de saída de rede**: pelo `CLAUDE.md`, isso exige esta ADR antes do código.
- **Cripto negocia 24 horas por dia, 7 dias por semana**, ao contrário da PTAX, que é de dia útil. Uma cotação por dia não serve.
- **Restrições do projeto:** HTTPS, mínimo de dado enviado, sem custo, sem chave sempre que der, e a tela não pode quebrar se a fonte cair. Ela mostra a última cotação guardada, com a data.

Testado em 21/10/2026, sem enviar dado nenhum além dos códigos das moedas:

| Fonte | Chave | Limite documentado | Resultado |
|---|---|---|---|
| **Mercado Bitcoin**, API v4 pública (`GET /tickers?symbols=BTC-BRL,…`) | não precisa | 1 requisição por segundo nesse endpoint; 500 por minuto no total ([especificação](https://api.mercadobitcoin.net/api/v4/docs)) | respondeu, com BTC, ETH, SOL, USDT e XRP em reais numa chamada |
| CoinGecko (`/simple/price`) | sem chave: limite por IP, sem número publicado; o plano Demo pede chave e dá 100 por minuto ([limites](https://docs.coingecko.com/docs/common-errors-rate-limit)) | | respondeu sem chave |
| brapi (`/api/v2/crypto`), que o site já usa para ações | exige token | o plano gratuito tem 15 mil por mês e não lista cripto; ela aparece nos planos pagos, a partir de R$ 99,99 por mês ([planos](https://brapi.dev/pricing)) | 401 sem token |

## Alternativas consideradas

- **brapi.** Não abriria domínio novo, porque o site já fala com ela. Contra: cripto parece estar só nos planos pagos (*inferido* da página de planos; não deu para testar o token, que fica criptografado no banco), e o plano gratuito se descreve como "cotações básicas e sandbox".
- **CoinGecko.** É uma média de várias corretoras, com variação em 24 h pronta. Contra: o modo sem chave tem limite por IP sem número publicado, e o caminho documentado pede cadastro e chave, que precisaria ficar criptografada como os outros segredos.
- **Mercado Bitcoin**, API pública. Corretora brasileira, preço direto em reais, sem chave, limite publicado, uma chamada para todas as moedas. Contra: é o preço de **uma** corretora, não uma média.
- **Binance** (`api.binance.com`): não testada. A cotação em reais tem menos pares, e a disponibilidade do domínio varia por país (*inferido*, não conferido).

## Decisão

Usamos a **API pública v4 do Mercado Bitcoin** (`https://api.mercadobitcoin.net/api/v4/tickers`) para a cotação de criptomoedas em reais. É a única das testadas que é gratuita, sem chave, com limite publicado e preço nativo em reais.

- **O que sai do computador:** só a lista fixa de pares (`BTC-BRL,ETH-BRL,SOL-BRL,USDT-BRL`), por HTTPS. Nenhum valor, nome ou dado da pessoa.
- **Quando:** a API consulta sozinha, a cada 30 minutos (`CriptoHostedService`), e guarda cada cotação em `CotacaoCripto` (moeda, data e hora da fonte, preço), por 30 dias. A tela só lê o que está guardado (`GET /api/cripto/cotacoes`), nunca consulta a internet na hora.
- **Variação em 24 h:** calculada pelo site, contra a cotação guardada perto de 24 h antes (entre 22 e 26 h). A especificação diz que `open` é "o primeiro preço" sem dizer de qual período, então não é usado. Sem cotação guardada nessa janela, por exemplo com o site desligado ou nas primeiras 24 h, a variação não aparece; o site não inventa número.
- **Preço usado:** `last`, o último negócio. É cotação de referência, não preço de compra.

## Consequências

- **Positivas:**
  - cotação de cripto sem custo nem cadastro;
  - a tela funciona sem internet, com a última cotação e a data dela;
  - uma chamada por ciclo para todas as moedas, bem abaixo do limite;
  - a tabela é pequena: 4 moedas × 48 por dia × 30 dias, cerca de 5.800 linhas.
- **Negativas:**
  - **preço de uma corretora só:** pode diferir alguns décimos de outras corretoras e de médias como a CoinGecko. A tela diz de onde vem;
  - **dependência nova:** a API pode mudar ou sair do ar sem aviso. Nesse caso, a tela mostra a última cotação guardada, com a data, e o erro vai para o registro de erros (ADR 0022);
  - **domínio novo de saída** (`api.mercadobitcoin.net`): mais um endereço com que a API do site conversa sozinha;
  - **com o site desligado, não há cotação nem variação** daquele período: o histórico tem buracos;
  - **cripto oscila muito em 30 minutos:** a data e a hora ficam sempre à vista, e o bloco avisa para conferir na corretora antes de negociar.
- **Na prática:**
  - **moeda nova:** acrescentar o par em `CriptoService.Moedas`, conferindo antes que o Mercado Bitcoin negocia `XXX-BRL`, e o nome em `MOEDAS_CRIPTO`, no front-end;
  - **o domínio** está na tabela de segurança do `README.md` e na lista de saídas do `CLAUDE.md`;
  - **trocar de fonte** é decisão nova: uma ADR que substitui esta.
