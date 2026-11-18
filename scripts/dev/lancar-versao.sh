#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - prepara a publicacao de uma versao na release/X.Y.Z
# (ADR 0070)
#
# Junta os arquivos de changelog.d/ numa secao "## [X.Y.Z] - data" do
# CHANGELOG.md, por tipo (Funcionalidades, Correcoes, Publicacao e
# documentacao), apaga os arquivos e faz o commit "chore: versao X.Y.Z".
# O trabalho e do src/backend/cmd/versao (-publicar), que tem os testes.
#
# Confere antes de mexer: a arvore esta limpa, a branch e release/X.Y.Z
# e o numero calculado neste commit e o X.Y.Z (a tag vX.Y.Z tem de ser
# igual; o workflow da versao confere de novo).
#
# Precisa de git e do Go (o da versao do src/backend/go.mod).
#
# Uso (na release/X.Y.Z, criada do develop):
#   scripts/dev/lancar-versao.sh X.Y.Z
#   scripts/dev/lancar-versao.sh X.Y.Z --data AAAA-MM-DD   # outra data que nao a de hoje
# Depois: o PR da release/X.Y.Z para a main e a tag (CONTRIBUINDO.md, "Publicar").
# =====================================================================
set -euo pipefail

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
versao="${1:-}"
if ! [[ "$versao" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 2
fi
data="$(date +%F)"
if [[ "${2:-}" == "--data" ]]; then
    data="${3:-}"
    [[ "$data" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { echo "--data pede AAAA-MM-DD." >&2; exit 2; }
fi

cd "$raiz"
command -v go >/dev/null 2>&1 || { echo "Falta o Go no PATH (o calculo da versao e em Go)." >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo "Ha mudancas nao commitadas: commite ou guarde antes." >&2; exit 1; }
branch="$(git rev-parse --abbrev-ref HEAD)"
[[ "$branch" == "release/$versao" ]] || { echo "Voce esta em '$branch': rode na release/$versao, criada do develop." >&2; exit 1; }

calculada="$(cd src/backend && go run ./cmd/versao)"
if [[ "$calculada" != "$versao" ]]; then
    echo "O numero calculado neste commit e $calculada, nao $versao." >&2
    echo "A release sai do develop (a tag e o numero do develop naquele commit): confira o X.Y.Z e a branch." >&2
    exit 1
fi

(cd src/backend && go run ./cmd/versao -publicar "$versao" -data "$data")
git commit -q -m "chore: versão $versao"
echo "Feito: CHANGELOG.md com a secao $versao ($data) e changelog.d/ esvaziada, no commit 'chore: versao $versao'."
echo "Proximo passo: PR da release/$versao para a main e a tag v$versao (CONTRIBUINDO.md, \"Publicar\")."
