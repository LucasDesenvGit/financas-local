#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - 1) Preparacao do ambiente
#
#  - .NET 10 SDK portatil em ferramentas/dotnet (reaproveita um dotnet 10.x
#    que ja esteja no PATH, se houver)
#  - Node.js LTS portatil em ferramentas/node (so para compilar o Angular)
#  - sqlite3: apenas verifica se ja esta instalado; senao, mostra como instalar
#
# Sem IIS e sem SQL Server Express: o Kestrel hospeda a API diretamente e o
# banco e SQLite. Funciona em Linux, macOS e Windows (Git Bash/MSYS ou WSL).
#
# Uso:
#   scripts/1-preparar-ambiente.sh
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "FinancasLocal - preparacao do ambiente em $Raiz"

garantir_pasta "$Ferramentas"
garantir_pasta "$Instaladores"

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
    metadados_dotnet=""
    for url in \
        'https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json' \
        'https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/10.0/releases.json'; do
        if metadados_dotnet="$(curl -fsSL "$url" 2>/dev/null)" && [[ -n "$metadados_dotnet" ]]; then
            break
        fi
        aviso "falha em $url"
        metadados_dotnet=""
    done
    [[ -n "$metadados_dotnet" ]] || erro_sai 'Nao foi possivel obter os metadados de versoes do .NET.'

    versao_release=""
    if command -v jq >/dev/null 2>&1; then
        versao_release="$(jq -r '."latest-release"' <<<"$metadados_dotnet")"
        sdk_versao="$(jq -r --arg v "$versao_release" '.releases[] | select(."release-version"==$v) | .sdk.version' <<<"$metadados_dotnet")"
        sdk_url="$(jq -r --arg v "$versao_release" --arg f "$nome_pacote_dotnet" '.releases[] | select(."release-version"==$v) | .sdk.files[] | select(.name==$f) | .url' <<<"$metadados_dotnet")"
        sdk_hash="$(jq -r --arg v "$versao_release" --arg f "$nome_pacote_dotnet" '.releases[] | select(."release-version"==$v) | .sdk.files[] | select(.name==$f) | .hash' <<<"$metadados_dotnet")"
    else
        requer_comando python3 'python3 nao encontrado (necessario para interpretar os metadados do .NET sem o jq).'
        leitura="$(NOME_PACOTE_DOTNET="$nome_pacote_dotnet" python3 -I -c '
import json, os, sys
dados = json.load(sys.stdin)
versao = dados["latest-release"]
alvo = next(r for r in dados["releases"] if r["release-version"] == versao)
sdk = alvo["sdk"]
arq = next((f for f in sdk["files"] if f["name"] == os.environ["NOME_PACOTE_DOTNET"]), None)
if not arq:
    sys.exit(1)
print(versao)
print(sdk["version"])
print(arq["url"])
print(arq["hash"])
' <<<"$metadados_dotnet")" || erro_sai "Pacote $nome_pacote_dotnet nao encontrado nos metadados do .NET."
        versao_release="$(sed -n '1p' <<<"$leitura")"
        sdk_versao="$(sed -n '2p' <<<"$leitura")"
        sdk_url="$(sed -n '3p' <<<"$leitura")"
        sdk_hash="$(sed -n '4p' <<<"$leitura")"
    fi
    [[ -n "$sdk_url" && "$sdk_url" != "null" ]] || erro_sai "Pacote $nome_pacote_dotnet nao encontrado nos metadados do .NET $versao_release."
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
            requer_comando unzip 'unzip nao encontrado. Instale-o ou extraia manualmente o pacote baixado em instaladores/.'
            unzip -q "$arq_sdk" -d "$destino_dotnet"
        else
            tar -xzf "$arq_sdk" -C "$destino_dotnet"
        fi
        ok "SDK $sdk_versao extraido em $destino_dotnet"
    fi
fi

# ------------------------------------------------------------------ Node.js (somente para compilar o front-end)
etapa 'Verificando o Node.js'
destino_node="$Ferramentas/node"

case "$SO-$ARCH" in
    linux-x64)    chave_arquivos_node="linux-x64";     sufixo_node="linux-x64" ;;
    linux-arm64)  chave_arquivos_node="linux-arm64";   sufixo_node="linux-arm64" ;;
    osx-x64)      chave_arquivos_node="osx-x64-tar";   sufixo_node="darwin-x64" ;;
    osx-arm64)    chave_arquivos_node="osx-arm64-tar"; sufixo_node="darwin-arm64" ;;
    windows-x64)  chave_arquivos_node="win-x64-zip";   sufixo_node="win-x64" ;;
    windows-arm64) chave_arquivos_node="win-arm64-zip"; sufixo_node="win-arm64" ;;
    *) erro_sai "Combinacao de SO/arquitetura sem pacote Node.js conhecido: $SO/$ARCH" ;;
esac

indice_node="$(curl -fsSL 'https://nodejs.org/dist/index.json')" || erro_sai 'Nao foi possivel obter o indice de versoes do Node.js.'

if command -v jq >/dev/null 2>&1; then
    node_versao="$(jq -r --arg chave "$chave_arquivos_node" '
        [ .[] | select(.lts != false and (.files | index($chave)) and (((.version | ltrimstr("v") | split(".")[0] | tonumber) as $maj | [20,22,24] | index($maj)) != null)) ]
        | .[0].version // empty' <<<"$indice_node")"
else
    requer_comando python3 'python3 nao encontrado (necessario para interpretar o indice do Node.js sem o jq).'
    node_versao="$(CHAVE_NODE="$chave_arquivos_node" python3 -I -c '
import json, os, sys
dados = json.load(sys.stdin)
chave = os.environ["CHAVE_NODE"]
for item in dados:
    if not item.get("lts"):
        continue
    if chave not in item.get("files", []):
        continue
    major = int(item["version"].lstrip("v").split(".")[0])
    if major in (20, 22, 24):
        print(item["version"])
        break
' <<<"$indice_node")"
fi
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
        requer_comando unzip 'unzip nao encontrado. Instale-o ou extraia manualmente o pacote baixado em instaladores/.'
        unzip -q "$arq_node" -d "$pasta_tmp"
    else
        tar -xzf "$arq_node" -C "$pasta_tmp"
    fi
    rm -rf "$destino_node"
    mv "$pasta_tmp/node-$node_versao-$sufixo_node" "$destino_node"
    rm -rf "$pasta_tmp"
    ok "Node.js $node_versao em $destino_node"
fi

# ------------------------------------------------------------------ sqlite3
etapa 'Verificando o sqlite3'
if command -v sqlite3 >/dev/null 2>&1; then
    ok "sqlite3 disponivel ($(sqlite3 --version | awk '{print $1}'))"
else
    aviso 'sqlite3 nao encontrado no PATH.'
    case "$SO" in
        linux)
            echo '    Instale com o gerenciador de pacotes da sua distro, por exemplo:'
            echo '      sudo apt install sqlite3        (Debian/Ubuntu)'
            echo '      sudo dnf install sqlite         (Fedora)'
            echo '      sudo pacman -S sqlite           (Arch)'
            ;;
        osx)
            echo '    Instale com: brew install sqlite3'
            ;;
        windows)
            echo '    Instale com: winget install SQLite.SQLite'
            echo '    ou baixe os binarios precompilados (pacote "sqlite-tools-win*") em'
            echo '    https://www.sqlite.org/download.html e adicione a pasta ao PATH.'
            ;;
    esac
    erro_sai 'Instale o sqlite3 e rode este script novamente.'
fi

echo ''
echo -e "\033[32mAmbiente pronto. Proximo passo:\033[0m"
echo "   $script_dir/2-criar-banco.sh"
