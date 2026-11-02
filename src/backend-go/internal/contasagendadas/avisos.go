package contasagendadas

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cobrancas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// As constantes dos avisos (#158, ADR 0043).
const (
	diasNaoVeio = 5  // "não veio": a cobrança automática passou tantos dias do vencimento sem dar baixa
	diasAvisos  = 70 // até quantos dias para trás as cobranças são olhadas
)

var (
	variacaoAumento       = decimalFixo("0.05") // "aumentou": a última cobrança passou disto em relação ao valor
	variacaoValorVariavel = decimalFixo("0.15") // valor que varia mais que isso (luz, água) não gera o aviso
)

// AvisoDto é o AvisoContaAgendadaDto do C#, na ordem do record.
type AvisoDto struct {
	ContaAgendadaId int64          `json:"contaAgendadaId"`
	Descricao       string         `json:"descricao"`
	Tipo            string         `json:"tipo"`
	Texto           string         `json:"texto"`
	ValorAnterior   *tipos.Decimal `json:"valorAnterior"`
	ValorNovo       *tipos.Decimal `json:"valorNovo"`
	ContaNovaId     *int64         `json:"contaNovaId"`
	ContaNovaNome   *string        `json:"contaNovaNome"`
	Data            *tipos.Data    `json:"data"`
}

// recorrente é a conta a pagar como os avisos a leem.
type recorrente struct {
	id                int64
	descricao         string
	valor             tipos.Decimal
	proximoVencimento tipos.Data
	formaCobranca     int64
	textoCobranca     *string
	contaId           *int64
	contaNome         *string
	contaArquivada    bool
}

// cobranca é uma saída do extrato, com o texto já normalizado e o valor positivo.
type cobranca struct {
	data      tipos.Data
	valor     tipos.Decimal
	contaId   int64
	contaNome string
	texto     string
}

// Avisos é o AvisosAsync: avisos das contas a pagar recorrentes e ativas. Só lê; as ações ("Atualizar") são o PUT.
//   - "cartao-arquivado": a conta ou o cartão em que ela cai está arquivado;
//   - "nao-veio": cobrada sozinha, com o texto, e o vencimento passou 5 dias sem a cobrança;
//   - "mudou-de-cartao": a última cobrança com o texto veio em outra conta ou cartão;
//   - "aumentou": a última cobrança passou 5% do valor (menos quando o valor varia sempre).
func (s *Servico) Avisos(ctx context.Context, hoje tipos.Data) ([]AvisoDto, error) {
	itens, err := s.lerRecorrentes(ctx)
	if err != nil {
		return nil, err
	}
	var lista []cobranca
	if slices.ContainsFunc(itens, func(c recorrente) bool { return c.textoCobranca != nil }) {
		regrasPorValor, err := cobrancas.RegrasPorValor(ctx, s.db)
		if err != nil {
			return nil, err
		}
		if lista, err = s.lerCobrancas(ctx, hoje.SomarDias(-diasAvisos), hoje, regrasPorValor); err != nil {
			return nil, err
		}
	}

	avisos := []AvisoDto{}
	for _, c := range itens {
		if c.contaArquivada {
			avisos = append(avisos, AvisoDto{ContaAgendadaId: c.id, Descricao: c.descricao, Tipo: "cartao-arquivado",
				Texto: fmt.Sprintf("%s está arquivado, mas %s ainda cobra nele: troque nos serviços.", valorOuVazio(c.contaNome), c.descricao)})
		}
		procurado := normalizar(c.textoCobranca)
		if procurado == "" {
			continue
		}
		var minhas []cobranca
		for _, t := range lista {
			if strings.Contains(t.texto, procurado) {
				minhas = append(minhas, t)
			}
		}
		slices.SortStableFunc(minhas, func(a, b cobranca) int { return b.data.NumeroDoDia() - a.data.NumeroDoDia() })

		if c.formaCobranca != cobrancas.FormaAMao && c.proximoVencimento.SomarDias(diasNaoVeio).Antes(hoje) {
			vencimento := c.proximoVencimento
			avisos = append(avisos, AvisoDto{ContaAgendadaId: c.id, Descricao: c.descricao, Tipo: "nao-veio",
				Texto: fmt.Sprintf("A cobrança de %s esperada em %s não apareceu. Cancelada? Cartão recusado?", c.descricao, diaMes(vencimento)),
				Data:  &vencimento})
		}
		if len(minhas) == 0 {
			continue
		}
		ultima := minhas[0]
		if c.contaId != nil && ultima.contaId != *c.contaId {
			contaNova, nome, data := ultima.contaId, ultima.contaNome, ultima.data
			avisos = append(avisos, AvisoDto{ContaAgendadaId: c.id, Descricao: c.descricao, Tipo: "mudou-de-cartao",
				Texto: fmt.Sprintf("A última cobrança de %s (%s) veio em %s, e não em %s.", c.descricao, diaMes(ultima.data),
					ultima.contaNome, valorOuVazio(c.contaNome)),
				ContaNovaId: &contaNova, ContaNovaNome: &nome, Data: &data})
		}
		if aviso, ok := aumento(c, minhas, ultima); ok {
			avisos = append(avisos, aviso)
		}
	}
	return avisos, nil
}

// aumento é o aviso "aumentou" (ou "baixou"): a última cobrança passou 5% do valor da conta a pagar. As três últimas
// da mesma conta ou cartão da última dizem se o valor varia sempre (mais de 15% entre elas): aí não avisa.
func aumento(c recorrente, minhas []cobranca, ultima cobranca) (AvisoDto, bool) {
	var ultimas []tipos.Decimal
	for _, t := range minhas {
		if t.contaId == ultima.contaId && len(ultimas) < 3 {
			ultimas = append(ultimas, t.valor)
		}
	}
	menor, maior := extremos(ultimas)
	variaSempre := len(ultimas) >= 3 && maior.Decimal().IsPositive() && maiorQue(tipos.Subtrair(maior, menor), maior, variacaoValorVariavel)
	diferenca := tipos.DeDecimal(tipos.Subtrair(ultima.valor, c.valor).Decimal().Abs())
	if variaSempre || !c.valor.Decimal().IsPositive() || !maiorQue(diferenca, c.valor, variacaoAumento) {
		return AvisoDto{}, false
	}
	verbo := "baixou"
	if ultima.valor.Decimal().GreaterThan(c.valor.Decimal()) {
		verbo = "aumentou"
	}
	anterior, novo, data := c.valor, tipos.Arredondar(ultima.valor, 2), ultima.data
	return AvisoDto{ContaAgendadaId: c.id, Descricao: c.descricao, Tipo: "aumentou",
		Texto:         fmt.Sprintf("%s %s na cobrança de %s.", c.descricao, verbo, diaMes(ultima.data)),
		ValorAnterior: &anterior, ValorNovo: &novo, Data: &data}, true
}

func valorOuVazio(t *string) string {
	if t == nil {
		return ""
	}
	return *t
}

func (s *Servico) lerRecorrentes(ctx context.Context) ([]recorrente, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT c."Id", c."Descricao", c."Valor", c."ProximoVencimento", c."FormaCobranca",
		c."TextoCobranca", c."ContaId", co."Nome", COALESCE(co."Arquivada", 0)
		FROM "ContaAgendada" AS c LEFT JOIN "Conta" AS co ON c."ContaId" = co."Id"
		WHERE c."Ativa" AND c."Recorrencia" <> ? ORDER BY c."Id"`, relatorios.RecorrenciaUnica)
	if err != nil {
		return nil, fmt.Errorf("ler as contas a pagar recorrentes: %w", err)
	}
	defer linhas.Close()
	var lista []recorrente
	for linhas.Next() {
		var c recorrente
		if err := linhas.Scan(&c.id, &c.descricao, &c.valor, &c.proximoVencimento, &c.formaCobranca, &c.textoCobranca,
			&c.contaId, &c.contaNome, &c.contaArquivada); err != nil {
			return nil, fmt.Errorf("ler conta a pagar recorrente: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// lerCobrancas são as saídas de desde a hoje, com o texto que a cobrança procura (cobrancas.TextoDaCobranca). O valor
// é texto no banco: o "menor que zero" é conferido em Go.
func (s *Servico) lerCobrancas(ctx context.Context, desde, hoje tipos.Data, regrasPorValor map[int64]bool) ([]cobranca, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Data", t."Valor", t."ContaId", co."Nome", t."Descricao",
		t."DescricaoOriginal", t."RegraDescricaoId"
		FROM "Transacao" AS t INNER JOIN "Conta" AS co ON t."ContaId" = co."Id"
		WHERE t."Data" >= ? AND t."Data" <= ? ORDER BY t."Id"`, desde, hoje)
	if err != nil {
		return nil, fmt.Errorf("ler as cobranças dos avisos: %w", err)
	}
	defer linhas.Close()
	var lista []cobranca
	for linhas.Next() {
		var (
			t                 cobranca
			descricao         string
			descricaoOriginal *string
			regraId           *int64
		)
		if err := linhas.Scan(&t.data, &t.valor, &t.contaId, &t.contaNome, &descricao, &descricaoOriginal, &regraId); err != nil {
			return nil, fmt.Errorf("ler cobrança dos avisos: %w", err)
		}
		if !t.valor.Decimal().IsNegative() {
			continue
		}
		t.valor = tipos.DeDecimal(t.valor.Decimal().Neg())
		t.texto = texto.ParaRegras(cobrancas.TextoDaCobranca(descricao, descricaoOriginal, regraId, regrasPorValor))
		lista = append(lista, t)
	}
	return lista, linhas.Err()
}
