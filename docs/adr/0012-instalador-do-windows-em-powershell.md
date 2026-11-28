# 0012. Instalador do Windows em PowerShell, sem bash

- **Status:** Aceita (em parte substituída pela [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md): para quem só usa, o caminho no Windows passa a ser o `.msi`; este instalador continua para desenvolvimento e para quem compila do código; e em parte substituída pela [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md): o SDK do .NET deu lugar ao Go portátil e saiu na #86)
- **Data:** 2026-10-16
- **Relacionadas:** substitui em parte a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (automação só em bash); [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) (ferramentas portáteis)

**Os passos e os detalhes** (o que o `.ps1` faz, as restrições do PowerShell 5.1, as pastas de `scripts/`, as opções e os testes de instalação) estão no [README dos scripts](../../scripts/README.md#windows-o-instalador-em-powershell-adr-0012). Esta ADR guarda a decisão, as alternativas e o custo.

## Contexto

A [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) decidiu automatizar tudo em scripts bash, os mesmos no Windows e no Linux. No Windows isso exige o Git Bash (Git for Windows) antes de qualquer coisa, e o Git Bash foi a origem de boa parte dos erros de instalação num computador novo: atalho do Python da Microsoft Store, ausência de `sqlite3`/`unzip`, caminhos POSIX (`/c/...`) que programas nativos não entendem. O usuário pediu que o instalador do Windows não dependa de bash.

## Alternativas consideradas

- **Bash portátil embutido** — o `instalar-windows.cmd` baixaria o PortableGit para `ferramentas/` e rodaria os mesmos `.sh`; uma implementação só, sem conflito com a 0002; mas o bash e a camada MSYS continuam no caminho (e com eles os problemas de caminho), só que escondidos.
- **Instalador empacotado (MSI/Inno Setup)** — experiência de instalador "de verdade"; mas exige empacotar e assinar um executável a cada versão, e ainda precisaria de lógica de build/publicação por trás. (Revisto pela [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md), quando a CI passou a empacotar.)
- **PowerShell nativo** — o Windows PowerShell 5.1 vem em todo Windows 10/11; nada para instalar antes. Custo: as etapas 1 a 3 passam a existir em duas linguagens.

## Decisão

No Windows, a instalação é feita por `scripts/windows/instalar-windows.ps1` (etapas 1, 2 e 3), chamado por `instalar-windows.cmd` na raiz, que pede elevação. Escolhido pelo usuário para eliminar a dependência do Git Bash. Os scripts bash continuam valendo para Linux/macOS. O mesmo vale para parar o sistema: `parar-windows.cmd` chama o `scripts/windows/parar-windows.ps1`.

Para reduzir a duplicação, o `.ps1` **não tem lista própria de colunas novas**. Desde a #723 ele não lê mais nada do bash: chama `financas-go aplicar-esquema`, e a lista fica em Go (`esquema.ColunasNovas`; antes eram as linhas `adicionar_coluna_se_ausente` do `2-criar-banco.sh`). Os arquivos `banco/*.sql` vão embutidos no executável.

## Consequências

- **Positivas:** num Windows novo, dois cliques no `.cmd` instalam tudo sem Git, bash, Python nem `winget`, sem nenhuma janela de confirmação no meio, e no fim abrem o site no navegador; os caminhos são nativos do começo ao fim; numa atualização, o banco é copiado antes de mudar a estrutura e, se a nova versão não subir, a anterior volta sozinha.
- **Negativas:**
  - duas implementações das etapas 1 a 3 (`.sh` e `.ps1`) até a #655 (abaixo): mudança de URL, versão aceita do Node, certificado, `appsettings.Production.json` ou registro do serviço tinha de ser feita nas duas;
  - o `.ps1` precisa ser compatível com o Windows PowerShell 5.1 (sem `??`, sem acentos no arquivo, entre outras), e o teste da CI não cobre o pedido de elevação (UAC) nem o duplo clique, que seguem como teste manual;
  - o certificado HTTPS é posto nas **Autoridades Raiz Confiáveis da máquina** (`LocalMachine\Root`), não só da conta: vale para todas as contas do computador, sem a janela do `dotnet dev-certs https --trust` e sem cair na conta errada quando outro administrador eleva no UAC. Como é uma raiz autoassinada, quem tiver a chave privada (o `.pfx` em `dados/chaves`) poderia emitir certificados aceitos neste computador. O SAN é só `localhost`, e a chave fica na pasta `dados`, que já guarda os dados financeiros;
  - o backup (`4-backup.sh`) continua pedindo bash no Windows.
- **Na prática:** coluna nova só em `ColunasNovas` (Go). Serviço, certificado e pastas de dados também existem no `instalador/configurar.ps1` (o `.msi`, [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md)): confira se mudam juntos.

## Atualização de 2026-10-20: scripts separados por sistema (issue #154)

- **Decisão:** `scripts/geral/` tem o bash dos dois sistemas; `scripts/windows/` e `scripts/linux/`, o que só roda num deles; `scripts/ci/`, o que só a CI usa. Os `.cmd` continuam na raiz, como porta de entrada. Os filtros `paths:` dos testes de instalação usam as pastas, para um script novo dentro delas também disparar o teste.
- **Custo:** quem tem o caminho antigo num atalho ou num script próprio precisa trocar. Os repasses temporários da raiz de `scripts/` saíram na #655.

## Atualização de 2026-11-06: o Git Bash repassa para o PowerShell (issue #655)

**Decisão (confirmada pelo usuário em 06/11/2026):** no Git Bash do Windows, `scripts/geral/instalar.sh` e `scripts/geral/3-publicar-atualizar.sh` passam a **repassar para `scripts/windows/instalar-windows.ps1`**, como o `parar.sh` já fazia com o `parar-windows.ps1`. Os ramos do Windows das etapas 1 e 3 saíram do bash. O Windows fica com **dois caminhos de instalação**: o `.ps1` (`instalar-windows.cmd`, ou o Git Bash que repassa a ele) e o `.msi` ([0029](0029-instalador-msi-com-wix-gerado-pela-ci.md)). Antes eram três, e a mesma lógica existia em duas linguagens.

- **Alternativa recusada:** manter os três caminhos e só documentar a duplicação. Cada mudança de URL, certificado ou registro do serviço teria de ser feita no bash e no `.ps1`, e a regra "mudou o bash, confira o `.ps1`" já tinha falhado antes.
- **O que continua em bash no Windows:** o `2-criar-banco.sh` e o `4-backup.sh` (o backup agendado do Windows roda pelo `bash.exe` do Git).
- **Código comum do PowerShell:** `scripts/windows/comum.ps1`. O `instalador/configurar.ps1` (`.msi`) usa só o `Confiar-No-Certificado` (#775): o `gerar-msi.ps1` leva o arquivo junto, e o resto do `.msi` não depende dele.
- **Custo:** quem usa o Git Bash no Windows passa a rodar o PowerShell por baixo, com a saída, o log e a exigência de Administrador dele. Quem pulou todas as versões entre a #154 e a #655 e roda o comando antigo do gerenciador em C# precisa usar os caminhos de `scripts/geral/`.
- **Regra que deixa de valer:** "mudou `1-preparar-ambiente.sh` ou `3-publicar-atualizar.sh`, confira o `.ps1`". Continua valendo: coluna nova só em `ColunasNovas` (desde a #723, ninguém lê isso de bash), e serviço, certificado e pastas de dados também no `instalador/configurar.ps1`.

## Atualização de 2026-11-27: pastas do serviço protegidas (#68, item 1)

**Decisão (do usuário, 27/11/2026):** o serviço roda como SYSTEM (a conta continua LocalSystem, [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md), porque outra conta perderia o DPAPI dos segredos). Por isso o `instalar-windows.ps1` deixa `publicado\api` (com `api-anterior` e `preparo`), `dados` e `backups` só para SYSTEM e Administradores, que passam a ser os donos, com Usuários só lendo e executando a publicação. É o `Proteger-Pasta` do `comum.ps1`, o mesmo do `.msi`. O teste de instalação no Windows confere as permissões.

- **O resto da pasta do projeto continua da sua conta** (`src`, `ferramentas`, `publicado\gerenciador`): compilar e o `instalar-gerenciador.ps1` seguem sem administrador.
- **Custo:** no Windows, o `2-criar-banco.sh` e o `4-backup.sh` pedem o Git Bash como Administrador, e o backup agendado roda com privilégios mais altos.
- **O que não cobre:** com o serviço parado, quem roda na sua conta pode renomear `publicado` (ou a pasta do projeto) e pôr outra no lugar, e a instalação elevada compila o `src` da sua conta. O caminho seguro para quem só usa continua o `.msi`.
