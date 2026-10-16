# 0012. Instalador do Windows em PowerShell, sem bash

- **Status:** Aceita
- **Data:** 2026-10-16
- **Relacionadas:** substitui em parte a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (automação só em bash); [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) (ferramentas portáteis)

## Contexto

A [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) decidiu automatizar tudo em scripts bash, os mesmos no Windows e no Linux. No Windows isso exige o Git Bash (Git for Windows) antes de qualquer coisa, e o Git Bash foi a origem de boa parte dos erros de instalação num computador novo: atalho do Python da Microsoft Store, ausência de `sqlite3`/`unzip`, caminhos POSIX (`/c/...`) que programas nativos não entendem. O usuário pediu que o instalador do Windows não dependa de bash.

## Alternativas consideradas

- **Bash portátil embutido** — o `instalar-windows.cmd` baixaria o PortableGit para `ferramentas/` e rodaria os mesmos `.sh`; uma implementação só, sem conflito com a 0002; mas o bash e a camada MSYS continuam no caminho (e com eles os problemas de caminho), só que escondidos.
- **Instalador empacotado (MSI/Inno Setup)** — experiência de instalador "de verdade"; mas exige empacotar e assinar um executável a cada versão, e ainda precisaria de lógica de build/publicação por trás.
- **PowerShell nativo** — o Windows PowerShell 5.1 vem em todo Windows 10/11; nada para instalar antes. Custo: as etapas 1 a 3 passam a existir em duas linguagens.

## Decisão

No Windows, a instalação é feita por `scripts/instalar-windows.ps1` (etapas 1, 2 e 3), chamado por `instalar-windows.cmd` na raiz, que pede elevação. Escolhido pelo usuário para eliminar a dependência do Git Bash. Os scripts bash continuam valendo para Linux/macOS e continuam funcionando no Windows pelo Git Bash.

Para reduzir a duplicação, o `.ps1` **não tem lista própria de colunas novas**: lê as linhas `adicionar_coluna_se_ausente "Tabela" "Coluna" "definição"` do `2-criar-banco.sh`. Os arquivos `banco/*.sql` também são os mesmos.

## Consequências

- **Positivas:** num Windows novo, dois cliques no `.cmd` instalam tudo sem Git, bash, Python nem `winget`; caminhos são nativos do começo ao fim; o serviço é registrado com `New-Service`/`Win32_Service.Change` e parado antes da cópia (DLLs em uso); a saída fica em `instaladores/instalacao-windows.log`.
- **Negativas:**
  - duas implementações das etapas 1 a 3 (`.sh` e `.ps1`): mudança de URL, versão aceita do Node, certificado, `appsettings.Production.json` ou registro do serviço tem de ser feita nas duas;
  - o `.ps1` precisa ser compatível com o Windows PowerShell 5.1 (sem `??`, sem `-SkipCertificateCheck`, sem aspas duplas em argumentos de programas externos, sem acentos no arquivo): o workflow `.github/workflows/teste-instalacao-windows.yml` roda o instalador num runner Windows (PATH reduzido ao do Windows puro, instalação do zero e depois atualização pelo `.cmd`), mas não cobre o pedido de elevação (UAC) nem o duplo clique, que seguem como teste manual;
  - o `.ps1` depende do formato da chamada `adicionar_coluna_se_ausente` no `2-criar-banco.sh` (três argumentos entre aspas duplas, um por linha); outra forma de escrever passa despercebida (o script só avisa se não encontrar nenhuma);
  - o backup (`4-backup.sh`) e a ferramenta de migração continuam pedindo bash no Windows.
- **Na prática:** o workflow `Teste de instalação no Windows` roda quando mudam o `.cmd`, o `.ps1`, o `2-criar-banco.sh`, `banco/` ou o próprio workflow; para testar outra mudança, rode-o manualmente (Actions › Teste de instalação no Windows › Run workflow). Ao mudar `1-preparar-ambiente.sh` ou `3-publicar-atualizar.sh`, conferir se `instalar-windows.ps1` precisa da mesma mudança (e vice-versa). Coluna nova continua só no `2-criar-banco.sh`, no formato acima.
