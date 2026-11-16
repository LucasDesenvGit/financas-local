# Avaliação de tela e revisão de código Angular

Use ao avaliar uma tela, ao revisar um PR com front-end e no fim do seu próprio trabalho. Cada achado sai com `arquivo:linha` (ou o print), o que dá errado (quem usa faz X → acontece Y) e a fonte: o princípio de `docs/UI-UX.md`, a ADR ou a regra. O que for gosto, sem fonte, diga que é julgamento.

## O que a CI já pega (não gaste a revisão nisso)

`npm run test:tipos`, Jest, tempo dos testes, build, telas do menu sem erro de console, exceção ou bloqueio de CSP (computador e celular), o arquivo de `changelog.d/` em `feat` e `fix`, aviso do Manual (job "manual do site e changelog.d"), seção do Manual por tela (`manual.spec.ts`), contraste das cores do tema (`contraste-tema.spec.ts`), CodeQL. Se um destes falhou, a revisão aponta o job; não reexplica a regra.

## Avaliar uma tela (UI/UX)

Abra a tela pelos prints (skill `testar-no-navegador`, seção 2: claro e escuro, computador e celular) **e** pelo código. O critério de cada item (número, limite, padrão) está em `docs/UI-UX.md`; aqui só a ordem, do mais grave ao menos:

1. **A pergunta principal da tela** respondida no topo, o resto atrás de `<app-mais-analises>` (Shneiderman, Progressive Disclosure).
2. **Teclado:** clique em linha, bloco ou `<canvas>` nunca é o único caminho (WCAG 2.1.1).
3. **Gráfico se explica**, e o resumo respeita "ocultar valores" (WCAG 1.1.1).
4. **Campos:** rótulo visível e erro que diz o campo (WCAG 3.3.2 e 3.3.1).
5. **Alvo e contraste** (WCAG 2.5.8, 1.4.3 e 1.4.11), inclusive cor fixa fora do tema, que o teste do tema não pega.
6. **Dica e balão:** nada só em `title=` (WCAG 1.4.13).
7. **Termos:** jargão com `<app-termo>` ou trocado por palavra de quem usa (Nielsen 2).
8. **Estado:** carregando, vazio e erro têm texto (Nielsen 1); filtro ativo em chips; voltar do navegador funciona (ADR 0018).
9. **Consistência:** o controle de `referencias/componentes.md`, interruptor só para efeito imediato, a mesma cor para receita e despesa (Nielsen 4).
10. **Celular:** a skill `revisar-no-celular`.

Separe o que viu no print do que leu no código, e diga o que não deu para ver com a API falsa (dados reais, volume, telas que dependem do back-end de verdade).

## Revisar o código

**Forma**
- [ ] Componente novo com `ChangeDetectionStrategy.Eager` (ou OnPush com **todo** o estado em signal). Sem ele, a tela para de atualizar sem erro: a CI não pega.
- [ ] HTTP só pelo `ApiService`; erro mostrado com `mensagemDeErro`.
- [ ] `npm run test:arquitetura` passa. **É só local**: rode você; a CI não roda. Exceção nova em `EXCECOES_DE_PASTA` tem um motivo que convence.
- [ ] Cálculo fora do componente, em função pura testável; o template sem lógica que mereça teste.
- [ ] Usou o componente de `compartilhado/` que já existe, em vez de refazer (`referencias/componentes.md`).
- [ ] `MoedaPipe` e não o `CurrencyPipe` do Angular; nada de `NgClass`/`NgStyle`, `*ngIf`, `@Input` novo.
- [ ] Preferência de exibição no `localStorage` com chave `financas.*` e `try`/`catch` (ADR 0017); dado que o usuário salva vai para a API.
- [ ] Nada carregado de fora (fonte, script, imagem): a CSP barra e a ADR 0004 proíbe.

**Testes**
- [ ] Bug corrigido com teste em `testes/jest/regressao/` que falhava antes.
- [ ] Cada comportamento num lugar só: lógica no Jest, desenho no navegador.
- [ ] Tela nova em `TELAS` e endpoint novo na `api-falsa.ts`; teste novo no navegador com `--repeat-each=5` dito no PR.

**Junto, no mesmo PR** (`CLAUDE.md`)
- [ ] Manual: a seção da tela, os passos e os termos; tela nova em `AJUDA_POR_ROTA`. Ou "Manual: não se aplica".
- [ ] Tela nova no grupo certo do `MENU` (ADR 0046).
- [ ] Cada decisão de desenho no PR cita o princípio de UI/UX, separando fonte de julgamento, e diz o que não foi validado com uso real.
- [ ] Padrão novo de UI: na lista de padrões de `docs/UI-UX.md` e, se mudar decisão, ADR.
- [ ] Termo novo do domínio em `docs/arquitetura/dominio.md`; dado pessoal novo em `docs/DADOS-PESSOAIS.md`.
