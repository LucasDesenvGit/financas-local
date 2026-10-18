# 0026. Manual do site como dados no front-end, com testes que o mantêm em dia

- **Status:** Aceita
- **Data:** 2026-10-17 (issue #113)
- **Relacionadas:** [0013](0013-testes-do-front-end-com-jest.md) (Jest), [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (estado na URL), [0020](0020-glossario-unico-com-balao-na-top-layer.md) (glossário único)

## Contexto

O site cresceu:
- Open Finance, cartões e faturas;
- categorias, regras e modo edição;
- orçamentos sugeridos, assistente de metas e investimentos.

A ajuda estava espalhada no README e nos ⓘ do glossário. A issue #113 pediu uma página **Manual**, com:
- passo a passo de cada parte;
- link que abre a tela certa;
- "Como usar esta tela" em cada página;
- busca.

Ela deixou em aberto o formato do conteúdo. O risco principal de um manual é **ficar velho**: a tela muda e o texto não.

## Alternativas consideradas

- **Markdown no build.**
  - Prós: fácil de escrever.
  - Contras:
    - precisa de um renderizador (dependência nova);
    - os links e âncoras viram texto solto, que nenhum teste confere.
- **Um componente Angular por capítulo.**
  - Prós: liberdade total de layout.
  - Contras: 14 templates com formatos que tendem a divergir; a busca teria que ler o DOM.
- **Prints de tela.**
  - Prós: mostram o caminho.
  - Contras:
    - ficam velhos a cada ajuste visual;
    - pesam no build;
    - cada um precisa de texto alternativo.
- **Conteúdo como dados tipados (TypeScript), desenhado por um componente só.**
  - Prós:
    - formato igual em todas as seções;
    - busca simples;
    - os testes conferem rotas, âncoras e termos.
  - Contras: escrever texto dentro de código.

## Decisão

O manual é um arquivo de dados, `paginas/manual-conteudo.ts`:
- **capítulos → seções**, cada seção com: para que serve, passos, links "Abrir a tela" (rota + parâmetros), dicas, erros comuns e termos do glossário;
- desenhado pelo `ManualComponent`.

**Endereços:**
- `/manual` mostra o índice e a busca (`?busca=` na URL, com `replaceUrl`);
- `/manual/<capitulo>#<secao>` mostra o capítulo e rola até a seção (`withInMemoryScrolling`).

**Ajuda em cada tela:** o botão "Ajuda" na barra de cima abre a seção da tela aberta (`AJUDA_POR_ROTA` / `ajudaDaUrl`). Fica num lugar só, na barra, em vez de repetido em cada página.

**Sem prints nesta versão:** o "Abrir a tela" leva à tela de verdade, já filtrada quando faz sentido.

**Testes de regressão:**
- toda tela do menu (`MENU`, exportado do shell) tem capítulo;
- todo "Abrir a tela" aponta para uma rota existente;
- as âncoras são únicas;
- os termos existem no glossário.

## Consequências

- **Positivas:**
  - ajuda no contexto (Nielsen 10);
  - formato padronizado (Nielsen 4);
  - links que não quebram em silêncio;
  - busca sem dependência;
  - o glossário é reaproveitado.
- **Negativas:**
  - O texto dos passos não é verificado por teste. Se um botão mudar de nome, o manual só fica certo se alguém atualizar. Isso entra no checklist do PR.
  - Sem imagens: quem aprende vendo depende do "Abrir a tela".
  - O conteúdo fica no bundle da página do manual (carregada só quando abre).
- **Na prática:**
  - **Tela nova no menu:** entra em `MENU` e em `AJUDA_POR_ROTA`, com o capítulo ou a seção no `manual-conteudo.ts` (o teste cobra).
  - **Mudou o jeito de usar uma tela:** atualize a seção. Nomes de botões vão entre aspas curvas, exatamente como na tela.
