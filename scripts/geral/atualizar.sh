#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - atualizar a instalacao de DESENVOLVIMENTO pelo git (#154, ADR 0040)
#
# So para quem desenvolve: a pasta do projeto e um clone git na branch develop.
# Traz o develop do GitHub (so avanca, nunca descarta nada) e roda o instalar.sh.
# A instalacao de uma versao publicada (tag vX.Y.Z, main) nao usa este script.
#
# Recusa, sem mexer em nada, quando:
#   - a pasta nao e um clone git, ou o git nao esta instalado;
#   - a branch atual nao e a develop;
#   - ha alteracoes locais em arquivos do repositorio;
#   - ha commits locais que nao estao no develop do GitHub (historico divergente).
#
# Codigos de saida (lidos pelo gerenciador):
#   0  atualizado e reinstalado      10  ja estava na ultima do develop
#   2  recusado (o motivo vai na saida)  1  erro
#
# Uso:
#   scripts/geral/atualizar.sh                  # busca e, se tiver novidade, reinstala
#   scripts/geral/atualizar.sh --so-codigo      # so busca (o Windows reinstala como Administrador a parte)
#   scripts/geral/atualizar.sh -- --pular-build # depois do "--", repassado ao instalar.sh
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

readonly BRANCH=develop
readonly RECUSADO=2
readonly SEM_NOVIDADE=10

so_codigo=0
repassar=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --so-codigo) so_codigo=1 ;;
        --) shift; repassar=("$@"); break ;;
        *) erro_sai "Argumento desconhecido: $1 (use --so-codigo ou -- <argumentos do instalar.sh>)" ;;
    esac
    shift
done

recusa() {
    printf '\033[33m    !!  %s\033[0m\n' "$1" >&2
    [[ -n "${2:-}" ]] && printf '        %s\n' "$2" >&2
    exit "$RECUSADO"
}

etapa 'Conferindo a pasta do projeto'
command -v git >/dev/null 2>&1 || recusa 'O git nao esta instalado.' 'Instale o git, ou atualize pelos scripts a mao.'
git -C "$Raiz" rev-parse --is-inside-work-tree >/dev/null 2>&1 \
    || recusa "A pasta $Raiz nao e um clone git." 'Esta atualizacao e so para a instalacao de desenvolvimento, feita a partir de um clone do repositorio.'

atual="$(git -C "$Raiz" rev-parse --abbrev-ref HEAD)"
[[ "$atual" == "$BRANCH" ]] \
    || recusa "A pasta esta na branch \"$atual\", nao na $BRANCH." "Nada foi trocado. Para atualizar pelo develop: git switch $BRANCH (depois de guardar o que estiver fazendo)."

alteracoes="$(git -C "$Raiz" status --porcelain --untracked-files=no)"
[[ -z "$alteracoes" ]] \
    || recusa 'Ha alteracoes locais em arquivos do repositorio; nada foi descartado.' "Guarde (git stash) ou desfaca antes. Arquivos: $(head -5 <<<"$alteracoes" | awk '{print $2}' | tr '\n' ' ')"
ok "clone na $BRANCH, sem alteracoes locais"

etapa "Buscando o $BRANCH no GitHub"
git -C "$Raiz" fetch --quiet origin "$BRANCH" || erro_sai 'Nao deu para buscar o develop (sem internet?). Nada mudou.'
local_sha="$(git -C "$Raiz" rev-parse HEAD)"
remoto_sha="$(git -C "$Raiz" rev-parse "origin/$BRANCH")"
if [[ "$local_sha" == "$remoto_sha" ]]; then
    ok "ja esta na ultima do $BRANCH ($(git -C "$Raiz" log -1 --format='%h, %cd' --date=short))"
    exit "$SEM_NOVIDADE"
fi
git -C "$Raiz" merge-base --is-ancestor HEAD "origin/$BRANCH" \
    || recusa "Ha commits locais que nao estao no $BRANCH do GitHub; nada foi mudado." 'Envie (git push) ou desfaca esses commits antes.'

echo "    Novidades:"
git -C "$Raiz" log --oneline --no-decorate "HEAD..origin/$BRANCH" | head -20 | sed 's/^/      /'
git -C "$Raiz" merge --ff-only --quiet "origin/$BRANCH" || erro_sai 'Nao deu para avancar para o develop novo.'
ok "codigo atualizado para $(git -C "$Raiz" log -1 --format='%h')"

if [[ "$so_codigo" -eq 1 ]]; then
    exit 0
fi

etapa 'Reinstalando (instalar.sh)'
bash "$script_dir/instalar.sh" "${repassar[@]}"
