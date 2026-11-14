// Pacote emprego é o emprego e o salário com vigência do módulo Trabalho (#210, ADR 0065): quem paga, desde quando, o
// salário bruto de cada período e a estimativa do mês (bruto, INSS, IRRF e líquido), pelas tabelas da vigência.
//
// Nada daqui entra nos relatórios: a receita continua vindo só das transações (ADR 0065, "não contar duas vezes").
package emprego

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Regime do emprego. Estágio e PJ ficam sem cálculo de encargos.
const (
	RegimeClt     int64 = 1
	RegimeEstagio int64 = 2
	RegimePj      int64 = 3
)

// Erros que a pessoa resolve (viram 400, 404 ou 409 com a mensagem).
var (
	ErrNaoEncontrado = errors.New("emprego não encontrado")
	ErrInvalido      = errors.New("dados inválidos")
	ErrUltimoSalario = errors.New("o emprego precisa de pelo menos um salário")
	ErrSemTabela     = errors.New("sem tabela do INSS ou do IRRF para o mês")
)

// erroDeValidacao é o ErrInvalido com a mensagem para a pessoa.
type erroDeValidacao struct{ mensagem string }

func (e erroDeValidacao) Error() string { return e.mensagem }
func (e erroDeValidacao) Unwrap() error { return ErrInvalido }

func invalido(mensagem string) error { return erroDeValidacao{mensagem} }

// Salario é uma vigência: o salário bruto a partir de um mês (sempre o dia 1).
type Salario struct {
	Id           int64         `json:"id"`
	Inicio       tipos.Data    `json:"inicio"`
	SalarioBruto tipos.Decimal `json:"salarioBruto"`
}

// Emprego é um emprego com os salários, do mais novo para o mais antigo.
type Emprego struct {
	Id                 int64       `json:"id"`
	Empregador         string      `json:"empregador"`
	Cnpj               *string     `json:"cnpj"`
	Admissao           tipos.Data  `json:"admissao"`
	Saida              *tipos.Data `json:"saida"`
	Regime             int64       `json:"regime"`
	JornadaMensalHoras int64       `json:"jornadaMensalHoras"`
	Dependentes        int64       `json:"dependentes"`
	ContaId            *int64      `json:"contaId"`
	ContaNome          *string     `json:"contaNome"`
	Salarios           []Salario   `json:"salarios"`
}

// Dados é o que a pessoa preenche no emprego. SalarioInicial só vale ao criar (a primeira vigência, no mês da admissão).
type Dados struct {
	Empregador         string
	Cnpj               *string
	Admissao           tipos.Data
	Saida              *tipos.Data
	Regime             int64
	JornadaMensalHoras int64
	Dependentes        int64
	ContaId            *int64
	SalarioInicial     *tipos.Decimal
}

// Servico lê e grava os empregos.
type Servico struct {
	db      *sql.DB
	relogio relogio.Relogio
	log     *slog.Logger
}

func NovoServico(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log}
}

// emReais deixa o valor com duas casas (o banco guarda "5000.0", no formato do EF Core).
func emReais(v tipos.Decimal) tipos.Decimal { return tipos.DeDecimal(v.Decimal().Round(2)) }

// InicioDoMes é o dia 1 do mês da data.
func InicioDoMes(d tipos.Data) tipos.Data { return tipos.NovaData(d.Ano(), d.Mes(), 1) }

// validar confere e normaliza os dados (o CNPJ só com os dígitos). Erro com a mensagem para a pessoa.
func validar(d *Dados, criando bool) error {
	d.Empregador = strings.TrimSpace(d.Empregador)
	switch {
	case d.Empregador == "":
		return invalido("Informe o empregador.")
	case len([]rune(d.Empregador)) > 150:
		return invalido("O nome do empregador vai até 150 caracteres.")
	case d.Regime < RegimeClt || d.Regime > RegimePj:
		return invalido("Escolha o regime: CLT, estágio ou PJ.")
	case d.JornadaMensalHoras < 1 || d.JornadaMensalHoras > 300:
		return invalido("A jornada mensal vai de 1 a 300 horas (a da CLT é 220).")
	case d.Dependentes < 0 || d.Dependentes > 20:
		return invalido("Os dependentes vão de 0 a 20.")
	case d.Saida != nil && d.Saida.Antes(d.Admissao):
		return invalido("A saída não pode ser antes da admissão.")
	}
	if d.Cnpj != nil {
		digitos := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, *d.Cnpj)
		switch {
		case digitos == "":
			d.Cnpj = nil
		case len(digitos) != 14:
			return invalido("O CNPJ tem 14 dígitos.")
		default:
			d.Cnpj = &digitos
		}
	}
	if criando {
		if d.SalarioInicial == nil || !d.SalarioInicial.Decimal().IsPositive() {
			return invalido("Informe o salário bruto.")
		}
	}
	return nil
}

func validarSalario(valor tipos.Decimal) error {
	if !valor.Decimal().IsPositive() {
		return invalido("O salário bruto tem de ser maior que zero.")
	}
	if valor.Decimal().GreaterThan(decimal.NewFromInt(10_000_000)) {
		return invalido("O salário bruto vai até R$ 10.000.000,00.")
	}
	return nil
}

func (s *Servico) conferirConta(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, contaId *int64) error {
	if contaId == nil {
		return nil
	}
	var existe bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Conta" WHERE "Id" = ?)`, *contaId).Scan(&existe); err != nil {
		return fmt.Errorf("conferir a conta: %w", err)
	}
	if !existe {
		return invalido("A conta escolhida não existe mais.")
	}
	return nil
}

// Criar grava o emprego e a primeira vigência do salário, no mês da admissão.
func (s *Servico) Criar(ctx context.Context, d Dados) (Emprego, error) {
	if err := validar(&d, true); err != nil {
		return Emprego{}, err
	}
	if err := validarSalario(*d.SalarioInicial); err != nil {
		return Emprego{}, err
	}
	var id int64
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.conferirConta(ctx, tx, d.ContaId); err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		r, err := tx.ExecContext(ctx, `INSERT INTO "Emprego" ("Empregador", "Cnpj", "Admissao", "Saida", "Regime",
			"JornadaMensalHoras", "Dependentes", "ContaId", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.Empregador, d.Cnpj, d.Admissao, d.Saida, d.Regime, d.JornadaMensalHoras, d.Dependentes, d.ContaId, agora)
		if err != nil {
			return fmt.Errorf("gravar o emprego: %w", err)
		}
		if id, err = r.LastInsertId(); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO "SalarioVigencia" ("EmpregoId", "Inicio", "SalarioBruto", "CriadoEm")
			VALUES (?, ?, ?, ?)`, id, InicioDoMes(d.Admissao), *d.SalarioInicial, agora)
		if err != nil {
			return fmt.Errorf("gravar o salário: %w", err)
		}
		return nil
	})
	if err != nil {
		return Emprego{}, err
	}
	return s.Obter(ctx, id)
}

// Atualizar troca os dados do emprego (o salário muda pelas vigências).
func (s *Servico) Atualizar(ctx context.Context, id int64, d Dados) (Emprego, error) {
	if err := validar(&d, false); err != nil {
		return Emprego{}, err
	}
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.conferirConta(ctx, tx, d.ContaId); err != nil {
			return err
		}
		var primeiro *tipos.Data
		err := tx.QueryRowContext(ctx, `SELECT (SELECT MIN("Inicio") FROM "SalarioVigencia" WHERE "EmpregoId" = e."Id")
			FROM "Emprego" e WHERE e."Id" = ?`, id).Scan(&primeiro)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNaoEncontrado
		}
		if err != nil {
			return fmt.Errorf("ler o primeiro salário: %w", err)
		}
		if primeiro != nil && InicioDoMes(d.Admissao).Depois(*primeiro) {
			return invalido("Há salário antes dessa admissão. Apague o salário antigo ou escolha uma admissão até " +
				nomeDoMes(*primeiro) + ".")
		}
		r, err := tx.ExecContext(ctx, `UPDATE "Emprego" SET "Empregador" = ?, "Cnpj" = ?, "Admissao" = ?, "Saida" = ?,
			"Regime" = ?, "JornadaMensalHoras" = ?, "Dependentes" = ?, "ContaId" = ? WHERE "Id" = ?`,
			d.Empregador, d.Cnpj, d.Admissao, d.Saida, d.Regime, d.JornadaMensalHoras, d.Dependentes, d.ContaId, id)
		if err != nil {
			return fmt.Errorf("atualizar o emprego: %w", err)
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return ErrNaoEncontrado
		}
		return nil
	})
	if err != nil {
		return Emprego{}, err
	}
	return s.Obter(ctx, id)
}

// Excluir apaga o emprego e os salários dele.
func (s *Servico) Excluir(ctx context.Context, id int64) error {
	r, err := s.db.ExecContext(ctx, `DELETE FROM "Emprego" WHERE "Id" = ?`, id)
	if err != nil {
		return fmt.Errorf("excluir o emprego: %w", err)
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// DefinirSalario grava o salário a partir do mês de inicio (o dia vira 1): um aumento é uma vigência nova; o mesmo mês
// troca o valor. O mês não pode ser antes da admissão nem depois da saída.
func (s *Servico) DefinirSalario(ctx context.Context, empregoId int64, inicio tipos.Data, valor tipos.Decimal) (Emprego, error) {
	if err := validarSalario(valor); err != nil {
		return Emprego{}, err
	}
	inicio = InicioDoMes(inicio)
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var admissao tipos.Data
		var saida *tipos.Data
		err := tx.QueryRowContext(ctx, `SELECT "Admissao", "Saida" FROM "Emprego" WHERE "Id" = ?`, empregoId).Scan(&admissao, &saida)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNaoEncontrado
		}
		if err != nil {
			return fmt.Errorf("ler o emprego: %w", err)
		}
		if inicio.Antes(InicioDoMes(admissao)) {
			return invalido("O salário começa no mês da admissão (" + nomeDoMes(admissao) + ") ou depois.")
		}
		if saida != nil && inicio.Depois(*saida) {
			return invalido("O salário não pode começar depois da saída (" + nomeDoMes(*saida) + ").")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO "SalarioVigencia" ("EmpregoId", "Inicio", "SalarioBruto", "CriadoEm")
			VALUES (?, ?, ?, ?) ON CONFLICT ("EmpregoId", "Inicio") DO UPDATE SET "SalarioBruto" = excluded."SalarioBruto"`,
			empregoId, inicio, valor, tipos.NovaDataHora(s.relogio.Agora()))
		if err != nil {
			return fmt.Errorf("gravar o salário: %w", err)
		}
		return nil
	})
	if err != nil {
		return Emprego{}, err
	}
	return s.Obter(ctx, empregoId)
}

// ExcluirSalario apaga uma vigência. A última que sobrou não sai (o emprego precisa de um salário).
func (s *Servico) ExcluirSalario(ctx context.Context, empregoId, salarioId int64) (Emprego, error) {
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "SalarioVigencia" WHERE "EmpregoId" = ?`, empregoId).Scan(&n); err != nil {
			return fmt.Errorf("contar os salários: %w", err)
		}
		r, err := tx.ExecContext(ctx, `DELETE FROM "SalarioVigencia" WHERE "Id" = ? AND "EmpregoId" = ?`, salarioId, empregoId)
		if err != nil {
			return fmt.Errorf("excluir o salário: %w", err)
		}
		if apagadas, _ := r.RowsAffected(); apagadas == 0 {
			return ErrNaoEncontrado
		}
		if n <= 1 {
			return ErrUltimoSalario
		}
		return nil
	})
	if err != nil {
		return Emprego{}, err
	}
	return s.Obter(ctx, empregoId)
}

const selecaoEmprego = `SELECT e."Id", e."Empregador", e."Cnpj", e."Admissao", e."Saida", e."Regime",
	e."JornadaMensalHoras", e."Dependentes", e."ContaId", c."Nome"
	FROM "Emprego" e LEFT JOIN "Conta" c ON c."Id" = e."ContaId"`

// Listar devolve os empregos: os atuais (sem saída) primeiro, do mais novo para o mais antigo.
func (s *Servico) Listar(ctx context.Context) ([]Emprego, error) {
	linhas, err := s.db.QueryContext(ctx, selecaoEmprego+` ORDER BY e."Saida" IS NOT NULL, e."Admissao" DESC, e."Id" DESC`)
	if err != nil {
		return nil, fmt.Errorf("listar os empregos: %w", err)
	}
	defer linhas.Close()
	lista := []Emprego{}
	for linhas.Next() {
		e, err := lerEmprego(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, e)
	}
	if err := linhas.Err(); err != nil {
		return nil, err
	}
	for i := range lista {
		if lista[i].Salarios, err = s.salarios(ctx, lista[i].Id); err != nil {
			return nil, err
		}
	}
	return lista, nil
}

// Obter devolve um emprego, com os salários.
func (s *Servico) Obter(ctx context.Context, id int64) (Emprego, error) {
	e, err := lerEmprego(s.db.QueryRowContext(ctx, selecaoEmprego+` WHERE e."Id" = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Emprego{}, ErrNaoEncontrado
	}
	if err != nil {
		return Emprego{}, err
	}
	e.Salarios, err = s.salarios(ctx, id)
	return e, err
}

func lerEmprego(l interface{ Scan(...any) error }) (Emprego, error) {
	var e Emprego
	err := l.Scan(&e.Id, &e.Empregador, &e.Cnpj, &e.Admissao, &e.Saida, &e.Regime, &e.JornadaMensalHoras, &e.Dependentes,
		&e.ContaId, &e.ContaNome)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return e, fmt.Errorf("ler o emprego: %w", err)
	}
	return e, err
}

func (s *Servico) salarios(ctx context.Context, empregoId int64) ([]Salario, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Inicio", "SalarioBruto" FROM "SalarioVigencia"
		WHERE "EmpregoId" = ? ORDER BY "Inicio" DESC`, empregoId)
	if err != nil {
		return nil, fmt.Errorf("ler os salários: %w", err)
	}
	defer linhas.Close()
	lista := []Salario{}
	for linhas.Next() {
		var v Salario
		if err := linhas.Scan(&v.Id, &v.Inicio, &v.SalarioBruto); err != nil {
			return nil, fmt.Errorf("ler salário: %w", err)
		}
		v.SalarioBruto = emReais(v.SalarioBruto)
		lista = append(lista, v)
	}
	return lista, linhas.Err()
}

// SalarioDoMes é o salário da última vigência que começou até o mês (os salários do mais novo para o mais antigo).
func SalarioDoMes(salarios []Salario, mes tipos.Data) (Salario, bool) {
	mes = InicioDoMes(mes)
	for _, v := range salarios {
		if !v.Inicio.Depois(mes) {
			return v, true
		}
	}
	return Salario{}, false
}

var meses = [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro",
	"outubro", "novembro", "dezembro"}

// nomeDoMes é "março de 2026".
func nomeDoMes(d tipos.Data) string { return fmt.Sprintf("%s de %d", meses[d.Mes()-1], d.Ano()) }

// Estimativa é o mês do emprego: o salário bruto da vigência, o INSS, o IRRF e o líquido, com como chegou neles.
type Estimativa struct {
	Mes          tipos.Data     `json:"mes"`
	SalarioBruto tipos.Decimal  `json:"salarioBruto"`
	VigenciaDe   tipos.Data     `json:"vigenciaDe"`  // desde quando vale esse salário
	SemEncargos  bool           `json:"semEncargos"` // estágio ou PJ: sem INSS nem IRRF calculados
	Inss         *encargos.Inss `json:"inss"`
	Irrf         *encargos.Irrf `json:"irrf"`
	Liquido      tipos.Decimal  `json:"liquido"`
}

// Estimar calcula o mês do emprego. Mês fora do emprego (antes da admissão ou depois da saída) é inválido; mês sem
// tabela do INSS ou do IRRF é ErrSemTabela.
func (s *Servico) Estimar(ctx context.Context, id int64, mes tipos.Data) (Estimativa, error) {
	e, err := s.Obter(ctx, id)
	if err != nil {
		return Estimativa{}, err
	}
	mes = InicioDoMes(mes)
	if mes.Antes(InicioDoMes(e.Admissao)) || (e.Saida != nil && mes.Depois(*e.Saida)) {
		return Estimativa{}, invalido("Escolha um mês em que o emprego estava ativo.")
	}
	salario, ok := SalarioDoMes(e.Salarios, mes)
	if !ok {
		return Estimativa{}, invalido("Não há salário para esse mês.")
	}
	est := Estimativa{Mes: mes, SalarioBruto: salario.SalarioBruto, VigenciaDe: salario.Inicio, Liquido: salario.SalarioBruto}
	if e.Regime != RegimeClt {
		est.SemEncargos = true
		return est, nil
	}
	tabelasInss, err := encargos.LerTabelasInss(ctx, s.db)
	if err != nil {
		return Estimativa{}, err
	}
	tabelasIrrf, err := encargos.LerTabelasIrrf(ctx, s.db)
	if err != nil {
		return Estimativa{}, err
	}
	tInss, okInss := encargos.Vigente(tabelasInss, mes)
	tIrrf, okIrrf := encargos.Vigente(tabelasIrrf, mes)
	if !okInss || !okIrrf {
		return Estimativa{}, ErrSemTabela
	}
	inss := encargos.CalcularInss(tInss, salario.SalarioBruto)
	irrf := encargos.CalcularIrrf(tIrrf, salario.SalarioBruto, inss.Valor, int(e.Dependentes))
	est.Inss, est.Irrf = &inss, &irrf
	est.Liquido = tipos.DeDecimal(salario.SalarioBruto.Decimal().Sub(inss.Valor.Decimal()).Sub(irrf.Valor.Decimal()).Round(2))
	return est, nil
}

// Tabelas são as vigências do INSS e do IRRF, para a tela mostrar (com a fonte).
type Tabelas struct {
	Inss []encargos.TabelaInss `json:"inss"`
	Irrf []encargos.TabelaIrrf `json:"irrf"`
}

// LerTabelas devolve as tabelas de todas as vigências.
func (s *Servico) LerTabelas(ctx context.Context) (Tabelas, error) {
	inss, err := encargos.LerTabelasInss(ctx, s.db)
	if err != nil {
		return Tabelas{}, err
	}
	irrf, err := encargos.LerTabelasIrrf(ctx, s.db)
	if err != nil {
		return Tabelas{}, err
	}
	if inss == nil {
		inss = []encargos.TabelaInss{}
	}
	if irrf == nil {
		irrf = []encargos.TabelaIrrf{}
	}
	return Tabelas{Inss: inss, Irrf: irrf}, nil
}
