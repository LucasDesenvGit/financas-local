# 0065. Trabalho: emprego, salário com vigência e as tabelas do INSS e do IRRF no banco

- **Status:** Proposta
- **Data:** 2026-11-13
- **Relacionadas:** [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md), [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md), [0025](0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md), [0031](0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md)

## Contexto

O épico #209 traz o dinheiro do trabalho para o site: o salário pelo bruto, o holerite, as férias, o 13º, a rescisão e o FGTS. Hoje o salário é só uma transação na categoria "Salário". A base (#210) precisa decidir quatro coisas que todas as outras histórias usam:

1. como guardar o salário que muda com o tempo (um aumento não pode mudar o mês que já passou);
2. onde ficam as tabelas do INSS e do IRRF, que o governo muda todo ano (às vezes no meio do ano, como o IRRF em maio de 2025);
3. como o holerite fecha e congela (detalhado na #211);
4. como não contar o salário duas vezes nos relatórios.

O Trabalho é o primeiro módulo do site (ADR 0064): mora em `internal/modulos/trabalho` e nasce desligado.

## Alternativas consideradas

- **Salário como um campo do emprego**, trocado a cada aumento: simples, mas o mês passado passaria a ser calculado com o salário novo. Rejeitada.
- **Salário com vigência** (uma linha por salário, com o mês em que começou a valer): o mês usa a última vigência que começou até ele. Escolhida.
- **Tabelas do INSS e do IRRF no código** (como o Manual, ADR 0026): os testes ficam ao lado, mas a pessoa depende de uma versão nova do site quando o governo muda a tabela no meio do ano. Rejeitada.
- **Tabelas no banco, por vigência, com os valores oficiais nos dados iniciais** (`02-dados-iniciais.sql`): vêm com o site, valem na hora e podem ser corrigidas sem versão nova. Escolhida.
- **O holerite como receita** (somado nos relatórios): o crédito do salário já entra pela transação, e somar os dois dobraria a receita. Rejeitada.

## Decisão

### Emprego e salário

- `Emprego`: empregador, CNPJ (opcional), admissão e saída, regime (CLT; estágio e PJ ficam "sem cálculo de encargos"), jornada mensal (padrão 220 h), dependentes para o IR e a conta onde o salário cai.
- `SalarioVigencia`: o salário bruto a partir de um mês (sempre o dia 1). "Aumento a partir de…" cria uma linha nova; o salário de um mês é o da última vigência que começou até ele. O emprego nasce com a primeira vigência, no mês da admissão.

### Tabelas oficiais, por vigência

- `InssVigencia` e `InssFaixa`: as faixas progressivas (até quanto e a alíquota); a última faixa é o teto.
- `IrrfVigencia` e `IrrfFaixa`: as faixas (até quanto, alíquota e parcela a deduzir), a dedução por dependente, o desconto simplificado mensal e, desde 2026, a redução da Lei 15.270/2025.
- A tabela de um mês é a da última vigência que começou até ele: a de 2025 continua calculando 2025 depois que a de 2026 entra.
- Cada vigência guarda a **fonte** (a portaria ou a lei), que a tela mostra.
- Os valores vêm nos dados iniciais e entram uma vez, sem repetir (`WHERE NOT EXISTS`). *Inferido:* a edição pelo site fica para uma história à parte (o pedido da #210 a pôs em Configurações › Parametrização, que hoje só edita parâmetros de chave e valor).

### O cálculo (estimativa, não holerite)

- **INSS:** progressivo, faixa por faixa, até o teto; arredondado no fim, ao centavo.
- **IRRF:** a base é o bruto menos o INSS e as deduções (dependentes); o site usa o **desconto simplificado** quando ele dá menos imposto (como a fonte pagadora pode fazer). Depois, a **redução da Lei 15.270/2025** (art. 3º-A da Lei 9.250/1995): até R$ 5.000,00 de rendimento tributável no mês, o imposto zera (redução de até R$ 312,89); de R$ 5.000,01 a R$ 7.350,00, a redução é R$ 978,62 − 0,133145 × o rendimento; nunca maior que o imposto. *Inferido:* o "rendimento tributável" da redução é o bruto tributável do mês (antes do INSS), como o simulador da Receita; fica guardado na vigência, então dá para corrigir sem mudar código.
- A tela mostra **como calculou**, linha a linha. O valor que vale é o do holerite e do extrato: convenção coletiva, benefícios e arredondamentos da empresa mudam o número.

### Não contar duas vezes

A receita dos relatórios continua vindo **só das transações** (o crédito do salário na conta). O emprego, o holerite, as férias, o 13º e a rescisão são o **registro e a explicação** desse dinheiro, nunca uma segunda receita. Nenhuma tabela do Trabalho entra na base dos relatórios (`internal/analises/relatorios`). Quando não há transação (conta não conectada), a pessoa lança como sempre. A ligação do holerite com a transação é a #212.

### Holerite fechado (#211)

O holerite de um mês, depois de fechado, guarda os números calculados (e não só as entradas): mudar o salário ou a tabela depois não muda o mês fechado. Reabrir para corrigir fica registrado.

Como ficou na #211:
- `Holerite` (um por emprego e mês, `Situacao` aberto ou fechado) e `HoleriteLancamento` (horas extras, adicional noturno, outros proventos, vale-transporte, VR/VA, plano de saúde, pensão e outros descontos).
- Fechado, a coluna `Calculo` guarda o resultado inteiro em JSON (as linhas, os totais, o INSS, o IRRF e o FGTS do mês); aberto, ela fica nula e o cálculo é refeito a cada leitura. *Inferido:* JSON em vez de uma tabela por número, porque o holerite fechado só é lido inteiro, nunca filtrado nem somado no SQL.
- Reabrir grava `HoleriteReaberto (AAAA-MM)` na auditoria.
- O salário do mês é proporcional na admissão e na saída, pelo mês comercial de 30 dias; o DSR das horas extras e do adicional noturno é o total ÷ dias úteis × domingos e feriados (o padrão vem do calendário, segunda a sábado; a pessoa acerta os feriados).
- Outro provento (bônus, PLR, comissão) diz se tem INSS e IRRF; a pensão alimentícia e o "outro desconto" marcado deduzem do IR; o vale-transporte desconta o custo do mês até 6% do salário.
- O lançamento marcado "repetir" passa para o holerite do mês seguinte, quando ele é aberto.

## Consequências

- **Positivas:**
  - Aumento e tabela nova não mudam o passado.
  - Tabela nova do governo pode entrar sem versão nova do site (pelo banco), e a próxima versão traz a oficial nos dados iniciais.
  - Os relatórios não mudam com o Trabalho ligado ou desligado.
- **Negativas:**
  - Seis tabelas novas no banco (`feat(banco)!`), que existem mesmo com o módulo desligado.
  - A tabela de cada ano precisa entrar nos dados iniciais a cada ano novo (tarefa de manutenção, com a fonte citada).
  - O cálculo é estimativa: a pessoa pode achar que é o valor do holerite. A tela diz que não é, sempre.
- **Na prática:**
  - Pacotes em `internal/modulos/trabalho` (o módulo), `internal/modulos/trabalho/emprego` e `internal/modulos/trabalho/encargos`; rotas em `/api/trabalho/`.
  - Tabela nova do governo: uma vigência nova em `banco/02-dados-iniciais.sql` (com a fonte), teste do cálculo com casos da tabela nova.
  - Dinheiro sempre em `tipos.Decimal`, nunca `float64`; comparação e soma em Go.
