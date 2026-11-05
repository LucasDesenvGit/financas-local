# 0036. Receita e despesa pelo tipo, não pelo sinal; estorno no cartão abate o gasto

- **Status:** Aceita
- **Data:** 2026-10-19 (issue #166)
- **Relacionadas:** [0031](0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md) (transferências); [0025](0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md) (tipo de lançamento)

## Contexto

O tipo de cada transação (receita ou despesa) era decidido **pelo sinal**, e os relatórios também somavam receitas e despesas pelo sinal. Com isso, todo crédito no cartão que não é pagamento de fatura virava **receita**:
- estorno;
- "crédito de confiança";
- devolução;
- cashback.

**Efeitos:**
- a receita do mês e a renda (50/30/20, comprometimento da renda, sobra das Metas) ficavam maiores;
- o gasto por categoria e os orçamentos não descontavam a compra devolvida;
- o resultado do mês (receitas − despesas) ficava certo, porque o erro aparecia dos dois lados.

Na base real, os estornos contados como receita chegaram a ~10% da receita de um mês.

**Variante:** estornos via NuPay chegam na categoria genérica "Transferências" e, pela ADR 0031, viravam receita "de outra pessoa".

## Alternativas consideradas

- **Tipo novo "Estorno":** fica explícito. Mas exige mexer em todos os filtros, telas e DTOs que conhecem três tipos, e o estorno continua sendo, para os totais, uma despesa negativa.
- **Marcar o estorno como "ignorar nos relatórios":** some da receita, mas também não abate o gasto da categoria.
- **Estorno = `Tipo = Despesa` com valor positivo, e os relatórios somando pelo tipo.** **Escolhida.** Para as transações comuns o tipo já sai do sinal, então somar pelo tipo dá o mesmo resultado. Só o estorno muda.

## Decisão

- **Tipo de um lançamento que não é transferência** (`CategorizacaoService.TipoPeloSinal`):
  - saída é despesa;
  - entrada é receita, **menos no cartão de crédito**, onde é estorno: despesa positiva.
- **Entrada no cartão:**
  - **pagamento de fatura** vira transferência fora dos relatórios, mesmo sem a categoria "Pagamento de fatura". É reconhecido pela categoria ou pelo texto e a operação, no texto do banco ou no nome mostrado (`TipoLancamentoService`);
  - **estorno:** tudo o que não é pagamento. Se chegou na categoria de transferências, só vira estorno quando o texto diz estorno, cashback ou devolução, e ganha a categoria do texto sem "Estorno de" (pelas regras) ou fica sem categoria. Os outros créditos nessa categoria seguem a ADR 0031: Pix seu pagando a fatura fica fora, e crédito de outra pessoa no cartão vira despesa.
- **Relatórios somam pelo tipo:**
  - receitas = soma das `Receita`;
  - despesas = −soma das `Despesa`, já líquida dos estornos.

  Vale para os totais do mês, o fluxo mensal, a Projeção, a Visão anual, a Saúde financeira, a renda, a lista de Transações e o caixa das Metas.
- **Gasto por categoria:** a soma é líquida. Categoria que no período só tem estorno (a compra foi antes) fica de fora do gráfico, porque não existe fatia negativa.
- **Transações que já existem:** acertadas ao iniciar a API (`ReclassificarEntradasNosCartoesAsync`), antes da conferência das transferências.

## Consequências

- **Positivas:**
  - receita e gasto por categoria batem com a realidade;
  - orçamento desconta a compra devolvida;
  - pagamento de fatura sem a categoria certa deixa de inflar a receita.
- **Negativas:**
  - **Despesa positiva:** quem lê `Transacao.Tipo` precisa saber que uma despesa pode ter valor positivo. Na tela, o estorno aparece em verde com o tipo de lançamento "Estorno".
  - **Somas que não fecham entre si:** num mês em que a categoria só tem estorno, a soma das categorias pode ficar um pouco acima do total de despesas.
  - **Classificação por texto:** reconhecer pagamento e estorno pelo texto pode errar com descrições incomuns. Categoria escolhida à mão do tipo transferência é respeitada.
- **Na prática:**
  - **Some receita e despesa pelo `Tipo`, nunca pelo sinal.** Para escrever o tipo de um lançamento novo, use `CategorizacaoService.TipoPeloSinal(valor, noCartao)` ou `AplicarTipo`.
  - **Filtros "só gastos" e "só receitas"** devem usar o tipo.

## Atualização de 2026-10-20: compra parcelada devolvida inteira (issue #257)

- **Problema visto na base real:** uma compra em 12 parcelas foi cancelada. O banco mandou **um estorno do valor total**
  (R$ 5.526,93) no mês em que só a 1ª parcela (R$ 460,66) tinha sido cobrada. Como o estorno abate o gasto do mês, as
  despesas de outubro ficaram **negativas** (−R$ 1.377) e o resultado do mês, R$ 12 mil "de sobra". A Projeção também
  continuava contando as parcelas que não vão vir.
- **Decisão:** estorno que devolve uma compra parcelada inteira (valor da parcela × número de parcelas, com tolerância
  de 10 centavos por parcela para o arredondamento do banco) **casa com a 1ª parcela dessa compra**: mesma conta,
  mesma descrição (as 8 primeiras letras, como em `ParcelasFuturas`), até 400 dias antes. O estorno, a 1ª parcela e as
  outras parcelas dela já lançadas (até 35 dias depois do estorno, para as que o banco antecipa) ficam **fora dos
  relatórios** (`IgnorarRelatorios`): a compra foi cancelada e, no total, não custou nada. Com várias compras iguais, cada
  estorno fica com a mais antiga ainda não casada; o efeito nos totais é o mesmo. Estorno de **parte** da compra não casa
  e continua abatendo o gasto do mês, como antes.
- **Onde:** `ParcelasFuturas.ComprasEstornadas` (o casamento) e `CategorizacaoService.IgnorarComprasParceladasEstornadasAsync`
  (grava). Roda ao iniciar a API e depois de cada sincronização; rodar de novo não muda nada.
- **Alternativas consideradas:** (a) contar no estorno só as parcelas já cobradas (o resto do estorno sumiria da conta):
  mexe em todas as somas por tipo e deixa o estorno com valor diferente do banco; (b) acertar só a Projeção: o Painel,
  os Orçamentos e o gasto por categoria continuariam com o mês negativo.
- **Consequências negativas:**
  - marcar "ignorar nos relatórios" à mão num desses lançamentos e desmarcar depois não vale: a rotina marca de novo;
  - parcela que o banco mandar mais de 35 dias depois do estorno (se mandar) não é casada e conta como gasto;
  - a regra é *inferida* dos casos da base real (Santander, Mercado Livre); outro banco pode mandar o cancelamento de
    outro jeito (ex.: um estorno por parcela, que já abate cada mês certo).
- **Projeção, junto (#257):** o saldo do cartão do Open Finance é a dívida toda, com as parcelas por vencer (glossário,
  "Dívida total nos cartões"), e os meses previstos tiravam essas parcelas de novo. As parcelas dos meses seguintes voltam
  ao "Disponível hoje" e saem uma vez só, no mês em que vencem. Cartão manual não muda (o saldo dele é o que já foi lançado).
  Isso parte do princípio de que o banco informa a dívida com as parcelas futuras, como o site já supõe no Painel;
  *não conferido banco a banco*.
