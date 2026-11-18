# 0052. Gerenciador em Go: ícone na bandeja e janela própria com o Wails

- **Status:** Aceita (#276; opção E escolhida pelo @JulioCezarSP em 27/10/2026)
- **Data:** 2026-10-25 (proposta); 2026-10-27 (aceita); 2026-10-28 (testada num Linux de verdade, #417)
- **Relacionadas:** [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) e [0033](0033-gerenciador-no-linux-com-avalonia.md) (o gerenciador em C#, que esta substituiu); [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) (atualizar pelo develop); [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o `.msi`); [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (o back-end em Go); épico #70

**Os detalhes** (como a janela imita a do C#, a bandeja, o que foi conferido no Linux, a instalação e as medições dos protótipos) estão no [README do gerenciador](../../src/gerenciador-go/README.md). Esta ADR guarda a decisão, as alternativas e o custo.

## Contexto

O gerenciador liga, desliga e acompanha o site, faz a cópia do banco e se atualiza. Era todo em C#, com cerca de 3.600 linhas: o núcleo comum (708), o de Windows em WinForms (1.956) e o de Linux em Avalonia (967).

O épico #70 remove o .NET no fim (#86). Sem tratar o gerenciador, o `.msi`, os scripts e a CI deixam de gerá-lo. **O time decidiu em 21/10/2026 que o destino é tudo em Go**, inclusive o gerenciador (comentário na #276). Faltava decidir **como** desenhar a parte com janela.

O que ele precisa continuar fazendo (lista da #276): funcionar com o site desligado, porque é ele que liga; UAC só nas ações que precisam; serviço do Windows e `systemctl --user`; bandeja e instância única; atualização pelo GitHub Releases com SHA-256, e pelo develop na instalação de desenvolvimento; os mesmos textos e ações nos dois sistemas; nada instalado à parte na máquina de quem usa; teclado, leitor de tela e contraste.

## Alternativas consideradas

Os protótipos (#276, 24 a 26/10/2026) tinham as mesmas funções (situação, ligar, desligar, ícone com menu) e foram medidos no Windows 11 e no Ubuntu 24.04 do WSL2, com o C# como referência. O resumo (tabelas completas no [README](../../src/gerenciador-go/README.md#medições-dos-protótipos-276)):

| | C# | A/D: bandeja + página pelo Edge | **E: Wails v3** | C: Fyne 2 |
|---|---|---|---|---|
| Tamanho (Windows) | 121 MB, 240 arquivos | 9,5 MB | **12,6 MB** | 23,6 MB |
| Memória privada, janela fechada | 14–15 MB | 46 MB | ~50 MB | 188 MB |
| Memória privada, janela aberta | – | ~100 MB (com o Edge) | ~200 MB (WebView2) | 188 MB |
| Precisa no Linux | – | nada | GTK 4 e WebKitGTK 6.0 | OpenGL e X11 |
| cgo para compilar | – | não | só no Linux | nos dois |
| Leitor de tela (UI Automation) | sim | só a moldura da janela do Edge | **todos os botões e links** | nada exposto |
| Porta aberta na máquina | – | sim (127.0.0.1, com segredo, `Host` e CSRF) | **não** | não |

- **Manter em C# (opção a da #276).** Custo quase zero, mas contraria a decisão do time: o SDK do .NET continuaria na CI e nos scripts só por causa do gerenciador.
- **Fyne (C).** Uma biblioteca só, que desenha a janela com OpenGL. **Descartada:** a UI Automation não viu nenhum texto nem botão, então o leitor de tela não lê nada, e a acessibilidade é exigência do projeto. Também é a maior e precisa de cgo nos dois sistemas.
- **Bandeja com a página pelo Edge (A e D).** O gerenciador serve a página em `127.0.0.1` e a abre com `msedge --app`. É o mais leve com a janela aberta e não depende de biblioteca em versão prévia nem, no Linux, de GTK. **Descartada:** a janela é do Edge (menu do botão direito, F12, zoom, extensões, agrupamento com o Edge na barra de tarefas), e é mais um servidor na máquina para proteger. O gerenciador tem de ser um programa.
- **Electron.** Levaria um Chromium e um Node em cada instalação e um terceiro runtime ao projeto: vai contra a instalação leve.

**Na memória, nenhuma opção ganha do C#:** o runtime do Go deixa mais memória reservada (46 MB contra 14–15 MB de memória privada). A vantagem do Go é o tamanho e não depender do .NET.

## Decisão

O gerenciador passa para Go como **um programa com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) (protótipo E): ícone na bandeja e janela própria em HTML**, desenhada pelo navegador do sistema (WebView2 no Windows, WebKitGTK no Linux), nos dois sistemas.

- **Janela igual à do C#** (`FormPrincipal.cs`): as mesmas cores, medidas, glifos, textos e bloqueios com o site desligado; tamanho fixo, sem barra de rolagem, sem F12 nem menu do botão direito. O teclado, o leitor de tela e o contraste seguem os padrões do site (WCAG 2.2, [docs/UI-UX.md](../UI-UX.md)).
- **Fechar destrói a janela** (e os processos do WebView2); a bandeja cria outra ao abrir. Ao iniciar com o sistema, começa sem janela. **No Linux, fechar esconde a janela** (#417), porque destruída ela deixa o `WebKitWebProcess` vivo, ~58 MB a mais a cada vez.
- **Sem porta aberta:** o HTML e as ações são servidos por dentro do programa (`AssetOptions.Handler` do Wails).
- **Bandeja e instância única:** as do próprio Wails. O aviso na bandeja (#448: o site caiu, versão nova) é feito pelo próprio gerenciador, sem dependência nova: no Windows, o balão no ícone do Wails; no Linux, o `notify-send`.
- **Elevação:** no Windows, o gerenciador roda sem elevação e se reabre como administrador só para a ação (`ShellExecute` com `runas`). No Linux (#417), o serviço é a unidade `financaslocal.service` do `systemd --user`, e tudo roda no próprio gerenciador, sem senha.
- **Ícone no `.exe`:** recurso `.syso` gerado pelo go-winres **na CI** (#435), guardado no repositório; ninguém roda o go-winres na própria máquina (código baixado de fora).
- **Atualização:** o GitHub Releases com SHA-256 (ADR 0028) e o develop na instalação de desenvolvimento (ADR 0040), em Go (`net/http` e `crypto/sha256`). Os domínios de saída são os mesmos.
- **Wails em versão prévia:** a versão fica fixa no `go.mod` e só sobe num PR próprio, com o teste de instalação passando nos dois sistemas. Se o v3 for abandonado ou travar, a saída é a opção D, que reaproveita a mesma página.
- **Dois executáveis** (#418): o gerenciador é um módulo Go próprio, em `src/gerenciador-go/`, para a API continuar sem o Wails e sem cgo. O custo é uma compilação a mais.
- **Windows primeiro:** a pedido do @JulioCezarSP (27/10/2026), a #418 começou pelo Windows sem esperar o teste no Linux (#417), que passou a bloquear a #86. O Wails serviu no Linux, com ressalvas.
- **Cópia do banco no próprio gerenciador** (#419), pelo `modernc.org/sqlite` da API. Custa ~5,6 MB no `.exe`; chamar o executável da API pouparia o tamanho, mas amarraria o gerenciador à versão da API instalada.

## Consequências

- **Positivas:**
  - instalação muito menor: o gerenciador cai de 121 MB (240 arquivos) para ~17 MB num arquivo só;
  - um programa de verdade, com a mesma cara do anterior, e não uma página do navegador;
  - nenhum servidor novo na máquina;
  - uma janela só em HTML para os dois sistemas, em vez de WinForms e Avalonia mantidos iguais à mão; e o .NET pôde sair na #86.
- **Negativas:**
  - **memória com a janela aberta:** ~200 MB no Windows e ~250 MB no Linux, contra 14–15 MB de memória privada do C#. Fechada a janela, volta a ~50 MB no Windows; no Linux, depois de abrir uma vez, fica em ~245 MB (a janela é escondida, não destruída);
  - **Wails v3 é versão prévia**, com versões novas quase todo dia: risco de quebra ao subir e de mudança de API;
  - **Linux pede mais coisa:** o GTK 4 e o WebKitGTK 6.0 (até ~231 MB de pacotes, que a instalação tem de pedir) e cgo para compilar. No Windows compila sem cgo;
  - **no Linux, com ressalvas** (#417, testado no KDE): a dica do ícone não acompanha a situação, e o ícone da janela vem do atalho `.desktop`. O GNOME não foi testado, e a bandeja nele depende da extensão de AppIndicator (*inferido*);
  - **o aviso na bandeja do Windows depende de como o Wails monta o ícone por dentro:** se isso mudar, o aviso só deixa de aparecer (fica no log);
  - **reescrever ~3.600 linhas,** inclusive a atualização com SHA-256, que é sensível.
- **Na prática:**
  - o Wails e o go-winres entram na lista de bibliotecas da [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), só para o gerenciador;
  - o `.msi` (#277) e os scripts (#430 no Windows, #277 no Linux) instalam o gerenciador em Go, com o nome `Financas.Gerenciador.exe` no Windows, para os atalhos e o "Iniciar com o Windows" de quem atualiza continuarem valendo;
  - compilar, testar e os detalhes de cada parte: [README do gerenciador](../../src/gerenciador-go/README.md).

## Atualizações (02 e 03/11/2026: o gerenciador em Go instalado, e o C# fora)

O passo a passo de cada uma está no [README do gerenciador](../../src/gerenciador-go/README.md#instalação).

- **O `.msi` instala o gerenciador em Go** (#277, parte 2). Sem o WebView2, a instalação **não para** (o site funciona sem o gerenciador), e a tela final manda para a página da Microsoft. *Alternativa descartada:* levar o instalador do WebView2 dentro do `.msi`, que baixa da Microsoft (mais uma saída de rede e um instalador de terceiros dentro do nosso) para um caso raro.
- **Os scripts do Linux** (#277, parte 3) **e do Windows** (#430) compilam e instalam o gerenciador em Go. Ganhou também o "Atualizar o gerenciador e reabrir", como o do C#.
- **O gerenciador em C# saiu** (#86, parte 1): a [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) e a [0033](0033-gerenciador-no-linux-com-avalonia.md) ficaram "Substituída por 0052". Quem precisar dele usa a tag `v-ultimo-dotnet`. No Linux, sem o compilador de C, o GTK 4 ou o WebKitGTK 6.0, a instalação diz o comando da distribuição e segue **sem gerenciador**, com o aviso; o site funciona igual. *Julgamento:* não falhar a instalação por causa da janela.
