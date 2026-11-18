# 0010. Reservas estimadas pelo extrato, com conferência manual

- **Status:** Substituída por [0037](0037-reservas-pelo-valor-do-open-finance.md) nos bancos que informam o valor reservado. **A estimativa abaixo continua valendo** para os que não informam, e para as reservas fora da lista do banco.
- **Data:** 2026-10-16; encurtada em 2026-11-15 (#712)
- **Relacionadas:** [0006](0006-cliente-http-proprio-para-a-pluggy.md) (a sincronização com a Pluggy não muda)

## Contexto

As Reservas do Mercado Pago ("cofrinhos") não chegavam pelo Open Finance com saldo (issue #25): nem em `/investments` nem no saldo da conta. Elas só aparecem no extrato da conta, como "Dinheiro reservado {Nome}" (saída) ou "Dinheiro retirado {Nome}" (entrada). Por isso o dinheiro guardado ficava fora do patrimônio, do total investido e da reserva de emergência.

## Alternativas consideradas

- **Só estimar pelo extrato:** automático, mas fica abaixo do real quando a reserva já tinha dinheiro antes da janela importada, e não vê o rendimento.
- **Só cadastro manual:** desatualiza a cada depósito ou retirada.
- **Extrato com conferência manual opcional.** **Escolhida.**
- **Esperar a instituição ou a Pluggy:** não resolvia nada na época (a 0037 veio quando a Pluggy passou a informar).

## Decisão

- **As reservas são descobertas pelo extrato**, sem cadastro: lançamentos efetivados das contas do Open Finance que entram no patrimônio, com a descrição original começando por "Dinheiro reservado " ou "Dinheiro retirado ". O nome é o resto da descrição, sem diferenciar maiúsculas.
- **Saldo estimado** = Σ(−valor) desses lançamentos, nunca negativo.
- **Com conferência** (tabela `ReservaConferencia`): saldo = saldo conferido + Σ(−valor) dos lançamentos **depois** da data da conferência. Os do próprio dia já estão no valor conferido.
- **Avisos para conferir:** "pode haver saldo anterior" (sem conferência, e o primeiro lançamento até 7 dias depois do começo do extrato importado) e "conferência antiga" (mais de 30 dias).
- O cálculo é feito **na leitura**, não na sincronização. O total entra no patrimônio, no total investido, na reserva de emergência e na distribuição da carteira, como "Reservas".

## Consequências

- **Positivas:** cada depósito ou retirada sincronizado já atualiza o saldo, sem chamada externa nova.
- **Negativas:**
  - sem conferência, o saldo fica por baixo (dinheiro anterior à janela e rendimento não retirado);
  - depende do texto que o Mercado Pago usa no extrato: se mudar, as reservas somem até os prefixos serem ajustados;
  - dinheiro que sai da reserva sem passar pela conta (compra paga direto com o cofrinho) só a conferência corrige;
  - não há histórico do saldo da reserva, só o valor atual.
- **Na prática:**
  - os prefixos e os prazos dos avisos ficam em `internal/patrimonio/reservas/texto.go`, e o cálculo no mesmo pacote;
  - quem soma reservas usa o `Listar`/`Total` de `internal/patrimonio/reservas` ou o `Composicao` de `internal/patrimonio/carteira`;
  - uma meta pode acompanhar um cofrinho (`Meta.ReservaNome`, #89), achado pela mesma regra (conta e nome). Se o cofrinho for renomeado, a meta avisa "não encontrado" e o vínculo não é apagado sozinho.
