# 0052. Gerenciador em Go: ícone na bandeja e a "janela" como página local no navegador

- **Status:** Proposta (#276; a decisão é do time)
- **Data:** 2026-10-25
- **Relacionadas:** [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) e [0033](0033-gerenciador-no-linux-com-avalonia.md) (o gerenciador de hoje, em C#, que esta substituiria); [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) (atualizar pelo develop); [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o `.msi`); [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (o back-end em Go); épico #70

## Contexto

O gerenciador liga, desliga e acompanha o site, faz a cópia do banco e se atualiza. É todo em C#, com cerca de 3.600 linhas:
- `Financas.Gerenciador.Nucleo` (708): serviço, situação, cópia do banco, atualização;
- `Financas.Gerenciador`, para Windows (1.956): WinForms, UAC, GitHub Releases;
- `Financas.Gerenciador.Linux` (967): Avalonia, bandeja, `systemctl --user`.

O épico #70 remove o .NET no fim (#86). Sem tratar o gerenciador, o `.msi`, os scripts e a CI deixam de gerá-lo. **O time decidiu em 21/10/2026 que o destino é tudo em Go**, inclusive o gerenciador (revisão do épico, comentário na #276). Falta decidir **como** desenhar a parte com janela.

O que ele precisa continuar fazendo (lista da #276):
- funcionar com o site desligado, porque é ele que liga;
- UAC só nas ações que precisam;
- serviço do Windows e `systemctl --user`;
- bandeja e instância única;
- atualização pelo GitHub Releases com SHA-256, e pelo develop na instalação de desenvolvimento;
- os mesmos textos e ações nos dois sistemas;
- nada instalado à parte na máquina de quem usa;
- teclado, leitor de tela e contraste.

## Os testes (#276, 24 e 25/10/2026)

Três protótipos, com as mesmas funções (situação, ligar, desligar, ícone com menu), e o C# de hoje como referência.
- **No Windows 11:** tamanho no disco; memória privada do processo e dos filhos; ícone registrado na bandeja (`NotifyIconSettings`); árvore de acessibilidade pela UI Automation (o que o Narrador lê); ordem do Tab; UAC de verdade.
- **No Linux:** Ubuntu 24.04 no WSL2, com o WSLg.

| | C# de hoje | **A: Go + bandeja + página local** | B: [Wails](https://github.com/wailsapp/wails) v3 | C: [Fyne](https://fyne.io) 2 |
|---|---|---|---|---|
| Tamanho (Windows) | 121 MB, 240 arquivos (self-contained) | **9,4 MB**, 1 arquivo | 12,5 MB | 23,6 MB |
| Tamanho (Linux) | – | **9,6 MB**, estático | 13 MB | 24 MB |
| Memória privada (Windows) | **14–15 MB** | 46 MB | 188,5 MB (7 processos do WebView2) | 188 MB |
| Memória em uso, *working set* (Windows) | 56–59 MB | 14–53 MB | – | – |
| Memória (Linux, WSLg) | – | **14 MB** | 133 MB (a página não carregou; abaixo) | 185 MB |
| Precisa de cgo para compilar | – | **não** (compilado com `CGO_ENABLED=0` para os dois sistemas) | no Linux | sim, nos dois (compilação de 4 a 6,5 min) |
| Precisa na máquina, no Linux | – | **nada** | GTK 4 e WebKitGTK 6.0: 113 pacotes, ~231 MB se faltarem | OpenGL e X11: 12 pacotes, ~16 MB |
| Ícone na bandeja (Windows) | sim | sim | sim | sim |
| Leitor de tela (Windows, UI Automation) | sim | a página no navegador; o menu do ícone é do sistema | texto e botões expostos; o Tab passa por Ligar → Desligar | **nada exposto** |
| UAC (ligar e desligar o serviço) | sim | **testado:** pediu o "Sim", ligou em 4 s e desligou em 2 s | o mesmo código do A | o mesmo código do A |
| Situação da biblioteca | – | [`fyne.io/systray`](https://github.com/fyne-io/systray), push em 24/10/2026 | **versão prévia:** v3.0.0-beta.34 em 20/10/2026 | estável |

**Na memória, o A não ganha do C#.** Medidos lado a lado, duas vezes: o C# fica com 14–15 MB de memória privada, e o A com 46 MB, porque o runtime do Go deixa mais memória reservada. Em uso de fato (*working set*), o A oscila entre 14 e 53 MB, e o C# fica perto de 58 MB. A vantagem do A é o tamanho e não depender de nada no Linux, e não a memória.

**O visual e a janela (protótipo D, 26/10/2026).** O A com a página no visual do site:
- **o `styles.css` do próprio site** vai embutido no executável, com as classes dele (`.botao`, `.botao-primario`, `.ponto`, `.marca`), e o tema claro ou escuro vem do "Modo de aplicativo" do Windows, como no `Tema.cs` de hoje. A página foi desenhada pelo Edge sem janela (`--headless --screenshot`), nos dois temas, e ficou igual ao site;
- **hoje, as cores do gerenciador são uma cópia à mão do site, e já se desencontraram:** linha `#e2dfee` × `#e8e6f1`, positivo `#2a9a6a` × `#1b7550`, negativo `#d2546b` × `#b23a52`. Com o CSS do site embutido, deixa de haver cópia;
- **janela de aplicativo:** o gerenciador abre a página com `msedge --app=<endereço>`, e a UI Automation achou uma janela própria do Edge com o título da página. *Não confirmado:* se a barra de endereço some. A UI Automation listou um campo "Barra de pesquisa e endereços" na janela, e não olhei a tela;
- **memória com a janela aberta:** o gerenciador fica em 46–48 MB, e o Edge soma 53 MB (com o Edge já aberto, 3 processos) ou 124 MB (com o Edge fechado, 5 processos). **Com a janela aberta, o total chega perto do Wails;** com ela fechada, só o gerenciador fica na memória;
- **não testados aqui:** o leitor de tela e o Tab dentro da página. O teste de teclado mandaria teclas para a janela que estivesse na frente, e foi interrompido.

O que **não** deu para conferir:
- **Bandeja no Linux:** o WSLg não tem quem mostre o ícone. Os três disseram "org.kde.StatusNotifierWatcher was not provided", porque o `fyne.io/systray` usa o padrão StatusNotifierItem pelo D-Bus. Falta testar num GNOME e num KDE de verdade.
- **Wails no Linux:** nem o MiniBrowser do próprio WebKitGTK abriu o processo que desenha a página no WSLg. É do ambiente, e não do Wails, mas a janela do B no Linux ficou sem teste.
- **Leitor de tela no Linux** (Orca), nas três.

**Página pelo Edge × programa com Wails (protótipos D e E, 26/10/2026).** A mesma página (o `styles.css` do site, o mesmo HTML e o mesmo tema) em duas "janelas": o D a abre com `msedge --app`; o E é um programa com o Wails v3, com janela própria e o HTML servido por dentro, sem porta aberta. Três rodadas de cada no Windows 11, sem teclas e sem captura de tela:

| | D: página pelo Edge | E: programa com Wails |
|---|---|---|
| Janela aparece em | 0,5 a 1 s | 0,7 a 1,3 s |
| Memória privada com a janela aberta | ~100 MB (gerenciador 48 + Edge 52, com o Edge já aberto em segundo plano; ~170 com ele fechado) | ~195 a 204 MB (gerenciador 52 + WebView2, 7 processos) |
| O que a UI Automation vê na janela | só os botões da moldura (minimizar, maximizar, fechar) | **todos os botões e links da página** |
| Tamanho | 9,5 MB | 12,6 MB |
| Porta aberta na máquina | sim (127.0.0.1, com segredo, `Host` e CSRF) | **não** |
| Cara de programa | janela do Edge, com o título da página | **janela própria** |

- **Leitor de tela no D:** o Edge só monta a árvore de acessibilidade da página quando percebe um leitor de tela de verdade (o Narrador). O teste não prova que o D é inacessível, só que o E expõe tudo direto. *A conferir com o Narrador ligado.*
- **Visual:** o mesmo nos dois, porque o HTML e o CSS são os mesmos e o motor também (o WebView2 é o Edge).
- **O que o D não consegue tirar do Edge:** menu do botão direito, F12, zoom com Ctrl e roda do mouse, extensões e o agrupamento com as janelas do Edge na barra de tarefas. É uma janela do navegador, e o `--app` só esconde a barra de endereço.

**O E com cara do gerenciador de hoje (26/10/2026).** O protótipo E foi acertado para ficar como a janela em C# (`FormPrincipal.cs`):

| No C# de hoje | No E (Wails v3) | Conferido |
|---|---|---|
| Ícone do cofrinho na janela, na barra de tarefas e na bandeja | `Options.Icon` com o PNG do cofrinho (o mesmo do site) | sim, no print da janela |
| Ícone no `.exe` (Explorer) | recurso `.syso` gerado do `icone.ico` (go-winres ou `wails3 generate syso`) | **não:** a ferramenta não foi baixada no protótipo |
| Desenho: faixa do topo com o degradê e o cofrinho, ponto com halo, cartões de 448, botões de 34 px com borda, "Mais opções ▾" fora do cartão, rodapé com a pasta e o endereço | HTML e CSS com as cores do `Tema.cs` e as medidas do `Controles.cs`, sem o CSS do site; claro e escuro pelo tema do Windows | sim, comparado com o print do C# |
| Tamanho fixo, sem maximizar, no centro | `DisableResize`, `MaximiseButtonState: ButtonHidden`, `InitialPosition: WindowCentered`; largura da página 480 (448 + 16 de cada lado) | sim |
| Fechar deixa na bandeja | gancho em `WindowClosing` com `Cancel()` e `Hide()`; "Sair" no menu da bandeja encerra | sim: depois do X, o processo continua e a janela some |
| Sem nada de navegador | build com `-tags production` tira o F12 e o menu do botão direito; zoom e atalhos do navegador já vêm desligados no Wails | pelo código do Wails v3.0.0-beta.34 (`webview_window_windows.go`) |
| Glifos nos botões (`IconeBotao`, Segoe Fluent Icons / MDL2) | os mesmos códigos, com a mesma fonte do Windows no CSS, fora do leitor de tela (`aria-hidden`); no Linux, que não tem a fonte, trocar por SVG do lucide (os do site) | sim, no Windows |
| Botão em destaque: "Abrir o site" no ar, "Ligar" desligado; um botão só para ligar ou desligar | o mesmo, pela situação | sim, nos dois estados |
| Site desligado: "Abrir o site", "Reiniciar", "Ver os erros do site" e "Abrir o manual" bloqueados, com a dica "É preciso ligar o site para ver isto. Use o botão “Ligar” acima." | os mesmos, com `disabled`; a dica sai ao passar o mouse e vai para o leitor de tela pelo `aria-describedby` | os bloqueios, sim; a dica ao passar o mouse, **não** (o teste não mexe no mouse) |
| Sem barra de rolagem: a janela cresce e encolhe com o conteúdo (`AutoSize`) | a página manda a altura do conteúdo (ao abrir e ao abrir ou fechar "Mais opções"), e o gerenciador chama `SetSize`, somando a moldura (janela menos a área visível; no WebView2, o `outerHeight` é o da página) | sim: 450 px fechado, 644 aberto, sem rolagem |

- **Botões personalizados:** como a página é HTML e CSS, o botão fica do jeito que se quiser (cor, borda, canto, ícone, passar o mouse, apertado, bloqueado, foco), sem desenhar à mão como o `Botao` do WinForms (`OnPaint`). O que não muda pelo CSS é a moldura da janela (título, minimizar, fechar), que é a do Windows, como no C#. Dá para tirar com `Frameless` e desenhar a barra na página, mas aí o arrastar, o encaixe nas bordas e o leitor de tela da moldura ficam por nossa conta: *julgamento:* não vale, a do Windows é a mesma do C#.
- Tamanho: 11,1 MB. Memória privada: 199 a 203 MB em 7 processos (o gerenciador sozinho, 52 MB), com a janela aberta, três rodadas.
- Leitor de tela: a UI Automation vê todos os botões, os links e o interruptor, com os nomes limpos (os glifos ficam fora, com `aria-hidden`). A primeira consulta volta vazia: o Chromium só monta a árvore quando um leitor pergunta. O Narrador e o NVDA perguntam de novo; *a conferir com o Narrador ligado.*

## Alternativas consideradas

- **Manter em C# (opção a da #276).** Custo quase zero, mas contraria a decisão do time: o SDK do .NET continua na CI e nos scripts só por causa do gerenciador.
- **Fyne (C).** Uma biblioteca só, que desenha a janela com OpenGL. **Descartada:** a UI Automation não viu nenhum texto nem botão na janela, então o leitor de tela não lê nada, e a acessibilidade é exigência do projeto. Também é a maior e precisa de cgo nos dois sistemas.
- **Wails v3 (B e E).** **Em aberto, como opção da janela (abaixo).** Janela de verdade em HTML, desenhada pelo navegador do sistema. Acessível no Windows: a UI Automation vê todos os botões e links. Contra:
  - é versão prévia (beta), com uma versão nova quase por dia;
  - no Linux, depende do GTK 4 e do WebKitGTK 6.0 (até ~231 MB de pacotes, que a instalação teria de pedir) e precisa de cgo;
  - no Windows, 188 MB de memória privada em 7 processos (WebView2): 4 vezes a do A e 12 vezes a do C# de hoje.
- **Electron.** Levaria um Chromium e um Node em cada instalação e um terceiro runtime ao projeto: vai contra a instalação leve (revisão do épico).

## Decisão (proposta)

O gerenciador passa para Go como **ícone na bandeja mais uma página com o visual do site**, nos dois sistemas.

**Fica para o time escolher como a página aparece** (protótipos D e E, acima):
- **Opção D, página pelo Edge:** o gerenciador serve a página em `127.0.0.1` e a abre com `msedge --app` (Chromium ou navegador padrão no Linux). É a mais leve e não depende de biblioteca em versão prévia, mas a janela é do Edge e há um servidor local para proteger (abaixo).
- **Opção E, programa com Wails:** janela própria, sem porta aberta, e o leitor de tela vê a página direto. Custa ~2 vezes a memória do D com a janela aberta, depende do Wails v3, que é beta, e no Linux pede o GTK 4 e o WebKitGTK.

**Recomendação: a opção E** (26/10/2026, depois do E acertado acima). *Julgamento próprio, com a preocupação do @JulioCezarSP:* o gerenciador tem de ser um programa, igual ao de C#, e não uma página do Edge. O D não consegue tirar o menu do botão direito, o F12, o zoom nem o agrupamento com o Edge. O E tira tudo isso, não abre porta na máquina (a seção "Segurança da página local" deixa de existir) e o leitor de tela vê a página direto. O preço:
- ~200 MB de memória com a janela aberta, contra ~100 do D. Com a janela fechada, o WebView2 continua na memória, a menos que a janela seja destruída ao fechar e recriada ao abrir (*a medir na história*);
- o Wails v3 em beta (fixar a versão no `go.mod` e só subir com o teste de instalação passando);
- no Linux, GTK 4 e WebKitGTK, que a instalação pede.

A escolha final continua com o time; esta ADR só passa a "Aceita" depois dela.

O resto vale para as duas. Os pontos marcados "(D)" só valem para a opção D.

- **Bandeja:** [`fyne.io/systray`](https://github.com/fyne-io/systray), com o menu "Abrir o gerenciador", "Ligar o site", "Desligar o site", "Abrir o site" e "Sair". Sem cgo no Windows nem no Linux (conferido). No Linux, o ícone vai em PNG: o protótipo com ICO falhou ("unknown format").
- **Página local:** o mesmo HTML, com o `styles.css` do site embutido (protótipo D), nos dois sistemas, com situação, ligar e desligar, cópia do banco, versão nova e "O que mudou". Os textos são os de hoje.
  - Ela funciona com o site desligado, porque quem a serve é o gerenciador, e não o site.
  - O teclado, o leitor de tela e o contraste são os do navegador de quem usa, com os mesmos padrões do site (WCAG 2.2, `CLAUDE.md`).
- **Segurança da página local (D):** é um servidor novo na máquina, então:
  - só em `127.0.0.1`, numa porta sorteada a cada partida;
  - um segredo sorteado na URL que o gerenciador abre, trocado por um cookie `HttpOnly` e `SameSite=Strict`;
  - o cabeçalho `Host` conferido, contra *DNS rebinding*;
  - token anti-CSRF em toda ação.

  *Julgamento:* o mesmo cuidado do site (ADR 0049), num servidor menor. Detalhar na história.
- **Elevação no Windows:** o gerenciador roda sem elevação e lê a situação do serviço assim (conferido). Para ligar ou desligar, ele se reabre como administrador só para a ação (`ShellExecute` com `runas` e `-acao=ligar`, conferido).
- **Instância única:** a segunda partida abre a página da primeira, e não sobe outra. *A detalhar:* arquivo com a porta na pasta do usuário, com trava.
- **Atualização:** o GitHub Releases com SHA-256 (ADR 0028) e o develop na instalação de desenvolvimento (ADR 0040), em Go (`net/http` e `crypto/sha256`). Os domínios de saída são os mesmos de hoje.
- **Um executável ou dois:** *a decidir na história.* Um só, `financas`, com o modo `gerenciador`, poupa ~9 MB e uma compilação. Dois deixam a API sem o código da bandeja.

## Consequências

- **Positivas:**
  - instalação muito menor: o gerenciador cai de 121 MB (240 arquivos) para ~10 MB num arquivo só. A memória fica na mesma ordem da de hoje (acima);
  - sem cgo, compila para os dois sistemas de qualquer um deles;
  - no Linux, não pede GTK nem WebKit, e não precisa mais do Avalonia;
  - a página é HTML, com os mesmos padrões de acessibilidade do site, num lugar só para Windows e Linux. Hoje são duas janelas, WinForms e Avalonia, que precisam ser mantidas iguais à mão;
  - o .NET pode sair de vez na #86.
- **Negativas:**
  - **não é uma janela de programa:** é o navegador. No Windows, o `msedge --app` abre uma janela própria, com o título da página, o que atenua o problema da aba perdida. No Linux, isso só vale com o Chromium, o Chrome ou o Edge instalados; sem eles, abre numa aba do navegador padrão. O ícone na bandeja é o caminho de volta;
  - **memória com a janela aberta:** o Edge soma de 53 a 124 MB enquanto a página está aberta, perto do Wails. A diferença é que, fechada a janela, essa memória volta;
  - **mais um servidor na máquina,** ainda que só local, com as proteções acima: mais código de segurança para manter e testar;
  - **bandeja no GNOME:** depende do StatusNotifierItem. *Inferido, não testado:* o Ubuntu traz a extensão de AppIndicator ligada, e outras distribuições podem não trazer. Sem ela, o gerenciador abre pelo atalho do menu de aplicativos;
  - **reescrever ~3.600 linhas,** inclusive a atualização com SHA-256, que é sensível;
  - **não testado num Linux de verdade:** a bandeja no GNOME e no KDE e o Orca. Isso fica para a primeira história.
- **Na prática (se aceita):**
  - as histórias de implementação entram no épico #70, bloqueando a #86;
  - a #84 e a #277 passam a gerar o gerenciador em Go no `.msi` e nos scripts;
  - a 0028 e a 0033 ficam "Substituída por 0052" quando o gerenciador em Go for publicado.
