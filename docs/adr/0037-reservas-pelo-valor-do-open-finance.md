# 0037. Reservas pelo valor que o Open Finance informa; a estimativa pelo extrato fica para o resto

- **Status:** Aceita
- **Data:** 2026-10-19 (issue #168); atualizada em 2026-10-20 (issue #237)
- **Relacionadas:** substitui a [0010](0010-reservas-estimadas-pelo-extrato.md) (reservas estimadas pelo extrato)

## Contexto

A ADR 0010 estimava o saldo das reservas ("cofrinhos" do Mercado Pago) pelos lançamentos do extrato, com conferência manual, porque a Pluggy não informava esse saldo. Ela mesma dizia para ser substituída quando a Pluggy passasse a informar.

A verificação com os dados reais mostrou o problema da estimativa: uma reserva que já tinha dinheiro antes da 1ª transação importada ficava com retiradas maiores que os depósitos, e a estimativa ia a zero. O patrimônio do Painel ficava ~25% abaixo do real. A conferência manual corrigiria, mas o campo da versão instalada gravou "10.123,42" como 10,12.

O MeuPluggy mostra, na conta do Mercado Pago, **"Reservado: R$ …"** com o valor exato do app. Na API da Pluggy isso vem em `Account.bankData`: `hasReservedBalance` e `reservedBalances` (um `object[]`, sem os campos documentados), e há também o `automaticallyInvestedBalance`.

## Alternativas consideradas

- **Continuar só com a estimativa e a conferência:** depende de o usuário lembrar de conferir. O rendimento da reserva não aparece no extrato, então o valor se afasta mesmo depois de conferido.
- **Usar só o valor da Pluggy e apagar a estimativa:** some a informação para os bancos que não mandam o `bankData`.
- **Valor informado pelo banco quando houver; estimativa e conferência quando não.** **Escolhida.**

## Decisão

- **A sincronização lê o `bankData` das contas bancárias** (`ReservaService.DoOpenFinance`):
  - as reservas com nome e valor de `reservedBalances` (procurando campos como `amount`/`balance`/`value` e `name`/`description`), gravadas em `Conta.ReservasInformadas` (JSON);
  - o total em `Conta.SaldoReservado`. Sem os detalhes, vale o `automaticallyInvestedBalance` positivo;
  - nada informado = `null`, e vale a estimativa;
  - o nome dos campos de cada item (sem valores) vai para o log, para conferir a estrutura real.
- **Nas contas com o reservado informado, ele substitui a estimativa só onde veio pelo nome** (`ReservaService.SubstituirPeloBanco`, ajustado na #237):
  - **com nomes:** uma reserva por nome da lista detalhada, marcada como "informado pelo banco". O nome do extrato é mantido quando bate, para não quebrar as metas ligadas a um cofrinho (#89);
  - **reserva do extrato fora da lista detalhada:** continua estimada (com a conferência, se houver), sem a etiqueta, com "conferir saldo" e o aviso de que não está na lista do banco (`ForaDaListaDoBanco`);
  - **só com o total:** cada reserva do extrato continua estimada e conferível, sem a etiqueta. O que o total passa da soma delas vira a linha "Reservado" ("restante do total do banco", `RestanteDoBanco`), sem conferência: conferir as outras reservas ajusta esse valor. Sem reserva no extrato, essa linha é o total inteiro. Se a soma das reservas passa do total, a linha não aparece.
- **Na tela:** só a reserva que veio pelo nome da lista do banco aparece como "informado pelo banco" e não tem "conferir saldo"; a linha do restante também não tem. A conferência das outras pede confirmação quando o valor foge muito do esperado: menos de 1/10 ou mais de 10× a estimativa ou o total guardado.
- **Nos totais:** patrimônio, total investido e reserva de emergência usam o mesmo `ReservaService` e não mudam de lugar.

## Consequências

- **Positivas:**
  - **Patrimônio certo sem conferência:** nos bancos que informam o reservado, ele bate com o app, inclusive com o rendimento.
  - **Os outros bancos continuam como antes:** estimativa e conferência.
- **Negativas:**
  - **Estrutura sem documentação:** a de `reservedBalances` não está documentada. A leitura tenta nomes de campo comuns e pode precisar de ajuste quando o log mostrar os campos reais.
  - **Só o total:** quando o banco não dá os nomes, não dá para dividir o total entre várias reservas. Até a #237, a regra era pôr o total na reserva de movimento mais recente e zerar as outras, marcando todas como "informado pelo banco" e escondendo o "conferir saldo": as outras ficavam com saldo errado e sem como corrigir.
  - **Restante do total (#237, *inferido*, não confirmado pelo usuário):** a linha "Reservado" mantém o patrimônio igual ao total do banco enquanto as reservas estimadas somam menos que ele. Quando somam mais (estimativa ou conferência acima do real), o patrimônio fica acima do que o banco informa até a pessoa conferir.
  - **Nome diferente entre o banco e o extrato (#237, *inferido*):** a reserva do extrato que não está na lista detalhada continua contando pela estimativa, e pode contar duas vezes o mesmo dinheiro se o banco a manda com outro nome. A tela avisa e sugere conferir essa com zero; preferimos isso a zerar em silêncio.
  - **Coluna nova em `Conta`:** `SaldoReservado` e `ReservasInformadas`, por `adicionar_coluna_se_ausente`.
- **Na prática:**
  - regras de leitura e de substituição só em `Servicos/Investimentos/ReservaService.cs`;
  - quem soma reservas usa `ReservaService.ListarAsync`/`TotalAsync` ou `CarteiraService.ComposicaoAsync`.
