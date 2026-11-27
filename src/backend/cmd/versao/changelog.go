package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// pastaDoChangelog tem um arquivo por PR (ADR 0070): changelog.d/<nome-da-branch>.md.
const pastaDoChangelog = "changelog.d"

// arquivoDoChangelog é o que o lancar-versao.sh monta na publicação; antes da ADR 0070, cada PR escrevia nele.
const arquivoDoChangelog = "CHANGELOG.md"

// tiposDoChangelog é a ordem dos títulos de tipo nas seções; um título fora da lista vai depois, na ordem em que apareceu.
var tiposDoChangelog = []string{"### Funcionalidades", "### Correções", "### Publicação e documentação"}

// ehEntradaDoChangelog: um .md da pasta, menos o LEIA-ME.md.
func ehEntradaDoChangelog(arquivo string) bool {
	nome := path.Base(arquivo)
	return path.Dir(arquivo) == pastaDoChangelog && strings.HasSuffix(nome, ".md") && nome != "LEIA-ME.md"
}

// entrada é um arquivo de changelog.d/ com o número da versão do merge que o trouxe.
type entrada struct {
	arquivo  string
	versao   numero
	data     string
	ordem    int // a posição do merge na linha principal; -1 para o que veio antes da base
	conteudo string
}

// bloco é um título de tipo ("### Correções") e as linhas dele.
type bloco struct {
	titulo string
	linhas []string
}

// blocosDe separa o conteúdo de uma entrada pelos títulos "### ". O que vier antes do primeiro título (um arquivo sem
// tipo, que a CI recusa) fica num bloco "### Sem tipo", para não se perder em silêncio.
func blocosDe(conteudo string) []bloco {
	var blocos []bloco
	for linha := range strings.SplitSeq(strings.ReplaceAll(conteudo, "\r\n", "\n"), "\n") {
		linha = strings.TrimRight(linha, " \t")
		if strings.HasPrefix(linha, "### ") {
			blocos = append(blocos, bloco{titulo: linha})
			continue
		}
		if len(blocos) == 0 {
			if linha == "" {
				continue
			}
			blocos = append(blocos, bloco{titulo: "### Sem tipo"})
		}
		b := &blocos[len(blocos)-1]
		b.linhas = append(b.linhas, linha)
	}
	for i := range blocos {
		blocos[i].linhas = aparar(blocos[i].linhas)
	}
	return blocos
}

// aparar tira as linhas em branco das pontas.
func aparar(linhas []string) []string {
	for len(linhas) > 0 && linhas[0] == "" {
		linhas = linhas[1:]
	}
	for len(linhas) > 0 && linhas[len(linhas)-1] == "" {
		linhas = linhas[:len(linhas)-1]
	}
	return linhas
}

// secao junta as entradas num texto "### Tipo" + linhas, um título por tipo, na ordem de tiposDoChangelog.
func secao(entradas []entrada) string {
	porTitulo := map[string][]string{}
	var ordem []string
	for _, e := range entradas {
		for _, b := range blocosDe(e.conteudo) {
			if _, visto := porTitulo[b.titulo]; !visto {
				ordem = append(ordem, b.titulo)
			}
			porTitulo[b.titulo] = append(porTitulo[b.titulo], b.linhas...)
		}
	}
	slices.SortStableFunc(ordem, func(a, b string) int { return posicaoDoTipo(a) - posicaoDoTipo(b) })
	var texto strings.Builder
	for _, titulo := range ordem {
		fmt.Fprintf(&texto, "%s\n%s\n\n", titulo, strings.Join(porTitulo[titulo], "\n"))
	}
	return texto.String()
}

func posicaoDoTipo(titulo string) int {
	if i := slices.Index(tiposDoChangelog, titulo); i >= 0 {
		return i
	}
	return len(tiposDoChangelog)
}

// secoesPorVersao são as seções "## [X.Y.Z] — data · desenvolvimento", a mais nova primeiro, uma por versão (a do merge
// de cada PR), no formato que o notas.sh e o gerenciador já leem. Versão sem entrada não tem seção. As entradas
// chegam da mais nova para a mais antiga.
func secoesPorVersao(entradas []entrada) string {
	var texto strings.Builder
	for i := 0; i < len(entradas); {
		j := i
		for j < len(entradas) && entradas[j].versao == entradas[i].versao {
			j++
		}
		fmt.Fprintf(&texto, "## [%s] — %s · desenvolvimento\n\n%s", entradas[i].versao, entradas[i].data, secao(entradas[i:j]))
		i = j
	}
	return texto.String()
}

// secaoPublicada é a seção da versão publicada: todas as entradas juntas, por tipo, sem o "· desenvolvimento".
func secaoPublicada(entradas []entrada, versao, data string) string {
	return fmt.Sprintf("## [%s] — %s\n\n%s", versao, data, secao(entradas))
}

// gravarPublicacao põe a seção da versão publicada no topo do CHANGELOG.md (antes da primeira "## [") e tira do git os
// arquivos de changelog.d/ que ela juntou, deixando tudo no índice para o commit "chore: versão X.Y.Z".
func gravarPublicacao(r repositorio, secao string, entradas []entrada) error {
	if len(entradas) == 0 {
		return fmt.Errorf("%s/ está sem entradas: escreva ao menos uma (publicar sem nada a dizer deixaria as notas vazias)", pastaDoChangelog)
	}
	caminho := filepath.Join(r.raiz, arquivoDoChangelog)
	atual, err := os.ReadFile(caminho)
	if err != nil {
		return err
	}
	if err := os.WriteFile(caminho, []byte(inserirSecao(string(atual), secao)), 0o644); err != nil {
		return err
	}
	if _, err := r.git("add", "--", arquivoDoChangelog); err != nil {
		return err
	}
	for _, e := range entradas {
		if _, err := r.git("rm", "-q", "--", e.arquivo); err != nil {
			return err
		}
	}
	return nil
}

// inserirSecao põe a seção antes da primeira versão do CHANGELOG ("## ["), ou no fim se ainda não há nenhuma.
func inserirSecao(changelog, secao string) string {
	if strings.HasPrefix(changelog, "## [") {
		return secao + changelog
	}
	if i := strings.Index(changelog, "\n## ["); i >= 0 {
		return changelog[:i+1] + secao + changelog[i+1:]
	}
	return strings.TrimRight(changelog, "\n") + "\n\n" + secao
}
