package relatorios

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Regra: ProximoVencimentoApos (#69): mensal é o AddMonths(1) do C#, anual é o AddYears(1), e a única não tem próximo.
// O AddMonths do .NET fica no último dia do mês de chegada (31/01 + 1 mês = 28/02); o AddDate do Go iria para 03/03.
func TestProximoVencimentoApos(t *testing.T) {
	casos := []struct {
		nome        string
		vencimento  tipos.Data
		recorrencia int64
		esperado    string // "" = sem próximo
	}{
		{"mensal, dia comum", dia(2026, 10, 15), RecorrenciaMensal, "2026-11-15"},
		{"mensal, dia 31 para fevereiro", dia(2026, 1, 31), RecorrenciaMensal, "2026-02-28"},
		{"mensal, dia 31 para fevereiro bissexto", dia(2024, 1, 31), RecorrenciaMensal, "2024-02-29"},
		{"mensal, dia 30 para fevereiro", dia(2026, 1, 30), RecorrenciaMensal, "2026-02-28"},
		{"mensal, dia 29 para fevereiro", dia(2026, 1, 29), RecorrenciaMensal, "2026-02-28"},
		{"mensal, dia 29 para fevereiro bissexto cabe", dia(2024, 1, 29), RecorrenciaMensal, "2024-02-29"},
		{"mensal, dia 31 para mês de 30 dias", dia(2026, 3, 31), RecorrenciaMensal, "2026-04-30"},
		{"mensal, dia 31 para mês de 31 dias", dia(2026, 7, 31), RecorrenciaMensal, "2026-08-31"},
		{"mensal, dia 30 em mês de 30 dias", dia(2026, 4, 30), RecorrenciaMensal, "2026-05-30"},
		{"mensal, virada de ano", dia(2026, 12, 15), RecorrenciaMensal, "2027-01-15"},
		{"mensal, virada de ano no dia 31", dia(2026, 12, 31), RecorrenciaMensal, "2027-01-31"},
		{"mensal, a partir do dia 28 não volta ao 31", dia(2026, 2, 28), RecorrenciaMensal, "2026-03-28"},
		{"anual, dia comum", dia(2026, 10, 15), RecorrenciaAnual, "2027-10-15"},
		{"anual, 29/02 vira 28/02", dia(2024, 2, 29), RecorrenciaAnual, "2025-02-28"},
		{"anual, 29/02 de um bissexto para outro", dia(2024, 2, 29), RecorrenciaAnual, "2025-02-28"},
		{"anual, 28/02 antes de um bissexto fica 28", dia(2023, 2, 28), RecorrenciaAnual, "2024-02-28"},
		{"única não tem próximo", dia(2026, 10, 15), RecorrenciaUnica, ""},
		{"recorrência desconhecida não tem próximo", dia(2026, 10, 15), 3, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			proximo, ok := ProximoVencimentoApos(c.vencimento, c.recorrencia)
			if c.esperado == "" {
				if ok {
					t.Errorf("ProximoVencimentoApos(%s, %d) = %s, esperado nenhum", c.vencimento, c.recorrencia, proximo)
				}
				return
			}
			if !ok || proximo.String() != c.esperado {
				t.Errorf("ProximoVencimentoApos(%s, %d) = %s, %v; esperado %s", c.vencimento, c.recorrencia, proximo, ok, c.esperado)
			}
		})
	}
}

// Cada pagamento avança a partir do vencimento anterior, e não do dia original: a conta de 31/01 vence 28/02, e dali em
// diante no dia 28. Comportamento do C# (#447, "ao pagar, o vencimento anda mês a mês"), fixado como está.
func TestProximoVencimentoAposEmCadeiaPerdeODia31(t *testing.T) {
	v := dia(2026, 1, 31)
	var datas []string
	for range 4 {
		v, _ = ProximoVencimentoApos(v, RecorrenciaMensal)
		datas = append(datas, v.String())
	}
	if esperado := "2026-02-28,2026-03-28,2026-04-28,2026-05-28"; strings.Join(datas, ",") != esperado {
		t.Errorf("em cadeia: %s, esperado %s", strings.Join(datas, ","), esperado)
	}
}

// Regra: PagamentosAte (#69): quantos pagamentos faltam até a data limite, contando o próximo; sem data limite, ou com
// recorrência única, não há número; o laço para em 1200.
func TestPagamentosAte(t *testing.T) {
	limite := func(d tipos.Data) *tipos.Data { return &d }
	casos := []struct {
		nome        string
		proximo     tipos.Data
		recorrencia int64
		limite      *tipos.Data
		esperado    int
		ok          bool
	}{
		{"sem data limite", dia(2026, 10, 15), RecorrenciaMensal, nil, 0, false},
		{"única não conta", dia(2026, 10, 15), RecorrenciaUnica, limite(dia(2027, 10, 15)), 0, false},
		{"o limite é o próprio vencimento: falta um", dia(2026, 10, 15), RecorrenciaMensal, limite(dia(2026, 10, 15)), 1, true},
		{"o limite é um dia antes do vencimento: nenhum", dia(2026, 10, 15), RecorrenciaMensal, limite(dia(2026, 10, 14)), 0, true},
		{"o limite já passou muito", dia(2026, 10, 15), RecorrenciaMensal, limite(dia(2020, 1, 1)), 0, true},
		{"o limite é um mês depois: dois", dia(2026, 10, 15), RecorrenciaMensal, limite(dia(2026, 11, 15)), 2, true},
		{"um dia antes do segundo vencimento: um", dia(2026, 10, 15), RecorrenciaMensal, limite(dia(2026, 11, 14)), 1, true},
		{"mensal por um ano", dia(2026, 1, 15), RecorrenciaMensal, limite(dia(2026, 12, 15)), 12, true},
		{"anual", dia(2026, 10, 15), RecorrenciaAnual, limite(dia(2029, 10, 15)), 4, true},
		{"anual, um dia antes do último", dia(2026, 10, 15), RecorrenciaAnual, limite(dia(2029, 10, 14)), 3, true},
		// O dia 31 vira 28 e fica: 31/10, 30/11, 30/12, 30/01. Com o dia 31 de volta, 31/01 passaria de 30/01 e seriam 3.
		{"o dia 31 que vira 30 ainda cabe", dia(2026, 10, 31), RecorrenciaMensal, limite(dia(2027, 1, 30)), 4, true},
		{"fevereiro bissexto no meio", dia(2024, 1, 31), RecorrenciaMensal, limite(dia(2024, 3, 29)), 3, true},
		{"anual a partir de 29/02", dia(2024, 2, 29), RecorrenciaAnual, limite(dia(2027, 3, 1)), 4, true},
		{"recorrência desconhecida conta só o próximo", dia(2026, 10, 15), 3, limite(dia(2030, 1, 1)), 1, true},
		{"para em 1200", dia(2026, 1, 1), RecorrenciaMensal, limite(dia(9999, 12, 31)), 1200, true},
		{"exatamente 1200 meses", dia(2026, 1, 1), RecorrenciaMensal, limite(dia(2125, 12, 1)), 1200, true},
		{"1199 meses", dia(2026, 1, 1), RecorrenciaMensal, limite(dia(2125, 11, 1)), 1199, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			n, ok := PagamentosAte(c.proximo, c.recorrencia, c.limite)
			if n != c.esperado || ok != c.ok {
				t.Errorf("PagamentosAte = %d, %v; esperado %d, %v", n, ok, c.esperado, c.ok)
			}
		})
	}
}

// prepararContas grava as contas a pagar dos testes de ContasAgendadas. Hoje, em Brasília, é 22/10/2026 às 23:30 (em UTC já
// é 23/10): a conta N vence "amanhã em UTC", e quem confundir os fusos a trata como a que vence hoje.
func prepararContas(t *testing.T) (*Servico, map[string]int64, testeapoio.Categoria, testeapoio.Conta) {
	t.Helper()
	relogio := testeapoio.AgoraEmBrasilia(2026, 10, 22, 23, 30)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	moradia := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Moradia" })
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta principal" })
	ids := map[string]int64{}
	nova := func(nome string, vencimento tipos.Data, ajustes ...func(*testeapoio.ContaAgendada)) {
		c := fab.ContaAgendada(append([]func(*testeapoio.ContaAgendada){func(c *testeapoio.ContaAgendada) {
			c.Descricao = nome
			c.ProximoVencimento = vencimento
		}}, ajustes...)...)
		ids[nome] = c.Id
	}
	nova("A", dia(2026, 10, 25), func(c *testeapoio.ContaAgendada) {
		c.Valor = dec("1500.00")
		c.CategoriaId = &moradia.Id
		c.ContaId = &conta.Id
		c.UltimoPagamentoEm = new(dia(2026, 9, 25))
		c.TextoCobranca = new("ALUGUEL")
		c.Observacao = new("dia 25")
	})
	nova("B", dia(2026, 10, 20))
	nova("C", dia(2026, 10, 23), func(c *testeapoio.ContaAgendada) { c.FormaCobranca = 3 }) // cartão de crédito
	nova("D", dia(2026, 10, 23), func(c *testeapoio.ContaAgendada) { c.Ativa = false })
	nova("E", dia(2026, 12, 1))
	nova("F", dia(2026, 10, 27))
	nova("G", dia(2026, 10, 28))
	nova("H", dia(2026, 10, 25)) // empata com a A: a ordem do Id
	nova("I", dia(2026, 10, 31), func(c *testeapoio.ContaAgendada) { c.Ativa = false; c.DataLimite = new(dia(2026, 9, 30)) })
	nova("J", dia(2026, 12, 31), func(c *testeapoio.ContaAgendada) { c.Recorrencia = 0; c.DataLimite = new(dia(2026, 12, 31)) })
	nova("K", dia(2026, 10, 31), func(c *testeapoio.ContaAgendada) { c.DataLimite = new(dia(2027, 1, 30)) })
	nova("L", dia(2026, 11, 15), func(c *testeapoio.ContaAgendada) { c.Ativa = false; c.DataLimite = new(dia(2026, 12, 31)) })
	nova("M", dia(2024, 2, 29), func(c *testeapoio.ContaAgendada) { c.Recorrencia = 2; c.DataLimite = new(dia(2027, 3, 1)) })
	nova("N", dia(2026, 10, 23))
	return NovoServico(db, relogio), ids, moradia, conta
}

func nomesNaOrdem(lista []ContaAgendadaDto) string {
	nomes := make([]string, len(lista))
	for i, c := range lista {
		nomes[i] = c.Descricao
	}
	return strings.Join(nomes, ",")
}

// Regras de ContasAgendadas: sem proximosDias vêm todas (até as inativas), por vencimento e depois por Id; com ele, só as
// ativas que vencem até hoje + dias (as atrasadas também), sem as cobradas no cartão de crédito (#158).
func TestContasAgendadasFiltroEOrdem(t *testing.T) {
	t.Parallel()
	s, _, _, _ := prepararContas(t)
	dias := func(n int) *int { return &n }
	casos := []struct {
		nome     string
		dias     *int
		esperado string
	}{
		{"todas, inclusive inativas e encerradas", nil, "M,B,C,D,N,A,H,F,G,I,K,L,E,J"},
		{"os próximos 5 dias: até o dia 27 (inclusive), sem cartão e sem inativas", dias(5), "M,B,N,A,H,F"},
		{"os próximos 6 dias: o dia 28 entra", dias(6), "M,B,N,A,H,F,G"},
		{"hoje: só as atrasadas, e a que vence amanhã em UTC não entra", dias(0), "M,B"},
		{"amanhã: a de dia 23 entra (e a de cartão e a inativa não)", dias(1), "M,B,N"},
		{"dias negativos olham para trás", dias(-3), "M"},
		{"os próximos 70 dias: tudo o que está ativo, sem cartão", dias(70), "M,B,N,A,H,F,G,K,E,J"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtidas, err := s.ContasAgendadas(t.Context(), c.dias)
			if err != nil {
				t.Fatal(err)
			}
			if nomes := nomesNaOrdem(obtidas); nomes != c.esperado {
				t.Errorf("contas = %s, esperado %s", nomes, c.esperado)
			}
		})
	}
}

// Dias para vencer, pagamentos restantes e encerrada, calculados com hoje em Brasília (22/10, mesmo com 23/10 em UTC).
func TestContasAgendadasCalculos(t *testing.T) {
	t.Parallel()
	s, _, _, _ := prepararContas(t)
	obtidas, err := s.ContasAgendadas(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	type esperado struct {
		dias      int
		restantes string // "" = null
		encerrada bool
	}
	casos := map[string]esperado{
		"A": {3, "", false},
		"B": {-2, "", false},
		"N": {1, "", false}, // vence 23/10: amanhã em Brasília; em UTC seria hoje (0)
		"I": {9, "", true},  // inativa, com data limite 30/09 e vencimento 31/10: passou dela
		"J": {70, "", false},
		"K": {9, "4", false}, // 31/10, 30/11, 30/12 e 30/01 (o dia 31 não volta)
		"L": {24, "", false}, // inativa, mas o vencimento (15/11) não passou da data limite (31/12)
		"M": {-966, "4", false},
	}
	porNome := map[string]ContaAgendadaDto{}
	for _, c := range obtidas {
		porNome[c.Descricao] = c
	}
	for nome, e := range casos {
		c := porNome[nome]
		restantes := ""
		if c.PagamentosRestantes != nil {
			restantes = strconv.Itoa(*c.PagamentosRestantes)
		}
		if c.DiasParaVencer != e.dias || restantes != e.restantes || c.Encerrada != e.encerrada {
			t.Errorf("conta %s: dias %d, restantes %q, encerrada %v; esperado %d, %q, %v",
				nome, c.DiasParaVencer, restantes, c.Encerrada, e.dias, e.restantes, e.encerrada)
		}
	}
}

// O JSON é o do record ContaAgendadaDto do C#, na ordem dos campos, com os nomes da categoria e da conta (LEFT JOIN) e null
// onde não há.
func TestContaAgendadaDtoJSON(t *testing.T) {
	t.Parallel()
	s, ids, moradia, conta := prepararContas(t)
	obtidas, err := s.ContasAgendadas(t.Context(), new(3))
	if err != nil {
		t.Fatal(err)
	}
	var a ContaAgendadaDto
	for _, c := range obtidas {
		if c.Id == ids["A"] {
			a = c
		}
	}
	esperado := `{"id":` + strconv.FormatInt(ids["A"], 10) + `,"descricao":"A","valor":1500.0,"proximoVencimento":"2026-10-25","recorrencia":1,` +
		`"categoriaId":` + strconv.FormatInt(moradia.Id, 10) + `,"categoriaNome":"Moradia","contaId":` + strconv.FormatInt(conta.Id, 10) +
		`,"contaNome":"Conta principal","ativa":true,"ultimoPagamentoEm":"2026-09-25","diasParaVencer":3,"dataLimite":null,` +
		`"pagamentosRestantes":null,"encerrada":false,"formaCobranca":1,"textoCobranca":"ALUGUEL","observacao":"dia 25"}`
	if obtido := comoJSON(t, a); obtido != esperado {
		t.Errorf("JSON = %s\nesperado %s", obtido, esperado)
	}
	b := obtidas[1] // a B: sem categoria, conta, texto nem observação
	if !strings.Contains(comoJSON(t, b), `"categoriaId":null,"categoriaNome":null,"contaId":null,"contaNome":null`) {
		t.Errorf("a conta sem categoria e sem conta devia ter null nos quatro campos: %s", comoJSON(t, b))
	}
}

// Sem nenhuma conta: a lista vazia ([]), e não null.
func TestContasAgendadasVazia(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	s := NovoServico(db, testeapoio.RelogioDosTestes())
	for _, dias := range []*int{nil, new(30)} {
		obtidas, err := s.ContasAgendadas(t.Context(), dias)
		if err != nil {
			t.Fatal(err)
		}
		if obtidas == nil || comoJSON(t, obtidas) != "[]" {
			t.Errorf("%s, esperado []", comoJSON(t, obtidas))
		}
	}
}

// Número de dias que leva a data para fora do ano 1 a 9999: o C# lança ArgumentOutOfRangeException (500). Aqui, erro.
func TestContasAgendadasComDiasForaDoIntervalo(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	s := NovoServico(db, testeapoio.HojeEmBrasilia(2026, 10, 22))
	casos := []struct {
		nome string
		dias int
		erro bool
	}{
		{"muito para a frente", 3_000_000, true},
		{"muito para trás", -800_000, true},
		// Hoje é o dia 739910 (DayNumber): o 9999-12-31 é o 3652058, e o 0001-01-01 é o 0.
		{"último dia do ano 9999", 2_912_148, false},
		{"um dia depois do ano 9999", 2_912_149, true},
		{"primeiro dia do ano 1", -739_910, false},
		{"um dia antes do ano 1", -739_911, true},
		{"o maior int", math.MaxInt, true},
		{"o menor int", math.MinInt, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := s.ContasAgendadas(t.Context(), &c.dias)
			if c.erro != errors.Is(err, ErroDataForaDoIntervalo) {
				t.Errorf("com %d dias: erro %v, esperado erro de intervalo = %v", c.dias, err, c.erro)
			}
			if !c.erro && err != nil {
				t.Errorf("com %d dias: erro inesperado %v", c.dias, err)
			}
		})
	}
}
