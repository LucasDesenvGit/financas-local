# 0071. Manual do site com prints gerados das telas: Playwright e API falsa, WebP, um por tema, guardados no repositório

- **Status:** Aceita
- **Data:** 2026-11-27 (issue #752; aval do Felipe em 26/11/2026; anda junto com a #733, que encurta o texto do Manual)
- **Relacionadas:** substitui em parte a [0026](0026-manual-do-site-como-dados-no-front-end.md) (o "sem prints"); [0041](0041-testes-no-navegador-com-playwright.md) (Playwright e API falsa, que o gerador reaproveita); [0034](0034-sem-sri-no-build-do-front-end.md) e a CSP do site; [0017](0017-preferencias-de-exibicao-no-navegador.md) (o tema fica no navegador); [0061](0061-acesso-pelo-celular-na-rede-local.md) (acesso pelo celular)

## Contexto

O Manual (`src/frontend/src/app/paginas/manual/manual-conteudo.ts`, ADR 0026) tem 15 capítulos, cerca de 90 seções e ~17 mil palavras, e a #752 pede **mostrar mais e escrever menos**: prints onde eles explicam melhor que um parágrafo, e o texto encurtado em volta (#733).

A 0026 decidiu **"Sem prints nesta versão"**, por três motivos: ficam velhos a cada ajuste visual, pesam no build e cada um precisa de texto alternativo. Desde então, a 0041 trouxe o Playwright com a API falsa (`testes/navegador/api-falsa.ts`, só dados inventados), e a skill `testar-no-navegador` já tira prints de qualquer tela.

As medidas de 16/11/2026 estão na [versão da ADR com as medições](https://github.com/LucasDesenvGit/financas-local/blob/2790797b27114f4f7561300c2084101cad9a57cd/docs/adr/0071-manual-do-site-com-prints-gerados-das-telas.md#o-que-foi-medido). Em resumo: um trecho no computador, em escala 1, dá 37 KiB em PNG, 12,8 KiB em WebP pelo Chromium e 7,9 KiB em AVIF; o trecho cortado no elemento sai igual byte a byte ao regerar; num mês, 108 dos 327 merges mexeram em tela, com 4 versões estáveis. O servidor do Go ainda não servia `.webp`, a CSP já aceita imagens da mesma origem, e a data do dia e a fonte do sistema aparecem nos prints.

## Alternativas consideradas

- **Prints ou não.**
  - **(a) Só encurtar o texto (a #733 sozinha).** Custo zero, mas quem aprende vendo depende do "Abrir a tela", e a #752 fica sem resposta.
  - **(b) Prints gerados pelo Playwright com a API falsa: escolhida.** Mostra o caminho (Nielsen 6 e 10), com dados inventados, regerado em segundos. Custa peso, um gerador e um jeito de saber que o print ficou velho.
  - **(c) GIF ou vídeo curto.** Pesa muito mais (*inferido*), não sai igual a cada vez e pede pausa, `prefers-reduced-motion` e descrição (WCAG 2.2.2).
  - **(d) "Mostrar na tela": o "Abrir a tela" destaca o controle.** Não fica velho, mas pede código em cada tela. Complementa os prints; fica para depois.
- **Como saber que um print ficou velho.**
  - **(e1) Comparar pixels na CI.** Falharia em até um terço dos PRs, a referência muda com o sistema e a fonte, e é o tipo de check que a #689 tira.
  - **(e2) Regerar no PR que muda a tela.** Depende de lembrar, e o CSS global regeraria tudo.
  - **(e3) Regerar todos antes de cada versão estável: escolhida**, mais o (e2) quando o PR muda de propósito a tela de um print. Nenhum check a mais nos PRs.
  - **(e4) Gerar no build.** Todo build passaria a precisar de navegador, e ninguém olharia os prints antes de saírem.
- **Formato.** PNG pesa ~3× o WebP. **WebP pelo `canvas.toBlob` do Chromium: escolhido**, sem ferramenta nova. AVIF (−38%) pede o `avifenc` (não vem no Windows) ou o `sharp` (dependência nativa).
- **Tema, tela e densidade.** Um print por tema (o site baixa só o do tema aberto; com um só, o Manual no escuro teria um bloco branco, Nielsen 4, *julgamento próprio*). Só o computador (o celular é opcional, ADR 0061, e dobraria o total). Escala 1 (a 2 fica 2,3× maior, e o que se lê é o texto do passo, *julgamento próprio*).

## Decisão

**O Manual ganha prints gerados pelo Playwright com a API falsa (dados inventados), cortados no trecho da tela que o passo explica, em WebP, um por tema, guardados no repositório em `src/frontend/public/imagens/manual/` e regerados todos de uma vez antes de cada versão estável. O texto do passo continua inteiro: o print é complemento, nunca o único lugar do passo.**

- **No dado da seção:** `SecaoManual.print?: { id, alt }`, um por seção, com os arquivos `imagens/manual/<id>-claro.webp` e `<id>-escuro.webp`. O `alt` diz numa frase o que o print mostra (WCAG 1.1.1).
- **O gerador:** `testes/navegador/prints-manual.spec.ts`, só com `PRINTS_MANUAL=1`, pelo `npm run prints:manual` (compila e roda). Uma tabela com a rota, o ponto de partida e o preparo (clique, aba) de cada `id`; dia fixo no navegador e na API falsa; projeto "computador", escala 1, os dois temas, com movimento reduzido; ~760 px de largura (a coluna do Manual; tabelas largas pedem mais). O WebP sai do `canvas.toBlob('image/webp', 0.8)`, e as medidas vão para `prints-manual.ts` (gerado).
- **Na página:** um `<img>` entre o "Passo a passo" e o "Abrir a tela", com o `src` do tema aberto, `width` e `height` (a página não pula ao rolar), `loading="lazy"`, `max-width: 100%` e sem `figcaption`. A busca do Manual procura também no `alt`.
- **O `manual.spec.ts` confere** o `alt` (sem começar por "Imagem", "Print" ou "Foto", até 160 caracteres), os dois arquivos e as medidas de cada print, que nada sobra na pasta, e o teto de 50 KiB por arquivo e 2 MiB na pasta *(julgamento próprio)*.
- **O servidor:** `.webp` entra no `tiposDeConteudo` do Go (com teste) e no `servidor.mjs` dos testes. A CSP não muda.
- **Em dia:** quem prepara a versão estável roda o `npm run prints:manual` e envia o que mudou no PR da versão (passo no `CONTRIBUINDO.md`); o PR que muda de propósito a tela de um print regera junto. O conjunto guardado deve sair sempre do mesmo ambiente, para a fonte não trocar: um workflow manual na CI, num PR `ci:` à parte. Os primeiros saíram de um Linux.
- **A 0026** fica "Aceita (em parte substituída pela 0071)": só o "sem prints" muda.

## O aval (Felipe, 26/11/2026)

1. **Prints ou não:** prints (0,75 a 1,5 MiB; o texto do passo continua inteiro).
2. **Formato:** WebP pelo `canvas.toBlob`, sem dependência nova.
3. **Quando e onde regerar:** antes de cada versão estável, num workflow manual da CI (sempre no mesmo ambiente).
4. **Quantos tópicos:** uns 30, um print por seção, para começar.
5. **Celular e escala:** só o computador, em escala 1.
6. **Onde guardar:** no repositório.

## Consequências

- **Positivas:**
  - Ajuda que mostra onde fica cada coisa (Nielsen 6 e 10), com o texto encurtado em volta (#733).
  - Nenhum dado pessoal nos prints, só os inventados da API falsa.
  - O build continua sem navegador, e os PRs não ganham check nem tempo de CI.
  - Nenhuma saída de rede, fronteira ou dependência nova: nada muda no modelo de ameaças nem nos diagramas.
- **Negativas:**
  - **O repositório cresce** até ~1 MiB cada vez que todos os prints mudam.
  - **O `.msi` e a pasta publicada crescem** o mesmo (o WebP não comprime de novo, *inferido*); o bundle JS não muda, porque as imagens são arquivos à parte.
  - **Entre duas versões, um print pode ficar velho** no `develop`; só regerar pega.
  - **Mais um passo** na versão estável, e uma tabela no gerador que acompanha a tela. Mudou a tela de um print: a seção e o `alt` também.
  - **O print mostra a paleta padrão:** com uma cor personalizada (ADR 0017), fica diferente da tela.
  - **No celular**, o print do computador aparece reduzido, com o texto pequeno.
  - **Imagem de texto** (WCAG 1.4.5): vale porque o mesmo conteúdo está no passo em texto *(julgamento próprio)*.
- **Na prática:**
  - **Seção nova com print:** uma linha na tabela do gerador, `print: { id, alt }` na seção e `npm run prints:manual`; envie os dois WebP e o `prints-manual.ts`.
  - **De passagem** (independente desta ADR): tirar o `ajudaDaUrl` e o `AJUDA_POR_ROTA` do `manual-conteudo.ts`, para o texto do Manual sair do bundle inicial.
