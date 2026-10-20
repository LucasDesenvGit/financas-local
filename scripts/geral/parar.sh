#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - Para tudo o que o projeto deixou rodando
#
#  1. Servico do sistema (systemd --user no Linux, sc.exe no Windows), que
#     hospeda a API e o front publicado.
#  2. API rodando fora do servico (dotnet run, Visual Studio, publicado/ na mao).
#  3. Front em desenvolvimento (npm start / ng serve e seus processos filhos).
#  4. Banco: SQLite nao e servico, e so o arquivo dados/sql/FinancasLocal.db.
#     O script apenas confere se algum processo ainda esta com ele aberto.
#
# Uso (Linux/macOS):
#   scripts/geral/parar.sh                 # para agora (o servico volta no proximo login/boot)
#   scripts/geral/parar.sh --desativar     # para e tira o servico da inicializacao automatica
#   scripts/geral/parar.sh --compiladores  # tambem encerra os servidores de build do .NET
#
# Para subir de novo: scripts/geral/3-publicar-atualizar.sh, ou
#   systemctl --user enable --now financaslocal.service
#
# Sai com codigo 1 se algo ficou rodando (ou o banco continua aberto).
# No Windows nao precisa de bash: de dois cliques em parar-windows.cmd, na raiz.
# Rodado pelo Git Bash, este script repassa para scripts/windows/parar-windows.ps1.
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"

unidade_systemd='financaslocal.service'
problemas=0
problema() { problemas=$((problemas + 1)); aviso "$1"; }
desativar=0
compiladores=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --desativar)    desativar=1 ;;
        --compiladores) compiladores=1 ;;
        -h|--help)      sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
        *)              erro_sai "Opcao desconhecida: $1 (use --help)" ;;
    esac
    shift
done

# A logica do Windows existe num lugar so: o .ps1 (ADR 0012).
if [[ "$SO" == windows ]]; then
    argumentos_ps=()
    [[ $desativar -eq 1 ]] && argumentos_ps+=(-Desativar)
    [[ $compiladores -eq 1 ]] && argumentos_ps+=(-Compiladores)
    exec powershell.exe -NoProfile -ExecutionPolicy Bypass \
        -File "$(caminho_nativo "$Raiz/scripts/windows/parar-windows.ps1")" "${argumentos_ps[@]}"
fi

# ------------------------------------------------------------------ 1. Servico do sistema (Linux)
etapa 'Servico do sistema'
case "$SO" in
    linux)
        if ! command -v systemctl >/dev/null 2>&1; then
            aviso 'systemctl nao encontrado; pulando o servico.'
        elif ! systemctl --user cat "$unidade_systemd" >/dev/null 2>&1; then
            ok "servico $unidade_systemd nao instalado"
        else
            if systemctl --user is-active --quiet "$unidade_systemd"; then
                systemctl --user stop "$unidade_systemd"
                ok "servico $unidade_systemd parado"
            else
                ok "servico $unidade_systemd ja estava parado"
            fi
            if [[ $desativar -eq 1 ]]; then
                systemctl --user disable --quiet "$unidade_systemd"
                ok 'removido da inicializacao automatica'
            fi
        fi
        ;;
    osx)
        ok 'nenhum servico registrado no macOS (launchd nao e configurado pela instalacao)'
        ;;
esac

# ------------------------------------------------------------------ 2 e 3. Processos fora do servico
# So processos DESTE projeto (a linha de comando tem a pasta do projeto seguida de /), para nao derrubar
# outros apps .NET/Node, nem um "dotnet build" com caminho relativo:
#  - API: executavel/DLL Financas.Api (dotnet run, Visual Studio, publicado/ na mao);
#  - front: qualquer processo rodando de src/frontend/node_modules (ng serve, esbuild...).
etapa 'Processos da API e do front fora do servico'

# pids_do_projeto 'regex-do-programa' 'subpasta/' -> PIDs cuja linha de comando casa a regex E contem $Raiz/subpasta.
# ps -eo (e nao pgrep -a, que no macOS significa outra coisa); a regex vai por ENVIRON para o awk nao mexer nas barras.
pids_do_projeto() {
    ps -eo pid=,args= | PADRAO="$1" PASTA="$Raiz/$2" EU="$$" awk '
        { pid = $1; linha = $0; sub(/^ *[0-9]+ /, "", linha) }
        linha ~ ENVIRON["PADRAO"] && index(linha, ENVIRON["PASTA"]) && pid != ENVIRON["EU"] { print pid }' || true
}

parar_processos_unix() {
    local descricao="$1" padrao="$2" subpasta="$3" pids
    pids="$(pids_do_projeto "$padrao" "$subpasta")"
    if [[ -z "$pids" ]]; then
        ok "$descricao: nada rodando"
        return
    fi
    # shellcheck disable=SC2086
    kill $pids 2>/dev/null || true
    for _ in $(seq 1 10); do
        pids="$(pids_do_projeto "$padrao" "$subpasta")"
        [[ -z "$pids" ]] && break
        sleep 1
    done
    if [[ -n "$pids" ]]; then
        # shellcheck disable=SC2086
        kill -9 $pids 2>/dev/null || true
        ok "$descricao: encerrado a forca (nao respondeu em 10 s)"
    else
        ok "$descricao: encerrado"
    fi
}

parar_processos_unix 'API' 'Financas\.Api(\.dll)?( |$)' ''
parar_processos_unix 'Front (ng serve)' 'node_modules' 'src/frontend/node_modules/'

# ------------------------------------------------------------------ Servidores de build (opcional)
if [[ $compiladores -eq 1 ]]; then
    etapa 'Servidores de build do .NET'
    if [[ -x "$Ferramentas/dotnet/dotnet" ]]; then
        export PATH="$Ferramentas/dotnet:$PATH" DOTNET_ROOT="$Ferramentas/dotnet"
    fi
    if command -v dotnet >/dev/null 2>&1; then
        dotnet build-server shutdown >/dev/null 2>&1 || true
        ok 'servidores de build encerrados (MSBuild, VBCSCompiler, Razor)'
    else
        aviso 'dotnet nao encontrado; pulando.'
    fi
fi

# ------------------------------------------------------------------ 4. Banco (SQLite)
etapa 'Banco (SQLite)'
banco="$PastaSql/FinancasLocal.db"
if [[ ! -f "$banco" ]]; then
    ok 'banco ainda nao criado'
elif command -v lsof >/dev/null 2>&1; then
    abertos="$(lsof -t -- "$banco" "$banco-wal" "$banco-shm" 2>/dev/null | sort -u || true)"
    if [[ -z "$abertos" ]]; then
        ok 'nenhum processo com o banco aberto (SQLite nao tem servico proprio)'
    else
        problema 'Ainda ha processo com o banco aberto (ex.: DB Browser, sqlite3, IDE):'
        # shellcheck disable=SC2086
        ps -o pid=,args= -p $(echo $abertos | tr ' ' ',') | sed 's/^/        /'
    fi
else
    ok 'SQLite nao tem servico proprio: com a API parada, o arquivo fica livre'
fi

echo
if [[ $problemas -gt 0 ]]; then
    aviso "Terminou com $problemas pendencia(s): veja os avisos acima."
    exit 1
fi
ok 'Pronto.'
