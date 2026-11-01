# Testes do front-end

O roteiro de onde cada teste mora e do que rodar. As regras e os números estão em `CONTRIBUINDO.md`, "Testes rápidos", e as pegadinhas do Playwright na skill `testar-no-navegador`; aqui só o caminho.

## Onde vai cada teste

Um comportamento testado num lugar só (#204). Não repita no Jest o que o navegador confere, nem o contrário.

| O que testar | Onde | Forma |
|---|---|---|
| Cálculo, texto, decisão da tela | Jest, `testes/regressao/` (bug corrigido ou comportamento garantido por um PR) | a função pura importada direto, sem `TestBed`: `paginas/limite-cartao.ts` → `testes/regressao/limite-cartao.spec.ts` |
| Critério de UI/UX que dá para conferir sem desenhar (atributo ARIA, teclado, texto, foco, ordem) | Jest, `testes/usabilidade/` | `TestBed` com um componente hospedeiro, como `testes/usabilidade/mais-analises.spec.ts` |
| Desenho, CSS calculado, tamanho de alvo, rota de verdade, CSP, console, celular | navegador, `testes-navegador/` | pelos papéis e nomes acessíveis (`getByRole`, `getByLabel`); estilo em `estilos.spec.ts` |

- **Dados de teste:** as fábricas de `testes/apoio/` (`conta()`, `contas.ts`; uma por assunto). Assunto novo com dados repetidos ganha a sua.
- **Título do teste:** `describe('Regressão: <o que> (#N)')` em `regressao/` e `describe('Usabilidade: ...')` em `usabilidade/`, com o número da issue; o comentário diz a regra que o teste garante.
- **Teste prova que falha:** estrague o código (ou o CSS do build, em `dist/`), veja falhar e desfaça. Não passou a falhar? Os dados não exercitam o caso.
- **Padrão novo com teste próprio:** escolher data ou mês entra em `testes/usabilidade/escolher-data-sobe-nivel.spec.ts`; clique em linha ou bloco com equivalente de teclado, em `testes/regressao/clique-com-teclado.spec.ts`; cor nova do tema, em `testes/regressao/contraste-tema.spec.ts` (`CLAUDE.md`, padrões adotados).

## Pegadinhas que custam um ciclo da CI

- **Dependência nova no front-end:** meça um teste que a use antes (`npx jest testes/<arquivo>.spec.ts`). Pesou? Substituto leve em `testes/apoio/` e no `moduleNameMapper` do `jest.config.js`, como o `lucide-falso.js`. A trava é de 20 s por arquivo e 150 s a suíte.
- **Tela nova:** linha em `TELAS` (`testes-navegador/base.ts`) e os endpoints em `api-falsa.ts`, senão o teste falha com "chamada da API sem simulação".
- **Teste novo no navegador** abre a tela pela rota escrita (`page.goto('/transacoes')`), senão a escolha dos testes afetados não o acha, e roda `--repeat-each=5` nos dois projetos antes do push.
- **Medida em px:** folga de 0,5 (`toBeGreaterThanOrEqual(23.5)` para 24 px).
- **Depois de um clique que troca o conteúdo:** espere a troca (`not.toHaveText(antes)`) antes de ler.
- **Arquivo novo que vale para todas as telas** fora das pastas globais: entra em `GLOBAIS` no `scripts/ci/testes-afetados.py`.

O porquê de cada uma está no `CONTRIBUINDO.md` e na `testar-no-navegador`.

## O que rodar, e quando

| Quando | Comando (em `src/frontend`) |
|---|---|
| Durante o trabalho | `npx jest testes/<arquivo>.spec.ts`; `npx playwright test -g <tela>` com o build em `dist/` |
| Antes do push | `npm run test:tipos` e `NAVEGADOR=chromium npm run test:afetados` (Chromium do Playwright, o mesmo da CI; `npx playwright install chromium` uma vez) |
| Mexeu em import, criou arquivo ou pasta | `npm run test:arquitetura` (só local, #476; a CI não roda) |
| Mudou o que se vê | `FOTOS=<rotas> FOTOS_PASTA=<pasta temporária> npx playwright test fotos`, e olhe os PNGs (skill `testar-no-navegador`) |
| Falhou só na CI | `gh run download <run> -n testes-navegador`; leia o `error-context.md` e o print antes de mexer |

**Vários agentes ao mesmo tempo:** o build em `dist/` é um só para todos, e o servidor de teste sobe na porta 4300 (`PORTA` troca). Quem chama compila uma vez (`npx ng build --configuration production`); cada agente só roda `PORTA=<4301, 4302…> npx playwright test ...`, com a sua `FOTOS_PASTA`, sem compilar de novo. Dois `ng build` ao mesmo tempo apagam o `dist/` um do outro; dois servidores na mesma porta, um deles não sobe.
