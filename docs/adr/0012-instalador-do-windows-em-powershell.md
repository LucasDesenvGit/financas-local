# 0012. Instalador do Windows em PowerShell, sem bash

- **Status:** Aceita (em parte substituída pela [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md): para quem só usa, o caminho no Windows passa a ser o `.msi`; este instalador continua para desenvolvimento e para quem compila do código; e em parte substituída pela [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md): o SDK do .NET deu lugar ao Go portátil e saiu na #86)
- **Data:** 2026-10-16
- **Relacionadas:** substitui em parte a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (automação só em bash); [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) (ferramentas portáteis)

## Contexto

A [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) decidiu automatizar tudo em scripts bash, os mesmos no Windows e no Linux. No Windows isso exige o Git Bash (Git for Windows) antes de qualquer coisa, e o Git Bash foi a origem de boa parte dos erros de instalação num computador novo: atalho do Python da Microsoft Store, ausência de `sqlite3`/`unzip`, caminhos POSIX (`/c/...`) que programas nativos não entendem. O usuário pediu que o instalador do Windows não dependa de bash.

## Alternativas consideradas

- **Bash portátil embutido** — o `instalar-windows.cmd` baixaria o PortableGit para `ferramentas/` e rodaria os mesmos `.sh`; uma implementação só, sem conflito com a 0002; mas o bash e a camada MSYS continuam no caminho (e com eles os problemas de caminho), só que escondidos.
- **Instalador empacotado (MSI/Inno Setup)** — experiência de instalador "de verdade"; mas exige empacotar e assinar um executável a cada versão, e ainda precisaria de lógica de build/publicação por trás.
- **PowerShell nativo** — o Windows PowerShell 5.1 vem em todo Windows 10/11; nada para instalar antes. Custo: as etapas 1 a 3 passam a existir em duas linguagens.

## Decisão

No Windows, a instalação é feita por `scripts/instalar-windows.ps1` (etapas 1, 2 e 3), chamado por `instalar-windows.cmd` na raiz, que pede elevação. Escolhido pelo usuário para eliminar a dependência do Git Bash. Os scripts bash continuam valendo para Linux/macOS. (Até 06/11/2026 também funcionavam no Windows pelo Git Bash; desde a #655 o Git Bash repassa para o `.ps1`: veja a última seção.)

Para reduzir a duplicação, o `.ps1` **não tem lista própria de colunas novas**: lê as linhas `adicionar_coluna_se_ausente "Tabela" "Coluna" "definição"` do `2-criar-banco.sh`. Os arquivos `banco/*.sql` também são os mesmos.

## Consequências

- **Positivas:** num Windows novo, dois cliques no `.cmd` instalam tudo sem Git, bash, Python nem `winget`, sem nenhuma janela de confirmação no meio, e no fim abrem o site no navegador; caminhos são nativos do começo ao fim; o serviço é registrado com `New-Service`/`Win32_Service.Change` e parado antes da cópia (DLLs em uso); numa atualização, o banco é copiado para `backups/` antes de mudar a estrutura e, se a nova versão não subir, a anterior (`publicado/api-anterior`) volta sozinha; a saída (com o stderr do npm/dotnet e os eventos do Windows quando a API não sobe) fica em `instaladores/instalacao-windows.log`, acumulando as execuções.
- **Negativas:**
  - duas implementações das etapas 1 a 3 (`.sh` e `.ps1`): mudança de URL, versão aceita do Node, certificado, `appsettings.Production.json` ou registro do serviço tem de ser feita nas duas;
  - o `.ps1` precisa ser compatível com o Windows PowerShell 5.1 (sem `??`, sem `-SkipCertificateCheck`, sem aspas duplas em argumentos de programas externos, sem acentos no arquivo): o workflow `.github/workflows/teste-instalacao-windows.yml` roda o instalador num runner Windows (PATH reduzido ao do Windows puro, instalação do zero e depois atualização pelo `.cmd`), mas não cobre o pedido de elevação (UAC) nem o duplo clique, que seguem como teste manual;
  - o `.ps1` depende do formato da chamada `adicionar_coluna_se_ausente` no `2-criar-banco.sh` (três argumentos entre aspas duplas, um por linha); outra forma de escrever passa despercebida (o script só avisa se não encontrar nenhuma);
  - o certificado HTTPS é posto nas **Autoridades Raiz Confiáveis da máquina** (`LocalMachine\Root`), não só da conta: vale para todas as contas do computador, sem a janela do `dotnet dev-certs https --trust` e sem cair na conta errada quando outro administrador eleva no UAC. Como é uma raiz autoassinada, quem tiver a chave privada (o `.pfx` em `dados/chaves`) poderia emitir certificados aceitos nesta máquina; o SAN do certificado é só `localhost`, e a chave fica na pasta `dados`, que já guarda os dados financeiros. O certificado antigo é trocado na renovação pelo `FriendlyName` `FinancasLocal`;
  - o backup (`4-backup.sh`) e a ferramenta de migração continuam pedindo bash no Windows.
- **Na prática:** o workflow `Teste de instalação no Windows` roda quando mudam o `.cmd`, o `.ps1`, o `2-criar-banco.sh`, `banco/` ou o próprio workflow; para testar outra mudança, rode-o manualmente (Actions › Teste de instalação no Windows › Run workflow). Coluna nova continua só no `2-criar-banco.sh`, no formato acima. O mesmo vale para parar o sistema: `parar-windows.cmd` (raiz, pede elevação) chama `scripts/parar-windows.ps1`, e o `scripts/parar.sh` no Git Bash repassa para esse `.ps1` em vez de ter lógica própria do Windows; ele segue as mesmas restrições do Windows PowerShell 5.1. Os workflows de instalação (Windows e Linux) param o sistema com `-Desativar`/`--desativar`, conferem que nada ficou de pé e sobem de novo; só o pedido de elevação (UAC) continua sendo teste manual. O script procura só processos com a pasta do projeto na linha de comando e sai com código 1 se algo ficou rodando.

## Atualização de 2026-10-20: scripts separados por sistema (issue #154)

- **Pastas:** `scripts/geral/` tem o bash que roda nos dois sistemas (no Windows, pelo Git Bash): `comum.sh`, `instalar.sh`, as etapas `1-` a `4-` e o `parar.sh`. `scripts/windows/` tem o que só roda no Windows (`instalar-windows.ps1`, `parar-windows.ps1`, `instalar-gerenciador.ps1`) e `scripts/linux/`, o que só roda no Linux (`instalar-gerenciador.sh`). `scripts/ci/` fica com o que só a CI usa. Os `.cmd` continuam na raiz, como porta de entrada.
- **Na prática, os caminhos desta ADR passam a ser:** `instalar-windows.cmd` → `scripts/windows/instalar-windows.ps1`; `parar-windows.cmd` → `scripts/windows/parar-windows.ps1`; colunas novas no `scripts/geral/2-criar-banco.sh` (lido também pela API, embutido no build). A raiz do projeto fica dois níveis acima de cada script.
- **Workflows:** os filtros `paths:` dos testes de instalação usam as pastas (`scripts/geral/**`, `scripts/windows/**`, `scripts/linux/**`), para um script novo dentro delas também disparar o teste.
- **Transição:** ficaram três repasses temporários na raiz de `scripts/` (`4-backup.sh`, `3-publicar-atualizar.sh`, `instalar-gerenciador.sh`), porque o backup agendado (crontab ou Agendador de Tarefas) guarda o caminho absoluto e o gerenciador do Linux antigo mostra um comando com os caminhos antigos. Eles saíram na #655 (veja a última seção).
- **Custo:** quem tem o caminho antigo num atalho ou num script próprio precisa trocar.

## Atualização de 2026-11-06: o Git Bash repassa para o PowerShell (issue #655)

**Decisão (confirmada pelo usuário em 06/11/2026):** no Git Bash do Windows, `scripts/geral/instalar.sh` e `scripts/geral/3-publicar-atualizar.sh` passam a **repassar para `scripts/windows/instalar-windows.ps1`**, como o `parar.sh` já fazia com o `parar-windows.ps1`. Os ramos `SO == windows` das etapas 1 e 3 (`sc.exe`, `certutil`, `powershell.exe` avulso, download e extração de `.zip` do Node, do Go e do sqlite3) saíram do bash. O Windows fica com **dois caminhos de instalação**: o `.ps1` (`instalar-windows.cmd`, ou o Git Bash que repassa a ele) e o `.msi` ([0029](0029-instalador-msi-com-wix-gerado-pela-ci.md)). Antes eram três, e a mesma lógica existia em duas linguagens.

- **Alternativa recusada:** manter os três caminhos e só documentar a duplicação. Cada mudança de URL, certificado ou registro do serviço teria de ser feita no bash e no `.ps1`, e a regra "mudou o bash, confira o `.ps1`" já tinha falhado antes.
- **Opções:** `--pular-build` do bash vira `-PularBuild`; `--sem-gerenciador` não se aplica (no Windows o gerenciador é o `instalar-gerenciador.ps1`, que o `.ps1` anuncia no fim); qualquer outra opção é recusada. Quem roda o `1-preparar-ambiente.sh` no Windows recebe o aviso para usar o `instalar-windows.cmd`.
- **O que continua em bash no Windows:** o `2-criar-banco.sh` e o `4-backup.sh` (o backup agendado do Windows roda o `4-backup.sh` pelo `bash.exe` do Git, com `schtasks`), e por isso o `comum.sh` ainda tem `caminho_nativo` e `caminho_sqlite` (`cygpath`).
- **Custo:** quem usa o Git Bash no Windows passa a rodar o PowerShell por baixo, com a saída e o log dele (`instaladores\instalacao-windows.log`, que acumula as execuções) e com a exigência de Administrador dele (o Git Bash aberto como Administrador repassa a elevação).
- **Código comum do PowerShell:** `scripts/windows/comum.ps1` (carregado por dot-source) junta as pastas e o `Etapa`/`Ok`/`Aviso`/`ErroSai` que os `.ps1` repetiam. O `instalador/configurar.ps1` (`.msi`) **não** o usa: o `.msi` teria de levar o arquivo junto, e mexer no instalador é a etapa de maior risco.
- **Repasses da raiz de `scripts/` apagados** (`3-publicar-atualizar.sh`, `4-backup.sh`, `instalar-gerenciador.sh`). O backup agendado antes da #154 é migrado antes: o `3-publicar-atualizar.sh` (Linux) troca a entrada do crontab, e o `instalar-windows.ps1` (`Migrar-BackupAgendado`) troca a do Agendador de Tarefas, ambos a cada atualização. Quem atualiza pelo gerenciador (`atualizar.sh`, `atualizar-windows.ps1`) passa por eles. Quem pulou todas as versões entre a #154 e a #655 e roda o comando antigo do gerenciador em C# (`git pull && scripts/3-publicar-atualizar.sh`) precisa usar os caminhos de `scripts/geral/`.
- **Regra que deixa de valer:** "mudou `1-preparar-ambiente.sh` ou `3-publicar-atualizar.sh`, confira o `.ps1`". Continua valendo: coluna nova só no `2-criar-banco.sh` (o `.ps1` lê dele), e serviço, certificado e pastas de dados também no `instalador/configurar.ps1`.
- **Workflow:** o passo do "Teste de instalação (Windows)" que testava o `3-publicar-atualizar.sh` com `sc.exe` passa a conferir o repasse (serviço trocado e no ar pelo `.ps1`, `instalar.sh` até o fim, opção desconhecida recusada), e o filtro `paths:` ganhou `instalar.sh`, `3-publicar-atualizar.sh` e `comum.sh`.
