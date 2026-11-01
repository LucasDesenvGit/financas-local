package partida

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// A partida de cada teste roda "hoje" (30/10/2026, meio-dia em Brasília); os dados do banco foram gravados "ontem", para o
// AtualizadoEm mostrar o que as rotinas mudaram.
var (
	ontem = testeapoio.AgoraEmBrasilia(2026, 10, 29, 12, 0)
	hoje  = testeapoio.AgoraEmBrasilia(2026, 10, 30, 12, 0)
)

func p[T any](v T) *T { return &v }

// banco de um usuário que instalou antes das regras novas: um caso que cada rotina acerta, em linhas que as outras
// rotinas deixam como estão.
type cena struct {
	db  *sql.DB
	fab *testeapoio.Fabrica

	cartao, corrente, poupanca testeapoio.Conta
	investimentos, generica    testeapoio.Categoria
	mercado, rendimentos       testeapoio.Categoria

	meliDolar     int64   // rotina 1: "Méli Dólar" na conta corrente
	estorno       int64   // rotina 2: entrada no cartão que o banco mandou como receita
	compras       []int64 // rotina 3: as 3 parcelas e o estorno do total, já como despesa
	saidaComPar   int64   // rotina 4: TED para a poupança, com a entrada do outro lado
	contaAPagar   int64   // rotina 5: débito automático que já chegou no extrato
	trocoDaCompra int64   // rotina 6: o troco sem dúvida de uma compra de renda variável
}

func novaCena(t *testing.T) *cena {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	c := &cena{db: db, fab: fab}
	categoria := func(nome string, tipo int64) testeapoio.Categoria {
		return fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = nome, tipo })
	}
	c.investimentos, c.generica = categoria("Investimentos", 3), categoria("Transferências", 3)
	c.mercado, c.rendimentos = categoria("Mercado", 1), categoria("Rendimentos", 2)
	c.cartao = fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 3 })
	c.corrente = fab.Conta()
	c.poupanca = fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 2 })
	dia := func(n int) tipos.Data { return relogio.Hoje(hoje).SomarDias(n) }

	// 1: a regra do Méli Dólar chegou depois da instalação.
	c.meliDolar = fab.Transacao(c.corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Descricao, tr.CategoriaId = dia(-3), "Meli Dólar", &c.mercado.Id
	}).Id
	// 2: o estorno que contava como receita.
	c.estorno = fab.Transacao(c.cartao, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Descricao, tr.Valor, tr.Tipo, tr.CategoriaId = dia(-2), "ESTORNO LOJA", testeapoio.Valor("50"), 2, &c.mercado.Id
	}).Id
	// 3: a compra parcelada devolvida inteira (o estorno já é despesa positiva).
	parcela := func(n int64, dias int, valor, descricao string, ajustes ...func(*testeapoio.Transacao)) int64 {
		return fab.Transacao(c.cartao, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao, tr.DescricaoOriginal = dia(dias), testeapoio.Valor(valor), descricao, &descricao
			if n > 0 {
				tr.ParcelaNumero, tr.ParcelaTotal = &n, p(int64(3))
			}
		}).Id
	}
	c.compras = []int64{
		parcela(1, -60, "-100", "LOJA ALFA 01/03"), parcela(2, -30, "-100", "LOJA ALFA 02/03"),
		parcela(3, -1, "-100", "LOJA ALFA 03/03"), parcela(0, -20, "300", "ESTORNO DE LOJA ALFA"),
	}
	// 4: TED para a poupança, com a entrada do outro lado, na categoria genérica de transferências.
	c.saidaComPar = fab.Transacao(c.corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Descricao, tr.Valor, tr.CategoriaId = dia(-10), "TED PARA POUPANCA", testeapoio.Valor("-500"), &c.generica.Id
	}).Id
	fab.Transacao(c.poupanca, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Descricao, tr.Valor, tr.Tipo = dia(-8), "TED RECEBIDA", testeapoio.Valor("500"), 2
	})
	// 5: o débito automático da academia, que vence daqui a 3 dias e já foi cobrado há 2.
	agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.Valor, a.ProximoVencimento, a.FormaCobranca = "Academia", testeapoio.Valor("89.9"), dia(3), 2
		a.TextoCobranca, a.ContaId = p("academia forma"), &c.corrente.Id
	})
	c.contaAPagar = agendada.Id
	fab.Transacao(c.corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Descricao, tr.DescricaoOriginal = dia(-2), testeapoio.Valor("-89.9"), "Minha academia", p("DEB AUT ACADEMIA FORMA")
	})
	// 6: compra de renda variável de 100 com o débito de 102, e os 2 de volta da corretora.
	ativo := fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "AAAA3" })
	fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) {
		o.Data, o.Quantidade, o.PrecoUnitario = dia(-20), testeapoio.Valor("10"), testeapoio.Valor("10")
	})
	fab.Transacao(c.corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Descricao, tr.Valor = dia(-20), "Compra de Renda Variável", testeapoio.Valor("-102")
	})
	c.trocoDaCompra = fab.Transacao(c.corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Descricao, tr.Valor, tr.Tipo, tr.CategoriaId = dia(-19), "Valor recebido de Investimentos", testeapoio.Valor("2"), 2, &c.rendimentos.Id
	}).Id
	return c
}

type estado struct {
	tipo         int64
	ignorar      bool
	categoria    *int64
	manual       bool
	atualizadoEm string
}

func (c *cena) estado(t *testing.T, id int64) estado {
	t.Helper()
	var e estado
	err := c.db.QueryRowContext(t.Context(), `SELECT Tipo, IgnorarRelatorios, CategoriaId, CategoriaManual, AtualizadoEm FROM Transacao WHERE Id = ?`, id).
		Scan(&e.tipo, &e.ignorar, &e.categoria, &e.manual, &e.atualizadoEm)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e estado) deHoje() bool { return e.atualizadoEm == tipos.NovaDataHora(hoje.Agora()).String() }

func (e estado) naCategoria(id int64) bool { return e.categoria != nil && *e.categoria == id }

// fotografia é o que as rotinas podem mudar, em texto: duas fotografias iguais são dois bancos iguais.
func (c *cena) fotografia(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, consulta := range []string{
		`SELECT Id, ContaId, Data, Descricao, Valor, Tipo, CategoriaId, CategoriaManual, IgnorarRelatorios, TransferenciaGrupo, AtualizadoEm FROM Transacao ORDER BY Id`,
		`SELECT * FROM RegraCategorizacao ORDER BY Id`,
		`SELECT * FROM Configuracao ORDER BY Chave`,
		`SELECT * FROM ContaAgendada ORDER BY Id`,
		`SELECT Evento, Login FROM AuditoriaAcesso ORDER BY Id`,
	} {
		linhas, err := c.db.QueryContext(t.Context(), consulta)
		if err != nil {
			t.Fatal(err)
		}
		colunas, _ := linhas.Columns()
		for linhas.Next() {
			valores := make([]any, len(colunas))
			ponteiros := make([]any, len(colunas))
			for i := range valores {
				ponteiros[i] = &valores[i]
			}
			if err := linhas.Scan(ponteiros...); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintln(&b, valores...)
		}
		if err := linhas.Err(); err != nil {
			t.Fatal(err)
		}
		linhas.Close()
	}
	return b.String()
}

func opcoes(db *sql.DB, log *slog.Logger) Opcoes { return Opcoes{DB: db, Relogio: hoje, Log: log} }

func logEmTexto() (*slog.Logger, *bytes.Buffer) {
	var saida bytes.Buffer
	return slog.New(slog.NewTextHandler(&saida, &slog.HandlerOptions{Level: slog.LevelDebug})), &saida
}

func nomes(r []Resultado) []string {
	var l []string
	for _, x := range r {
		l = append(l, x.Rotina)
	}
	return l
}

// Regra: a partida roda as seis rotinas, na ordem do Program.cs, e cada uma acerta o dado que lhe cabe; a segunda
// partida não muda nada (nem o carimbo, nem as marcas, nem a auditoria).
func TestPartidaAcertaOQueCadaRotinaPrometeEESegundaNaoMudaNada(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	log, saida := logEmTexto()

	resultados := Executar(t.Context(), opcoes(c.db, log))

	ordem := []string{RegrasPadrao, EntradasNosCartoes, ComprasParceladasEstornadas, Transferencias, ContasAPagar, TrocosSemDuvida}
	if obtida := nomes(resultados); strings.Join(obtida, "|") != strings.Join(ordem, "|") {
		t.Fatalf("rotinas %v, esperado %v", obtida, ordem)
	}
	quantas := map[string]int{RegrasPadrao: 1, EntradasNosCartoes: 1, ComprasParceladasEstornadas: 4, Transferencias: 1, ContasAPagar: 1, TrocosSemDuvida: 1}
	for _, r := range resultados {
		if r.Erro != nil || r.Mudaram != quantas[r.Rotina] {
			t.Errorf("%s: mudaram %d (esperado %d), erro %v", r.Rotina, r.Mudaram, quantas[r.Rotina], r.Erro)
		}
	}

	// 1
	if e := c.estado(t, c.meliDolar); !e.naCategoria(c.investimentos.Id) || e.tipo != transacoes.Transferencia || !e.ignorar || !e.deHoje() {
		t.Errorf("regras padrão: o Méli Dólar deveria ir para Investimentos, fora dos relatórios: %+v", e)
	}
	var regras int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM RegraCategorizacao WHERE CategoriaId = ?`, c.investimentos.Id).Scan(&regras); err != nil || regras != 9 {
		t.Errorf("regras criadas: %d, %v", regras, err)
	}
	// 2
	if e := c.estado(t, c.estorno); e.tipo != transacoes.Despesa || e.ignorar || !e.naCategoria(c.mercado.Id) || !e.deHoje() {
		t.Errorf("entradas nos cartões: o estorno deveria ser despesa positiva, na categoria dele: %+v", e)
	}
	// 3
	for _, id := range c.compras {
		if e := c.estado(t, id); !e.ignorar || !e.deHoje() {
			t.Errorf("compra parcelada estornada: %d deveria ficar fora dos relatórios: %+v", id, e)
		}
	}
	// 4
	if e := c.estado(t, c.saidaComPar); e.tipo != transacoes.Transferencia || !e.ignorar || !e.deHoje() {
		t.Errorf("transferências: a TED com par deveria ficar fora dos relatórios: %+v", e)
	}
	// 5
	var ultimo, proximo string
	if err := c.db.QueryRow(`SELECT UltimoPagamentoEm, ProximoVencimento FROM ContaAgendada WHERE Id = ?`, c.contaAPagar).Scan(&ultimo, &proximo); err != nil {
		t.Fatal(err)
	}
	if ultimo != relogio.Hoje(hoje).SomarDias(-2).String() || proximo != relogio.Hoje(hoje).SomarDias(3).SomarMeses(1).String() {
		t.Errorf("contas a pagar: último pagamento %s, próximo vencimento %s", ultimo, proximo)
	}
	// 6
	if e := c.estado(t, c.trocoDaCompra); !e.naCategoria(c.investimentos.Id) || e.tipo != transacoes.Transferencia || !e.ignorar || !e.manual || !e.deHoje() {
		t.Errorf("trocos sem dúvida: o troco deveria ir para Investimentos, fora dos relatórios, como escolha manual: %+v", e)
	}
	for _, rotina := range ordem {
		if !strings.Contains(saida.String(), "level=INFO msg=\"rotina de partida acertou dados\" rotina="+campoDoLog(rotina)+" mudaram=") {
			t.Errorf("o log deveria dizer o que a rotina %q acertou:\n%s", rotina, saida)
		}
	}

	// A segunda partida, um dia depois: nada muda.
	antes := c.fotografia(t)
	log2, saida2 := logEmTexto()
	segunda := opcoes(c.db, log2)
	segunda.Relogio = testeapoio.AgoraEmBrasilia(2026, 10, 31, 12, 0)
	for _, r := range Executar(t.Context(), segunda) {
		if r.Erro != nil || r.Mudaram != 0 {
			t.Errorf("segunda partida, %s: mudaram %d, erro %v", r.Rotina, r.Mudaram, r.Erro)
		}
	}
	if depois := c.fotografia(t); depois != antes {
		t.Errorf("a segunda partida mudou o banco:\nantes:\n%s\ndepois:\n%s", antes, depois)
	}
	if saida2.Len() != 0 {
		t.Errorf("a partida que não mudou nada não escreve no log:\n%s", saida2)
	}
}

// campoDoLog é o valor como o TextHandler do slog o escreve: entre aspas só quando tem espaço.
func campoDoLog(s string) string {
	if strings.ContainsAny(s, " =\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// Regra: a ordem importa (Program.cs:211-212). O estorno do cartão que o banco manda como receita só vira despesa
// positiva na rotina 2, e só então a rotina 3 o casa com a compra parcelada: com a ordem certa, uma partida só deixa
// tudo fora dos relatórios.
func TestPartidaNaOrdemOEstornoComoReceitaSaiJuntoComACompra(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	// O estorno do total da compra parcelada chega como receita, como o banco manda.
	if _, err := c.db.Exec(`UPDATE Transacao SET Tipo = 2 WHERE Id = ?`, c.compras[3]); err != nil {
		t.Fatal(err)
	}
	log, _ := logEmTexto()
	resultados := Executar(t.Context(), opcoes(c.db, log))
	for _, r := range resultados {
		if r.Erro != nil {
			t.Fatalf("%s: %v", r.Rotina, r.Erro)
		}
	}
	if e := c.estado(t, c.compras[3]); e.tipo != transacoes.Despesa || !e.ignorar {
		t.Errorf("o estorno deveria virar despesa (rotina 2) e sair dos relatórios (rotina 3), na mesma partida: %+v", e)
	}
	for _, id := range c.compras[:3] {
		if e := c.estado(t, id); !e.ignorar {
			t.Errorf("a parcela %d deveria sair dos relatórios junto: %+v", id, e)
		}
	}
	if resultados[1].Mudaram != 2 || resultados[2].Mudaram != 4 {
		t.Errorf("entradas nos cartões %d (esperado 2), compras parceladas %d (esperado 4)", resultados[1].Mudaram, resultados[2].Mudaram)
	}
}

// falhaNaRotina faz o banco recusar a gravação que a rotina faria: um gatilho que aborta o UPDATE ou o INSERT.
func (c *cena) falhaNaRotina(t *testing.T, gatilho string) {
	t.Helper()
	if _, err := c.db.Exec(gatilho); err != nil {
		t.Fatal(err)
	}
}

// Regra: erro numa rotina não derruba a subida: a partida devolve o erro dela, registra um aviso e segue para as seguintes
// (#444; o try/catch único do Program.cs parava ali), e deixa o que as rotinas de antes já gravaram. A que falhou desfaz o
// que fez pela metade (o SaveChanges único do .NET) e acerta na próxima partida.
func TestPartidaErroNumaRotinaSegueParaAsSeguintes(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	// A rotina 3 falha ao marcar a compra parcelada.
	c.falhaNaRotina(t, `CREATE TRIGGER falha_rotina_3 BEFORE UPDATE OF IgnorarRelatorios ON Transacao
		WHEN NEW.Descricao LIKE 'LOJA ALFA 02%' BEGIN SELECT RAISE(ABORT, 'banco recusou'); END`)
	log, saida := logEmTexto()

	resultados := Executar(t.Context(), opcoes(c.db, log))

	if len(resultados) != 6 || resultados[2].Rotina != ComprasParceladasEstornadas || resultados[2].Erro == nil ||
		!strings.Contains(resultados[2].Erro.Error(), "banco recusou") {
		t.Fatalf("a partida deveria rodar as seis, com o erro do banco na rotina 3: %+v", resultados)
	}
	for _, r := range resultados[3:] {
		if r.Erro != nil || r.Mudaram != 1 {
			t.Errorf("%s deveria ter rodado depois da falha da 3: %+v", r.Rotina, r)
		}
	}
	if resultados[0].Erro != nil || resultados[1].Erro != nil || resultados[0].Mudaram != 1 || resultados[1].Mudaram != 1 {
		t.Errorf("as rotinas 1 e 2 rodaram direito: %+v", resultados[:2])
	}
	// O que as rotinas 1 e 2 gravaram ficou.
	if e := c.estado(t, c.meliDolar); !e.naCategoria(c.investimentos.Id) {
		t.Errorf("a rotina 1 gravou antes da 3 falhar: %+v", e)
	}
	if e := c.estado(t, c.estorno); e.tipo != transacoes.Despesa {
		t.Errorf("a rotina 2 gravou antes da 3 falhar: %+v", e)
	}
	// A rotina 3 não deixou nada pela metade: as parcelas que ela já tinha marcado voltaram (a transação foi desfeita).
	for _, id := range c.compras {
		if e := c.estado(t, id); e.ignorar {
			t.Errorf("a rotina 3 desfaz tudo o que fez: %d ficou fora dos relatórios", id)
		}
	}
	// As seguintes rodaram assim mesmo: a TED acertada, a conta a pagar com baixa e o troco na categoria da compra.
	if e := c.estado(t, c.saidaComPar); e.tipo != transacoes.Transferencia || !e.deHoje() {
		t.Errorf("a rotina 4 deveria ter rodado: %+v", e)
	}
	var ultimo *string
	if err := c.db.QueryRow(`SELECT UltimoPagamentoEm FROM ContaAgendada WHERE Id = ?`, c.contaAPagar).Scan(&ultimo); err != nil || ultimo == nil {
		t.Errorf("a rotina 5 deveria ter rodado: %v, %v", ultimo, err)
	}
	if e := c.estado(t, c.trocoDaCompra); !e.naCategoria(c.investimentos.Id) {
		t.Errorf("a rotina 6 deveria ter rodado: %+v", e)
	}
	// O aviso vai para o log (e, no Go sozinho, para Configurações › Erros) com a rotina e o erro.
	texto := saida.String()
	for _, esperado := range []string{"level=WARN", "Não foi possível acertar os dados ao iniciar", "rotina=" + campoDoLog(ComprasParceladasEstornadas), "banco recusou"} {
		if !strings.Contains(texto, esperado) {
			t.Errorf("o aviso deveria ter %q:\n%s", esperado, texto)
		}
	}

	// Sem o defeito, a próxima partida acerta só o que a rotina 3 deixou.
	if _, err := c.db.Exec(`DROP TRIGGER falha_rotina_3`); err != nil {
		t.Fatal(err)
	}
	proxima := Executar(t.Context(), opcoes(c.db, slog.New(slog.DiscardHandler)))
	if len(proxima) != 6 || proxima[2].Mudaram != 4 || proxima[3].Mudaram != 0 || proxima[4].Mudaram != 0 || proxima[5].Mudaram != 0 {
		t.Errorf("a próxima partida deveria acertar só a rotina 3: %+v", proxima)
	}
	for _, r := range proxima {
		if r.Erro != nil {
			t.Errorf("%s: %v", r.Rotina, r.Erro)
		}
	}
}

// Regra: a rotina que falha no meio desfaz o que fez, a marca das regras padrão inclusive (uma transação por rotina), e a
// partida registra o erro e segue.
func TestPartidaRotinaQueFalhaNoMeioDesfazTudo(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	c.falhaNaRotina(t, `CREATE TRIGGER falha_regra BEFORE INSERT ON RegraCategorizacao
		WHEN NEW.Texto = 'RESGATE RDB' BEGIN SELECT RAISE(ABORT, 'regra recusada'); END`)
	log, _ := logEmTexto()

	resultados := Executar(t.Context(), opcoes(c.db, log))

	if len(resultados) != 6 || resultados[0].Rotina != RegrasPadrao || resultados[0].Erro == nil || resultados[0].Mudaram != 0 {
		t.Fatalf("a partida deveria registrar o erro na rotina 1 e seguir: %+v", resultados)
	}
	var n int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM RegraCategorizacao`).Scan(&n); err != nil || n != 0 {
		t.Errorf("nenhuma regra deveria ter ficado: %d, %v", n, err)
	}
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM Configuracao WHERE Chave LIKE 'Regras.%'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("nenhuma marca deveria ter ficado: %d, %v", n, err)
	}
	if e := c.estado(t, c.meliDolar); e.naCategoria(c.investimentos.Id) || e.deHoje() {
		t.Errorf("a transação do Méli Dólar não deveria ter mudado: %+v", e)
	}
}

// Regra: erro ou pânico numa rotina vira o erro dela, sem derrubar o processo; as seguintes rodam assim mesmo, e as
// anteriores ficam como estão. A rotina que roda é a que está na lista, na ordem dela.
func TestExecutarComRotinasQueFalhamEEntramEmPanico(t *testing.T) {
	t.Parallel()
	var rodaram []string
	registrar := func(nome string, n int, err error, entraEmPanico bool) rotina {
		return rotina{nome, func(context.Context) (int, error) {
			rodaram = append(rodaram, nome)
			if entraEmPanico {
				panic("bug de programação")
			}
			return n, err
		}}
	}
	casos := []struct {
		nome     string
		lista    []rotina
		rodaram  []string
		mudaram  []int
		falhouEm string // vazio: nenhuma
	}{
		{"todas dão certo", []rotina{registrar("a", 1, nil, false), registrar("b", 0, nil, false), registrar("c", 3, nil, false)},
			[]string{"a", "b", "c"}, []int{1, 0, 3}, ""},
		{"erro na do meio", []rotina{registrar("a", 1, nil, false), registrar("b", 0, errors.New("falhou"), false), registrar("c", 3, nil, false)},
			[]string{"a", "b", "c"}, []int{1, 0, 3}, "b"},
		{"pânico na primeira", []rotina{registrar("a", 0, nil, true), registrar("b", 1, nil, false)}, []string{"a", "b"}, []int{0, 1}, "a"},
		{"sem rotinas", nil, nil, nil, ""},
	}
	for _, caso := range casos {
		rodaram = nil
		log, saida := logEmTexto()
		resultados := executar(t.Context(), log, caso.lista)
		if strings.Join(rodaram, ",") != strings.Join(caso.rodaram, ",") {
			t.Errorf("%s: rodaram %v, esperado %v", caso.nome, rodaram, caso.rodaram)
		}
		if len(resultados) != len(caso.mudaram) {
			t.Fatalf("%s: %d resultados, esperado %d", caso.nome, len(resultados), len(caso.mudaram))
		}
		for i, r := range resultados {
			if r.Mudaram != caso.mudaram[i] && caso.falhouEm != r.Rotina {
				t.Errorf("%s: %s mudou %d, esperado %d", caso.nome, r.Rotina, r.Mudaram, caso.mudaram[i])
			}
			if (r.Erro != nil) != (r.Rotina == caso.falhouEm) {
				t.Errorf("%s: %s com erro %v", caso.nome, r.Rotina, r.Erro)
			}
		}
		if (strings.Contains(saida.String(), "level=WARN")) != (caso.falhouEm != "") {
			t.Errorf("%s: o aviso só sai quando uma rotina falha:\n%s", caso.nome, saida)
		}
	}
	// O pânico vira erro com o nome da rotina e o que foi dito.
	resultados := executar(t.Context(), slog.New(slog.DiscardHandler), []rotina{registrar("x", 0, nil, true)})
	if e := resultados[0].Erro; e == nil || !strings.Contains(e.Error(), `"x"`) || !strings.Contains(e.Error(), "bug de programação") {
		t.Errorf("erro do pânico: %v", e)
	}

	// O contexto cancelado no meio (o sistema parando) interrompe a partida: as seguintes falhariam do mesmo jeito.
	ctx, cancelar := context.WithCancel(t.Context())
	rodaram = nil
	resultados = executar(ctx, slog.New(slog.DiscardHandler), []rotina{
		registrar("a", 1, nil, false),
		{"b", func(ctx context.Context) (int, error) {
			rodaram = append(rodaram, "b")
			cancelar()
			return 0, ctx.Err()
		}},
		registrar("c", 1, nil, false),
	})
	if strings.Join(rodaram, ",") != "a,b" || len(resultados) != 2 || !errors.Is(resultados[1].Erro, context.Canceled) {
		t.Errorf("com o contexto cancelado na b, a c não roda: rodaram %v, resultados %+v", rodaram, resultados)
	}
}

// Regra: a partida com o contexto cancelado (o sistema parando) não derruba nada: a primeira rotina falha com o erro do
// contexto, as outras não rodam, e o banco fica como estava.
func TestPartidaComContextoCancelado(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	antes := c.fotografia(t)
	ctx, cancelar := context.WithCancel(t.Context())
	cancelar()
	log, _ := logEmTexto()
	resultados := Executar(ctx, opcoes(c.db, log))
	if len(resultados) != 1 || !errors.Is(resultados[0].Erro, context.Canceled) {
		t.Fatalf("resultados %+v", resultados)
	}
	if depois := c.fotografia(t); depois != antes {
		t.Error("com o contexto cancelado, o banco não muda")
	}
}

// Regra: banco sem nada para acertar (instalação nova, só com os dados iniciais): as seis rotinas rodam sem mudar
// nada de usuário e a primeira cria as marcas das regras padrão uma vez só.
func TestPartidaNoBancoNovo(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBanco(t)
	log, saida := logEmTexto()
	resultados := Executar(t.Context(), opcoes(db, log))
	if len(resultados) != 6 {
		t.Fatalf("%d rotinas", len(resultados))
	}
	for _, r := range resultados {
		if r.Erro != nil || r.Mudaram != 0 {
			t.Errorf("%s: mudaram %d, erro %v", r.Rotina, r.Mudaram, r.Erro)
		}
	}
	if saida.Len() != 0 {
		t.Errorf("nada a acertar, nada no log:\n%s", saida)
	}
	var marcas, regras int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Configuracao WHERE Chave IN ('Regras.MeliDolarCriada', 'Regras.AplicacaoResgateCriadas')`).Scan(&marcas); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM RegraCategorizacao WHERE Texto IN ('MELI DOLAR', 'APLICACAO RDB')`).Scan(&regras); err != nil {
		t.Fatal(err)
	}
	if marcas != 2 || regras != 2 {
		t.Errorf("marcas %d e regras %d (as dos dados iniciais não são repetidas)", marcas, regras)
	}
}
