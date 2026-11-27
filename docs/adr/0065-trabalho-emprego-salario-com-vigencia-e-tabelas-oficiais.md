# 0065. Trabalho: emprego, salário com vigência e as tabelas do INSS e do IRRF no banco

- **Status:** Aceita
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
- **Tabelas no banco, por vigência, com os valores oficiais num script próprio** (`03-tabelas-oficiais.sql`, idempotente, aplicado pelo site a cada partida, também em banco existente; #805): vêm com o site, valem na hora e podem ser corrigidas sem versão nova. Ficam fora do `02-dados-iniciais.sql`, que só roda em banco novo (rodá-lo de novo recriaria categoria apagada). Escolhida.
- **O holerite como receita** (somado nos relatórios): o crédito do salário já entra pela transação, e somar os dois dobraria a receita. Rejeitada.

## Decisão

### Emprego e salário

- `Emprego`: empregador, CNPJ (opcional), admissão e saída, regime (CLT; estágio e PJ ficam "sem cálculo de encargos"), jornada mensal (padrão 220 h), dependentes para o IR e a conta onde o salário cai.
- `SalarioVigencia`: o salário bruto a partir de um mês (sempre o dia 1). "Aumento a partir de…" (um seletor de mês) cria uma linha nova; o salário de um mês é o da última vigência que começou até ele, **sem proporção pelos dias** no mês do aumento (o holerite da empresa pode pagar proporcional; aqui é estimativa). O emprego nasce com a primeira vigência, no mês da admissão, e ela não sai: corrigir a admissão para antes leva o começo dela junto, e a saída não pode ficar antes de uma vigência (revisão do #698).

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

### A rescisão (#215)

- A simulação não é guardada: junta o salário do mês da saída, a média das horas extras dos 12 meses antes, as férias (os períodos completos com os dias que sobram, em dobro se o prazo passou; os avos do período em curso), o 13º (os avos do ano e a 1ª parcela que já veio) e o FGTS estimado (ADR 0066).
- O aviso indenizado conta como tempo de serviço: a **data projetada** vale para os avos do 13º e das férias. No acordo, metade dos dias (art. 484-A).
- Por tipo: dispensa sem justa causa (tudo, multa de 40%, saque de 100%, seguro com 12 meses); pedido de demissão (sem aviso pago, e sem cumprir desconta 30 dias; sem multa e sem saque); acordo (aviso pela metade, multa de 20%, saque de 80%); justa causa (só o saldo de salário e as férias vencidas); fim da experiência (sem aviso e sem multa, saque de 100%).
- INSS e IRRF só sobre o saldo de salário e, separados, sobre o 13º; as férias indenizadas e o aviso indenizado não têm.
- **Registrar** guarda a conta (JSON, como o holerite fechado) e põe a data de saída no emprego; **desfazer** apaga o registro e reabre.

### O recebimento (#212)

- `HoleriteRecebimento` liga a transação (o crédito na conta) ao holerite, com a parte do valor: um holerite pode vir em vários créditos (adiantamento e saldo), e um crédito pode pagar mais de um holerite. Sem saber o valor, a ligação leva o que está livre no crédito, até o que falta do líquido.
- A ligação guarda a conta, a data, o valor e a descrição da transação (revisão do #703): quando a sincronização troca o crédito (a pendente que o banco efetiva com outro código), a ligação fica sem a transação (`ON DELETE SET NULL`), conta como recebida e o site religa sozinho na transação nova da mesma conta, com o mesmo valor, até 5 dias de diferença (a mais perto na data). *Inferido:* a categoria "Salário" posta à mão na pendente se perde na troca (é do núcleo da sincronização); ao religar, ela volta.
- A sugestão precisa de um motivo forte (o valor, o banco marcou como salário, cita o empregador); a parte só conta a partir de 20% do líquido, e a transferência entre contas suas nunca é sugerida.
- O site **sugere**, e a pessoa confirma: os créditos livres do dia 15 do mês ao dia 15 do seguinte, por pontos (valor até 5% do líquido; menor que o líquido, como o adiantamento; o tipo de lançamento "salário" do banco, ADR 0025; a descrição que cita o empregador; a conta do salário), com o motivo em texto. Os outros créditos da janela ficam para ligar à mão.
- A transação ligada que estava **sem categoria** passa para "Salário", como escolha manual (`CategoriaManual`). Com categoria, fica como está.
- **O teste de regressão** confere que a receita dos relatórios (`relatorios.Totais`) é a mesma antes de ligar, depois de ligar, ao dividir um crédito entre dois holerites e depois de desligar.
- *Ficou de fora:* a etiqueta "salário de outubro" na tela de Transações, que é do núcleo: o núcleo não importa módulo (ADR 0064) e ainda não há um ponto de extensão para um módulo mostrar algo numa tela do núcleo.

### Holerite fechado (#211)

O holerite de um mês, depois de fechado, guarda os números calculados (e não só as entradas): mudar o salário ou a tabela depois não muda o mês fechado. Reabrir para corrigir fica registrado.

Como ficou na #211:
- `Holerite` (um por emprego e mês, `Situacao` aberto ou fechado) e `HoleriteLancamento` (horas extras, adicional noturno, outros proventos, vale-transporte, VR/VA, plano de saúde, pensão e outros descontos).
- Fechado, a coluna `Calculo` guarda o resultado inteiro em JSON (as linhas, os totais, o INSS, o IRRF e o FGTS do mês); aberto, ela fica nula e o cálculo é refeito a cada leitura. *Inferido:* JSON em vez de uma tabela por número, porque o holerite fechado só é lido inteiro, nunca filtrado nem somado no SQL.
- Reabrir grava `HoleriteReaberto (AAAA-MM)` na auditoria. Recusa (409, com o motivo) quando o cálculo de agora daria aviso, sem salário, fora do emprego ou sem tabela, porque o número congelado sumiria e o fechar não o devolveria (#750). O caso típico é a saída corrigida para antes de um mês fechado: mudar a saída confere os salários, não os holerites.
- O salário do mês é proporcional na admissão e na saída, pelo **mês comercial de 30 dias**: a admissão no dia D conta 30 − D + 1 dias, também em fevereiro: quem entra em 15/02 recebe 16/30, e quem entra em 28/02 recebe 3/30, e não 1/28 (#750). A saída no último dia do mês conta o mês inteiro. *Julgamento* (revisão do #699): há empresas que pagam fevereiro pelos dias do calendário; o comercial é o mais comum na folha.
- O DSR das horas extras e do adicional noturno é o total ÷ dias úteis × domingos e feriados. O padrão vem do calendário: segunda a sábado são úteis; os domingos e os **feriados nacionais de data fixa** (1/1, 21/4, 1/5, 7/9, 12/10, 2/11, 15/11, 20/11 e 25/12) são descanso. A pessoa acerta os outros feriados e, no mês da admissão ou da saída, os dias do período.
- **Fica de fora** (*julgamento*, dito no Manual): a hora extra noturna com o adicional noturno na base (OJ 97 da SDI-1 do TST) e a hora noturna reduzida (52'30"). A pessoa lança a hora extra noturna com o percentual somado (1,2 × 1,5 = 80%) e as horas já convertidas.
- Outro provento (bônus, PLR, comissão) diz se tem INSS e IRRF; a pensão alimentícia e o "outro desconto" marcado deduzem do IR; o vale-transporte desconta o custo do mês até 6% do salário.
- O lançamento marcado "repetir" passa para o holerite do mês seguinte, quando ele é aberto.

### 13º salário (#214)

- Calculado, sem guardar o resultado: (salário de dezembro, ou o do mês da saída, + a média das horas extras, do adicional noturno e do DSR dos holerites do ano) × avos ÷ 12. Os avos contam o mês com 15 dias ou mais de calendário.
- *Inferido:* a média divide a soma das variáveis do ano pelos avos (e não por 11 ou 12 fixos); bônus e comissão não entram (só o que é hora). *Julgamento* (#750): a soma usa só os holerites dos meses que contam avo, para o numerador e o denominador cobrirem os mesmos meses. O mês da admissão ou da saída com menos de 15 dias não conta para o 13º, e as horas dele também não; antes, elas entravam na soma sem entrar nos avos e subiam a média.
- A 1ª parcela informada: o "não passa do total" só confere quando o valor muda; trocar só a data de uma 1ª que ficou acima do total (um holerite apagado, o salário de dezembro menor) grava (#750). A resposta diz se a data e o valor vieram da pessoa (`dataInformada`, `valorInformado`), para a tela não gravar o 30/11 padrão nem a metade calculada como informados.
- **Súmula 347 do TST** (revisão do #700): as horas extras entram pelas horas, com o salário-hora de agora. O valor de cada mês vai corrigido por salário de agora ÷ salário daquele mês, o que dá o mesmo que refazer as horas e vale também para o holerite lido do PDF, que não guarda as horas. O mês sem holerite entra como zero, e a tela diz quantos meses têm holerite.
- A 2ª parcela nunca fica negativa: a 1ª informada maior que o 13º líquido vira aviso ("a empresa desconta a diferença"), e a 1ª não pode passar do 13º do ano inteiro.
- 1ª parcela: metade, sem desconto, até 30/11. A tabela `DecimoTerceiro` guarda só o que a pessoa informa (quando e quanto veio, como na 1ª adiantada nas férias).
- 2ª parcela: o total menos o INSS (sobre o total), o IRRF exclusivo na fonte e a 1ª. *Inferido:* o IR do 13º usa só as deduções da lei (INSS e dependentes), sem o desconto simplificado; a redução da Lei 15.270/2025 vale para ele (art. 3º-A, § 3º).
- Quem saiu no ano recebe o proporcional na rescisão (#215): a tela avisa.

### Férias (#213)

- Os períodos aquisitivos saem da admissão (de 12 em 12 meses) e não são guardados; o concessivo são os 12 meses seguintes. Guardados: as faltas sem justificativa de cada período (`FeriasFaltas`, que dão os dias de direito do art. 130 da CLT) e as frações agendadas (`FeriasAgendamento`).
- Os períodos contam da admissão pelo calendário: quem entrou em 29/02 faz aniversário em 01/03 nos anos comuns (revisão do #701).
- Regras conferidas ao agendar: as férias de um período começam depois que ele termina; até 3 frações, uma de 14 dias ou mais e as outras de 5 ou mais (art. 134, § 1º), regra que só vale com fracionamento (o direito de 12 dias vai num bloco só); venda de até 1/3 (art. 143); o total até o direito; a fração não cruza com outra; nada depois da saída; a 1ª parcela do 13º junto uma vez por ano, só com as férias pagas (2 dias antes do início) até 30/11, quando ela vence, e no mesmo ano; os dias e os vendidos não passam do direito (conferido antes de somar); e as faltas novas não deixam o direito menor que o já agendado, nem a venda acima de 1/3 do direito novo.
- **Em dobro:** os dias da fração depois do fim do concessivo são pagos duas vezes (art. 137; Súmula 81 do TST: só os dias de depois), também no "Ver a conta" do período. A dobra e o 1/3 dela ficam fora da base do INSS (Lei 8.212/1991, art. 28, § 9º, "d"; o 1/3 da dobra junto, *inferido*); o IRRF incide sobre o todo (*inferido*: a dobra das férias gozadas é rendimento do trabalho; revisão do #701). *Ficou de fora:* não começar nos 2 dias antes de feriado ou do descanso (art. 134, § 3º), que pede o calendário de feriados.
- A conta: a remuneração é o salário do mês do início mais a média das horas extras dos 12 meses antes (*inferido:* soma ÷ 12); férias + 1/3 com INSS e IRRF (pelas regras do salário, com o desconto simplificado e a redução da Lei 15.270/2025, *inferido*; as tabelas são as do mês do início para a fração inteira, mesmo a que entra no mês seguinte, e o INSS das férias é calculado à parte do salário do mês, *inferido*: a folha costuma somar os dois na base do mês); o abono pecuniário + 1/3 sem INSS nem IR; pagamento até 2 dias antes (art. 145); a 1ª parcela do 13º junto, se pedida (metade da remuneração). O 13º mostra essa 1ª parcela no valor que as férias pagaram, e desconta da 2ª: ele pede às férias por uma interface (`decimoterceiro.Ferias`, ligada no `trabalho.go`), sem ler a tabela delas (revisão do #701).
- Vencida (o concessivo passou com saldo) e vencendo (faltam 60 dias ou menos) aparecem na tela.

## Consequências

- **Positivas:**
  - Aumento e tabela nova não mudam o passado.
  - Tabela nova do governo pode entrar sem versão nova do site (pelo banco), e a próxima versão traz a oficial nos dados iniciais.
  - Os relatórios não mudam com o Trabalho ligado ou desligado.
- **Negativas:**
  - Seis tabelas novas no banco (`feat(banco)!`), que existem mesmo com o módulo desligado.
  - A tabela de cada ano precisa entrar no `03-tabelas-oficiais.sql` a cada ano novo (tarefa de manutenção, com a fonte citada).
  - O cálculo é estimativa: a pessoa pode achar que é o valor do holerite. A tela diz que não é, sempre.
- **Na prática:**
  - Pacotes em `internal/modulos/trabalho` (o módulo), `internal/modulos/trabalho/emprego` e `internal/modulos/trabalho/encargos`; rotas em `/api/trabalho/`.
  - Tabela nova do governo: uma vigência nova em `banco/03-tabelas-oficiais.sql` (com a fonte), teste do cálculo com casos da tabela nova.
  - Dinheiro sempre em `tipos.Decimal`, nunca `float64`; comparação e soma em Go.

### Revisão da rescisão (#704, 16/11/2026)

- O FGTS da rescisão é o saldo até o mês antes da saída, como se o emprego terminasse nela (`fgts.ParaRescisao`): o mês da saída, o aviso e o 13º do ano entram só no depósito da rescisão (antes, contavam duas vezes). A multa é sobre todos os depósitos corrigidos, sem descontar os saques (Lei 8.036, art. 18, § 1º; *inferido*: os saques voltam sem a correção deles). Quem fez saque-aniversário saca só a multa (Lei 13.932/2019).
- O aviso indenizado que projeta a saída para o ano seguinte traz os avos do 13º desse ano; a saída depois de 20/12 não paga o 13º do ano de novo; no acordo, o aviso pela metade paga o meio dia (a projeção conta os dias inteiros, *inferido*).
- Registrar guarda a saída que o emprego já tinha (`Rescisao.SaidaAnterior`), e o desfazer a devolve.
- 2ª revisão: na saída depois de 20/12, o 13º do ano já veio, mas os 8% dele vencem em 20/01 e entram no depósito da rescisão. O saque do mês da saída sai do saldo (a estimativa vai até o mês antes) e não muda a base da multa, que soma só os saques de antes desse mês.
