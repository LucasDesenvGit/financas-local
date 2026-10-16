# 0008. Moeda estrangeira só em investimentos; demais registros em reais

- **Status:** Aceita
- **Data:** 2026-10-16
- **Relacionadas:** [0007](0007-cotacao-de-cambio-pela-ptax-do-banco-central.md)

## Contexto

Com a cotação de câmbio disponível ([0007](0007-cotacao-de-cambio-pela-ptax-do-banco-central.md)), falta decidir **em quais registros** a moeda passa a valer. A necessidade atual é só de **investimentos em outras moedas**; contas, transações, orçamentos e empréstimos continuam sendo em reais.

Estado do código no momento da decisão:

- A Pluggy envia `currencyCode` em contas, investimentos e empréstimos, e a sincronização já grava esse código na coluna `Moeda` (padrão `'BRL'`) das tabelas `Conta`, `Investimento` e `Emprestimo`. Transações não têm moeda.
- Os totais somam os valores sem olhar a moeda: o patrimônio no `RelatorioService` e os totais do `CarteiraService` somam `Investimento.Saldo` direto. Um investimento de US$ 1.000 entra como R$ 1.000.
- A carteira de renda variável lançada à mão (`Ativo`) só aceita códigos da B3, sempre em reais.

## Alternativas consideradas

- **Moeda em todos os registros** (conta, transação, investimento, empréstimo) — modelo completo. Contras: toda soma, orçamento, projeção e relatório passa a converter; transação precisa de valor original + valor em reais + taxa; muito trabalho para uma necessidade que hoje não existe.
- **Moeda só em investimentos** — resolve a necessidade atual e aproveita a coluna que já existe. Contras: uma conta em dólar (ex.: conta global) ou uma compra internacional no cartão continuam tratadas como reais.
- **Gravar o valor já convertido** (ex.: `SaldoEmReais`) junto do original — leitura simples. Contras: dado duplicado que envelhece a cada variação do câmbio e precisa ser recalculado; o valor original continuaria sendo a fonte da verdade de qualquer forma.

## Decisão

A única moeda respeitada pelo sistema é a de **`Investimento.Moeda`**:

- `Investimento.Saldo`, `ValorAplicado` e `Rendimento` ficam **na moeda original**, como vêm do Open Finance.
- A conversão para reais acontece **na leitura**, onde os valores são somados ou comparados, com a última cotação guardada pelo `CambioService`. Nenhum valor convertido é gravado.
- `Conta.Moeda` e `Emprestimo.Moeda` continuam sendo gravadas pela sincronização, mas **não são usadas**: contas, transações e empréstimos são tratados como reais.
- A carteira manual (`Ativo`) continua só em reais.

## Consequências

- **Positivas:** nada muda no banco; o escopo da conversão fica limitado aos poucos pontos que somam investimentos; o valor original nunca se perde, e trocar o critério de conversão não exige migrar dados.
- **Negativas:**
  - enquanto os pontos de soma não converterem, investimentos em moeda estrangeira **distorcem o patrimônio e os totais da carteira** — é a próxima mudança obrigatória desta decisão;
  - o valor em reais de um investimento muda com o câmbio mesmo sem o investimento mudar, e a cotação usada é a última guardada (pode ter até um dia útil de atraso);
  - uma conta ou empréstimo em moeda estrangeira, se aparecer, é somado como reais sem aviso;
  - se a moeda do investimento não tiver PTAX (fora das 10 moedas da [0007](0007-cotacao-de-cambio-pela-ptax-do-banco-central.md)) ou não houver cotação guardada, não há como converter — é preciso decidir entre deixar de fora do total e mostrar um aviso.
- **Na prática:**
  - toda soma ou comparação de `Investimento` com outros valores em reais passa pela conversão com o `CambioService`;
  - DTOs de investimento expõem a moeda original e o valor em reais.

## Quando revisitar

Abra uma ADR nova que substitua esta quando surgir:
- uma conta em moeda estrangeira;
- a necessidade do valor original de compras internacionais no cartão;
- um empréstimo em moeda estrangeira;
- ativos de bolsa estrangeira na carteira manual.

O ponto de partida já existe: `Conta.Moeda` e `Emprestimo.Moeda` estão gravadas, e as transações precisariam de colunas novas (valor original, moeda e taxa) pelo processo de [BANCO-DE-DADOS.md](../BANCO-DE-DADOS.md).
