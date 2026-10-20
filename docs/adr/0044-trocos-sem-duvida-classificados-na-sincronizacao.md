# 0044. Trocos "sem dúvida" classificados na sincronização, com aviso e "Desfazer"

- **Status:** Aceita
- **Data:** 2026-10-20
- **Relacionadas:** [0027](0027-conferencia-de-investimentos-com-previa-e-desfazer.md) (em parte substituída por esta), [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (detalhe por navegação), [0023](0023-edicao-em-massa-no-modo-edicao-de-transacoes.md) (prévia e desfazer)

## Contexto

A ADR 0027 decidiu que nada muda sem confirmação: o troco das compras de renda variável (o Nubank debita um pouco a mais e devolve a sobra com a mesma descrição dos proventos) só sai da receita quando o usuário abre "Conferir investimentos" e confirma. O Painel avisa (#170), mas cada compra nova traz trocos novos, e até alguém abrir a conferência os rendimentos do mês ficam maiores que no app da corretora.

O pedido da #173 foi que os "Rendimentos" batam com o app sem precisar rodar a conferência. Ao mesmo tempo, a conferência da carteira real mostrou que, nos dias em que todas as compras têm operação, a conta do dia fecha ao centavo: o que saiu da conta − o troco que voltou − cotas × preço = a taxa da B3 que o app mostra em cada compra (#173 a).

## Alternativas consideradas

- **Continuar só com a conferência (ADR 0027).**
  - Prós: nada muda sem o usuário ver.
  - Contras: os rendimentos ficam errados a cada compra nova até alguém lembrar de conferir.
- **Classificar na sincronização todo crédito que a conferência chama de troco.**
  - Prós: simples.
  - Contras: a ligação crédito × compra é por estimativa (valor até 2,5% do débito, até 3 dias depois). Um provento que cai no mesmo dia pode ser tomado por troco e sumir da receita sem ninguém ver.
- **Uma opção em Configurações para ligar a classificação automática.**
  - Prós: quem não quer, não liga.
  - Contras: mais uma tela e mais uma coisa para lembrar. O aviso com "Desfazer" já deixa a pessoa voltar atrás.
- **Classificar só o troco "sem dúvida", com aviso, "Ver" e "Desfazer" no Painel.**
  - Prós: os rendimentos batem com o app logo depois da sincronização, e o caso duvidoso continua na conferência.
  - Contras: o troco espera 5 dias para entrar, e há uma mudança feita sem confirmação (com aviso).

## Decisão

Depois de cada sincronização (e ao iniciar a API, para valer logo depois da atualização), `ConferenciaInvestimentosService.ClassificarTrocosSemDuvidaAsync` leva para a categoria da compra ("Investimentos"), como escolha manual, o troco que é **sem dúvida**:

- **De um dia de compras em que a conta fecha:** todos os débitos de compra têm operação, o dia tem pelo menos 5 dias (o troco já teve tempo de voltar) e débitos − trocos − custo das cotas (com a taxa já lançada) sobra só a taxa da B3, de zero até o menor de 1% do valor e R$ 5. Um provento tomado por troco quebraria essa conta.
- **Que ainda conta como receita** e cuja categoria **não foi escolhida à mão**. O valor de venda fica de fora, e continua na conferência.

O que mudou fica guardado na `Configuracao` (`Investimentos.TrocosAutomaticos`, até 500 itens) com o estado de antes. O Painel mostra um aviso com:
- **Ver em Transações:** a lista filtrada só com esses lançamentos (`?ids=&de=trocos-automaticos`, ADR 0018);
- **Desfazer:** volta cada crédito que continua como ficou para a categoria de antes, como escolha manual, para a próxima sincronização não classificar de novo o que o usuário disse que não é troco;
- **Ok:** dispensa o aviso, e os trocos ficam.

Um erro na classificação não desfaz a sincronização: o troco continua na conferência.

O resto da ADR 0027 continua valendo: tipo do ativo, compras que faltam, taxa da B3, venda e rendimento contando como investimento só mudam com prévia e confirmação.

## Consequências

- **Positivas:** os rendimentos do mês batem com o app sem abrir a conferência. O usuário vê o que mudou, com um clique para a lista e outro para desfazer.
- **Negativas:**
  - É a primeira mudança em transações feita sem confirmação na renda variável. Quem não olhar o Painel só fica sabendo pelo aviso que espera lá.
  - O troco de uma compra recente continua como receita por até 5 dias (o aviso #170 continua mostrando).
  - O "Desfazer" fixa a categoria de antes como manual: as regras automáticas deixam de mexer nesses créditos (*inferido*: o preço aceito para a sincronização não refazer).
- **Na prática:**
  - O critério "sem dúvida" fica em `AnalisarExtratoAsync` (a mesma conta do dia da taxa da B3). Mudar o critério é mudar esta ADR.
  - Endpoints: `POST /api/carteira/conferencia/trocos-automaticos/desfazer` e `.../dispensar`; o resumo vai no `PainelDto.TrocosAutomaticos`.
