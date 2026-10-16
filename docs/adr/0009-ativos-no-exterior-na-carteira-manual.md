# 0009. Ativos no exterior na carteira manual

- **Status:** Aceita
- **Data:** 2026-10-16
- **Relacionadas:** substitui em parte a [0008](0008-moeda-estrangeira-somente-em-investimentos.md) (o item "a carteira manual continua só em reais"); usa a [0007](0007-cotacao-de-cambio-pela-ptax-do-banco-central.md)

## Contexto

A carteira de renda variável lançada à mão (`Ativo` + `OperacaoAtivo`) só aceitava códigos da B3, com preço em reais e cotação do Yahoo com sufixo `.SA`. Surgiu a necessidade de lançar ativos comprados no exterior (ex.: AAPL, VOO), com o preço em dólar e o câmbio usado na compra. Pela [0008](0008-moeda-estrangeira-somente-em-investimentos.md), esse era um dos gatilhos para revisitar a decisão.

Para o investidor brasileiro, o que importa é o resultado **em reais**: o custo de cada compra é o preço em dólar vezes o câmbio daquele dia, e o ganho inclui a variação do dólar.

## Alternativas consideradas

- **Converter na hora do lançamento** — o usuário digita preço em dólar e câmbio, e o sistema grava só o preço em reais. Pouca mudança. Contras: o valor do ativo fica "congelado" no câmbio da compra e não acompanha o dólar; perde-se o preço original; a cotação automática não funciona para o ativo.
- **Moeda no ativo e câmbio em cada operação** — o ativo guarda a moeda; cada operação guarda preço e taxas na moeda do ativo e o câmbio usado. Contras: os cálculos da carteira precisam converter em vários pontos.
- **Moeda e câmbio só no nível do ativo** (um câmbio médio) — mais simples que a anterior. Contras: compras em datas diferentes têm câmbios diferentes; um câmbio médio distorce o custo e o lucro de cada venda.

## Decisão

Usamos **moeda no ativo e câmbio em cada operação**:

- **`Ativo.Moeda`** (ISO 4217, padrão `BRL`) é definida no primeiro lançamento e não muda depois. Ativo em reais é sempre da B3. Ativo em outra moeda usa o código como no Yahoo Finance (ex.: `AAPL`, `VOO`, `SAP.DE`) e não pode ser FII nem BDR, que são da B3.
- **`OperacaoAtivo.Cambio`** guarda os reais por 1 unidade da moeda naquela operação (`1` para BRL). `PrecoUnitario` e `Taxas` ficam na moeda do ativo.
- **No formulário**, o câmbio vem preenchido com a PTAX da data da operação e pode ser editado para a taxa que a corretora cobrou. Na compra a PTAX sugerida é a de venda, porque a corretora vende a moeda a você; na venda, é a de compra.
- **Custo, preço médio em reais, lucro realizado e comparação com a Selic** usam o valor de cada operação convertido pelo seu próprio câmbio.
- **Valor atual** = quantidade × cotação na moeda do ativo × a última PTAX de compra guardada (`ConversorMoeda`). A valorização em reais inclui a variação do câmbio.
- **Proventos** são convertidos pela PTAX de compra da data-com (último dia útil anterior, até 7 dias), com o histórico guardado na `CotacaoMoeda`. Na falta dele, usa a última cotação guardada.
- **A cotação automática** usa o Yahoo com o código sem `.SA`. A cotação é recusada se a moeda informada pelo Yahoo for diferente da do ativo. A brapi só atende a B3.
- **O histórico de câmbio** necessário é baixado quando as cotações são atualizadas, desde a primeira operação do ativo.

## Consequências

- **Positivas:** o resultado em reais fica correto, com a variação do câmbio; cada venda apura o lucro com o custo real de cada compra; o valor original nunca se perde; ativos da B3 continuam exatamente como antes (moeda BRL, câmbio 1).
- **Negativas:**
  - **proventos do exterior aparecem brutos** — nos EUA há 30% de imposto retido na fonte, e o Yahoo informa o valor antes do imposto, então proventos e resultado ficam maiores que o recebido;
  - o código do ativo no exterior depende do Yahoo: um código digitado errado só aparece como "sem cotação" (sem validação de que o ativo existe no lançamento);
  - sem cotação da moeda guardada, o ativo é avaliado pelo custo em reais, e a moeda aparece nos avisos de "sem cotação";
  - a moeda de um ativo não pode ser trocada: lançou errado, é excluir e lançar de novo;
  - o painel de FIIs, a importação pelo extrato e a importação do Open Finance continuam só para a B3.
- **Na prática:**
  - toda conta com `OperacaoAtivo` usa o valor convertido pelo `Cambio` da operação (`CarteiraService.Calcular` e `ValorOperacao`) — nunca `Quantidade × PrecoUnitario` direto;
  - a cotação do ativo (`Ativo.PrecoAtual`) é sempre na moeda do ativo e passa pelo `ConversorMoeda` antes de ser somada;
  - colunas novas `Ativo.Moeda` e `OperacaoAtivo.Cambio`, adicionadas aos bancos existentes por `adicionar_coluna_se_ausente` no `scripts/2-criar-banco.sh`.
