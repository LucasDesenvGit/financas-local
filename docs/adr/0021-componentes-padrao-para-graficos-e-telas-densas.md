# 0021. Componentes padrão para gráficos e telas densas

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois das decisões, nos PRs #53 e #56 — UX-9 e UX-11)
- **Relacionadas:** [0017](0017-preferencias-de-exibicao-no-navegador.md) (escolha de abrir/fechar guardada no navegador); referências de UI/UX do `CLAUDE.md`

## Contexto

Os gráficos mostravam dados sem dizer o que concluir nem como ler (cores, eixos, linhas de referência), e o `<canvas>` não tinha alternativa em texto (WCAG 1.1.1). Algumas telas — FIIs com 7 gráficos e 9 blocos, Visão anual com 6 e 8 — mostravam tudo com o mesmo peso, sem ponto de partida (NN/g, Progressive Disclosure; Nielsen 8). Resolver tela a tela, cada uma do seu jeito, deixaria o site inconsistente.

## Alternativas consideradas

- **Cada tela resolve sozinha** — flexível; inconsistente e fácil de esquecer a acessibilidade.
- **Abas em todas as telas densas** — separa bem; esconde demais e muda a navegação de telas já conhecidas.
- **Dois componentes compartilhados que impõem o padrão** — `app-grafico-explicado` e `app-mais-analises`.

## Decisão

- **Todo gráfico** fica dentro de `<app-grafico-explicado [resumo] comoLer>`: a conclusão em uma frase (calculada pela tela e respeitando "Ocultar valores"), o gráfico e a linha "como ler"; resumo + "como ler" viram o `aria-label` da área (`role="img"`). As cores dos gráficos vêm de `cores-graficos.ts` (`CORES_GRAFICO`), com significado fixo (receita, despesa, emergência, financiamento, empréstimo, Selic).
- **Tela densa** mostra primeiro o que responde à pergunta principal dela; o resto vai para `<app-mais-analises chave="tela" conteudo="o que tem dentro"><ng-template>…</ng-template>`, um botão com `aria-expanded` que diz o que está recolhido. O conteúdo vem num `<ng-template>` e **só é criado ao abrir** (gráfico recolhido não é carregado nem desenhado); a escolha fica lembrada por tela (ADR 0017).

## Consequências

- **Positivas:** todo gráfico explica a conclusão e tem alternativa em texto; telas densas começam pelo essencial; menos gráficos desenhados ao abrir FIIs (2 de 7); comportamento coberto pelos testes de usabilidade.
- **Negativas:**
  - cada gráfico novo precisa de uma regra de resumo escrita para ele (mais código por gráfico) — e a frase pode enganar se a regra for ingênua (já aconteceu na comparação anual com poucos meses de dados, corrigido para avisar que a comparação não é confiável);
  - o que fica visível ou recolhido em cada tela é *julgamento próprio*, não validado com uso real; esconder o bloco errado irrita mais do que mostrar demais;
  - conteúdo recolhido não existe no DOM até abrir: busca do navegador (Ctrl+F) não encontra o que está fechado.
- **Na prática:** gráfico novo usa `app-grafico-explicado` com resumo e "como ler", e cor de `CORES_GRAFICO`; bloco de apoio em tela densa vai para o `app-mais-analises` daquela tela, não para uma seção nova sempre aberta.
