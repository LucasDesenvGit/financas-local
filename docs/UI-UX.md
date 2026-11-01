# UI/UX: referências e padrões adotados

As fontes de UI/UX do projeto e os padrões que já saíram delas. Vale para quem propõe, revisa ou implementa tela, navegação, gráfico, filtro, formulário ou card de melhoria, seja pessoa ou o Claude. Até a #566 (01/11/2026), este conteúdo morava no `CLAUDE.md`; lá ficou uma linha apontando para cá. O roteiro do código Angular (componentes, testes, revisão) está na skill `angular-no-projeto` (`.claude/skills/`), que aponta para este documento em vez de copiá-lo.

## Referências obrigatórias

Ao propor, revisar ou implementar qualquer coisa de UI/UX (telas, navegação, gráficos, filtros, cards de melhoria), basear a recomendação nestas fontes e **citar qual princípio sustenta cada decisão**:

| Fonte | Usar para | Link |
|---|---|---|
| Shneiderman — *"Overview first, zoom and filter, then details-on-demand"* (*The Eyes Have It*, IEEE Symposium on Visual Languages, 1996) | Gráficos, painéis, drill-down do resumo ao detalhe | [doi:10.1109/VL.1996.545307](https://doi.org/10.1109/VL.1996.545307) |
| Nielsen — 10 heurísticas de usabilidade (NN/g) | Avaliação geral de tela; as mais usadas aqui: 1 *Visibility of System Status*, 2 *Match Between the System and the Real World* (sem jargão), 3 *User Control and Freedom*, 4 *Consistency and Standards*, 6 *Recognition Rather than Recall*, 8 *Aesthetic and Minimalist Design*, 10 *Help and Documentation* | [nngroup.com/articles/ten-usability-heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/) |
| Nielsen — *Progressive Disclosure* (NN/g, 2006) | Telas densas: mostrar primeiro o essencial, o especializado atrás de um controle secundário ("Mais análises", abas) | [nngroup.com/articles/progressive-disclosure](https://www.nngroup.com/articles/progressive-disclosure/) |
| WCAG 2.2 — 1.1.1 *Non-text Content* (A) | Todo gráfico (`<canvas>`) tem alternativa em texto: a frase-resumo do bloco serve para isso | [w3.org/WAI/WCAG22/Understanding/non-text-content](https://www.w3.org/WAI/WCAG22/Understanding/non-text-content) |
| WCAG 2.2 — 2.1.1 *Keyboard* (A) | Toda ação clicável funciona por teclado; clique em `<canvas>` (Chart.js) **nunca** é o único caminho, precisa de equivalente em HTML (`<a>`/`<button>`) | [w3.org/WAI/WCAG22/Understanding/keyboard](https://www.w3.org/WAI/WCAG22/Understanding/keyboard) |
| WCAG 2.2 — 1.4.3 *Contrast (Minimum)* (AA) | Texto ≥ 4,5:1 sobre o fundo (3:1 se grande: 18 pt, ou 14 pt em negrito); cores do tema conferidas em `testes/regressao/contraste-tema.spec.ts` | [w3.org/WAI/WCAG22/Understanding/contrast-minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum) |
| WCAG 2.2 — 2.5.8 *Target Size (Minimum)* (AA) | Alvo de clique/toque ≥ 24×24 CSS px (fatias pequenas de gráfico não atendem) | [w3.org/WAI/WCAG22/Understanding/target-size-minimum](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum) |
| WCAG 2.2 — 1.4.13 *Content on Hover or Focus* (AA) | Tooltips: dispensável, alcançável e persistente; em toque não existe hover | [w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus](https://www.w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus) |
| WCAG 2.2 — 1.4.11 *Non-text Contrast* (AA) | Borda e estados de controles (caixa de marcar, interruptor, foco) com contraste ≥ 3:1 com o que está ao lado | [w3.org/WAI/WCAG22/Understanding/non-text-contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast) |
| Kendrick — *Toggle-Switch Guidelines* (NN/g, 2018) | Interruptor só para efeito **imediato**; o que depende de "Salvar" fica caixa de marcar; rótulo curto, palavra-chave primeiro (teste: ler com "ligado/desligado" no fim) | [nngroup.com/articles/toggle-switch-guidelines](https://www.nngroup.com/articles/toggle-switch-guidelines/) |
| WAI-ARIA APG — *Switch Pattern* | Interruptor: `role="switch"` (no checkbox nativo usa o `checked`), Espaço alterna, rótulo **não muda** com o estado | [w3.org/WAI/ARIA/apg/patterns/switch](https://www.w3.org/WAI/ARIA/apg/patterns/switch/) |
| WAI-ARIA APG — *Disclosure (Show/Hide) Pattern* | Mostrar e esconder um bloco (grupo do menu, seção que abre): o controle é `<button>`, Enter e Espaço alternam, `aria-expanded` diz o estado e `aria-controls` aponta para o conteúdo | [w3.org/WAI/ARIA/apg/patterns/disclosure](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/) |
| Huei-Hsin Wang — *Accordions on Desktop: When and How to Use* (NN/g, 2023) | Sanfona: deixar várias seções abertas ao mesmo tempo (fechar as outras sozinho atrapalha quem usa mais de uma) | [nngroup.com/articles/accordions-on-desktop](https://www.nngroup.com/articles/accordions-on-desktop/) |
| WCAG 2.2 — 3.3.1 *Error Identification* (A) | Erro de entrada detectado: diz qual campo e o que está errado, em texto | [w3.org/WAI/WCAG22/Understanding/error-identification](https://www.w3.org/WAI/WCAG22/Understanding/error-identification) |
| WCAG 2.2 — 3.3.2 *Labels or Instructions* (A) | Todo campo com rótulo visível (o "R$" dentro do campo não substitui o rótulo) | [w3.org/WAI/WCAG22/Understanding/labels-or-instructions](https://www.w3.org/WAI/WCAG22/Understanding/labels-or-instructions) |

## Padrões já adotados

A partir das referências acima (seguir, salvo motivo registrado):
- **Detalhe por navegação, não modal**, quando já existe tela com o detalhe (ex.: total por categoria → `/transacoes` filtrada), levando o contexto completo (categoria + período + tipo) em query params.
- **Filtro ativo sempre visível** (chips com ✕ e "Limpar filtros") em tela que pode abrir já filtrada.
- **Escolher data ou mês sobe de nível pelo título** (#264, #342): todo quadro de escolher data ou mês tem o título como botão que sobe de nível (dia → mês → ano, em páginas de 12 anos), e Esc volta um nível antes de fechar. Vale para o calendário dos campos de data e para o seletor de mês; componente novo de data segue o mesmo e entra em `testes/usabilidade/escolher-data-sobe-nivel.spec.ts` (Nielsen 4 e 7).
- **Estado de tela navegável na URL**: botão voltar do navegador tem que funcionar; atualização de filtro usa `replaceUrl` para não poluir o histórico.
- **Clique em linha ou bloco é atalho de mouse**: a mesma ação tem `<button>`/`<a>` no template (ex.: o nome da transação abre a edição), conferido em `testes/regressao/clique-com-teclado.spec.ts` (WCAG 2.1.1).
- **Gráfico se explica**: título + frase-resumo com a conclusão + linha "como ler" + legenda quando há mais de uma série; resumo respeita "ocultar valores".
- **Gráfico de linha: a principal preenchida até a base, com pontos; a tracejada sem nada** ([ADR 0021](adr/0021-componentes-padrao-para-graficos-e-telas-densas.md), #292). O `<app-grafico>` faz isso sozinho (`linhasNoPadrao`, em `compartilhado/grafico-linhas.ts`): não use `fill: true`, que preenche até o zero e inverte a área com valor negativo. Para fugir do padrão, o conjunto declara `fill: false` ou `pointRadius`.
- **Barras com linha: a linha por cima, com um ponto por mês** ([ADR 0021](adr/0021-componentes-padrao-para-graficos-e-telas-densas.md), #282). O `<app-grafico>` faz isso sozinho (`linhaPorCimaDasBarras`, em `compartilhado/grafico-misto.ts`): não repita `order`/`pointRadius` na tela. Linha de referência (média, meta) declara `pointRadius: 0`. O valor de cada ponto também vai na tabela do bloco.
- **Termo técnico com definição no contexto** (ⓘ que abre por clique/teclado, vindo de um glossário único) — nunca só em `title=`, que não funciona em toque nem teclado.
- **Tela densa mostra primeiro a resposta à pergunta principal dela**; análises de apoio vão para `<app-mais-analises>` (botão com `aria-expanded` que diz o que tem dentro, conteúdo em `<ng-template>` criado só ao abrir, escolha lembrada por tela). Nada é removido: tudo a no máximo um clique.
- **Interruptor** (`label.interruptor` + `role="switch"` no checkbox nativo) para o que muda a tela na hora; caixa de marcar para o que depende de "Salvar" ([ADR 0032](adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)).
- **Menu lateral em grupos** que abrem e fecham, vários abertos ao mesmo tempo, todos abertos no começo; tela nova entra no grupo do assunto dela em `MENU` ([ADR 0046](adr/0046-menu-lateral-em-grupos-que-abrem-e-fecham.md)).
- **Seleção em tabela:** a célula inteira é o alvo (`label.celula-marcar`), a linha marcada fica destacada (`tr.selecionada`), e o nome da caixa diz qual linha é ([ADR 0032](adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)).
- O porquê e o custo aceito de cada padrão acima estão nas ADRs [0017](adr/0017-preferencias-de-exibicao-no-navegador.md) a [0021](adr/0021-componentes-padrao-para-graficos-e-telas-densas.md).

## Ao propor ou revisar

Separar explicitamente o que vem dessas fontes do que é **julgamento próprio** (trade-off sem fonte direta), e dizer quando algo não foi validado com uso real. Fonte nova entra nesta tabela com link verificado, não citada de memória.
