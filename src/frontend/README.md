# Front-end (Angular)

As telas do site, em Angular 22. O build de produção vai para o `wwwroot`, ao lado do executável do Go, e fala com a API na mesma origem. O detalhe de cada pasta está em [docs/ARQUITETURA.md, "Front-end"](../../docs/ARQUITETURA.md#front-end--srcfrontend).

| Onde | O que tem |
|---|---|
| `src/app/paginas/` | as telas, uma subpasta por grupo do menu (`movimentacao/`, `planejamento/`, `patrimonio/`...) |
| `src/app/compartilhado/` | o que duas ou mais telas usam: controles, gráficos, glossário |
| `src/app/core/` | o `ApiService` (a única porta para a API), modelos, login, tema |
| `src/app/layout/` | barra superior e menu lateral |
| `testes/jest/`, `testes/navegador/`, `testes/arquitetura/` | Jest, Playwright e o teste de arquitetura (só local) |

**Rodar:** `npm start` (http://localhost:4200, com proxy para a API), `npm run build`, `npm run test:tipos && npm test`, e `NAVEGADOR=chromium npm run test:afetados` antes do push. As regras dos testes estão no [CONTRIBUINDO.md](../../CONTRIBUINDO.md#testes-rápidos-padrões-de-ci).

**Tela nova ou mudança:** comece pela skill `angular-no-projeto` (`.claude/skills/`) e pela [ADR 0059](../../docs/adr/0059-paginas-grandes-divididas-em-tela-apresentacao-e-funcoes-puras.md) (tela, componentes de apresentação e funções puras). Mudou o que o usuário vê? O Manual (`paginas/manual/manual-conteudo.ts`) muda no mesmo PR.

**Não mexer:** `dist/` e `node_modules/` são gerados. Componente novo leva `changeDetection: ChangeDetectionStrategy.Eager`, e nenhum componente usa o `HttpClient` direto.
