# 0028. Gerenciador do Windows, com atualização pelo GitHub Releases

- **Status:** Aceita
- **Data:** 2026-10-17 (issue #119)
- **Relacionadas:** [0032](0032-gerenciador-no-linux-com-avalonia.md) (gerenciador no Linux), [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (instalador .msi), [0012](0012-instalador-do-windows-em-powershell.md) (instalador em PowerShell), [0005](0005-segredos-criptografados-com-data-protection.md) (segredos)

## Contexto

Depois de instalado, o site é um serviço do Windows sem janela.

**O que faltava ao usuário:**
- saber se o site está no ar sem abrir o navegador;
- ligá-lo ou desligá-lo sem o Painel de Serviços;
- atualizar sem terminal: hoje é preciso rodar o `instalar-windows.cmd` de novo ou um script como administrador;
- tirar uma cópia do banco, que fica numa pasta que só SYSTEM e Administradores leem.

A issue #119 pediu um "gerenciadorzinho" para isso.

## Alternativas consideradas

- **Botão "Atualizar" no próprio site.**
  - Prós: nada para instalar além do site.
  - Contras: a API roda como LocalSystem. Deixar o site baixar e executar instaladores transforma qualquer falha web numa execução como SYSTEM, sem um UAC visível. O site pode, no máximo, avisar que há versão nova.
- **Atualização automática e silenciosa** (o serviço se atualiza sozinho).
  - Prós: zero cliques.
  - Contras: o mesmo problema de privilégio; uma versão quebrada chega sem a pessoa saber; nenhum momento para ler o que mudou.
- **Squirrel, ClickOnce ou winget.**
  - Prós: atualização pronta.
  - Contras:
    - Squirrel e ClickOnce instalam por usuário e não lidam com serviço do Windows;
    - o winget exige manifesto num repositório da Microsoft e um processo de revisão, e não liga nem desliga o serviço.
- **App pequeno do Windows (WinForms), instalado pelo .msi.**
  - Roda como o usuário e pede UAC só ao ligar, desligar ou copiar o banco.
  - Baixa a versão nova do GitHub Releases e confere o SHA256 antes de abrir o instalador.

## Decisão

**O app:** `src/gerenciador/Financas.Gerenciador` (WinForms, .NET 10), publicado na mesma pasta da API pelo .msi.

**Na janela:**
- **Primeiro:** a situação ("O site está no ar", "desligado", "ligado mas não responde", que vem do serviço e de `GET /api/auth/estado`), **com a versão do site** (campo `versao` dessa resposta; a versão fica no `Directory.Build.props`, a mesma da API e do gerenciador), e "Abrir o site", "Ligar/Desligar" e "Reiniciar".
- **Logo abaixo:** a versão instalada, a nova (se houver), "O que mudou" (as notas do Release) e "Atualizar para x.y.z".
- **Em "Mais opções":**
  - "Salvar uma cópia do banco…" (`VACUUM INTO` para onde a pessoa escolher);
  - os erros do site e o manual;
  - "Iniciar com o Windows", que deixa o app na área de notificação, com o ponto colorido e o aviso quando o site cai;
  - "Procurar versão nova ao abrir".

**Ações de administrador:** o gerenciador reabre a si mesmo com `runas` e argumentos fixos (`--servico ligar|desligar|reiniciar`, `--backup "destino"`), sem janela. A pasta de dados continua fechada para o usuário comum: a cópia do banco é a forma de levar os dados para fora.

**Atualização:**
1. Consulta `https://api.github.com/repos/LucasDesenvGit/financas-local/releases/latest`.
2. Baixa o `.msi` e o `SHA256SUMS.txt` do mesmo Release; o download pode ser redirecionado para `objects.githubusercontent.com` ou `release-assets.githubusercontent.com`.
3. Só abre o `msiexec` (com a janela do instalador, que pede o UAC) se o hash bater.
4. O gerenciador fecha para o instalador poder trocá-lo, e o instalador oferece reabri-lo no fim.
5. Nada é enviado além do pedido HTTP com o `User-Agent` do gerenciador.

**Procurar versão nova:** só avisa. Instalar é sempre um clique da pessoa, depois de uma confirmação que diz que o site fica fora do ar por alguns segundos e que o banco ganha uma cópia antes de mudar (ADR 0003).

## Consequências

- **Positivas:**
  - Situação sempre visível (Nielsen 1).
  - Linguagem sem "serviço", "porta" ou "msiexec" na tela principal (Nielsen 2).
  - Confirmação, cancelamento do download e cópia antes de mudar o banco (Nielsen 3 e 5).
  - Erro que diz o que fazer (Nielsen 9).
  - O essencial primeiro e o resto em "Mais opções" (Progressive Disclosure).
  - A elevação aparece sempre num UAC, ligada a uma ação que a pessoa pediu.
- **Negativas:**
  - **Saída de rede nova**, só no gerenciador (a API não muda): `api.github.com`, `github.com` e o domínio de download de arquivos do GitHub.
  - **Sem assinatura de código** (decisão em aberto na #119), o SHA256 só prova que o arquivo é o mesmo que foi publicado no Release, e não quem o publicou. Quem tiver acesso de escrita ao repositório pode publicar um instalador. Com assinatura, o gerenciador passa a conferi-la também.
  - **O repositório é fixo** (`Instalacao.Repositorio`): um fork precisa trocá-lo.
  - **Só Windows.** O Linux ganhou o seu gerenciador na [ADR 0032](0032-gerenciador-no-linux-com-avalonia.md).
  - **Mais um projeto na solução.** O `EnableWindowsTargeting` deixa a CI do Linux compilá-lo, mas não executá-lo.
- **Atualização de 2026-10-18 (instalação pelos scripts):**
  - O gerenciador acha o site pelo registro do serviço `FinancasLocal` (pasta da API e o `appsettings` dela), e não pela própria pasta. Assim ele funciona também na instalação feita pelo `instalar-windows.cmd` ou pelo `3-publicar-atualizar.sh`.
  - O `scripts/instalar-gerenciador.ps1` publica o gerenciador em `publicado\gerenciador` (fora de `publicado\api`, que a publicação espelha) e cria o atalho no Menu Iniciar do usuário, sem administrador.
  - Nessa instalação, o banco vem do `Banco:Caminho`, e o gerenciador **não procura nem instala versões do GitHub**: o `.msi` trocaria a instalação. A tela diz para atualizar pelos scripts, ou para passar para o instalador. O botão **Baixar o instalador .msi** abre a página da versão mais recente, e o `.msi` migra a instalação (ADR 0029).
  - Ele só atualiza quando o script roda de novo; a publicação do site não mexe nele.
- **Atualização de 2026-10-18 (atalho e visual):**
  - O gerenciador também ganha um atalho na **área de trabalho**:
    - no `.msi`, na área de trabalho de todos os usuários, e `ATALHOAREADETRABALHO=0` instala sem ele;
    - no script, na do usuário, e `-SemAtalhoNaAreaDeTrabalho` não cria.
  - A janela segue as cores do site e o tema claro ou escuro do Windows (`Tema.cs`, `Controles.cs`).
  - Os controles desenhados continuam `Button` e `CheckBox` por baixo, para o teclado e o leitor de tela não mudarem.
- **Atualização de 2026-10-18 (núcleo comum com o Linux, ADR 0032):**
  - `Instalacao`, `VerificadorSituacao` e a cópia do banco foram para o `Financas.Gerenciador.Nucleo` (`net10.0`), que o gerenciador do Linux também usa.
  - O serviço do Windows ficou no `ServicoWindows`: o estado, a pasta da API pelo registro e o `--servico`, que roda elevado.
  - A janela WinForms e o comportamento no Windows não mudaram. O `.msi` leva o `Financas.Gerenciador.Nucleo.dll` junto, na mesma pasta.
- **Na prática:**
  - O Release precisa ter o `.msi` e o `SHA256SUMS.txt` (formato do `sha256sum`), gerados pelo workflow de versão (ADR 0029).
  - Sem os dois, o gerenciador diz que não há versão nova para instalar.
