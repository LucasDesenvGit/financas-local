# 0018. Estado da tela na URL e detalhe por navegação

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois das decisões, nos PRs #44, #45, #47, #48, #49 e #57 — UX-1 a UX-6)
- **Relacionadas:** épico de UX #30; referências de UI/UX do `CLAUDE.md`

## Contexto

Os resumos (Painel, Orçamentos, Visão anual, Categorias) mostravam totais por categoria sem caminho para as transações por trás deles; filtros de Transações se perdiam no F5, e não dava para salvar ou mandar um link de uma lista filtrada. Shneiderman (*overview first … details on demand*) e Nielsen 3 e 4 apontam para o mesmo lugar: do resumo ao detalhe com um clique, e o navegador (voltar, F5, favorito) funcionando como em qualquer site.

## Alternativas consideradas

- **Modal com o detalhe em cada tela de resumo** — não sai da tela; mas duplica a lista de transações em vários lugares, não tem link e o voltar do navegador não fecha o modal.
- **Estado em serviço/memória do Angular** — simples; perde tudo no F5 e não gera link.
- **Estado em query params da URL, com detalhe em `/transacoes`** — uma lista só, com link, F5 e voltar funcionando.

## Decisão

- **Detalhe por navegação:** totais por categoria (gráficos, listas, orçamentos, categorias) levam a `/transacoes` filtrada, com o contexto completo — categoria, período, tipo e conta — montado **sempre** por `paramsTransacoes()` (`compartilhado/utilidades.ts`), nunca string na mão. Clique em gráfico (canvas) tem equivalente em `<a>`/`<button>` (WCAG 2.1.1).
- **Estado na URL:** Transações lê e grava o filtro em query params (`categoria` — uma ou várias, separadas por vírgula, cada valor validado sozinho —, `conta`, `tipo`, `inicio`, `fim`, `emergencias`, `busca`, `pagina`), só o que difere do padrão. Cada parâmetro é validado sozinho; inválido é ignorado e a URL se corrige. Atualização de filtro usa `replaceUrl`, para o voltar levar à tela anterior e não ao filtro anterior (*julgamento próprio*: é a convenção mais comum, mas há apps que empilham). Busca textual vai para a URL só no Enter.
- **Filtro ativo sempre visível:** tela que pode abrir filtrada mostra chips com ✕ e "Limpar filtros" (Nielsen 1 e 3).

## Consequências

- **Positivas:** uma lista de transações só; link compartilhável; F5 e voltar funcionam; testado em `testes/regressao/transacoes-filtros-url.spec.ts` e `testes/usabilidade/filtros-ativos-transacoes.spec.ts`.
- **Negativas:**
  - o nome dos parâmetros vira contrato público: link salvo quebra se um nome mudar — renomear exige aceitar o nome antigo por um tempo;
  - parâmetro inválido é descartado em silêncio (só a URL se corrige): quem colou um link errado não recebe mensagem;
  - com `replaceUrl`, não dá para "voltar" ao filtro anterior pelo navegador.
- **Na prática:** link para Transações usa `paramsTransacoes()`; filtro novo em Transações precisa de leitura validada em `aplicarParametrosDaUrl`, gravação em `gravarFiltroNaUrl`, chip em `chipsFiltro` e teste de regressão.
