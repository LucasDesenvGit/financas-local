#!/usr/bin/env bash
# =====================================================================
# #294: notas do pré-lançamento "desenvolvimento" (workflow Versão).
#
# Além de "tudo desde a última versão publicada" (as seções "· desenvolvimento"
# do CHANGELOG, uma por versão, ADR 0039), as notas trazem um bloco por versão de desenvolvimento
# com os PRs que entraram nela (título e número). Os blocos se acumulam de
# uma versão para a outra (os 20 mais novos), entre os marcadores
# <!-- por-versao:inicio --> e <!-- por-versao:fim -->: o gerenciador mostra
# primeiro só os blocos mais novos que a versão instalada (FormNotas).
#
# Ambiente: ROTULO (ex.: 2.26.1-dev.180), GITHUB_SHA, GITHUB_REPOSITORY,
# GH_TOKEN. Precisa do histórico do git (checkout com fetch-depth: 0).
# Para testar sem o GitHub: NOTAS_ANTERIORES=<arquivo com as notas antigas>.
#
# Uso:  scripts/ci/notas-desenvolvimento.sh > notas.md
# =====================================================================
set -euo pipefail

: "${ROTULO:?informe ROTULO}" "${GITHUB_SHA:?informe GITHUB_SHA}" "${GITHUB_REPOSITORY:?informe GITHUB_REPOSITORY}"
MAXIMO_BLOCOS=20

if [[ -n "${NOTAS_ANTERIORES:-}" ]]; then
    anteriores="$(cat "$NOTAS_ANTERIORES")"
else
    anteriores="$(gh release view desenvolvimento --repo "$GITHUB_REPOSITORY" --json body -q .body 2>/dev/null || true)"
fi
anteriores="${anteriores//$'\r'/}"

# Commit da versão anterior ("gerada do develop no commit <sha>") e os blocos que ela já tinha.
sha_anterior="$(grep -oE 'no commit [0-9a-f]{40}' <<<"$anteriores" | head -n1 | awk '{print $3}' || true)"
blocos_anteriores="$(awk '/<!-- por-versao:fim -->/{f=0} f; /<!-- por-versao:inicio -->/{f=1}' <<<"$anteriores")"

# PRs que entraram desde a anterior: os merges no develop (só o primeiro pai, para não listar os commits de dentro).
if [[ -n "$sha_anterior" ]] && git cat-file -e "${sha_anterior}^{commit}" 2>/dev/null; then
    faixa="$sha_anterior..$GITHUB_SHA"
else
    faixa="$GITHUB_SHA~1..$GITHUB_SHA"
fi
numeros="$( (git log --first-parent --pretty=%s "$faixa" || true) | grep -oE 'Merge pull request #[0-9]+' | grep -oE '[0-9]+' || true)"

linhas=""
for n in $numeros; do
    titulo="$(gh pr view "$n" --repo "$GITHUB_REPOSITORY" --json title -q .title 2>/dev/null || true)"
    [[ -n "$titulo" ]] || titulo="PR #$n"
    linhas+="- $titulo ([#$n](https://github.com/$GITHUB_REPOSITORY/pull/$n))"$'\n'
done
[[ -n "$linhas" ]] || linhas="- Mudanças direto no develop, sem PR."$'\n'

data="$(TZ=America/Sao_Paulo date +%d/%m/%Y)"
bloco_novo="### $ROTULO · $data"$'\n'"$linhas"
# O novo primeiro; os anteriores, sem repetir este rótulo (reexecução do workflow), até o máximo de blocos.
blocos="$(printf '%s\n%s\n' "$bloco_novo" "$blocos_anteriores" \
    | awk -v rotulo="### $ROTULO " -v max="$MAXIMO_BLOCOS" '
        /^### / { n++; pular = (n > 1 && index($0, rotulo) == 1) }
        n <= max && !pular && NF')"

cat <<EOF
**Versão de desenvolvimento $ROTULO**, gerada do develop no commit $GITHUB_SHA.

Tem o que já foi aprovado nos PRs e ainda não saiu numa versão. Pode ter coisa ainda não testada no uso real.
Para o dia a dia, prefira a [versão mais recente](https://github.com/$GITHUB_REPOSITORY/releases/latest).

- **Instalar ou atualizar:** baixe o \`FinancasLocal-desenvolvimento.msi\` e abra. Ele troca a versão instalada (pelo \`.msi\` ou pelos scripts) e mantém os dados.
- **Voltar para a versão publicada:** instale o \`.msi\` da versão mais recente por cima; a mesma versão X.Y.Z troca uma pela outra.
- **Avisos do gerenciador:** instalado este \`.msi\`, o gerenciador avisa e atualiza para a versão de desenvolvimento mais nova (ADR 0040). Quem tem uma versão publicada não recebe aviso destas.

## O que entrou em cada versão de desenvolvimento

A mais nova primeiro. O gerenciador mostra só as mais novas que a sua.

<!-- por-versao:inicio -->
$blocos
<!-- por-versao:fim -->

## Tudo desde a última versão publicada

EOF
# As seções do CHANGELOG do topo até a última versão publicada (a primeira sem "· desenvolvimento"), um nível
# abaixo: "## [2.26.1] — data · desenvolvimento" vira "### 2.26.1 — data · desenvolvimento", e "### Correções", "#### Correções".
awk '
    /^## \[/ { if ($0 !~ /desenvolvimento/) exit; f = 1; sub(/^## \[/, "### "); sub(/\]/, ""); print; next }
    f && /^### / { sub(/^### /, "#### ") }
    f' CHANGELOG.md
