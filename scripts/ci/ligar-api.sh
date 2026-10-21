#!/usr/bin/env bash
# =====================================================================
# #309: liga a API compilada e confere que ela responde (teste rápido de
# inicialização, ~20 s). Pega o que só aparece ao iniciar: injeção de
# dependência faltando, configuração errada, rota repetida, erro no esquema
# aplicado pelo EsquemaBanco.
#
# - Banco e chaves numa pasta temporária (o EsquemaBanco cria o banco).
# - HTTPS em 127.0.0.1:5080, com um certificado descartável do
#   dotnet dev-certs (o cookie anti-CSRF exige HTTPS), passado ao endpoint
#   "Https" do appsettings.json por variável de ambiente.
# - Espera GET /api/auth/estado responder 200 com "configurado"; desliga.
#
# O que é só do Windows (serviço, DPAPI, certificado do instalador) continua
# com o teste do .msi (ADR 0047).
#
# Uso (depois do build):  scripts/ci/ligar-api.sh [Release|Debug]
# =====================================================================
set -euo pipefail

configuracao="${1:-Release}"
raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
dll="$raiz/src/backend/Financas.Api/bin/$configuracao/net10.0/Financas.Api.dll"
[[ -f "$dll" ]] || { echo "API não compilada em $dll (rode o dotnet build antes)."; exit 1; }

pasta="$(mktemp -d)"
log="$pasta/api.log"
porta=5080
url="https://127.0.0.1:$porta"

export ASPNETCORE_ENVIRONMENT=Production
export Kestrel__Endpoints__Https__Url="$url"
# Certificado descartável, só para este teste (a senha é aleatória e some com a pasta).
senha="$(head -c 18 /dev/urandom | base64 | tr -d '/+=')"
dotnet dev-certs https -ep "$pasta/teste.pfx" -p "$senha" >/dev/null
export Kestrel__Endpoints__Https__Certificate__Path="$pasta/teste.pfx"
export Kestrel__Endpoints__Https__Certificate__Password="$senha"
export Banco__Caminho="$pasta/sql/FinancasLocal.db"
export Seguranca__PastaChaves="$pasta/chaves"
mkdir -p "$pasta/sql" "$pasta/chaves"

dotnet "$dll" >"$log" 2>&1 &
pid=$!
parar() { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
trap parar EXIT

resposta=""
for _ in $(seq 1 60); do
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "A API parou ao iniciar. Log:"; cat "$log"; exit 1
    fi
    resposta="$(curl -fsSk "$url/api/auth/estado" 2>/dev/null || true)"
    [[ -n "$resposta" ]] && break
    sleep 1
done

if [[ "$resposta" != *'"configurado"'* ]]; then
    echo "A API não respondeu /api/auth/estado em 60 s (resposta: '${resposta:-nada}'). Log:"; cat "$log"; exit 1
fi
echo "OK: a API subiu e /api/auth/estado respondeu: $resposta"
