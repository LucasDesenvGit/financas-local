# 0052. Gerenciador em Go: ícone na bandeja e janela própria com o Wails

- **Status:** Aceita (#276; opção E escolhida pelo @JulioCezarSP em 27/10/2026)
- **Data:** 2026-10-25 (proposta); 2026-10-27 (aceita)
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
- **Bandeja com a página pelo Edge (A e D).** O gerenciador serve a página em `127.0.0.1` e a abre com `msedge --app`. É o mais leve com a janela aberta (~100 MB, contra ~200 do Wails), não depende de biblioteca em versão prévia e, no Linux, não pede GTK nem WebKit. **Descartada:** a janela é do Edge, e não dá para tirar o menu do botão direito, o F12, o zoom, as extensões nem o agrupamento com o Edge na barra de tarefas; e é mais um servidor na máquina para proteger (porta sorteada, segredo na URL, `Host` conferido, CSRF). O gerenciador tem de ser um programa, igual ao de C#.
- **Electron.** Levaria um Chromium e um Node em cada instalação e um terceiro runtime ao projeto: vai contra a instalação leve (revisão do épico).

## Decisão

O gerenciador passa para Go como **um programa com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) (protótipo E): ícone na bandeja e janela própria em HTML**, desenhada pelo navegador do sistema (WebView2 no Windows, WebKitGTK no Linux), nos dois sistemas.

- **Janela igual à de hoje** (tabela "O E com cara do gerenciador de hoje", acima): o desenho do `FormPrincipal.cs` (cores do `Tema.cs`, medidas do `Controles.cs`), os mesmos glifos, os mesmos bloqueios com o site desligado, tamanho fixo no centro, sem barra de rolagem (a janela acompanha o conteúdo), sem F12 nem menu do botão direito (`-tags production`). Os textos são os de hoje.
  - No Linux, os glifos vão em SVG do lucide (os do site), porque a Segoe Fluent Icons é do Windows.
  - O teclado, o leitor de tela e o contraste seguem os padrões do site (WCAG 2.2, `CLAUDE.md`).
- **Fechar destrói a janela** (e os processos do WebView2); a bandeja cria outra ao abrir. Parado, o gerenciador fica em ~50 MB (medido acima). Ao iniciar com o sistema, começa sem janela.
- **Sem porta aberta:** o HTML e as ações são servidos por dentro do programa (`AssetOptions.Handler` do Wails), e não por um servidor na máquina.
- **Bandeja e instância única:** as do próprio Wails (`SystemTray`, `SingleInstanceOptions`), com o menu "Abrir o gerenciador", "Ligar o site", "Desligar o site", "Abrir o site" e "Sair". No Linux, o ícone vai em PNG.
- **Ícone no `.exe`:** recurso `.syso` gerado do `icone.ico` de hoje (`wails3 generate syso` ou go-winres), na CI.
- **Elevação no Windows:** o gerenciador roda sem elevação e lê a situação do serviço assim (conferido). Para ligar ou desligar, ele se reabre como administrador só para a ação (`ShellExecute` com `runas` e `-acao=ligar`, conferido).
- **Atualização:** o GitHub Releases com SHA-256 (ADR 0028) e o develop na instalação de desenvolvimento (ADR 0040), em Go (`net/http` e `crypto/sha256`). Os domínios de saída são os mesmos de hoje.
- **Wails em versão prévia:** a versão fica fixa no `go.mod` e só sobe num PR próprio, com o teste de instalação passando nos dois sistemas. Se o v3 for abandonado ou travar, a saída é a opção D (acima), que reaproveita a mesma página.
- **Um executável ou dois: dois** (decidido na #418). O gerenciador é um programa à parte, num módulo Go próprio, em `src/gerenciador-go/`, com o `go.mod` dele:
  - a API continua sem o Wails e sem cgo, e o `go.mod` dela sem as dependências do Wails;
  - o `go vet ./...` e o `go test ./...` da API no Linux da CI não precisam do GTK 4 nem do WebKitGTK;
  - o custo é uma compilação a mais, e o job de Go da CI passa a cobrir também esse módulo (PR `ci:` à parte).
  A montagem da janela fica em `internal/tela`, sem o Wails, e é testada sem janela.
- **Windows primeiro:** a pedido do @JulioCezarSP (27/10/2026), a #418 começa pelo Windows sem esperar o teste no Linux (#417). A #417 passou a bloquear a #86 (remover o .NET); se o Wails não servir no Linux, esta ADR volta para decisão.
- **Cache do WebView2:** em `%LOCALAPPDATA%\FinancasLocal\Gerenciador\WebView2`. A pasta de dados do site (`%ProgramData%\FinancasLocal\dados`) é do serviço, e o gerenciador roda sem administrador.

## Consequências

- **Positivas:**
  - instalação muito menor: o gerenciador cai de 121 MB (240 arquivos) para ~11 MB num arquivo só;
  - um programa de verdade, com a mesma cara do de hoje, e não uma página do navegador;
  - nenhum servidor novo na máquina;
  - a janela é HTML, num lugar só para Windows e Linux. Hoje são duas janelas, WinForms e Avalonia, que precisam ser mantidas iguais à mão;
  - o .NET pode sair de vez na #86.
- **Negativas:**
  - **memória com a janela aberta:** ~200 MB (o WebView2), contra 14–15 MB de memória privada do C#. Fechada a janela, volta a ~50 MB;
  - **Wails v3 é versão prévia**, com versões novas quase todo dia: risco de quebra ao subir e de mudança de API;
  - **Linux pede mais coisa:** o GTK 4 e o WebKitGTK 6.0 (até ~231 MB de pacotes, que a instalação tem de pedir) e cgo para compilar. No Windows compila sem cgo (conferido);
  - **não testado num Linux de verdade:** no WSLg, a página do Wails não carregou (é do ambiente, mas fica sem prova). A bandeja no GNOME e no KDE e o Orca também. Isso é a primeira história; se o Wails não servir no Linux, volta para cá;
  - **bandeja no GNOME:** depende do StatusNotifierItem. *Inferido, não testado:* o Ubuntu traz a extensão de AppIndicator ligada, e outras distribuições podem não trazer;
  - **reescrever ~3.600 linhas,** inclusive a atualização com SHA-256, que é sensível.
- **Na prática:**
  - as histórias de implementação entram no épico #70, bloqueando a #86, e a primeira prova o Wails num Linux de verdade;
  - a #84 e a #277 passam a gerar o gerenciador em Go no `.msi` e nos scripts;
  - o Wails e o go-winres entram na lista de bibliotecas da [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), só para o gerenciador;
  - a 0028 e a 0033 ficam "Substituída por 0052" quando o gerenciador em Go for publicado.
