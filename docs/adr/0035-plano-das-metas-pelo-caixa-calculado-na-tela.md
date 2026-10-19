# 0035. Plano das metas pelo caixa do mês, calculado na tela

- **Status:** Aceita
- **Data:** 2026-10-19 (issue #149)
- **Relacionadas:** [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (estado na URL); [0013](0013-testes-do-front-end-com-jest.md) (testes no front-end); [0017](0017-preferencias-de-exibicao-no-navegador.md) (preferências); [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) (telas densas)

## Contexto

O assistente de metas (#90) dizia quanto guardar este mês pela sobra da Projeção: receitas médias − despesas à vista − parcelas, com a **compra no cartão contando no dia da compra**.

**O problema (#149):** a fatura que vence no mês não entrava como saída. Com o salário já recebido e poucas compras no mês, a sobra parecia muito maior do que o dinheiro que de fato ia sobrar. Além disso, quando os meses seguintes tinham pouca folga, a divisão "pela capacidade de cada mês" jogava quase tudo no mês atual. O resultado eram pedidos de ~8× a divisão igual, num mês de fatura alta.

O usuário escolheu a abordagem A + B + C + D da issue:
- **A.** Sobra pelo dinheiro que entra e sai, com limites.
- **B.** Plano em degraus.
- **C.** Percentual da sobra na tela.
- **D.** "Quanto consigo guardar" recalculando na hora.

O back-end não tem testes automatizados; o front-end tem (Jest, ADR 0013).

## Alternativas consideradas

- **Tudo no back-end, com o percentual e o ajuste como parâmetros da API:**
  - igual ao #90;
  - cada mudança na tela vira uma chamada, e o cenário da issue (fatura alta agora, caindo em janeiro) ficaria sem teste automatizado.
- **Tudo no front-end:**
  - o caixa precisa de transações, faturas importadas, ciclos dos cartões e saldos;
  - mandar isso tudo para a tela é muito dado e duplica regras de fatura que já estão no `ContaService` (#144).
- **Dividir:** o back-end calcula o caixa de cada mês; a tela divide entre as metas. **Escolhida.**

## Decisão

**Caixa de cada mês na API** (`GET /api/metas/plano`, `AssistenteMetasService`):
- **receitas e despesas das contas de dinheiro** (corrente, poupança, dinheiro e outras; sem transferências, como nos relatórios);
- **menos as faturas de cartão que vencem no mês:**
  - já fechada: o total do banco;
  - aberta: o que já foi lançado + a média diária de compras à vista do cartão nos dias que faltam;
  - futura: a média de compras à vista;
  - nas três, mais as parcelas já contratadas que caem nela;
- **mês atual:** o realizado mais a média do que falta;
- **meses seguintes:** médias dos últimos meses completos (até 6, a mesma janela da Projeção).

A API também devolve:
- o **saldo previsto nas contas no fim do mês**, já pagas as faturas que vencem nele;
- a **maior sobra de verdade** entre os meses completos considerados.

**Divisão entre as metas na tela** (`paginas/metas-plano.ts`, função pura):
1. capacidade = sobra × percentual (C), igual a 100 − margem de segurança: o mesmo parâmetro de antes, agora ajustável na própria tela;
2. no mês atual, a capacidade não passa do saldo previsto no fim do mês nem da melhor sobra recente. Havendo sobra e saldo, recomenda pelo menos R$ 50;
3. cada meta com prazo pede pela folga de cada mês, como no #90, **até 2× a divisão igual** por mês;
4. a prioridade decide no aperto, como no #90;
5. o valor que o usuário diz que consegue guardar (D) vale no lugar da recomendação. Ele fica **na URL** (`?guardar=meta:valor`, ADR 0018) e é só simulação até registrar o aporte;
6. a partir do mês seguinte, meses parecidos (até R$ 20 ou 15% de diferença) viram **degraus** com o mesmo valor, com a fatura média como motivo (B). O total não muda.

## Consequências

- **Positivas:**
  - o pedido do mês cabe no dinheiro real: a fatura que vence entra, e o saldo e o histórico limitam;
  - fatura caindo (parcelas acabando) aparece no plano: pouco agora, mais depois;
  - percentual e "consigo guardar" recalculam na hora, sem chamada à API;
  - o cenário da issue tem teste de regressão (`testes/regressao/assistente-metas.spec.ts`).
- **Negativas:**
  - **Regra em dois lugares:** o caixa fica no C#, a divisão no TypeScript. Quem muda o assistente mexe nos dois e nesta ADR.
  - **Duas "sobras":** a sobra das Metas (pelo caixa) é diferente da Projeção (compra no dia da compra). Os termos do glossário são diferentes ("Sobra do mês" × "Sobra prevista") e a definição avisa.
  - **Constantes de julgamento próprio, não validadas com uso real:**
    - o mínimo de R$ 50;
    - o teto de 2×;
    - a tolerância dos degraus;
    - limitar só o mês atual pelo saldo e pelo histórico (os meses seguintes podem pedir mais do que o usuário já guardou, porque a fatura cai).
  - **Simplificações da estimativa:**
    - a média de compras à vista do cartão é por dia e pelo ciclo, sem sazonalidade;
    - fatura sem pagamento informado e já vencida conta como paga para o saldo previsto;
    - o "melhor mês" usa as faturas pelo vencimento, que, sem fatura importada, são estimadas pelos lançamentos do ciclo.
  - **O plano depende da fatura cair.** Se entrar parcelamento novo, ele atrasa. A tela avisa quando o degrau maior depende disso, e o plano é refeito a cada visita.
- **Na prática:**
  - **Mudar a conta do caixa:** `AssistenteMetasService` (+ `MesCaixaDto`/`PlanoMetasDto`).
  - **Mudar a divisão, os limites ou os degraus:** `metas-plano.ts`, com teste em `testes/regressao/assistente-metas.spec.ts`.
  - **O percentual da tela grava o parâmetro `Metas.MargemSeguranca`** (0 a 50%), pelo `PUT /api/parametros`.
  - **Textos do glossário:** `sobraDoCaixa`, `parteDaSobra`, `recomendacaoMetas` (definição em até 2 frases e 240 caracteres).
