# 0065. Trabalho: emprego, salário com vigência e as tabelas do INSS e do IRRF no banco

- **Status:** Aceita
- **Data:** 2026-11-13
- **Relacionadas:** [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md), [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md), [0025](0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md), [0031](0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md)

As regras de cada história (holerite #211, recebimento #212, férias #213, 13º #214, rescisão #215) e das revisões (#699 a #704, #750, #751) estão no código e nos testes de cada pacote ([componentes](../arquitetura/componentes.md)), no Manual e nos [termos do domínio](../arquitetura/dominio.md#termos-para-quem-desenvolve). O texto de cada uma, com a revisão da rescisão, ficou na [versão anterior desta ADR](https://github.com/LucasDesenvGit/financas-local/blob/5082de4d0dbf632df229e1cdf461b3d3196ef1cf/docs/adr/0065-trabalho-emprego-salario-com-vigencia-e-tabelas-oficiais.md).

## Contexto

O épico #209 traz o dinheiro do trabalho para o site: o salário pelo bruto, o holerite, as férias, o 13º, a rescisão e o FGTS. Antes, o salário era só uma transação na categoria "Salário". A base (#210) decide quatro coisas que as outras histórias usam:

1. como guardar o salário que muda com o tempo (um aumento não pode mudar o mês que já passou);
2. onde ficam as tabelas do INSS e do IRRF, que o governo muda todo ano (às vezes no meio do ano, como o IRRF em maio de 2025);
3. como o holerite fecha e congela;
4. como não contar o salário duas vezes nos relatórios.

O Trabalho é o primeiro módulo do site (ADR 0064): mora em `internal/modulos/trabalho` e nasce desligado.

## Alternativas consideradas

- **Salário como um campo do emprego**, trocado a cada aumento: simples, mas o mês passado passaria a ser calculado com o salário novo. Rejeitada.
- **Salário com vigência** (uma linha por salário, com o mês em que começou a valer): o mês usa a última vigência que começou até ele. Escolhida.
- **Tabelas do INSS e do IRRF no código** (como o Manual, ADR 0026): os testes ficam ao lado, mas a pessoa depende de uma versão nova do site quando o governo muda a tabela no meio do ano. Rejeitada.
- **Tabelas no banco, por vigência, com os valores oficiais num script próprio** (`03-tabelas-oficiais.sql`, idempotente, aplicado pelo site a cada partida, também em banco existente; #805): vêm com o site, valem na hora e podem ser corrigidas sem versão nova. Ficam fora do `02-dados-iniciais.sql`, que só roda em banco novo (rodá-lo de novo recriaria categoria apagada). Escolhida.
- **O holerite como receita** (somado nos relatórios): o crédito do salário já entra pela transação, e somar os dois dobraria a receita. Rejeitada.

## Decisão

### Emprego e salário com vigência

- `Emprego`: empregador, CNPJ (opcional), admissão e saída, regime (CLT; estágio e PJ ficam sem cálculo de encargos), jornada mensal, dependentes para o IR e a conta onde o salário cai.
- `SalarioVigencia`: o salário bruto a partir de um mês (sempre o dia 1). Um aumento cria uma linha nova; o salário de um mês é o da última vigência que começou até ele, **sem proporção pelos dias** no mês do aumento (o holerite da empresa pode pagar proporcional; aqui é estimativa). O emprego nasce com a primeira vigência, no mês da admissão, e ela não sai.

### Tabelas oficiais, por vigência

- `InssVigencia` e `InssFaixa`: as faixas progressivas; a última é o teto. `IrrfVigencia` e `IrrfFaixa`: as faixas com a parcela a deduzir, a dedução por dependente, o desconto simplificado e, desde 2026, os parâmetros da redução da Lei 15.270/2025.
- A tabela de um mês é a da última vigência que começou até ele: a de 2025 continua calculando 2025 depois que a de 2026 entra. Cada vigência guarda a **fonte** (a portaria ou a lei), que a tela mostra.
- Os valores entram uma vez, sem repetir (`WHERE NOT EXISTS`), pelo `03-tabelas-oficiais.sql` ([ADR 0003](0003-esquema-por-scripts-sql-idempotentes.md)). *Inferido:* a edição pelo site fica para uma história à parte (a Parametrização de hoje só edita parâmetros de chave e valor).

### O cálculo é estimativa, não holerite

- **INSS** progressivo, faixa por faixa, até o teto, arredondado no fim. **IRRF** sobre o bruto menos o INSS e as deduções, com o **desconto simplificado** quando ele dá menos imposto (como a fonte pagadora pode fazer); depois, a **redução da Lei 15.270/2025** (art. 3º-A da Lei 9.250/1995), nunca maior que o imposto.
- A tela mostra **como calculou**, linha a linha. O valor que vale é o do holerite e do extrato: convenção coletiva, benefícios e arredondamentos da empresa mudam o número.

### Não contar duas vezes

A receita dos relatórios continua vindo **só das transações** (o crédito do salário na conta). O emprego, o holerite, as férias, o 13º e a rescisão são o **registro e a explicação** desse dinheiro, nunca uma segunda receita. Nenhuma tabela do Trabalho entra na base dos relatórios (`internal/analises/relatorios`). Quando não há transação (conta não conectada), a pessoa lança como sempre. A ligação do holerite com a transação é a #212.

### Holerite fechado (#211)

O holerite de um mês (`Holerite`, com os `HoleriteLancamento`), depois de fechado, guarda o resultado inteiro na coluna `Calculo`: mudar o salário ou a tabela depois não muda o mês fechado. Aberto, a coluna fica nula e o cálculo é refeito a cada leitura. *Inferido:* JSON em vez de uma tabela por número, porque o holerite fechado só é lido inteiro, nunca filtrado nem somado no SQL. Reabrir fica na auditoria e é recusado quando o cálculo de agora não daria número (sem salário, fora do emprego ou sem tabela), porque o congelado sumiria e o fechar não o devolveria (#750).

### 13º, férias e rescisão: calculados, não guardados

O 13º (#214), os períodos de férias (#213) e a simulação da rescisão (#215) saem do emprego, dos salários e dos holerites a cada leitura. As tabelas guardam só o que a pessoa informa ou registra: `DecimoTerceiro` (a 1ª parcela que veio), `FeriasFaltas`, `FeriasAgendamento` e `Rescisao` (a conta registrada, em JSON como o holerite fechado, e a saída anterior, que o desfazer devolve). As regras da CLT estão no código, com os testes, e no Manual. O 13º pede às férias a 1ª parcela paga junto com elas por uma interface (`decimoterceiro.Ferias`), sem ler a tabela delas.

### O recebimento (#212)

- `HoleriteRecebimento` liga a transação (o crédito na conta) ao holerite, com a parte do valor: um holerite pode vir em vários créditos (adiantamento e saldo), e um crédito pode pagar mais de um holerite.
- O site **sugere**, por pontos e com o motivo em texto, e a pessoa confirma; a transferência entre contas suas nunca é sugerida. A transação ligada que estava **sem categoria** passa para "Salário", como escolha manual (`CategoriaManual`).
- A ligação guarda a conta, a data, o valor e a descrição da transação. Quando a sincronização troca o crédito (a pendente que o banco efetiva com outro código), ou a pessoa apaga à mão a transação ligada (*julgamento*, #751), a ligação fica sem ela (`ON DELETE SET NULL`), continua contando como recebida, e o site religa na substituta: mesma conta, mesmo valor, até 5 dias de diferença. Quem não quer a ligação usa "Desligar".
- **O religar roda no GET do recebimento** (*julgamento*, #751): é um GET que grava, por desenho. A sincronização é do núcleo, que não importa módulo (ADR 0064), então não tem como avisar o Trabalho da troca; o religar só preenche a ligação vazia, é idempotente, e a falha dele vai para o log sem derrubar a tela.
- **O teste de regressão** confere que a receita dos relatórios (`relatorios.Totais`) é a mesma antes de ligar, depois de ligar, ao dividir um crédito entre dois holerites e depois de desligar.
- *Ficou de fora:* a etiqueta "salário de outubro" na tela de Transações, que é do núcleo, até existir um ponto de extensão para um módulo mostrar algo numa tela do núcleo.

### Escolhas sem confirmação

O que a lei ou a fonte não fecha e o site escolheu (*inferido* ou *julgamento*). O código cita esta ADR perto de cada conta:

- **IRRF:** o "rendimento tributável" da redução da Lei 15.270/2025 é o bruto tributável do mês, antes do INSS, como o simulador da Receita (fica na vigência, corrigível sem mudar código). O IR do 13º usa só as deduções da lei (INSS e dependentes), sem o desconto simplificado, e com a redução (art. 3º-A, § 3º).
- **Holerite:** o salário proporcional na admissão e na saída é pelo **mês comercial de 30 dias**, também em fevereiro (há empresas que pagam pelos dias do calendário; o comercial é o mais comum na folha). Ficam de fora a hora extra noturna com o adicional na base (OJ 97 da SDI-1 do TST) e a hora noturna reduzida; o Manual diz como lançar.
- **13º:** a média das horas divide a soma pelos avos, e só os holerites dos meses que contam avo entram na soma (#750); bônus e comissão não entram.
- **Férias:** a média das horas é a soma dos 12 meses antes ÷ 12; as tabelas são as do mês do início para a fração inteira; o INSS das férias é calculado à parte do salário do mês (a folha costuma somar os dois); a dobra entra no IRRF, e o 1/3 dela fica fora do INSS junto com ela. Fica de fora não começar nos 2 dias antes de feriado ou do descanso (art. 134, § 3º).
- **Rescisão (#704):** a multa do FGTS é sobre todos os depósitos corrigidos, sem descontar os saques, que voltam sem a correção deles; no acordo, a projeção do aviso pela metade conta os dias inteiros.

## Consequências

- **Positivas:**
  - Aumento e tabela nova não mudam o passado.
  - Tabela nova do governo pode entrar sem versão nova do site (pelo banco), e a próxima versão traz a oficial no `03-tabelas-oficiais.sql`.
  - Os relatórios não mudam com o Trabalho ligado ou desligado.
- **Negativas:**
  - Seis tabelas novas no banco na #210 (`feat(banco)!`), mais as das histórias seguintes, que existem mesmo com o módulo desligado.
  - A tabela de cada ano precisa entrar no `03-tabelas-oficiais.sql` a cada ano novo (tarefa de manutenção, com a fonte citada).
  - O cálculo é estimativa: a pessoa pode achar que é o valor do holerite. A tela diz que não é, sempre.
  - O religar do recebimento é um GET que grava.
- **Na prática:**
  - Pacotes em `internal/modulos/trabalho` e nos subpacotes listados nos [componentes](../arquitetura/componentes.md); rotas em `/api/trabalho/`.
  - Tabela nova do governo: uma vigência nova em `banco/03-tabelas-oficiais.sql` (com a fonte), teste do cálculo com casos da tabela nova.
