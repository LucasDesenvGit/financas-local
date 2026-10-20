# 0017. Preferências de exibição ficam no navegador

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois das decisões, nos PRs #29 e #56 e em telas anteriores)
- **Relacionadas:** [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) (parâmetros do sistema no banco)

## Contexto

Várias escolhas só mudam **como a tela aparece**, não o que o sistema faz: tema claro/escuro, paleta e cor personalizada, "ocultar valores", itens por página em Transações, seções "Mais análises" abertas ou fechadas. Era preciso decidir onde guardá-las.

## Alternativas consideradas

- **Na tabela `Configuracao` (banco)** — vale em qualquer navegador; mas uma preferência visual de um aparelho (ex.: tema escuro no celular) passaria para todos, e cada clique viraria uma chamada à API.
- **No `localStorage` do navegador** — instantâneo, sem API, por aparelho.

## Decisão

Preferência de **exibição** fica no `localStorage`, com chaves `financas.*` (`financas.tema`, `financas.paleta`, `financas.cor`, `financas.ocultarValores`, `financas.tamanhoPagina`, `financas.maisAnalises.<tela>`, `financas.menuRecolhido`, `financas.menuGrupos` — os grupos do menu fechados, ADR 0046). Toda leitura e escrita fica em `try/catch`: sem armazenamento (modo privado, bloqueado), a tela funciona com o padrão e só não lembra. O que muda o **comportamento do sistema** vai para a Parametrização no banco (ADR 0015).

## Consequências

- **Positivas:** muda na hora; cada aparelho pode ter a sua aparência; nenhuma chamada à API por preferência.
- **Negativas:**
  - a escolha não acompanha a pessoa entre navegadores/aparelhos (configura de novo em cada um);
  - limpar os dados do navegador apaga as preferências;
  - "ocultar valores" é por navegador: quem ligou no computador e abre no celular vê os valores — é privacidade de tela, não controle de acesso.
- **Na prática:** preferência visual nova usa uma chave `financas.<nome>` com `try/catch` e padrão seguro; nada de dado financeiro ou segredo no `localStorage`. Os testes limpam o `localStorage` entre um caso e outro (`testes/configurar-jest.ts`).
