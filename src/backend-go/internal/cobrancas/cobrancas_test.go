package cobrancas

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

func ptr(s string) *string { return &s }

// Regra (#158, #69, #205): a conciliação dá baixa na conta a pagar cobrada sozinha quando a cobrança chega no extrato,
// na janela de 10 dias antes do vencimento até hoje, na conta dela (ou em qualquer uma, se ela não diz).
func TestConciliar(t *testing.T) {
	t.Parallel()
	hoje := testeapoio.Dia(2026, 10, 14)
	type esperado struct {
		mudou           bool
		ultimo, proximo string
		ativa           bool
		chave           string
	}
	casos := []struct {
		nome     string
		conta    func(*testeapoio.ContaAgendada, testeapoio.Conta)
		cobranca func(*testeapoio.Transacao)
		quer     esperado
	}{
		{"débito automático pago 3 dias antes",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) {},
			esperado{true, "2026-10-12", "2026-11-15", true, "ACADEMIA FORMA"}},
		{"à mão não concilia",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) { c.FormaCobranca = FormaAMao },
			func(tr *testeapoio.Transacao) {},
			esperado{false, "null", "2026-10-15", true, ""}},
		{"texto que não casa",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) { c.TextoCobranca = ptr("NATACAO") },
			func(tr *testeapoio.Transacao) {},
			esperado{false, "null", "2026-10-15", true, ""}},
		{"cobrança antes da janela",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) { tr.Data = testeapoio.Dia(2026, 10, 4) },
			esperado{false, "null", "2026-10-15", true, ""}},
		{"cobrança no primeiro dia da janela",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) { tr.Data = testeapoio.Dia(2026, 10, 5) },
			esperado{true, "2026-10-05", "2026-11-15", true, "ACADEMIA FORMA"}},
		{"estorno não é cobrança",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) { tr.Valor = testeapoio.Valor("89.9") },
			esperado{false, "null", "2026-10-15", true, ""}},
		{"cobrança de valor zero não é cobrança",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) { tr.Valor = testeapoio.Valor("0.0") },
			esperado{false, "null", "2026-10-15", true, ""}},
		{"cobrança depois de hoje",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) { tr.Data = testeapoio.Dia(2026, 10, 15) },
			esperado{false, "null", "2026-10-15", true, ""}},
		{"única é desativada",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) { c.Recorrencia = 0 },
			func(tr *testeapoio.Transacao) {},
			esperado{true, "2026-10-12", "2026-10-15", false, "ACADEMIA FORMA"}},
		{"passa da data limite e é encerrada",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {
				limite := testeapoio.Dia(2026, 11, 1)
				c.DataLimite = &limite
			},
			func(tr *testeapoio.Transacao) {},
			esperado{true, "2026-10-12", "2026-11-15", false, "ACADEMIA FORMA"}},
		{"a chave que já existe fica",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) { c.ChaveExtrato = ptr("OUTRA") },
			func(tr *testeapoio.Transacao) {},
			esperado{true, "2026-10-12", "2026-11-15", true, "OUTRA"}},
		{"vencimentos atrasados avançam até depois da cobrança",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {
				c.ProximoVencimento = testeapoio.Dia(2026, 8, 20)
			},
			func(tr *testeapoio.Transacao) { tr.Data = testeapoio.Dia(2026, 8, 18) },
			esperado{true, "2026-08-18", "2026-09-20", true, "ACADEMIA FORMA"}},
		{"o estabelecimento dá a chave",
			func(c *testeapoio.ContaAgendada, _ testeapoio.Conta) {},
			func(tr *testeapoio.Transacao) { tr.Estabelecimento = ptr("Academia Forma Física Ltda") },
			esperado{true, "2026-10-12", "2026-11-15", true, "ACADEMIA FORMA FISICA"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
			conta := fab.Conta()
			agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
				a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca = testeapoio.Dia(2026, 10, 15), 2, ptr("academia forma")
				a.ContaId = &conta.Id
				c.conta(a, conta)
			})
			fab.Transacao(conta, func(tr *testeapoio.Transacao) {
				tr.Data, tr.Valor = testeapoio.Dia(2026, 10, 12), testeapoio.Valor("-89.9")
				tr.Descricao, tr.DescricaoOriginal = "Minha academia", ptr("DEB AUT ACADEMIA FORMA")
				c.cobranca(tr)
			})

			mudaram, err := Conciliar(context.Background(), db, hoje)
			if err != nil {
				t.Fatal(err)
			}
			if (mudaram == 1) != c.quer.mudou {
				t.Errorf("mudaram: %d", mudaram)
			}
			var (
				ultimo, chave *string
				proximo       string
				ativa         bool
			)
			if err := db.QueryRow(`SELECT "UltimoPagamentoEm", "ProximoVencimento", "Ativa", "ChaveExtrato" FROM "ContaAgendada"
				WHERE "Id" = ?`, agendada.Id).Scan(&ultimo, &proximo, &ativa, &chave); err != nil {
				t.Fatal(err)
			}
			obtido := esperado{c.quer.mudou, "null", proximo, ativa, ""}
			if ultimo != nil {
				obtido.ultimo = *ultimo
			}
			if chave != nil {
				obtido.chave = *chave
			}
			if obtido != c.quer {
				t.Errorf("obtido %+v, quer %+v", obtido, c.quer)
			}
		})
	}
}

// Regra: a conta a pagar que não diz a conta casa a cobrança de qualquer conta; a que diz, só a da conta dela. Na
// segunda vez nada muda (já conciliada).
func TestConciliarPelaConta(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	mesma, outra := fab.Conta(), fab.Conta()
	naOutra := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca, a.ContaId = testeapoio.Dia(2026, 10, 15), 3, ptr("STREAMFLIX"), &mesma.Id
	})
	semConta := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca = testeapoio.Dia(2026, 10, 15), 3, ptr("STREAMFLIX")
	})
	fab.Transacao(outra, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 13), testeapoio.Valor("-39.9"), "ASSINATURA STREAMFLIX"
	})
	for i, quer := range []int{1, 0} {
		mudaram, err := Conciliar(context.Background(), db, testeapoio.Dia(2026, 10, 14))
		if err != nil || mudaram != quer {
			t.Fatalf("rodada %d: %d, %v", i, mudaram, err)
		}
	}
	var ultimoNaOutra, ultimoSemConta *string
	_ = db.QueryRow(`SELECT "UltimoPagamentoEm" FROM "ContaAgendada" WHERE "Id" = ?`, naOutra.Id).Scan(&ultimoNaOutra)
	_ = db.QueryRow(`SELECT "UltimoPagamentoEm" FROM "ContaAgendada" WHERE "Id" = ?`, semConta.Id).Scan(&ultimoSemConta)
	if ultimoNaOutra != nil || ultimoSemConta == nil || *ultimoSemConta != "2026-10-13" {
		t.Errorf("na outra conta: %v; sem conta: %v", ultimoNaOutra, ultimoSemConta)
	}
}

// Regra: das cobranças que casam, vale a mais recente; no empate de data, a primeira gravada.
func TestConciliarAMaisRecente(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conta := fab.Conta()
	agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca = testeapoio.Dia(2026, 10, 15), 2, ptr("LUZ")
	})
	for _, d := range []struct {
		dia             int
		estabelecimento string
	}{{8, "Luz Velha"}, {11, "Luz Primeira"}, {11, "Luz Segunda"}, {9, "Luz Antiga"}} {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, d.dia), testeapoio.Valor("-100.0"), "CONTA DE LUZ"
			tr.Estabelecimento = ptr(d.estabelecimento)
		})
	}
	if _, err := Conciliar(context.Background(), db, testeapoio.Dia(2026, 10, 14)); err != nil {
		t.Fatal(err)
	}
	var ultimo, chave string
	_ = db.QueryRow(`SELECT "UltimoPagamentoEm", "ChaveExtrato" FROM "ContaAgendada" WHERE "Id" = ?`, agendada.Id).Scan(&ultimo, &chave)
	if ultimo != "2026-10-11" || chave != "LUZ PRIMEIRA" {
		t.Errorf("último %s, chave %s", ultimo, chave)
	}
}

// Regra (#241): a transação com o nome de uma regra de descrição com faixa de valor é reconhecida por esse nome, e a
// chave é o nome inteiro (com o número).
func TestConciliarPelaRegraComValor(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conta := fab.Conta()
	var regra, semFaixa int64
	if err := db.QueryRow(`INSERT INTO "RegraDescricao" ("Texto", "TipoCorrespondencia", "NovaDescricao", "ValorMinimo",
		"Ativa", "CriadoEm") VALUES ('CONDOMINIO', 1, 'Condomínio 2', '500.0', 1, '2026-01-01 12:00:00') RETURNING "Id"`).Scan(&regra); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO "RegraDescricao" ("Texto", "TipoCorrespondencia", "NovaDescricao", "Ativa", "CriadoEm")
		VALUES ('PADARIA', 1, 'Padaria', 1, '2026-01-01 12:00:00') RETURNING "Id"`).Scan(&semFaixa); err != nil {
		t.Fatal(err)
	}
	regras, err := RegrasPorValor(context.Background(), db)
	if err != nil || len(regras) != 1 || !regras[regra] {
		t.Fatalf("regras por valor: %v, %v", regras, err)
	}
	agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca = testeapoio.Dia(2026, 10, 15), 2, ptr("CONDOMINIO 2")
	})
	fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor = testeapoio.Dia(2026, 10, 10), testeapoio.Valor("-700.0")
		tr.Descricao, tr.DescricaoOriginal, tr.RegraDescricaoId = "Condomínio 2", ptr("BOLETO CONDOMINIO"), &regra
	})
	if _, err := Conciliar(context.Background(), db, testeapoio.Dia(2026, 10, 14)); err != nil {
		t.Fatal(err)
	}
	var chave string
	_ = db.QueryRow(`SELECT "ChaveExtrato" FROM "ContaAgendada" WHERE "Id" = ?`, agendada.Id).Scan(&chave)
	if chave != "CONDOMINIO 2" {
		t.Errorf("chave: %q", chave)
	}
}

// Regra: o texto que a cobrança procura é a descrição do banco, menos quando o nome veio de regra com faixa de valor.
func TestTextoDaCobranca(t *testing.T) {
	t.Parallel()
	um, dois := int64(1), int64(2)
	porValor := map[int64]bool{1: true}
	casos := []struct {
		nome     string
		original *string
		regra    *int64
		quer     string
	}{
		{"sem original", nil, nil, "Meu nome"},
		{"com original", ptr("DEB AUT"), nil, "DEB AUT"},
		{"regra com faixa de valor", ptr("DEB AUT"), &um, "Meu nome"},
		{"regra sem faixa", ptr("DEB AUT"), &dois, "DEB AUT"},
	}
	for _, c := range casos {
		if obtido := TextoDaCobranca("Meu nome", c.original, c.regra, porValor); obtido != c.quer {
			t.Errorf("%s: %q", c.nome, obtido)
		}
	}
}

// Regra: a chave do extrato é a da sugestão de categorias; sem palavra significativa, o texto todo sem as partes que
// não são só letras; a da regra é o nome inteiro, normalizado e cortado em 150 chars.
func TestChaves(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		"Pagamento NETFLIX.COM assinatura": ChaveDe("Pagamento NETFLIX.COM assinatura"),
		"BOLETO F I A P":                   "BOLETO F I A P",
		"BOLETO F 1 A P2":                  "BOLETO F A",
		"":                                 "",
	}
	for entrada, quer := range casos {
		if obtido := ChaveDe(entrada); obtido != quer {
			t.Errorf("ChaveDe(%q) = %q, quer %q", entrada, obtido, quer)
		}
	}
	if obtido := ChaveDe("Pagamento NETFLIX.COM assinatura"); obtido == "" || obtido != strings.ToUpper(obtido) {
		t.Errorf("chave da sugestão: %q", obtido)
	}
	if obtido := ChaveDaRegra(" Condomínio 2 "); obtido != "CONDOMINIO 2" {
		t.Errorf("ChaveDaRegra: %q", obtido)
	}
	longo := strings.Repeat("A", 149) + " B" + strings.Repeat("C", 20)
	if obtido := ChaveDaRegra(longo); obtido != strings.Repeat("A", 149) {
		t.Errorf("ChaveDaRegra longa: %q (%d)", obtido, len(obtido))
	}
	if obtido := ChaveDaRegra(strings.Repeat("D", 150)); obtido != strings.Repeat("D", 150) {
		t.Errorf("ChaveDaRegra de 150: %d", len(obtido))
	}
}
