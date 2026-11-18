# 0071. Manual do site com prints gerados das telas: Playwright e API falsa, WebP, um por tema, guardados no git

- **Status:** Proposta
- **Data:** 2026-11-16 (issue #752; anda junto com a #733, que encurta o texto do Manual, grupo 6 da #690)
- **Relacionadas:** substituirá em parte a [0026](0026-manual-do-site-como-dados-no-front-end.md) quando for aceita (só o "Sem prints nesta versão"; o resto da 0026 continua); [0041](0041-testes-no-navegador-com-playwright.md) (Playwright e API falsa, que o gerador reaproveita); [0034](0034-sem-sri-no-build-do-front-end.md) e a CSP do site; [0017](0017-preferencias-de-exibicao-no-navegador.md) (o tema fica no navegador); [0061](0061-acesso-pelo-celular-na-rede-local.md) (acesso pelo celular)

Enquanto for Proposta, a 0026 continua valendo: nada muda no site nem no Manual. Esta ADR só mede e propõe.

## Contexto

O Manual (`src/frontend/src/app/paginas/manual/manual-conteudo.ts`, ADR 0026) tem 15 capítulos e cerca de 90 seções. São ~17 mil palavras, e a #752 pede **mostrar mais e escrever menos**: prints onde eles explicam melhor que um parágrafo, e o texto encurtado em volta (#733).

A 0026 avaliou prints e decidiu **"Sem prints nesta versão"**, por três motivos:
1. ficam velhos a cada ajuste visual;
2. pesam no build;
3. cada um precisa de texto alternativo.

Desde então, a 0041 trouxe o Playwright com a API falsa (`testes/navegador/api-falsa.ts`, só dados inventados), e a skill `testar-no-navegador` já tira prints de qualquer tela (`FOTOS=… npx playwright test fotos`). Esta ADR responde aos três motivos com medidas feitas em 16/11/2026, nesta máquina (Linux, Chromium do Playwright, `NAVEGADOR=chromium`).

### O que foi medido

**Como:** um spec temporário (fora do commit) cortou 4 trechos de tela que o Manual explicaria, no elemento e não na página inteira, com `locator.screenshot({ animations: 'disabled', caret: 'hide' })`:
- o formulário "Nova conta a pagar" (`section.modal`);
- os filtros de Transações (`section.filtros`);
- os cards "Situação hoje" do Painel (`section.kpis-equilibrados`);
- um card de Metas (`app-metas-cartao`).

Cada um saiu nos temas claro e escuro e nos dois projetos do Playwright: computador (1366×860, escala 1) e celular (Pixel 7, salvo em escala 1 e na escala do aparelho, 2,625). Os PNGs foram convertidos para:
- WebP pelo encoder do próprio Chromium (`canvas.toBlob('image/webp', 0.8)`, sem ferramenta nova);
- WebP pelo `magick` (ImageMagick 7.1.2, libwebp 1.6.0, qualidade 80, `method=6`);
- AVIF pelo `avifenc` 1.4.2 (aom, `-q 60` e `-q 50`, velocidade 4).

O `cwebp` não está instalado, e o `magick` desta máquina não grava AVIF.

**Tamanho por print** (KiB; trechos, escala 1):

| Trecho | Tela | Tema | Pixels | PNG | WebP (Chromium 0,8) | WebP (libwebp 80) | AVIF (q 60) | AVIF (q 50) |
|---|---|---|---|---:|---:|---:|---:|---:|
| Formulário de Contas a pagar | computador | claro | 620×740 | 43,9 | 16,9 | 16,4 | 10,6 | 9,5 |
| Formulário de Contas a pagar | computador | escuro | 620×740 | 44,5 | 15,8 | 15,2 | 9,9 | 9,1 |
| Formulário de Contas a pagar | celular | claro | 380×723 | 31,6 | 11,4 | 10,8 | 7,4 | 6,6 |
| Formulário de Contas a pagar | celular | escuro | 380×723 | 32,0 | 10,5 | 9,9 | 7,0 | 6,1 |
| Filtros de Transações | computador | claro | 1042×267 | 22,6 | 9,1 | 8,4 | 5,8 | 5,4 |
| Filtros de Transações | computador | escuro | 1042×267 | 23,0 | 8,7 | 8,2 | 5,6 | 5,2 |
| Filtros de Transações | celular | claro | 380×685 | 27,1 | 8,6 | 7,9 | 5,1 | 4,6 |
| Filtros de Transações | celular | escuro | 380×685 | 27,8 | 8,2 | 7,6 | 4,9 | 4,4 |
| Cards "Situação hoje" do Painel | computador | claro | 1042×162 | 50,6 | 16,4 | 15,6 | 9,2 | 8,1 |
| Cards "Situação hoje" do Painel | computador | escuro | 1042×162 | 51,4 | 15,7 | 15,0 | 9,1 | 8,0 |
| Cards "Situação hoje" do Painel | celular | claro | 380×644 | 62,9 | 16,2 | 15,5 | 8,8 | 7,7 |
| Cards "Situação hoje" do Painel | celular | escuro | 380×644 | 62,3 | 15,6 | 14,9 | 8,7 | 7,7 |
| Card de Metas | computador | claro | 1042×383 | 30,1 | 9,9 | 9,3 | 6,4 | 5,6 |
| Card de Metas | computador | escuro | 1042×383 | 31,7 | 9,7 | 9,1 | 6,4 | 5,8 |
| Card de Metas | celular | claro | 380×471 | 29,4 | 9,4 | 8,7 | 6,3 | 5,7 |
| Card de Metas | celular | escuro | 380×471 | 31,3 | 9,2 | 8,5 | 6,4 | 5,6 |
| **Média, trecho no computador** | | | | **37,2** | **12,8** | **12,1** | **7,9** | **7,1** |

**Para comparar** (médias, KiB):

| Variante | PNG | WebP (libwebp 80) | AVIF (q 60) |
|---|---:|---:|---:|
| Trecho no computador, escala 2 (tela de alta densidade) | 98,6 | 27,8 | 17,4 |
| Trecho no celular, escala do aparelho (2,625) | 136,7 | 34,1 | 19,4 |
| Página inteira do Painel, computador (1366×1801) | 249,6 | 83,4 | 53,0 |

**Qualidade:** no filtro de Transações (claro, computador), contra o PNG, o AVIF q 60 deu PSNR 48,2 dB e SSIM 0,9996, e o WebP 80 deu 42,6 dB e 0,997. No formulário escuro, o AVIF q 50 deu 45,2 dB. Ampliado 2×, o texto do WebP 80 e do AVIF q 50 não tem borrão nem halo visível. A conferência foi a olho, nesta máquina.

**O build hoje** (`npm run build` em `src/frontend`, 5,7 s): `dist/financas-web/browser` tem **2.026.351 bytes (1.979 KiB) em 58 arquivos**. Os únicos binários são os três ícones PNG (113 KiB juntos); o resto são os `.js`, o `.css` e os SVGs. O `.msi` de desenvolvimento de 16/11 tem **13.737.984 bytes (13,1 MiB)**. O repositório tem 10,1 MiB, pelo tamanho que o GitHub informa.

**Projeção:** a média dos trechos no computador vezes o número de prints.

| Tópicos com print | Variantes | Arquivos | PNG | WebP (Chromium) | AVIF (q 60) |
|---|---|---:|---:|---:|---:|
| 30 | só claro | 30 | 1.116 KiB (+56% do build) | 384 KiB (+19%) | 237 KiB (+12%) |
| 30 | claro e escuro | 60 | 2.232 KiB (+113%) | 767 KiB (+39%) | 473 KiB (+24%) |
| 60 | claro e escuro | 120 | 4.464 KiB (+226%) | 1.534 KiB (+78%) | 947 KiB (+48%) |
| 60 | claro e escuro, computador e celular | 240 | 8.928 KiB (+451%) | ~2.870 KiB (+145%) | ~1.770 KiB (+89%) |

**Tempo e repetição:**
- os 16 prints levaram **3,3 s** (8 workers, com o servidor), e 3,0 s na segunda vez;
- entre as duas vezes, **os 24 trechos saíram iguais byte a byte**. As duas páginas inteiras mudaram (38 e 708 pixels): o recorte no elemento é o que dá estabilidade;
- o WebP do Chromium converteu os 26 arquivos em 1,0 s, e o `avifenc` (velocidade 4), 18 arquivos em 6,5 s. Os dois deram bytes iguais ao repetir.

**Com que frequência as telas mudam** (merges no `develop`, de 16/10 a 16/11): **327 merges**. **108** mexeram em `paginas/` ou `compartilhado/`, e **20** no CSS global (`styles.css`). Saíram 4 versões estáveis no período (a última, `v3.3.14`, em 05/11).

**O que mais o código mostrou:**
- **O servidor do Go não serve WebP nem AVIF.** O `tiposDeConteudo` de `internal/plataforma/servidor/estaticos.go` só conhece `.css`, `.html`, `.js`, `.png`, `.svg`, `.txt` e `.webmanifest`, e extensão sem tipo responde 401. O `servidor.mjs` dos testes também não tem os dois tipos.
- **A CSP já aceita as imagens:** `img-src 'self' data:` (`intermediarios.go`). Os prints vêm da mesma origem e a CSP não muda.
- **Cache:** arquivo sem hash no nome recebe `Cache-Control: no-cache` e ETag, e o navegador revalida com um 304.
- **A data aparece nos prints.** A API falsa calcula "hoje" no Node (`new Date()`) e o site, no navegador. O formulário mostrou "16/11/2026" e os filtros, "01/11/2026 a 30/11/2026". Sem data fixa, regerar no mês seguinte muda todos os prints que mostram data.
- **A fonte depende do sistema:** `--fonte: "Segoe UI Variable Text", "Segoe UI", system-ui, …` (`styles.css`). Gerado no Windows, o print sai com a Segoe UI, a mesma que a maioria vê (*inferido*: a equipe usa Windows, salvo o Felipe). No Linux, sai com a fonte do sistema. Os prints medidos aqui são do Linux.
- **De passagem:** o texto do Manual está no **bundle inicial** (`main-*.js`), e não só no da página do Manual, como diz a 0026. O `shell.component.ts` importa o `ajudaDaUrl` do mesmo `manual-conteudo.ts`, e o esbuild leva o arquivo inteiro para o `main`. Num teste à parte, com o `ajudaDaUrl` e o `AJUDA_POR_ROTA` num arquivo próprio, o `main` caiu de 449,1 kB para 346,8 kB (transferência estimada de 63,1 para 36,4 kB). O Manual foi para um chunk lazy de 113,1 kB. Isso vale com ou sem prints: o texto alternativo dos prints iria para o mesmo lugar.

## Alternativas consideradas

### Prints ou não

- **(a) Continuar sem prints e só encurtar o texto (a #733 sozinha).**
  - Prós: custo zero; nada para ficar velho; a 0026 fica como está.
  - Contras: quem aprende vendo continua dependendo do "Abrir a tela". Com dados de verdade, ou sem dados no começo, a tela pode não mostrar o que o passo descreve (um formulário fechado, um aviso que não aparece). O pedido da #752 fica sem resposta.
- **(b) Prints gerados pelo Playwright com a API falsa: escolhida.**
  - Prós: mostra o caminho (Nielsen 6, reconhecer em vez de lembrar; Nielsen 10, ajuda concreta); os dados são inventados; regera tudo em segundos; trecho cortado no elemento sai igual byte a byte quando a tela não mudou.
  - Contras: peso no build e no git; precisa de um gerador e de um jeito de saber que o print ficou velho (abaixo).
- **(c) GIF ou vídeo curto.**
  - Prós: mostra a sequência (abrir, preencher, salvar).
  - Contras:
    - não medido aqui: um vídeo de 5 s pesa algumas dezenas a centenas de KiB, contra ~13 KiB de um print (*inferido*);
    - o vídeo do Playwright (`recordVideo`) não sai igual a cada vez, e não dá para saber se ficou velho sem olhar;
    - a acessibilidade pede mais: pausar o que se mexe por mais de 5 s (WCAG 2.2.2), respeitar o `prefers-reduced-motion` e descrever a sequência em texto.

    Pode voltar depois, para um ou dois fluxos, se os prints não bastarem.
- **(d) "Mostrar na tela": o "Abrir a tela" destaca o controle na tela de verdade** (ex.: `?destacar=nova-conta`).
  - Prós: nunca fica velho e não pesa nada.
  - Contras: código novo em cada tela; não funciona quando a tela está vazia (primeiro uso) nem para ler o Manual com calma. Complementa os prints, não os substitui.

### Como saber que um print ficou velho

- **(e1) Comparar pixels na CI** (`toHaveScreenshot` do Playwright).
  - Prós: pega toda mudança visual.
  - Contras:
    - pelos números de outubro e novembro, falharia em até um terço dos PRs (108 de 327 mexem em tela), e qualquer um dos 20 que mexem no CSS global pode invalidar todos os prints;
    - a referência muda com o sistema e o navegador (o Playwright guarda uma por plataforma; Edge aqui, Chromium na CI; Segoe UI × fonte do Linux);
    - a #747 mostrou que comparar prints falha por 1 ponto de suavização de borda;
    - é exatamente o tipo de check que a #689 está tirando: trava o PR para pedir um `--update-snapshots`.
- **(e2) Regerar no PR que muda a tela.**
  - Prós: o print fica em dia o tempo todo.
  - Contras: depende de lembrar. No git, cada mudança do CSS global regeraria até 120 prints, ~1,5 MiB por vez. Com 20 por mês, seriam até ~30 MiB por mês, num repositório de 10 MiB (pior caso, *inferido*).
- **(e3) Regerar todos de uma vez antes de cada versão estável: escolhida**, mais o (e2) quando o PR mudar de propósito a tela de um print.
  - Prós:
    - nenhum check novo nem navegador a mais nos PRs;
    - o gerador é determinístico, então o diff do PR de versão mostra só os prints que mudaram, e o GitHub mostra a imagem antes e depois para quem revisa;
    - o crescimento do git fica limitado ao número de versões (4 no último mês).
  - Contras: entre duas versões, o Manual do `develop` pode mostrar um print velho; quem prepara a versão ganha um passo.
- **(e4) Gerar no build** (os prints fora do git).
  - Prós: nunca ficam velhos; o git não cresce.
  - Contras:
    - o build passa a precisar de navegador, em todo lugar onde roda: o `3-publicar-atualizar.sh`, o `instalar-windows.ps1`, o `versao.yml` e a instalação pelo código (que hoje só precisa do Node);
    - ninguém olha os prints antes de saírem: um print com erro, ou com a tela no estado errado, vai direto para quem usa;
    - um build sem navegador sai com o Manual sem imagens.

### Formato

- **PNG:**
  - Prós: sem perda.
  - Contras: ~3× o WebP e ~5× o AVIF (medido).
- **WebP pelo encoder do Chromium: escolhido.**
  - Prós: o próprio navegador do Playwright converte (`canvas.toBlob`), sem dependência nem ferramenta nova, no Windows e no Linux; todo navegador atual abre.
  - Contras: ~5% maior que o libwebp com `method=6` (12,8 contra 12,1 KiB) e ~60% maior que o AVIF q 60.
- **AVIF:**
  - Prós: o menor (7,9 KiB de média, −38% contra o WebP), com qualidade igual ou melhor (PSNR 48 dB).
  - Contras: o Chromium não grava AVIF. Precisa do `avifenc` no sistema (não vem no Windows) ou de uma dependência nova (o `sharp`, com binários nativos), e de uma ADR para ela.

### Tema, tela e densidade

- **Tema: um print por tema, escolhido.** O site sabe o tema em uso (`TemaService.tema()`, ADR 0017), e a página baixa só o do tema aberto. O custo é o dobro no disco e no git, mas não na rede. Com um print só, claro, o Manual no escuro mostraria um bloco branco no meio da página (Nielsen 4, consistência) *(julgamento próprio)*.
- **Tela: só o computador, por enquanto.** O acesso pelo celular é opcional e começa desligado (ADR 0061). Prints do celular dobrariam o total (240 arquivos, ~2,8 MiB em WebP). No celular, o print do computador aparece reduzido. O passo continua inteiro em texto, e o gesto de pinça amplia.
- **Densidade: escala 1.** Na escala 2, o print fica 2,3× maior em WebP (27,8 contra 12,1 KiB) e mais nítido em tela de alta densidade. O print mostra onde fica cada coisa, e o texto do passo é o que se lê *(julgamento próprio; não conferido num notebook com escala de 150%)*.

## Decisão (proposta)

**O Manual ganha prints gerados pelo Playwright com a API falsa (dados inventados), cortados no trecho da tela que o passo explica, em WebP, um por tema, guardados no git em `src/frontend/public/imagens/manual/` e regerados todos de uma vez antes de cada versão estável. O texto do passo continua inteiro: o print é complemento, nunca o único lugar do passo.**

Respostas aos três motivos da 0026:

1. **"Ficam velhos":** ninguém tira print à mão. Um comando regera todos em segundos e, com a data fixa, sai igual byte a byte quando a tela não mudou. O diff do PR de versão mostra o que mudou, e o GitHub mostra a imagem antes e depois. Não há comparação de pixels na CI (e1).
2. **"Pesam no build":** WebP de ~13 KiB por trecho (medido). Com 30 a 60 tópicos nos dois temas, são 0,75 a 1,5 MiB, de 39% a 78% do build de hoje e de 6% a 11% do `.msi`. Cada página baixa só os prints do tema aberto, e só quando chegam perto da tela (`loading="lazy"`).
3. **"Texto alternativo":** campo obrigatório no tipo da seção, conferido pelo `manual.spec.ts`.

Como fica:

- **No dado da seção** (`manual-conteudo.ts`):
  ```ts
  export interface PrintManual {
    /** Nome do print no gerador; os arquivos são imagens/manual/<id>-claro.webp e <id>-escuro.webp. */
    id: string;
    /** O que o print mostra, numa frase (WCAG 1.1.1). Não repete o título nem começa com "Imagem de". */
    alt: string;
  }
  // SecaoManual ganha: print?: PrintManual;   (um por seção, para começar)
  ```
- **O gerador:** `src/frontend/testes/navegador/prints-manual.spec.ts`, que só roda com `PRINTS_MANUAL=1` (como o `fotos.spec.ts`), chamado por `npm run prints:manual`.
  - Uma tabela diz, para cada `id`: a rota, o seletor do trecho e o preparo (ex.: clicar em "+ Nova conta").
  - A data é fixa: `page.clock.setFixedTime` no navegador e a mesma data na API falsa, por variável de ambiente.
  - Roda no projeto "computador", em escala 1, nos dois temas, com o movimento reduzido.
  - Grava o WebP pelo `canvas.toBlob('image/webp', 0.8)` e escreve `paginas/manual/prints-manual.ts`, com a largura e a altura de cada `id`.
  - O trecho tem no máximo ~760 px de largura, para aparecer em tamanho real na coluna do Manual. Trecho mais largo é tirado numa janela mais estreita ou cortado.
- **Na página do Manual:** um `<img>` depois do "Passo a passo" e antes do "Abrir a tela", com:
  - o `src` do tema aberto (`TemaService.tema()`);
  - o `alt` da seção;
  - `width` e `height` do `prints-manual.ts`, para a página não pular quando a imagem chega. Isso importa porque o link `/manual/<capitulo>#<secao>` rola até a âncora antes de as imagens carregarem *(julgamento próprio)*;
  - `loading="lazy"` e `decoding="async"`.

  O print não leva legenda (`figcaption`), para o leitor de tela não ler a mesma frase duas vezes. A busca do Manual passa a procurar também no `alt`.
- **O `manual.spec.ts` confere** que:
  - toda seção com `print` tem `alt` com texto (sem começar por "Imagem", "Print" ou "Foto", e com até 160 caracteres);
  - o `id` existe no `prints-manual.ts`, e os dois arquivos existem em `public/imagens/manual/`;
  - nenhum arquivo passa de 50 KiB, e a pasta inteira, de 2 MiB. É um teto, para um PNG ou uma página inteira não entrarem por engano *(números do julgamento próprio, a partir das medidas)*.
- **O servidor:** `.webp` entra no `tiposDeConteudo` (`estaticos.go`, `image/webp`, com teste, ADR 0051) e no `servidor.mjs` dos testes. A CSP não muda.
- **Em dia:** quem prepara a versão estável roda `npm run prints:manual` e commita o que mudou, e isso entra no `CONTRIBUINDO.md`. O PR que muda de propósito a tela de um print regera o print junto.
  - **Onde gerar** (pergunta 4): para a fonte não trocar conforme quem gera, o conjunto que vai para o git sai sempre do mesmo ambiente. A proposta é o Windows, com o Chromium do Playwright, que tem a Segoe UI que a maioria vê. Quem não está no Windows gera só para conferir, ou usa um workflow manual (`workflow_dispatch`, `windows-latest`), num PR `ci:` à parte (ADR 0039).
- **A 0026**, quando esta for aceita: "Aceita (em parte substituída pela 0071)". Pelo critério do `docs/adr/README.md`, a decisão "sem prints" mudou, então não é só um detalhe da mesma decisão. O resto da 0026 (o Manual como dados tipados, os endereços, o botão "Ajuda" e os testes de regressão) continua igual. É o mesmo arranjo da 0008, da 0027 e da 0029 no índice.

## Consequências

- **Positivas:**
  - Ajuda que mostra onde fica cada coisa (Nielsen 6 e 10), com o texto encurtado em volta (#733).
  - Nenhum dado pessoal nos prints: só os dados inventados da API falsa (nada muda no `docs/DADOS-PESSOAIS.md`).
  - O build continua sem navegador, e o `.msi`, o `3-publicar-atualizar.sh` e a instalação pelo código trazem os mesmos prints.
  - Os PRs não ganham check nem tempo de CI.
  - Nenhuma saída de rede nova, nenhuma fronteira nova, nenhuma dependência nova: nada muda no modelo de ameaças nem nos diagramas da `docs/ARQUITETURA.md`.
- **Negativas:**
  - **O git cresce.** Cada vez que todos os prints mudam, entram até ~1,5 MiB (60 tópicos × 2 temas), num repositório de 10 MiB. Com 4 versões por mês, no pior caso, seriam ~6 MiB por mês (*inferido*: só acontece se o CSS global mudar todos os prints entre duas versões). Em AVIF, seria ~0,9 MiB por vez.
  - **O build cresce** de 39% a 78% (0,75 a 1,5 MiB), e o `.msi`, de 6% a 11% (o WebP não comprime de novo dentro do `.msi`, *inferido*).
  - **Entre duas versões, um print pode ficar velho** no `develop`. A data fixa não pega isso; só regerar pega.
  - **Mais um passo** para quem prepara a versão, e uma tabela no gerador que precisa acompanhar a tela (o seletor do trecho pode quebrar quando a tela muda, e então o gerador falha, o que é bom).
  - **A paleta e o tema do usuário** podem ser outros: o print mostra a paleta padrão. Com uma cor personalizada, o print fica com outra cor que a tela (ADR 0017).
  - **No celular**, o print do computador aparece reduzido; o texto dentro dele fica pequeno (o passo continua em texto).
  - **Imagem de texto** (WCAG 1.4.5): o print é imagem de uma tela que tem texto. Vale porque o mesmo conteúdo está no passo em texto, e o print só mostra a aparência da tela *(julgamento próprio)*.
- **Na prática** (depois do aval, num PR `feat(web)`, e os prints por capítulo junto da #733):
  - **Seção nova com print:** uma linha na tabela do `prints-manual.spec.ts`, o campo `print: { id, alt }` na seção e `npm run prints:manual`. Commite os dois WebP e o `prints-manual.ts`.
  - **Mudou de propósito a tela de um print:** regere no mesmo PR.
  - **Versão estável:** `npm run prints:manual` e os prints que mudaram no PR da versão.
  - **De passagem** (independente desta ADR): tirar o `ajudaDaUrl` e o `AJUDA_POR_ROTA` do `manual-conteudo.ts`, para o texto do Manual sair do bundle inicial (−102 kB no `main`, medido), como a 0026 já dizia.

## Perguntas para o aval

1. **Prints ou não?** A equipe aceita o peso (0,75 a 1,5 MiB no build e até ~1,5 MiB no git a cada regeração completa) em troca do Manual mais visual? Ou fica a (a), só o texto mais curto?
2. **WebP ou AVIF?** O WebP sai sem nada novo; o AVIF economiza ~38%, mas pede o `avifenc` ou o `sharp` (dependência nova, com ADR).
3. **Quando regerar:** antes de cada versão estável (proposta), em todo PR que muda tela, ou uma vez por semana?
4. **Onde gerar:** no Windows de quem prepara a versão, num workflow manual da CI (`windows-latest`), ou em qualquer máquina, aceitando que a fonte mude conforme quem gerou?
5. **Quantos tópicos:** começar com uns 30 (as telas principais) ou ir até 60? Um print por seção basta?
6. **Celular e densidade:** só o computador, em escala 1 (proposta)? Ou também o celular, ou a escala 2 para telas de alta densidade (~2,3× o peso)?
7. **No git ou no build:** a proposta guarda no git, para o build seguir sem navegador e alguém olhar os prints no PR. Alguém prefere gerar no build (e4), aceitando o navegador no build e os prints sem revisão?
