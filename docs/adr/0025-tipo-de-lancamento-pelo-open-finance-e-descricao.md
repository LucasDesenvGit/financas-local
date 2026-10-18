# 0025. Tipo de lançamento: dado do Open Finance, com a descrição como reserva

- **Status:** Aceita
- **Data:** 2026-10-17 (issue #111)
- **Relacionadas:** [0006](0006-cliente-http-proprio-para-a-pluggy.md) (cliente da Pluggy), [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (filtro na URL), [0022](0022-erros-do-sistema-gravados-no-banco.md) (filtro calculado em memória, mesmo padrão do #9), [0024](0024-banco-de-cada-conta-com-selo-local.md) (selo do banco)

## Contexto

Na tabela de Transações não dava para saber de relance o que era cada lançamento:
- da conta ou do cartão;
- Pix enviado ou recebido, TED, boleto, compra no débito, saque, tarifa, rendimento;
- transferência entre contas próprias, ou Pix para outra pessoa;
- pagamento da fatura do cartão, que não é gasto novo, porque as compras já entraram no cartão.

A sincronização guardava só os dados de cartão da Pluggy (parcela, fatura). A Pluggy manda mais:
- `operationType`, classificado pela instituição no Open Finance:
  - na conta: `PIX`, `TED`, `DOC`, `BOLETO`, `CARTAO`, `SAQUE`, tarifas, rendimento, folha de pagamento, operação de crédito…;
  - no cartão: `PAGAMENTO`, `PAGAMENTO_FATURA`, `ESTORNO`, `TARIFA`…;
- `paymentData`, com o meio (`PIX`, `TED`, `DOC`, `BOLETO`) e nome e documento de quem paga e de quem recebe.

Esses campos estão na [documentação da Pluggy](https://docs.pluggy.ai/docs/transactions).

## Alternativas consideradas

- **Só pela descrição.**
  - Prós: nada muda no banco nem na sincronização.
  - Contras: depende de cada banco escrever "PIX"; Pix para você e Pix para outra pessoa ficam iguais.
- **Guardar o `paymentData` inteiro (nomes e documentos).**
  - Prós: dá para mostrar para quem foi.
  - Contras: dado pessoal de terceiros (CPF, nome) gravado sem necessidade.
- **Gravar o tipo já calculado.**
  - Prós: filtro direto em SQL.
  - Contras:
    - a regra muda e as transações antigas ficam com o tipo velho;
    - lançamentos manuais e edições teriam que recalcular.
- **Gravar o dado bruto mínimo e calcular o tipo na hora.**
  - Prós: a regra pode melhorar sem migrar dados, e a descrição cobre o que o banco não informa.
  - Contras: o filtro calcula em memória.

## Decisão

A sincronização grava dois campos novos em `Transacao`:
- **`MeioPagamento`:** o `operationType` ou, sem ele, o `paymentData.paymentMethod`.
- **`MesmaTitularidade`:** sim ou não, comparando os documentos de pagador e recebedor.
  - O resultado é null quando algum documento vem mascarado ou faltando.
  - **Nomes e documentos não são gravados.**

O **tipo de lançamento** (`TipoLancamento`) é calculado na hora pelo `TipoLancamentoService.Classificar`, nesta ordem:
1. transferência lançada pelo próprio sistema → "entre suas contas";
2. no cartão, o `operationType` (compra, estorno, pagamento de fatura, tarifa, crédito); sem ele, pelo sinal e pela descrição;
3. na conta, um pagamento de fatura reconhecido pela descrição; depois, o `operationType` ou meio informado (Pix com mesma titularidade → "entre suas contas");
4. sem dado do banco, pela descrição, por palavras inteiras (PIX, TED, BOLETO, SAQUE, TARIFA, RENDIMENTO…);
5. nada indica → **"outro"** (não inventa).

A API devolve o tipo em `TransacaoDto.TipoLancamento` (a projeção chama o método no cliente). O filtro `lancamento` (grupos como `pix`, `cartao`, `fatura`, `entre-contas`…) calcula em memória sobre o que os outros filtros já restringiram, como o filtro de descrição pouco informativa (#9).

A tela mostra:
- **em cada linha:** o tipo com ícone e texto, e o selo do banco da conta com "cartão de crédito" quando é cartão;
- **"fora dos gastos"** quando não entra nos totais;
- **"somando nos gastos"** ou **"somando nas receitas"** quando um pagamento de fatura ou uma transferência entre contas próprias está entrando nos totais, ou seja, contado duas vezes.

## Consequências

- **Positivas:**
  - o que é cada lançamento fica visível sem abrir a transação;
  - Pix para você e Pix para outra pessoa se separam;
  - o pagamento de fatura contado em dobro fica à mostra;
  - nenhum dado pessoal de terceiros gravado.
- **Negativas:**
  - Transações antigas só ganham o dado do banco quando são relidas: a sincronização relê os últimos dias (Parametrização), e as mais antigas ficam pela descrição até lá.
  - A qualidade depende do banco: nem todo banco manda `operationType` ou documento completo. A descrição é a reserva, e pode errar.
  - O aviso "somando nos gastos" só aponta o problema. Para corrigir, é preciso trocar a categoria para uma de transferência ("Pagamento de fatura", "Transferências"), e o modo edição (ADR 0023) faz isso em lote. Mudar o tipo da transação sozinho mudaria relatórios sem a pessoa saber.
  - O filtro por lançamento calcula em memória, o que pesa com períodos muito longos.
- **Na prática:**
  - **Novo valor de `operationType`:** entra no `switch` do `TipoLancamentoService`.
  - **Novo tipo:**
    1. enum `TipoLancamento` (API e `core/modelos.ts`);
    2. rótulo e ícone em `compartilhado/tipos-lancamento.ts`;
    3. grupo do filtro nos dois lados.
  - A regra é testada de ponta a ponta na API de teste; o back-end não tem testes automatizados.
