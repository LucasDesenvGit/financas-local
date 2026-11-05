#!/usr/bin/env bash
# =====================================================================
# #309: liga o site compilado e confere que ele responde (teste rápido de
# inicialização). Pega o que só aparece ao iniciar: configuração errada,
# rota repetida, erro no esquema aplicado ou nas rotinas de partida.
#
# #86: o site é o executável do Go, montado como a publicação o monta:
# - numa pasta temporária, o appsettings.json do src/backend e um
#   appsettings.Production.json com o banco, as chaves e o certificado ali
#   dentro (o banco é criado pelo -aplicar-esquema);
# - o certificado feito pelo próprio executável (gerar-certificado), como
#   nos scripts de instalação, em 127.0.0.1:5080;
# - a mesma linha do serviço: -aplicar-esquema -rotinas-de-partida.
# Espera GET /api/auth/estado responder 200 com "configurado"; desliga.
#
# O que é só do Windows (serviço, certificado do instalador) continua com o
# teste do .msi (ADR 0047).
#
# Uso:  scripts/ci/ligar-api.sh
# =====================================================================
set -euo pipefail

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v go >/dev/null || { echo "Go não encontrado no PATH."; exit 1; }

pasta="$(mktemp -d)"
log="$pasta/site.log"
porta=5080
url="https://127.0.0.1:$porta"
trap 'rm -rf "$pasta"' EXIT

(cd "$raiz/src/backend" && CGO_ENABLED=0 go build -o "$pasta/financas-go" ./cmd/financas)
cp "$raiz/src/backend/appsettings.json" "$pasta/appsettings.json"
mkdir -p "$pasta/dados/sql" "$pasta/dados/chaves" "$pasta/wwwroot"
echo '<!doctype html><title>teste</title>' >"$pasta/wwwroot/index.html"

# Certificado descartável, só para este teste: a senha é aleatória, vai pela entrada padrão e some com a pasta.
senha="$(head -c 18 /dev/urandom | base64 | tr -d '/+=')"
printf '%s' "$senha" | "$pasta/financas-go" gerar-certificado -pfx "$pasta/dados/chaves/financaslocal.pfx" >/dev/null
cat >"$pasta/appsettings.Production.json" <<JSON
{
  "Banco": { "Caminho": "$pasta/dados/sql/FinancasLocal.db" },
  "Seguranca": { "PastaChaves": "$pasta/dados/chaves" },
  "Kestrel": { "Endpoints": { "Https": { "Url": "$url",
    "Certificate": { "Path": "$pasta/dados/chaves/financaslocal.pfx", "Password": "$senha" } } } }
}
JSON

"$pasta/financas-go" -pasta "$pasta" -aplicar-esquema -rotinas-de-partida >"$log" 2>&1 &
pid=$!
parar() { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; rm -rf "$pasta"; }
trap parar EXIT

resposta=""
for _ in $(seq 1 60); do
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "O site parou ao iniciar. Log:"; cat "$log"; exit 1
    fi
    resposta="$(curl -fsSk "$url/api/auth/estado" 2>/dev/null || true)"
    [[ -n "$resposta" ]] && break
    sleep 1
done

if [[ "$resposta" != *'"configurado"'* ]]; then
    echo "O site não respondeu /api/auth/estado em 60 s (resposta: '${resposta:-nada}'). Log:"; cat "$log"; exit 1
fi
[[ -f "$pasta/dados/sql/FinancasLocal.db" ]] || { echo "O -aplicar-esquema não criou o banco."; exit 1; }
echo "OK: o site subiu e /api/auth/estado respondeu: $resposta"
