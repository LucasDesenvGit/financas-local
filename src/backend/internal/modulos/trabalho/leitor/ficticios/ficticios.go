// Pacote ficticios gera os holerites em PDF dos testes do leitor (#228): PDFs de verdade, mínimos, em quatro estilos
// de sistemas de folha comuns. Empresas, CNPJs, pessoas e valores inventados: nunca holerite real no repositório. Só
// os testes importam este pacote; ele não entra no executável.
package ficticios

import (
	"bytes"
	"crypto/md5"
	"crypto/rc4"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Texto é um texto numa posição (pontos, a origem embaixo à esquerda), para os PDFs fictícios dos testes.
type Texto struct {
	X, Y  int
	Texto string
}

// preenchimento da senha do PDF (PDF 32000-1:2008, §7.6.3.3, Algoritmo 2).
var preenchimento = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

// GerarPDF monta um PDF de verdade, mínimo, com Courier (cada letra com 600/1000 da fonte de largura) e WinAnsi,
// uma página por lista. Com senha, o PDF sai criptografado com RC4 de 40 bits (revisão 2), como muitos holerites que
// os bancos e as empresas mandam (a senha é o CPF ou a data de nascimento).
func GerarPDF(paginas [][]Texto, senha string) []byte {
	var objs []string
	kids := []string{}
	n := len(paginas)
	o := bytes.Repeat([]byte{0x4F}, 32) // a biblioteca não confere a senha do dono: basta ter 32 bytes
	id := []byte("holerite-ficticio")
	p := uint32(0xFFFFFFFC)
	var chave []byte
	if senha != "" {
		h := md5.New()
		h.Write(append([]byte(senha), preenchimento[:32-len(senha)]...))
		h.Write(o)
		h.Write([]byte{byte(p), byte(p >> 8), byte(p >> 16), byte(p >> 24)})
		h.Write(id)
		chave = h.Sum(nil)[:5]
	}
	cifrar := func(obj int, dados []byte) []byte {
		if chave == nil {
			return dados
		}
		h := md5.New()
		h.Write(chave)
		h.Write([]byte{byte(obj), byte(obj >> 8), byte(obj >> 16), 0, 0})
		c, _ := rc4.NewCipher(h.Sum(nil)[:10])
		saida := make([]byte, len(dados))
		c.XORKeyStream(saida, dados)
		return saida
	}
	// 1 catálogo, 2 páginas, 3 fonte, depois página e conteúdo de cada uma.
	for i, pg := range paginas {
		var c strings.Builder
		c.WriteString("BT /F1 9 Tf\n")
		for _, t := range pg {
			enc, _ := charmap.Windows1252.NewEncoder().String(t.Texto)
			esc := strings.NewReplacer(`\`, `\`, "(", `\(`, ")", `\)`).Replace(enc)
			fmt.Fprintf(&c, "1 0 0 1 %d %d Tm (%s) Tj\n", t.X, t.Y, esc)
		}
		c.WriteString("ET\n")
		pagina, conteudo := 4+2*i, 5+2*i
		corpo := cifrar(conteudo, []byte(c.String()))
		kids = append(kids, fmt.Sprintf("%d 0 R", pagina))
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", conteudo),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(corpo), corpo))
	}
	todos := append([]string{"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding /FirstChar 32 /LastChar 255 /Widths [" +
			strings.TrimSpace(strings.Repeat("600 ", 224)) + "] >>"}, objs...)
	extra := ""
	if chave != nil {
		u := make([]byte, 32)
		c, _ := rc4.NewCipher(chave)
		c.XORKeyStream(u, preenchimento)
		todos = append(todos, fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /Length 40 /O <%x> /U <%x> /P %d >>", o, u, int32(p)))
		extra = fmt.Sprintf(" /Encrypt %d 0 R /ID [<%x> <%x>]", len(todos), id, id)
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(todos))
	for i, o := range todos {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(todos)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R%s >>\nstartxref\n%d\n%%%%EOF\n", len(todos)+1, extra, xref)
	return b.Bytes()
}

// Linha põe os textos numa altura, cada um na coluna dele (x).
func Linha(y int, celulas ...any) []Texto {
	var t []Texto
	for i := 0; i+1 < len(celulas); i += 2 {
		t = append(t, Texto{X: celulas[i].(int), Y: y, Texto: celulas[i+1].(string)})
	}
	return t
}

// Pagina junta as linhas de cima para baixo, de 12 em 12 pontos a partir do topo.
func Pagina(linhas ...[]Texto) []Texto {
	var t []Texto
	for i, l := range linhas {
		for _, c := range l {
			c.Y = 800 - i*12
			t = append(t, c)
		}
	}
	return t
}

// ViaClassica é o estilo mais comum: código, descrição, referência, vencimentos e descontos; os totais com os rótulos
// em cima e os valores embaixo; o rodapé com as bases em grade.
func ViaClassica() []Texto {
	return Pagina(
		Linha(0, 30, "EMPRESA FICTÍCIA DE TESTES LTDA", 330, "Recibo de Pagamento de Salário"),
		Linha(0, 30, "CNPJ: 12.345.678/0001-95", 330, "Referente ao Mês: Outubro/2026"),
		Linha(0, 30, "Código", 80, "Nome do Funcionário", 300, "CBO", 380, "Admissão"),
		Linha(0, 30, "000042", 80, "FULANA DE TESTE", 300, "252210", 380, "01/03/2020"),
		Linha(0, 30, "Cód.", 65, "Descrição", 250, "Referência", 330, "Vencimentos", 430, "Descontos"),
		Linha(0, 30, "001", 65, "SALARIO", 265, "30,00", 340, "5.000,00"),
		Linha(0, 30, "101", 65, "HORAS EXTRAS 50%", 265, "10,00", 350, "340,91"),
		Linha(0, 30, "102", 65, "DSR S/ HORAS EXTRAS", 355, "68,18"),
		Linha(0, 30, "998", 65, "INSS", 265, "12,02", 445, "601,22"),
		Linha(0, 30, "999", 65, "IRRF", 265, "22,50", 445, "437,45"),
		Linha(0, 30, "210", 65, "VALE TRANSPORTE 6%", 270, "6,00", 445, "300,00"),
		Linha(0, 30, "305", 65, "ASSIST. MEDICA", 445, "150,00"),
		Linha(0, 300, "Total de Vencimentos", 430, "Total de Descontos"),
		Linha(0, 340, "5.409,09", 470, "1.488,67"),
		Linha(0, 300, "Valor Líquido", 385, "->", 470, "3.920,42"),
		Linha(0, 30, "Salário Base", 115, "Sal. Contr. INSS", 215, "Base Cálc. FGTS", 315, "F.G.T.S do Mês", 415, "Base Cálc. IRRF", 515, "Faixa IRRF"),
		Linha(0, 40, "5.000,00", 125, "5.409,09", 225, "5.409,09", 325, "432,73", 425, "4.807,87", 525, "22,5"),
	)
}

// DuasVias: a mesma página com a segunda via embaixo (a da empresa), que o leitor não pode somar.
func DuasVias() []Texto {
	primeira := ViaClassica()
	segunda := ViaClassica()
	for i := range segunda {
		segunda[i].Y -= 220
	}
	return append(primeira, segunda...)
}

// ProventosEQtde: sem código, "Proventos" e "Qtde", a referência em horas ("220:00"), a competência "10/2026", o
// rótulo e o valor na mesma célula ("Base INSS: 4.130,91") e a linha "Totais" com os dois números.
func ProventosEQtde() []Texto {
	return Pagina(
		Linha(0, 30, "DEMONSTRATIVO DE PAGAMENTO MENSAL", 380, "Competência: 10/2026"),
		Linha(0, 30, "Empregador: COMÉRCIO EXEMPLO S.A.", 300, "CNPJ 98.765.432/0001-10"),
		Linha(0, 30, "Funcionário: CICRANO EXEMPLO", 300, "Admissão: 01/03/2020"),
		Linha(0, 30, "Descrição", 200, "Qtde", 290, "Proventos", 400, "Descontos"),
		Linha(0, 30, "Salário Mensal", 195, "220:00", 295, "4.000,00"),
		Linha(0, 30, "Adicional Noturno 20%", 200, "20:00", 305, "109,09"),
		Linha(0, 30, "Repouso Remunerado", 310, "21,82"),
		Linha(0, 30, "Contrib. Previdenciária", 410, "351,90"),
		Linha(0, 30, "Imposto de Renda", 415, "32,47"),
		Linha(0, 30, "Desc. VT", 410, "240,00"),
		Linha(0, 30, "Vale Refeição", 415, "80,00"),
		Linha(0, 30, "Pensão Alimentícia", 410, "400,00"),
		Linha(0, 30, "Adto. Salarial", 400, "1.200,00"),
		Linha(0, 30, "Totais", 295, "4.130,91", 400, "2.304,37"),
		Linha(0, 30, "Líquido a Receber: 1.826,54"),
		Linha(0, 30, "Base INSS: 4.130,91", 160, "Base FGTS: 4.130,91", 290, "FGTS do mês: 330,47", 420, "Base IRRF: 3.379,01"),
		Linha(0, 30, "Salário Base: 4.000,00"),
	)
}

// CreditosEDebitos: "Créditos" e "Débitos", a referência em dias e em % ("30d", "8,63%"), a competência por extenso
// ("outubro de 2026"), o CNPJ na linha do nome, os totais lado a lado e uma página de aviso antes do holerite.
func CreditosEDebitos() [][]Texto {
	return [][]Texto{
		Pagina(Linha(0, 30, "Comunicado: o seu holerite está na próxima página.")),
		Pagina(
			Linha(0, 30, "INDÚSTRIA DE TESTE EIRELI - CNPJ 11.222.333/0001-81"),
			Linha(0, 30, "Contracheque referente a outubro de 2026"),
			Linha(0, 30, "Cód", 70, "Discriminação", 240, "Ref.", 320, "Créditos", 430, "Débitos"),
			Linha(0, 30, "0010", 70, "Salário", 240, "30d", 320, "3.200,00"),
			Linha(0, 30, "0250", 70, "H.E. 100%", 240, "5:00", 330, "145,45"),
			Linha(0, 30, "0251", 70, "Reflexo DSR H.E.", 335, "29,09"),
			Linha(0, 30, "0600", 70, "I.N.S.S.", 240, "8,63%", 435, "289,74"),
			Linha(0, 30, "0610", 70, "Seguro de Vida", 440, "12,50"),
			Linha(0, 30, "0620", 70, "Plano Odontológico", 440, "25,90"),
			Linha(0, 30, "Total de Créditos", 150, "3.374,54", 300, "Total de Débitos", 400, "328,14"),
			Linha(0, 30, "Valor Líquido", 150, "3.046,40"),
			Linha(0, 30, "FGTS do Mês", 150, "269,96"),
		),
	}
}

// EventosEReais: "Evento", "Quantidade", os totais com "R$", o líquido com o valor na linha de baixo, a rubrica
// "SALARIO BASE" na tabela (não é o rodapé) e o rodapé só com os rótulos curtos.
func EventosEReais() []Texto {
	return Pagina(
		Linha(0, 30, "FOLHA DE PAGAMENTO - MENSAL", 330, "Período: 01/10/2026 a 31/10/2026"),
		Linha(0, 30, "Empresa: SERVIÇOS DIGITAIS DE EXEMPLO ME"),
		Linha(0, 30, "CNPJ: 55.444.333/0001-22", 330, "Mês/Ano: 10/2026"),
		Linha(0, 30, "Evento", 75, "Descrição", 230, "Quantidade", 320, "Proventos", 420, "Descontos"),
		Linha(0, 30, "1000", 75, "SALARIO BASE", 245, "30,00", 320, "8.000,00"),
		Linha(0, 30, "1050", 75, "GRATIFICACAO", 330, "800,00"),
		Linha(0, 30, "1100", 75, "PLR", 320, "1.500,00"),
		Linha(0, 30, "5001", 75, "INSS FOLHA", 245, "14,00", 430, "988,07"),
		Linha(0, 30, "5002", 75, "IRRF FOLHA", 245, "27,50", 420, "1.623,18"),
		Linha(0, 30, "5100", 75, "PREVIDENCIA PRIVADA", 430, "400,00"),
		Linha(0, 30, "5200", 75, "PLANO DE SAUDE", 430, "320,00"),
		Linha(0, 30, "5300", 75, "ADIANTAMENTO QUINZENAL", 420, "3.200,00"),
		Linha(0, 30, "5400", 75, "CONTRIB. SINDICAL", 440, "45,00"),
		Linha(0, 30, "TOTAL PROVENTOS", 130, "R$ 10.300,00", 300, "TOTAL DESCONTOS", 400, "R$ 6.576,25"),
		Linha(0, 30, "LÍQUIDO"),
		Linha(0, 30, "R$ 3.723,75"),
		Linha(0, 30, "BASE INSS", 110, "BASE FGTS", 190, "FGTS", 250, "BASE IRRF"),
		Linha(0, 30, "8.800,00", 110, "8.800,00", 190, "704,00", 250, "7.811,93"),
	)
}
