# Testes do front-end

O roteiro de onde cada teste mora e do que rodar. As regras e os números estão em `CONTRIBUINDO.md`, "Testes rápidos", e as pegadinhas do Playwright na skill `testar-no-navegador`; aqui só o caminho.

## Onde vai cada teste

Um comportamento testado num lugar só (#204). Não repita no Jest o que o navegador confere, nem o contrário.

| O que testar | Onde | Forma |
|---|---|---|
| Cálculo, texto, decisão da tela | Jest, `testes/jest/regressao/` (bug corrigido ou comportamento garantido por um PR) | a função pura importada direto, sem `TestBed`: `paginas/movimentacao/limite-cartao.ts` → `testes/jest/regressao/limite-cartao.spec.ts` |
| Critério de UI/UX que dá para conferir sem desenhar (atributo ARIA, teclado, texto, foco, ordem) | Jest, `testes/jest/usabilidade/` | `TestBed` com um componente hospedeiro, como `testes/jest/usabilidade/mais-analises.spec.ts` |
| Desenho, CSS calculado, tamanho de alvo, rota de verdade, CSP, console, celular | navegador, `testes/navegador/` | pelos papéis e nomes acessíveis (`getByRole`, `getByLabel`); estilo em `estilos.spec.ts` |

- **Dados de teste:** as fábricas de `testes/jest/apoio/` (`conta()`, `contas.ts`; uma por assunto). Assunto novo com dados repetidos ganha a sua.
- **Título do teste:** `describe('Regressão: <o que> (#N)')` em `regressao/` e `describe('Usabilidade: ...')` em `usabilidade/`, com o número da issue; o comentário diz a regra que o teste garante.
- **Teste prova que falha:** estrague o código, veja falhar e desfaça (o do CSS, no `CONTRIBUINDO.md`).
- **Padrão com teste próprio** (escolher data ou mês, clique com equivalente de teclado, cor do tema): o componente ou a cor nova entra no teste que `docs/UI-UX.md` cita no padrão.

## Pegadinhas que custam um ciclo da CI

Estão no `CONTRIBUINDO.md`, "Testes rápidos", com o porquê: dependência nova medida antes (e o substituto leve no `moduleNameMapper`), teste novo no navegador pela rota escrita e com `--repeat-each=5`, folga de 0,5 px, esperar a troca antes de ler, arquivo global em `GLOBAIS`. Tela nova em `TELAS` e na `api-falsa.ts`: skill `testar-no-navegador`, seção 3. E a da ADR 0059:

- **Teste que lê o código como texto** (procura no template ou no CSS): pelo `testes/jest/apoio/codigo.ts`, nunca `readFileSync` direto no `.ts`, senão o template em `.html` fica de fora e o teste passa sem olhar (ADR 0059).

## O que rodar, e quando

| Quando | Comando (em `src/frontend`) |
|---|---|
| Durante o trabalho | `npx jest testes/jest/<pasta>/<arquivo>.spec.ts`; `npx playwright test -g <tela>` com o build em `dist/` |
| Antes do push | os comandos de "Antes de dizer que terminou", no `SKILL.md` |
| Mexeu em import, criou arquivo ou pasta | `npm run test:arquitetura` (só local, #476; a CI não roda) |
| Mudou o que se vê | `FOTOS=<rotas> FOTOS_PASTA=<pasta temporária> npx playwright test fotos`, e olhe os PNGs (skill `testar-no-navegador`) |
| Falhou só na CI | `CONTRIBUINDO.md`, "Testes rápidos" |

**Vários agentes ao mesmo tempo:** o build em `dist/` é um só para todos, e o servidor de teste sobe na porta 4300 (`PORTA` troca). Quem chama compila uma vez (`npx ng build --configuration production`); cada agente só roda `PORTA=<4301, 4302…> npx playwright test ...`, com a sua `FOTOS_PASTA`, sem compilar de novo. Dois `ng build` ao mesmo tempo apagam o `dist/` um do outro; dois servidores na mesma porta, um deles não sobe.
