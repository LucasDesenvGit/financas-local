package contasagendadas

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func ptr[T any](v T) *T { return &v }

func dia(d int) tipos.Data { return testeapoio.Dia(2026, 10, d) }

// Regra: o Aplicar do C#: aparar, null no vazio, sem data limite na única, forma fora do enum vira "à mão", texto da
// cobrança normalizado, e a chave do extrato só quando vem (a ligação que já existe fica).
func TestAplicar(t *testing.T) {
	t.Parallel()
	limite := dia(30)
	casos := []struct {
		nome  string
		d     salvarDto
		antes *string
		conta conta
	}{
		{"apara e normaliza",
			salvarDto{Descricao: "  Luz  ", Recorrencia: 1, DataLimite: &limite, FormaCobranca: 2, TextoCobranca: ptr(" enél sp "),
				Observacao: ptr("  obs  "), ChaveExtrato: ptr("  ENEL  ")},
			nil,
			conta{Descricao: "Luz", Recorrencia: 1, DataLimite: &limite, FormaCobranca: 2, TextoCobranca: ptr("ENEL SP"),
				Observacao: ptr("obs"), ChaveExtrato: ptr("ENEL")}},
		{"vazios viram null e a chave antiga fica",
			salvarDto{Descricao: "Luz", Recorrencia: 0, DataLimite: &limite, FormaCobranca: 9, TextoCobranca: ptr("   "),
				Observacao: ptr(" "), ChaveExtrato: ptr("  ")},
			ptr("ANTIGA"),
			conta{Descricao: "Luz", Recorrencia: 0, FormaCobranca: 1, ChaveExtrato: ptr("ANTIGA")}},
		{"forma zero vira à mão", salvarDto{Descricao: "x", FormaCobranca: 0}, nil, conta{Descricao: "x", FormaCobranca: 1}},
		{"forma 3 fica", salvarDto{Descricao: "x", FormaCobranca: 3}, nil, conta{Descricao: "x", FormaCobranca: 3}},
		{"forma 4 vira à mão", salvarDto{Descricao: "x", FormaCobranca: 4}, nil, conta{Descricao: "x", FormaCobranca: 1}},
	}
	for _, c := range casos {
		obtido := conta{ChaveExtrato: c.antes}
		aplicar(&obtido, c.d)
		if obtido.Descricao != c.conta.Descricao || obtido.Recorrencia != c.conta.Recorrencia ||
			obtido.FormaCobranca != c.conta.FormaCobranca || !iguais(obtido.TextoCobranca, c.conta.TextoCobranca) ||
			!iguais(obtido.Observacao, c.conta.Observacao) || !iguais(obtido.ChaveExtrato, c.conta.ChaveExtrato) ||
			(obtido.DataLimite == nil) != (c.conta.DataLimite == nil) {
			t.Errorf("%s: %+v", c.nome, obtido)
		}
	}
}

func iguais(a, b *string) bool { return a == b || (a != nil && b != nil && *a == *b) }

// Regra (#62, #158): mensal é pago nos últimos 45 dias, em 3 meses ou mais, até 1,5 pagamento por mês e cobrindo 60%
// dos meses do intervalo.
func TestEhMensal(t *testing.T) {
	t.Parallel()
	hoje := dia(14)
	d := func(m, dd int) tipos.Data { return testeapoio.Dia(2026, time.Month(m), dd) }
	casos := []struct {
		nome  string
		datas []tipos.Data
		quer  bool
	}{
		{"três meses seguidos", []tipos.Data{d(10, 5), d(9, 5), d(8, 5)}, true},
		{"só dois meses", []tipos.Data{d(10, 5), d(9, 5)}, false},
		{"último há 46 dias", []tipos.Data{d(8, 29), d(7, 29), d(6, 29)}, false},
		{"último há 45 dias", []tipos.Data{d(8, 30), d(7, 30), d(6, 30)}, true},
		{"gasto frequente: 2 por mês", []tipos.Data{d(10, 9), d(10, 2), d(9, 9), d(9, 2), d(8, 9), d(8, 2)}, false},
		{"1,5 por mês ainda vale", []tipos.Data{d(10, 9), d(10, 2), d(9, 9), d(8, 9)}, true},
		{"cobertura de 60% vale", []tipos.Data{d(10, 5), d(9, 5), d(6, 5)}, true},
		{"cobertura abaixo de 60%", []tipos.Data{d(10, 5), d(8, 5), d(5, 5)}, false},
		{"vazio", nil, false},
	}
	for _, c := range casos {
		if obtido := ehMensal(c.datas, hoje); obtido != c.quer {
			t.Errorf("%s: %v", c.nome, obtido)
		}
	}
}

// Regra (#158): anual são 2 ou 3 cobranças, de 330 a 400 dias uma da outra.
func TestEhAnual(t *testing.T) {
	t.Parallel()
	base := testeapoio.Dia(2026, 10, 1)
	casos := []struct {
		nome  string
		datas []tipos.Data
		quer  bool
	}{
		{"duas a 365 dias", []tipos.Data{base, base.SomarDias(-365)}, true},
		{"duas a 330", []tipos.Data{base, base.SomarDias(-330)}, true},
		{"duas a 329", []tipos.Data{base, base.SomarDias(-329)}, false},
		{"duas a 400", []tipos.Data{base, base.SomarDias(-400)}, true},
		{"duas a 401", []tipos.Data{base, base.SomarDias(-401)}, false},
		{"três", []tipos.Data{base, base.SomarDias(-365), base.SomarDias(-730)}, true},
		{"quatro", []tipos.Data{base, base.SomarDias(-365), base.SomarDias(-730), base.SomarDias(-1095)}, false},
		{"uma", []tipos.Data{base}, false},
	}
	for _, c := range casos {
		if obtido := ehAnual(c.datas); obtido != c.quer {
			t.Errorf("%s: %v", c.nome, obtido)
		}
	}
}

// Regra: a mediana, e no número par a média dos dois do meio (com a divisão do decimal).
func TestMediana(t *testing.T) {
	t.Parallel()
	v := testeapoio.Valor
	casos := []struct {
		valores []tipos.Decimal
		quer    string
	}{
		{[]tipos.Decimal{v("30.0"), v("10.0"), v("20.0")}, "20.0"},
		{[]tipos.Decimal{v("10.0"), v("20.0")}, "15.0"},
		{[]tipos.Decimal{v("39.9")}, "39.9"},
		{[]tipos.Decimal{v("1"), v("2")}, "1.5"},
	}
	for _, c := range casos {
		if obtido := mediana(c.valores).String(); obtido != c.quer {
			t.Errorf("mediana(%v) = %s, quer %s", c.valores, obtido, c.quer)
		}
	}
}

// Regra: o dia habitual dentro do mês; o 31 vira o último dia dos meses mais curtos (com o bissexto).
func TestNoDia(t *testing.T) {
	t.Parallel()
	casos := []struct {
		inicio tipos.Data
		dia    int
		quer   string
	}{
		{testeapoio.Dia(2026, 11, 1), 31, "2026-11-30"},
		{testeapoio.Dia(2027, 2, 1), 30, "2027-02-28"},
		{testeapoio.Dia(2028, 2, 1), 31, "2028-02-29"},
		{testeapoio.Dia(2026, 12, 1), 29, "2026-12-29"},
		{testeapoio.Dia(2027, 1, 1), 1, "2027-01-01"},
	}
	for _, c := range casos {
		if obtido := noDia(c.inicio, c.dia).String(); obtido != c.quer {
			t.Errorf("noDia(%s, %d) = %s", c.inicio, c.dia, obtido)
		}
	}
}

// Regra (#158): compra parcelada no cartão que veio sem os campos de parcela, com as bordas de palavra do .NET.
func TestParcelaNoTexto(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{
		"LOJA X PARC 3/10":      true,
		"LOJA X 03/10":          true,
		"LOJA X 3 / 10":         true,
		"LOJA X parcela 2":      true,
		"LOJA X 3 / 10":         true, // espaço não separável
		"PARCELADO":             false,
		"LOJA 123/45":           false,
		"LOJA 12/345":           false,
		"ÁPARC":                 false, // o Á é letra para o \b do .NET
		"ASSINATURA STREAMFLIX": false,
		"":                      false,
	}
	for entrada, quer := range casos {
		if obtido := parcelaNoTexto(entrada); obtido != quer {
			t.Errorf("parcelaNoTexto(%q) = %v", entrada, obtido)
		}
	}
}

// Regra (#158): o débito automático pelo texto ("DEB AUT", "DEBITO AUTOMATICO", "DEB.AUT.").
func TestEhDebitoAutomatico(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{
		"DEB AUT ENEL": true, "débito automático Sabesp": true, "DEB.AUT.CLARO": true, "PIX ENEL": false,
		"PREDEB AUT": false, "DEBAUT": false,
	}
	for entrada, quer := range casos {
		if obtido := ehDebitoAutomatico(entrada); obtido != quer {
			t.Errorf("ehDebitoAutomatico(%q) = %v", entrada, obtido)
		}
	}
}

// Regra: o termo da busca em Transações é a palavra como está gravada (a que contém a mais longa da chave), sem a
// pontuação das pontas; sem ela, a descrição cortada em 100.
func TestTermoBusca(t *testing.T) {
	t.Parallel()
	estab := "Netflix.com,"
	casos := []struct {
		nome, chave, descricao string
		estab                  *string
		quer                   string
	}{
		{"pelo estabelecimento", "NETFLIX COM", "PAG*NETFLIX", &estab, "Netflix.com"},
		{"pela descrição, sem a pontuação das duas pontas", "ACADEMIA FORMA", "Pgto -academia- forma", nil, "academia"},
		{"sem palavra", "F I A P", "BOLETO F I A P", nil, "BOLETO F I A P"},
		{"cortada em 100", "", strings.Repeat("x", 120), nil, strings.Repeat("x", 100)},
	}
	for _, c := range casos {
		if obtido := termoBusca(c.chave, c.estab, c.descricao); obtido != c.quer {
			t.Errorf("%s: %q", c.nome, obtido)
		}
	}
}

// Regra (#205): valor perto é a até 15% do maior; parecida é valor perto (ou na faixa da variável), dia a até 3 e a
// mesma conta (a conta a pagar sem conta serve para qualquer uma); certeza é a mesma conta, sem texto da cobrança.
func TestParecidas(t *testing.T) {
	t.Parallel()
	v := testeapoio.Valor
	if !valorPerto(v("100.0"), v("85.0")) || valorPerto(v("100.0"), v("84.9")) || valorPerto(v("0"), v("0")) {
		t.Error("valorPerto")
	}
	s := SugestaoDto{Valor: v("100.0"), ContaId: 1, DiaHabitual: 30, Recorrencia: 1, ProximoVencimento: dia(30)}
	casos := []struct {
		nome            string
		c               cadastrada
		parecida, certa bool
	}{
		{"mesma conta e dia", cadastrada{valor: v("95.0"), contaId: ptr(int64(1)), proximoVencimento: testeapoio.Dia(2026, 11, 1), recorrencia: 1}, true, true},
		{"sem conta", cadastrada{valor: v("100.0"), proximoVencimento: dia(30), recorrencia: 1}, true, false},
		{"outra conta", cadastrada{valor: v("100.0"), contaId: ptr(int64(2)), proximoVencimento: dia(30), recorrencia: 1}, false, false},
		{"dia longe", cadastrada{valor: v("100.0"), contaId: ptr(int64(1)), proximoVencimento: dia(26), recorrencia: 1}, false, true},
		{"anual com mensal", cadastrada{valor: v("100.0"), contaId: ptr(int64(1)), proximoVencimento: dia(30), recorrencia: 2}, false, true},
		{"com texto da cobrança", cadastrada{valor: v("100.0"), contaId: ptr(int64(1)), textoCobranca: ptr("X"), proximoVencimento: dia(30), recorrencia: 1}, true, false},
		{"valor longe", cadastrada{valor: v("80.0"), contaId: ptr(int64(1)), proximoVencimento: dia(30), recorrencia: 1}, false, true},
	}
	for _, c := range casos {
		if parecida(s, c.c) != c.parecida || certeza(s, c.c) != c.certa {
			t.Errorf("%s: parecida %v, certeza %v", c.nome, parecida(s, c.c), certeza(s, c.c))
		}
	}
	variavel := s
	variavel.ValorVariavel, variavel.ValorMinimo, variavel.ValorMaximo = true, v("50.0"), v("150.0")
	if !parecida(variavel, cadastrada{valor: v("60.0"), proximoVencimento: dia(30), recorrencia: 1}) {
		t.Error("na faixa da variável")
	}
}

// Regra (#205): a sugestão com uma só conta a pagar certa some; com dúvida, fica com o "parece ser" da melhor.
func TestSemAsParecidas(t *testing.T) {
	t.Parallel()
	v := testeapoio.Valor
	certa := cadastrada{id: 1, descricao: "Internet", ativa: true, valor: v("100.0"), contaId: ptr(int64(1)),
		proximoVencimento: dia(10), recorrencia: 1}
	semConta := cadastrada{id: 2, descricao: "Internet antiga", ativa: true, valor: v("100.0"), proximoVencimento: dia(10), recorrencia: 1}
	ligada := cadastrada{id: 3, descricao: "Já ligada", ativa: true, chaveExtrato: ptr("X"), valor: v("100.0"),
		contaId: ptr(int64(1)), proximoVencimento: dia(10), recorrencia: 1}
	s := SugestaoDto{Chave: "NET", Valor: v("100.0"), ContaId: 1, DiaHabitual: 10, Recorrencia: 1}

	if r := semAsParecidas([]SugestaoDto{s}, []cadastrada{certa, ligada}); len(r) != 0 {
		t.Errorf("a certa: %+v", r)
	}
	r := semAsParecidas([]SugestaoDto{s}, []cadastrada{semConta, certa})
	if len(r) != 1 || r[0].PareceSerId == nil || *r[0].PareceSerId != 1 || *r[0].PareceSerNome != "Internet" {
		t.Errorf("duas candidatas: %+v", r)
	}
	r = semAsParecidas([]SugestaoDto{s}, []cadastrada{semConta})
	if len(r) != 1 || r[0].PareceSerId == nil || *r[0].PareceSerId != 2 {
		t.Errorf("sem conta: %+v", r)
	}
	if r := semAsParecidas([]SugestaoDto{s}, nil); len(r) != 1 || r[0].PareceSerId != nil {
		t.Errorf("sem cadastradas: %+v", r)
	}
}

// Regra: o corte do C# em chars UTF-16, e a primeira das palavras mais longas.
func TestCortarEMaisLonga(t *testing.T) {
	t.Parallel()
	if cortar("abc", 2) != "ab" || cortar("ab", 2) != "ab" || cortar("😀b", 2) != "😀" {
		t.Error("cortar")
	}
	if p, ok := maisLonga([]string{"ABC", "DEFG", "HIJK"}); !ok || p != "DEFG" {
		t.Errorf("maisLonga: %q", p)
	}
	if _, ok := maisLonga(nil); ok {
		t.Error("maisLonga vazia")
	}
}
