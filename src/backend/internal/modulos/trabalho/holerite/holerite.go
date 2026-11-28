package holerite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Situação do holerite.
const (
	Aberto  int64 = 1
	Fechado int64 = 2
)

// Origem do holerite: calculado pelo site (#211) ou lido do PDF (#228: fechado, com o que o PDF imprime).
const (
	Calculado int64 = 1
	LidoDoPdf int64 = 2
)

// EventoReaberto é o da auditoria ao reabrir um holerite fechado ("HoleriteReaberto (2026-10)").
const EventoReaberto = "HoleriteReaberto"

// Erros que a pessoa resolve.
var (
	ErrNaoEncontrado = errors.New("holerite não encontrado")
	ErrFechado       = errors.New("holerite fechado")
	ErrAberto        = errors.New("holerite aberto")
	ErrJaExiste      = errors.New("já existe holerite do mês")
	ErrSemSalario    = errors.New("sem salário no mês")
	ErrForaDoEmprego = errors.New("mês fora do emprego")
	ErrNaoELido      = errors.New("o holerite não é mais o lido do PDF")
	// ErrReabrirSemCalculo: reaberto, o holerite ficaria sem o cálculo (um dos semCalculo) e perderia o congelado, que
	// nem o Fechar devolve depois (#750). Vem embrulhado com o erro do cálculo.
	ErrReabrirSemCalculo = errors.New("reaberto, o holerite ficaria sem o cálculo")
	// ErrInvalido é a família dos erros de validação, para quem chama de fora do pacote (a mensagem é o Error()).
	ErrInvalido = errors.New("dados inválidos")
)

// semCalculo são os erros que deixam o holerite sem o cálculo: o holerite existe, e a lista e a tela continuam
// funcionando (revisão do #699). Lidos com errors.Is, e não como chave de mapa (#750): o erro embrulhado também vale, e um
// de tipo não comparável não faz o mapa entrar em pânico.
var semCalculo = []struct {
	err    error
	aviso  string // na tela do holerite aberto e no "Não dá para fechar"
	motivo string // no "Não dá para reabrir"
}{
	{encargos.ErrSemTabela, "Ainda não há tabela do INSS ou do IRRF para esse mês: o holerite fica sem o cálculo.",
		"ainda não há tabela do INSS ou do IRRF para esse mês."},
	{ErrSemSalario, "Não há salário para esse mês: cadastre em Trabalho › Salário. O holerite fica sem o cálculo.",
		"não há salário para esse mês. Cadastre em Trabalho › Salário e reabra."},
	{ErrForaDoEmprego, "Esse mês é de antes da admissão ou depois da saída: o holerite fica sem o cálculo. Exclua-o ou corrija o emprego.",
		"esse mês ficou antes da admissão ou depois da saída. Corrija o emprego e reabra."},
}

// avisoDe diz se o erro deixa o holerite sem o cálculo e, se deixa, o aviso e o motivo.
func avisoDe(err error) (aviso, motivo string, ok bool) {
	for _, s := range semCalculo {
		if errors.Is(err, s.err) {
			return s.aviso, s.motivo, true
		}
	}
	return "", "", false
}

type erroDeValidacao struct{ mensagem string }

func (e erroDeValidacao) Error() string { return e.mensagem }
func (e erroDeValidacao) Unwrap() error { return ErrInvalido }

func invalido(mensagem string) error { return erroDeValidacao{mensagem} }

// Holerite é o holerite de um mês, com os lançamentos e o cálculo (o congelado, se fechado).
type Holerite struct {
	Id           int64             `json:"id"`
	EmpregoId    int64             `json:"empregoId"`
	Mes          tipos.Data        `json:"mes"`
	Situacao     int64             `json:"situacao"`
	Origem       int64             `json:"origem"`
	DiasUteis    int64             `json:"diasUteis"`
	DiasDescanso int64             `json:"diasDescanso"`
	FechadoEm    *tipos.DataHora   `json:"fechadoEm"`
	Lancamentos  []LancamentoSalvo `json:"lancamentos"`
	Calculo      *Calculo          `json:"calculo"`
	// Aviso: não deu para calcular (sem tabela, sem salário, mês fora do emprego). O holerite existe, sem o cálculo.
	Aviso *string `json:"aviso"`
	// semCalculo é o porquê do Aviso, para o Fechar recusar com a mensagem certa.
	semCalculo error
}

// LancamentoSalvo é o lançamento com o id.
type LancamentoSalvo struct {
	Id int64 `json:"id"`
	Lancamento
}

// Resumo é a linha da lista de holerites.
type Resumo struct {
	Id             int64          `json:"id"`
	Mes            tipos.Data     `json:"mes"`
	Situacao       int64          `json:"situacao"`
	Origem         int64          `json:"origem"`
	TotalProventos *tipos.Decimal `json:"totalProventos"`
	TotalDescontos *tipos.Decimal `json:"totalDescontos"`
	Liquido        *tipos.Decimal `json:"liquido"`
}

// Servico lê, grava e calcula os holerites.
type Servico struct {
	db       *sql.DB
	empregos *emprego.Servico
	relogio  relogio.Relogio
	log      *slog.Logger
}

func NovoServico(db *sql.DB, empregos *emprego.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, empregos: empregos, relogio: r, log: log}
}

var meses = [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro",
	"outubro", "novembro", "dezembro"}

// NomeDoMes é "outubro de 2026".
func NomeDoMes(d tipos.Data) string { return fmt.Sprintf("%s de %d", meses[d.Mes()-1], d.Ano()) }

// Criar abre o holerite do mês, com os dias do DSR do calendário e os lançamentos recorrentes do último holerite antes
// dele (o plano de saúde do mês passado, por exemplo).
func (s *Servico) Criar(ctx context.Context, empregoId int64, mes tipos.Data) (Holerite, error) {
	mes = emprego.InicioDoMes(mes)
	e, err := s.empregos.Obter(ctx, empregoId)
	if errors.Is(err, emprego.ErrNaoEncontrado) {
		return Holerite{}, ErrNaoEncontrado
	}
	if err != nil {
		return Holerite{}, err
	}
	if DiasTrabalhados(mes, e.Admissao, e.Saida) == 0 {
		return Holerite{}, invalido("Escolha um mês em que o emprego estava ativo.")
	}
	if _, ok := emprego.SalarioDoMes(e.Salarios, mes); !ok {
		return Holerite{}, invalido("Não há salário para esse mês: cadastre em Trabalho › Salário antes de abrir o holerite.")
	}
	uteis, descanso := DiasDoDsr(mes)
	var id int64
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var existe bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Holerite" WHERE "EmpregoId" = ? AND "Mes" = ?)`,
			empregoId, mes).Scan(&existe); err != nil {
			return fmt.Errorf("conferir o holerite do mês: %w", err)
		}
		if existe {
			return ErrJaExiste
		}
		r, err := tx.ExecContext(ctx, `INSERT INTO "Holerite" ("EmpregoId", "Mes", "Situacao", "DiasUteis", "DiasDescanso", "CriadoEm")
			VALUES (?, ?, ?, ?, ?, ?)`, empregoId, mes, Aberto, uteis, descanso, tipos.NovaDataHora(s.relogio.Agora()))
		if err != nil {
			return fmt.Errorf("criar o holerite: %w", err)
		}
		if id, err = r.LastInsertId(); err != nil {
			return err
		}
		var anterior int64
		err = tx.QueryRowContext(ctx, `SELECT "Id" FROM "Holerite" WHERE "EmpregoId" = ? AND "Mes" < ? ORDER BY "Mes" DESC LIMIT 1`,
			empregoId, mes).Scan(&anterior)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("achar o holerite anterior: %w", err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO "HoleriteLancamento" ("HoleriteId", "Tipo", "Descricao", "Quantidade",
			"Percentual", "Valor", "IncideInss", "IncideIrrf", "DeduzIrrf", "Recorrente")
			SELECT ?, "Tipo", "Descricao", "Quantidade", "Percentual", "Valor", "IncideInss", "IncideIrrf", "DeduzIrrf", "Recorrente"
			FROM "HoleriteLancamento" WHERE "HoleriteId" = ? AND "Recorrente" = 1 ORDER BY "Id"`, id, anterior)
		if err != nil {
			return fmt.Errorf("trazer os lançamentos recorrentes: %w", err)
		}
		return nil
	})
	if err != nil {
		return Holerite{}, err
	}
	return s.Obter(ctx, id)
}

// Obter devolve o holerite com o cálculo: o de agora, se aberto; o guardado, se fechado.
func (s *Servico) Obter(ctx context.Context, id int64) (Holerite, error) {
	h, calculo, err := s.ler(ctx, s.db, id)
	if err != nil {
		return Holerite{}, err
	}
	if calculo != nil { // só o fechado guarda o cálculo
		var c Calculo
		if err := json.Unmarshal([]byte(*calculo), &c); err != nil {
			return Holerite{}, fmt.Errorf("ler o holerite fechado: %w", err)
		}
		h.Calculo = &c
		return h, nil
	}
	c, err := s.calcular(ctx, h)
	if aviso, _, ok := avisoDe(err); ok {
		h.Aviso, h.semCalculo = &aviso, err
		return h, nil
	}
	if err != nil {
		return Holerite{}, err
	}
	h.Calculo = &c
	return h, nil
}

type consultor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Servico) ler(ctx context.Context, q consultor, id int64) (Holerite, *string, error) {
	var h Holerite
	var calculo *string
	err := q.QueryRowContext(ctx, `SELECT "Id", "EmpregoId", "Mes", "Situacao", "Origem", "DiasUteis", "DiasDescanso", "FechadoEm",
		"Calculo" FROM "Holerite" WHERE "Id" = ?`, id).Scan(&h.Id, &h.EmpregoId, &h.Mes, &h.Situacao, &h.Origem, &h.DiasUteis,
		&h.DiasDescanso, &h.FechadoEm, &calculo)
	if errors.Is(err, sql.ErrNoRows) {
		return h, nil, ErrNaoEncontrado
	}
	if err != nil {
		return h, nil, fmt.Errorf("ler o holerite: %w", err)
	}
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Tipo", "Descricao", "Quantidade", "Percentual", "Valor", "IncideInss",
		"IncideIrrf", "DeduzIrrf", "Recorrente" FROM "HoleriteLancamento" WHERE "HoleriteId" = ? ORDER BY "Id"`, id)
	if err != nil {
		return h, nil, fmt.Errorf("ler os lançamentos: %w", err)
	}
	defer linhas.Close()
	h.Lancamentos = []LancamentoSalvo{}
	for linhas.Next() {
		var l LancamentoSalvo
		if err := linhas.Scan(&l.Id, &l.Tipo, &l.Descricao, &l.Quantidade, &l.Percentual, &l.Valor, &l.IncideInss, &l.IncideIrrf,
			&l.DeduzIrrf, &l.Recorrente); err != nil {
			return h, nil, fmt.Errorf("ler lançamento: %w", err)
		}
		h.Lancamentos = append(h.Lancamentos, l)
	}
	return h, calculo, linhas.Err()
}

// calcular faz o cálculo do holerite com o salário e as tabelas do mês.
func (s *Servico) calcular(ctx context.Context, h Holerite) (Calculo, error) {
	e, err := s.empregos.Obter(ctx, h.EmpregoId)
	if err != nil {
		return Calculo{}, err
	}
	if DiasTrabalhados(h.Mes, e.Admissao, e.Saida) == 0 {
		return Calculo{}, ErrForaDoEmprego
	}
	salario, ok := emprego.SalarioDoMes(e.Salarios, h.Mes)
	if !ok {
		return Calculo{}, ErrSemSalario
	}
	entrada := Entrada{Mes: h.Mes, Admissao: e.Admissao, Saida: e.Saida, SalarioContratual: salario.SalarioBruto,
		JornadaHoras: e.JornadaMensalHoras, Dependentes: e.Dependentes, SemEncargos: e.Regime != emprego.RegimeClt,
		DiasUteis: h.DiasUteis, DiasDescanso: h.DiasDescanso}
	for _, l := range h.Lancamentos {
		entrada.Lancamentos = append(entrada.Lancamentos, l.Lancamento)
	}
	if !entrada.SemEncargos {
		var err error
		if entrada.Inss, entrada.Irrf, err = encargos.TabelasDoMes(ctx, s.db, h.Mes); err != nil {
			return Calculo{}, err
		}
	}
	return Calcular(entrada), nil
}

// Listar devolve os holerites do emprego, do mês mais novo para o mais antigo, com os totais.
func (s *Servico) Listar(ctx context.Context, empregoId int64) ([]Resumo, error) {
	if _, err := s.empregos.Obter(ctx, empregoId); errors.Is(err, emprego.ErrNaoEncontrado) {
		return nil, ErrNaoEncontrado
	} else if err != nil {
		return nil, err
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id" FROM "Holerite" WHERE "EmpregoId" = ? ORDER BY "Mes" DESC`, empregoId)
	if err != nil {
		return nil, fmt.Errorf("listar os holerites: %w", err)
	}
	var ids []int64
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			linhas.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	linhas.Close()
	lista := []Resumo{}
	for _, id := range ids {
		h, err := s.Obter(ctx, id)
		if err != nil {
			return nil, err
		}
		r := Resumo{Id: h.Id, Mes: h.Mes, Situacao: h.Situacao, Origem: h.Origem}
		if h.Calculo != nil {
			r.TotalProventos, r.TotalDescontos, r.Liquido = &h.Calculo.TotalProventos, &h.Calculo.TotalDescontos, &h.Calculo.Liquido
		}
		lista = append(lista, r)
	}
	return lista, nil
}

// validarLancamento confere e normaliza o lançamento (a descrição, as marcas que valem para o tipo).
func validarLancamento(l *Lancamento) error {
	l.Descricao = strings.TrimSpace(l.Descricao)
	cem, milhoes := decimal.NewFromInt(300), decimal.NewFromInt(10_000_000)
	switch {
	case !TipoConhecido(l.Tipo):
		return invalido("Escolha o tipo de cada lançamento.")
	case len([]rune(l.Descricao)) > 100:
		return invalido("A descrição vai até 100 caracteres.")
	}
	if EhPorHora(l.Tipo) {
		if l.Quantidade == nil || !l.Quantidade.Decimal().IsPositive() || l.Quantidade.Decimal().GreaterThan(cem) {
			return invalido("As horas vão de 0,01 a 300.")
		}
		if l.Tipo == AdicionalNoturno && l.Percentual == nil {
			p := tipos.DeDecimal(decimal.NewFromInt(20))
			l.Percentual = &p
		}
		if l.Percentual == nil || !l.Percentual.Decimal().IsPositive() || l.Percentual.Decimal().GreaterThan(cem) {
			return invalido("O percentual vai de 0,01% a 300%.")
		}
		l.Valor, l.IncideInss, l.IncideIrrf, l.DeduzIrrf = nil, true, true, false
		return nil
	}
	if l.Valor == nil && l.Tipo == ValeTransporte {
		l.Quantidade, l.Percentual = nil, nil
		l.IncideInss, l.IncideIrrf, l.DeduzIrrf = false, false, false
		return nil // sem o custo, desconta os 6%
	}
	if l.Valor == nil || !l.Valor.Decimal().IsPositive() || l.Valor.Decimal().GreaterThan(milhoes) {
		return invalido("O valor vai de R$ 0,01 a R$ 10.000.000,00.")
	}
	l.Quantidade, l.Percentual = nil, nil
	if l.Tipo == OutroProvento {
		l.DeduzIrrf = false
	} else {
		l.IncideInss, l.IncideIrrf = false, false
		l.DeduzIrrf = l.DeduzIrrf || l.Tipo == PensaoAlimenticia
	}
	return nil
}

// Salvar troca os dias do DSR e os lançamentos do holerite aberto. Fechado, ErrFechado (reabra antes).
func (s *Servico) Salvar(ctx context.Context, id, diasUteis, diasDescanso int64, lancamentos []Lancamento) (Holerite, error) {
	if diasUteis < 1 || diasDescanso < 0 || diasUteis+diasDescanso > 31 {
		return Holerite{}, invalido("Os dias úteis e os de descanso somam no máximo 31.")
	}
	if len(lancamentos) > 100 {
		return Holerite{}, invalido("O holerite vai até 100 lançamentos.")
	}
	for i := range lancamentos {
		if err := validarLancamento(&lancamentos[i]); err != nil {
			return Holerite{}, err
		}
	}
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var situacao int64
		err := tx.QueryRowContext(ctx, `SELECT "Situacao" FROM "Holerite" WHERE "Id" = ?`, id).Scan(&situacao)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNaoEncontrado
		}
		if err != nil {
			return fmt.Errorf("ler o holerite: %w", err)
		}
		if situacao == Fechado {
			return ErrFechado
		}
		if _, err := tx.ExecContext(ctx, `UPDATE "Holerite" SET "DiasUteis" = ?, "DiasDescanso" = ? WHERE "Id" = ?`,
			diasUteis, diasDescanso, id); err != nil {
			return fmt.Errorf("gravar os dias: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM "HoleriteLancamento" WHERE "HoleriteId" = ?`, id); err != nil {
			return fmt.Errorf("trocar os lançamentos: %w", err)
		}
		for _, l := range lancamentos {
			if _, err := tx.ExecContext(ctx, `INSERT INTO "HoleriteLancamento" ("HoleriteId", "Tipo", "Descricao", "Quantidade",
				"Percentual", "Valor", "IncideInss", "IncideIrrf", "DeduzIrrf", "Recorrente") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, l.Tipo, l.Descricao, l.Quantidade, l.Percentual, l.Valor, l.IncideInss, l.IncideIrrf, l.DeduzIrrf, l.Recorrente); err != nil {
				return fmt.Errorf("gravar o lançamento: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Holerite{}, err
	}
	return s.Obter(ctx, id)
}

// Fechar congela o holerite: guarda o cálculo de agora, que não muda mais com o salário ou a tabela. Lê os lançamentos e
// grava na mesma transação, só se ele continua aberto: um "Salvar" no meio não deixa congelado um cálculo velho.
func (s *Servico) Fechar(ctx context.Context, id int64) (Holerite, error) {
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		h, _, err := s.ler(ctx, tx, id)
		if err != nil {
			return err
		}
		if h.Situacao == Fechado {
			return ErrFechado
		}
		// O calcular lê o emprego e as tabelas pelo s.db, fora da tx (#750): outra conexão do pool, que não tem limite.
		// Com SetMaxOpenConns(1) travaria esperando a conexão que a tx segura. Ler fora dela não muda o resultado: a tx
		// só escreve no holerite, e o que o cálculo lê (emprego, salários, tabelas) não é escrito aqui.
		c, err := s.calcular(ctx, h)
		if err != nil {
			return err
		}
		dados, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("guardar o cálculo: %w", err)
		}
		r, err := tx.ExecContext(ctx, `UPDATE "Holerite" SET "Situacao" = ?, "Calculo" = ?, "FechadoEm" = ? WHERE "Id" = ? AND "Situacao" = ?`,
			Fechado, string(dados), tipos.NovaDataHora(s.relogio.Agora()), id, Aberto)
		if err != nil {
			return fmt.Errorf("fechar o holerite: %w", err)
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return ErrFechado
		}
		return nil
	})
	if err != nil {
		return Holerite{}, err
	}
	return s.Obter(ctx, id)
}

// Reabrir volta o holerite para aberto, para corrigir, e registra na auditoria (r: o pedido de quem reabriu). O lido do
// PDF volta a ser o calculado, com os lançamentos que tinha antes de ler. Recusa (ErrReabrirSemCalculo) quando o
// cálculo de agora daria um aviso, como o mês que ficou fora do emprego depois de corrigir a saída (#750): reaberto, o
// número congelado sumiria, e o Fechar recusaria congelar de novo. O lido do PDF reabre sempre: não tem cálculo a
// preservar (volta sem os números) e, preso, não poderia nem ser excluído.
func (s *Servico) Reabrir(ctx context.Context, r *http.Request, id int64) (Holerite, error) {
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		h, _, err := s.ler(ctx, tx, id)
		if err != nil {
			return err
		}
		if h.Situacao != Fechado {
			return ErrAberto
		}
		if h.Origem == Calculado { // o lido do PDF perde os números ao reabrir de qualquer jeito: reabre sempre, para poder excluir
			if _, err := s.calcular(ctx, h); err != nil { // o mesmo cálculo que o Obter fará depois de reabrir
				if _, _, ok := avisoDe(err); ok {
					return fmt.Errorf("%w: %w", ErrReabrirSemCalculo, err)
				}
				return err
			}
		}
		mes := h.Mes
		if _, err := tx.ExecContext(ctx, `UPDATE "Holerite" SET "Situacao" = ?, "Origem" = ?, "Calculo" = NULL, "FechadoEm" = NULL
			WHERE "Id" = ?`, Aberto, Calculado, id); err != nil {
			return fmt.Errorf("reabrir o holerite: %w", err)
		}
		login, err := auditoria.Login(ctx, tx)
		if err != nil {
			return err
		}
		evento := fmt.Sprintf("%s (%04d-%02d)", EventoReaberto, mes.Ano(), mes.Mes())
		return auditoria.Registrar(ctx, tx, r, tipos.NovaDataHora(s.relogio.Agora()), evento, login)
	})
	if err != nil {
		return Holerite{}, err
	}
	return s.Obter(ctx, id)
}

// GravarLido grava o holerite lido do PDF (#228): fechado, com o cálculo montado do que o PDF imprime. O aberto do mês
// (a estimativa) vira o lido e guarda os lançamentos para quando reabrir; o fechado, reabra antes (ErrFechado). Diz
// também se o holerite já existia, para o desfazer.
func (s *Servico) GravarLido(ctx context.Context, empregoId int64, c Calculo) (Holerite, bool, error) {
	c.Mes = emprego.InicioDoMes(c.Mes)
	e, err := s.empregos.Obter(ctx, empregoId)
	if errors.Is(err, emprego.ErrNaoEncontrado) {
		return Holerite{}, false, ErrNaoEncontrado
	}
	if err != nil {
		return Holerite{}, false, err
	}
	if DiasTrabalhados(c.Mes, e.Admissao, e.Saida) == 0 {
		return Holerite{}, false, invalido("O mês do holerite é de antes da admissão ou depois da saída deste emprego.")
	}
	dados, err := json.Marshal(c)
	if err != nil {
		return Holerite{}, false, fmt.Errorf("guardar o cálculo: %w", err)
	}
	var id int64
	existia := false
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		agora := tipos.NovaDataHora(s.relogio.Agora())
		var situacao int64
		err := tx.QueryRowContext(ctx, `SELECT "Id", "Situacao" FROM "Holerite" WHERE "EmpregoId" = ? AND "Mes" = ?`, empregoId,
			c.Mes).Scan(&id, &situacao)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			uteis, descanso := DiasDoDsr(c.Mes)
			r, err := tx.ExecContext(ctx, `INSERT INTO "Holerite" ("EmpregoId", "Mes", "Situacao", "Origem", "DiasUteis", "DiasDescanso",
				"Calculo", "FechadoEm", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, empregoId, c.Mes, Fechado, LidoDoPdf, uteis,
				descanso, string(dados), agora, agora)
			if err != nil {
				return fmt.Errorf("gravar o holerite lido: %w", err)
			}
			id, err = r.LastInsertId()
			return err
		case err != nil:
			return fmt.Errorf("achar o holerite do mês: %w", err)
		case situacao == Fechado:
			return ErrFechado
		}
		existia = true
		if _, err := tx.ExecContext(ctx, `UPDATE "Holerite" SET "Situacao" = ?, "Origem" = ?, "Calculo" = ?, "FechadoEm" = ?
			WHERE "Id" = ?`, Fechado, LidoDoPdf, string(dados), agora, id); err != nil {
			return fmt.Errorf("gravar o holerite lido: %w", err)
		}
		return nil
	})
	if err != nil {
		return Holerite{}, false, err
	}
	h, err := s.Obter(ctx, id)
	return h, existia, err
}

// DesfazerLido volta ao que era antes de gravar o lido do PDF: apaga o holerite, se ele era novo, ou volta para a
// estimativa aberta que ele substituiu. Só enquanto ele continua o lido (ErrNaoELido se reabriram ou apagaram).
func (s *Servico) DesfazerLido(ctx context.Context, id int64, existia bool) error {
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var origem, situacao int64
		err := tx.QueryRowContext(ctx, `SELECT "Origem", "Situacao" FROM "Holerite" WHERE "Id" = ?`, id).Scan(&origem, &situacao)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (origem != LidoDoPdf || situacao != Fechado)) {
			return ErrNaoELido
		}
		if err != nil {
			return fmt.Errorf("ler o holerite: %w", err)
		}
		if !existia {
			_, err = tx.ExecContext(ctx, `DELETE FROM "Holerite" WHERE "Id" = ?`, id)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE "Holerite" SET "Situacao" = ?, "Origem" = ?, "Calculo" = NULL, "FechadoEm" = NULL
				WHERE "Id" = ?`, Aberto, Calculado, id)
		}
		if err != nil {
			return fmt.Errorf("desfazer o holerite lido: %w", err)
		}
		return nil
	})
}

// Estimativa é o cálculo do mês pelo salário e pelas tabelas, para comparar com o lido do PDF (#228): com os
// lançamentos do holerite do mês, se houver, ou sem lançamentos. Nil quando não dá (sem salário, sem tabela ou fora
// do emprego: os avisos do Obter).
func (s *Servico) Estimativa(ctx context.Context, empregoId int64, mes tipos.Data) (*Calculo, error) {
	mes = emprego.InicioDoMes(mes)
	uteis, descanso := DiasDoDsr(mes)
	h := Holerite{EmpregoId: empregoId, Mes: mes, DiasUteis: uteis, DiasDescanso: descanso}
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT "Id" FROM "Holerite" WHERE "EmpregoId" = ? AND "Mes" = ?`, empregoId, mes).Scan(&id)
	switch {
	case err == nil:
		if h, _, err = s.ler(ctx, s.db, id); err != nil {
			return nil, err
		}
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("achar o holerite do mês: %w", err)
	}
	c, err := s.calcular(ctx, h)
	var validacao erroDeValidacao
	_, _, semCalculo := avisoDe(err)
	switch {
	case semCalculo || errors.As(err, &validacao):
		return nil, nil
	case errors.Is(err, emprego.ErrNaoEncontrado):
		return nil, ErrNaoEncontrado
	case err != nil:
		return nil, err
	}
	return &c, nil
}

// Excluir apaga o holerite aberto (o fechado, reabra antes).
func (s *Servico) Excluir(ctx context.Context, id int64) error {
	var situacao int64
	err := s.db.QueryRowContext(ctx, `SELECT "Situacao" FROM "Holerite" WHERE "Id" = ?`, id).Scan(&situacao)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNaoEncontrado
	}
	if err != nil {
		return fmt.Errorf("ler o holerite: %w", err)
	}
	if situacao == Fechado {
		return ErrFechado
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM "Holerite" WHERE "Id" = ?`, id); err != nil {
		return fmt.Errorf("excluir o holerite: %w", err)
	}
	return nil
}

// VariaveisEntre soma as horas extras, o adicional noturno e o DSR dos holerites de inicio (incluído) até fim (fora),
// abertos e fechados, e diz em quantos meses havia holerite. É a base da média do 13º e das férias (#213, #214). Holerite sem cálculo (sem tabela
// do mês) não entra.
//
// Súmula 347 do TST (revisão do #700): as horas extras entram na média pelas horas, com o salário-hora de agora, e
// não pelo valor em reais da época. O valor de cada mês vai corrigido pela razão salarioAtual ÷ o salário daquele mês:
// é o mesmo que refazer as horas com a hora de agora, e vale também para o holerite lido do PDF, que não guarda as horas.
func (s *Servico) VariaveisEntre(ctx context.Context, empregoId int64, inicio, fim tipos.Data, salarioAtual tipos.Decimal) (soma tipos.Decimal, meses int, err error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id" FROM "Holerite" WHERE "EmpregoId" = ? AND "Mes" >= ? AND "Mes" < ?
		ORDER BY "Mes"`, empregoId, inicio, fim)
	if err != nil {
		return tipos.Decimal{}, 0, fmt.Errorf("ler os holerites do ano: %w", err)
	}
	var ids []int64
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			linhas.Close()
			return tipos.Decimal{}, 0, err
		}
		ids = append(ids, id)
	}
	if err := linhas.Close(); err != nil {
		return tipos.Decimal{}, 0, err
	}
	total := decimal.Zero
	for _, id := range ids {
		h, err := s.Obter(ctx, id)
		if err != nil {
			return tipos.Decimal{}, 0, err
		}
		if h.Calculo == nil {
			continue
		}
		meses++
		doMes := decimal.Zero
		for _, l := range h.Calculo.Proventos {
			if l.Tipo == HoraExtra || l.Tipo == AdicionalNoturno || l.Tipo == TipoDSR {
				doMes = doMes.Add(l.Valor.Decimal())
			}
		}
		if salarioDoMes := h.Calculo.SalarioContratual.Decimal(); salarioDoMes.IsPositive() {
			doMes = doMes.Mul(salarioAtual.Decimal()).Div(salarioDoMes)
		}
		total = total.Add(doMes)
	}
	return tipos.Centavos(total), meses, nil
}
