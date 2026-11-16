# Gerenciador em Go

O gerenciador do site em Go, com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) ([ADR 0052](../../docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md), épico #70). O de C# saiu na #86 (fica na tag `v-ultimo-dotnet`). **O `.msi` (#277) e os scripts (#430 no Windows, #277 no Linux) instalam este**; no Windows, com o nome `Financas.Gerenciador.exe`.

| Pronto | Falta |
|---|---|
| Janela igual à do `FormPrincipal.cs`, ícone na bandeja, uma cópia só, `--bandeja`, fechar destrói a janela (#418) | |
| Situação do site (de 5 em 5 s, na janela e no ícone da bandeja), ligar, desligar e reiniciar pelo UAC, cópia do banco, preferências (#419) | |
| Versão nova pelo GitHub Releases: procurar, "O que mudou", baixar conferindo o SHA-256 e abrir o instalador; "Receber versões de desenvolvimento" (#420, ADR 0053) |
| "Atualizar para a última do develop" na instalação de desenvolvimento pelos scripts, com a saída ao vivo (#428, ADR 0040), e "O que mudou" depois, pelo número e pelas seções que o clone calcula (`go run ./cmd/versao`, ADR 0070) e pelo `CHANGELOG.md` dele (#432); "Atualizar o gerenciador e reabrir" quando a reinstalação dá certo (#430) | |
| Linux (#417): instalado pelo `scripts/linux/instalar-gerenciador.sh` com o atalho `br.financaslocal.Gerenciador.desktop` (#277); `systemctl --user` sem senha, tema pelo portal, "Iniciar com o sistema" (`~/.config/autostart`), registro do serviço, comando para atualizar, "sem login" (linger), ícones em SVG | |
| Aviso na bandeja com a janela fechada: o site caiu, versão nova (#448) | |
| "Restaurar uma cópia…" em Mais opções: confere o arquivo, guarda o banco de agora em `dados/antes-de-restaurar`, troca e liga de novo; no Windows, pelo UAC (`--restaurar`) (#533) | |

## Organização

- `main.go`: a entrada, e as ações que o próprio gerenciador roda como administrador (`--servico ligar|desligar|reiniciar`, `--backup "destino"`, `--restaurar "cópia"`, `--instalar-pelos-scripts`), como o `Program.cs`. `--situacao` escreve a situação e sai (0 = no ar), sem janela, como o do Linux em C# (#277).
- `gerenciador.go`: o Wails (janela, bandeja, instância única), a consulta da situação e as rotas da página, servidas por dentro do programa, **sem porta aberta** (`AssetOptions.Handler`).
- `sistema_windows.go`, `sistema_linux.go`: o que muda entre os dois. No Windows, ligar, desligar, copiar o banco e reinstalar pedem o UAC; no Linux, rodam no próprio gerenciador, e fechar a janela só a esconde (ADR 0052, #417).
- `foco_windows.go`, `foco_linux.go`: abrir o gerenciador de novo traz a janela que já está aberta para a frente. No Windows, a segunda cópia passa à primeira o direito de vir para a frente (`AllowSetForegroundWindow`) antes de avisá-la e fechar; no Linux, não precisa.
- `internal/tela`: a página (`pagina.html`, `estilo.css`) e o que ela mostra em cada situação. Não depende do Wails: os testes rodam sem janela.
- `internal/instalacao`: onde estão a API, o banco, os dados e o endereço (o `Instalacao.cs`), pelos mesmos `appsettings`.
- `internal/situacao`: o serviço, o `GET /api/auth/estado` (o `VerificadorSituacao`) e o `GET /api/celular/estado` (#679: o cartão "Acesso pelo celular").
- `internal/acessocelular`: liga e desliga o acesso pelo celular (#684), com o segredo da pasta das chaves; no Windows, elevado (`--celular`).
- `internal/servico`, `internal/elevacao`: o serviço do Windows e o UAC só para a ação; no Linux, a unidade `financaslocal.service` do `systemd --user`, o linger e o registro.
- `internal/comando`: roda os programas do sistema (`systemctl`, `loginctl`, `journalctl`, `busctl`), como o `Comando.cs`.
- `internal/copia`: a cópia do banco pelo `VACUUM INTO` (`modernc.org/sqlite`, o mesmo da API).
- `internal/restauracao`: o "Restaurar uma cópia" (#533), com os passos do `docs/OPERACAO.md`: conferir a cópia antes de desligar o site, guardar o banco de agora, trocar sem o `-wal` e o `-shm` antigos e devolver o de antes se algo falhar no meio.
- `internal/atualizacao`: a versão nova pelo GitHub Releases (o `Atualizacoes.cs`): procurar, baixar conferindo o SHA-256, abrir o `.msi` e as notas separadas e em HTML seguro. `versao_nova.go` liga isso à janela.
- `internal/desenvolvimento`: o "Atualizar para a última do develop" (ADR 0040): reconhecer o clone na `develop`, rodar os scripts com a saída ao vivo e acompanhar o log do instalador. `develop.go` liga isso à janela.
- `internal/icones`: o ícone da bandeja com o ponto da cor da situação.
- `internal/preferencias`: o tema do Windows e as preferências do C# (`HKCU`); no Linux, o atalho em `~/.config/autostart` e o tema pelo portal do freedesktop.

É um módulo à parte (`go.mod` próprio), para a API não levar o Wails nem precisar de cgo no Linux.

## Compilar e testar no Linux

Pede cgo, o GTK 4 e o WebKitGTK 6.0 (os `-dev` no Ubuntu: `libgtk-4-dev libwebkitgtk-6.0-dev`; no Arch, `webkitgtk-6.0`):

```bash
cd src/gerenciador-go
go vet ./... && go test -race ./...
CGO_ENABLED=1 go build -tags production -trimpath -ldflags "-s -w -X main.versao=X.Y.Z" -o gerenciador-go .
./gerenciador-go -mostrar=desligado -mais-opcoes
```

- O ícone da janela vem do atalho `br.financaslocal.Gerenciador.desktop` (em `~/.local/share/applications`); sem ele, aparece o do Wails.
- **"Iniciar com o sistema" usa o mesmo atalho do C#** (`~/.config/autostart/financaslocal-gerenciador.desktop`): ligar aqui troca o que abre com a sessão.

## Compilar e testar (Windows)

```bash
cd src/gerenciador-go
go vet ./... && go test ./...
CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.versao=X.Y.Z" -o gerenciador-go.exe .
```

- `-tags production` tira o F12 e o menu do botão direito; `-H=windowsgui`, a janela do console.
- Para comparar com o C# sem mexer no serviço: `gerenciador-go.exe -mostrar=desligado -mais-opcoes` (ou `no-ar`, `ligando`, `sem-resposta`, `nao-instalado`). Sem `-mostrar`, a situação é a de verdade.
- **"Iniciar com o Windows" usa a mesma entrada do C#:** ligar aqui faz o Windows abrir o gerenciador em Go no lugar do de C#.
- **Ícone e manifesto do `.exe`:** o `rsrc_windows_amd64.syso` (o cofrinho do `icone.ico`, o manifesto "gui": controles comuns v6 e DPI por monitor, e o nome "Finanças Local", o do topo das notificações), que o `go build` junta sozinho. Ele é gerado pelo [go-winres](https://github.com/tc-hib/go-winres) na CI (job "gerenciador em Go"), que confere se o do repositório está em dia; mudou o `icone.ico`, baixe o artefato `icone-gerenciador` da CI e suba o `.syso` novo.
- O cache do WebView2 fica em `%LOCALAPPDATA%\FinancasLocal\Gerenciador\WebView2`, fora da pasta de dados do site (`%ProgramData%\FinancasLocal\dados`), que é do serviço: o gerenciador roda sem administrador.
- **Elevação:** o gerenciador roda sem elevação e lê a situação do serviço assim. Para ligar, desligar, copiar ou restaurar, ele se reabre como administrador só para a ação (`ShellExecute` com `runas` e a opção da ação, como `--servico ligar`).
- **Nome no topo da notificação:** o `.syso` leva `--file-description` e `--product-name` "Finanças Local" (sem eles, o Windows mostrava "gerenciador.exe"). O go-winres é o v0.3.3 (`simply --manifest gui`), sem número de versão no recurso, para o `.syso` não mudar a cada PR. Ninguém roda o go-winres na própria máquina (código baixado de fora).

## A janela, como a do C# (ADR 0052)

A janela imita a do `FormPrincipal.cs` do gerenciador em C# (tag `v-ultimo-dotnet`), conferida com prints lado a lado (#276, #418):

| No C# | No Go (Wails v3) |
|---|---|
| Ícone do cofrinho na janela, na barra de tarefas e na bandeja | `Options.Icon` com o PNG do cofrinho (o mesmo do site); no `.exe`, o `.syso` (acima) |
| Faixa do topo com o degradê e o cofrinho, ponto com halo, cartões de 448, botões de 34 px com borda, "Mais opções ▾" fora do cartão, rodapé com a pasta e o endereço | HTML e CSS com as cores do `Tema.cs` e as medidas do `Controles.cs`, sem o CSS do site; claro e escuro pelo tema do sistema |
| Tamanho fixo, sem maximizar, no centro | `DisableResize`, `MaximiseButtonState: ButtonHidden`, `InitialPosition: WindowCentered`; largura da página 480 (448 + 16 de cada lado) |
| Fechar deixa na bandeja | gancho em `WindowClosing` com `Cancel()` e `Hide()`; "Sair" no menu da bandeja encerra |
| Sem nada de navegador | `-tags production` tira o F12 e o menu do botão direito; zoom e atalhos do navegador já vêm desligados no Wails |
| Glifos nos botões (Segoe Fluent Icons / MDL2) | os mesmos códigos, com a mesma fonte, fora do leitor de tela (`aria-hidden`); no Linux, que não tem a fonte, SVG do lucide (os do site), na cor do texto |
| Botão em destaque: "Abrir o site" no ar, "Ligar" desligado; um botão só para ligar ou desligar | o mesmo, pela situação |
| Site desligado: "Abrir o site", "Reiniciar", "Ver os erros do site" e "Abrir o manual" bloqueados, com a dica "É preciso ligar o site para ver isto. Use o botão “Ligar” acima." | os mesmos, com `disabled`; a dica sai ao passar o mouse e vai para o leitor de tela pelo `aria-describedby` |
| Sem barra de rolagem: a janela cresce e encolhe com o conteúdo (`AutoSize`) | a página manda a altura do conteúdo (ao abrir e ao abrir ou fechar "Mais opções"), e o gerenciador chama `SetSize`, somando a moldura (no WebView2, o `outerHeight` é o da página): 450 px fechado, 644 aberto |

- **Botões:** como a página é HTML e CSS, o botão fica do jeito que se quiser (cor, borda, canto, ícone, passar o mouse, apertado, bloqueado, foco), sem desenhar à mão como o `Botao` do WinForms. A moldura da janela (título, minimizar, fechar) é a do sistema, como no C#. Dá para tirá-la com `Frameless` e desenhar a barra na página, mas aí o arrastar, o encaixe nas bordas e o leitor de tela da moldura ficam por nossa conta: *julgamento:* não vale.
- **Leitor de tela:** a UI Automation (Windows) e o AT-SPI (Linux) veem todos os botões, os links e o interruptor, com os nomes limpos. A primeira consulta volta vazia: o Chromium só monta a árvore quando um leitor pergunta, e o Narrador e o NVDA perguntam de novo.
- **Fechar a janela:** no Windows, destrói (o WebView2 sai junto), e parado na bandeja o gerenciador fica em ~50 MB. No Linux, esconde: no Wails v3.0.0-beta.34 com o WebKitGTK 6.0, a janela destruída deixa o `WebKitWebProcess` dela vivo, e cada abrir e fechar somava ~58 MB (316 → 374 → 432 MB); escondida, a mesma volta e fica em ~242 MB. Vale abrir uma issue no Wails.

## A bandeja

- **Menu:** no Windows, "Abrir o gerenciador", "Ligar o site", "Desligar o site", "Abrir o site" e "Sair". No Linux (StatusNotifierItem), a situação na primeira linha, "Abrir o site", "Mostrar o gerenciador" e "Sair"; o ícone vai em PNG, com o ponto da cor da situação.
- **Clique:** no Linux não há duplo clique (no StatusNotifierItem, o clique simples é "ativar" e mostra a janela, como no C# do Linux). A janela aberta pela bandeja pode ficar atrás da ativa, pela proteção do KWin contra roubo de foco.
- **Dica do ícone no Linux:** no Wails v3.0.0-beta.34 ela não está feita (`setTooltip` vazio). O ícone mostra "Finanças Local" (pelo `SetLabel`; sem ele, "Wails"), e a situação fica na primeira linha do menu e na cor do ponto.
- **Sem bandeja (GNOME sem a extensão de AppIndicator):** começando com o sistema (`--bandeja`), o gerenciador pergunta ao D-Bus se há quem mostre o ícone (`org.kde.StatusNotifierWatcher`) e, sem ninguém, abre a janela minimizada, como o C# do Linux (conferido num D-Bus à parte, sem bandeja).
- **Aviso na bandeja** (#448), como o `ShowBalloonTip` do C#: com a janela fechada, quando o site que estava no ar desliga ou para de responder, e quando a procura acha versão nova. O Wails v3.0.0-beta.34 não tem balão, e o serviço de notificações dele traria uma dependência nova (`go-toast`). No Windows, o gerenciador manda o balão (`Shell_NotifyIconW` com `NIF_INFO`) ao próprio ícone do Wails: a janela só de mensagens dele, achada pela classe própria (`WndClass`) e pelo processo, com o id 1 (a única bandeja). Se uma versão nova do Wails mudar esse ícone por dentro, o aviso só deixa de aparecer (fica no log). No Linux, o `notify-send`; sem ele, não avisa.

## Linux: o que foi conferido (#417, 28/10/2026)

Numa máquina de verdade: CachyOS (Arch), KDE Plasma 6.8 em Wayland, GTK 4.24.1 e WebKitGTK 6.0 2.54.2, com o site instalado pelos scripts. As conferências à mão (Orca, Tab, copiar com o mouse, cópia do banco e a bandeja no painel) foram feitas pelo FelipeJun no mesmo dia. O Ubuntu 24.04 com GNOME **não foi testado**; o roteiro da #417 continua valendo para ele.

| | Resultado | Como foi conferido |
|---|---|---|
| Janela | a página inteira, nos temas claro e escuro (o do sistema, pelo portal do freedesktop), com a altura acompanhando o conteúdo | prints; o tema trocado no KDE (BreezeLight × BreezeDark) |
| Ícone na barra de título | no GTK 4 vem do atalho `.desktop` com o nome do id do aplicativo (o Wails não consegue pôr: `setIcon` vazio). Com o id `br.financaslocal.Gerenciador` e o atalho com esse nome, aparece o cofrinho; sem ele, o "W" do Wails | print, com um atalho de teste |
| Bandeja (KDE) | no painel, com o cofrinho e o ponto da situação; "Sair" encerra tudo | StatusNotifierItem e dbusmenu pelo D-Bus; à mão |
| Ligar, desligar, reiniciar | pelo `systemctl --user`, sem senha; o site voltou em ~5 s | o botão apertado pelo AT-SPI; o horário de início da unidade mudou |
| Situação | a do serviço e a do site, igual à do Windows | com o site de verdade |
| Registro do serviço, comando para atualizar, "sem login" | o journald do usuário no diálogo das notas; "Copiar o comando" cola no terminal; o linger lido pelo `loginctl` | AT-SPI; a cópia, à mão |
| Cópia do banco | o diálogo de salvar do GTK 4 abre e a cópia é salva, sem senha | à mão |
| Leitor de tela (Orca) | os landmarks ("Situação do site", "Versão"), a situação como `status`, os botões com o nome limpo, o campo do comando com rótulo, o interruptor como botão de alternar; o Tab percorre os botões com o foco visível | AT-SPI; ouvido com o Orca |
| Instância única | abrir de novo mostra a janela da cópia que já roda, e a segunda sai | duas vezes |
| Compilar | com cgo, 16 s nesta máquina, sem cache; na CI (Ubuntu 24.04, #440), pacotes 20 s, `go vet` 53 s, `go test -race` 66 s e compilação 45 s (3 min 20 s o job) | relógio e os passos do job |
| Tamanho e bibliotecas | 16 MB num arquivo só (17 MB no Windows); pede `libwebkitgtk-6.0`, `libgtk-4`, `libjavascriptcoregtk-6.0`, `libsoup-3.0`, pango, cairo, harfbuzz, gdk-pixbuf, graphene, vulkan, X11 e a glib | `du`, `objdump -p` |
| Pacotes a instalar | no KDE, só o `webkitgtk-6.0` (38,5 MiB para baixar, 138,6 MiB instalado); no Ubuntu sem nada, até 113 pacotes e ~231 MB (medido no WSL, #276) | `pacman` |
| Memória | **51 MB** (PSS) só na bandeja; **~245 a 260 MB** com a janela, em 8 processos (o gerenciador, o `WebKitWebProcess`, o `WebKitNetworkProcess` e o sandbox: `bwrap`, `xdg-dbus-proxy`) | `smaps_rollup`, três leituras |

**Copiar no Wayland** só vale depois de um clique ou tecla de verdade na janela: com o mouse, copia; pelo AT-SPI, a janela diz "Copiado" e nada é copiado.

## Instalação

- **`.msi`** (#277, parte 2): o `instalador/gerar-msi.ps1` compila este módulo sem cgo (`-tags production`, `-H=windowsgui`, a versão pelo `-ldflags`) com o nome `Financas.Gerenciador.exe`. A CI confere que nada do C# sobrou na pasta e mede o `.msi` e a pasta instalada. O `.msi` procura o WebView2 nas duas chaves da [documentação da Microsoft](https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution#detect-if-a-webview2-runtime-is-already-installed) (o valor `pv`, para a máquina ou para quem usa, diferente de `0.0.0.0`); sem ele, a instalação segue e a tela final manda para a [página da Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/consumer/), um dos dois caminhos que a Microsoft recomenda para os poucos Windows 10 sem ele.
- **Scripts do Windows** (#430): o `scripts/windows/instalar-gerenciador.ps1` compila sem cgo, com o Go que a instalação baixou em `ferramentas\go` e o mesmo ambiente do `instalar-windows.ps1`, em `publicado\gerenciador`, com o nome `Financas.Gerenciador.exe`. Numa pasta escolhida com `-Destino`, nada é apagado, só o aviso.
- **Scripts do Linux** (#277, parte 3): o `scripts/linux/instalar-gerenciador.sh` compila com cgo em `publicado/gerenciador-go`, põe o `icone.png` ao lado (o "Iniciar com o sistema" aponta para ele) e cria o atalho `br.financaslocal.Gerenciador.desktop`. Sem o compilador de C, o `pkg-config`, o GTK 4 ou o WebKitGTK 6.0, diz o que falta e o comando da distribuição (Debian/Ubuntu, Fedora, Arch; nas outras, o que procurar) e para sem instalar gerenciador; o site segue funcionando, e o `instalar.sh` mostra o aviso. No fim, avisa quando o *linger* está desligado (o site só sobe com a sessão aberta).
- **`--situacao`** é o que a CI do Linux usa, porque o runner não tem tela.
- **Quem tinha o gerenciador em C#** (#86) tem tudo trocado na próxima atualização: o atalho do menu sai, o da área de trabalho e o "Iniciar com o sistema" passam a abrir este, e a pasta `publicado/gerenciador` sai (no Linux). A CI do Linux confere isso com um gerenciador antigo de mentira.
- **"Atualizar o gerenciador e reabrir"** (#430), como o `AtualizarGerenciadorEReabrir` do C#: aparece depois de a reinstalação pelo develop dar certo. O gerenciador deixa um comando rodando à parte (no Windows, o PowerShell numa janela própria, que mostra o andamento e, se falhar, fica aberta com o aviso; no Linux, `setsid bash -c`), fecha, e o comando roda o instalador do gerenciador do clone e o abre de novo. Os comandos ficam em `internal/desenvolvimento/reabrir.go`, puros e testados.

## Medições dos protótipos (#276)

Os protótipos que levaram à ADR 0052, com as mesmas funções (situação, ligar, desligar, ícone com menu) e o C# como referência. No Windows 11: tamanho no disco, memória privada do processo e dos filhos, ícone na bandeja (`NotifyIconSettings`), árvore de acessibilidade pela UI Automation, ordem do Tab e UAC de verdade. No Linux: Ubuntu 24.04 no WSL2, com o WSLg.

<details>
<summary>As tabelas e as notas de cada rodada</summary>

**24 e 25/10/2026:**

| | C# | A: Go + bandeja + página local | B: Wails v3 | C: Fyne 2 |
|---|---|---|---|---|
| Tamanho (Windows) | 121 MB, 240 arquivos (self-contained) | 9,4 MB, 1 arquivo | 12,5 MB | 23,6 MB |
| Tamanho (Linux) | – | 9,6 MB, estático | 13 MB | 24 MB |
| Memória privada (Windows) | 14–15 MB | 46 MB | 188,5 MB (7 processos do WebView2) | 188 MB |
| Memória em uso, *working set* (Windows) | 56–59 MB | 14–53 MB | – | – |
| Memória (Linux, WSLg) | – | 14 MB | 133 MB (a página não carregou) | 185 MB |
| cgo para compilar | – | não (`CGO_ENABLED=0` nos dois) | no Linux | sim, nos dois (4 a 6,5 min) |
| Precisa na máquina, no Linux | – | nada | GTK 4 e WebKitGTK 6.0: 113 pacotes, ~231 MB se faltarem | OpenGL e X11: 12 pacotes, ~16 MB |
| Ícone na bandeja (Windows) | sim | sim | sim | sim |
| Leitor de tela (UI Automation) | sim | a página no navegador; o menu do ícone é do sistema | texto e botões expostos; o Tab passa por Ligar → Desligar | nada exposto |
| UAC (ligar e desligar) | sim | pediu o "Sim", ligou em 4 s e desligou em 2 s | o mesmo código do A | o mesmo código do A |
| Situação da biblioteca | – | [`fyne.io/systray`](https://github.com/fyne-io/systray), push em 24/10/2026 | versão prévia: v3.0.0-beta.34 em 20/10/2026 | estável |

Medidos lado a lado, duas vezes: o C# fica com 14–15 MB de memória privada, e o A com 46 MB, porque o runtime do Go deixa mais memória reservada; em uso de fato (*working set*), o A oscila entre 14 e 53 MB, e o C# fica perto de 58 MB.

**O visual (protótipo D, 26/10/2026):** o A com o `styles.css` do site embutido (as classes `.botao`, `.botao-primario`, `.ponto`, `.marca`) e o tema do "Modo de aplicativo" do Windows, desenhado pelo Edge sem janela (`--headless --screenshot`) nos dois temas, ficou igual ao site. As cores do gerenciador em C# eram uma cópia à mão das do site e já se desencontravam (linha `#e2dfee` × `#e8e6f1`, positivo `#2a9a6a` × `#1b7550`, negativo `#d2546b` × `#b23a52`). Aberto com `msedge --app`, a UI Automation achou uma janela própria do Edge (não confirmado se a barra de endereço some: ela listou um campo "Barra de pesquisa e endereços"). Com a janela aberta, o gerenciador fica em 46–48 MB, e o Edge soma 53 MB (já aberto, 3 processos) ou 124 MB (fechado, 5 processos). O leitor de tela e o Tab dentro da página não foram testados.

**D × E (26/10/2026), a mesma página, três rodadas de cada no Windows 11, sem teclas e sem captura de tela:**

| | D: página pelo Edge | E: programa com Wails |
|---|---|---|
| Janela aparece em | 0,5 a 1 s | 0,7 a 1,3 s |
| Memória privada com a janela aberta | ~100 MB (gerenciador 48 + Edge 52, com o Edge já aberto; ~170 com ele fechado) | ~195 a 204 MB (gerenciador 52 + WebView2, 7 processos) |
| O que a UI Automation vê | só os botões da moldura | todos os botões e links da página |
| Tamanho | 9,5 MB | 12,6 MB (11,1 MB já com a janela do C#; 199 a 203 MB de memória com ela aberta) |
| Porta aberta | sim (127.0.0.1, com segredo, `Host` e CSRF) | não |
| Cara de programa | janela do Edge, com o título da página | janela própria |

- **Leitor de tela no D:** o Edge só monta a árvore da página quando percebe um leitor de verdade; o teste não prova que o D é inacessível, só que o E expõe tudo direto.
- **Visual:** o mesmo nos dois (o mesmo HTML e CSS, e o WebView2 é o Edge).
- **O que o D não tira do Edge:** menu do botão direito, F12, zoom com Ctrl e roda do mouse, extensões e o agrupamento com as janelas do Edge na barra de tarefas. O `--app` só esconde a barra de endereço.
- **O que não deu para conferir no WSLg:** a bandeja no Linux (os três disseram "org.kde.StatusNotifierWatcher was not provided", porque o `fyne.io/systray` usa o StatusNotifierItem pelo D-Bus), a janela do Wails no Linux (o processo que desenha a página não abriu nem no MiniBrowser do WebKitGTK; é do ambiente, não do Wails) e o Orca. Ficaram para a #417, acima.

</details>
