# 0063. Gasto pago direto de um cofrinho: um lançamento do app na conta, marcado com a reserva

- **Status:** Proposta
- **Data:** 2026-11-11 (issue #691)
- **Relacionadas:** ajusta a [0010](0010-reservas-estimadas-pelo-extrato.md) e a [0037](0037-reservas-pelo-valor-do-open-finance.md) (cálculo das reservas), a [0043](0043-assinaturas-e-recorrentes-em-contas-a-pagar.md) (conciliação das contas a pagar) e conversa com a [0060](0060-cdbs-e-caixinhas-com-rendimento-e-a-regra-caixinha-x-reserva.md) (caixinha × reserva)

## Contexto

No Mercado Pago dá para pagar uma conta em débito automático com o dinheiro de um cofrinho. Esse pagamento sai direto da reserva, não passa pela conta e não chega pelo Open Finance: o extrato da conta fecha com o saldo informado, e nem o `bankData` nem o `/investments` trazem a saída (levantamento da #691, com os dados de 11/11/2026). A ADR 0010 já registrava a limitação: "dinheiro que sai da reserva sem passar pela conta não aparece no extrato: só a conferência corrige".

Resultado: a despesa não entra nos relatórios, nos orçamentos e no painel, a reserva estimada fica acima do real, e não havia como registrar o gasto, porque o app não deixa lançar à mão numa conta do Open Finance.

## Alternativas consideradas

- **Tabela própria da reserva (as saídas do cofrinho):** não mexe na `Transacao`, mas relatórios, orçamentos, painel, categorias e emergências leem a `Transacao`. Cada um teria de somar a tabela nova, ou a despesa continuaria de fora.
- **Lançamento comum na conta do Open Finance:** entra em tudo sozinho, mas não baixa a reserva, e a regra "não se lança à mão no Open Finance" existe para o extrato não ficar com o que o banco não mandou.
- **Lançamento do app na conta do Open Finance, marcado com a reserva de onde saiu.** **Escolhida.**

## Decisão

- **O gasto pago com um cofrinho é uma `Transacao` do app** (sem `PluggyTransactionId`) na conta do Open Finance dona da reserva, com `Transacao.ReservaNome` = o nome da reserva. Ela entra como despesa normal (categoria, tipo, relatórios, orçamentos, painel). Na estimativa da reserva (ADR 0010), conta como retirada: baixa a reserva pelo valor. **O saldo da conta não muda**, porque o de conta do Open Finance é o que a instituição informa (`contas.Saldos`).
- **A conta a pagar guarda de onde sai o dinheiro** (`ContaAgendada.ReservaNome`, nula = saldo da conta), pelo mesmo critério da meta (#89): conta + nome, sem diferenciar maiúsculas. Só com conta do Open Finance e fora do cartão de crédito. O nome pode ser de uma reserva que o app ainda não conhece: ela passa a aparecer em Reservas, conferível (*inferido*, sugestão da issue).
- **Baixa no vencimento:** com "débito automático" e um cofrinho, a conciliação (`cobrancas.Conciliar`, que já roda na partida, depois de cada sincronização e ao salvar) grava a despesa no dia do vencimento e avança o vencimento, como o "Pagar". Com "à mão", o "Pagar" lança a mesma despesa do cofrinho.
- **Cada baixa pelo cofrinho fica em `PagamentoCofrinho`** (conta a pagar, transação, vencimento pago, o próximo, o último pagamento de antes, automática ou não). Ela permite **"Desfazer"** (apaga o lançamento e volta o vencimento) e **"Ajustar valor"** dias depois, sem guardar nada na memória do processo. Desfeita, a linha fica sem o lançamento, e a baixa automática não repete aquele vencimento: a conta fica vencida, com "Pagar".
- **Sem contar duas vezes:** se a cobrança chegar mesmo assim pelo extrato (por exemplo, o cofrinho estava sem saldo e o banco tirou da conta), com o texto da cobrança, até 10 dias antes ou depois do vencimento e importada depois da baixa, a conciliação apaga o lançamento automático e vale a do banco.
- **Os lançamentos do app ficam fora do que procura cobranças do banco:** a conciliação, as sugestões e os avisos das recorrentes não os leem.

## Consequências

- **Positivas:**
  - o gasto do cofrinho aparece no mês e na categoria, e a reserva baixa, sem a pessoa lembrar de conferir;
  - genérico: vale para qualquer banco cuja conta tenha reservas no app;
  - nada muda para quem não usa: sem "Pago com", tudo segue como antes.
- **Negativas:**
  - a regra "não se lança à mão em conta do Open Finance" ganha uma exceção, limitada ao lançamento marcado com a reserva;
  - nas reservas **informadas pelo banco** pelo nome (ADR 0037), o valor do banco continua valendo: a despesa entra nos relatórios, mas não baixa de novo a reserva (o banco já baixou);
  - vencimento em fim de semana ou feriado: a baixa usa a data do vencimento, e o banco pode debitar no dia útil seguinte (*inferido*: quase sempre no mesmo mês);
  - sem o texto da cobrança, a duplicata não é achada: se a cobrança chegar pelo extrato, a pessoa desfaz a baixa;
  - a caixinha da ADR 0060, quando sair, também terá de baixar pelo `ReservaNome`.
- **Na prática:**
  - colunas novas `ContaAgendada.ReservaNome` e `Transacao.ReservaNome` (`adicionar_coluna_se_ausente`) e a tabela `PagamentoCofrinho`, apagada junto com a conta a pagar e com o lançamento vazio (`SET NULL`) quando ele é apagado;
  - a regra da baixa e da duplicata fica em `internal/movimentacao/cobrancas`; o cálculo das reservas, em `internal/patrimonio/reservas` (`ReservaNome` vira retirada);
  - código que procura cobranças do banco no extrato filtra `"ReservaNome" IS NULL`.
