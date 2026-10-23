#!/usr/bin/env bash
# =====================================================================
# #72 (ADR 0049): roda o contrato contra o back-end .NET, do zero.
#
# 1. Monta o banco fictício numa pasta temporária: banco/01-estrutura.sql,
#    banco/02-dados-iniciais.sql e dados/base.sql (dados inventados).
# 2. Sobe a API .NET já compilada sobre ele, com:
#    - o relógio começando em 2026-10-15 01:00:00 UTC (libfaketime, "começa
#      em": o relógio anda a partir dali; o monotônico não é mexido, então
#      Task.Delay, PeriodicTimer e o limite de login seguem o tempo real);
#    - sem rede (unshare -rn: só a interface de loopback). O câmbio e a cripto
#      tentam a internet 1 minuto depois de subir e falham sem gravar nada;
#    - cultura pt-BR e fuso de Brasília, como no computador de quem usa;
#    - HTTPS com um certificado descartável (o cookie anti-CSRF exige HTTPS).
# 3. Roda o comparador (go test ./testes/contrato) contra ela, no mesmo
#    ambiente sem rede, e desliga a API.
#
# Uso (depois do build da API):
#   src/backend-go/testes/contrato/rodar-dotnet.sh              # compara com esperado/
#   src/backend-go/testes/contrato/rodar-dotnet.sh --atualizar  # grava esperado/
#
# Variáveis opcionais: CONFIGURACAO (Release), DOTNET (dotnet), PORTA (5451),
# FAKETIME_LIB (caminho da libfaketime.so.1), CULTURA (pt_BR.UTF-8), MANTER_PASTA=1 (não apaga a
# pasta temporária, para olhar o banco e o log da API).
# =====================================================================
set -euo pipefail

atualizar=""
for arg in "$@"; do
    case "$arg" in
        --atualizar) atualizar="-atualizar" ;;
        -h|--help) sed -n '2,27p' "$0"; exit 0 ;;
        *) echo "Argumento desconhecido: $arg" >&2; exit 2 ;;
    esac
done

pasta_contrato="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
pasta_go="$(cd "$pasta_contrato/../.." && pwd)"
raiz="$(cd "$pasta_go/../.." && pwd)"
configuracao="${CONFIGURACAO:-Release}"
dotnet="${DOTNET:-dotnet}"
porta="${PORTA:-5451}"
dll="$raiz/src/backend/Financas.Api/bin/$configuracao/net10.0/Financas.Api.dll"
# O mesmo instante do InicioPadrao (contrato_test.go), em segundos desde 1970: assim não depende do fuso de quem roda.
inicio="2026-10-15T01:00:00Z"
inicio_epoch=1792026000

[[ -f "$dll" ]] || { echo "API não compilada em $dll (rode: $dotnet build FinancasLocal.sln -c $configuracao)." >&2; exit 1; }
command -v sqlite3 >/dev/null || { echo "sqlite3 não encontrado." >&2; exit 1; }
command -v go >/dev/null || { echo "go não encontrado." >&2; exit 1; }

faketime_lib="${FAKETIME_LIB:-}"
if [[ -z "$faketime_lib" ]]; then
    for candidato in /usr/lib/x86_64-linux-gnu/faketime/libfaketime.so.1 /usr/lib/aarch64-linux-gnu/faketime/libfaketime.so.1 \
                     /usr/lib/faketime/libfaketime.so.1 /usr/local/lib/faketime/libfaketime.so.1; do
        [[ -f "$candidato" ]] && { faketime_lib="$candidato"; break; }
    done
fi
[[ -n "$faketime_lib" && -f "$faketime_lib" ]] || {
    echo "libfaketime não encontrada: instale o pacote faketime (apt-get install faketime) ou aponte FAKETIME_LIB." >&2; exit 1; }

if ! unshare -rn true 2>/dev/null; then
    echo "unshare -rn falhou: este sistema não deixa criar um namespace de usuário sem root." >&2
    echo "No Ubuntu 24.04: sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0" >&2
    exit 1
fi

pasta="$(mktemp -d)"
limpar() {
    if [[ "${MANTER_PASTA:-}" == 1 ]]; then echo "Pasta mantida: $pasta"; else rm -rf "$pasta"; fi
}
trap limpar EXIT
mkdir -p "$pasta/dados/sql" "$pasta/dados/chaves"
banco_db="$pasta/dados/sql/FinancasLocal.db"

# 1. Banco fictício, na ordem do 2-criar-banco.sh: estrutura, colunas novas (as mesmas linhas
#    adicionar_coluna_se_ausente que o EsquemaBanco lê), dados iniciais e, por fim, os dados inventados.
(
    # shellcheck source=../../../../scripts/geral/comum.sh
    source "$raiz/scripts/geral/comum.sh"
    banco="$banco_db" # o adicionar_coluna_se_ausente usa $banco
    sqlite3 -bail "$banco" < "$raiz/banco/01-estrutura.sql"
    while IFS= read -r linha; do
        eval "$linha" >/dev/null
    done < <(grep -E '^[[:space:]]*adicionar_coluna_se_ausente[[:space:]]' "$raiz/scripts/geral/2-criar-banco.sh")
    sqlite3 -bail "$banco" < "$raiz/banco/02-dados-iniciais.sql"
    sqlite3 -bail "$banco" < "$pasta_contrato/dados/base.sql"
)

# Certificado descartável (senha aleatória; some com a pasta).
senha="$(head -c 18 /dev/urandom | base64 | tr -d '/+=')"
"$dotnet" dev-certs https -ep "$pasta/teste.pfx" -p "$senha" >/dev/null
[[ -f "$pasta/teste.pfx" ]] || { echo "dotnet dev-certs não gerou o certificado." >&2; exit 1; }

# O comparador compilado fora do namespace sem rede (o go pode precisar baixar módulos).
(cd "$pasta_go" && go test -c -o "$pasta/contrato.test" ./testes/contrato)

# 2 e 3, dentro de um namespace de rede só com o loopback.
cat >"$pasta/dentro.sh" <<'DENTRO'
set -euo pipefail
ip link set lo up
url="https://127.0.0.1:$PORTA"
# Na pasta da API, como o serviço instalado: o appsettings.json (AllowedHosts, logs) é lido da pasta atual.
cd "$(dirname "$DLL")"
LD_PRELOAD="$FAKETIME_LIB" FAKETIME="@$INICIO_EPOCH" FAKETIME_FMT=%s FAKETIME_DONT_FAKE_MONOTONIC=1 \
ASPNETCORE_ENVIRONMENT=Production \
Kestrel__Endpoints__Https__Url="$url" \
Kestrel__Endpoints__Https__Certificate__Path="$PASTA/teste.pfx" \
Kestrel__Endpoints__Https__Certificate__Password="$SENHA" \
Banco__Caminho="$PASTA/dados/sql/FinancasLocal.db" \
Seguranca__PastaChaves="$PASTA/dados/chaves" \
LANG="$CULTURA" LC_ALL="$CULTURA" TZ=America/Sao_Paulo \
DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1 \
    "$DOTNET" "$DLL" >"$PASTA/api.log" 2>&1 &
pid=$!
parar() { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
trap parar EXIT

pronta=""
for _ in $(seq 1 120); do
    if ! kill -0 "$pid" 2>/dev/null; then echo "A API parou ao iniciar. Log:"; cat "$PASTA/api.log"; exit 1; fi
    if curl -fsSk "$url/api/auth/estado" >/dev/null 2>&1; then pronta=1; break; fi
    sleep 0.5
done
[[ -n "$pronta" ]] || { echo "A API não respondeu em 60 s. Log:"; cat "$PASTA/api.log"; exit 1; }

cd "$PASTA_CONTRATO"
status=0
"$PASTA/contrato.test" -test.run "^TestContrato$" -test.v -test.count=1 -alvo="$url" -banco="$PASTA/dados/sql/FinancasLocal.db" -inicio="$INICIO" $ATUALIZAR || status=$?
if [[ $status -ne 0 ]]; then echo "--- Fim do log da API ---"; tail -n 60 "$PASTA/api.log"; fi
exit $status
DENTRO

PASTA="$pasta" PASTA_CONTRATO="$pasta_contrato" PORTA="$porta" SENHA="$senha" DOTNET="$(command -v "$dotnet")" DLL="$dll" \
FAKETIME_LIB="$faketime_lib" INICIO="$inicio" INICIO_EPOCH="$inicio_epoch" ATUALIZAR="$atualizar" CULTURA="${CULTURA:-pt_BR.UTF-8}" \
    unshare -rn bash "$pasta/dentro.sh"
