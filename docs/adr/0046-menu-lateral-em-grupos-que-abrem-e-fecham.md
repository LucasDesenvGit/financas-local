# 0046. Menu lateral em grupos que abrem e fecham (sanfona)

- **Status:** Aceita
- **Data:** 2026-10-20
- **Relacionadas:** [0017](0017-preferencias-de-exibicao-no-navegador.md) (preferências de exibição no navegador), [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) (telas densas: o essencial primeiro, o resto a um clique)

## Contexto

O menu lateral era uma lista corrida: 13 telas (mais o Manual), sem divisão. Vêm mais por aí: Trabalho (#209: salário, holerites, férias, 13º, rescisão, FGTS) e os módulos que se ligam e desligam (#190). Do jeito que estava, o menu viraria uma lista longa, com rolagem, onde é difícil achar a tela.

O roteiro de primeiro uso (#193) já divide o site em módulos com nome: Início, Movimentação, Planejamento, Patrimônio, Trabalho, Análises e Ajustes. O menu continua tendo de funcionar no computador (menu fixo que o ☰ esconde) e no celular (gaveta), como na #131.

Fontes de UI/UX usadas ([docs/UI-UX.md](../UI-UX.md); até a #566, no `CLAUDE.md`):
- Nielsen, *Progressive Disclosure*, e heurística 8 (*Aesthetic and Minimalist Design*): mostrar primeiro o essencial, o resto a um clique.
- Heurística 6 (*Recognition Rather than Recall*): o nome do grupo leva à tela pelo assunto, sem decorar a posição.
- Heurística 4 (*Consistency and Standards*): os mesmos nomes no menu, no roteiro e no Manual.
- WAI-ARIA APG, *Disclosure (Show/Hide) Pattern*: o botão que mostra e esconde, com `aria-expanded` e `aria-controls`.
- Huei-Hsin Wang, *Accordions on Desktop* (NN/g, 2023): sanfona que fecha sozinha atrapalha quem usa mais de uma seção.

## Alternativas consideradas

- **Lista corrida, como antes.**
  - Prós: nada a aprender; tudo à vista.
  - Contras: cresce a cada tela nova e passa da altura da tela; o assunto de cada tela não aparece.
- **Títulos de seção fixos, sem abrir e fechar.**
  - Prós: simples; ajuda a achar pelo assunto.
  - Contras: o menu fica ainda mais alto (cada título é uma linha a mais); quem só usa Transações e Orçamentos continua vendo tudo.
- **Sanfona exclusiva (abrir um grupo fecha o outro).**
  - Prós: o menu nunca passa da altura da tela.
  - Contras: quem pula entre Transações e Relatórios abre e fecha grupos o tempo todo. A NN/g desaconselha (*Accordions on Desktop*).
- **Menu de dois níveis (só ícones dos grupos; as telas num painel ao lado).**
  - Prós: menu curto.
  - Contras: dois cliques para toda tela; não cabe bem na gaveta do celular; muda demais o que a pessoa já conhece.
- **Sanfona com vários grupos abertos ao mesmo tempo, todos abertos no começo.**
  - Prós: ninguém "perde" uma tela que já conhecia; quem quer um menu curto fecha o que não usa; abre espaço para Trabalho e os módulos.
  - Contras: um nível a mais no menu; com tudo aberto, o menu fica um pouco mais alto que antes (os cabeçalhos).

## Decisão

O menu lateral é dividido em **grupos que abrem e fecham**, com **vários abertos ao mesmo tempo**. Os grupos são os módulos do roteiro (#193), com os mesmos nomes, a mesma ordem e as mesmas telas:

| Grupo | Telas |
|---|---|
| *(solto, no topo)* | Painel |
| Movimentação | Transações · Contas e cartões · Contas a pagar |
| Planejamento | Orçamentos · Metas |
| Patrimônio | Investimentos · Empréstimos e financiamentos |
| Análises | Relatórios · Visão anual · Projeção |
| Ajustes | Categorias e regras · Configurações |
| *(solto, no rodapé)* | Manual |

Trabalho (#209) entra entre Patrimônio e Análises quando existir.

Como funciona:
- **Cabeçalho do grupo é um `<button>`** com ícone, nome e seta, `aria-expanded` e `aria-controls` (APG, *Disclosure*). As telas continuam `<a routerLink>`.
- **Padrão: todos abertos.** A escolha fica no navegador, em `financas.menuGrupos` (ADR 0017), como a **lista dos grupos fechados**. Assim, um grupo novo aparece aberto até para quem já tinha fechado outros.
- **O grupo da tela aberta abre sozinho** ao entrar nela (menu, link, voltar, URL). Trocar só o filtro da mesma tela não reabre o que a pessoa fechou. O grupo aberto assim também fica salvo: o menu volta como estava na última vez *(inferido)*. Com o grupo fechado, o cabeçalho fica marcado: cor da tela ativa, um ponto e o texto "tela atual está aqui" para o leitor de tela (Nielsen 1).
- **Grupo fechado** fica fora do Tab e do leitor de tela (`inert` e `visibility: hidden`), sem sair da página, para abrir e fechar deslizando. Com `prefers-reduced-motion`, sem animação.
- **Celular:** tocar no grupo não fecha a gaveta; tocar numa tela fecha. **Computador:** o ☰ (#131) continua escondendo o menu inteiro, e os grupos não mudam.

## Consequências

- **Positivas:**
  - o menu tem lugar para Trabalho e os módulos sem virar uma lista longa;
  - quem quer um menu curto fecha o que não usa, e a escolha fica salva;
  - menu, roteiro e Manual falam a mesma língua.
- **Negativas:**
  - um nível a mais: com o grupo fechado, a tela fica a um clique a mais;
  - com tudo aberto (o padrão), o menu é mais alto que antes (5 cabeçalhos) e, em tela baixa, o Manual só aparece rolando o menu;
  - a escolha é por navegador (ADR 0017): no outro aparelho, o menu volta todo aberto;
  - mudar o nome ou as telas de um grupo exige mudar também o roteiro (#193) e o Manual;
  - os prints do Manual que mostram o menu (#197) precisam ser feitos depois desta mudança, ou refeitos.
- **Na prática:**
  - **Tela nova** entra em `MENU` (`src/app/layout/shell.component.ts`) dentro do grupo do assunto dela, e ganha seção no Manual (o teste `manual.spec.ts` percorre as telas dentro dos grupos).
  - **Grupo novo** só quando nenhum grupo serve e ele junta pelo menos duas telas. Tela que é sozinha um assunto e serve de entrada (Painel) ou ajuda de tudo (Manual) fica solta. Grupo novo entra também no roteiro (#193) e no Manual, com o mesmo nome. *(inferido: o mínimo de duas telas não foi confirmado com o usuário.)*
  - O grupo não tem o mesmo nome de uma tela dele (por isso "Patrimônio", e não "Investimentos").
  - No código, o grupo do menu se chama `grupo`, nunca `modulo`: "módulo" é a funcionalidade que se liga e desliga (#190).
  - **Módulos (#190):** a tela de módulo desligado sai do menu pelo `telaVisivel` do `ShellComponent` (`filtrarMenu`). O grupo que fica sem tela some.
  - **Roteiro (#194):** os elementos do menu têm `data-roteiro` estável (`menu`, `menu-grupo-<id>`, `menu-tela-<rota>`), e `mostrarTelaNoMenu(rota)` abre o grupo da tela e, no celular, a gaveta, antes de destacar.
  - *Não validado com uso real:* a divisão dos grupos (a mesma do #193) e o padrão "todos abertos" podem mudar depois de usar.
