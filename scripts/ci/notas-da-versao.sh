#!/usr/bin/env bash
# =====================================================================
# #641: notas do Release da tag vX.Y.Z (workflow Versão).
#
# "Como instalar ou atualizar" e, do CHANGELOG, a seção da versão e as
# versões "· desenvolvimento" abaixo dela, até a última publicada
# (exclusive), um nível abaixo ("### X.Y.Z — data", "#### Correções") e
# sem o "· desenvolvimento". Sem a seção da versão, a "[Não publicado]".
#
# O GitHub recusa notas com mais de 125 mil caracteres, e entre duas
# versões publicadas cabem muitas de desenvolvimento (da 2.17.0 à 3.3.5,
# quase 200). Como no pré-lançamento de desenvolvimento (#600), as versões
# entram até onde couber (contando bytes, que nunca são menos que os
# caracteres), cortadas no fim de uma versão, com um link para o
# CHANGELOG da tag.
#
# Ambiente: VERSAO (ex.: 3.3.5), GITHUB_REPOSITORY.
#
# Uso:  scripts/ci/notas-da-versao.sh > notas.md
# =====================================================================
set -euo pipefail

: "${VERSAO:?informe VERSAO}" "${GITHUB_REPOSITORY:?informe GITHUB_REPOSITORY}"
# A folga cobre a diferença de contagem do lado de lá.
LIMITE_NOTAS="${LIMITE_NOTAS:-120000}"
repo="https://github.com/$GITHUB_REPOSITORY"

cabecalho="$(cat <<EOF
## Como instalar ou atualizar

- **Windows:** baixe o \`FinancasLocal-$VERSAO.msi\` (em **Assets**, abaixo) e abra. Ele instala ou atualiza a versão que você tem e **mantém os seus dados**. Quem já tem o Gerenciador do Finanças Local recebe o aviso e atualiza por ele.
- **"O Windows protegeu o computador":** o instalador ainda não é assinado. Clique em **Mais informações › Executar assim mesmo**. Para conferir o arquivo, compare o \`Get-FileHash\` dele com o \`SHA256SUMS.txt\`.
- **Linux, ou instalado pelos scripts:** veja [Instalar a partir do código]($repo/blob/v$VERSAO/docs/INSTALACAO-PELO-CODIGO.md).

## O que mudou
EOF
)"
printf '%s\n\n' "$cabecalho"

resto=$(( LIMITE_NOTAS - $(printf '%s\n\n' "$cabecalho" | LC_ALL=C wc -c) ))
link="_As versões mais antigas não cabem aqui: veja o [CHANGELOG.md]($repo/blob/v$VERSAO/CHANGELOG.md)._"

LC_ALL=C awk -v versao="$VERSAO" -v resto="$resto" -v link="$link" '
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
        } else if (index($0, "## [" versao "]") != 1 && index($0, "## [Não publicado]") != 1) {
            next
        }
        f = 1
        sub(/^## \[/, "### "); gsub(/\]/, ""); gsub(/ · desenvolvimento/, "")
        secao = $0 "\n"; next
    }
    f && /^### / { sub(/^### /, "#### ") }
    f { secao = secao $0 "\n" }
    END {
        if (!acabou) fechar()
        sub(/\n+$/, "", saida)
        if (cortou) saida = (saida == "" ? link : saida "\n\n" link)
        if (saida != "") print saida
    }' CHANGELOG.md
