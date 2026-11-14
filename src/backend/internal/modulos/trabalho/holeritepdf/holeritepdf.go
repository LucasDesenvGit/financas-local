// Pacote holeritepdf é o holerite em PDF do módulo Trabalho (#228, ADR 0068): lê o PDF (o pacote leitor), classifica
// as rubricas (o dicionário no código e o que a pessoa escolheu antes, por emprego), monta a prévia com a prova real e
// o lido × calculado, e grava o holerite do mês só com a confirmação, com "Desfazer". O PDF e a senha não são
// guardados, e nada do PDF vai para o log.
package holeritepdf

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/desfazer"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Como o tipo da rubrica foi achado (a situação da linha na prévia, em texto).
const (
	ComoDicionario   = "dicionario"   // reconhecida
	ComoEscolha      = "escolha"      // reconhecida pelo que a pessoa escolheu antes, para este emprego
	ComoDesconhecida = "desconhecida" // a pessoa escolhe o tipo
)

// Erros que a pessoa resolve.
var ErrNaoEncontrado = errors.New("emprego não encontrado")

type erroDeValidacao struct{ mensagem string }

func (e erroDeValidacao) Error() string { return e.mensagem }

func invalido(mensagem string) error { return erroDeValidacao{mensagem} }

// RubricaLida é a rubrica do PDF com o tipo que o site deu a ela.
type RubricaLida struct {
	leitor.Rubrica
	Tipo int64  `json:"tipo"`
	Como string `json:"como"`
}

// Comparacao é um valor lido do PDF ao lado do calculado pelas tabelas (#210) para o mês.
type Comparacao struct {
	Descricao string        `json:"descricao"`
	Lido      tipos.Decimal `json:"lido"`
	Calculado tipos.Decimal `json:"calculado"`
	Diferenca tipos.Decimal `json:"diferenca"` // lido - calculado
}

// Previa é o holerite lido, antes de gravar: nada é gravado até a confirmação.
type Previa struct {
	leitor.Lido
	Rubricas    []RubricaLida      `json:"rubricas"`
	Comparacoes []Comparacao       `json:"comparacoes"`
	Avisos      []string           `json:"avisos"`
	Existente   *holerite.Holerite `json:"existente"` // o holerite que o mês já tem
}

// RubricaConfirmada é a linha que a pessoa confirmou (ou corrigiu) na prévia.
type RubricaConfirmada struct {
	Codigo     string        `json:"codigo"`
	Descricao  string        `json:"descricao"`
	Referencia string        `json:"referencia"`
	Valor      tipos.Decimal `json:"valor"`
	Provento   bool          `json:"provento"` // o lado em que o PDF trazia o valor (para lembrar a escolha)
	Tipo       int64         `json:"tipo"`
}

// Confirmacao é o que a tela manda para gravar: o mês, as linhas e o rodapé que o leitor achou.
type Confirmacao struct {
	Mes         tipos.Data          `json:"mes"`
	Rubricas    []RubricaConfirmada `json:"rubricas"`
	SalarioBase *tipos.Decimal      `json:"salarioBase"`
	BaseInss    *tipos.Decimal      `json:"baseInss"`
	BaseFgts    *tipos.Decimal      `json:"baseFgts"`
	FgtsDoMes   *tipos.Decimal      `json:"fgtsDoMes"`
}

// Gravado é o holerite gravado e a chave do "Desfazer" (10 minutos).
type Gravado struct {
	Holerite holerite.Holerite `json:"holerite"`
	Desfazer string            `json:"desfazer"`
}

// antes é o que o "Desfazer" precisa: qual holerite e se ele já existia (a estimativa que o lido substituiu).
type antes struct {
	empregoId, holeriteId int64
	existia               bool
}

// Servico lê o PDF, monta a prévia e grava o holerite confirmado.
type Servico struct {
	db        *sql.DB
	empregos  *emprego.Servico
	holerites *holerite.Servico
	memoria   *desfazer.Memoria[antes]
	relogio   relogio.Relogio
	log       *slog.Logger
}

func NovoServico(db *sql.DB, empregos *emprego.Servico, holerites *holerite.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, empregos: empregos, holerites: holerites, memoria: desfazer.Nova[antes](r), relogio: r, log: log}
}

func (s *Servico) emprego(ctx context.Context, id int64) (emprego.Emprego, error) {
	e, err := s.empregos.Obter(ctx, id)
	if errors.Is(err, emprego.ErrNaoEncontrado) {
		return e, ErrNaoEncontrado
	}
	return e, err
}

// Ler lê o PDF e monta a prévia. Os erros do leitor (leitor.ErrSenha, leitor.ErrSemTexto...) voltam como estão.
func (s *Servico) Ler(ctx context.Context, empregoId int64, pdf []byte, senha string) (Previa, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Previa{}, err
	}
	paginas, err := leitor.LerPDF(pdf, senha)
	if err != nil {
		return Previa{}, err
	}
	lido, err := leitor.Interpretar(paginas)
	if err != nil {
		return Previa{}, err
	}
	escolhas, err := s.escolhas(ctx, empregoId)
	if err != nil {
		return Previa{}, err
	}
	p := Previa{Lido: lido, Rubricas: []RubricaLida{}, Comparacoes: []Comparacao{}, Avisos: []string{}}
	for _, r := range lido.Rubricas {
		tipo, conhecida := Classificar(r.Descricao, r.Provento)
		como := ComoDicionario
		if escolhido, ok := escolhas[chave(r.Codigo, r.Descricao)]; ok {
			tipo, como = escolhido, ComoEscolha
		} else if !conhecida {
			como = ComoDesconhecida
		}
		p.Rubricas = append(p.Rubricas, RubricaLida{Rubrica: r, Tipo: tipo, Como: como})
	}
	if err := s.avisar(ctx, e, &p); err != nil {
		return Previa{}, err
	}
	return p, nil
}

// avisar confere o emprego (CNPJ, mês, salário), o holerite que o mês já tem e o lido × calculado.
func (s *Servico) avisar(ctx context.Context, e emprego.Emprego, p *Previa) error {
	if len(p.Provas) == 0 {
		p.Avisos = append(p.Avisos, "Não achei os totais no PDF, então não deu para conferir a soma: confira as linhas com o holerite.")
	}
	if err := s.avisarCnpj(ctx, e, p); err != nil {
		return err
	}
	if p.Mes == nil {
		p.Avisos = append(p.Avisos, "Não achei o mês do holerite no PDF: escolha o mês antes de confirmar.")
		return nil
	}
	mes := *p.Mes
	if holerite.DiasTrabalhados(mes, e.Admissao, e.Saida) == 0 {
		p.Avisos = append(p.Avisos, fmt.Sprintf("%s é de antes da admissão ou depois da saída deste emprego: confira o emprego.",
			maiuscula(holerite.NomeDoMes(mes))))
	}
	if salario, ok := emprego.SalarioDoMes(e.Salarios, mes); ok && p.SalarioBase != nil &&
		!salario.SalarioBruto.Decimal().Equal(p.SalarioBase.Decimal()) {
		p.Avisos = append(p.Avisos, fmt.Sprintf("O salário base do PDF (%s) é diferente do cadastrado para %s (%s). Se foi aumento, "+
			"registre em Trabalho › Salário, a partir de %s.", emReais(p.SalarioBase.Decimal()), holerite.NomeDoMes(mes),
			emReais(salario.SalarioBruto.Decimal()), holerite.NomeDoMes(mes)))
	}
	existente, err := s.holeriteDoMes(ctx, e.Id, mes)
	if err != nil {
		return err
	}
	p.Existente = existente
	switch {
	case existente != nil && existente.Situacao == holerite.Fechado:
		p.Avisos = append(p.Avisos, fmt.Sprintf("O holerite de %s está fechado: reabra antes de gravar o lido.", holerite.NomeDoMes(mes)))
	case existente != nil:
		p.Avisos = append(p.Avisos, fmt.Sprintf("O holerite de %s (a estimativa) vai ser trocado pelo lido. Os lançamentos ficam "+
			"guardados: reabrir o holerite volta para a estimativa.", holerite.NomeDoMes(mes)))
	}
	return s.comparar(ctx, e.Id, mes, p)
}

// avisarCnpj: o CNPJ do PDF é o deste emprego? É de outro emprego cadastrado? De nenhum?
func (s *Servico) avisarCnpj(ctx context.Context, e emprego.Emprego, p *Previa) error {
	if p.Cnpj == nil || (e.Cnpj != nil && *e.Cnpj == *p.Cnpj) {
		return nil
	}
	todos, err := s.empregos.Listar(ctx)
	if err != nil {
		return err
	}
	for _, outro := range todos {
		if outro.Cnpj != nil && *outro.Cnpj == *p.Cnpj && outro.Id != e.Id {
			p.Avisos = append(p.Avisos, fmt.Sprintf("O CNPJ do PDF (%s) é o do emprego %q, não deste. Leia o PDF naquele emprego.",
				cnpjFormatado(*p.Cnpj), outro.Empregador))
			return nil
		}
	}
	if e.Cnpj == nil {
		p.Avisos = append(p.Avisos, fmt.Sprintf("Este emprego não tem CNPJ cadastrado; o do PDF é %s (%s). Se for o mesmo, "+
			"cadastre o CNPJ no emprego, em Trabalho › Salário.", cnpjFormatado(*p.Cnpj), p.Empregador))
		return nil
	}
	p.Avisos = append(p.Avisos, fmt.Sprintf("O CNPJ do PDF (%s, %s) não é o deste emprego (%s) nem o de outro cadastrado. Se é "+
		"um emprego novo, cadastre em Trabalho › Salário com esses dados.", cnpjFormatado(*p.Cnpj), p.Empregador, cnpjFormatado(*e.Cnpj)))
	return nil
}

// comparar põe o INSS, o IRRF e o líquido lidos ao lado dos calculados para o mês (#210). Diferença no INSS ou no
// IRRF costuma ser cadastro errado (dependentes, salário) e vira aviso.
func (s *Servico) comparar(ctx context.Context, empregoId int64, mes tipos.Data, p *Previa) error {
	calculo, err := s.holerites.Estimativa(ctx, empregoId, mes)
	if err != nil || calculo == nil || calculo.SemEncargos {
		return err
	}
	inss, irrf, proventos, descontos := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
	for _, r := range p.Rubricas {
		v := r.Valor.Decimal()
		switch r.Tipo {
		case holerite.TipoInss:
			inss = inss.Add(v)
		case holerite.TipoIrrf:
			irrf = irrf.Add(v)
		}
		if provento, _ := ehProvento(r.Tipo); provento {
			proventos = proventos.Add(v)
		} else {
			descontos = descontos.Add(v)
		}
	}
	calc := func(lista []holerite.Linha, tipo int64) decimal.Decimal {
		soma := decimal.Zero
		for _, l := range lista {
			if l.Tipo == tipo {
				soma = soma.Add(l.Valor.Decimal())
			}
		}
		return soma
	}
	comparacoes := []struct {
		descricao       string
		lido, calculado decimal.Decimal
		avisar          bool
	}{
		{"INSS", inss, calc(calculo.Descontos, holerite.TipoInss), true},
		{"IRRF", irrf, calc(calculo.Descontos, holerite.TipoIrrf), true},
		{"Líquido", proventos.Sub(descontos), calculo.Liquido.Decimal(), false},
	}
	um := decimal.NewFromInt(1)
	for _, c := range comparacoes {
		d := c.lido.Sub(c.calculado)
		p.Comparacoes = append(p.Comparacoes, Comparacao{Descricao: c.descricao, Lido: tipos.DeDecimal(c.lido),
			Calculado: tipos.DeDecimal(c.calculado), Diferenca: tipos.DeDecimal(d)})
		if c.avisar && d.Abs().GreaterThan(um) {
			p.Avisos = append(p.Avisos, fmt.Sprintf("O %s do PDF (%s) é diferente do calculado pelas tabelas (%s). Costuma ser o "+
				"cadastro: confira o salário e os dependentes do emprego.", c.descricao, emReais(c.lido), emReais(c.calculado)))
		}
	}
	return nil
}

func (s *Servico) holeriteDoMes(ctx context.Context, empregoId int64, mes tipos.Data) (*holerite.Holerite, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT "Id" FROM "Holerite" WHERE "EmpregoId" = ? AND "Mes" = ?`, empregoId,
		emprego.InicioDoMes(mes)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("achar o holerite do mês: %w", err)
	}
	h, err := s.holerites.Obter(ctx, id)
	return &h, err
}

// Confirmar grava o holerite lido, como a pessoa confirmou, e lembra os tipos que ela escolheu para as rubricas que o
// dicionário não conhecia (ou que ela corrigiu), para este emprego.
func (s *Servico) Confirmar(ctx context.Context, empregoId int64, c Confirmacao) (Gravado, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Gravado{}, err
	}
	calculo, err := montar(e, c)
	if err != nil {
		return Gravado{}, err
	}
	h, existia, err := s.holerites.GravarLido(ctx, empregoId, calculo)
	if err != nil {
		return Gravado{}, err
	}
	if err := s.lembrar(ctx, empregoId, c.Rubricas); err != nil {
		return Gravado{}, err
	}
	chave := s.memoria.Guardar(antes{empregoId: empregoId, holeriteId: h.Id, existia: existia})
	return Gravado{Holerite: h, Desfazer: chave}, nil
}

// Desfazer volta o holerite ao que era antes da confirmação. False se a chave venceu, já foi usada ou é de outro
// emprego. As escolhas das rubricas ficam: são o que a pessoa ensinou.
func (s *Servico) Desfazer(ctx context.Context, empregoId int64, chave string) (bool, error) {
	a, ok := s.memoria.Tirar(chave)
	if !ok || a.empregoId != empregoId {
		return false, nil
	}
	err := s.holerites.DesfazerLido(ctx, a.holeriteId, a.existia)
	if errors.Is(err, holerite.ErrNaoELido) {
		return false, nil
	}
	return err == nil, err
}

var (
	limiteDoValor = decimal.NewFromInt(10_000_000)
	oitoPct       = decimal.RequireFromString("0.08")
)

// montar faz o cálculo do holerite (o que fica guardado no fechado) com as linhas confirmadas: os totais e o líquido
// são as somas, e o FGTS é o do PDF (ou 8% da base do FGTS, ou dos proventos).
func montar(e emprego.Emprego, c Confirmacao) (holerite.Calculo, error) {
	if len(c.Rubricas) == 0 || len(c.Rubricas) > 100 {
		return holerite.Calculo{}, invalido("O holerite precisa de pelo menos uma linha (e vai até 100).")
	}
	mes := emprego.InicioDoMes(c.Mes)
	calc := holerite.Calculo{Mes: mes, Proventos: []holerite.Linha{}, Descontos: []holerite.Linha{},
		DiasTrabalhados: holerite.DiasTrabalhados(mes, e.Admissao, e.Saida), SemEncargos: e.Regime != emprego.RegimeClt}
	proventos, descontos, salario := decimal.Zero, decimal.Zero, decimal.Zero
	for _, r := range c.Rubricas {
		r.Descricao = strings.TrimSpace(r.Descricao)
		provento, existe := ehProvento(r.Tipo)
		v := r.Valor.Decimal()
		switch {
		case !existe:
			return holerite.Calculo{}, invalido("Escolha o tipo de cada linha.")
		case r.Descricao == "" || len([]rune(r.Descricao)) > 100:
			return holerite.Calculo{}, invalido("A descrição de cada linha vai de 1 a 100 caracteres.")
		case len([]rune(r.Referencia)) > 30 || len([]rune(r.Codigo)) > 20:
			return holerite.Calculo{}, invalido("O código vai até 20 caracteres, e a referência, até 30.")
		case !v.IsPositive() || v.GreaterThanOrEqual(limiteDoValor) || !v.Equal(v.Round(2)):
			return holerite.Calculo{}, invalido("O valor de cada linha vai de R$ 0,01 a R$ 9.999.999,99, em centavos.")
		}
		linha := holerite.Linha{Tipo: r.Tipo, Descricao: r.Descricao, Referencia: r.Referencia, Valor: r.Valor}
		if provento {
			calc.Proventos = append(calc.Proventos, linha)
			proventos = proventos.Add(v)
			if r.Tipo == holerite.TipoSalario {
				salario = salario.Add(v)
			}
		} else {
			calc.Descontos = append(calc.Descontos, linha)
			descontos = descontos.Add(v)
		}
	}
	calc.TotalProventos, calc.TotalDescontos = tipos.DeDecimal(proventos), tipos.DeDecimal(descontos)
	calc.Liquido = tipos.DeDecimal(proventos.Sub(descontos))
	contratual := salario
	if s, ok := emprego.SalarioDoMes(e.Salarios, mes); ok {
		contratual = s.SalarioBruto.Decimal()
	}
	if c.SalarioBase != nil {
		contratual = c.SalarioBase.Decimal()
	}
	calc.SalarioContratual = tipos.DeDecimal(contratual)
	calc.ValorHora = tipos.DeDecimal(contratual.Div(decimal.NewFromInt(e.JornadaMensalHoras)).Round(2))
	if c.BaseInss != nil {
		calc.BaseInss = *c.BaseInss
	}
	switch {
	case calc.SemEncargos:
	case c.FgtsDoMes != nil:
		calc.FgtsDoMes = *c.FgtsDoMes
	case c.BaseFgts != nil:
		calc.FgtsDoMes = tipos.DeDecimal(c.BaseFgts.Decimal().Mul(oitoPct).Round(2))
	default:
		calc.FgtsDoMes = tipos.DeDecimal(proventos.Mul(oitoPct).Round(2))
	}
	return calc, nil
}

// chave é como a rubrica é lembrada: o código e a descrição normalizada (ADR 0030).
func chave(codigo, descricao string) string {
	return strings.TrimSpace(codigo) + "|" + texto.ParaPalavras(descricao)
}

// escolhas são os tipos que a pessoa deu às rubricas deste emprego.
func (s *Servico) escolhas(ctx context.Context, empregoId int64) (map[string]int64, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Codigo", "Descricao", "Tipo" FROM "HoleriteRubricaEscolha" WHERE "EmpregoId" = ?`,
		empregoId)
	if err != nil {
		return nil, fmt.Errorf("ler as escolhas das rubricas: %w", err)
	}
	defer linhas.Close()
	m := map[string]int64{}
	for linhas.Next() {
		var codigo, descricao string
		var tipo int64
		if err := linhas.Scan(&codigo, &descricao, &tipo); err != nil {
			return nil, fmt.Errorf("ler escolha: %w", err)
		}
		m[codigo+"|"+descricao] = tipo
	}
	return m, linhas.Err()
}

// lembrar guarda o tipo da rubrica que o dicionário não dá (desconhecida ou corrigida) e esquece a escolha que ele já
// dá sozinho.
func (s *Servico) lembrar(ctx context.Context, empregoId int64, rubricas []RubricaConfirmada) error {
	agora := tipos.NovaDataHora(s.relogio.Agora())
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		for _, r := range rubricas {
			codigo, descricao := strings.TrimSpace(r.Codigo), texto.ParaPalavras(r.Descricao)
			if tipo, conhecida := Classificar(r.Descricao, r.Provento); conhecida && tipo == r.Tipo {
				if _, err := tx.ExecContext(ctx, `DELETE FROM "HoleriteRubricaEscolha" WHERE "EmpregoId" = ? AND "Codigo" = ? AND "Descricao" = ?`,
					empregoId, codigo, descricao); err != nil {
					return fmt.Errorf("esquecer a escolha: %w", err)
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "HoleriteRubricaEscolha" ("EmpregoId", "Codigo", "Descricao", "Tipo", "CriadoEm")
				VALUES (?, ?, ?, ?, ?) ON CONFLICT ("EmpregoId", "Codigo", "Descricao") DO UPDATE SET "Tipo" = excluded."Tipo"`,
				empregoId, codigo, descricao, r.Tipo, agora); err != nil {
				return fmt.Errorf("lembrar a escolha: %w", err)
			}
		}
		return nil
	})
}

// emReais é "R$ 1.234,56".
func emReais(v decimal.Decimal) string {
	s := v.Abs().StringFixed(2)
	inteiro, centavos := s[:len(s)-3], s[len(s)-2:]
	var partes []string
	for len(inteiro) > 3 {
		partes = append([]string{inteiro[len(inteiro)-3:]}, partes...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	partes = append([]string{inteiro}, partes...)
	sinal := ""
	if v.IsNegative() {
		sinal = "-"
	}
	return sinal + "R$ " + strings.Join(partes, ".") + "," + centavos
}

// cnpjFormatado é "12.345.678/0001-95".
func cnpjFormatado(d string) string {
	return d[:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:]
}

func maiuscula(s string) string { return strings.ToUpper(s[:1]) + s[1:] }
