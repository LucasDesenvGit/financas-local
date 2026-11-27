# Scripts

Índice único dos scripts do repositório (#655): uma linha por script, com o que faz e quem roda. Quem muda, move ou cria
um script atualiza esta tabela no mesmo PR.

As pastas separam **quem roda** (#154, #655):

| Pasta | O que tem |
|---|---|
| `geral/` | O que quem **instala** roda, em bash, no Linux e no macOS. No Windows, só o `2-criar-banco.sh` e o `4-backup.sh` rodam em bash (Git Bash); o `instalar.sh`, o `3-publicar-atualizar.sh` e o `parar.sh` repassam para o PowerShell. |
| `windows/`, `linux/` | O que só funciona num dos dois. No Windows, a instalação existe só em PowerShell: o `.ps1` daqui e o `.msi` ([ADR 0012](../docs/adr/0012-instalador-do-windows-em-powershell.md), [0029](../docs/adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)). |
| `dev/` | Ferramentas de quem **desenvolve**, rodadas à mão (a CI usa algumas). |
| `ci/` | O que a **CI** roda, por assunto: `versao/`, `go/`, `front/` e `banco/`. Cada um pode ser rodado à mão, na pasta do repositório (o cabeçalho de cada arquivo diz como). |

Os `.cmd` (`instalar-windows.cmd`, `parar-windows.cmd`) ficam na raiz do repositório, porque são a porta de entrada de
quem não usa terminal.

**Caminhos que não podem mudar:** `geral/instalar.sh`, `geral/atualizar.sh`, `geral/4-backup.sh`,
`windows/atualizar-windows.ps1`, `linux/instalar-gerenciador.sh` e os `.cmd`. Os gerenciadores já instalados chamam esses
caminhos no clone (`src/gerenciador-go/internal/desenvolvimento/`), e o crontab e o Agendador de Tarefas guardam o
caminho do `geral/4-backup.sh`.

## Instalação e operação (`geral/`, `windows/`, `linux/`)

| Script | O que faz | Quem roda |
|---|---|---|
| `geral/instalar.sh` | As etapas 1, 2 e 3 em sequência; para no primeiro erro. No Git Bash do Windows, repassa para o `windows/instalar-windows.ps1`. | Pessoa, no Linux ou no Git Bash; a CI ("Teste de instalação") |
| `geral/1-preparar-ambiente.sh` | Confere as ferramentas básicas do terminal; instala Node.js LTS, Go (a linha do `src/backend/go.mod`) e, se o sistema não tiver, `sqlite3` portáteis em `ferramentas/`. Não depende de Python ([ADR 0011](../docs/adr/0011-ferramentas-de-instalacao-portateis-sem-python.md)). No Windows, recusa e manda usar o `instalar-windows.cmd`. | Pessoa (etapa 1 do `instalar.sh`) |
| `geral/2-criar-banco.sh` | Cria as pastas de `dados/` com permissão restrita e cria ou atualiza o banco pelo executável do Go (`go run ./cmd/financas aplicar-esquema`, #723): estrutura, colunas novas e dados iniciais vêm embutidos nele, e não há lista de colunas no bash. | Pessoa (etapa 2); a CI |
| `geral/3-publicar-atualizar.sh` | Compila o front-end e o back-end em Go (`financas-go`), publica em `publicado/api`, gera ou reaproveita o certificado HTTPS local e registra o serviço (systemd `--user`). No Git Bash do Windows, repassa para o `windows/instalar-windows.ps1`. | Pessoa (etapa 3); `atualizar.sh`; a CI |
| `geral/4-backup.sh` | Backup online do banco (`sqlite3 .backup`), com retenção e opção de agendar (`--agendar`). | Pessoa; o crontab ou o Agendador de Tarefas; a CI |
| `geral/atualizar.sh` | Só na instalação de desenvolvimento (clone na branch `develop`, [ADR 0040](../docs/adr/0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md)): traz o `develop` (só avança, recusa com alteração local ou outra branch) e reinstala. | Gerenciador (botão "Atualizar para a última do develop") |
| `geral/parar.sh` | Para o serviço, o site fora dele e o `npm start`; no Git Bash, repassa para o `windows/parar-windows.ps1`. | Pessoa |
| `geral/comum.sh` | Funções compartilhadas pelos scripts (detecção de SO, pastas, `sqlite3` portátil no PATH, repasse para o PowerShell e caminho nativo no Windows, `systemctl` do host num Flatpak, extração de `.zip`, conferência de hash, ajudantes de coluna do banco, a versão do sistema pelo `go run ./cmd/versao`, ADR 0070). | Os outros scripts de `geral/` (`source`) |
| `windows/comum.ps1` | Pastas, `Etapa`/`Ok`/`Aviso`/`ErroSai`, `VersaoDoSistema` (o `go run ./cmd/versao`, ADR 0070) e `Confiar-No-Certificado` (a loja Root, #775) dos `.ps1` de `windows/` (#655). O `instalador/configurar.ps1` (`.msi`) só usa o `Confiar-No-Certificado`. | Os outros `.ps1` de `windows/` e o `configurar.ps1` (dot-source) |
| `windows/instalar-windows.ps1` | Etapas 1, 2 e 3 no Windows PowerShell 5.1, sem bash: a única implementação das etapas 1 e 3 no Windows. O banco, na etapa 2, pelo mesmo `aplicar-esquema` do Go. | Pessoa, pelo `instalar-windows.cmd` (que pede elevação) ou pelo `instalar.sh` e o `3-publicar-atualizar.sh` no Git Bash; o `atualizar-windows.ps1` |
| `windows/parar-windows.ps1` | O `parar.sh` em PowerShell. | Pessoa, pelo `parar-windows.cmd` ou pelo `parar.sh` no Git Bash |
| `windows/atualizar-windows.ps1` | O `atualizar.sh` no Windows (instalação de desenvolvimento). | Gerenciador |
| `windows/instalar-gerenciador.ps1`, `linux/instalar-gerenciador.sh` | Opcionais, sem administrador/root: compilam o gerenciador em Go (`publicado/gerenciador` no Windows, #430; `publicado/gerenciador-go` no Linux, #277) e criam os atalhos. Rodar de novo atualiza. | Pessoa; o `instalar.sh`; o gerenciador no Linux |

## Windows: o instalador em PowerShell ([ADR 0012](../docs/adr/0012-instalador-do-windows-em-powershell.md))

- **Entradas:** `instalar-windows.cmd` → `windows/instalar-windows.ps1` e `parar-windows.cmd` → `windows/parar-windows.ps1`, os dois pedindo elevação. No Git Bash, o `instalar.sh` e o `3-publicar-atualizar.sh` repassam para o `instalar-windows.ps1`, e o `parar.sh`, para o `parar-windows.ps1` (#655); o Git Bash aberto como Administrador repassa a elevação. A raiz do projeto fica dois níveis acima de cada script.
- **Opções no repasse:** `--pular-build` vira `-PularBuild`; `--sem-gerenciador` não se aplica (no Windows o gerenciador é o `instalar-gerenciador.ps1`, que o `.ps1` anuncia no fim); qualquer outra opção é recusada. Quem roda o `1-preparar-ambiente.sh` no Windows recebe o aviso para usar o `instalar-windows.cmd`.
- **O que o `instalar-windows.ps1` faz, além das etapas:** nenhuma janela de confirmação no meio e, no fim, abre o site no navegador; registra o serviço com `New-Service`/`Win32_Service.Change` e o para antes da cópia (arquivos em uso); numa atualização, copia o banco para `backups/` antes de mudar a estrutura e, se a versão nova não subir, volta a anterior (`publicado/api-anterior`) sozinho; põe o certificado em `LocalMachine\Root` e troca o antigo na renovação pelo `FriendlyName` `FinancasLocal`.
- **Log:** `instaladores\instalacao-windows.log`, que acumula as execuções, com o stderr das ferramentas (npm, Go) e os eventos do Windows quando o site não sobe.
- **Windows PowerShell 5.1:** os `.ps1` não usam `??` nem `-SkipCertificateCheck`, não passam aspas duplas em argumentos de programas externos e não têm acentos no arquivo.
- **O que continua em bash no Windows:** o `2-criar-banco.sh` e o `4-backup.sh` (o backup agendado roda o `4-backup.sh` pelo `bash.exe` do Git, com `schtasks`). Por isso o `geral/comum.sh` ainda tem `caminho_nativo` e `caminho_sqlite` (`cygpath`).
- **Backup agendado com o caminho antigo** (de antes da #154): o `3-publicar-atualizar.sh` (Linux) troca a entrada do crontab, e o `instalar-windows.ps1` (`Migrar-BackupAgendado`) troca a do Agendador de Tarefas, a cada atualização. Quem atualiza pelo gerenciador (`atualizar.sh`, `atualizar-windows.ps1`) passa por eles. Quem pulou todas as versões entre a #154 e a #655 e roda o comando antigo do gerenciador em C# (`git pull && scripts/3-publicar-atualizar.sh`) precisa usar os caminhos de `geral/`.
- **O `parar-windows.ps1`** procura só processos com a pasta do projeto na linha de comando e sai com código 1 se algo ficou rodando.
- **Teste de instalação no Windows** (`.github/workflows/teste-instalacao-windows.yml`): num runner Windows, com o PATH reduzido ao do Windows puro, instala do zero e depois atualiza pelo `.cmd`; confere o repasse do Git Bash (serviço trocado e no ar pelo `.ps1`, `instalar.sh` até o fim, opção desconhecida recusada); para o sistema com `-Desativar` (`--desativar` no Linux), confere que nada ficou de pé e sobe de novo. Roda quando mudam as pastas `windows/`, os `.cmd`, o `instalar.sh`, o `3-publicar-atualizar.sh`, o `comum.sh`, o `2-criar-banco.sh`, `banco/` ou o próprio workflow (filtros por pasta, para um script novo também disparar); para outra mudança, rode-o à mão (Actions › Teste de instalação no Windows › Run workflow). O pedido de elevação (UAC) e o duplo clique continuam sendo teste manual.

## Quem desenvolve (`dev/`)

| Script | O que faz | Quem roda |
|---|---|---|
| `dev/lancar-versao.sh` | Na `release/X.Y.Z`: junta os arquivos de `changelog.d/` numa seção `## [X.Y.Z] — data` do `CHANGELOG.md`, apaga os arquivos e faz o commit `chore: versão X.Y.Z`, depois de conferir a branch, a árvore limpa e que o número calculado é o `X.Y.Z` (ADR 0070). O trabalho é do `src/backend/cmd/versao` (`-publicar`), que tem os testes. | Pessoa (publicar uma versão) |
| `dev/gerar-diagrama-banco.py` | Gera os diagramas do banco de `docs/BANCO-DE-DADOS.md` a partir de `banco/01-estrutura.sql`; com `--conferir`, só confere que estão em dia. | Pessoa (mudou a estrutura); a CI (`--conferir`) |
| `dev/testes-afetados.py` | Escolhe os testes no navegador das telas afetadas pelo que mudou desde o develop, com o que não foi commitado; sozinho, só mostra (#286). A CI roda tudo (#774). | `npm run test:afetados` |
| `dev/medir-desempenho.py` | Mede a partida e o tempo das telas contra uma pasta publicada (`--api <pasta>`), para os requisitos de [docs/REQUISITOS-NAO-FUNCIONAIS.md](../docs/REQUISITOS-NAO-FUNCIONAIS.md). | Pessoa |

## CI (`ci/`)

Na raiz de `ci/`:

| Script | O que faz | Quem roda |
|---|---|---|
| `ci/comum.py` | Código dividido pelos scripts de `ci/`: a lista de arquivos que o PR mudou. Os scripts o importam, e os que estão em subpastas põem a pasta `scripts/ci` no `sys.path` antes (explicado no docstring dele). | Importado pelos scripts de `ci/` |
| `ci/ligar-api.sh` | Liga o site compilado como a publicação o monta e confere que `GET /api/auth/estado` responde (#309). | CI (`ci.yml`, job API e banco); pessoa |

`ci/versao/` (CHANGELOG e notas dos Releases; a versão em si é calculada pelo `src/backend/cmd/versao`, [ADR 0070](../docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md)):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-changelog.py` | Confere o formato de cada arquivo de `changelog.d/` (um título de tipo, entradas, sem versão), exige o arquivo em PR `feat` e `fix` (com `BASE` e `TITULO_PR`); não confere o `CHANGELOG.md` (só o `cmd/versao -publicar` o escreve, #721); avisa (sem barrar) a entrada com mais de ~60 palavras fora do "Técnico" (#722). O teste é o `test_verificar_changelog.py`. | CI (job "manual do site e changelog.d"); pessoa |
| `notas.sh` | Notas dos Releases do GitHub, no limite de 125 mil caracteres, sem o sub-item "Técnico" (#722): `--versao` (tag `vX.Y.Z`, #641, do `CHANGELOG.md`) e `--desenvolvimento` (pré-lançamento, #294 e #600; antes do `CHANGELOG.md`, as seções do que ainda não foi publicado, que o `go run ./cmd/versao -changelog` monta de `changelog.d/`). O teste é o `test_notas.py`, que cobre os dois modos. | CI (workflow "Versão") |

`ci/go/` (back-end e gerenciador em Go, [ADR 0051](../docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-regras-go.py` | As regras de ouro (`float64`, `time.Now()`, `time.Sleep` em teste, #457), com as exceções em `regras-go-excecoes.txt`. O teste é o `test_verificar_regras_go.py`. | CI |
| `verificar-cobertura-go.py` | Catraca da cobertura: o piso de cada pacote (`src/backend/cobertura-minima.txt`) só sobe (#370). | CI |
| `verificar-tempo-testes-go.py` | Trava de tempo dos testes do Go: por teste, por pacote e da suíte (#371). | CI |
| `mutacao-go.py` | Teste de mutação (gremlins) nos pacotes das listas `mutacao-go.txt` e `mutacao-gerenciador-go.txt`, com os equivalentes (`*-equivalentes.txt`) e os fora de propósito (`mutacao-go-fora.txt`). `--conferir-lista` só confere a lista (#373, #456); `--grupos` e `--grupo` dividem a medição por área, um job por grupo no workflow (#673). O pacote novo, fora das duas listas, é medido sem piso, com o aviso da linha a pôr (#716). O teste é o `test_mutacao_go.py`. | CI (workflow "Teste de mutação (Go)"); pessoa |

`ci/front/` (front-end):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-tempo-testes.py` | Trava de tempo do Jest, por arquivo e da suíte (#204). | CI |
| `verificar-cache-estaticos.py` | Confere que a regra do cache longo aceita o hash do build do Angular e nada além (#305). | CI |
| `verificar-manual.py` | Avisa quando o PR mexe no que o usuário vê e não atualiza o Manual (#177). | CI |

`ci/banco/` (banco):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-sql.py` | Confere que os scripts de `banco/` são idempotentes e sem sintaxe de T-SQL. | CI |
