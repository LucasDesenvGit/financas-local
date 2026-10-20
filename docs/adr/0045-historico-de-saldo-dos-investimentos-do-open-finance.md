# 0045. Histórico de saldo dos investimentos do Open Finance, um registro por dia

- **Status:** Aceita
- **Data:** 2026-10-20
- **Relacionadas:** [0003](0003-esquema-por-scripts-sql-idempotentes.md) (estrutura só acrescenta), [0027](0027-conferencia-de-investimentos-com-previa-e-desfazer.md) (renda variável pelo extrato)

## Contexto

O app do banco mostra quanto a caixinha (um CDB) rendeu em cada mês. O site não tinha como saber: o Open Finance manda só o saldo e o valor aplicado **de hoje**, o rendimento da caixinha não vira lançamento no extrato, e a sincronização sobrescrevia o investimento a cada vez. Na carteira real, os 6 CDBs vêm com o valor aplicado; o fundo, só com o saldo.

A #173 pediu o rendimento mensal da caixinha, como no app.

## Alternativas consideradas

- **Calcular pelo extrato** (aplicações e resgates de RDB/CDB, regras da #172).
  - Prós: nada para guardar.
  - Contras: o rendimento não aparece no extrato. Com várias caixinhas, não dá para saber de qual é cada aplicação.
- **Buscar as movimentações de cada CDB na Pluggy** (`/investments/{id}/transactions`) e fazer Δ saldo − aportes + resgates.
  - Prós: separa aporte e resgate exatamente.
  - Contras: uma chamada a mais por investimento a cada sincronização, e nem toda instituição manda. Ainda precisaria do saldo do início do mês, que também não vem.
- **Guardar o saldo de cada sincronização** (todas as linhas).
  - Prós: o máximo de detalhe.
  - Contras: várias linhas por dia sem uso, e a tabela cresce à toa.
- **Guardar um registro por investimento e dia** (a última sincronização do dia vale).
  - Prós: pouco espaço (cerca de 10 linhas por dia); o fim de cada mês fica registrado.
  - Contras: o passado não volta, e o histórico começa no dia da instalação.

## Decisão

A tabela nova **`HistoricoInvestimento`** guarda, por investimento do Open Finance e por dia: saldo, valor aplicado, nome e tipo do banco. A sincronização faz o upsert no dia (`HistoricoInvestimentosService.RegistrarAsync`). Entram renda fixa, fundos e outros em reais; a renda variável fica de fora, porque tem a carteira própria.

**Sem chave estrangeira para `Investimento`, de propósito:** quando a instituição para de mandar um investimento resgatado, ele sai de `Investimento`, e o histórico dele fica.

**Rendimento do mês** (`GET /api/investimentos/historico`): para cada investimento, é a variação de (saldo − valor aplicado) do fim do mês anterior ao fim do mês.
- O que foi guardado ou tirado entra no valor aplicado, então não conta como rendimento.
- Sem o valor aplicado, o investimento mostra só o saldo.
- O mês em que o histórico começa conta a partir do primeiro registro, e a tela diz "desde dd/MM".

A tela é a seção **"Rendimento por mês"** na aba Open Finance de Investimentos. Ela segue o mesmo padrão do "Mês a mês" dos FIIs: a frase do último mês, a tabela com o mais recente primeiro e o detalhe "por investimento".

## Consequências

- **Positivas:**
  - o rendimento mensal da caixinha aparece como no app, a partir da instalação;
  - investimento resgatado não some do histórico;
  - os dados ficam prontos para outras análises (evolução do saldo, comparação com a Selic mês a mês).
- **Negativas:**
  - **O passado não volta:** o primeiro mês fica parcial, e os meses de antes não existem.
  - **Resgate:** quando o banco tira do valor aplicado só a parte do principal, o rendimento já recebido no resgate some de (saldo − aplicado). O mês do resgate pode mostrar um rendimento menor (*inferido*; conferir com o app no primeiro resgate).
  - **Mês com várias sincronizações paradas:** se a sincronização ficar dias sem rodar, o fim do mês usa o último dia registrado.
  - **Mais uma tabela** na cópia de segurança e na migração.
- **Na prática:**
  - Mudou a estrutura: tabela no fim de `banco/01-estrutura.sql`, entidade em `Entidades/OpenFinance.cs`, mapeamento no `FinancasDbContext` e commit `feat(banco)!`.
  - Para comparar com o app, valem os meses completos, a partir do segundo mês de histórico.
