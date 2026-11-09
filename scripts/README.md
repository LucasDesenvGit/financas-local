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
| `geral/2-criar-banco.sh` | Cria as pastas de `dados/` com permissão restrita e aplica os arquivos de `banco/` em ordem. Também é lido pelo site (cópia embutida em `internal/plataforma/banco/esquema`) e pelo `instalar-windows.ps1`, que tiram dele a lista de colunas novas. | Pessoa (etapa 2); o `go generate` e a CI leem |
| `geral/3-publicar-atualizar.sh` | Compila o front-end e o back-end em Go (`financas-go`), publica em `publicado/api`, gera ou reaproveita o certificado HTTPS local e registra o serviço (systemd `--user`). No Git Bash do Windows, repassa para o `windows/instalar-windows.ps1`. | Pessoa (etapa 3); `atualizar.sh`; a CI |
| `geral/4-backup.sh` | Backup online do banco (`sqlite3 .backup`), com retenção e opção de agendar (`--agendar`). | Pessoa; o crontab ou o Agendador de Tarefas; a CI |
| `geral/atualizar.sh` | Só na instalação de desenvolvimento (clone na branch `develop`, [ADR 0040](../docs/adr/0040-versao-de-desenvolvimento-pelo-gerenciador.md)): traz o `develop` (só avança, recusa com alteração local ou outra branch) e reinstala. | Gerenciador (botão "Atualizar para a última do develop") |
| `geral/parar.sh` | Para o serviço, o site fora dele e o `npm start`; no Git Bash, repassa para o `windows/parar-windows.ps1`. | Pessoa |
| `geral/comum.sh` | Funções compartilhadas pelos scripts (detecção de SO, pastas, `sqlite3` portátil no PATH, repasse para o PowerShell e caminho nativo no Windows, extração de `.zip`, conferência de hash, ajudantes de coluna do banco). | Os outros scripts de `geral/` (`source`) |
| `windows/comum.ps1` | Pastas e `Etapa`/`Ok`/`Aviso`/`ErroSai` dos `.ps1` de `windows/` (#655). O `instalador/configurar.ps1` (`.msi`) não o usa. | Os outros `.ps1` de `windows/` (dot-source) |
| `windows/instalar-windows.ps1` | Etapas 1, 2 e 3 no Windows PowerShell 5.1, sem bash: a única implementação das etapas 1 e 3 no Windows. Lê a lista de colunas novas do `geral/2-criar-banco.sh`. | Pessoa, pelo `instalar-windows.cmd` (que pede elevação) ou pelo `instalar.sh` e o `3-publicar-atualizar.sh` no Git Bash; o `atualizar-windows.ps1` |
| `windows/parar-windows.ps1` | O `parar.sh` em PowerShell. | Pessoa, pelo `parar-windows.cmd` ou pelo `parar.sh` no Git Bash |
| `windows/atualizar-windows.ps1` | O `atualizar.sh` no Windows (instalação de desenvolvimento). | Gerenciador |
| `windows/instalar-gerenciador.ps1`, `linux/instalar-gerenciador.sh` | Opcionais, sem administrador/root: compilam o gerenciador em Go (`publicado/gerenciador` no Windows, #430; `publicado/gerenciador-go` no Linux, #277) e criam os atalhos. Rodar de novo atualiza. | Pessoa; o `instalar.sh`; o gerenciador no Linux |

## Quem desenvolve (`dev/`)

| Script | O que faz | Quem roda |
|---|---|---|
| `dev/atualizar-pr.sh` | Traz o `develop` para a branch do PR e resolve sozinho o conflito de versão e de CHANGELOG (#587, ADR 0039). | Pessoa |
| `dev/atualizar-versao-do-pr.py` | A conta por baixo do `atualizar-pr.sh`: a versão nova a partir da do `develop` e a seção do PR no topo do CHANGELOG. O teste é o `dev/test_atualizar_versao_do_pr.py`. | O `atualizar-pr.sh`; a CI (o teste) |
| `dev/gerar-diagrama-banco.py` | Gera os diagramas do banco de `docs/BANCO-DE-DADOS.md` a partir de `banco/01-estrutura.sql`; com `--conferir`, só confere que estão em dia. | Pessoa (mudou a estrutura); a CI (`--conferir`) |
| `dev/medir-desempenho.py` | Mede a partida e o tempo das telas contra uma pasta publicada (`--api <pasta>`), para os requisitos de [docs/REQUISITOS-NAO-FUNCIONAIS.md](../docs/REQUISITOS-NAO-FUNCIONAIS.md). | Pessoa |

## CI (`ci/`)

Na raiz de `ci/`:

| Script | O que faz | Quem roda |
|---|---|---|
| `ci/comum.py` | Código dividido pelos scripts de `ci/` e `dev/`: ler o `<Version>` do `Directory.Build.props`, a regra "`feat` sobe o do meio, o resto sobe o último" e a lista de arquivos que o PR mudou. Os scripts o importam, e os que estão em `dev/` ou numa subpasta põem `scripts/ci` no `sys.path`. | Os outros scripts (importado) |
| `ci/ligar-api.sh` | Liga o site compilado como a publicação o monta e confere que `GET /api/auth/estado` responde (#309). | CI (`ci.yml`, job API e banco); pessoa |

`ci/versao/` (versão e CHANGELOG, [ADR 0039](../docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md)):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-versao.py` | Falha o PR se a versão do `Directory.Build.props` não subiu o que o tipo do título pede. | CI; `dev/atualizar-pr.sh` |
| `verificar-changelog.py` | Confere uma seção por versão e um título de cada tipo dentro dela; `--corrigir` junta os repetidos (#294). | CI; pessoa (`--corrigir`) |
| `notas.sh` | Notas dos Releases do GitHub, no limite de 125 mil caracteres: `--versao` (tag `vX.Y.Z`, #641) e `--desenvolvimento` (pré-lançamento, #294 e #600). O teste é o `test_notas.py`, que cobre os dois modos. | CI (workflow "Versão") |

`ci/go/` (back-end e gerenciador em Go, [ADR 0051](../docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-regras-go.py` | As regras de ouro (`float64`, `time.Now()`, `time.Sleep` em teste, #457), com as exceções em `regras-go-excecoes.txt`. O teste é o `test_verificar_regras_go.py`. | CI |
| `verificar-cobertura-go.py` | Catraca da cobertura: o piso de cada pacote (`src/backend/cobertura-minima.txt`) só sobe (#370). | CI |
| `verificar-tempo-testes-go.py` | Trava de tempo dos testes do Go: por teste, por pacote e da suíte (#371). | CI |
| `mutacao-go.py` | Teste de mutação (gremlins) nos pacotes das listas `mutacao-go.txt` e `mutacao-gerenciador-go.txt`, com os equivalentes (`*-equivalentes.txt`) e os fora de propósito (`mutacao-go-fora.txt`). `--conferir-lista` só confere a lista (#373, #456); `--grupos` e `--grupo` dividem a medição por área, um job por grupo no workflow (#673). | CI (workflow "Teste de mutação (Go)"); pessoa |

`ci/front/` (front-end):

| Script | O que faz | Quem roda |
|---|---|---|
| `testes-afetados.py` | Descobre o que rodar (Jest e navegador) pelo que o PR mudou; `--base` mostra ou roda (#255, #286). | CI; `npm run test:afetados` |
| `verificar-tempo-testes.py` | Trava de tempo do Jest, por arquivo e da suíte (#204). | CI |
| `verificar-cache-estaticos.py` | Confere que a regra do cache longo aceita o hash do build do Angular e nada além (#305). | CI |
| `verificar-manual.py` | Avisa quando o PR mexe no que o usuário vê e não atualiza o Manual (#177). | CI |

`ci/banco/` (banco):

| Script | O que faz | Quem roda |
|---|---|---|
| `verificar-sql.py` | Confere que os scripts de `banco/` são idempotentes e sem sintaxe de T-SQL. | CI |
