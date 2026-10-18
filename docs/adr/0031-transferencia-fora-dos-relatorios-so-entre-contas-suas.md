# 0031. Transferência só fica fora dos relatórios quando é entre contas suas

- **Status:** Aceita
- **Data:** 2026-10-18
- **Relacionadas:** [0025](0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md) (mesmo titular pelo Open Finance), [0010](0010-reservas-estimadas-pelo-extrato.md) (reservas pelo extrato)

## Contexto

Toda categoria do tipo transferência tira a transação dos relatórios (`CategorizacaoService.AplicarTipo`). As categorias de transferência do Open Finance ("Transfers", "Transfer - PIX", "Transfer - TED", "Same person transfer") vão todas para a categoria local "Transferências".

O efeito: um **Pix para outra pessoa**, ou recebido dela, sumia das despesas e receitas, como se fosse dinheiro passando entre contas suas.

Nos dados reais, desde janeiro, ficaram de fora **R$ 30.748 em despesas e R$ 17.156 em receitas**, incluindo o salário e pagamentos mensais a terceiros.

O usuário pediu: só as transferências entre contas dele devem ficar fora.

O dado de mesmo titular (`MesmaTitularidade`, ADR 0025) existe só nas transações relidas depois da #111. Em 243 de 258 transferências ele está vazio.

## Alternativas consideradas

- **Só o dado de mesmo titular.**
  - Prós: simples e exato quando existe.
  - Contras: vazio na maioria. Tratar o vazio como "de terceiros" contaria em dobro as transferências entre contas; tratar como "suas" não muda nada.
- **Mudar a categoria das transferências para terceiros** (ex.: para "Outros").
  - Contras: troca uma escolha da pessoa e perde a informação de que foi um Pix.
- **Decidir por lançamento, na categoria "Transferências", com quatro pistas de "entre contas suas":**
  1. lançada pelo site como transferência entre contas;
  2. o banco informa o mesmo titular;
  3. o par em outra conta sua (mesmo valor, sinal oposto, até 5 dias);
  4. dinheiro guardado numa reserva ("Dinheiro reservado/retirado", como na ADR 0010).

  O banco informar **outro** titular vale mais que o par. O resto conta como despesa ou receita, e a categoria continua "Transferências".

## Decisão

Adotada a terceira opção (`TransferenciasService`).

**Onde vale:** só na **categoria genérica "Transferências"**, reconhecida pelo nome, sem acento e sem diferença de maiúsculas. Investimentos, Pagamento de fatura e as categorias do tipo transferência que a pessoa criar continuam sempre fora: são movimentos dela por natureza.

**Quando roda:**
- no fim de cada sincronização;
- ao iniciar a API (acerta o que já está no banco);
- depois de mudar categoria, valor ou excluir (formulário, lote, desfazer, reaplicar regras, mapear categoria do Open Finance).

**Como muda o lançamento:**
- **Entre contas suas:** `Tipo = Transferência` e `IgnorarRelatorios`.
- **Para ou de outra pessoa:** `Tipo` pelo sinal (despesa ou receita) e conta nos relatórios.

## Consequências

- **Positivas:**
  - receitas e despesas passam a incluir salário e pagamentos a terceiros feitos por Pix ou TED;
  - a etiqueta "fora dos gastos" aparece só no que é movimento seu;
  - nada muda nas outras categorias de transferência.
- **Negativas:**
  - **Transferência para uma conta sua que não está conectada** não tem par e passa a contar como despesa. Saída: uma categoria do tipo transferência própria, como "Entre minhas contas" (explicado no Manual).
  - **O par é estimado:** duas transferências de terceiros com o mesmo valor, em contas diferentes e com até 5 dias de diferença, podem ser tomadas como par. O dado de titular, quando existe, prevalece.
  - **Identificação por nome:** a categoria genérica é achada pelo nome ("Transferências"). Se for renomeada, volta o comportamento antigo (tudo fora).
  - **"Ignorar nos relatórios" marcado à mão** numa transferência para terceiros é desfeito na próxima categorização, como já acontecia com as transferências.
  - **Os números dos relatórios mudam** depois da atualização (para mais perto do real).
- **Na prática:**
  - Código que muda categoria, valor ou exclui transações deve chamar `TransferenciasService.ReclassificarAsync` depois de salvar.
  - A janela do par fica em `TransferenciasService.DiasDoPar`.
