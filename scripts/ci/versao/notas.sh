#!/usr/bin/env bash
# =====================================================================
# Notas dos Releases do GitHub (workflow Versão), em dois modos que
# dividem o corte no limite de 125 mil caracteres do GitHub (#655;
# eram o notas-da-versao.sh e o notas-desenvolvimento.sh):
#
# --versao (#641): notas do Release da tag vX.Y.Z.
#   "Como instalar ou atualizar" e, do CHANGELOG, a seção da versão e as
#   versões "· desenvolvimento" abaixo dela, até a última publicada
#   (exclusive), um nível abaixo ("### X.Y.Z — data", "#### Correções") e
#   sem o "· desenvolvimento". Sem a seção da versão, a "[Não publicado]".
#   Ambiente: VERSAO (ex.: 3.3.5), GITHUB_REPOSITORY.
#
# --desenvolvimento (#294): notas do pré-lançamento "desenvolvimento".
#   Além de "tudo desde a última versão publicada" (as seções
#   "· desenvolvimento" do CHANGELOG, uma por versão, ADR 0039), traz um
#   bloco por versão de desenvolvimento com os PRs que entraram nela
#   (título e número). Os blocos se acumulam de uma versão para a outra
#   (os 20 mais novos), entre os marcadores <!-- por-versao:inicio --> e
#   <!-- por-versao:fim -->: o gerenciador mostra primeiro só os blocos
#   mais novos que a versão instalada (FormNotas).
#   Ambiente: ROTULO (ex.: 2.26.1-dev.180), GITHUB_SHA, GITHUB_REPOSITORY,
#   GH_TOKEN. Precisa do histórico do git (checkout com fetch-depth: 0).
#   Para testar sem o GitHub: NOTAS_ANTERIORES=<arquivo com as notas antigas>.
#
# O GitHub recusa notas com mais de 125 mil caracteres (#600, #641), e
# entre duas versões publicadas cabem muitas de desenvolvimento (da 2.17.0
# à 3.3.5, quase 200). Nos dois modos as versões do CHANGELOG entram até
# onde couber (contando bytes, que nunca são menos que os caracteres),
# cortadas no fim de uma versão, com um link para o CHANGELOG. O que os
# gerenciadores leem (cabeçalho e blocos por versão) entra inteiro.
#
# Uso:  scripts/ci/versao/notas.sh --versao > notas.md
#       scripts/ci/versao/notas.sh --desenvolvimento > notas.md
# =====================================================================
set -euo pipefail

modo="${1:-}"
[[ "$modo" == "--versao" || "$modo" == "--desenvolvimento" ]] || { echo "uso: $0 --versao | --desenvolvimento" >&2; exit 2; }

: "${GITHUB_REPOSITORY:?informe GITHUB_REPOSITORY}"
# A folga cobre a diferença de contagem do lado de lá.
LIMITE_NOTAS="${LIMITE_NOTAS:-120000}"

if [[ "$modo" == "--versao" ]]; then
    : "${VERSAO:?informe VERSAO}"
    versao="$VERSAO"
    repo="https://github.com/$GITHUB_REPOSITORY"
    link="_As versões mais antigas não cabem aqui: veja o [CHANGELOG.md]($repo/blob/v$VERSAO/CHANGELOG.md)._"
    cabecalho="$(cat <<EOF
## Como instalar ou atualizar

- **Windows:** baixe o \`FinancasLocal-$VERSAO.msi\` (em **Assets**, abaixo) e abra. Ele instala ou atualiza a versão que você tem e **mantém os seus dados**. Quem já tem o Gerenciador do Finanças Local recebe o aviso e atualiza por ele.
- **"O Windows protegeu o computador":** o instalador ainda não é assinado. Clique em **Mais informações › Executar assim mesmo**. Para conferir o arquivo, compare o \`Get-FileHash\` dele com o \`SHA256SUMS.txt\`.
- **Linux, ou instalado pelos scripts:** veja [Instalar a partir do código]($repo/blob/v$VERSAO/docs/INSTALACAO-PELO-CODIGO.md).

## O que mudou
EOF
)"
else
    : "${ROTULO:?informe ROTULO}" "${GITHUB_SHA:?informe GITHUB_SHA}"
    versao=""
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

    link="_As versões mais antigas não cabem aqui: veja o [CHANGELOG.md](https://github.com/$GITHUB_REPOSITORY/blob/$GITHUB_SHA/CHANGELOG.md)._"
    cabecalho="$(cat <<EOF
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
)"
fi
printf '%s\n\n' "$cabecalho"

resto=$(( LIMITE_NOTAS - $(printf '%s\n\n' "$cabecalho" | LC_ALL=C wc -c) ))

# As seções do CHANGELOG, um nível abaixo ("## [2.26.1] — data" vira "### 2.26.1 — data", e "### Correções",
# "#### Correções"), do topo até a última versão publicada (a primeira sem "· desenvolvimento"), que fica de fora.
# --versao: começa na seção da versão (ou na "[Não publicado]"), a publicada entra, e tira o "· desenvolvimento" dos
# títulos. --desenvolvimento: começa no topo, que já é de desenvolvimento, e mantém o título e as linhas em branco.
LC_ALL=C awk -v versao="$versao" -v resto="$resto" -v link="$link" '
    # Junta a seção que acabou, se ela couber inteira com o link; se não, para ali.
    function fechar() {
        if (secao == "") return
        if (usado + length(secao) + length(link) + 2 > resto) { cortou = 1; return }
        saida = saida secao; usado += length(secao); secao = ""
    }
    { sub(/\r$/, "") }
    /^## \[/ {
        if (f) {
            fechar()
            if (cortou || $0 !~ /desenvolvimento/) { acabou = 1; exit }
        } else if (versao != "") {
            if (index($0, "## [" versao "]") != 1 && index($0, "## [Não publicado]") != 1) next
        } else if ($0 !~ /desenvolvimento/) {
            exit
        }
        f = 1
        sub(/^## \[/, "### "); gsub(/\]/, "")
        if (versao != "") gsub(/ · desenvolvimento/, "")
        secao = $0 "\n"; next
    }
    f && /^### / { sub(/^### /, "#### ") }
    f { secao = secao $0 "\n" }
    END {
        if (!acabou) fechar()
        if (versao != "") {
            sub(/\n+$/, "", saida)
            if (cortou) saida = (saida == "" ? link : saida "\n\n" link)
            if (saida != "") print saida
        } else {
            printf "%s", saida
            if (cortou) print "\n" link
        }
    }' CHANGELOG.md
