// Pacote regrasdescricao é o RegraDescricaoService do .NET (#123, ADR 0030): "quando a descrição do banco for ou
// contiver X, mostrar Y". Vale só nas transações de nome automático (o do banco ou o dado por uma regra); nome escolhido
// à mão fica. #241: a regra pode ter uma faixa de valor (sem o sinal). #250: as regras valem em cadeia, cada uma uma vez
// só, até MaximoNaCadeia passos; RegraDescricaoId guarda a última, a que deu o nome final.
package regrasdescricao

import (
	"context"
	"fmt"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// MaximoNaCadeia é quantas regras, no máximo, uma transação atravessa em cadeia (#250).
const MaximoNaCadeia = 10

type regra struct {
	id             int64
	texto          string // normalizado
	tipo           int64
	novaDescricao  string
	minimo, maximo *tipos.Decimal
	criadoEm       string
}

// Regras são as regras ativas, da mais específica para a mais geral (o CarregarAsync).
type Regras struct {
	regras []regra
}

// Carregar lê as regras ativas e as ordena como o C#: primeiro as que têm faixa de valor (#241); depois "é igual a",
// "começa com" e "contém" (o tipo, do maior para o menor); no mesmo tipo, o texto mais longo (em unidades UTF-16, o
// string.Length do C#) e, empatando, a mais antiga e o menor Id. A ordenação é estável, como a do LINQ.
func Carregar(ctx context.Context, q consultas.DBTX) (*Regras, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Texto", "TipoCorrespondencia", "NovaDescricao", "ValorMinimo",
		"ValorMaximo", "CriadoEm" FROM "RegraDescricao" WHERE "Ativa"`)
	if err != nil {
		return nil, fmt.Errorf("ler as regras de descrição: %w", err)
	}
	defer linhas.Close()
	var lista []regra
	for linhas.Next() {
		var r regra
		var criado tipos.DataHora
		if err := linhas.Scan(&r.id, &r.texto, &r.tipo, &r.novaDescricao, &r.minimo, &r.maximo, &criado); err != nil {
			return nil, fmt.Errorf("ler regra de descrição: %w", err)
		}
		r.texto = texto.ParaRegras(r.texto)
		r.criadoEm = criado.String()
		if r.texto != "" {
			lista = append(lista, r)
		}
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as regras de descrição: %w", err)
	}
	slices.SortStableFunc(lista, func(a, b regra) int {
		if fa, fb := comFaixa(a), comFaixa(b); fa != fb {
			if fa {
				return -1
			}
			return 1
		}
		if a.tipo != b.tipo {
			return int(b.tipo - a.tipo)
		}
		if ta, tb := respostas.TamanhoUtf16(a.texto), respostas.TamanhoUtf16(b.texto); ta != tb {
			return tb - ta
		}
		if a.criadoEm != b.criadoEm { // "2006-01-02 15:04:05.9999999": a ordem do texto é a do tempo
			return compararDataHora(a.criadoEm, b.criadoEm)
		}
		switch {
		case a.id < b.id:
			return -1
		case a.id > b.id:
			return 1
		}
		return 0
	})
	return &Regras{regras: lista}, nil
}

func comFaixa(r regra) bool { return r.minimo != nil || r.maximo != nil }

func compararDataHora(a, b string) int {
	da, _ := tipos.LerDataHora(a)
	db, _ := tipos.LerDataHora(b)
	return da.Tempo().Compare(db.Tempo())
}

// NomeAutomatico: a transação veio do banco e o nome não foi escolhido à mão.
func NomeAutomatico(t *transacoes.Transacao) bool {
	return !t.DescricaoManual && t.DescricaoOriginal != nil &&
		(t.RegraDescricaoId != nil || t.Descricao == *t.DescricaoOriginal)
}

// NaFaixa (#241): o valor, sem o sinal, está na faixa da regra (cada ponta é opcional).
func NaFaixa(valor tipos.Decimal, minimo, maximo *tipos.Decimal) bool {
	absoluto := valor.Decimal().Abs()
	return (minimo == nil || absoluto.GreaterThanOrEqual(minimo.Decimal())) &&
		(maximo == nil || absoluto.LessThanOrEqual(maximo.Decimal()))
}

// Passo é uma regra da cadeia: o id e o nome que ela dá.
type Passo struct {
	Id            int64
	NovaDescricao string
}

func (rs *Regras) resolver(descricao *string, valor tipos.Decimal, usadas map[int64]bool) (Passo, bool) {
	if descricao == nil {
		return Passo{}, false
	}
	t := texto.ParaRegras(*descricao)
	if t == "" {
		return Passo{}, false
	}
	for _, r := range rs.regras {
		if !usadas[r.id] && categorizacao.Corresponde(t, r.texto, r.tipo) && NaFaixa(valor, r.minimo, r.maximo) {
			return Passo{r.id, r.novaDescricao}, true
		}
	}
	return Passo{}, false
}

// Cadeia (#250): as regras que a transação atravessa, em ordem; a primeira pela descrição do banco, cada uma das
// seguintes pelo nome que a anterior deu.
func (rs *Regras) Cadeia(descricaoOriginal *string, valor tipos.Decimal) []Passo {
	cadeia := []Passo{}
	usadas := map[int64]bool{}
	t := descricaoOriginal
	for len(cadeia) < MaximoNaCadeia {
		p, ok := rs.resolver(t, valor, usadas)
		if !ok {
			break
		}
		cadeia = append(cadeia, p)
		usadas[p.Id] = true
		nome := p.NovaDescricao
		t = &nome
	}
	return cadeia
}

// Aplicar dá à transação de nome automático o nome da última regra da cadeia (ou o do banco, sem regra). True se mudou.
func (rs *Regras) Aplicar(t *transacoes.Transacao) bool {
	if !NomeAutomatico(t) {
		return false
	}
	cadeia := rs.Cadeia(t.DescricaoOriginal, t.Valor)
	descricao := *t.DescricaoOriginal
	var regraId *int64
	if len(cadeia) > 0 {
		ultima := cadeia[len(cadeia)-1]
		descricao, regraId = ultima.NovaDescricao, &ultima.Id
	}
	if t.Descricao == descricao && mesmoId(t.RegraDescricaoId, regraId) {
		return false
	}
	t.Descricao = descricao
	t.RegraDescricaoId = regraId
	return true
}

func mesmoId(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// ondeNomeAutomatico é o filtro das transações de nome automático, no SQL.
const ondeNomeAutomatico = `NOT "DescricaoManual" AND "DescricaoOriginal" IS NOT NULL
	AND ("RegraDescricaoId" IS NOT NULL OR "Descricao" = "DescricaoOriginal")`

// NomeAutomaticoNoBanco lê as transações que estão com o nome automático no banco (a consulta do ReaplicarAsync).
func NomeAutomaticoNoBanco(ctx context.Context, q consultas.DBTX) ([]*transacoes.Transacao, error) {
	return transacoes.Listar(ctx, q, `WHERE `+ondeNomeAutomatico)
}

// Reaplicar (o ReaplicarAsync) aplica as regras em todas as transações de nome automático e devolve as que mudaram,
// com AtualizadoEm = agora. Não grava: quem chama decide (transacoes.GravarTodas).
func Reaplicar(ctx context.Context, q consultas.DBTX, agora tipos.DataHora) ([]*transacoes.Transacao, error) {
	rs, err := Carregar(ctx, q)
	if err != nil {
		return nil, err
	}
	lista, err := NomeAutomaticoNoBanco(ctx, q)
	if err != nil {
		return nil, err
	}
	mudaram := []*transacoes.Transacao{}
	for _, t := range lista {
		if rs.Aplicar(t) {
			t.AtualizadoEm = agora
			mudaram = append(mudaram, t)
		}
	}
	return mudaram, nil
}

// Previa (o PreviaAsync): quantas transações de nome automático batem com o texto (pela descrição do banco ou pelo
// nome de agora, #250) e com a faixa de valor (#241), e até 3 exemplos diferentes, na ordem em que o banco devolve.
func Previa(ctx context.Context, q consultas.DBTX, textoRegra string, tipo int64, excetoId int64, minimo, maximo *tipos.Decimal) (int, []string, error) {
	normalizado := texto.ParaRegras(textoRegra)
	if normalizado == "" {
		return 0, []string{}, nil
	}
	linhas, err := q.QueryContext(ctx, `SELECT "DescricaoOriginal", "Descricao", "Valor" FROM "Transacao"
		WHERE "Id" <> ? AND `+ondeNomeAutomatico, excetoId)
	if err != nil {
		return 0, nil, fmt.Errorf("ler as transações da prévia: %w", err)
	}
	defer linhas.Close()
	quantidade := 0
	exemplos := []string{}
	vistos := map[string]bool{}
	for linhas.Next() {
		var original, atual string
		var valor tipos.Decimal
		if err := linhas.Scan(&original, &atual, &valor); err != nil {
			return 0, nil, fmt.Errorf("ler transação da prévia: %w", err)
		}
		if !NaFaixa(valor, minimo, maximo) {
			continue
		}
		var bateu string
		switch {
		case categorizacao.Corresponde(texto.ParaRegras(original), normalizado, tipo):
			bateu = original
		case categorizacao.Corresponde(texto.ParaRegras(atual), normalizado, tipo):
			bateu = atual
		default:
			continue
		}
		quantidade++
		if len(exemplos) < 3 && !vistos[bateu] {
			vistos[bateu] = true
			exemplos = append(exemplos, bateu)
		}
	}
	return quantidade, exemplos, linhas.Err()
}
