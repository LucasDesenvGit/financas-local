# 0029. Instalador .msi para Windows, com WiX, gerado e testado pela CI

- **Status:** Aceita
- **Data:** 2026-10-17 (issue #119)
- **Relacionadas:** substitui em parte a [0012](0012-instalador-do-windows-em-powershell.md) (instalador em PowerShell); [0003](0003-esquema-por-scripts-sql-idempotentes.md) (a API aplica o esquema); [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) (gerenciador); [0005](0005-segredos-criptografados-com-data-protection.md) (chaves)

## Contexto

A [ADR 0012](0012-instalador-do-windows-em-powershell.md) instala no Windows com `instalar-windows.cmd` e `scripts/instalar-windows.ps1`, que fazem três coisas no computador do usuário:
- baixam o .NET SDK e o Node;
- compilam tudo, o que leva minutos e ocupa mais de 1 GB de ferramentas;
- registram o serviço.

Atualizar é repetir isso. A 0012 considerou um instalador empacotado e o descartou porque ele "exige empacotar e assinar um executável a cada versão".

A issue #119 pediu um `.msi` de "Avançar, Avançar, Concluir" e um gerenciador. Com a CI gerando o pacote a cada versão, o custo de empacotar deixa de ser manual.

## Alternativas consideradas

- **Continuar só com o PowerShell (ADR 0012).**
  - Prós: nada novo.
  - Contras: compila na máquina de quem só quer usar, e atualizar exige terminal ou script como administrador.
- **Inno Setup ou NSIS** (`setup.exe`).
  - Prós: script simples, interface fácil de personalizar.
  - Contras:
    - não é MSI: sem atualização e remoção padronizadas pelo Windows Installer, nem implantação por política;
    - serviço e certificado viram script do mesmo jeito.
- **MSIX.**
  - Prós: atualização moderna.
  - Contras: não instala serviço do Windows de forma simples, exige assinatura, e o app fica isolado do resto do sistema.
- **WiX Toolset 5** (`dotnet tool`, código aberto).
  - Prós:
    - MSI padrão, com serviço (`ServiceInstall`/`ServiceControl`), atualização (`MajorUpgrade`), atalhos e interface em pt-BR;
    - compila no runner Windows da CI;
    - o elemento `Files` coleta a pasta publicada.
  - Contras: o que o MSI não faz sozinho (certificado, permissões, migração) fica numa ação personalizada em PowerShell.

## Decisão

O `.msi` é gerado por `instalador/gerar-msi.ps1` a partir de `instalador/FinancasLocal.wxs` (WiX 5.0.2).

**Como ele é montado:**
- API e gerenciador vão **self-contained `win-x64` na mesma pasta**: o computador não precisa de .NET, Node, Git, bash nem `sqlite3`.
- A API cria e atualiza o banco ao iniciar (ADR 0003).
- O workflow `.github/workflows/versao.yml` gera e testa o pacote a cada pull request que mexe no instalador. Na tag `vX.Y.Z`, publica o `.msi` e o `SHA256SUMS.txt` no Release.
- A versão vem do `Directory.Build.props`, e a tag tem de ser igual.

**Onde as coisas ficam:**
- **Programa:** `%ProgramFiles%\FinancasLocal`.
- **Dados:** `%ProgramData%\FinancasLocal\dados` (`sql`, `chaves`, `backups`, `certificado`, `configuracao.json`), só para SYSTEM e Administradores (por SID).
- O `appsettings.Production.json` do pacote tem só `Dados:Pasta`. A API deriva banco e chaves dessa pasta e lê o `configuracao.json`, que guarda o certificado e a senha dele.
- Atualizar e desinstalar não mexem nos dados.

**Serviço:** `FinancasLocal`, início automático, reinício em caso de falha, conta **LocalSystem**, como na instalação pelos scripts. As chaves do Data Protection usam DPAPI da conta do serviço. Com outra conta (ex.: `NT SERVICE\FinancasLocal`), a migração perderia o Client Secret da Pluggy.

**Ação personalizada** (`instalador/configurar.ps1`, como SYSTEM, sem janela, saída no log do `msiexec`):
- **Na instalação, no reparo e na atualização**, antes de registrar o serviço:
  1. cria e protege a pasta de dados;
  2. **migra a instalação pelos scripts**, quando existe o serviço `FinancasLocal` apontando para outra pasta: para o serviço, copia banco e chaves (sem apagar a pasta antiga), deixa uma nota e remove o serviço antigo, que tem o mesmo nome;
  3. gera ou reaproveita o certificado (5 anos, SAN `localhost`/`127.0.0.1`/`::1`) e o põe nas Autoridades Raiz da máquina, como na 0012.
- **Só na desinstalação de verdade** (não na troca de versão): tira o certificado. Com `APAGARDADOS=1`, apaga também os dados.

**Interface:** boas-vindas, pasta de instalação e progresso, sem tela de licença (o projeto não tem uma). No fim, uma caixa marcada para abrir o gerenciador.

## Consequências

- **Positivas:**
  - Instalação em dois cliques, sem compilar nada no computador.
  - Atualização pelo gerenciador ou abrindo o `.msi` novo.
  - O Windows lista o programa em "Aplicativos instalados".
  - A CI testa o que o usuário vai fazer: instalar do zero, atualizar mantendo o usuário criado, desinstalar mantendo os dados e migrar uma instalação feita pelos scripts.
- **Negativas:**
  - **Sem assinatura de código** (em aberto na #119), o Windows mostra "Editor desconhecido" ao abrir o `.msi`. Assinar exige um certificado de código pago.
  - **Dois caminhos de instalação no Windows:**
    - mudança no certificado, no serviço ou na pasta de dados precisa ser feita no `instalar-windows.ps1` e no `configurar.ps1`;
    - o `configurar.ps1` tem as mesmas restrições do PowerShell 5.1 (ASCII, sem `??`).
  - **O pacote fica grande** (runtime do .NET, do ASP.NET e do Windows Desktop dentro), na casa de dezenas de MB.
  - **Migração automática:** a pasta antiga fica, mas o serviço antigo é removido. Para voltar, rode o `instalar-windows.cmd` antigo de novo. Se o banco da instalação antiga não for encontrado (caminho do `appsettings.Production.json` dela), a instalação **para sem mudar nada**, em vez de começar com um banco vazio.
  - **Só Windows x64.** No Linux continuam os scripts.
- **Na prática:**
  - **Publicar uma versão:**
    1. mude a versão no `Directory.Build.props` e o título da seção no `CHANGELOG.md`;
    2. faça o merge em `main`;
    3. crie a tag `vX.Y.Z`;
    4. o workflow publica o `.msi` no Release.
  - **Testar um pull request:** o `.msi` fica como artefato do workflow "Versão (instalador .msi)".
  - **Mudou serviço, certificado ou pastas?** Ajuste também o `instalar-windows.ps1` (e vice-versa).
