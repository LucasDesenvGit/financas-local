# 0020. Glossário único, com o balão na top layer (Popover API)

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois da decisão, no PR #55 — UX-10)
- **Relacionadas:** referências de UI/UX do `CLAUDE.md`

## Contexto

Telas de investimentos, empréstimos e da Visão anual usam termos técnicos (DY, yield on cost, CET, Selic, preço médio, regra 50/30/20). Algumas explicações estavam em `title=`, que não aparece em toque nem no teclado (WCAG 1.4.13), e o mesmo termo podia ser explicado de jeitos diferentes em telas diferentes. Na primeira versão, o balão com a definição, em `position: fixed`, aparecia cortado dentro de tabelas com rolagem e deslocado em cartões com `transform` (bug encontrado no teste em celular).

## Alternativas consideradas

- **`title=` nos termos** — zero código; não funciona em toque nem teclado.
- **Página de glossário separada** — ajuda completa; tira a pessoa do contexto (Nielsen 10 pede ajuda no ponto de uso).
- **Biblioteca de tooltip/popover (ex.: CDK Overlay)** — resolve posicionamento; dependência nova para um componente pequeno.
- **Componente próprio + Popover API nativa (`popover="manual"`)** — o balão vai para a top layer do navegador, acima de qualquer `overflow`/`transform`, sem dependência.

## Decisão

As definições ficam num **glossário único** (`compartilhado/glossario.ts`, 1 a 2 frases e exemplo quando ajuda), usado pelo componente `<app-termo chave="...">`: o texto do termo continua normal e um ⓘ (botão de 24×24 px) abre a definição por clique, toque ou teclado; Esc e clique fora fecham sem mover o foco. O balão usa a **Popover API** do navegador (top layer) e é reposicionado para encostar na margem quando não cabe ao lado do ⓘ.

## Consequências

- **Positivas:** o mesmo termo tem a mesma definição em toda tela; acessível por teclado e toque; o balão não é cortado por tabelas nem cartões; sem dependência nova.
- **Negativas:**
  - exige navegador com Popover API (Chrome/Edge 114+, Firefox 125+, Safari 17+, de 2023–2024); em navegador mais antigo o ⓘ não abre o balão;
  - o balão fecha ao rolar a página (posição fixa não acompanha a rolagem) — *julgamento próprio*, para não ficar solto na tela;
  - o jsdom dos testes não tem Popover API: os testes usam um substituto mínimo (`testes/apoio/popover.ts`) e simulam o layout.
- **Na prática:** termo técnico novo entra em `glossario.ts` e é marcado com `<app-termo>`; nunca explicar só com `title=`. Testes em `testes/usabilidade/glossario-termo.spec.ts` e `testes/regressao/termo-balao.spec.ts`.
