# 0014. Empréstimos e financiamentos pelo Open Finance, separados e fora das despesas

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois da decisão, no PR #19)
- **Relacionadas:** [0006](0006-cliente-http-proprio-para-a-pluggy.md) (cliente da Pluggy)

## Contexto

O patrimônio líquido do Painel ignorava dívidas de crédito (financiamento imobiliário, de veículo, consignado, crédito pessoal), então mostrava a pessoa mais rica do que era. A Pluggy expõe as operações de crédito do Open Finance em `GET /loans` (saldo devedor, parcelas, taxa, CET, histórico de pagamentos). As parcelas desses contratos **já aparecem** como despesas comuns no extrato da conta (issue #14).

## Alternativas consideradas

- **Cadastro manual dos contratos** — funciona com qualquer banco; mas é trabalho repetitivo e desatualiza a cada parcela paga.
- **Somar as parcelas pagas como uma categoria especial de despesa** — sem tabela nova; mas não dá saldo devedor nem prazo, e duplicaria a parcela que já está no extrato.
- **Importar os contratos do `GET /loans` em tabelas próprias, só como posição de dívida** — dado oficial e atualizado a cada sincronização; não mexe no fluxo de despesas.

## Decisão

Os contratos de crédito vêm do Open Finance (`GET /loans`) para as tabelas `Emprestimo` e `PagamentoEmprestimo`, recarregadas a cada sincronização sem duplicar. Eles entram como **dívida** (o patrimônio líquido desconta o saldo devedor; a Visão anual soma as parcelas no comprometimento da renda), **não como despesa**: as parcelas continuam só nas transações, para não contar duas vezes.

Financiamento e empréstimo ficam **sempre separados** (cartões, cores e blocos próprios), pela modalidade do Open Finance (`ClassificacaoCredito.Natureza`): `FINANCIAMENTO_*` e `AQUISICAO_BENS_*` (ou `kind = FINANCING`) são financiamento; o resto é empréstimo. Motivo (*inferido*): financiamento costuma ter bem como garantia e juros menores — misturar os dois esconderia a dívida mais cara.

## Consequências

- **Positivas:** patrimônio líquido realista; nova página "Empréstimos e financiamentos" com saldo, parcelas, taxa, CET e atrasos; alerta de parcela em atraso no Painel; nenhuma despesa contada duas vezes.
- **Negativas:**
  - só aparecem contratos de instituições que compartilham "operações de crédito" no Open Finance; dívida fora disso (com pessoa física, de banco sem o consentimento) não entra — não há cadastro manual;
  - a sincronização dos contratos é complementar: se a instituição recusa (HTTP 400/403/404), as contas seguem sincronizando e a dívida simplesmente fica de fora, sem erro visível além do log;
  - a classificação depende da modalidade informada pela instituição; modalidade errada ou ausente cai em "empréstimo".
- **Na prática:** dívida de crédito é lida de `Emprestimo`; não criar despesa a partir de `PagamentoEmprestimo`. As cores de financiamento (`#8fa8f0`) e empréstimo (`#d9b45a`) são fixas: não mudam com a paleta escolhida em Aparência.
