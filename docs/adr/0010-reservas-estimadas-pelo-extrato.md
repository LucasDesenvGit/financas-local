# 0010. Reservas estimadas pelo extrato, com conferência manual

- **Status:** Aceita
- **Data:** 2026-10-16
- **Relacionadas:** [0006](0006-cliente-http-proprio-para-a-pluggy.md) (a sincronização com a Pluggy não muda)

## Contexto

As Reservas do Mercado Pago ("cofrinhos", ex.: "Emergência") não chegam pelo Open Finance com saldo (issue #25). O que a Pluggy devolve para o item do Mercado Pago:

- **`GET /investments`:** só CDBs com vencimento e um fundo, todos com status `TOTAL_WITHDRAWAL` e saldo 0. Nenhum corresponde às Reservas.
- **`GET /accounts`:** o saldo da conta é só o disponível, sem o dinheiro reservado.
- **`/v2/transactions`:** cada movimento aparece no extrato da conta como "Dinheiro reservado {Nome}" (saída) ou "Dinheiro retirado {Nome}" (entrada). Esses lançamentos já são classificados como transferência (categoria Investimentos) e não afetam receitas e despesas.

Resultado: o dinheiro guardado nas Reservas ficava fora do patrimônio, do total investido e da reserva de emergência. A limitação é da instituição, não do mapeamento da sincronização (no Nubank, o mesmo código traz os CDBs com saldo).

## Alternativas consideradas

- **Só estimar pelo extrato:** soma os lançamentos de cada reserva. É automático, mas tem dois problemas:
  - fica abaixo do real quando a reserva já tinha dinheiro antes do primeiro lançamento importado (a primeira carga traz até 365 dias);
  - o rendimento fica dentro da reserva e só aparece quando é retirado.
- **Só cadastro manual:** o usuário informa o saldo de cada reserva. É simples, mas desatualiza a cada depósito ou retirada.
- **Extrato com conferência manual opcional:** o extrato acompanha os depósitos e retiradas, e uma conferência eventual corrige o ponto de partida e o rendimento. Custa uma tabela nova e uma regra de cálculo a mais.
- **Esperar a instituição ou a Pluggy:** não resolve nada hoje.

## Decisão

Usamos **extrato com conferência manual opcional**:

- As reservas são **descobertas pelo extrato**, sem cadastro. Vêm de lançamentos efetivados das contas Open Finance que entram no patrimônio, com descrição original começando por "Dinheiro reservado " ou "Dinheiro retirado ". O nome da reserva é o resto da descrição, sem diferenciar maiúsculas de minúsculas.
- **Saldo estimado** = Σ(−valor) desses lançamentos, nunca negativo. Valor negativo na conta é dinheiro indo para a reserva; positivo é dinheiro voltando.
- **Com conferência** (tabela `ReservaConferencia`: conta, nome, saldo conferido, data): saldo = saldo conferido + Σ(−valor) dos lançamentos com data **posterior** à da conferência. Os lançamentos do próprio dia são considerados já incluídos no valor conferido.
- **Avisos para conferir**, na aba Reservas e nos alertas do Painel:
  - **pode haver saldo anterior:** a reserva não tem conferência e o primeiro lançamento dela está até 7 dias depois do primeiro lançamento importado da conta, ou seja, no início da janela que a sincronização trouxe;
  - **conferência antiga:** a última conferência tem mais de 30 dias.
- O cálculo é feito **na leitura** (`ReservaService`), não na sincronização. Nada é gravado além da conferência.
- O total das reservas entra no **total investido e no patrimônio** (`ComposicaoInvestimentos.Reservas`), na **reserva de emergência** (saúde financeira) e na **distribuição** da carteira, como "Reservas". A página de Investimentos ganha a aba "Reservas", com a conferência.

## Consequências

- **Positivas:**
  - o patrimônio passa a refletir o dinheiro guardado nas Reservas, e cada depósito ou retirada sincronizado já atualiza o saldo;
  - nenhuma chamada externa nova e nenhum dado a mais enviado.
- **Negativas:**
  - sem conferência, o saldo é uma estimativa por baixo (dinheiro anterior à janela importada e rendimento não retirado ficam de fora);
  - depende do texto que o Mercado Pago usa no extrato: se a descrição mudar, as reservas somem da lista até os prefixos em `ReservaService` serem ajustados;
  - outra instituição com o mesmo texto no extrato também vira reserva (aceitável: o texto é específico);
  - dinheiro que sai da reserva sem passar pela conta (ex.: compras pagas direto com o "Cofrinho Mercado Livre") não aparece no extrato: só a conferência corrige, e o aviso de 30 dias existe para lembrar disso;
  - o aviso de saldo anterior é uma heurística: uma reserva criada de fato no início da janela também é sinalizada até ser conferida;
  - lançamento editado ou excluído à mão no extrato continua contando pela descrição original enquanto existir;
  - não há histórico do saldo da reserva, só o valor atual.
- **Na prática:**
  - prefixos e regra de cálculo ficam só em `Servicos/Investimentos/ReservaService.cs`;
  - quem soma investimentos para patrimônio ou reserva de emergência deve usar `CarteiraService.ComposicaoAsync` (que já inclui as reservas) ou `ReservaService.TotalAsync`;
  - se a Pluggy passar a informar o saldo das Reservas (em `/investments` ou `/accounts`), esta ADR deve ser substituída e o cálculo removido, para não contar duas vezes;
  - tabela nova `ReservaConferencia` no fim de `banco/01-estrutura.sql`, criada nos bancos existentes pelo `scripts/2-criar-banco.sh`.
