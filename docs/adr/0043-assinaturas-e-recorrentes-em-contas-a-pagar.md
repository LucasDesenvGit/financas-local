# 0043. Assinaturas e cobranças recorrentes estendendo Contas a pagar

- **Status:** Aceita
- **Data:** 2026-10-20 (issue #158)
- **Relacionadas:** [0017](0017-preferencias-de-exibicao-no-navegador.md) (sugestão ignorada lembrada no navegador), [0030](0030-regras-de-descricao-em-tabela-propria.md) (descrição original)

## Contexto

Com vários cartões e contas, o usuário se perde:
- qual assinatura cobra em qual cartão;
- qual débito automático sai de qual conta;
- quanto vai por mês nisso.

O site já tinha **Contas a pagar** (`ContaAgendada`): descrição, valor, recorrência, próximo vencimento, categoria, conta de pagamento e o fluxo "Pagar". Tinha também a **sugestão pelo extrato**, mas ela deixava os cartões de fora, e por isso nenhuma assinatura no cartão era encontrada.

`ContaAgendada` aparece em dois lugares: na tela de Contas a pagar e nos "Próximos vencimentos" do Painel. Ela **não** entra na Projeção nem nas Metas, que tiram o futuro das transações.

## Alternativas consideradas

- **Tabela nova `CobrancaRecorrente`:**
  - separa o que se paga à mão do que é cobrado sozinho;
  - mas duplica a recorrência, o vencimento, a tela e a sugestão, e o usuário teria duas listas de "coisas que se repetem".
- **Estender `ContaAgendada`** com a forma de cobrança e o texto que liga às transações. **Escolhida pelo usuário** (20/10/2026).

## Decisão

- **Três colunas novas em `ContaAgendada`:**
  - `FormaCobranca`: à mão (1, o padrão de antes), débito automático (2) ou cartão de crédito (3);
  - `TextoCobranca`: "a descrição do banco contém…", gravado sem acento e em maiúsculas;
  - `Observacao`.

  `ContaId` passa a ser também o cartão onde a assinatura cai.
- **Baixa sozinha** (`ContaAgendadaService.ConciliarAsync`). Vale para a conta cobrada sozinha que tem o texto da cobrança:
  - quando chega uma transação com o texto, de até 10 dias antes do vencimento esperado e na conta ou cartão dela (se escolhido), a conta se dá como paga, como no "Pagar", e o vencimento avança;
  - a comparação é com a **descrição original** do banco, então o renome (ADR 0030) não atrapalha;
  - roda no fim da sincronização, ao iniciar a API e ao salvar uma conta a pagar.
  - Sem o texto, a conta continua com o "Pagar".
- **Sugestão pelo extrato** (`SugestaoContaAgendadaService`):
  - **cartões:** entram, menos as parcelas (pelos campos de parcela ou "PARC 3/10" no texto). No cartão, assinatura tem **valor fixo: até 5% de diferença nas últimas três cobranças** (decisão do usuário), e o resto (iFood, Uber) é gasto frequente;
  - **anual:** 2 ou 3 cobranças, cada uma de 330 a 400 dias depois da anterior, numa janela de 400 dias;
  - **forma sugerida:** cartão quando cai num cartão; "débito automático" quando o texto diz "DEB AUT"/"DEBITO AUT…"; à mão no resto;
  - **texto sugerido:** a palavra mais longa da chave (ex.: NETFLIX).
- **"Não é" (ignorar a sugestão)** continua lembrado no navegador (ADR 0017): é conveniência de tela, não dado financeiro.
- **Painel:** o que cai no cartão sai dos "Próximos vencimentos", porque já entra na fatura do mês.
- **Avisos** (`ContaAgendadaService.AvisosAsync`, parte 3), calculados na hora, sem gravar nada. Os limiares são *julgamento próprio*:
  - **"não veio":** cobrada sozinha, com o texto, e 5 dias depois do vencimento ainda sem a baixa;
  - **"mudou de cartão":** a última cobrança com o texto veio em outra conta;
  - **"aumentou" (ou baixou):** a última cobrança passou 5% do valor. Não vale quando as três últimas variam mais de 15% entre si (luz, água);
  - **"cartão arquivado":** a conta a pagar ativa cai numa conta ou num cartão arquivado.
  - **Onde aparecem:** no Painel, só "não veio" e "cartão arquivado" (decisão do usuário); os outros, na tela, com a ação "Atualizar" (o PUT da conta a pagar).

## Consequências

- **Positivas:**
  - Uma lista só para o que se repete (Nielsen 4), com onde cai e como é cobrada escrito na tela (Nielsen 6).
  - O débito automático e a assinatura não precisam de clique para baixar.
  - Nada muda para quem já usava Contas a pagar: tudo começa como "à mão".
- **Negativas:**
  - **O texto da cobrança é estimado** pela palavra mais longa da chave. Uma palavra comum ("PAGAMENTO") ligaria cobranças erradas. A sugestão mostra o texto, que dá para trocar.
  - **Assinatura anual** só é sugerida com as duas cobranças no histórico importado. A primeira carga busca 365 dias, então quase sempre a anual vai ser cadastrada à mão.
  - **±5% no cartão:**
    - uma assinatura que mudou de preço nas três últimas cobranças não é sugerida até as três ficarem iguais;
    - uma assinatura em dólar varia com o câmbio e pode nunca ser sugerida.
  - **Cobrança adiantada em mais de 10 dias** não dá baixa.
- **Na prática:**
  - Coluna nova por `adicionar_coluna_se_ausente`. `FormaCobranca` tem padrão 1, então as contas antigas viram "à mão".
  - Código que mostra "o que vence" deve olhar a forma de cobrança: o que cai no cartão não é conta a pagar à parte.
  - A tela "Assinaturas e recorrentes" (aba de Contas a pagar) e a linha-resumo de Contas e cartões vieram na parte 2. Os avisos vieram na parte 3.
