package leitor

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor/ficticios"
)

// As bordas do leitor: empate de coluna, várias páginas, a tabela até o fim da página, a segunda via, o CNPJ repetido,
// o CNPJ na primeira linha, a folga de 10 pontos do valor embaixo do rótulo, o limite do arquivo e a ordem das letras.

func TestPertoNoEmpateFicaComAPrimeira(t *testing.T) {
	if got := perto(5, 0, 10); got != 0 {
		t.Errorf("perto(5, 0, 10) = %d, quero 0", got)
	}
	if got := perto(6, 0, 10, 12); got != 1 {
		t.Errorf("perto(6, 0, 10, 12) = %d, quero 1", got)
	}
}

// Duas páginas com tabela: vale a primeira.
func TestVariasPaginasComTabelaValeAPrimeira(t *testing.T) {
	l := ler(t, [][]ficticios.Texto{ficticios.ViaClassica(), ficticios.ProventosEQtde(), ficticios.Pagina(ficticios.Linha(0, 30, "Aviso"))})
	if l.Empregador != "EMPRESA FICTÍCIA DE TESTES LTDA" || len(l.Rubricas) != 7 {
		t.Errorf("empregador %q, %d rubricas", l.Empregador, len(l.Rubricas))
	}
}

// A tabela que vai até o fim da página, sem totais nem rodapé.
func TestTabelaAteOFimDaPagina(t *testing.T) {
	l := ler(t, [][]ficticios.Texto{ficticios.Pagina(
		ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
		ficticios.Linha(0, 30, "Salário", 295, "2.000,00"),
		ficticios.Linha(0, 30, "INSS", 400, "150,00"),
	)})
	if got := rubricas(l); got != "|Salário||2000.00|P\n|INSS||150.00|D\n" {
		t.Errorf("rubricas: %q", got)
	}
}

// A segunda via começa com o título da tabela (sem CNPJ) ou com o CNPJ: o rodapé dela não entra, nem no campo que a
// primeira não trouxe.
func TestSegundaViaNaoEntraNoRodape(t *testing.T) {
	primeira := []([]ficticios.Texto){
		ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
		ficticios.Linha(0, 30, "Salário", 295, "2.000,00"),
		ficticios.Linha(0, 30, "Valor Líquido", 150, "2.000,00"),
	}
	casos := map[string][]ficticios.Texto{
		"pelo título da tabela": ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
		"pelo CNPJ":             ficticios.Linha(0, 30, "ACME LTDA", 300, "CNPJ 10.000.000/0001-00"),
	}
	for nome, comeco := range casos {
		linhas := append(append([]([]ficticios.Texto){}, primeira...), comeco, ficticios.Linha(0, 30, "FGTS do Mês", 150, "99,99"))
		l := ler(t, [][]ficticios.Texto{ficticios.Pagina(linhas...)})
		if l.FgtsDoMes != nil {
			t.Errorf("%s: o FGTS da segunda via entrou: %s", nome, emTexto(l.FgtsDoMes))
		}
	}
}

// Vale o primeiro CNPJ do cabeçalho (o segundo pode ser o do tomador do serviço), e o CNPJ sozinho na primeira linha
// não quebra a busca do nome.
func TestCnpjDoCabecalho(t *testing.T) {
	l := ler(t, [][]ficticios.Texto{ficticios.Pagina(
		ficticios.Linha(0, 30, "CNPJ: 12.345.678/0001-95"),
		ficticios.Linha(0, 30, "Tomador: BETA LTDA", 300, "CNPJ 98.765.432/0001-10"),
		ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
		ficticios.Linha(0, 30, "Salário", 295, "2.000,00"),
	)})
	if l.Cnpj == nil || *l.Cnpj != "12345678000195" || l.Empregador != "" {
		t.Errorf("cnpj %v, empregador %q", l.Cnpj, l.Empregador)
	}
}

// O valor embaixo do rótulo vale até 10 pontos para cada lado, inclusive.
func TestValorEmbaixoNaFolgaExata(t *testing.T) {
	rotulo := Linha{Celulas: []Celula{{Texto: "Líquido", X0: 100, X1: 140}}}
	casos := []struct {
		nome   string
		x0, x1 Pontos
		achou  bool
	}{
		{"acaba 10 antes", 50, 90, true},
		{"acaba 10,5 antes", 50, 89.5, false},
		{"começa 10 depois", 150, 190, true},
		{"começa 10,5 depois", 150.5, 190, false},
	}
	for _, c := range casos {
		linhas := []Linha{rotulo, {Celulas: []Celula{{Texto: "1.850,00", X0: c.x0, X1: c.x1}}}}
		if _, ok := valorDoRotulo(linhas, 0, 0); ok != c.achou {
			t.Errorf("%s: achou %t", c.nome, ok)
		}
	}
}

func TestTamanhoNoLimite(t *testing.T) {
	noLimite := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte(" "), TamanhoMaximo-9)...)
	if _, err := LerPDF(noLimite, ""); !errors.Is(err, ErrIlegivel) {
		t.Errorf("no limite: %v, quero ErrIlegivel (não ErrGrande)", err)
	}
}

// A ordem das letras: na mesma linha (até 2 pontos), pela posição da esquerda para a direita, mesmo que a de cima
// venha depois; na mesma posição, a ordem do PDF.
func TestOrdemDasLetras(t *testing.T) {
	l := func(s string, x, y Pontos) letra { return letra{s: s, x: x, y: y, w: 5, fonte: 10} }
	casos := []struct {
		nome   string
		letras []letra
		want   string
	}{
		{"2 pontos acima, mas à direita", []letra{l("b", 5, 100), l("a", 0, 98)}, "ab"},
		{"mesma posição: a ordem do PDF", []letra{l("a", 0, 100), l("b", 0, 100)}, "ab"},
	}
	for _, c := range casos {
		var textos []string
		for _, ln := range Agrupar(c.letras) {
			textos = append(textos, ln.Texto())
		}
		if got := strings.Join(textos, "/"); got != c.want {
			t.Errorf("%s: %q, quero %q", c.nome, got, c.want)
		}
	}
}
