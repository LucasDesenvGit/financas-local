#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - funcoes compartilhadas pelos scripts de instalacao.
# Carregado com 'source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"' (nao execute
# diretamente). Funciona em Linux e macOS. No Windows (Git Bash/MSYS) so o 2-criar-banco.sh e o 4-backup.sh
# rodam em bash; o instalar.sh, o 3-publicar-atualizar.sh e o parar.sh repassam para o PowerShell (ADR 0012, #655).
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
Raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
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
# formato POSIX (ex.: /c/FinancasLocal), que o powershell.exe nao entende; la ele
# vira o formato misto (C:/FinancasLocal).
caminho_nativo() {
    if [[ "$SO" == windows ]]; then cygpath -m "$1"; else echo "$1"; fi
}

# repassar_para_powershell script.ps1 [argumentos]: no Git Bash, a logica do Windows existe num lugar so, o .ps1 de
# scripts/windows (ADR 0012, #655). Substitui o processo: o codigo de saida e o do PowerShell.
repassar_para_powershell() {
    local script="$1"; shift
    exec powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(caminho_nativo "$Raiz/scripts/windows/$script")" "$@"
}

# extrair_zip arquivo.zip pasta_destino
extrair_zip() {
    local arquivo="$1" destino="$2"
    garantir_pasta "$destino"
    command -v unzip >/dev/null 2>&1 || erro_sai "unzip nao encontrado para extrair $(basename "$arquivo"). Instale o unzip e rode de novo."
    unzip -q -o "$arquivo" -d "$destino"
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

# ---------------------------------------------------------------- Porta ocupada (#300)
# quem_usa_a_porta PORTA -> "nome (PID n)" de quem escuta na porta, ou nada se ela esta livre.
# Linux: ss; macOS: lsof. Sem a ferramenta, nada (nao bloqueia).
quem_usa_a_porta() {
    local porta="$1" pid="" nome=""
    case "$SO" in
        linux)
            local linha
            linha="$(ss -ltnpH "sport = :$porta" 2>/dev/null | head -n1 || true)"
            [[ -n "$linha" ]] || return 0
            pid="$(grep -oE 'pid=[0-9]+' <<<"$linha" | head -n1 | cut -d= -f2 || true)"
            nome="$(grep -oE '\(\("[^"]+"' <<<"$linha" | head -n1 | cut -d'"' -f2 || true)"
            ;;
        osx)
            pid="$(lsof -nP -iTCP:"$porta" -sTCP:LISTEN -t 2>/dev/null | head -n1 || true)"
            [[ -n "$pid" ]] || return 0
            nome="$(ps -p "$pid" -o comm= 2>/dev/null || true)"
            ;;
        *) return 0 ;;
    esac
    if [[ -n "$pid" ]]; then
        echo "${nome:-programa desconhecido} (PID $pid)"
    else
        echo "outro programa"
    fi
}

# ---------------------------------------------------------------- Ambiente de build (node/go portateis ou do PATH)
configurar_ambiente_build() {
    if [[ -x "$Ferramentas/node/bin/node" ]]; then
        export PATH="$Ferramentas/node/bin:$PATH"
    elif ! command -v node >/dev/null 2>&1; then
        erro_sai 'Node.js nao encontrado. Rode antes o 1-preparar-ambiente.sh.'
    fi

    # Go (#231): o portatil, se houver; senao, o do PATH (o 1-preparar-ambiente.sh confere a versao). So o
    # 3-publicar-atualizar.sh precisa dele, e reclama se faltar.
    if [[ -x "$Ferramentas/go/bin/go" ]]; then
        export PATH="$Ferramentas/go/bin:$PATH"
    fi
    # Modulos e cache de compilacao dentro de ferramentas/ (como o npm). GOTOOLCHAIN=local: nunca baixar
    # outra versao do Go por conta propria. -modcacherw: o cache de modulos fica apagavel com rm -rf.
    # CGO_ENABLED=0: o executavel nao depende de compilador C (o SQLite e o modernc.org/sqlite, em Go puro).
    # (caminho_nativo: o go.exe nao entende /c/... dentro de variavel de ambiente.)
    GOPATH="$(caminho_nativo "$Ferramentas/go-caminho")"
    GOCACHE="$(caminho_nativo "$Ferramentas/go-cache")"
    export GOPATH GOCACHE
    export GOTOOLCHAIN=local
    export GOFLAGS=-modcacherw
    export CGO_ENABLED=0

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
# .timeout: se a API (no ar durante uma atualizacao) estiver gravando, o sqlite3 espera ate 15 s
# em vez de falhar com "database is locked". Uso: sqlite3 "${SQLITE_ESPERA[@]}" "$banco" ...
SQLITE_ESPERA=(-cmd '.timeout 15000')

# caminho_sqlite <caminho>: o caminho para DENTRO de um comando do sqlite3 (.backup "..."), entre as aspas duplas. No
# Windows, o Git Bash so converte /c/... em C:/... quando o caminho e um argumento de um programa do Windows, e nao
# dentro de um texto: o sqlite3 do Windows nao abre /c/Users/... ("cannot open"). O escape e o do shell do sqlite3, que
# so tem \ e \" dentro de "...".
caminho_sqlite() {
    local c="$1"
    if [[ "$SO" == windows ]] && command -v cygpath >/dev/null 2>&1; then c="$(cygpath -m "$c")"; fi
    c=${c//\\/\\\\}
    c=${c//\"/\\\"}
    printf '%s' "$c"
}

coluna_existe() {
    local tabela="$1" coluna="$2" colunas
    # Le a lista inteira antes de comparar (em vez de canalizar direto para um
    # "grep -qx", que fecha o pipe cedo e poderia, com pipefail, propagar um
    # SIGPIPE do sqlite3 como falha espuria).
    colunas="$(sqlite3 "${SQLITE_ESPERA[@]}" "$banco" "PRAGMA table_info($tabela);" | cut -d'|' -f2)"
    grep -qx "$coluna" <<<"$colunas"
}

adicionar_coluna_se_ausente() {
    local tabela="$1" coluna="$2" definicao="$3"
    if ! coluna_existe "$tabela" "$coluna"; then
        sqlite3 "${SQLITE_ESPERA[@]}" "$banco" "ALTER TABLE $tabela ADD COLUMN $coluna $definicao;"
        ok "coluna $coluna adicionada em $tabela"
    fi
}
