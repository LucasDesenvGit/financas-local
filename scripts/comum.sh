#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - funcoes compartilhadas pelos scripts de instalacao.
# Carregado com 'source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"' (nao execute
# diretamente). Funciona em Linux, macOS e Windows (via Git Bash/MSYS ou WSL).
# =====================================================================

set -euo pipefail

# ---------------------------------------------------------------- Deteccao de SO e arquitetura
case "$(uname -s)" in
    Linux*)                SO=linux ;;
    Darwin*)               SO=osx ;;
    MINGW*|MSYS*|CYGWIN*)  SO=windows ;;
    *)
        echo "Sistema operacional nao suportado: $(uname -s)" >&2
        exit 1
        ;;
esac

case "$(uname -m)" in
    x86_64|amd64)   ARCH=x64 ;;
    arm64|aarch64)  ARCH=arm64 ;;
    *)
        echo "Arquitetura nao suportada: $(uname -m)" >&2
        exit 1
        ;;
esac

# ---------------------------------------------------------------- Pastas (mesma convencao do antigo comum.ps1)
Raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
Ferramentas="$Raiz/ferramentas"
Instaladores="$Raiz/instaladores"
Dados="$Raiz/dados"
PastaSql="$Dados/sql"
PastaChaves="$Dados/chaves"
PastaBackups="$Raiz/backups"
PastaPublicado="$Raiz/publicado"

# sqlite3 portatil (baixado pelo 1-preparar-ambiente.sh quando o sistema nao tem
# um) vale para todos os scripts, inclusive 2-criar-banco.sh e 4-backup.sh.
if [[ -d "$Ferramentas/sqlite" ]]; then
    export PATH="$Ferramentas/sqlite:$PATH"
fi

# ---------------------------------------------------------------- Mensagens
etapa() {
    printf '\n\033[36m==> %s\033[0m\n' "$1"
}

ok() {
    printf '\033[32m    OK  %s\033[0m\n' "$1"
}

aviso() {
    printf '\033[33m    !!  %s\033[0m\n' "$1"
}

erro_sai() {
    printf '\033[31m    ERRO  %s\033[0m\n' "$1" >&2
    exit 1
}

# ---------------------------------------------------------------- Utilitarios de pasta/arquivo
garantir_pasta() {
    mkdir -p "$1"
}

baixar_arquivo() {
    local url="$1" destino="$2"
    [[ "$url" == https://* ]] || erro_sai "Somente downloads via HTTPS sao permitidos: $url"
    echo "    baixando $url"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$destino"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$destino"
    else
        erro_sai "Nem curl nem wget estao disponiveis para baixar $url"
    fi
}

# Caminho no formato que programas nativos entendem. No Git Bash, $Raiz fica no
# formato POSIX (ex.: /c/FinancasLocal), que node.exe/dotnet.exe nao entendem
# quando o caminho vai dentro de variavel de ambiente ou de um arquivo; la ele
# vira o formato misto (C:/FinancasLocal).
caminho_nativo() {
    if [[ "$SO" == windows ]]; then cygpath -m "$1"; else echo "$1"; fi
}

# extrair_zip arquivo.zip pasta_destino
# O Git Bash nem sempre traz o unzip; no Windows o PowerShell (Expand-Archive)
# esta sempre disponivel e serve de alternativa.
extrair_zip() {
    local arquivo="$1" destino="$2"
    garantir_pasta "$destino"
    if command -v unzip >/dev/null 2>&1; then
        unzip -q -o "$arquivo" -d "$destino"
    elif [[ "$SO" == windows ]] && command -v powershell.exe >/dev/null 2>&1; then
        ORIGEM_ZIP="$(cygpath -w "$arquivo")" DESTINO_ZIP="$(cygpath -w "$destino")" \
            powershell.exe -NoProfile -NonInteractive -Command \
            '$ErrorActionPreference = "Stop"; $ProgressPreference = "SilentlyContinue"; Expand-Archive -LiteralPath $env:ORIGEM_ZIP -DestinationPath $env:DESTINO_ZIP -Force'
    else
        erro_sai "unzip nao encontrado para extrair $(basename "$arquivo"). Instale o unzip e rode de novo."
    fi
}

# verificar_hash arquivo hash_esperado algoritmo(sha256|sha512|sha3-256)
# sha3-256 (usado pelo sqlite.org) e calculado pelo Node.js, que o
# 1-preparar-ambiente.sh instala antes de precisar dele.
verificar_hash() {
    local arquivo="$1" esperado="$2" algoritmo="$3" hash=""
    case "$algoritmo" in
        sha256)
            if command -v sha256sum >/dev/null 2>&1; then
                hash="$(sha256sum "$arquivo" | awk '{print $1}')"
            elif command -v shasum >/dev/null 2>&1; then
                hash="$(shasum -a 256 "$arquivo" | awk '{print $1}')"
            else
                erro_sai 'Nenhuma ferramenta de SHA-256 disponivel (sha256sum/shasum).'
            fi
            ;;
        sha512)
            if command -v sha512sum >/dev/null 2>&1; then
                hash="$(sha512sum "$arquivo" | awk '{print $1}')"
            elif command -v shasum >/dev/null 2>&1; then
                hash="$(shasum -a 512 "$arquivo" | awk '{print $1}')"
            else
                erro_sai 'Nenhuma ferramenta de SHA-512 disponivel (sha512sum/shasum).'
            fi
            ;;
        sha3-256)
            requer_comando node 'Node.js nao encontrado (necessario para conferir o hash SHA3-256).'
            hash="$(node -e 'const c = require("crypto"), fs = require("fs"); process.stdout.write(c.createHash("sha3-256").update(fs.readFileSync(process.argv[1])).digest("hex"));' "$(caminho_nativo "$arquivo")")"
            ;;
        *)
            erro_sai "Algoritmo de hash desconhecido: $algoritmo"
            ;;
    esac
    if [[ "${hash,,}" != "${esperado,,}" ]]; then
        rm -f "$arquivo"
        erro_sai "Hash $algoritmo nao confere para $(basename "$arquivo"). O arquivo foi apagado por seguranca."
    fi
    ok "hash $algoritmo conferido: $(basename "$arquivo")"
}

# Atalho usado pelos scripts (mantém o nome pedido na convenção do projeto).
verificar_sha256() {
    verificar_hash "$1" "$2" sha256
}

# requer_comando nome_do_comando mensagem_de_instalacao
requer_comando() {
    local nome="$1" mensagem="$2"
    command -v "$nome" >/dev/null 2>&1 || erro_sai "$mensagem"
}

# ---------------------------------------------------------------- Ambiente de build (dotnet/node portateis ou do PATH)
configurar_ambiente_build() {
    local dotnet_bin=dotnet node_bin=node
    if [[ "$SO" == windows ]]; then dotnet_bin=dotnet.exe; node_bin=node.exe; fi

    if [[ -x "$Ferramentas/dotnet/$dotnet_bin" ]]; then
        export PATH="$Ferramentas/dotnet:$PATH"
        export DOTNET_ROOT="$Ferramentas/dotnet"
    elif ! command -v dotnet >/dev/null 2>&1; then
        erro_sai 'SDK do .NET nao encontrado. Rode antes o 1-preparar-ambiente.sh.'
    fi

    if [[ -x "$Ferramentas/node/bin/$node_bin" ]]; then
        export PATH="$Ferramentas/node/bin:$PATH"
    elif [[ -x "$Ferramentas/node/$node_bin" ]]; then
        export PATH="$Ferramentas/node:$PATH"
    elif ! command -v node >/dev/null 2>&1; then
        erro_sai 'Node.js nao encontrado. Rode antes o 1-preparar-ambiente.sh.'
    fi

    export DOTNET_CLI_HOME="$Ferramentas/dotnet-home"
    export NUGET_PACKAGES="$Ferramentas/nuget-pacotes"
    export DOTNET_CLI_TELEMETRY_OPTOUT=1
    export DOTNET_NOLOGO=1
    export DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1
    export npm_config_cache="$Ferramentas/npm-cache"
    export npm_config_update_notifier=false
    export NG_CLI_ANALYTICS=false
}

# ---------------------------------------------------------------- Evolucao de schema do SQLite
# O SQLite nao tem "ALTER TABLE ... ADD COLUMN IF NOT EXISTS". Quando uma versao
# futura de 2-criar-banco.sh (ou um novo banco/NN-nome.sql rodado via sqlite3)
# precisar adicionar uma coluna em uma tabela existente sem quebrar quem ja tem
# o banco criado, use estes dois helpers. Ambos dependem da variavel $banco
# (caminho do arquivo .db) ja estar definida no script chamador. Exemplo:
#   adicionar_coluna_se_ausente "Transacoes" "Observacao" "TEXT"
coluna_existe() {
    local tabela="$1" coluna="$2" colunas
    # Le a lista inteira antes de comparar (em vez de canalizar direto para um
    # "grep -qx", que fecha o pipe cedo e poderia, com pipefail, propagar um
    # SIGPIPE do sqlite3 como falha espuria).
    colunas="$(sqlite3 "$banco" "PRAGMA table_info($tabela);" | cut -d'|' -f2)"
    grep -qx "$coluna" <<<"$colunas"
}

adicionar_coluna_se_ausente() {
    local tabela="$1" coluna="$2" definicao="$3"
    if ! coluna_existe "$tabela" "$coluna"; then
        sqlite3 "$banco" "ALTER TABLE $tabela ADD COLUMN $coluna $definicao;"
        ok "coluna $coluna adicionada em $tabela"
    fi
}
