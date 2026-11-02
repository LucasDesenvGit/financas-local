#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - traz o develop para um PR e resolve a versao e o
# CHANGELOG sozinho (#587, ADR 0039)
#
# Todo PR sobe a versao no Directory.Build.props e escreve no topo do
# CHANGELOG.md; quando outro PR entra antes, os dois arquivos conflitam.
# Este script, numa pasta temporaria (nao mexe na sua):
#   1. traz o origin/develop para a branch do PR;
#   2. se o conflito e so nesses dois arquivos, sobe a versao do develop
#      no mesmo nivel que o PR tinha subido (nunca menos do que o tipo do
#      titulo pede) e poe a secao do PR no topo, com o numero novo e a
#      data de hoje (scripts/geral/atualizar-versao-do-pr.py);
#   3. confere com o verificar-versao.py e o verificar-changelog.py e
#      envia com a sua conta (a CI roda normal).
# Conflito em outro arquivo: desfaz e diz quais resolver a mao.
#
# Precisa de git, gh (logado) e python3 (ou python, no Windows).
#
# Uso:
#   scripts/geral/atualizar-pr.sh <numero do PR>
#   scripts/geral/atualizar-pr.sh <numero do PR> --sem-enviar   # so prepara e mostra
# =====================================================================
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
raiz="$(cd "$script_dir/../.." && pwd)"

numero="${1:-}"
enviar=1
[ "${2:-}" = "--sem-enviar" ] && enviar=0
if ! [[ "$numero" =~ ^[0-9]+$ ]]; then
    sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 2
fi

python=python3
command -v python3 >/dev/null 2>&1 && python3 -c '' >/dev/null 2>&1 || python=python
command -v gh >/dev/null 2>&1 || { echo "Falta o gh (GitHub CLI), logado." >&2; exit 1; }

cd "$raiz"
IFS=$'\t' read -r branch base titulo de_fora autor < <(gh pr view "$numero" \
    --json headRefName,baseRefName,title,isCrossRepository,author \
    -q '[.headRefName, .baseRefName, .title, (.isCrossRepository|tostring), .author.login] | @tsv')
if [ "$base" != develop ]; then
    echo "O PR #$numero vai para o $base, nao para o develop: nada a fazer." >&2; exit 1
fi
if [ "$de_fora" = true ]; then
    echo "O PR #$numero vem de um fork: atualize pela branch de la." >&2; exit 1
fi
if [ "$autor" = "app/dependabot" ] || [ "$autor" = "dependabot" ]; then
    echo "PR do Dependabot nao sobe a versao: comente \"@dependabot rebase\" no PR." >&2; exit 1
fi

echo "PR #$numero: $titulo"
echo "Branch $branch; trazendo o origin/develop..."
git fetch -q origin develop "$branch"

pasta="$(mktemp -d)"
limpar() { git -C "$raiz" worktree remove --force "$pasta/pr" >/dev/null 2>&1 || true; rm -rf "$pasta"; }
trap limpar EXIT
git worktree add -q --detach "$pasta/pr" "origin/$branch"
cd "$pasta/pr"

if git merge -q -m "Merge remote-tracking branch 'origin/develop' into $branch" origin/develop >/dev/null 2>&1; then
    echo "Sem conflito."
else
    conflitos="$(git diff --name-only --diff-filter=U)"
    outros="$(printf '%s\n' "$conflitos" | grep -vxE 'CHANGELOG\.md|Directory\.Build\.props' || true)"
    if [ -n "$outros" ]; then
        git merge --abort
        echo "Conflito fora da versao e do CHANGELOG; resolva a mao (git merge origin/develop na branch $branch):" >&2
        printf '  %s\n' $outros >&2
        exit 1
    fi
    "$python" -B "$script_dir/atualizar-versao-do-pr.py" "$titulo"
    git add CHANGELOG.md Directory.Build.props
    git commit -q --no-edit
fi

if git merge-base --is-ancestor HEAD "origin/$branch"; then
    echo "O PR ja estava em dia com o develop: nada a enviar."
    exit 0
fi
BASE=develop TITULO_PR="$titulo" "$python" -B scripts/ci/verificar-versao.py
"$python" -B scripts/ci/verificar-changelog.py
git --no-pager log --oneline -1

if [ "$enviar" = 0 ]; then
    echo "--sem-enviar: nada foi enviado. O que o PR muda em relacao ao develop, depois de atualizado:"
    git --no-pager diff --stat origin/develop HEAD
    exit 0
fi
git push -q origin "HEAD:$branch"
echo "Enviado para $branch. A CI do PR #$numero roda de novo."
