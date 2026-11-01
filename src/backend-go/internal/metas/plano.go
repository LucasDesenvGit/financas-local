package metas

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/faturas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parcelas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

const (
	// MinimoMesesHistorico é quantos meses completos o plano precisa para sair.
	MinimoMesesHistorico = 3
	// horizonteMeses é até onde o caixa é estimado (a data provável de uma meta pode estar longe).
	horizonteMeses = 120
	// chaveMargem é o parâmetro da margem de segurança, que a tela aplica ao plano.
	chaveMargem = "Metas.MargemSeguranca"
	tipoCartao  = 3
)

// diasPorMes é o 30.44m do C#.
var diasPorMes = mustDecimal("30.44")

// tiposCaixa são as contas de dinheiro: corrente, poupança, dinheiro e outras.
var tiposCaixa = []int64{1, 2, 5, 6}

func mustDecimal(s string) tipos.Decimal {
	d, err := tipos.NovoDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

// MesCaixaDto é um mês do caixa (#149).
type MesCaixaDto struct {
	Ano      int           `json:"ano"`
	Mes      int           `json:"mes"`
	Receitas tipos.Decimal `json:"receitas"`
	Despesas tipos.Decimal `json:"despesas"`
	Faturas  tipos.Decimal `json:"faturas"`
	Parcelas tipos.Decimal `json:"parcelas"`
	Sobra    tipos.Decimal `json:"sobra"`
}

// PlanoMetasDto é o que o plano das metas precisa (#90, #149, ADR 0035).
type PlanoMetasDto struct {
	Disponivel        bool          `json:"disponivel"`
	MesesConsiderados int           `json:"mesesConsiderados"`
	Ano               int           `json:"ano"`
	Mes               int           `json:"mes"`
	MargemPercentual  int           `json:"margemPercentual"`
	SaldoContas       tipos.Decimal `json:"saldoContas"`
	FaturaAPagar      tipos.Decimal `json:"faturaAPagar"`
	SaldoFimDoMes     tipos.Decimal `json:"saldoFimDoMes"`
	MelhorMesRecente  tipos.Decimal `json:"melhorMesRecente"`
	Meses             []MesCaixaDto `json:"meses"`
}

func (s *Servico) plano(w http.ResponseWriter, r *http.Request) {
	p, err := s.Planejar(r.Context())
	if err != nil {
		s.erroInterno(w, r, "plano", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, p)
}

type contaDoPlano struct {
	id, tipo                     int64
	incluirNoSaldo               bool
	diaFechamento, diaVencimento *int64
}

// faturaDoMes é a soma das faturas dos cartões que vencem num mês, e a parte delas que é parcela.
type faturaDoMes struct{ total, parcelas tipos.Decimal }

// Planejar é o PlanejarAsync do C#: o dinheiro que entra e sai de cada mês, que é o que dá para guardar.
//   - receitas e despesas das contas de dinheiro, sem transferências;
//   - menos as faturas de cartão que vencem no mês (a compra no cartão sai do caixa quando a fatura vence);
//   - mês atual: o que já entrou e saiu, mais a média do que falta (receitas: o maior entre o recebido e a média);
//   - meses seguintes: as médias dos últimos meses completos (até 6) e a fatura estimada de cada mês.
//
// Fatura estimada: já fechada, vale o total do banco; aberta, o lançado mais a média diária de compras à vista nos dias
// que faltam; futura, a média de compras à vista. Nas três, mais as parcelas já contratadas que caem nela.
func (s *Servico) Planejar(ctx context.Context) (PlanoMetasDto, error) {
	hoje := relogio.Hoje(s.relogio)
	mesAtual := tipos.NovaData(hoje.Ano(), hoje.Mes(), 1)
	margem, err := s.parametros.ObterInteiro(ctx, chaveMargem)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	inicioHistorico, mesesConsiderados, err := s.relatorios.JanelaHistorico(ctx)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	plano := PlanoMetasDto{MesesConsiderados: mesesConsiderados, Ano: mesAtual.Ano(), Mes: int(mesAtual.Mes()),
		MargemPercentual: margem, SaldoContas: tipos.Zero, FaturaAPagar: tipos.Zero, SaldoFimDoMes: tipos.Zero,
		MelhorMesRecente: tipos.Zero, Meses: []MesCaixaDto{}}
	if mesesConsiderados < MinimoMesesHistorico {
		return plano, nil
	}
	fimHistorico := mesAtual.SomarDias(-1)
	meses := tipos.DecimalDeInteiro(int64(mesesConsiderados))

	contasAtivas, err := s.lerContas(ctx)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	var idsCaixa []int64
	var cartoes []contaDoPlano
	var idsCartoes []any
	for _, c := range contasAtivas {
		if c.incluirNoSaldo && slices.Contains(tiposCaixa, c.tipo) {
			idsCaixa = append(idsCaixa, c.id)
		}
		if c.tipo == tipoCartao {
			cartoes = append(cartoes, c)
			idsCartoes = append(idsCartoes, c.id)
		}
	}

	// Contas de dinheiro: receitas e despesas como nos relatórios (sem transferências nem "ignorar nos relatórios").
	relevantes, err := s.relatorios.Relevantes(ctx, inicioHistorico, hoje)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	var caixa []movimento
	for _, t := range relevantes {
		if slices.Contains(idsCaixa, t.ContaId) {
			caixa = append(caixa, movimento{t.Data, t.Valor, t.Tipo})
		}
	}
	mediaReceitas, err := tipos.Dividir(receitas(caixa, inicioHistorico, fimHistorico), meses)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	mediaDespesas, err := tipos.Dividir(despesas(caixa, inicioHistorico, fimHistorico), meses)
	if err != nil {
		return PlanoMetasDto{}, err
	}

	// Cartões: tudo o que entra na fatura (como a "fatura do mês", #144), inclusive o marcado para ignorar nos
	// relatórios e as parcelas que o banco já mandou com data futura. Janela larga: a parcela já ligada a uma fatura
	// de cada compra decide em que fatura caem as outras (#167).
	lancamentos, err := faturas.LerLancamentos(ctx, s.db, idsCartoes, inicioHistorico.SomarMeses(-24))
	if err != nil {
		return PlanoMetasDto{}, err
	}
	importadas, err := faturas.LerFaturas(ctx, s.db, `WHERE "ContaId" IN (`+marcadores(len(idsCartoes))+`)`, idsCartoes...)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	// #251: pagamento lançado no cartão, para o banco que não manda os pagamentos da fatura.
	pagamentos, err := faturas.LerPagamentos(ctx, s.db, idsCartoes, hoje.SomarMeses(-2))
	if err != nil {
		return PlanoMetasDto{}, err
	}
	futuras := projetarParcelas(lancamentos, cartoes, hoje)

	porMes := map[int]faturaDoMes{} // pelo NumeroDoDia do 1º dia do mês do vencimento
	faturaAPagar := tipos.Zero
	for _, cartao := range cartoes {
		dele := lancamentos[cartao.id]
		porDia, err := compraAVistaPorDia(dele, inicioHistorico, fimHistorico, meses)
		if err != nil {
			return PlanoMetasDto{}, err
		}
		apagar, err := estimarFaturas(cartao, dele, futuras, importadas[cartao.id], pagamentos[cartao.id], porDia,
			inicioHistorico, mesAtual, hoje, porMes)
		if err != nil {
			return PlanoMetasDto{}, err
		}
		faturaAPagar = tipos.Somar(faturaAPagar, apagar)
	}

	// Mês atual: o realizado mais a média do que falta; meses seguintes: as médias e a fatura estimada.
	diasMes := relogio.UltimoDiaMes(hoje.Ano(), hoje.Mes()).Dia()
	fracaoRestante, err := tipos.Dividir(tipos.DecimalDeInteiro(int64(diasMes-hoje.Dia())), tipos.DecimalDeInteiro(int64(diasMes)))
	if err != nil {
		return PlanoMetasDto{}, err
	}
	recebido := receitas(caixa, mesAtual, hoje)
	saido := despesas(caixa, mesAtual, hoje)
	receitasMes := tipos.Maior(recebido, mediaReceitas)
	despesasMes := tipos.Somar(saido, tipos.Multiplicar(mediaDespesas, fracaoRestante))
	for k := range horizonteMeses {
		m := mesAtual.SomarMeses(k)
		rec, desp := mediaReceitas, mediaDespesas
		if k == 0 {
			rec, desp = receitasMes, despesasMes
		}
		f := porMes[m.NumeroDoDia()]
		plano.Meses = append(plano.Meses, MesCaixaDto{Ano: m.Ano(), Mes: int(m.Mes()),
			Receitas: tipos.Arredondar(rec, 2), Despesas: tipos.Arredondar(desp, 2),
			Faturas: tipos.Arredondar(zeroSeVazio(f.total), 2), Parcelas: tipos.Arredondar(zeroSeVazio(f.parcelas), 2),
			Sobra: tipos.Arredondar(tipos.Subtrair(tipos.Subtrair(rec, desp), zeroSeVazio(f.total)), 2)})
	}

	// Saldo previsto nas contas no fim do mês, depois de pagar a fatura que ainda vence nele.
	saldos, err := contas.Saldos(ctx, s.db)
	if err != nil {
		return PlanoMetasDto{}, err
	}
	saldoContas := tipos.Zero
	for _, id := range idsCaixa {
		if v, ok := saldos[id]; ok {
			saldoContas = tipos.Somar(saldoContas, v)
		} else {
			saldoContas = tipos.Somar(saldoContas, tipos.Zero)
		}
	}
	saldoFimDoMes := tipos.Subtrair(tipos.Subtrair(tipos.Somar(saldoContas, tipos.Subtrair(receitasMes, recebido)),
		tipos.Subtrair(despesasMes, saido)), faturaAPagar)

	// Teste de realidade: quanto de fato sobrou no melhor dos últimos meses completos (o Max do LINQ: o primeiro no
	// empate).
	var melhorMes tipos.Decimal
	for i := range mesesConsiderados {
		m := inicioHistorico.SomarMeses(i)
		fim := m.SomarMeses(1).SomarDias(-1)
		sobra := tipos.Subtrair(tipos.Subtrair(receitas(caixa, m, fim), despesas(caixa, m, fim)), zeroSeVazio(porMes[m.NumeroDoDia()].total))
		if i == 0 || sobra.Decimal().GreaterThan(melhorMes.Decimal()) {
			melhorMes = sobra
		}
	}

	plano.Disponivel = true
	plano.SaldoContas = tipos.Arredondar(saldoContas, 2)
	plano.FaturaAPagar = tipos.Arredondar(faturaAPagar, 2)
	plano.SaldoFimDoMes = tipos.Arredondar(saldoFimDoMes, 2)
	plano.MelhorMesRecente = tipos.Arredondar(melhorMes, 2)
	return plano, nil
}

// zeroSeVazio é o default(decimal) do GetValueOrDefault: o mês sem fatura vale 0.
func zeroSeVazio(d tipos.Decimal) tipos.Decimal { return tipos.Somar(tipos.Zero, d) }

func marcadores(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

// lerContas são as contas não arquivadas, na ordem do Id (a da tabela, que o C# lê sem ORDER BY).
func (s *Servico) lerContas(ctx context.Context) ([]contaDoPlano, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Tipo", "IncluirNoSaldo", "DiaFechamento", "DiaVencimento"
		FROM "Conta" WHERE NOT "Arquivada" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as contas do plano: %w", err)
	}
	defer linhas.Close()
	var lista []contaDoPlano
	for linhas.Next() {
		var c contaDoPlano
		if err := linhas.Scan(&c.id, &c.tipo, &c.incluirNoSaldo, &c.diaFechamento, &c.diaVencimento); err != nil {
			return nil, fmt.Errorf("ler conta do plano: %w", err)
		}
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as contas do plano: %w", err)
	}
	return lista, nil
}

// movimento é uma transação das contas de dinheiro.
type movimento struct {
	data  tipos.Data
	valor tipos.Decimal
	tipo  int64
}

// somaDe é o Sum do LINQ em memória: começa do 0 e soma na ordem.
func somaDe(lista []movimento, de, ate tipos.Data, tipo int64) tipos.Decimal {
	soma := tipos.Zero
	for _, t := range lista {
		if t.tipo == tipo && t.data.NumeroDoDia() >= de.NumeroDoDia() && t.data.NumeroDoDia() <= ate.NumeroDoDia() {
			soma = tipos.Somar(soma, t.valor)
		}
	}
	return soma
}

func receitas(lista []movimento, de, ate tipos.Data) tipos.Decimal {
	return somaDe(lista, de, ate, transacoes.Receita)
}

func despesas(lista []movimento, de, ate tipos.Data) tipos.Decimal {
	return tipos.Subtrair(tipos.Zero, somaDe(lista, de, ate, transacoes.Despesa))
}

// compraAVistaPorDia é a média diária de compras à vista do cartão (estornos descontados), para as faturas ainda
// abertas ou futuras: max(0, −soma) ÷ meses ÷ 30,44.
func compraAVistaPorDia(dele []faturas.Lancamento, inicio, fim tipos.Data, meses tipos.Decimal) (tipos.Decimal, error) {
	soma, tem := tipos.Zero, false
	for _, t := range dele {
		if !t.Parcelado() && t.Data.NumeroDoDia() >= inicio.NumeroDoDia() && t.Data.NumeroDoDia() <= fim.NumeroDoDia() {
			soma, tem = tipos.Somar(soma, t.Valor), true
		}
	}
	if !tem {
		return tipos.Zero, nil // o cartão sem compras à vista não entra no dicionário: GetValueOrDefault dá 0
	}
	media, err := tipos.Dividir(tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, soma)), meses)
	if err != nil {
		return tipos.Zero, err
	}
	return tipos.Dividir(media, diasPorMes)
}

// projetarParcelas são as parcelas que ainda vão ser cobradas, só as projetadas: as que já estão no banco entram pelo
// lançamento (#165). A base são as parcelas recentes (até DiasRecentes dias) dos cartões, na ordem da consulta.
func projetarParcelas(lancamentos map[int64][]faturas.Lancamento, cartoes []contaDoPlano, hoje tipos.Data) []parcelas.Futura {
	desde := hoje.SomarDias(-parcelas.DiasRecentes)
	var base []parcelas.Parcela
	for _, c := range cartoes {
		for _, t := range lancamentos[c.id] {
			if t.Data.NumeroDoDia() >= desde.NumeroDoDia() && t.Valor.Decimal().IsNegative() && t.Parcelado() &&
				t.ParcelaNumero != nil {
				base = append(base, parcelas.Parcela{Id: t.Id, ContaId: t.ContaId, Data: t.Data, Valor: t.Valor,
					Numero: *t.ParcelaNumero, Total: *t.ParcelaTotal, Descricao: t.Descricao})
			}
		}
	}
	var projetadas []parcelas.Futura
	for _, f := range parcelas.Projetar(base, hoje) {
		if !f.JaNoBanco {
			projetadas = append(projetadas, f)
		}
	}
	return projetadas
}

// estimarFaturas soma em porMes a fatura de cada mês do cartão, pelo vencimento, e devolve o que ainda vai sair da
// conta este mês: a fatura que vence de hoje em diante e não está paga.
func estimarFaturas(cartao contaDoPlano, dele []faturas.Lancamento, futuras []parcelas.Futura, importadas []faturas.Fatura,
	pagamentos []faturas.Pagamento, porDia tipos.Decimal, inicioHistorico, mesAtual, hoje tipos.Data,
	porMes map[int]faturaDoMes) (tipos.Decimal, error) {
	var futurasDele []parcelas.Futura
	for _, p := range futuras {
		if p.ContaId == cartao.id {
			futurasDele = append(futurasDele, p)
		}
	}
	mesDeCobranca := faturas.MesesDeCobranca(dele, importadas, cartao.diaFechamento)
	aPagar := tipos.Zero
	ultimo := mesAtual.SomarMeses(horizonteMeses).NumeroDoDia()
	for mes := inicioHistorico.SomarMeses(-1); mes.NumeroDoDia() <= ultimo; mes = mes.SomarMeses(1) {
		ciclo := faturas.CicloDaFatura(mes, mes.SomarMeses(1).SomarDias(-1), cartao.diaFechamento, cartao.diaVencimento, importadas)
		// Sem dia de fechamento conhecido, o ciclo é o mês do calendário e vence no mês seguinte (a fatura importada
		// que vence no próprio mês é de compras anteriores).
		semFechamento := ciclo.Fechamento == nil
		fechamento := ciclo.Fim
		if !semFechamento {
			fechamento = *ciclo.Fechamento
		}
		vencimento := fechamento.SomarMeses(1)
		var importada *faturas.Fatura
		if !semFechamento {
			if ciclo.Vencimento != nil {
				vencimento = *ciclo.Vencimento
			}
			importada = ciclo.Fatura
		}
		mesVencimento := tipos.NovaData(vencimento.Ano(), vencimento.Mes(), 1)
		if mesVencimento.NumeroDoDia() < inicioHistorico.NumeroDoDia() {
			continue
		}

		mesDoCiclo := tipos.NovaData(fechamento.Ano(), fechamento.Mes(), 1)
		somaLancado, somaParcelado := tipos.Zero, tipos.Zero
		for _, t := range dele {
			if m, ok := mesDeCobranca[t.Id]; ok && m.Igual(mesDoCiclo) {
				somaLancado = tipos.Somar(somaLancado, t.Valor)
				if t.Parcelado() {
					somaParcelado = tipos.Somar(somaParcelado, t.Valor)
				}
			}
		}
		parcelasLancadas := tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, somaParcelado))
		var total, parcelasDoMes tipos.Decimal
		if fechamento.NumeroDoDia() <= hoje.NumeroDoDia() {
			informado := tipos.Zero
			if importada != nil {
				informado = tipos.DeDecimal(importada.ValorTotal.Decimal().Abs())
			}
			if informado.Decimal().IsPositive() {
				total = informado
			} else {
				total = tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, somaLancado))
			}
			parcelasDoMes = parcelasLancadas
		} else {
			de := hoje.SomarDias(1)
			if ciclo.Inicio.NumeroDoDia() > hoje.NumeroDoDia() {
				de = ciclo.Inicio
			}
			diasQueFaltam := fechamento.NumeroDoDia() - de.NumeroDoDia() + 1
			projetadas := tipos.Zero
			for _, p := range futurasDele {
				var cai bool
				if m, ok := mesDeCobranca[p.OrigemId]; ok {
					cai = m.SomarMeses(p.Passos).Igual(mesDoCiclo)
				} else {
					cai = p.Data.NumeroDoDia() >= de.NumeroDoDia() && p.Data.NumeroDoDia() <= fechamento.NumeroDoDia()
				}
				if cai {
					projetadas = tipos.Somar(projetadas, p.Valor)
				}
			}
			total = tipos.Somar(tipos.Somar(tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, somaLancado)),
				tipos.Multiplicar(porDia, tipos.DecimalDeInteiro(int64(diasQueFaltam)))), projetadas)
			parcelasDoMes = tipos.Somar(parcelasLancadas, projetadas)
		}
		anterior := porMes[mesVencimento.NumeroDoDia()]
		porMes[mesVencimento.NumeroDoDia()] = faturaDoMes{tipos.Somar(zeroSeVazio(anterior.total), total),
			tipos.Somar(zeroSeVazio(anterior.parcelas), parcelasDoMes)}

		if mesVencimento.Igual(mesAtual) && vencimento.NumeroDoDia() >= hoje.NumeroDoDia() {
			var informadoPago *tipos.Decimal
			if importada != nil {
				informadoPago = importada.ValorPago
			}
			pago := faturas.Pago(informadoPago, faturas.PagoNasTransacoes(vencimento, faturas.Vencimentos(importadas), pagamentos))
			jaPago := tipos.Zero
			if pago != nil {
				jaPago = *pago
			}
			aPagar = tipos.Somar(aPagar, tipos.Maior(tipos.Zero, tipos.Subtrair(total, jaPago)))
		}
	}
	return aPagar, nil
}
