#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - 1) Preparacao do ambiente
#
#  - Confere as ferramentas basicas do terminal (curl, tar, sha256sum...)
#  - Node.js LTS portatil em ferramentas/node (compila o Angular e tambem
#    interpreta JSON nos scripts, no lugar do python3/jq)
#  - .NET 10 SDK portatil em ferramentas/dotnet (reaproveita um dotnet 10.x
#    que ja esteja no PATH, se houver)
#  - sqlite3: usa o do sistema se houver; senao baixa o portatil oficial
#    (sqlite.org) em ferramentas/sqlite
#
# Tudo e baixado so por HTTPS e conferido pelo hash publicado pela origem.
# Sem IIS e sem SQL Server Express: o Kestrel hospeda a API diretamente e o
# banco e SQLite. Funciona em Linux, macOS e Windows (Git Bash ou WSL).
# Pode ser executado de novo: so baixa o que estiver faltando ou desatualizado.
#
# Uso:
#   scripts/1-preparar-ambiente.sh
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "FinancasLocal - preparacao do ambiente em $Raiz ($SO/$ARCH)"

garantir_pasta "$Ferramentas"
garantir_pasta "$Instaladores"

# ------------------------------------------------------------------ Pre-requisitos do terminal
etapa 'Verificando as ferramentas basicas do terminal'
faltando=()
for cmd in curl tar awk sed grep find; do
    command -v "$cmd" >/dev/null 2>&1 || faltando+=("$cmd")
done
# tar -xz (pacotes .tar.gz do Linux/macOS) depende do gzip.
[[ "$SO" == windows ]] || command -v gzip >/dev/null 2>&1 || faltando+=("gzip")
command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || faltando+=("sha256sum")
command -v sha512sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || faltando+=("sha512sum")
if ! command -v unzip >/dev/null 2>&1; then
    if [[ "$SO" == windows ]] && command -v powershell.exe >/dev/null 2>&1; then
        aviso 'unzip nao encontrado; os pacotes .zip serao extraidos pelo PowerShell.'
    else
        faltando+=("unzip")
    fi
fi
if [[ ${#faltando[@]} -gt 0 ]]; then
    case "$SO" in
        windows) dica='Use o Git Bash do Git for Windows (winget install Git.Git), que ja traz todas elas.' ;;
        linux)   dica='Instale pelo gerenciador de pacotes da distro (ex.: sudo apt install curl tar unzip coreutils).' ;;
        osx)     dica='Instale com o Homebrew (ex.: brew install coreutils).' ;;
    esac
    erro_sai "Ferramentas ausentes: ${faltando[*]}. $dica"
fi
if [[ "$SO" == windows ]]; then
    command -v cygpath >/dev/null 2>&1 || erro_sai 'cygpath nao encontrado. Rode os scripts pelo Git Bash (Git for Windows), nao pelo PowerShell/cmd.'
    if [[ "$Raiz" == *' '* ]]; then
        aviso "O caminho da instalacao tem espacos ($Raiz). Funciona, mas se algo falhar, mova a pasta para um caminho sem espacos (ex.: C:/FinancasLocal)."
    fi
fi
ok 'ferramentas basicas disponiveis'

# ------------------------------------------------------------------ Node.js (build do Angular e leitura de JSON nos scripts)
etapa 'Verificando o Node.js'
destino_node="$Ferramentas/node"

case "$SO-$ARCH" in
    linux-x64)     chave_arquivos_node="linux-x64";     sufixo_node="linux-x64" ;;
    linux-arm64)   chave_arquivos_node="linux-arm64";   sufixo_node="linux-arm64" ;;
    osx-x64)       chave_arquivos_node="osx-x64-tar";   sufixo_node="darwin-x64" ;;
    osx-arm64)     chave_arquivos_node="osx-arm64-tar"; sufixo_node="darwin-arm64" ;;
    windows-x64)   chave_arquivos_node="win-x64-zip";   sufixo_node="win-x64" ;;
    windows-arm64) chave_arquivos_node="win-arm64-zip"; sufixo_node="win-arm64" ;;
    *) erro_sai "Combinacao de SO/arquitetura sem pacote Node.js conhecido: $SO/$ARCH" ;;
esac

# index.tab e o mesmo indice do index.json em texto separado por tabulacao
# (versao, data, arquivos, ..., lts na 10a coluna): da para ler so com awk.
# Versoes aceitas pelo Angular 20: 20, 22 e 24 (a mais nova vem primeiro).
indice_node="$(curl -fsSL 'https://nodejs.org/dist/index.tab')" || erro_sai 'Nao foi possivel obter o indice de versoes do Node.js (sem internet?).'
node_versao="$(awk -F'\t' -v chave="$chave_arquivos_node" '
    NR > 1 && $10 != "-" {
        split($1, partes, "."); maior = substr(partes[1], 2) + 0
        if (maior != 20 && maior != 22 && maior != 24) next
        n = split($3, arquivos, ",")
        for (i = 1; i <= n; i++) if (arquivos[i] == chave) { print $1; exit }
    }' <<<"$indice_node")"
[[ -n "$node_versao" ]] || erro_sai 'Nenhuma versao LTS compativel do Node.js encontrada.'

versao_instalada=""
[[ -x "$destino_node/bin/node" ]] && versao_instalada="$("$destino_node/bin/node" --version)"
[[ -x "$destino_node/node.exe" ]] && versao_instalada="$("$destino_node/node.exe" --version)"

if [[ "$versao_instalada" == "$node_versao" ]]; then
    ok "Node.js $node_versao ja presente"
else
    extensao_node="tar.gz"; [[ "$SO" == windows ]] && extensao_node="zip"
    nome_pacote_node="node-$node_versao-$sufixo_node.$extensao_node"
    arq_node="$Instaladores/$nome_pacote_node"
    arq_shas="$Instaladores/node-$node_versao-SHASUMS256.txt"
    [[ -f "$arq_node" ]] || baixar_arquivo "https://nodejs.org/dist/$node_versao/$nome_pacote_node" "$arq_node"
    baixar_arquivo "https://nodejs.org/dist/$node_versao/SHASUMS256.txt" "$arq_shas"
    hash_esperado="$( (grep " $nome_pacote_node\$" "$arq_shas" || true) | awk '{print $1}')"
    [[ -n "$hash_esperado" ]] || erro_sai 'Hash do Node.js nao encontrado no SHASUMS256.txt.'
    verificar_sha256 "$arq_node" "$hash_esperado"

    etapa 'Extraindo o Node.js'
    pasta_tmp="$Instaladores/node-extracao"
    rm -rf "$pasta_tmp"
    garantir_pasta "$pasta_tmp"
    if [[ "$SO" == windows ]]; then
        extrair_zip "$arq_node" "$pasta_tmp"
    else
        tar -xzf "$arq_node" -C "$pasta_tmp"
    fi
    rm -rf "$destino_node"
    mv "$pasta_tmp/node-$node_versao-$sufixo_node" "$destino_node"
    rm -rf "$pasta_tmp"
    ok "Node.js $node_versao em $destino_node"
fi

# Daqui em diante o node portatil e usado para ler JSON e conferir SHA3-256.
if [[ "$SO" == windows ]]; then
    export PATH="$destino_node:$PATH"
else
    export PATH="$destino_node/bin:$PATH"
fi
node --version >/dev/null 2>&1 || erro_sai "O Node.js em $destino_node nao executa. Apague a pasta e rode de novo."

# ------------------------------------------------------------------ .NET 10 SDK
etapa 'Verificando o SDK do .NET'
usar_dotnet_do_path=0
if command -v dotnet >/dev/null 2>&1 && dotnet --list-sdks 2>/dev/null | grep -qE '^10\.'; then
    ok "SDK do .NET 10 ja disponivel no PATH ($(command -v dotnet))"
    usar_dotnet_do_path=1
fi

if [[ "$usar_dotnet_do_path" -eq 0 ]]; then
    destino_dotnet="$Ferramentas/dotnet"

    case "$SO" in
        linux)   nome_pacote_dotnet="dotnet-sdk-linux-$ARCH.tar.gz" ;;
        osx)     nome_pacote_dotnet="dotnet-sdk-osx-$ARCH.tar.gz" ;;
        windows) nome_pacote_dotnet="dotnet-sdk-win-$ARCH.zip" ;;
    esac

    etapa 'Obtendo a versao mais recente do .NET 10 (metadados oficiais da Microsoft)'
    arq_metadados="$Instaladores/dotnet-10.0-releases.json"
    rm -f "$arq_metadados"
    for url in \
        'https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json' \
        'https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/10.0/releases.json'; do
        if curl -fsSL "$url" -o "$arq_metadados" 2>/dev/null && [[ -s "$arq_metadados" ]]; then
            break
        fi
        aviso "falha em $url"
        rm -f "$arq_metadados"
    done
    [[ -s "$arq_metadados" ]] || erro_sai 'Nao foi possivel obter os metadados de versoes do .NET.'

    leitura="$(NOME_PACOTE_DOTNET="$nome_pacote_dotnet" node -e '
const dados = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
const versao = dados["latest-release"];
const alvo = dados.releases.find(r => r["release-version"] === versao);
const arq = alvo && alvo.sdk.files.find(f => f.name === process.env.NOME_PACOTE_DOTNET);
if (!arq) process.exit(1);
console.log([versao, alvo.sdk.version, arq.url, arq.hash].join("\n"));
' "$(caminho_nativo "$arq_metadados")")" || erro_sai "Pacote $nome_pacote_dotnet nao encontrado nos metadados do .NET."
    versao_release="$(sed -n '1p' <<<"$leitura" | tr -d '\r')"
    sdk_versao="$(sed -n '2p' <<<"$leitura" | tr -d '\r')"
    sdk_url="$(sed -n '3p' <<<"$leitura" | tr -d '\r')"
    sdk_hash="$(sed -n '4p' <<<"$leitura" | tr -d '\r')"
    [[ -n "$sdk_url" ]] || erro_sai "Pacote $nome_pacote_dotnet nao encontrado nos metadados do .NET $versao_release."
    ok ".NET SDK $sdk_versao"

    host_sdk="${sdk_url#*://}"
    host_sdk="${host_sdk%%/*}"
    case "$host_sdk" in
        *.microsoft.com|*.windows.net|*.azureedge.net) ;;
        *) erro_sai "Host inesperado para download do .NET: $host_sdk" ;;
    esac

    if [[ -d "$destino_dotnet/sdk/$sdk_versao" ]]; then
        ok "SDK $sdk_versao ja presente em $destino_dotnet"
    else
        extensao="tar.gz"; [[ "$SO" == windows ]] && extensao="zip"
        arq_sdk="$Instaladores/dotnet-sdk-$sdk_versao-$SO-$ARCH.$extensao"
        [[ -f "$arq_sdk" ]] || baixar_arquivo "$sdk_url" "$arq_sdk"
        verificar_hash "$arq_sdk" "$sdk_hash" sha512

        etapa 'Extraindo o SDK do .NET'
        rm -rf "$destino_dotnet"
        garantir_pasta "$destino_dotnet"
        if [[ "$SO" == windows ]]; then
            extrair_zip "$arq_sdk" "$destino_dotnet"
        else
            tar -xzf "$arq_sdk" -C "$destino_dotnet"
        fi
        ok "SDK $sdk_versao extraido em $destino_dotnet"
    fi
fi

# ------------------------------------------------------------------ sqlite3
etapa 'Verificando o sqlite3'
destino_sqlite="$Ferramentas/sqlite"
if command -v sqlite3 >/dev/null 2>&1 && sqlite3 --version >/dev/null 2>&1; then
    ok "sqlite3 disponivel ($(sqlite3 --version | awk '{print $1}'), $(command -v sqlite3))"
else
    case "$SO-$ARCH" in
        windows-x64)   sufixo_sqlite="win-x64" ;;
        windows-arm64) sufixo_sqlite="win-arm64" ;;
        linux-x64)     sufixo_sqlite="linux-x64" ;;
        osx-x64)       sufixo_sqlite="osx-x64" ;;
        osx-arm64)     sufixo_sqlite="osx-arm64" ;;
        *)
            aviso "O sqlite.org nao publica o sqlite3 pronto para $SO/$ARCH."
            echo '    Instale com o gerenciador de pacotes da sua distro, por exemplo:'
            echo '      sudo apt install sqlite3        (Debian/Ubuntu)'
            echo '      sudo dnf install sqlite         (Fedora)'
            echo '      sudo pacman -S sqlite           (Arch)'
            erro_sai 'Instale o sqlite3 e rode este script novamente.'
            ;;
    esac

    etapa 'Obtendo a versao mais recente do sqlite3 (pagina oficial de download)'
    # A pagina de download do sqlite.org traz um bloco "para scripts lerem":
    #   PRODUCT,VERSAO,ANO/arquivo.zip,TAMANHO,HASH-SHA3-256
    pagina_sqlite="$(curl -fsSL 'https://www.sqlite.org/download.html')" || erro_sai 'Nao foi possivel acessar https://www.sqlite.org/download.html (sem internet?).'
    linha_sqlite="$( (grep -E "^PRODUCT,[^,]+,[0-9]{4}/sqlite-tools-$sufixo_sqlite-[0-9]+\.zip," <<<"$pagina_sqlite" || true) | head -n1 | tr -d '\r')"
    [[ -n "$linha_sqlite" ]] || erro_sai "Pacote sqlite-tools-$sufixo_sqlite nao encontrado em sqlite.org/download.html."
    IFS=',' read -r _ sqlite_versao sqlite_relativo _ sqlite_hash <<<"$linha_sqlite"
    ok "sqlite3 $sqlite_versao"

    arq_sqlite="$Instaladores/$(basename "$sqlite_relativo")"
    [[ -f "$arq_sqlite" ]] || baixar_arquivo "https://www.sqlite.org/$sqlite_relativo" "$arq_sqlite"
    verificar_hash "$arq_sqlite" "$sqlite_hash" sha3-256

    etapa 'Extraindo o sqlite3'
    pasta_tmp="$Instaladores/sqlite-extracao"
    rm -rf "$pasta_tmp"
    extrair_zip "$arq_sqlite" "$pasta_tmp"
    executavel="$(find "$pasta_tmp" -type f \( -name sqlite3 -o -name sqlite3.exe \) | head -n1)"
    [[ -n "$executavel" ]] || erro_sai "sqlite3 nao encontrado dentro de $(basename "$arq_sqlite")."
    rm -rf "$destino_sqlite"
    mv "$(dirname "$executavel")" "$destino_sqlite"
    rm -rf "$pasta_tmp"
    chmod +x "$destino_sqlite"/* 2>/dev/null || true
    export PATH="$destino_sqlite:$PATH"
    sqlite3 --version >/dev/null 2>&1 || erro_sai "O sqlite3 em $destino_sqlite nao executa."
    ok "sqlite3 $sqlite_versao em $destino_sqlite"
fi

# ------------------------------------------------------------------ Resumo
etapa 'Conferindo o ambiente'
configurar_ambiente_build
ok "dotnet  $(dotnet --version)"
ok "node    $(node --version)"
ok "npm     $(npm --version)"
ok "sqlite3 $(sqlite3 --version | awk '{print $1}')"
if [[ "$SO" == windows ]] && ! net session >/dev/null 2>&1; then
    aviso 'Este terminal nao esta como Administrador. O 2-criar-banco.sh funciona assim, mas'
    aviso 'rode o 3-publicar-atualizar.sh num Git Bash aberto como Administrador (registro do servico).'
fi

echo ''
echo -e "\033[32mAmbiente pronto. Proximo passo:\033[0m"
echo "   $script_dir/2-criar-banco.sh"
