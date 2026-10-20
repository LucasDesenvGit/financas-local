# 0038. Méli Dólar como reserva em dólar, estimada pelo extrato e convertida pela PTAX

- **Status:** Aceita
- **Data:** 2026-10-20 (issue #169)
- **Relacionadas:** [0037](0037-reservas-pelo-valor-do-open-finance.md) (reservas), [0008](0008-moeda-estrangeira-somente-em-investimentos.md) (moeda estrangeira só em investimentos)

## Contexto

O **Méli Dólar** é o dólar digital do Mercado Pago: o usuário compra e vende dólares dentro da conta. A verificação com os dados reais (19/10/2026) achou três problemas:

- **O saldo não entra no patrimônio.** O Open Finance não o informa: o MeuPluggy mostra só os CDBs e um fundo zerado da instituição.
- **A compra e a venda estavam em categorias diferentes:** compra como despesa ("Compras") ou como "Investimentos"; venda como receita ("Outras receitas"), às vezes contando nos relatórios. Comprar e vender dólar é mover o próprio dinheiro, não gasto nem ganho.
- **Só algumas descrições trazem a quantidade:** "Venda de Meli Dólar MUSD 32,09161263". As outras trazem só o valor em reais.

O site já tinha as duas peças:
- **Reservas estimadas pelo extrato, com conferência manual** (ADR 0010/0037), que entram no patrimônio, no total investido e na reserva de emergência;
- **A PTAX diária guardada no banco** (`CambioService`, ADR 0008).

## Alternativas consideradas

- **Ativo da carteira em USD** (quantidade em dólar, preço 1, câmbio por operação):
  - a carteira já calcula a variação cambial;
  - mas o Méli Dólar contaria como renda variável (separada no Painel) e misturaria um saldo de conta com as ações e FIIs;
  - e as operações teriam de ser importadas uma a uma.
- **Tabela nova só para o Méli Dólar:**
  - repetiria a conferência e os avisos que as reservas já têm.
- **Reserva em dólar dentro do `ReservaService`.** **Escolhida.**
  - Aproveita a conferência, o aviso "para conferir", o total no patrimônio e o cofrinho das metas.
  - Sem mudança de estrutura no banco.

## Decisão

- **O Méli Dólar é uma reserva em dólar de cada conta** (`ReservaService.MeliDolarAsync`, nome "Méli Dólar", `Moeda = "USD"`).
  - **Quantidade em dólar:** cada compra soma, cada venda subtrai. A quantidade vem do "MUSD x" da descrição; sem ela, é o valor em reais dividido pela PTAX de compra do dia (`ConversorMoeda`).
  - **Saldo em reais:** a quantidade pela última PTAX guardada. Sem nenhuma cotação do dólar, vale o que entrou menos o que saiu.
  - **O histórico do dólar** é baixado do Banco Central quando falta (`GarantirHistoricoAsync`, a mesma saída de rede de sempre).
- **A conferência guarda a quantidade em dólar** (`ReservaConferencia.SaldoConferido`, com 8 casas). Ela pode ser informada em reais: aí é convertida pela PTAX da data.
- **A compra e a venda vão para "Investimentos"** (transferência, fora dos relatórios):
  - pela regra padrão "MELI DOLAR" em `02-dados-iniciais.sql`;
  - para quem instalou antes, `CategorizacaoService.CriarRegraMeliDolarAsync` cria a mesma regra **uma vez** ao iniciar a API (marca `Regras.MeliDolarCriada` em `Configuracao`). Ela também passa para "Investimentos" os lançamentos já importados cuja categoria o usuário não escolheu.
  - Uma regra apagada pelo usuário não volta.

## Consequências

- **Positivas:**
  - O Méli Dólar aparece em Investimentos › Reservas e entra no patrimônio.
  - A variação do dólar aparece no saldo: a quantidade é fixa e a cotação muda.
  - A compra e a venda param de inflar despesas e receitas.
- **Negativas:**
  - **A quantidade é estimada:**
    - a PTAX não é a cotação do Mercado Pago, que tem spread, então cada compra sem "MUSD" na descrição fica com alguns centavos de diferença;
    - dinheiro de antes da janela importada não aparece.
    - A conferência corrige as duas coisas.
  - **Formato do texto *inferido*** a partir do Mercado Pago ("Compra/Venda de Meli Dólar", "MUSD 32,09161263"). Outro formato não é reconhecido.
  - **Em qual moeda o app mostra o saldo** (US$ ou R$) não foi confirmado (*inferido*). Por isso a conferência aceita as duas, e começa em reais: o valor que o usuário passou na verificação (R$ 52,71) só é coerente em reais.
  - **"Méli Dólar" é um nome reservado** nas conferências de reserva da conta: uma reserva do extrato com esse nome seria tratada como o Méli Dólar.
- **Na prática:**
  - Outra instituição com dólar na conta: estender `EhMeliDolar` e `QuantidadeNaDescricao`, ou generalizar o nome e a moeda.
  - Regra padrão nova para quem já instalou: seguir o mesmo esquema da marca única em `Configuracao`.
