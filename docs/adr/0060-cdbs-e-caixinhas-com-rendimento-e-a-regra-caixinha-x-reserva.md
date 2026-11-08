# 0060. CDBs e caixinhas numa aba própria, com o rendimento no valor, e a regra caixinha × reserva

- **Status:** Proposta
- **Data:** 2026-11-07 (issue #338)
- **Relacionadas:** substitui em parte a [0037](0037-reservas-pelo-valor-do-open-finance.md) (a reserva que é um investimento do Open Finance sai de Reservas); usa a [0045](0045-historico-de-saldo-dos-investimentos-do-open-finance.md) (histórico de saldo), a [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) (o que muda um número fica no banco) e a [0010](0010-reservas-estimadas-pelo-extrato.md) (reserva pelo extrato, com conferência)

## Contexto

Hoje o dinheiro guardado aparece em três lugares de Investimentos, com regras diferentes:

- **Aba "Open Finance":** tudo o que a Pluggy manda em `/investments`, misturado: CDBs ("CDB · Nubank", "CDB · Mercado Pago"), fundos, previdência. São 19 itens na carteira real. O banco informa o saldo e, quase sempre, o valor aplicado. O "Rendimento por mês" (ADR 0045) mostra quanto cada um rendeu, mas o rendimento não tem lugar próprio na tela.
- **Aba "Reservas":** os "cofrinhos" do extrato ("Dinheiro reservado {nome}", ADR 0010), com o valor que o banco informa quando informa (`bankData`, ADR 0037), e a conferência manual quando não. A reserva não tem rendimento.
- **A mesma aplicação pode estar nos dois:** a caixinha do Nubank chega como CDB em `/investments` e, conforme o banco, também como movimento de reserva no extrato. Contar as duas vezes infla o patrimônio.

A #338 pediu:
- uma aba "CDBs e caixinhas", com o rendimento de cada aplicação identificado e somado no valor, sem a pessoa lançar nada;
- a regra: **caixinha rende sozinha, reserva não rende**. A reserva muda só com os aportes e as retiradas da pessoa;
- a pessoa poder trocar uma reserva para caixinha e vice-versa, e desligar o rendimento automático de uma aplicação.

Restrições:
- **Nenhuma saída de rede nova:** a Selic já vem do Banco Central, e o resto vem da sincronização que já existe.
- **O passado do histórico não volta** (ADR 0045).
- **A estrutura do banco só acrescenta** (ADR 0003).

## Alternativas consideradas

**Quem decide o que é caixinha e o que é reserva:**
- **Só a pessoa, uma por uma:**
  - **Prós:** nunca erra o que ela quis.
  - **Contras:** dá trabalho a cada aplicação nova, e quem não classificar fica com a tela errada.
- **Só automática, pelo Open Finance, sem troca:**
  - **Prós:** zero trabalho.
  - **Contras:** quando erra (nome diferente entre o extrato e o banco, banco que não manda `/investments`), não tem como corrigir.
- **Automática, com a troca manual valendo mais.** **Escolhida** (decidido na #338).

**Onde guardar a escolha e o "rendimento automático":**
- **No navegador** (ADR 0017):
  - **Prós:** simples.
  - **Contras:** muda o valor e o patrimônio, que outro navegador veria diferente.
- **No banco.** **Escolhida:** o que muda um número fica no banco (ADR 0015).

**Rendimento da caixinha que só aparece no extrato** (sem o produto em `/investments`):
- **Não estimar:**
  - **Prós:** nunca mostra número inventado.
  - **Contras:** o valor fica abaixo do real a cada mês.
- **Estimar pelo % do CDI que a pessoa informa e pela Selic de cada dia** (`TaxaSelicDiaria`, que já é guardada). **Escolhida:** a linha fica marcada como "estimado", com "conferir", como as reservas.

## Decisão

**Três grupos, cada aplicação em um só:**

| Grupo | O que entra | Valor | Rendimento |
|---|---|---|---|
| **CDBs e caixinhas** (aba nova) | Investimentos do Open Finance de renda fixa (CDB, RDB, LCI, LCA e as caixinhas, que vêm como CDB/RDB), e as reservas que viraram caixinha | O saldo que o banco informa (que já tem o rendimento) ou, sem ele, a estimativa abaixo | Identificado e somado no valor |
| **Reservas** (aba que já existe) | As reservas do extrato e do `bankData` que não são caixinha | Como hoje (ADRs 0010 e 0037) | **Nenhum automático:** só os aportes e as retiradas, e a conferência |
| **Open Finance** (aba que já existe) | O resto do `/investments`: fundos, previdência, outros | Como hoje | Como hoje (ADR 0045) |

**Classificação automática**, refeita a cada leitura, sem gravar:
1. Investimento do Open Finance de renda fixa → "CDBs e caixinhas".
2. A reserva do extrato que corresponde a um investimento do Open Finance → é a caixinha, e sai de Reservas, **para não contar duas vezes**. Corresponder é ser da mesma conexão bancária e ter:
   - o mesmo nome, normalizado como as regras (`texto.ParaRegras`);
   - ou, sem nome que bata, o valor a até 1% do saldo do investimento.

   O 1% é *inferido* e precisa da carteira real para ajustar.
3. O resto das reservas fica em Reservas.

**A escolha da pessoa vale mais**, numa tabela nova, `ClassificacaoAplicacao`. Só tem linha a aplicação que a pessoa mudou; sem linha, vale a automática.
- **Chave:**
  - a reserva, como na ADR 0010: `ContaId` + nome, sem diferenciar maiúsculas;
  - ou o investimento: `PluggyInvestmentId`, que não muda entre as sincronizações.
- **Grupo escolhido:** caixinha ou reserva.
- **`RendimentoAutomatico`**, ligado por padrão.
- **`PercentualCdi`**, para a estimativa da caixinha que só está no extrato.
- **`ValorDePartida` + `DataDePartida`:** o saldo da reserva no dia em que ela virou caixinha.
- **`AlteradoEm`.**
- **"Voltar ao automático"** apaga a linha, com o "Desfazer" no aviso.

**Rendimento de cada CDB e caixinha:**
- **Com o valor aplicado do banco:** rendimento = saldo − aplicado; valor = saldo (ADR 0045).
- **Sem o valor aplicado:** rendimento = a variação do saldo entre os registros do histórico (ADR 0045), menos os aportes e os resgates do extrato no período. Fica marcado "aproximado", porque ligar um lançamento do extrato a uma aplicação é por nome e pode errar (*inferido*).
- **Caixinha que só está no extrato:**
  - valor = valor de partida + aportes − retiradas + o rendimento de cada dia, calculado pela Selic do dia × o % do CDI informado;
  - a Selic faz as vezes do CDI, que fica uns 0,1 ponto abaixo (*inferido*);
  - fica marcada "estimado", com "conferir"; a conferência corrige o valor de partida, como nas reservas.
- **Rendimento automático desligado:**
  - nada é calculado nem somado;
  - o valor é o que o banco informa ou, sem ele, partida + aportes − retiradas;
  - a linha mostra "rendimento manual", e o total da aba diz quantas estão assim.

**A troca** (pela tela, com confirmação que diz o que muda e "Desfazer"):
- **Reserva → caixinha:**
  - passa para "CDBs e caixinhas", com o rendimento automático ligado;
  - o valor de partida é o saldo da reserva no dia da troca.
- **Caixinha → reserva:**
  - volta para Reservas e deixa de somar rendimento;
  - o valor fica o último conhecido, e daí em diante muda só com os aportes e as retiradas;
  - o rendimento já somado não some: ele é dinheiro que existe e fica no valor de partida.
- **A sincronização não desfaz a escolha.**

**Metas ligadas a um cofrinho** (#89, `Meta.ReservaNome`): continuam achando a aplicação por conta + nome. Quando o cofrinho vira caixinha, o saldo da meta acompanha o valor da caixinha, com o rendimento. A tabela `Meta` não muda.

**Patrimônio, total investido e reserva de emergência** somam os três grupos num lugar só (a composição da carteira). Com isso, a mesma aplicação nunca soma duas vezes.

## Consequências

- **Positivas:**
  - **O rendimento da caixinha aparece no valor sem lançar nada**, e a aba Open Finance fica só com fundos e previdência.
  - **Sem contar duas vezes** a caixinha que é reserva e investimento ao mesmo tempo.
  - **A reserva fica previsível:** muda só com o que a pessoa guarda e tira.
  - **Quando a regra automática erra, a pessoa corrige**, e a correção fica.
- **Negativas:**
  - **Correspondência por nome ou valor:** pode errar nos dois sentidos. Juntar duas aplicações diferentes some com uma do patrimônio; não juntar conta duas vezes. A troca manual corrige, e a tela mostra a origem ("automática" ou "escolhida por você").
  - **Estimativas:** a caixinha que só está no extrato tem valor estimado (a Selic no lugar do CDI, o % do CDI informado à mão). O "aproximado" dos sem valor aplicado depende de ligar os lançamentos certos.
  - **Mais uma tabela com dado financeiro:** entra na cópia de segurança, no `DADOS-PESSOAIS.md` e no diagrama do banco.
  - **Três abas para o dinheiro guardado:** a pessoa precisa entender a diferença. A tela diz em cada uma o que entra ("reserva: só os seus aportes e retiradas; não rende").
  - **A regra da 0037 muda em parte:** a reserva que corresponde a um investimento sai de Reservas. Quem soma reservas por fora da composição passa a errar.
- **Na prática:**
  - **Código novo em Go**, atrás do mesmo login (ADR 0049; o .NET já saiu):
    - a classificação e o rendimento num pacote do assunto, em `internal/patrimonio`;
    - a soma do patrimônio só pela composição;
    - nada some por SQL em coluna de dinheiro.
  - **Banco:**
    - `ClassificacaoAplicacao` no fim do `banco/01-estrutura.sql`;
    - commit `feat(banco)!`;
    - `python3 scripts/ci/gerar-diagrama-banco.py`, com a tabela em `ASSUNTOS`;
    - `docs/DADOS-PESSOAIS.md` atualizado.
  - **Termos novos** em `docs/arquitetura/dominio.md`: caixinha, reserva, rendimento automático, valor de partida.
  - **Em partes, um PR cada:**
    1. esta ADR;
    2. a tabela, a classificação automática e a API (sem tela), com os casos de contrato;
    3. a aba "CDBs e caixinhas";
    4. a troca e o rendimento automático;
    5. a estimativa pelo CDI.

    Cada parte tem Manual e testes (ADR 0051 no Go; Jest e navegador no front-end).
  - **Ao aceitar esta ADR:** a 0037 ganha "Substituída em parte por 0060".
