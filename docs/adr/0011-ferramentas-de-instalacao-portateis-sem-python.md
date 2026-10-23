# 0011. Ferramentas de instalação portáteis, sem depender de Python

- **Status:** Aceita (vai ser substituída em parte pela [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md): o Go portátil entrou nos scripts na #231, ao lado do SDK do .NET, que sai na #86)
- **Data:** 2026-10-16 (atualizada em 2026-10-23: Go portátil, #231)
- **Relacionadas:** [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (scripts bash nos dois sistemas); [0012](0012-instalador-do-windows-em-powershell.md) (instalador do Windows sem bash)

## Contexto

Os scripts de instalação (`scripts/*.sh`) baixavam o .NET e o Node.js portáteis, mas esperavam que o computador já tivesse `python3` (para ler JSON dos metadados do .NET e gerar o `appsettings.Production.json`), `sqlite3` e `unzip`. Num Windows novo, com só o Git for Windows instalado, nada disso existe:

- `python3` costuma ser apenas o atalho da Microsoft Store (`WindowsApps\python3.exe`): `command -v python3` encontra, mas a execução falha com "Python was not found";
- `sqlite3` não vem com o Windows, e o script só mostrava como instalar;
- o Git Bash nem sempre traz o `unzip` (*inferido*: depende da versão/opções do instalador do Git for Windows).

O resultado eram vários erros seguidos na primeira instalação.

## Alternativas consideradas

- **Instalar Python e sqlite3 pelo `winget`** — usa o gerenciador oficial do Windows; mas o `winget` pode não existir (Windows antigo/corporativo), a instalação vai para fora da pasta do projeto, pede elevação em alguns casos e o PATH novo só vale num terminal novo.
- **Python portátil baixado pelo script** — mantém o código Python; mas é mais um runtime de dezenas de MB só para ler/escrever JSON.
- **Usar o Node.js portátil (que já era baixado) como interpretador de JSON, e baixar o sqlite3 portátil oficial** — nenhum runtime novo; tudo fica em `ferramentas/`, como o .NET e o Node.

## Decisão

Os scripts não dependem mais de `python3` nem de `jq`: o `1-preparar-ambiente.sh` instala primeiro o Node.js (o índice de versões é lido do `index.tab`, só com `awk`) e passa a usá-lo para ler os metadados do .NET e conferir hashes SHA3-256; o `3-publicar-atualizar.sh` usa o Node para ler e gravar o `appsettings.Production.json`. Quando o sistema não tem `sqlite3`, ele é baixado de `www.sqlite.org` (pacote `sqlite-tools` oficial) para `ferramentas/sqlite`, que `comum.sh` põe no PATH de todos os scripts. Sem `unzip` no Windows, os `.zip` são extraídos pelo PowerShell (`Expand-Archive`).

**Go portátil (#231, [ADR 0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)).** O back-end em Go roda atrás da API e é compilado na instalação, no mesmo padrão:
- a versão é a linha do `src/backend-go/go.mod` (`go 1.27.3` → a 1.27.x mais nova, nunca mais velha que a do `go.mod`), lida do índice oficial `https://go.dev/dl/?mode=json` pelo Node (no bash) ou pelo `Invoke-RestMethod` (no `instalar-windows.ps1`);
- o arquivo vem de `https://dl.google.com/go/`, para onde o `go.dev/dl` redireciona, e o **SHA-256 do próprio índice** é conferido antes de extrair em `ferramentas/go`;
- um `go` do PATH da mesma linha e não mais velho é reaproveitado, como o `dotnet` 10.x;
- `GOTOOLCHAIN=local`: o Go nunca baixa outra versão de si mesmo por conta própria;
- os módulos (`modernc.org/sqlite` e os outros do `go.mod`) vêm de `proxy.golang.org`, conferidos pelo `go.sum` do repositório. O cache de módulos e o de compilação ficam em `ferramentas/go-caminho` e `ferramentas/go-cache` (apagáveis com `rm -rf`: `-modcacherw`);
- sem `go mod vendor`: a pasta teria 137 MB no repositório (o `go.sum` já garante a integridade dos módulos).

Para instalar em um passo, `scripts/instalar.sh` roda as três etapas em sequência. No Windows, o instalador de dois cliques não usa bash: ver [0012](0012-instalador-do-windows-em-powershell.md), que segue a mesma estratégia de ferramentas portáteis.

## Consequências

- **Positivas:** num Windows novo basta o Git for Windows; o primeiro script instala o resto e confere tudo no final (`dotnet`, `go`, `node`, `npm`, `sqlite3`). O mesmo vale para Linux sem `sqlite3` (x64).
- **Negativas:**
  - novo domínio de download na instalação: `www.sqlite.org` (HTTPS, hash SHA3-256 publicado na própria página de download, que é lida para descobrir a versão — se o sqlite.org mudar o formato do bloco `PRODUCT,...`, o download automático quebra e é preciso instalar o `sqlite3` à mão);
  - o hash do sqlite3 vem da mesma origem que o arquivo (não há assinatura separada), proteção igual à do `SHASUMS256.txt` do Node;
  - o sqlite3 portátil não se atualiza sozinho: se já existir um `sqlite3` (do sistema ou de `ferramentas/sqlite`), ele é mantido;
  - Linux arm64 não tem `sqlite-tools` publicado: lá o `sqlite3` continua vindo do gerenciador de pacotes;
  - o `scripts/ci/verificar-sql.py` continua em Python, mas roda só no CI;
  - **domínios novos na instalação (#231):** `go.dev` (índice de versões), `dl.google.com` (o Go) e `proxy.golang.org` (os módulos). O hash do Go vem da mesma origem que o arquivo, como o do Node e o do sqlite3;
  - **mais espaço em disco enquanto o .NET e o Go convivem** (medido no Linux x64): 276 MB do Go portátil, 68 MB do arquivo baixado, 306 MB do cache de módulos e 155 MB do cache de compilação. Some quando o .NET sair e o executável do Go vier pronto (#84, #86).
- **Na prática:** o workflow `Teste de instalação no Linux` (`.github/workflows/teste-instalacao-linux.yml`) roda `instalar.sh` num runner Ubuntu com um PATH sem `dotnet`, `node`, `go`, `sqlite3`, `python3` e `jq` (instalação do zero, atualização e backup) sempre que um `scripts/*.sh` ou `banco/` mudar — se um script voltar a depender de Python, ele falha. Em script novo, não usar `python3`/`jq`; para JSON, usar `node -e` com caminhos convertidos por `caminho_nativo` (no Git Bash, `/c/...` não é entendido pelo `node.exe` dentro de variáveis de ambiente). Para `.zip`, usar `extrair_zip` de `comum.sh`.
