# 0057. Confirmação pelo modal do site, e não pelo `confirm()` do navegador

- **Status:** Aceita
- **Data:** 2026-11-04 (issue #631)
- **Relacionadas:** [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) e [0032](0032-controles-padrao-interruptor-e-campo-de-valor.md) (componentes padrão), [0034](0034-sem-sri-no-build-do-front-end.md) (nada de terceiros no front-end)

## Contexto

O site pedia confirmação pela janela nativa do navegador (`window.confirm()`) em 19 lugares de 8 telas: Metas, Transações, Contas, Categorias, Investimentos, Contas a pagar, Configurações e Erros. O usuário pediu, em 04/11/2026, um modal próprio no lugar dela e o "Excluir" no card de cada meta.

**O que a janela nativa não faz:**
- **o título é o endereço do site** ("localhost:5443 diz"), e não o que vai acontecer;
- **os botões são "OK" e "Cancelar"**, sem o verbo da ação, e a ação que apaga não se distingue da que não apaga. Isso vai contra o Nielsen 5 (prevenção de erros) e o NN/g ("OK" é rótulo fraco para ação destrutiva);
- **não segue o tema** (claro ou escuro) nem a fonte do site, contra o Nielsen 4 (consistência);
- **trava a página toda** enquanto está aberta, inclusive os temporizadores;
- **não se testa no Jest** sem espiar o `window`, e no Playwright pede tratar o evento `dialog` à parte.

## Alternativas consideradas

- **Manter o `confirm()`.**
  - Prós: nenhum código.
  - Contras: tudo o que está no contexto.
- **Um modal de confirmação em cada tela**, com o `<app-modal>` e um estado próprio.
  - Prós: nada compartilhado.
  - Contras: 19 cópias do mesmo HTML e do mesmo estado; o rótulo, o foco e o perigo divergem com o tempo (Nielsen 4).
- **Um serviço que devolve uma `Promise<boolean>` e um componente que desenha, uma vez só no `layout`** (escolhida).
  - Prós: a tela troca `confirm(texto)` por `await confirmacao.confirmar({...})`, em uma linha; o desenho, a acessibilidade e os testes ficam num lugar só.
  - Contras: a função que pergunta vira `async`; quem a chama e espera o resultado precisa do `await`.
- **Um diálogo pronto de biblioteca** (Angular CDK Dialog, Material).
  - Contras: dependência nova só para isso, contra a ADR 0034 e o padrão de componentes próprios (ADR 0021 e 0032).

## Decisão

**Usamos o `ConfirmacaoService` e o `<app-confirmacao>`**, em `src/app/compartilhado/`, no lugar de todo `confirm()` do site.

- **O serviço:** `confirmar(pedido)` devolve `Promise<boolean>`, que é `true` só quando a pessoa confirma. O pedido é a pergunta (`mensagem`), e opcionalmente o `titulo`, o verbo do botão (`confirmar`, padrão "Confirmar") e `perigo`, para a ação que apaga ou não tem volta.
- **Um pedido por vez:** um pedido novo com outro aberto responde `false` ao anterior.
- **O componente** usa o `<app-modal>`, com o mesmo empilhamento, Esc e "×", e fica uma vez só no `layout/shell.component.ts`.
- **O padrão "Alert and Message Dialogs" do WAI-ARIA:**
  - `role="alertdialog"` (o `<app-modal>` ganhou a entrada `papel`);
  - a pergunta ligada por `aria-describedby` (a entrada `descricao`);
  - o foco inicial no **Cancelar**, o botão que não faz nada, para um Enter dado sem olhar não apagar.
- **Os botões:**
  - o de confirmar diz o verbo ("Excluir", "Descartar", "Salvar assim mesmo"), e não "OK";
  - sai vermelho (`botao-perigo`) quando `perigo`, e azul (`botao-primario`) no resto;
  - Cancelar, Esc e "×" dão `false` (Nielsen 3, controle e liberdade).

## Consequências

- **Positivas:**
  - a confirmação segue o tema e a fonte do site e diz o que vai acontecer;
  - a ação que apaga se distingue da que não apaga;
  - teclado e leitor de tela seguem o padrão do WAI-ARIA;
  - no Jest, o teste espia `TestBed.inject(ConfirmacaoService).confirmar` (com `mockResolvedValue`), sem tocar no `window`.
- **Negativas:**
  - a função que pergunta vira `async`, e a tela que a chamava esperando o resultado na hora precisa do `await`. Em Transações, o `buscar` continua síncrono quando não há nada pendente: só o caminho que pergunta espera a resposta, para as outras chamadas não mudarem;
  - fora do `layout`, por exemplo na tela de login, não há `<app-confirmacao>`: um pedido feito ali nunca responde. Hoje nenhuma tela fora do `layout` pede confirmação.
- **Na prática:**
  - **não use `confirm()`, `alert()` nem `prompt()`** no front-end: confirmação é `await this.confirmacao.confirmar({ titulo, mensagem, confirmar: '<verbo>', perigo })`;
  - a mensagem diz o objeto da ação ("Excluir a meta “Viagem”?"), e o verbo do botão é o da ação;
  - `perigo: true` vale para o que apaga ou não tem volta;
  - o teste da tela espia o serviço; o desenho e o foco do modal são testados uma vez, em `testes/usabilidade/confirmacao.spec.ts`;
  - num componente que já tenha um campo `confirmacao` (Configurações tem, o da senha), dê outro nome ao serviço (`pedirConfirmacao`).
