package holeritepdf

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

type tiposDecimal = tipos.Decimal

func decimalDe(s string) tipos.Decimal { return tipos.DeDecimal(decimal.RequireFromString(s)) }

func dataDe(s string) tipos.Data {
	d, err := tipos.LerData(s)
	if err != nil {
		panic(err)
	}
	return d
}

func conferirMensagem(t *testing.T, r testeapoio.Resposta, mensagem string) {
	t.Helper()
	var corpo struct{ Mensagem string }
	if err := json.Unmarshal(r.Corpo, &corpo); err != nil || corpo.Mensagem != mensagem {
		t.Errorf("mensagem: %s, quero %q", r.Corpo, mensagem)
	}
}

// Os nomes comuns de cada tipo, dos exemplos da #228 e dos sistemas de folha mais usados.
func TestClassificar(t *testing.T) {
	casos := []struct {
		descricao string
		provento  bool
		tipo      int64
		conhecida bool
	}{
		{"SALARIO", true, holerite.TipoSalario, true},
		{"Salário Mensal", true, holerite.TipoSalario, true},
		{"Horas Normais", true, holerite.TipoSalario, true},
		{"Sal. Base", true, holerite.TipoSalario, true},
		{"Pró-labore", true, holerite.TipoSalario, true},
		{"Salário Família", true, holerite.OutroProvento, true},
		{"H.E. 50%", true, holerite.HoraExtra, true},
		{"Hora Extra 50%", true, holerite.HoraExtra, true},
		{"HORAS EXTRAS 100%", true, holerite.HoraExtra, true},
		{"DSR s/ HE", true, holerite.TipoDSR, true},
		{"DSR S/ HORAS EXTRAS", true, holerite.TipoDSR, true},
		{"Repouso Remunerado", true, holerite.TipoDSR, true},
		{"Reflexo DSR H.E.", true, holerite.TipoDSR, true},
		{"Adicional Noturno 20%", true, holerite.AdicionalNoturno, true},
		{"Adic. Noturno", true, holerite.AdicionalNoturno, true},
		{"13º Salário 2ª Parcela", true, holerite.OutroProvento, true},
		{"Férias", true, holerite.OutroProvento, true},
		{"1/3 Férias", true, holerite.OutroProvento, true},
		{"Gratificação", true, holerite.OutroProvento, true},
		{"PLR", true, holerite.OutroProvento, true},
		{"Comissões", true, holerite.OutroProvento, true},
		{"Prêmio por desempenho", true, holerite.OutroProvento, true},
		{"Ajuda de custo", true, holerite.OutroProvento, true},
		{"Auxílio home office", true, holerite.OutroProvento, false},
		{"INSS", false, holerite.TipoInss, true},
		{"I.N.S.S.", false, holerite.TipoInss, true},
		{"Contrib. Previdenciária", false, holerite.TipoInss, true},
		{"INSS s/ 13º Salário", false, holerite.TipoInss, true},
		{"IRRF", false, holerite.TipoIrrf, true},
		{"I.R.R.F.", false, holerite.TipoIrrf, true},
		{"Imposto de Renda", false, holerite.TipoIrrf, true},
		{"IRRF s/ Férias", false, holerite.TipoIrrf, true},
		{"IR Fonte", false, holerite.TipoIrrf, true},
		{"Vale Transporte", false, holerite.ValeTransporte, true},
		{"Desc. VT", false, holerite.ValeTransporte, true},
		{"Vale Refeição", false, holerite.ValeAlimentacao, true},
		{"Vale Alimentação", false, holerite.ValeAlimentacao, true},
		{"Assist. Médica", false, holerite.PlanoDeSaude, true},
		{"Plano de Saúde", false, holerite.PlanoDeSaude, true},
		{"Plano Odontológico", false, holerite.PlanoDeSaude, true},
		{"Coparticipação Unimed", false, holerite.PlanoDeSaude, true},
		{"Pensão Alimentícia", false, holerite.PensaoAlimenticia, true},
		{"Pensão Judicial", false, holerite.PensaoAlimenticia, true},
		{"Adiantamento", false, holerite.TipoAdiantamento, true},
		{"Adto. Salarial", false, holerite.TipoAdiantamento, true},
		{"ADIANTAMENTO QUINZENAL", false, holerite.TipoAdiantamento, true},
		{"Adiantamento 13º", false, holerite.OutroDesconto, true},
		{"Faltas", false, holerite.OutroDesconto, true},
		{"Previdência Privada", false, holerite.OutroDesconto, true},
		{"Empréstimo consignado", false, holerite.OutroDesconto, true},
		{"Seguro de Vida", false, holerite.OutroDesconto, false},
		{"Contrib. Sindical", false, holerite.OutroDesconto, false},
		// O lado conta: "Faltas" no provento não é desconto, e "INSS" no provento (reembolso) não é o INSS.
		{"INSS", true, holerite.OutroProvento, false},
		{"Salário", false, holerite.OutroDesconto, false},
		{"Desconto DSR", false, holerite.OutroDesconto, false},
	}
	for _, c := range casos {
		tipo, conhecida := Classificar(c.descricao, c.provento)
		if tipo != c.tipo || conhecida != c.conhecida {
			t.Errorf("Classificar(%q, %t) = %d %t, quero %d %t", c.descricao, c.provento, tipo, conhecida, c.tipo, c.conhecida)
		}
	}
}

// Cada expressão do dicionário já está como o texto.ParaPalavras deixa: senão, ela nunca casa.
func TestDicionarioNormalizado(t *testing.T) {
	for _, r := range dicionario {
		if provento, existe := ehProvento(r.tipo); !existe || provento != r.provento {
			t.Errorf("regra do tipo %d no lado errado", r.tipo)
		}
		for _, termo := range r.termos {
			if texto.ParaPalavras(termo) != termo {
				t.Errorf("%q não está normalizado: %q", termo, texto.ParaPalavras(termo))
			}
		}
	}
}

func TestEhProvento(t *testing.T) {
	casos := []struct {
		tipo             int64
		provento, existe bool
	}{
		{holerite.TipoSalario, true, true}, {holerite.TipoDSR, true, true}, {holerite.OutroProvento, true, true},
		{holerite.ValeTransporte, false, true}, {holerite.TipoAdiantamento, false, true}, {holerite.TipoInss, false, true},
		{5, false, false}, {23, false, false}, {-1, false, false},
	}
	for _, c := range casos {
		if provento, existe := ehProvento(c.tipo); provento != c.provento || existe != c.existe {
			t.Errorf("ehProvento(%d) = %t %t", c.tipo, provento, existe)
		}
	}
}
