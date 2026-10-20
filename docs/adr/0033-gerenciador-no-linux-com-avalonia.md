# 0033. Gerenciador no Linux, com Avalonia e núcleo comum ao do Windows

- **Status:** Aceita (em parte substituída pela [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md): na instalação de desenvolvimento, o botão roda o git)
- **Data:** 2026-10-18 (issue #138)
- **Relacionadas:** [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) (gerenciador do Windows), [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) (ferramentas portáteis)

## Contexto

No Linux, o site é um serviço do `systemd --user`, sem janela, escrito pelo `3-publicar-atualizar.sh`. A ADR 0028 deixou o gerenciador "só Windows". No Linux, quem queria saber se o site estava no ar, ligar ou desligar, ver por que não subiu ou copiar o banco precisava do terminal e de comandos como `systemctl --user` e `journalctl`.

O gerenciador do Windows é WinForms (`net10.0-windows`), que não roda no Linux.

**Diferenças do Linux que mudam o desenho:**
- o serviço e o banco (`dados/`, `chmod 700`) são do próprio usuário: nada pede senha nem elevação;
- não existe `.msi`: a versão nova vem pelos scripts (`git pull` + `3-publicar-atualizar.sh`);
- o serviço do usuário só roda com a sessão aberta, a não ser com o *linger* do `systemd-logind`.

## Alternativas consideradas

- **Avalonia só no Linux, com núcleo comum** (escolhida).
  - Prós:
    - o WinForms, recém-redesenhado, e o `.msi` não mudam;
    - a lógica (instalação, situação, serviço, cópia do banco) fica num lugar só.
  - Contras: duas telas finas para manter em paralelo.
- **Avalonia nos dois sistemas**, substituindo o WinForms.
  - Prós: uma tela só.
  - Contras:
    - refaz o `Tema.cs` e o `Controles.cs` recém-feitos;
    - muda o conteúdo do `.msi`;
    - obriga a revalidar o leitor de tela no Windows.
- **GTK (GtkSharp) só no Linux.**
  - Contras:
    - binding com pouca manutenção;
    - mais uma pilha de interface, sem ganho sobre o Avalonia.
- **Só um comando no terminal**, mais um atalho que abre o site.
  - Prós: nenhuma dependência nova.
  - Contras: não resolve o "sem terminal" (Nielsen 2), que é o motivo da ADR 0028.

## Decisão

Usamos **três projetos**, porque assim o Windows fica como estava e a lógica não se duplica:

| Projeto | Plataforma | O que tem |
|---|---|---|
| `Financas.Gerenciador.Nucleo` | `net10.0` | `Instalacao`, `VerificadorSituacao`, `CopiaDoBanco`, `Comando`, e o serviço do sistema (`ServicoDoSite`: `ServicoWindows` e `ServicoLinux`) |
| `Financas.Gerenciador` | WinForms, `net10.0-windows` | A janela do Windows. Usa o núcleo e se comporta como antes |
| `Financas.Gerenciador.Linux` | Avalonia 12, `net10.0` | A janela do Linux, na mesma ordem e com as mesmas frases da do Windows |

**O que muda no Linux:**

| | Windows (ADR 0028) | Linux |
|---|---|---|
| Achar o site | registro do serviço (`ImagePath`) | `systemctl --user show financaslocal.service` (`WorkingDirectory`, `ExecStart`) |
| Ligar, desligar, reiniciar | `runas` + `ServiceController` (UAC) | `systemctl --user start/stop/restart`, sem senha |
| Cópia do banco | elevada (`--backup`) | direta (`VACUUM INTO`), num arquivo criado com permissão `600` |
| Por que não subiu | Visualizador de Eventos | **Ver o registro do serviço**: `journalctl --user -u financaslocal.service`, as últimas 200 linhas |
| Iniciar com o sistema | chave `Run` | `~/.config/autostart/financaslocal-gerenciador.desktop` |
| Ícone na bandeja | `NotifyIcon` | `TrayIcon` (StatusNotifierItem), só se houver `org.kde.StatusNotifierWatcher` |
| Aviso de "o site caiu" | balão da bandeja | `notify-send` |
| Um por usuário | `Mutex` + `EventWaitHandle` com nome | socket Unix em `$XDG_RUNTIME_DIR` |
| Versão nova | baixa o `.msi` do GitHub | mostra o comando para atualizar, com o botão **Copiar o comando** |
| Sobe sem login | sempre (serviço do sistema) | avisa quando o *linger* está desligado e oferece `loginctl enable-linger` |

**Instalação:** `scripts/instalar-gerenciador.sh`, sem root:
1. publica self-contained em `publicado/gerenciador`;
2. cria o atalho no menu de aplicativos (`~/.local/share/applications`);
3. cria o atalho na área de trabalho, se ela existir.

`--situacao` escreve a situação no terminal, sem abrir a janela; a CI usa esse modo.

## Consequências

- **Positivas:**
  - A situação fica visível sem terminal (Nielsen 1), com as mesmas palavras do Windows (Nielsen 4).
  - O comando de atualização fica pronto para copiar (Nielsen 6, reconhecer em vez de lembrar).
  - O registro do serviço fica a um clique, inclusive quando o site nem sobe (Nielsen 9).
  - **Nenhuma saída de rede nova:** o gerenciador do Linux não consulta o GitHub.
  - O WinForms e o `.msi` não mudam, a não ser pelo `Financas.Gerenciador.Nucleo.dll`, que a pasta publicada já leva junto.
- **Negativas:**
  - **Dependência nova:** Avalonia 12.1.3, com a versão exata, mais SkiaSharp e HarfBuzzSharp.
    - O pacote publicado tem cerca de 100 MB, porque leva o runtime do .NET.
    - Atualizar o Avalonia é uma decisão: troque a versão no `.csproj` e confira a janela nos temas claro e escuro.
  - **Duas janelas em paralelo:** um texto ou uma ação nova no gerenciador precisa ser feito nas duas.
  - **Bandeja no GNOME:** sem a extensão AppIndicator, não há ícone. "Iniciar com o sistema" então abre a janela minimizada.
  - **Foco:** com o gerenciador já aberto, abri-lo de novo mostra a janela, mas o KDE pode deixá-la atrás da janela ativa (proteção contra roubo de foco).
  - **A atualização continua manual:** a tela só mostra o comando.
  - **Leitor de tela no Linux:** o Avalonia 12 publica a interface pelo AT-SPI. Não foi testado com o Orca.
- **Na prática:**
  - Regra comum (situação, serviço, banco) vai no `Financas.Gerenciador.Nucleo`. O que é só de um sistema leva `[SupportedOSPlatform]` ou `[UnsupportedOSPlatform]`, para o analisador CA1416 avisar quando for chamado do lado errado.
  - Mudou o `3-publicar-atualizar.sh` (nome da unidade, `WorkingDirectory`)? Confira o `ServicoLinux`.
  - A CI ("Teste de instalação no Linux") roda o `instalar-gerenciador.sh`, confere o atalho e o `--situacao` com o site no ar e desligado. Ela não abre a janela.
