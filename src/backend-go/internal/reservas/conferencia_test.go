package reservas

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// cambioFalso é o câmbio da conferência: a cotação que o teste escolher (ou nenhuma, ou um erro). Ele anota as
// cotações pedidas; o histórico e o conversor, só do Listar, são erro do teste.
type cambioFalso struct {
	t       *testing.T
	cotacao *cambio.Cotacao
	erro    error
	pedidas []string
}

func (c *cambioFalso) ObterCotacao(_ context.Context, moeda string, data *tipos.Data) (*cambio.Cotacao, error) {
	c.pedidas = append(c.pedidas, moeda+" "+data.String())
	return c.cotacao, c.erro
}

func (c *cambioFalso) GarantirHistorico(context.Context, string, tipos.Data) error {
	c.t.Error("a conferência não usa o histórico do dólar")
	return nil
}

func (c *cambioFalso) ObterConversor(context.Context, *tipos.Data) (*cambio.Conversor, error) {
	c.t.Error("a conferência não usa o conversor")
	return nil, nil
}

// conferenciasDoBanco são as linhas de ReservaConferencia, na ordem em que foram gravadas.
func conferenciasDoBanco(t *testing.T, db *sql.DB) []string {
	t.Helper()
	linhas, err := db.Query(`SELECT ContaId, Nome, SaldoConferido, DataConferencia, AtualizadoEm FROM ReservaConferencia ORDER BY Id`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var conta, nome, saldo, data, atualizado string
		if err := linhas.Scan(&conta, &nome, &saldo, &data, &atualizado); err != nil {
			t.Fatal(err)
		}
		lista = append(lista, strings.Join([]string{conta, nome, saldo, data, atualizado}, " | "))
	}
	if err := linhas.Err(); err != nil {
		t.Fatal(err)
	}
	return lista
}

func dolar(compra string) *cambio.Cotacao {
	return &cambio.Cotacao{Moeda: "USD", Compra: testeapoio.Valor(compra), Venda: testeapoio.Valor(compra)}
}

func textoDe(s string) *string { return &s }

var erroDoCambio = errors.New("a PTAX caiu")

// meioDia é o carimbo da conferência antiga do teste.
var meioDia = func() tipos.DataHora {
	d, err := tipos.LerDataHora("2026-08-25 12:00:00")
	if err != nil {
		panic(err)
	}
	return d
}()

// Regra: a conferência é o ConferirAsync do C#: a conta tem de existir; o nome aparado; a reserva do mesmo nome, sem
// diferença de maiúsculas (a primeira, e o nome dela fica), é atualizada, senão uma nova; 2 casas pelo método
// bancário; o Méli Dólar com o nome do app e 8 casas, e o valor em reais dividido pela compra da PTAX da data (sem
// cotação, ou com a compra zero, é recusado); o carimbo em UTC.
func TestConferir(t *testing.T) {
	t.Parallel()
	const agora = "2026-10-30 18:00:00" // 15h em Brasília
	casos := []struct {
		nome          string
		conta         int64 // 0 = a conta do teste
		reserva       string
		saldo         string
		moeda         *string
		cotacao       *cambio.Cotacao
		erroDoCambio  error
		erro          error
		depois        []string
		cotacoesPedem []string
	}{
		{nome: "conta que não existe", conta: 999, reserva: "Viagem", saldo: "10", erro: ErrContaInvalida,
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00"}},
		{nome: "nova, com o nome aparado e 2 casas pelo método bancário", reserva: " \tCasa  ", saldo: "10.125",
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00", "1 | Casa | 10.12 | 2026-10-20 | " + agora}},
		{nome: "a existente, sem diferença de maiúsculas: o nome dela fica", reserva: "viagem", saldo: "35.5",
			depois: []string{"1 | VIAGEM | 35.5 | 2026-10-20 | " + agora}},
		{nome: "outra reserva em reais não converte nem pede cotação", reserva: "Viagem", saldo: "50", moeda: textoDe("BRL"),
			depois: []string{"1 | VIAGEM | 50.0 | 2026-10-20 | " + agora}},
		{nome: "Méli Dólar em dólares: o nome do app e 8 casas pelo método bancário", reserva: "MELI dolar", saldo: "1.123456785",
			moeda:  textoDe("USD"),
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00", "1 | Méli Dólar | 1.12345678 | 2026-10-20 | " + agora}},
		{nome: "Méli Dólar sem a moeda: em dólares", reserva: "Méli Dólar", saldo: "2",
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00", "1 | Méli Dólar | 2.0 | 2026-10-20 | " + agora}},
		{nome: "Méli Dólar em reais: dividido pela compra da PTAX da data", reserva: "Meli Dólar", saldo: "100", moeda: textoDe("BRL"),
			cotacao: dolar("5.3123"), cotacoesPedem: []string{"USD 2026-10-20"},
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00", "1 | Méli Dólar | 18.82423809 | 2026-10-20 | " + agora}},
		{nome: "Méli Dólar em reais sem cotação", reserva: "Meli Dolar", saldo: "100", moeda: textoDe("BRL"), erro: ErrSemCotacao,
			cotacoesPedem: []string{"USD 2026-10-20"}, depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00"}},
		{nome: "Méli Dólar em reais com a compra zero", reserva: "Meli Dolar", saldo: "100", moeda: textoDe("BRL"),
			cotacao: dolar("0.0"), erro: ErrSemCotacao, cotacoesPedem: []string{"USD 2026-10-20"},
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00"}},
		{nome: "erro do câmbio", reserva: "Meli Dolar", saldo: "100", moeda: textoDe("BRL"), erroDoCambio: erroDoCambio,
			erro: erroDoCambio, cotacoesPedem: []string{"USD 2026-10-20"},
			depois: []string{"1 | VIAGEM | 120.0 | 2026-08-25 | 2026-08-25 12:00:00"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
			fab := testeapoio.NovaFabrica(t, db, r)
			conta := fab.Conta()
			fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) {
				rc.Nome, rc.SaldoConferido = "VIAGEM", testeapoio.Valor("120.0")
				rc.DataConferencia, rc.AtualizadoEm = testeapoio.Dia(2026, 8, 25), meioDia
			})
			cambioDoTeste := &cambioFalso{t: t, cotacao: c.cotacao, erro: c.erroDoCambio}
			s := NovoServico(db, cambioDoTeste, r, slog.New(slog.DiscardHandler))
			id := conta.Id
			if c.conta != 0 {
				id = c.conta
			}
			err := s.Conferir(t.Context(), id, c.reserva, testeapoio.Valor(c.saldo), testeapoio.Dia(2026, 10, 20), c.moeda)
			if !errors.Is(err, c.erro) {
				t.Fatalf("erro %v, esperado %v", err, c.erro)
			}
			obtido := conferenciasDoBanco(t, db)
			esperado := c.depois
			if strings.Join(obtido, "\n") != strings.Join(esperado, "\n") {
				t.Errorf("no banco:\n%s\nesperado:\n%s", strings.Join(obtido, "\n"), strings.Join(esperado, "\n"))
			}
			if strings.Join(cambioDoTeste.pedidas, ",") != strings.Join(c.cotacoesPedem, ",") {
				t.Errorf("cotações pedidas %v, esperado %v", cambioDoTeste.pedidas, c.cotacoesPedem)
			}
		})
	}
}

// Regra: com duas reservas de mesmo nome em caixas diferentes (o índice único do SQLite diferencia maiúsculas), a
// conferência atualiza a primeira que a consulta lê, como o FirstOrDefault do C#. A consulta do EF e a do Go usam o
// índice único (ContaId, Nome) e leem na ordem dele, pelos bytes do nome: "EMERGÊNCIA" antes de "Emergência", mesmo
// gravada depois. As de outra conta não mudam.
func TestConferirAtualizaAPrimeiraDoMesmoNome(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta, outra := fab.Conta(), fab.Conta()
	fab.ReservaConferencia(outra, func(rc *testeapoio.ReservaConferencia) { rc.Nome = "EMERGÊNCIA" })
	fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) { rc.Nome = "Emergência" })
	fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) { rc.Nome = "EMERGÊNCIA" })
	s := NovoServico(db, &cambioFalso{t: t}, r, slog.New(slog.DiscardHandler))
	if err := s.Conferir(t.Context(), conta.Id, "emergência", testeapoio.Valor("7"), testeapoio.Dia(2026, 10, 29), nil); err != nil {
		t.Fatal(err)
	}
	esperado := []string{
		"2 | EMERGÊNCIA | 100.0 | 2026-10-30 | 2026-10-30 18:00:00",
		"1 | Emergência | 100.0 | 2026-10-30 | 2026-10-30 18:00:00",
		"1 | EMERGÊNCIA | 7.0 | 2026-10-29 | 2026-10-30 18:00:00",
	}
	if obtido := conferenciasDoBanco(t, db); strings.Join(obtido, "\n") != strings.Join(esperado, "\n") {
		t.Errorf("no banco:\n%s", strings.Join(obtido, "\n"))
	}
}

// Regra: tirar a conferência é o RemoverConferenciaAsync: o nome aparado, sem diferença de maiúsculas (inclusive
// com acento: o OrdinalIgnoreCase do C# põe "ê" e "Ê" juntos, o LIKE do SQLite não), todas as do nome, só da conta.
// False se não havia nenhuma.
func TestRemoverConferencia(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome    string
		reserva string
		removeu bool
		ficam   []string
	}{
		{"todas as do nome, sem diferença de maiúsculas", "  emergência\t", true,
			[]string{"1 | Casa", "2 | Emergência"}},
		{"só a do nome exato", "Casa", true, []string{"1 | Emergência", "1 | EMERGÊNCIA", "2 | Emergência"}},
		{"não havia", "Viagem", false, []string{"1 | Emergência", "1 | EMERGÊNCIA", "1 | Casa", "2 | Emergência"}},
		{"o nome por inteiro, não o começo", "Emerg", false,
			[]string{"1 | Emergência", "1 | EMERGÊNCIA", "1 | Casa", "2 | Emergência"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
			fab := testeapoio.NovaFabrica(t, db, r)
			conta, outra := fab.Conta(), fab.Conta()
			for _, nome := range []string{"Emergência", "EMERGÊNCIA", "Casa"} {
				fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) { rc.Nome = nome })
			}
			fab.ReservaConferencia(outra, func(rc *testeapoio.ReservaConferencia) { rc.Nome = "Emergência" })
			s := NovoServico(db, &cambioFalso{t: t}, r, slog.New(slog.DiscardHandler))
			removeu, err := s.RemoverConferencia(t.Context(), conta.Id, c.reserva)
			if err != nil || removeu != c.removeu {
				t.Fatalf("removeu %v, %v; esperado %v", removeu, err, c.removeu)
			}
			var ficaram []string
			for _, linha := range conferenciasDoBanco(t, db) {
				partes := strings.Split(linha, " | ")
				ficaram = append(ficaram, partes[0]+" | "+partes[1])
			}
			if strings.Join(ficaram, ",") != strings.Join(c.ficam, ",") {
				t.Errorf("ficaram %v, esperado %v", ficaram, c.ficam)
			}
		})
	}
}

// Regra: erro do banco sobe com o contexto da etapa, em cada operação.
func TestErroDoBanco(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	s := NovoServico(db, &cambioFalso{t: t}, r, slog.New(slog.DiscardHandler))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Listar(t.Context()); err == nil {
		t.Error("Listar sem erro")
	}
	if _, err := s.Total(t.Context()); err == nil {
		t.Error("Total sem erro")
	}
	if err := s.Conferir(t.Context(), 1, "Casa", testeapoio.Valor("1"), testeapoio.Dia(2026, 10, 1), nil); err == nil {
		t.Error("Conferir sem erro")
	}
	if _, err := s.RemoverConferencia(t.Context(), 1, "Casa"); err == nil {
		t.Error("RemoverConferencia sem erro")
	}
}
