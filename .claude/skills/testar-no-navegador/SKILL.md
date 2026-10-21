---
name: testar-no-navegador
description: Confere no navegador de verdade uma mudança do front-end do Finanças Local (Playwright, ADR 0041) — roda a regressão de todas as telas com a CSP da API, tira prints das telas mudadas (claro/escuro, computador/celular) e olha, e acrescenta o teste do que é novo. Usar depois de mudar tela, CSS, rota ou chamada da API no front-end, antes de dizer que terminou, e quando pedirem "confere no navegador", "testa a tela" ou "tira print".
---

# Testar no navegador (Finanças Local)

Os testes do Jest (`src/frontend/testes/`) não desenham a tela. Esta skill fecha essa lacuna com o Playwright: o **build de produção**, servido com a **mesma CSP da API** (lida do `Seguranca.cs`), num navegador de verdade, com a **API falsa** (`testes-navegador/api-falsa.ts`). Não precisa de back-end nem de banco, e **nenhum dado real** aparece.

Tudo roda em `src/frontend`, com o Node portátil no PATH quando o do sistema não for o do projeto:

```bash
export PATH="$(pwd)/../../ferramentas/node:$PATH"   # a partir de src/frontend, se preciso
```

## 1. Regressão: todas as telas

```bash
npm run test:navegador          # compila (ng build) e roda tudo: computador e celular
npx playwright test             # só roda, se o build de agora já está em dist/
npx playwright test -g metas    # só os testes com "metas" no nome
```

Cada tela do menu tem de abrir **sem**: erro no console, exceção, bloqueio da CSP, erro mandado para `/api/erros/front-end` e chamada da API sem simulação. Neste computador usa o Edge instalado (`NAVEGADOR=chrome` troca); na CI, o Chromium do Playwright.

**Antes de subir (push), rode também com o navegador da CI** (#264: passou no Edge e falhou na CI, só no celular):

```bash
npx playwright install chromium                         # uma vez
NAVEGADOR=chromium npm run test:afetados                # antes do push: só o que o PR afeta (Jest + build + navegador)
NAVEGADOR=chromium npx playwright test <arquivo> --repeat-each=5   # teste novo: 5 vezes, computador e celular
NAVEGADOR=chromium npx playwright test                  # tudo, com o Chromium da CI (quando quiser a suíte inteira)
```

`npm run test:afetados` (#286) faz a mesma escolha da CI no PR (ADR 0047), contando também o que ainda não foi commitado:
- só os testes ligados ao que mudou desde o `develop`;
- tudo, se mudou arquivo global (estilos, `core/`, configuração);
- nada, se o front-end não mudou.

Numa mudança de uma tela, leva uns 45 s, contra uns 2,7 min da suíte inteira. A suíte inteira continua rodando na CI, no merge. `python3 ../../scripts/ci/testes-afetados.py --base origin/develop` só mostra o que rodaria.

Falhou só na CI? `gh run download <run> -n testes-navegador` e olhe o print e o `error-context.md` antes de mexer; não suba tentativas para ver se passa.

Para simular a CI mais lenta: `--workers=8 --repeat-each=10 --retries=0`. Medida em px com folga de 0,5 (`toBeGreaterThanOrEqual(23.5)` para 24 px: no Linux da CI, 24 vira 23,99997); depois de um clique que troca o conteúdo, espere a troca (`not.toHaveText(antes)`) antes de ler.

**Falhou?** Leia `testes-navegador/resultados/<teste>/error-context.md` (a árvore da página no momento da falha) e o `test-failed-1.png` antes de mexer. Não "conserte" o teste para passar: descubra se a tela quebrou.

- **"chamadas da API sem simulação"**: a tela chama um endpoint novo. Acrescente a resposta em `api-falsa.ts`, com o formato do modelo em `src/app/core/modelos.ts`.
- **"CSP bloqueou"**: algo inline ou de fora entrou no `index.html` ou na tela (veja a #157 e a ADR 0034). Corrija a causa; nunca afrouxe a CSP para o teste passar.

## 2. Olhar a tela mudada (prints)

```bash
FOTOS=painel,metas,investimentos?aba=fii FOTOS_PASTA=<pasta temporária da sessão>/fotos npx playwright test fotos
```

- Rotas **sem a barra** do começo (o Git Bash troca `/painel` por um caminho do Windows).
- Sai um PNG por rota × tema (claro, escuro) × projeto (computador 1366×860, celular Pixel 7).
- **Abra os PNGs com a ferramenta de ler arquivo e olhe de verdade**: texto cortado, sobreposição, contraste, alvo pequeno, rolagem para o lado no celular, o que mudou no PR.
- O menu lateral "acabando" no meio do print de página inteira é efeito do print (ele é fixo), não defeito.

## 3. Mudou ou criou tela? Acrescente o teste

- **Tela nova no menu**: uma linha em `TELAS` (`testes-navegador/base.ts`) e as respostas dos endpoints dela em `api-falsa.ts`.
- **Interação importante** (botão, filtro, navegação): um teste em `testes-navegador/*.spec.ts` que faz o que o usuário faz, pelos papéis e nomes acessíveis (`getByRole`, `getByLabel`), como em `celular.spec.ts`. Se o nome acessível não acha o controle, isso já é um problema de acessibilidade a corrigir na tela.
- Todo teste já liga a API falsa e confere os erros sozinho (fixtures `auto` em `base.ts`).
- Prove que o teste pega o defeito: estrague de propósito, veja falhar, desfaça.

## 4. No fim: nada rodando

O `playwright test` sobe e derruba o servidor de teste sozinho. Se você subiu algo à mão (`npm start`, `node testes-navegador/servidor.mjs`, a API), **encerre também os processos filhos**: parar só a tarefa deixa `node.exe`/`esbuild.exe` órfãos, e um `esbuild.exe` preso faz o `npm ci` do instalador falhar com EPERM. No Windows, procure por linha de comando que contenha `financas-local` e `ng.js serve`, `servidor.mjs` ou `esbuild.exe`.

## 5. Dizer o que foi conferido

No PR e na resposta, separe:
- **conferido no navegador**: os testes que passaram, os prints que você olhou e o que viu neles;
- **não conferido**: o que depende de dados reais, do back-end de verdade, do instalador ou do gerenciador (fora do alcance da API falsa).
