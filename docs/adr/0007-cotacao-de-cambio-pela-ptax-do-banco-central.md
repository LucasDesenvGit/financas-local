# 0007. Cotação de câmbio pela PTAX do Banco Central, guardada para uso offline

- **Status:** Aceita
- **Data:** 2026-10-16
- **Relacionadas:** [0003](0003-esquema-por-scripts-sql-idempotentes.md), [0006](0006-cliente-http-proprio-para-a-pluggy.md)

## Contexto

O sistema passa a lidar com moedas estrangeiras, e o primeiro passo é ter a cotação em reais de cada moeda numa data. Restrições do projeto: roda local, sem custo, só saídas HTTPS com o dado mínimo enviado, e precisa continuar funcionando sem internet. Para finanças pessoais, a cotação de referência (e não a de mercado em tempo real) é a que importa.

## Alternativas consideradas

- **PTAX pela API Olinda do Banco Central** (`olinda.bcb.gov.br`) — oficial, gratuita, sem chave; uma consulta por moeda e período, com boletins de abertura, intermediários e fechamento. Contras: um domínio novo de saída; o fechamento do dia só sai por volta das 13h; sem cotação em fim de semana e feriado; cobre 10 moedas (AUD, CAD, CHF, DKK, EUR, GBP, JPY, NOK, SEK, USD).
- **Yahoo Finance** (`USDBRL=X`, domínio já liberado para cotação de ativos) — cotação durante o dia e muitas moedas. Contras: fonte não oficial, sem contrato, pode mudar ou bloquear sem aviso; valor de mercado, não referência.
- **Séries do SGS** (`api.bcb.gov.br`, domínio já liberado para a Selic) — oficial. Contras: uma série numerada por moeda e por lado (compra/venda), a manter à mão, e sem a lista de moedas disponíveis.
- **APIs de câmbio de terceiros** (com chave) — muitas moedas. Contras: cadastro, limite de uso ou custo, e mais um segredo para guardar.

## Decisão

Usamos a PTAX de fechamento do Banco Central, consultada pela API Olinda (`CambioService`, com `HttpClient` próprio conforme a [0006](0006-cliente-http-proprio-para-a-pluggy.md)). Cada fechamento é guardado na tabela `CotacaoMoeda` e serve de fonte para uso offline:

- Quando a data não teve boletim, vale o último dia útil anterior, olhando até 7 dias para trás; a resposta informa a data solicitada e a data da cotação usada.
- O `CambioHostedService` atualiza todas as moedas uma vez por dia e de novo depois do fechamento (13h30 no horário de Brasília). Ele busca os últimos 7 dias e faz *upsert*: insere o que é novo e corrige o que o Banco Central republicou.

## Consequências

- **Positivas:** fonte oficial e gratuita, sem segredo para guardar; a última cotação continua disponível sem internet (`GET /api/cambio/cotacoes`); só o código da moeda e datas saem do computador.
- **Negativas:**
  - antes das ~13h de um dia útil a cotação "de hoje" é a do dia útil anterior;
  - não há histórico antigo: a atualização automática cobre só os últimos 7 dias — se a máquina ficar desligada mais que isso, os dias intermediários só chegam se forem consultados um a um;
  - datas passadas sem boletim (fim de semana, feriado) não ficam marcadas no cache e geram uma consulta à API a cada pedido;
  - a PTAX é referência, não a taxa que o cartão ou o banco cobrou — para converter transações, a taxa efetiva do Open Finance, quando houver, deve ter precedência (a decidir, provável nova ADR);
  - moedas fora das 10 da PTAX ficam sem cotação.
- **Na prática:** `olinda.bcb.gov.br` está na lista de saídas para a internet do `README.md`; moedas são sempre códigos ISO 4217 validados por `CambioService.NormalizarMoeda`; `BRL` é a moeda base, com cotação fixa 1; quem precisar de câmbio usa o `CambioService`, não chama a API do Banco Central diretamente.
