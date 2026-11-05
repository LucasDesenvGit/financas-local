#!/usr/bin/env bash
# =====================================================================
# #72 (ADR 0049 e 0051): roda o contrato contra o back-end em Go, do zero.
#
# Os esperados (esperado/) foram gravados do .NET até a #86; desde então,
# são a regressão do Go: um esperado só muda com o motivo escrito no PR.
#
# 1. Monta o banco fictício numa pasta temporária: banco/01-estrutura.sql,
#    as colunas novas do 2-criar-banco.sh, banco/02-dados-iniciais.sql e
#    dados/base.sql (dados inventados).
# 2. Sobe o Go como o serviço instalado (-aplicar-esquema -rotinas-de-partida),
#    com:
#    - o relógio de teste começando em 2026-10-15 01:00:00 UTC
#      (-inicio-do-relogio; o relógio anda a partir dali);
#    - um certificado só em memória (o cookie anti-CSRF exige HTTPS);
#    - a Pluggy falsa (testes/contrato/pluggyfalsa) no lugar da Pluggy, pela
#      variável Pluggy__UrlBase, com a raiz dela em SSL_CERT_FILE;
#    - sem internet: no Linux, num namespace de rede só com o loopback
#      (unshare -rn), se o sistema deixar; senão, com a saída (PTAX, Mercado
#      Bitcoin) mandada para um proxy numa porta fechada (#416).
# 3. Roda o comparador (go test ./testes/contrato) contra ele e o desliga.
#
# Uso:
#   src/backend/testes/contrato/rodar.sh               # compara com esperado/
#   src/backend/testes/contrato/rodar.sh --atualizar   # grava esperado/: confira a diferença e diga o motivo no PR
#   CONTRATO_SOMENTE="00-auth,01-parametros" src/backend/testes/contrato/rodar.sh
#
# Roda também no Git Bash do Windows (conferido em 03/11/2026, com todos os
# arquivos), sem o namespace de rede.
#
# Variáveis opcionais: PORTA (5451), CONTRATO_SOMENTE (todos os arquivos, na
# ordem), MANTER_PASTA=1 (não apaga a pasta temporária, para olhar o banco e o
# log do Go).
# =====================================================================
set -euo pipefail

atualizar=""
for arg in "$@"; do
    case "$arg" in
        --atualizar) atualizar="-atualizar" ;;
        -h|--help) sed -n '2,33p' "$0"; exit 0 ;;
        *) echo "Argumento desconhecido: $arg" >&2; exit 2 ;;
    esac
done

pasta_contrato="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
pasta_go="$(cd "$pasta_contrato/../.." && pwd)"
raiz="$(cd "$pasta_go/../.." && pwd)"
porta="${PORTA:-5451}"
# O mesmo instante do InicioPadrao (contrato_test.go).
inicio="2026-10-15T01:00:00Z"

command -v sqlite3 >/dev/null || { echo "sqlite3 não encontrado." >&2; exit 1; }
command -v go >/dev/null || { echo "go não encontrado." >&2; exit 1; }

pasta="$(mktemp -d)"
limpar() {
    if [[ "${MANTER_PASTA:-}" == 1 ]]; then echo "Pasta mantida: $pasta"; else rm -rf "$pasta"; fi
}
trap limpar EXIT
mkdir -p "$pasta/dados/sql" "$pasta/dados/chaves"
banco_db="$pasta/dados/sql/FinancasLocal.db"

# 1. Banco fictício, na ordem do 2-criar-banco.sh: estrutura, colunas novas (as linhas adicionar_coluna_se_ausente),
#    dados iniciais e, por fim, os dados inventados.
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

# O comparador, o Go e a Pluggy falsa compilados fora do namespace sem rede (o go pode precisar baixar módulos). O Go
# sai como na publicação: sem cgo e com a versão do Directory.Build.props (o /api/go/saude a devolve).
exe="$(cd "$pasta_go" && go env GOEXE)" # ".exe" no Windows
versao="$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' "$raiz/Directory.Build.props" | head -n1)"
(cd "$pasta_go" &&
    go test -c -o "$pasta/contrato.test$exe" ./testes/contrato &&
    CGO_ENABLED=0 go build -ldflags "-X github.com/LucasDesenvGit/financas-local/src/backend/internal/versao.Atual=$versao" \
        -o "$pasta/financas-go$exe" ./cmd/financas &&
    go build -o "$pasta/pluggy-falsa$exe" ./testes/contrato/pluggyfalsa)

# O certificado do Go é o da instalação (gerar-certificado, #84), lido do appsettings como no site, e o comparador
# confia só na parte pública dele (-raiz): a conexão é conferida, sem InsecureSkipVerify (#638).
senha_certificado=contrato
printf '%s\n' "$senha_certificado" | "$pasta/financas-go$exe" gerar-certificado -pfx "$pasta/dados/chaves/financaslocal.pfx" >/dev/null
printf '{"Kestrel":{"Endpoints":{"Https":{"Certificate":{"Path":"dados/chaves/financaslocal.pfx","Password":"%s"}}}}}\n' \
    "$senha_certificado" >"$pasta/appsettings.Production.json"

# Todos os arquivos de casos, na ordem: os de um arquivo deixam rastro no banco que os seguintes veem.
somente="${CONTRATO_SOMENTE:-}"
if [[ -z "$somente" ]]; then
    for caso in "$pasta_contrato"/casos/*.json; do
        nome="$(basename "$caso" .json)"
        somente="${somente:+$somente,}$nome"
    done
fi

# 2 e 3. Roda dentro do namespace sem rede (Linux) ou com o proxy fechado (nos outros).
cat >"$pasta/dentro.sh" <<'DENTRO'
set -euo pipefail
if [[ -n "${SEM_REDE:-}" ]]; then ip link set lo up; fi
url="https://127.0.0.1:$PORTA"
"$PASTA/pluggy-falsa$EXE" -endereco 127.0.0.1:5452 -dados "$PASTA_CONTRATO/dados/pluggy" -certificado "$PASTA/pluggy-raiz.pem" >"$PASTA/pluggy.log" 2>&1 &
pid_pluggy=$!
for _ in $(seq 1 40); do [[ -s "$PASTA/pluggy-raiz.pem" ]] && break; sleep 0.25; done
[[ -s "$PASTA/pluggy-raiz.pem" ]] || { echo "A Pluggy falsa não subiu. Log:"; cat "$PASTA/pluggy.log"; exit 1; }

Pluggy__UrlBase="https://127.0.0.1:5452/" SSL_CERT_FILE="$PASTA/pluggy-raiz.pem" \
HTTPS_PROXY="$PROXY" HTTP_PROXY="$PROXY" NO_PROXY= \
    "$PASTA/financas-go$EXE" -pasta "$PASTA" -endereco "127.0.0.1:$PORTA" \
        -wwwroot "$PASTA_GO/internal/servidor/testdata/wwwroot" -banco "$PASTA/dados/sql/FinancasLocal.db" \
        -chaves "$PASTA/dados/chaves" -inicio-do-relogio "$INICIO" -aplicar-esquema -rotinas-de-partida \
        >"$PASTA/go.log" 2>&1 &
pid=$!
parar() {
    kill "$pid" "$pid_pluggy" 2>/dev/null || true
    wait "$pid" "$pid_pluggy" 2>/dev/null || true
    if [[ -s "$PASTA/pluggy.log" ]]; then echo "--- Pedidos sem resposta na Pluggy falsa ---"; cat "$PASTA/pluggy.log"; fi
}
trap parar EXIT
for _ in $(seq 1 60); do
    grep -q 'escutando' "$PASTA/go.log" && break
    kill -0 "$pid" 2>/dev/null || { echo "O Go parou ao iniciar. Log:"; cat "$PASTA/go.log"; exit 1; }
    sleep 0.5
done
grep -q 'escutando' "$PASTA/go.log" || { echo "O Go não subiu em 30 s. Log:"; cat "$PASTA/go.log"; exit 1; }
echo "Contrato contra o Go: $SOMENTE"

cd "$PASTA_CONTRATO"
status=0
"$PASTA/contrato.test$EXE" -test.run "^TestContrato$" -test.v -test.count=1 -alvo="$url" \
    -raiz="$PASTA/dados/chaves/financaslocal.pem" -banco="$PASTA/dados/sql/FinancasLocal.db" -inicio="$INICIO" -somente="$SOMENTE" $ATUALIZAR || status=$?
if [[ $status -ne 0 ]]; then echo "--- Fim do log do Go ---"; tail -n 60 "$PASTA/go.log"; fi
exit $status
DENTRO

export PASTA="$pasta" PASTA_CONTRATO="$pasta_contrato" PASTA_GO="$pasta_go" PORTA="$porta" INICIO="$inicio" \
    ATUALIZAR="$atualizar" SOMENTE="$somente" EXE="$exe"
if [[ "$(uname -s)" == Linux ]] && unshare -rn true 2>/dev/null; then
    SEM_REDE=1 PROXY="" unshare -rn bash "$pasta/dentro.sh"
else
    # Sem o namespace (Windows, macOS, ou um Linux que não deixa; no Ubuntu 24.04:
    # sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0): o http.Client do Go segue o HTTPS_PROXY, e
    # 127.0.0.1 (o teste e a Pluggy falsa) nunca passa por ele.
    SEM_REDE="" PROXY="http://127.0.0.1:9" bash "$pasta/dentro.sh"
fi
