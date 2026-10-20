# 0042. Contas fora do saldo e arquivadas continuam nos relatórios

- **Status:** Aceita
- **Data:** 2026-10-20 (issue #172)
- **Relacionadas:** [0031](0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md) (transferências fora dos relatórios)

## Contexto

Cada conta tem duas marcas: **"Incluir no saldo total"** (`Conta.IncluirNoSaldo`) e **"Arquivada"** (`Conta.Arquivada`).
- Elas tiram a conta do saldo total, do patrimônio e da reserva líquida.
- As transações dela continuam nas receitas e despesas: Painel, Visão anual, Relatórios, Orçamentos e Metas.

A verificação geral (#172) perguntou se isso é de propósito. Na base real, nenhuma conta estava fora do saldo ou arquivada, então nada mudaria hoje.

## Alternativas consideradas

- **Tirar dos relatórios também as transações das contas fora do saldo:**
  - serve para a conta que não é sua (de um filho, de uma empresa);
  - mas um cartão tirado do saldo para não somar a dívida perderia todas as compras dos gastos, e esse erro seria silencioso.
- **Tirar dos relatórios as contas arquivadas:**
  - mas conta arquivada é conta encerrada: os gastos dela aconteceram, e os meses passados mudariam ao arquivar.
- **Manter as transações nos relatórios e dizer isso na tela.** **Escolhida.**

## Decisão

- "Incluir no saldo" e "Arquivada" mexem só no saldo (total, patrimônio, reserva). As transações das duas continuam nas receitas e despesas.
- A edição da conta diz isso ao lado da marca, com o caminho para tirar as transações dos relatórios ("Ignorar nos relatórios", em lote).
- *Inferido*, não confirmado pelo usuário: a escolha segue o comportamento que já existia.

## Consequências

- **Positivas:**
  - nenhum gasto some sem aviso ao mexer numa marca de saldo;
  - os meses passados não mudam ao arquivar uma conta encerrada.
- **Negativas:**
  - **Conta de outra pessoa** acompanhada no site soma nos seus gastos até as transações serem marcadas "Ignorar nos relatórios".
  - **Transação nova dessa conta** precisa ser marcada de novo; não há marca por conta para isso.
- **Na prática:**
  - Consulta de receitas e despesas (`RelatorioService.Relevantes` e afins) não filtra por `IncluirNoSaldo` nem por `Arquivada`.
  - Se um dia surgir o caso da conta de terceiros, o caminho é uma marca nova ("fora dos relatórios") e uma ADR que substitua esta. Não reaproveitar "Incluir no saldo".
