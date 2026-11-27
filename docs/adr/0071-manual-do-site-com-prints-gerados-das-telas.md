# 0071. Manual do site com prints gerados das telas: Playwright e API falsa, WebP, um por tema, guardados no git

- **Status:** Proposta
- **Data:** 2026-11-16 (issue #752; anda junto com a #733, que encurta o texto do Manual, grupo 6 da #690)
- **Relacionadas:** substituirá em parte a [0026](0026-manual-do-site-como-dados-no-front-end.md) quando for aceita; [0041](0041-testes-no-navegador-com-playwright.md) (Playwright e API falsa, que o gerador reaproveita); [0034](0034-sem-sri-no-build-do-front-end.md) e a CSP do site; [0017](0017-preferencias-de-exibicao-no-navegador.md) (o tema fica no navegador); [0061](0061-acesso-pelo-celular-na-rede-local.md) (acesso pelo celular)

Enquanto for Proposta, a 0026 continua valendo: nada muda no site nem no Manual. Esta ADR só mede e propõe.

## Contexto

O Manual (`src/frontend/src/app/paginas/manual/manual-conteudo.ts`, ADR 0026) tem 15 capítulos, cerca de 90 seções e ~17 mil palavras, e a #752 pede **mostrar mais e escrever menos**: prints onde eles explicam melhor que um parágrafo, e o texto encurtado em volta (#733).

A 0026 decidiu **"Sem prints nesta versão"**, por três motivos: ficam velhos a cada ajuste visual, pesam no build e cada um precisa de texto alternativo. Desde então, a 0041 trouxe o Playwright com a API falsa (`testes/navegador/api-falsa.ts`, só dados inventados), e a skill `testar-no-navegador` já tira prints de qualquer tela.

As medidas de 16/11/2026 (4 trechos de tela, em PNG, WebP e AVIF) estão na [versão da ADR com as medições](https://github.com/LucasDesenvGit/financas-local/blob/2790797b27114f4f7561300c2084101cad9a57cd/docs/adr/0071-manual-do-site-com-prints-gerados-das-telas.md#o-que-foi-medido). Em resumo: um trecho no computador, em escala 1, dá 37 KiB em PNG, 12,8 KiB em WebP pelo Chromium e 7,9 KiB em AVIF; o build tem 1.979 KiB, o `.msi` 13,1 MiB e o repositório 10,1 MiB; o trecho cortado no elemento sai igual byte a byte ao regerar; num mês, 108 dos 327 merges mexeram em tela, com 4 versões estáveis. O código mostrou que o servidor do Go ainda não serve `.webp`, que a CSP já aceita imagens da mesma origem, que a data do dia e a fonte do sistema aparecem nos prints, e que o texto do Manual está no bundle inicial (o `shell.component.ts` importa o `ajudaDaUrl` do `manual-conteudo.ts`).

## Alternativas consideradas

### Prints ou não

- **(a) Continuar sem prints e só encurtar o texto (a #733 sozinha).** Custo zero, mas quem aprende vendo depende do "Abrir a tela", que pode não mostrar o que o passo descreve, e a #752 fica sem resposta.
- **(b) Prints gerados pelo Playwright com a API falsa: escolhida.** Mostra o caminho (Nielsen 6 e 10), com dados inventados, regerado em segundos. Custa peso, um gerador e um jeito de saber que o print ficou velho.
- **(c) GIF ou vídeo curto.** Mostra a sequência, mas pesa muito mais (*inferido*), não sai igual a cada vez e pede pausa, `prefers-reduced-motion` e descrição (WCAG 2.2.2). Pode voltar para um ou dois fluxos.
- **(d) "Mostrar na tela": o "Abrir a tela" destaca o controle** (ex.: `?destacar=nova-conta`). Não fica velho nem pesa, mas pede código em cada tela e não serve na tela vazia. Complementa os prints.

### Como saber que um print ficou velho

- **(e1) Comparar pixels na CI** (`toHaveScreenshot`). Falharia em até um terço dos PRs, a referência muda com o sistema, o navegador e a fonte (a #747 falhou por 1 ponto de suavização), e é o tipo de check que a #689 tira.
- **(e2) Regerar no PR que muda a tela.** Sempre em dia, mas depende de lembrar, e cada mudança do CSS global regeraria até ~1,5 MiB de prints (*inferido*).
- **(e3) Regerar todos de uma vez antes de cada versão estável: escolhida**, mais o (e2) quando o PR muda de propósito a tela de um print. Nenhum check a mais nos PRs; o PR de versão mostra o antes e depois dos prints que mudaram; o repositório cresce no ritmo das versões.
- **(e4) Gerar no build** (fora do repositório). Nunca fica velho, mas todo build passa a precisar de navegador (scripts, `versao.yml`, instalação pelo código) e ninguém olha os prints antes de saírem.

### Formato

- **PNG:** sem perda, mas ~3× o WebP e ~5× o AVIF.
- **WebP pelo encoder do Chromium: escolhido.** O próprio Chromium converte (`canvas.toBlob`), sem ferramenta nova; ~60% maior que o AVIF.
- **AVIF:** −38% contra o WebP, mas pede o `avifenc` (não vem no Windows) ou o `sharp` (dependência nativa, com ADR).

### Tema, tela e densidade

- **Tema: um print por tema, escolhido.** O site baixa só o do tema aberto (`TemaService.tema()`, ADR 0017); com um só, o Manual no escuro teria um bloco branco (Nielsen 4) *(julgamento próprio)*.
- **Tela: só o computador, por enquanto.** O acesso pelo celular é opcional (ADR 0061), e prints dele dobrariam o total.
- **Densidade: escala 1.** A escala 2 fica 2,3× maior, e o que se lê é o texto do passo *(julgamento próprio)*.

## Decisão (proposta)

**O Manual ganha prints gerados pelo Playwright com a API falsa (dados inventados), cortados no trecho da tela que o passo explica, em WebP, um por tema, guardados no git em `src/frontend/public/imagens/manual/` e regerados todos de uma vez antes de cada versão estável. O texto do passo continua inteiro: o print é complemento, nunca o único lugar do passo.**

Aos três motivos da 0026: ninguém tira print à mão (um comando regera todos); ~13 KiB por trecho, 0,75 a 1,5 MiB com 30 a 60 tópicos nos dois temas; e o texto alternativo é campo obrigatório, conferido pelo `manual.spec.ts`.

Como fica:

- **No dado da seção:** `SecaoManual` ganha `print?: { id, alt }`, um por seção para começar, com os arquivos `imagens/manual/<id>-claro.webp` e `<id>-escuro.webp`. O `alt` diz numa frase o que o print mostra (WCAG 1.1.1).
- **O gerador:** `testes/navegador/prints-manual.spec.ts`, só com `PRINTS_MANUAL=1`, pelo `npm run prints:manual`: uma tabela com a rota, o seletor do trecho e o preparo de cada `id`; data fixa no navegador e na API falsa; projeto "computador", escala 1, os dois temas; o WebP pelo `canvas.toBlob('image/webp', 0.8)` e um `prints-manual.ts` com a largura e a altura de cada `id`. O trecho tem até ~760 px de largura, a da coluna do Manual.
- **Na página:** um `<img>` entre o "Passo a passo" e o "Abrir a tela", com o `src` do tema aberto, `width` e `height` (para a página não pular ao rolar até a âncora), `loading="lazy"`, sem `figcaption`. A busca do Manual procura também no `alt`.
- **O `manual.spec.ts` confere** o `alt` (sem começar por "Imagem", "Print" ou "Foto", até 160 caracteres), os arquivos e um teto de 50 KiB por arquivo e 2 MiB na pasta *(julgamento próprio)*.
- **O servidor:** `.webp` entra no `tiposDeConteudo` (com teste) e no `servidor.mjs` dos testes. A CSP não muda.
- **Em dia:** quem prepara a versão estável roda `npm run prints:manual` e commita o que mudou (passo no `CONTRIBUINDO.md`); o PR que muda de propósito a tela de um print regera junto. O conjunto guardado sai sempre do mesmo ambiente, para a fonte não trocar: a proposta é o Windows, ou um workflow manual (`windows-latest`) num PR `ci:` à parte.
- **A 0026**, quando esta for aceita, fica "Aceita (em parte substituída pela 0071)": só o "sem prints" muda.

## Consequências

- **Positivas:**
  - Ajuda que mostra onde fica cada coisa (Nielsen 6 e 10), com o texto encurtado em volta (#733).
  - Nenhum dado pessoal nos prints, só os inventados da API falsa.
  - O build continua sem navegador, e os PRs não ganham check nem tempo de CI.
  - Nenhuma saída de rede, fronteira ou dependência nova: nada muda no modelo de ameaças nem nos diagramas.
- **Negativas:**
  - **O repositório cresce** até ~1,5 MiB cada vez que todos os prints mudam (~6 MiB por mês no pior caso, *inferido*).
  - **O build cresce** de 39% a 78%, e o `.msi`, de 6% a 11% (o WebP não comprime de novo, *inferido*).
  - **Entre duas versões, um print pode ficar velho** no `develop`; só regerar pega.
  - **Mais um passo** na versão estável, e uma tabela no gerador que acompanha a tela.
  - **O print mostra a paleta padrão:** com uma cor personalizada (ADR 0017), fica diferente da tela.
  - **No celular**, o print do computador aparece reduzido, com o texto pequeno.
  - **Imagem de texto** (WCAG 1.4.5): vale porque o mesmo conteúdo está no passo em texto *(julgamento próprio)*.
- **Na prática** (depois do aval, num PR `feat(web)`, e os prints por capítulo junto da #733):
  - **Seção nova com print:** uma linha na tabela do gerador, `print: { id, alt }` na seção e `npm run prints:manual`; commite os dois WebP e o `prints-manual.ts`.
  - **De passagem** (independente desta ADR): tirar o `ajudaDaUrl` e o `AJUDA_POR_ROTA` do `manual-conteudo.ts`, para o texto do Manual sair do bundle inicial.

## Perguntas para o aval

1. **Prints ou não**, com o peso acima, ou fica a (a)?
2. **WebP ou AVIF?**
3. **Quando e onde regerar:** antes de cada versão (proposta), em todo PR que muda tela ou toda semana; no Windows, num workflow manual ou em qualquer máquina?
4. **Quantos tópicos:** uns 30 ou até 60? Um print por seção basta?
5. **Celular e densidade:** só o computador em escala 1, ou também o celular ou a escala 2?
6. **No repositório ou no build (e4)?**
