# 0013. Testes automatizados do front-end com Jest

- **Status:** Aceita
- **Data:** 2026-10-16
- **Relacionadas:** [0004](0004-front-end-publicado-no-wwwroot-da-api.md) (front-end Angular); [0041](0041-testes-no-navegador-com-playwright.md) (testes no navegador, que completam estes com o que o jsdom não desenha)

## Contexto

Até aqui o projeto não tinha nenhum teste automatizado: a validação era build + checklist manual do PR. O épico de UX (#30) trouxe comportamentos fáceis de quebrar sem perceber — filtros de Transações lidos e gravados na URL, balão do glossário que não pode sair da tela, conteúdo recolhido que não pode ser criado antes de abrir, atributos de acessibilidade — e um bug já tinha aparecido e sido corrigido (balão cortado no celular). O usuário pediu testes de **regressão** e de **usabilidade** usando **Jest**.

## Alternativas consideradas

- **Karma + Jasmine (padrão antigo do Angular CLI)** — integrado ao `ng test`; mas o Karma foi descontinuado pelo time do Angular e roda num navegador de verdade (mais lento, precisa de Chrome no CI).
- **Builder experimental de Jest do Angular CLI (`@angular-devkit/build-angular:jest`)** — sem configuração extra; mas experimental e deixado de lado pelo Angular em favor do Vitest.
- **Vitest** — caminho que o Angular está adotando; mas não foi o pedido, e no Angular 20 o suporte ainda é experimental.
- **Jest + `jest-preset-angular`** — maduro, roda em Node com jsdom (rápido, sem navegador), suporta Angular 20 (preset 17.x) e componentes standalone. Escolhido pelo usuário.

## Decisão

Os testes do front-end usam **Jest 30 + `jest-preset-angular` 17**, com `jsdom`, fora do build do Angular:

- ficam em `src/frontend/testes/`, separados por objetivo: `regressao/` (comportamento que já quebrou ou que um PR garantiu) e `usabilidade/` (critérios de UI/UX do CLAUDE.md — Nielsen, WCAG 2.2 — que dá para verificar sem navegador: nome/estado acessível, teclado, Esc, conteúdo carregado só sob demanda, filtro ativo visível); `apoio/` tem o que os testes compartilham;
- `npm test` roda tudo; `npm run test:regressao` e `npm run test:usabilidade` rodam cada grupo; `npm run test:tipos` confere os tipos dos testes (o ts-jest só transpila);
- a CI roda `test:tipos` e `npm test` logo depois do build do front-end.

## Consequências

- **Positivas:** os comportamentos das UX-1 a UX-11 têm rede de proteção (84 testes, ~5 s); cada teste diz qual issue ou princípio protege; uma mudança que quebre, por exemplo, o `replaceUrl` dos filtros ou o "conteúdo só ao abrir" falha na CI (conferido quebrando o código de propósito).
- **Negativas:**
  - 5 dependências de desenvolvimento novas — `jest`, `jest-environment-jsdom`, `jest-preset-angular`, `@types/jest` e `@types/node` (para testes que leem arquivos, como o `styles.css`) — e ~370 pacotes no `package-lock.json`, que não vão para o bundle publicado mas aumentam o `npm ci`;
  - o jsdom não calcula layout nem implementa a Popover API: tamanho de alvo (24×24), contraste e posição real ficam fora — a posição do balão é testada com `getBoundingClientRect` simulado, e a Popover API com um substituto mínimo (`testes/apoio/popover.ts`);
  - se o Angular padronizar o Vitest, migrar os testes será trabalho extra (a API do Vitest é quase igual à do Jest);
  - o back-end (.NET) continua sem testes automatizados.
- **Na prática:** bug corrigido ganha teste em `testes/regressao/` no mesmo PR; padrão novo de UI/UX do CLAUDE.md ganha teste em `testes/usabilidade/`. Telas que usam o roteador são abertas com `RouterTestingHarness` (veja `testes/apoio/transacoes.ts`), sempre com a API falsa (nunca HTTP de verdade). O `import` do `jest-preset-angular/setup-env/zone` tem de ser o primeiro do `testes/configurar-jest.ts`.
