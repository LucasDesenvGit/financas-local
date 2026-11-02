// Package cobrancas é a regra que liga uma conta a pagar às cobranças do extrato (#158, ADR 0043; #205; #241): a
// conciliação (o ContaAgendadaService.ConciliarAsync do C#), o texto da transação que a cobrança procura e as chaves
// do extrato. É um pacote de regra, sem HTTP (ADR 0056), porque três lugares conciliam: as rotas das contas a pagar
// (#79), as rotinas de partida (#444) e a sincronização (#83).
package cobrancas

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sugestoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// DiasAntesDoVencimento: a cobrança que chega até tantos dias antes do vencimento esperado já é a dele (o banco
// adianta às vezes).
const DiasAntesDoVencimento = 10

// FormaAMao é a FormaCobranca "à mão": não concilia, porque quem paga é a pessoa.
const FormaAMao = 1

// tamanhoMaximoDaChave é o tamanho da coluna ChaveExtrato.
const tamanhoMaximoDaChave = 150

// RegrasPorValor são os ids das regras de descrição com faixa de valor (#241), o RegrasPorValorAsync do C#.
func RegrasPorValor(ctx context.Context, q consultas.DBTX) (map[int64]bool, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id" FROM "RegraDescricao" WHERE "ValorMinimo" IS NOT NULL OR "ValorMaximo" IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("ler as regras de descrição por valor: %w", err)
	}
	defer linhas.Close()
	regras := map[int64]bool{}
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return nil, fmt.Errorf("ler regra de descrição por valor: %w", err)
		}
		regras[id] = true
	}
	return regras, linhas.Err()
}

// TextoDaCobranca é o texto da transação que o texto da cobrança procura: a descrição do banco (o nome que você deu,
// #123, não atrapalha), menos quando o nome veio de uma regra com faixa de valor (#241). Aí as duas cobranças têm a
// mesma descrição do banco, e é o nome da regra que diz qual é qual.
func TextoDaCobranca(descricao string, descricaoOriginal *string, regraDescricaoId *int64, regrasPorValor map[int64]bool) string {
	if regraDescricaoId != nil && regrasPorValor[*regraDescricaoId] {
		return descricao
	}
	if descricaoOriginal != nil {
		return *descricaoOriginal
	}
	return descricao
}

// ChaveDe é a chave do extrato de uma descrição (#205): a mesma da sugestão de categorias; sem palavra significativa
// (ex.: "BOLETO F I A P"), o texto todo sem as partes que não são só letras, para não perder o pagamento.
func ChaveDe(descricao string) string {
	if chave := sugestoes.Chave(descricao); chave != "" {
		return chave
	}
	var partes []string
	for _, p := range strings.Split(texto.ParaRegras(descricao), " ") {
		if p != "" && soLetras(p) {
			partes = append(partes, p)
		}
	}
	return strings.Join(partes, " ")
}

// soLetras é o p.All(char.IsLetter) do C#, um char UTF-16 por vez: a letra fora do plano básico chega em duas metades
// que não são letra.
func soLetras(p string) bool {
	for _, r := range p {
		if r >= 0x10000 || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// ChaveDaRegra é a chave do grupo de uma regra com faixa de valor (#241): o nome inteiro, sem acento e em maiúsculas,
// com os números ("Condomínio 1" e "Condomínio 2" são dois), cortado em 150 chars UTF-16.
func ChaveDaRegra(nome string) string {
	chave := texto.ParaRegras(nome)
	if texto.TamanhoUtf16(chave) <= tamanhoMaximoDaChave {
		return chave
	}
	return strings.TrimRightFunc(string(utf16.Decode(utf16.Encode([]rune(chave))[:tamanhoMaximoDaChave])), unicode.IsSpace)
}

// contaAConciliar é a conta a pagar como a conciliação a lê e muda.
type contaAConciliar struct {
	id                int64
	textoCobranca     string
	proximoVencimento tipos.Data
	contaId           *int64
	recorrencia       int64
	dataLimite        *tipos.Data
	ultimoPagamentoEm *tipos.Data
	ativa             bool
	chaveExtrato      *string
}

// candidata é uma cobrança do extrato que pode ser a da conta a pagar.
type candidata struct {
	data            tipos.Data
	texto           string
	estabelecimento *string
	porValor        bool
}

// Conciliar dá baixa nas contas a pagar cobradas sozinhas (débito automático ou cartão) cuja cobrança já chegou no
// extrato (#158): o último pagamento vai para a data dela e o próximo vencimento avança, como no "Pagar" (#69: passou
// da data limite, a conta é encerrada). A cobrança casada também grava a chave do extrato (#205), se a conta ainda não
// tinha. Grava só as contas que mudaram e devolve quantas.
func Conciliar(ctx context.Context, q consultas.DBTX, hoje tipos.Data) (int, error) {
	contas, err := lerContas(ctx, q)
	if err != nil || len(contas) == 0 {
		return 0, err
	}
	regrasPorValor, err := RegrasPorValor(ctx, q)
	if err != nil {
		return 0, err
	}
	mudaram := 0
	for _, c := range contas {
		procurado := texto.ParaRegras(c.textoCobranca)
		if procurado == "" {
			continue
		}
		candidatas, err := lerCandidatas(ctx, q, c.proximoVencimento.SomarDias(-DiasAntesDoVencimento), hoje, c.contaId, regrasPorValor)
		if err != nil {
			return mudaram, err
		}
		casada := maisRecente(candidatas, procurado)
		if casada == nil {
			continue
		}
		antes := *c
		var chave string
		if casada.porValor {
			chave = ChaveDaRegra(casada.texto)
		} else if casada.estabelecimento != nil {
			chave = ChaveDe(*casada.estabelecimento)
		} else {
			chave = ChaveDe(casada.texto)
		}
		if tamanho := texto.TamanhoUtf16(chave); c.chaveExtrato == nil && tamanho > 0 && tamanho <= tamanhoMaximoDaChave {
			c.chaveExtrato = &chave
		}
		c.ultimoPagamentoEm = &casada.data
		avancar(c, casada.data)
		if mudou(antes, *c) {
			if err := gravar(ctx, q, c); err != nil {
				return mudaram, err
			}
			mudaram++
		}
	}
	return mudaram, nil
}

// avancar passa os vencimentos que a cobrança de data já pagou (com a folga de DiasAntesDoVencimento).
func avancar(c *contaAConciliar, data tipos.Data) {
	limite := data.SomarDias(DiasAntesDoVencimento)
	for c.ativa && !c.proximoVencimento.Depois(limite) {
		proximo, ok := relatorios.ProximoVencimentoApos(c.proximoVencimento, c.recorrencia)
		if !ok {
			c.ativa = false
			return
		}
		c.proximoVencimento = proximo
		if c.dataLimite != nil && proximo.Depois(*c.dataLimite) {
			c.ativa = false
		}
	}
}

// maisRecente é a cobrança de data mais recente cujo texto contém o procurado; no empate, a primeira lida (o
// OrderByDescending estável do C#).
func maisRecente(candidatas []candidata, procurado string) *candidata {
	var melhor *candidata
	for i := range candidatas {
		x := &candidatas[i]
		if strings.Contains(texto.ParaRegras(x.texto), procurado) && (melhor == nil || x.data.Depois(melhor.data)) {
			melhor = x
		}
	}
	return melhor
}

func mudou(a, b contaAConciliar) bool {
	return !datasIguais(a.ultimoPagamentoEm, b.ultimoPagamentoEm) || !a.proximoVencimento.Igual(b.proximoVencimento) ||
		a.ativa != b.ativa || !textosIguais(a.chaveExtrato, b.chaveExtrato)
}

func datasIguais(a, b *tipos.Data) bool { return a == b || (a != nil && b != nil && a.Igual(*b)) }

func textosIguais(a, b *string) bool { return a == b || (a != nil && b != nil && *a == *b) }

func lerContas(ctx context.Context, q consultas.DBTX) ([]*contaAConciliar, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "TextoCobranca", "ProximoVencimento", "ContaId", "Recorrencia",
		"DataLimite", "UltimoPagamentoEm", "Ativa", "ChaveExtrato" FROM "ContaAgendada"
		WHERE "Ativa" AND "FormaCobranca" <> ? AND "TextoCobranca" IS NOT NULL ORDER BY "Id"`, FormaAMao)
	if err != nil {
		return nil, fmt.Errorf("ler as contas a pagar a conciliar: %w", err)
	}
	defer linhas.Close()
	var contas []*contaAConciliar
	for linhas.Next() {
		c := &contaAConciliar{}
		if err := linhas.Scan(&c.id, &c.textoCobranca, &c.proximoVencimento, &c.contaId, &c.recorrencia, &c.dataLimite,
			&c.ultimoPagamentoEm, &c.ativa, &c.chaveExtrato); err != nil {
			return nil, fmt.Errorf("ler conta a pagar a conciliar: %w", err)
		}
		contas = append(contas, c)
	}
	return contas, linhas.Err()
}

// lerCandidatas são as despesas de desde a hoje, na conta da conta a pagar (ou em qualquer uma, se ela não diz). O
// valor é texto no banco: o "menor que zero" é conferido em Go, com decimal.
func lerCandidatas(ctx context.Context, q consultas.DBTX, desde, hoje tipos.Data, contaId *int64,
	regrasPorValor map[int64]bool) ([]candidata, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Data", "Descricao", "DescricaoOriginal", "RegraDescricaoId", "Estabelecimento",
		"Valor" FROM "Transacao" WHERE "Data" >= ? AND "Data" <= ? AND (? IS NULL OR "ContaId" = ?) ORDER BY "Id"`,
		desde, hoje, contaId, contaId)
	if err != nil {
		return nil, fmt.Errorf("ler as cobranças do extrato: %w", err)
	}
	defer linhas.Close()
	var lista []candidata
	for linhas.Next() {
		var (
			x                 candidata
			descricao         string
			descricaoOriginal *string
			regraId           *int64
			valor             tipos.Decimal
		)
		if err := linhas.Scan(&x.data, &descricao, &descricaoOriginal, &regraId, &x.estabelecimento, &valor); err != nil {
			return nil, fmt.Errorf("ler cobrança do extrato: %w", err)
		}
		if !valor.Decimal().IsNegative() {
			continue
		}
		x.texto = TextoDaCobranca(descricao, descricaoOriginal, regraId, regrasPorValor)
		x.porValor = regraId != nil && regrasPorValor[*regraId]
		lista = append(lista, x)
	}
	return lista, linhas.Err()
}

func gravar(ctx context.Context, q consultas.DBTX, c *contaAConciliar) error {
	_, err := q.ExecContext(ctx, `UPDATE "ContaAgendada" SET "UltimoPagamentoEm" = ?, "ProximoVencimento" = ?, "Ativa" = ?,
		"ChaveExtrato" = ? WHERE "Id" = ?`, c.ultimoPagamentoEm, c.proximoVencimento, c.ativa, c.chaveExtrato, c.id)
	if err != nil {
		return fmt.Errorf("gravar a conta a pagar %d conciliada: %w", c.id, err)
	}
	return nil
}
