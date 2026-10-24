package testeapoio

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Executor é o que a fábrica precisa para gravar: um *sql.DB ou uma *sql.Tx (a mesma interface do sqlc).
type Executor interface {
	ExecContext(ctx context.Context, consulta string, argumentos ...any) (sql.Result, error)
}

// Fabrica grava dados inventados no formato do EF Core (decimal "10.5", data "2026-10-22", data e hora UTC
// "2026-10-23 02:30:00", booleano 0/1), com todas as colunas preenchidas como o EF Core faz num INSERT, e não com
// o DEFAULT do banco.
//
// Cada método começa de um valor padrão sensato e aplica os ajustes do teste, para o teste mostrar só o que importa:
//
//	conta := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 3 }) // cartão de crédito
//
// Nomes que o banco exige únicos (Categoria.Nome) recebem um número sequencial da fábrica. Os carimbos (CriadoEm,
// AtualizadoEm) vêm do relógio dado. Os Ids de enum seguem o Entidades/Enums.cs do C# e estão no comentário de
// cada campo; o pacote do assunto tem os tipos dele, que este pacote não importa (ver README.md). Campo opcional
// se preenche com o new do Go 1.26: c.Banco = new("nubank").
type Fabrica struct {
	t       testing.TB
	db      Executor
	relogio relogio.Relogio
	numero  atomic.Int64
}

// NovaFabrica cria uma fábrica que grava em db, com os carimbos do relógio r.
func NovaFabrica(t testing.TB, db Executor, r relogio.Relogio) *Fabrica {
	return &Fabrica{t: t, db: db, relogio: r}
}

func (f *Fabrica) proximo() int64 { return f.numero.Add(1) }

func (f *Fabrica) agora() tipos.DataHora { return tipos.NovaDataHora(f.relogio.Agora()) }

// inserir roda o INSERT e devolve o Id gerado; falha o teste se o banco recusar.
func (f *Fabrica) inserir(tabela, consulta string, argumentos ...any) int64 {
	f.t.Helper()
	resultado, err := f.db.ExecContext(f.t.Context(), consulta, argumentos...)
	if err != nil {
		f.t.Fatalf("fábrica: gravar %s: %v", tabela, err)
	}
	id, err := resultado.LastInsertId()
	if err != nil {
		f.t.Fatalf("fábrica: Id de %s: %v", tabela, err)
	}
	return id
}

// bit é o bool como o EF Core grava no SQLite (0 ou 1).
func bit(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// bitOpcional é o bool? do C#: NULL, 0 ou 1.
func bitOpcional(b *bool) any {
	if b == nil {
		return nil
	}
	return bit(*b)
}

// Categoria é uma linha de Categoria (Entidades/Financeiro.cs).
type Categoria struct {
	Id             int64
	Nome           string
	Tipo           int64 // 1 Despesa, 2 Receita, 3 Transferência
	Cor            string
	Icone          *string
	CategoriaPaiId *int64
	Ativa          bool
	Natureza       *int64 // 1 Essencial, 2 Estilo de vida; nil = não definida
}

// Categoria grava uma categoria de despesa ativa, "Categoria de teste N".
func (f *Fabrica) Categoria(ajustes ...func(*Categoria)) Categoria {
	f.t.Helper()
	c := Categoria{Nome: fmt.Sprintf("Categoria de teste %d", f.proximo()), Tipo: 1, Cor: "#64748b", Ativa: true}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	c.Id = f.inserir("Categoria",
		`INSERT INTO Categoria (Nome, Tipo, Cor, Icone, CategoriaPaiId, Ativa, Natureza) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Nome, c.Tipo, c.Cor, c.Icone, c.CategoriaPaiId, bit(c.Ativa), c.Natureza)
	return c
}

// Subcategoria grava uma categoria filha de pai, com o mesmo tipo dele.
func (f *Fabrica) Subcategoria(pai Categoria, ajustes ...func(*Categoria)) Categoria {
	f.t.Helper()
	paiId := pai.Id
	return f.Categoria(append([]func(*Categoria){func(c *Categoria) {
		c.Nome = fmt.Sprintf("Subcategoria de teste %d", f.proximo())
		c.Tipo = pai.Tipo
		c.CategoriaPaiId = &paiId
	}}, ajustes...)...)
}

// Conta é uma linha de Conta (Entidades/Financeiro.cs).
type Conta struct {
	Id                      int64
	ConexaoBancariaId       *int64
	PluggyAccountId         *string
	Nome                    string
	Tipo                    int64 // 1 Conta corrente, 2 Poupança, 3 Cartão de crédito, 4 Investimento, 5 Dinheiro, 6 Outro
	Origem                  int64 // 1 Manual, 2 Open Finance
	Numero                  *string
	Moeda                   string
	SaldoInicial            tipos.Decimal
	Saldo                   tipos.Decimal
	LimiteCredito           *tipos.Decimal
	LimiteDisponivel        *tipos.Decimal
	LimiteAjustadoNoApp     *bool
	LimiteDiferencaAvaliada *tipos.Decimal
	DiaFechamento           *int64
	DiaVencimento           *int64
	Cor                     string
	IncluirNoSaldo          bool
	Arquivada               bool
	Banco                   *string
	ContaVinculadaId        *int64
	SaldoReservado          *tipos.Decimal
	ReservasInformadas      *string
	CriadoEm                tipos.DataHora
	AtualizadoEm            tipos.DataHora
}

// Conta grava uma conta corrente manual, em reais, com saldo zero, que entra no saldo.
func (f *Fabrica) Conta(ajustes ...func(*Conta)) Conta {
	f.t.Helper()
	agora := f.agora()
	c := Conta{
		Nome: fmt.Sprintf("Conta de teste %d", f.proximo()), Tipo: 1, Origem: 1, Moeda: "BRL",
		Cor: "#0f766e", IncluirNoSaldo: true, CriadoEm: agora, AtualizadoEm: agora,
	}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	c.Id = f.inserir("Conta", `INSERT INTO Conta (ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Numero, Moeda,
		SaldoInicial, Saldo, LimiteCredito, LimiteDisponivel, LimiteAjustadoNoApp, LimiteDiferencaAvaliada, DiaFechamento,
		DiaVencimento, Cor, IncluirNoSaldo, Arquivada, Banco, ContaVinculadaId, SaldoReservado, ReservasInformadas,
		CriadoEm, AtualizadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ConexaoBancariaId, c.PluggyAccountId, c.Nome, c.Tipo, c.Origem, c.Numero, c.Moeda,
		c.SaldoInicial, c.Saldo, c.LimiteCredito, c.LimiteDisponivel, bitOpcional(c.LimiteAjustadoNoApp),
		c.LimiteDiferencaAvaliada, c.DiaFechamento, c.DiaVencimento, c.Cor, bit(c.IncluirNoSaldo), bit(c.Arquivada),
		c.Banco, c.ContaVinculadaId, c.SaldoReservado, c.ReservasInformadas, c.CriadoEm, c.AtualizadoEm)
	return c
}

// Transacao é uma linha de Transacao (Entidades/Financeiro.cs). Valor negativo é saída, positivo é entrada.
type Transacao struct {
	Id                  int64
	ContaId             int64
	PluggyTransactionId *string
	Data                tipos.Data
	Descricao           string
	DescricaoOriginal   *string
	Estabelecimento     *string
	Valor               tipos.Decimal
	Tipo                int64 // 1 Despesa, 2 Receita, 3 Transferência
	Status              int64 // 1 Efetivada, 2 Pendente
	CategoriaId         *int64
	CategoriaManual     bool
	PluggyCategoriaId   *string
	FaturaCartaoId      *int64
	ParcelaNumero       *int64
	ParcelaTotal        *int64
	IgnorarRelatorios   bool
	Emergencia          bool
	DescricaoRevisada   bool
	MeioPagamento       *string
	MesmaTitularidade   *bool
	RegraDescricaoId    *int64
	DescricaoManual     bool
	TransferenciaGrupo  *tipos.Guid
	Observacao          *string
	CriadoEm            tipos.DataHora
	AtualizadoEm        tipos.DataHora
}

// Transacao grava uma despesa efetivada de R$ 10,00 (Valor -10.0) na conta dada, sem categoria, com a data de hoje
// em Brasília pelo relógio da fábrica.
func (f *Fabrica) Transacao(conta Conta, ajustes ...func(*Transacao)) Transacao {
	f.t.Helper()
	agora := f.agora()
	tr := Transacao{
		ContaId: conta.Id, Data: relogio.Hoje(f.relogio), Descricao: fmt.Sprintf("Transação de teste %d", f.proximo()),
		Valor: Valor("-10.00"), Tipo: 1, Status: 1, CriadoEm: agora, AtualizadoEm: agora,
	}
	for _, ajustar := range ajustes {
		ajustar(&tr)
	}
	tr.Id = f.inserir("Transacao", `INSERT INTO Transacao (ContaId, PluggyTransactionId, Data, Descricao,
		DescricaoOriginal, Estabelecimento, Valor, Tipo, Status, CategoriaId, CategoriaManual, PluggyCategoriaId,
		FaturaCartaoId, ParcelaNumero, ParcelaTotal, IgnorarRelatorios, Emergencia, DescricaoRevisada, MeioPagamento,
		MesmaTitularidade, RegraDescricaoId, DescricaoManual, TransferenciaGrupo, Observacao, CriadoEm, AtualizadoEm)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tr.ContaId, tr.PluggyTransactionId, tr.Data, tr.Descricao, tr.DescricaoOriginal, tr.Estabelecimento, tr.Valor,
		tr.Tipo, tr.Status, tr.CategoriaId, bit(tr.CategoriaManual), tr.PluggyCategoriaId, tr.FaturaCartaoId,
		tr.ParcelaNumero, tr.ParcelaTotal, bit(tr.IgnorarRelatorios), bit(tr.Emergencia), bit(tr.DescricaoRevisada),
		tr.MeioPagamento, bitOpcional(tr.MesmaTitularidade), tr.RegraDescricaoId, bit(tr.DescricaoManual),
		tr.TransferenciaGrupo, tr.Observacao, tr.CriadoEm, tr.AtualizadoEm)
	return tr
}

// RegraCategorizacao é uma linha de RegraCategorizacao (Entidades/Financeiro.cs).
type RegraCategorizacao struct {
	Id                  int64
	Texto               string
	TipoCorrespondencia int64 // 1 Contém, 2 Começa com, 3 Igual
	CategoriaId         int64
	Prioridade          int64
	Ativa               bool
	CriadoEm            tipos.DataHora
}

// RegraCategorizacao grava uma regra ativa "contém <texto>" para a categoria dada, com a prioridade padrão (100).
func (f *Fabrica) RegraCategorizacao(categoria Categoria, ajustes ...func(*RegraCategorizacao)) RegraCategorizacao {
	f.t.Helper()
	r := RegraCategorizacao{
		Texto: fmt.Sprintf("TEXTO DE TESTE %d", f.proximo()), TipoCorrespondencia: 1, CategoriaId: categoria.Id,
		Prioridade: 100, Ativa: true, CriadoEm: f.agora(),
	}
	for _, ajustar := range ajustes {
		ajustar(&r)
	}
	r.Id = f.inserir("RegraCategorizacao", `INSERT INTO RegraCategorizacao (Texto, TipoCorrespondencia, CategoriaId,
		Prioridade, Ativa, CriadoEm) VALUES (?, ?, ?, ?, ?, ?)`,
		r.Texto, r.TipoCorrespondencia, r.CategoriaId, r.Prioridade, bit(r.Ativa), r.CriadoEm)
	return r
}
