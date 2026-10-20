# 0041. Testes no navegador com Playwright, com a CSP da API e uma API falsa

- **Status:** Aceita
- **Data:** 2026-10-20 (issue #185)
- **Relacionadas:** [0013](0013-testes-do-front-end-com-jest.md) (testes com Jest, que continuam); [0034](0034-sem-sri-no-build-do-front-end.md) (CSP sem script inline); [0004](0004-front-end-publicado-no-wwwroot-da-api.md) (front-end servido pela API)

## Contexto

Os testes do front-end (ADR 0013) rodam com Jest e jsdom: rápidos, mas sem desenhar a tela. Eles não pegam CSS, layout, gráfico (`<canvas>`), animação, erro de console no build de produção nem bloqueio da CSP. Casos reais recentes:

- o erro de CSP do import map (#157) só aparecia no console do navegador, com o build de produção;
- vários PRs ficaram com "falta conferir no navegador" no checklist, porque isso dependia de alguém abrir e olhar;
- a conferência no navegador era improvisada (Edge headless com `--dump-dom`), sem clicar nem navegar.

A issue #185 pediu um jeito de o Claude Code (e qualquer pessoa) conferir sozinho, no navegador, as mudanças do front-end, e testes regressivos que mostrem que nada quebrou.

## Alternativas consideradas

- **Selenium.** Funciona, mas precisa do driver de cada navegador e de código de espera à mão; mais teste instável com o Angular.
- **Cypress.** Bom para testes de ponta a ponta, mas roda o teste dentro do navegador, com o próprio servidor; mais pesado e sem o Edge do sistema tão direto.
- **Servidor MCP do Playwright (`@playwright/mcp`), só para o Claude dirigir o navegador.** Útil para explorar, mas não deixa teste nenhum na CI. Pode vir depois, sem mudar esta decisão.
- **Playwright Test, com o build de produção, a CSP da API e uma API falsa:** escolhida.

## Decisão

Testes no navegador com **`@playwright/test`** (dependência só de desenvolvimento, em `src/frontend`), na pasta `src/frontend/testes-navegador/`:

- **O que é testado:** o **build de produção** (`dist/`), servido por `testes-navegador/servidor.mjs` com a **mesma CSP da API**, lida do `Seguranca.cs` (sem cópia para ficar velha). O servidor se recusa a subir se a CSP lida parecer vazia.
- **Sem back-end:** as chamadas `/api/**` são respondidas pela **API falsa** (`api-falsa.ts`), com dados de exemplo; nenhum dado real é usado. Chamada que a API falsa não conhece **faz o teste falhar**, com o caminho.
- **O que conta como "quebrou"**, em todo teste (fixtures automáticas em `base.ts`): erro no console, exceção, **violação de CSP** (pelo evento `securitypolicyviolation` do navegador) e erro que o site manda para `/api/erros/front-end`.
- **Testes:** cada tela do menu abre (computador e celular), entrada no site, gaveta do menu no celular, nada passando da largura do celular, navegação e voltar do navegador. Mais **prints sob demanda** (`fotos.spec.ts`), em claro e escuro.
- **Navegador:** neste computador, o **Edge instalado** (`channel: msedge`, nada é baixado); na CI, o **Chromium do Playwright** (`npx playwright install --with-deps chromium`).
- **Skill do Claude Code** em `.claude/skills/testar-no-navegador/`: o roteiro de rodar, olhar os prints, acrescentar o teste do que é novo e derrubar tudo no fim.

A eficácia foi provada recolocando de propósito o bug da #157 (SRI ligado): o teste falha com "CSP bloqueou: script-src-elem (inline)". Durante essa prova apareceu um defeito no próprio servidor de teste (a CSP lida vinha vazia, e o teste passava sem testar nada), corrigido junto, com a trava de não subir sem CSP.

## Consequências

- **Positivas:**
  - toda tela do menu é aberta num navegador de verdade a cada PR, com a CSP real: erro de console e bloqueio de CSP aparecem antes do merge;
  - os prints deixam conferir o visual sem subir o site nem usar dados reais;
  - o Jest continua para o que é rápido de testar sem navegador; os dois se completam.
- **Negativas:**
  - **Dependência nova** (`@playwright/test`) e o **download do Chromium na CI** (só lá, de `playwright.azureedge.net`/CDN do Playwright, em tempo de build; o site não ganha saída de rede nenhuma).
  - **Mais tempo de CI:** uns minutos no job `build` (instalar o Chromium e rodar ~40 testes).
  - **A API falsa precisa acompanhar a API:** endpoint novo usado por uma tela exige a resposta em `api-falsa.ts` (o teste avisa). Ela testa o front-end, não o contrato com o back-end: uma mudança no formato da API que a API falsa não acompanhe passa despercebida.
  - **Neste computador depende do Edge**; sem ele, `NAVEGADOR=chrome` ou `npx playwright install chromium`.
- **Na prática:**
  - `npm run test:navegador` (compila e roda) e `npx playwright test` (só roda) em `src/frontend`;
  - tela nova no menu: uma linha em `TELAS` (`base.ts`) e as respostas dela em `api-falsa.ts`;
  - mudou tela ou CSS: rodar a skill (testes + prints) antes de dizer que terminou, e dizer no PR o que foi conferido no navegador e o que não foi.
