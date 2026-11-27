# 0073. Cotação de criptomoedas pela API pública da Binance

- **Status:** Aceita
- **Data:** 2026-11-18
- **Relacionadas:** substitui a [0048](0048-cotacao-de-criptomoedas-pelo-mercado-bitcoin.md); issue #762

## Contexto

A 0048 escolheu a API pública v4 do Mercado Bitcoin para a cotação de criptomoedas em reais. Desde 14/11/2026, ela responde **403** a toda consulta do site, e a cotação parou: 94 avisos em Configurações › Erros até 18/11.

Testado em 18/11/2026, sem enviar dado nenhum além dos códigos das moedas:

| Fonte | Resultado |
|---|---|
| **Mercado Bitcoin**, a mesma chamada do site | 403 com a página "Attention Required! \| Cloudflare" (`Server: cloudflare`). A página de documentação da API também dá 403, daqui e de outra rede. Um projeto de terceiros que testa a mesma URL pelo GitHub Actions também registrou 403 em 11/2026. É o bloqueio de robôs do Cloudflare para quem não é navegador, e não um caminho ou um cabeçalho que mudou. |
| **Binance**, `GET /api/v3/ticker/24hr?symbols=[...]&type=MINI` | respondeu os 4 pares em reais numa chamada, sem chave, com os preços em texto e o `closeTime` em milissegundos |
| **CoinGecko**, `/simple/price?vs_currencies=brl` | respondeu em reais, sem chave |
| **Yahoo Finance**, que o site já usa para ações | só tem cripto em dólar (`BTC-USD`); `BTC-BRL` dá 404 |

Passar pelo bloqueio fingindo ser navegador não é uma saída: é contornar a defesa do serviço, e ela pode piorar a qualquer momento.

## Alternativas consideradas

- **Esperar o Mercado Bitcoin.** Sem domínio novo. Contra: nada indica que o bloqueio é passageiro, e a cotação fica parada até lá.
- **CoinGecko.** É uma média de várias corretoras. Contra: sem chave, o limite é por IP e não é publicado; o caminho documentado pede cadastro e uma chave, que teria de ficar criptografada como os outros segredos.
- **Yahoo Finance.** Sem domínio novo. Contra: só tem cripto em dólar; converter pela PTAX ou pelo `BRL=X` misturaria duas cotações de horários diferentes.
- **Binance**, API pública de dados de mercado. Corretora com os pares em reais (`BTCBRL`, `ETHBRL`, `SOLBRL`, `USDTBRL`), sem chave, com limite publicado (o ticker de até 20 pares pesa 2, de 6.000 por minuto por IP), uma chamada para todas as moedas, e o formato perto do que o site já lia.

## Decisão

Usamos a **API pública da Binance** (`https://api.binance.com/api/v3/ticker/24hr`, na forma `MINI`) para a cotação de criptomoedas em reais, no lugar do Mercado Bitcoin. A escolha foi do usuário, em 18/11/2026, entre Binance, CoinGecko e esperar o Mercado Bitcoin.

O resto da 0048 continua: a lista fixa de pares, a consulta a cada 30 minutos pela API do site (nunca na hora, pela tela), as cotações guardadas por 30 dias e a variação em 24 h calculada pelo site.

- **O que sai do computador:** só a lista fixa de pares (`["BTCBRL","ETHBRL","SOLBRL","USDTBRL"]`), por HTTPS. Nenhum valor, nome ou dado da pessoa.
- **Preço usado:** `lastPrice`, o último negócio; a data e a hora são o `closeTime` (o fim da janela de 24 h, que é o momento da consulta). *Inferido:* para a cotação de referência, a diferença entre os dois instantes não importa.
- **Status:** 5xx, 408 e 429 são passageiros (aviso, tenta no próximo ciclo); os outros 4xx (o 403 do WAF, o 418 do IP banido depois de passar do limite, o 451 de país bloqueado) são recusa: erro, registrado uma vez enquanto a recusa for a mesma (#762).

## Consequências

- **Positivas:**
  - a cotação de cripto volta a atualizar;
  - sem chave e sem custo, com o limite publicado e longe dele (48 consultas por dia, de peso 2);
  - a recusa da fonte aparece como erro, uma vez, e não como falta de internet.
- **Negativas:**
  - **domínio novo de saída** (`api.binance.com`), e o `api.mercadobitcoin.net` sai;
  - **preço de uma corretora só**, como antes, mas de outra: os números mudam alguns décimos em relação ao Mercado Bitcoin. A variação em 24 h das primeiras horas depois da troca compara cotações das duas fontes;
  - **a Binance também pode bloquear** ou mudar a API sem aviso. Nesse caso, a recusa vira erro na tela de erros, a cotação fica "desatualizada" na tela, e trocar de fonte é outra ADR;
  - a Binance não opera em alguns países (o 451); em quem usa o site do Brasil, responde (*inferido* para outros países).
- **Na prática:**
  - **moeda nova:** acrescentar em `cripto.Moedas`, conferindo antes que a Binance negocia `XXXBRL`, e o nome em `MOEDAS_CRIPTO`, no front-end, com o ícone;
  - **o domínio** está na tabela de saídas de `docs/SEGURANCA.md`, no "Como funciona" do `README.md`, na lista do `CLAUDE.md`, nos diagramas de `docs/ARQUITETURA.md` e no modelo de ameaças;
  - os testes do cliente usam a resposta gravada da Binance (`testdata/ticker-24hr.json`, de 18/11/2026).
