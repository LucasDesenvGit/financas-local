# 0066. FGTS fora do patrimônio: estimado no Trabalho e conferido com o app

- **Status:** Proposta
- **Data:** 2026-11-14
- **Relacionadas:** [0065](0065-trabalho-emprego-salario-com-vigencia-e-tabelas-oficiais.md), [0010](0010-reservas-estimadas-pelo-extrato.md), [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md)

## Contexto

O épico Trabalho (#209) quer acompanhar o FGTS (#216): o depósito de cada mês (8% do bruto, também sobre horas extras, férias e 13º), o saldo por emprego e os saques. Duas perguntas:

1. **O FGTS entra no patrimônio** (o total do Painel)? É dinheiro da pessoa, mas não está disponível: só sai em situações da lei (demissão sem justa causa, imóvel, aposentadoria, saque-aniversário).
2. **De onde vem o saldo?** O site não tem acesso à Caixa (e o Trabalho não acessa a internet, ADR 0064). A correção oficial é TR + 3% ao ano, mais a distribuição de resultados, e a TR muda todo mês.

## Alternativas consideradas

- **FGTS no patrimônio**, somado ao total do Painel: mostra a riqueza inteira, mas mistura dinheiro disponível com bloqueado, e o Painel é do núcleo, que não pode depender de um módulo (ADR 0064). Rejeitada.
- **FGTS à parte, no Trabalho** (escolhida): uma tela própria, com o saldo estimado e o conferido, fora do total do Painel.
- **Buscar a TR no Banco Central** (o site já fala com `api.bcb.gov.br` para a Selic): a correção ficaria certa, mas o Trabalho passaria a acessar a internet (nasceria desligado por isso, e a busca seria uma saída de rede nova). Rejeitada por enquanto.
- **Só o saldo informado**, sem estimativa: simples, mas não diz quanto entrou no mês nem quanto deveria haver.

## Decisão

- **O FGTS fica fora do patrimônio**, numa tela própria em Trabalho › FGTS. Não entra no Painel nem nos relatórios. O saque entra pela transação, como qualquer crédito (ADR 0065, "não contar duas vezes").
- **Saldo estimado** mês a mês, a partir do último saldo conferido (ou de zero na admissão): o saldo de antes corrigido a **3% ao ano** (0,2466% ao mês, juros compostos), mais o depósito do mês (8% da base do INSS do holerite, ou do salário quando não há holerite), mais 8% do 13º em dezembro e das férias (+ 1/3) no mês em que começam, mais a distribuição de resultados informada, menos os saques. *Inferido:* sem a TR, que tem sido perto de zero e pede uma saída de rede; a conferência com o app acerta a diferença.
- **Saldo conferido**, como nas reservas (ADR 0010): a pessoa digita o saldo do app do FGTS de vez em quando, e a estimativa recomeça dali. A tela mostra a diferença entre o que o site estimava e o que o app disse.
- **Lançamentos** numa tabela só (`FgtsLancamento`): a conferência do saldo, o saque (com o motivo: rescisão, saque-aniversário, imóvel, outro) e a distribuição de resultados.
- O FGTS é por emprego (como a conta vinculada da Caixa).

## Consequências

- **Positivas:** o Painel continua mostrando só o que está disponível; a pessoa vê quanto entrou de FGTS em cada mês e se a empresa depositou (a diferença na conferência); nenhuma saída de rede nova.
- **Negativas:** a estimativa erra pela TR e pelos arredondamentos; a pessoa precisa conferir com o app de vez em quando. Patrimônio "total com FGTS" não existe por enquanto.
- **Na prática:** pacote `internal/modulos/trabalho/fgts`; rotas em `/api/trabalho/empregos/{id}/fgts`. A rescisão (#215) usa o saldo estimado para a multa de 40% (20% no acordo) e o saque.
