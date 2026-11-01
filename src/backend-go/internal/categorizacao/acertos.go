package categorizacao

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parcelas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipolancamento"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Os acertos de dados que o C# faz ao iniciar a API (Program.cs) e, no caso do último, depois de cada sincronização
// (#83). Cada um é uma função só, chamada pela partida (#444) e, depois, pela sincronização. Nenhum abre transação: o
// SaveChanges único do EF Core vira a transação de quem chama, para o que a rotina acerta valer inteiro ou nada.

// Marcas em Configuracao de que o grupo de regras padrão já foi criado (ConfiguracaoService).
const (
	// ChaveRegraMeliDolar marca que a regra padrão do Méli Dólar já foi criada (#169): apagada por você, ela não volta.
	ChaveRegraMeliDolar = "Regras.MeliDolarCriada"
	// ChaveRegrasAplicacaoResgate marca que as regras padrão de aplicação e resgate já foram criadas (#172).
	ChaveRegrasAplicacaoResgate = "Regras.AplicacaoResgateCriadas"
)

// prioridadeDaRegraPadrao é a das regras que chegam depois da instalação (a mesma do 02-dados-iniciais.sql).
const prioridadeDaRegraPadrao = 50

// regraPadraoNova é um grupo do RegrasPadraoNovas do C#. Filtros é o LIKE que acha os candidatos no banco; a
// comparação certa é pelo texto normalizado.
type regraPadraoNova struct {
	chave   string
	textos  []string
	filtros []string
}

// regrasPadraoNovas: regras padrão (→ Investimentos) que chegaram depois da instalação. Os mesmos textos estão no
// banco/02-dados-iniciais.sql.
var regrasPadraoNovas = []regraPadraoNova{
	// #169 (ADR 0038): comprar e vender dólar é mover o próprio dinheiro, não gasto nem receita.
	{ChaveRegraMeliDolar, []string{"MELI DOLAR"}, []string{"%meli d_lar%"}},
	// #172: aplicação e resgate de renda fixa são movimento seu. O produto vai junto no texto para não pegar "resgate
	// de pontos" ou "resgate de cashback"; "aplicação financeira" sozinha ficou de fora porque também aparece em
	// "rendimento de aplicação financeira", que é receita.
	{ChaveRegrasAplicacaoResgate,
		[]string{"APLICACAO RDB", "RESGATE RDB", "APLICACAO CDB", "RESGATE CDB", "APLICACAO POUPANCA", "RESGATE POUPANCA",
			"RESGATE DE APLICACAO", "RESGATE APLICACAO"},
		[]string{"%aplica%", "%resgat%"}},
}

// CriarRegrasPadrao é o CriarRegrasPadraoAsync: cria, uma vez por banco, as regras padrão novas para quem instalou antes
// delas e passa para "Investimentos" as transações já importadas que batem com elas e cuja categoria você não escolheu.
// A marca em Configuracao faz cada grupo ser criado uma vez só, e uma regra apagada por você não volta. Sem a categoria
// "Investimentos", a marca não é gravada, e as regras nascem quando ela voltar (#568). Devolve quantas transações mudaram.
//
// As regras lidas para o AplicarTipo são as de antes das criadas aqui: no C# elas ainda estão só na memória do contexto
// (Add, sem SaveChanges) quando o CarregarAsync lê o banco, então não entram. Só o texto de estorno no cartão olha as
// regras, e a regra nova nunca é a certa para ele (a categoria dela é "Investimentos", de transferência).
func CriarRegrasPadrao(ctx context.Context, q consultas.DBTX, rel relogio.Relogio) (int, error) {
	config := configuracao.Novo(q, rel)
	agora := tipos.NovaDataHora(rel.Agora())
	var investimentos *int64
	var regras *Regras
	mudaram := 0
	for _, grupo := range regrasPadraoNovas {
		existe, err := config.Existe(ctx, grupo.chave)
		if err != nil {
			return mudaram, err
		}
		if existe {
			continue
		}
		if investimentos == nil {
			if investimentos, err = categoriaInvestimentos(ctx, q); err != nil {
				return mudaram, err
			}
		}
		if investimentos == nil {
			// Sem a categoria, nenhum grupo nasce nem ganha marca: nascem na partida seguinte à volta dela (#568).
			return mudaram, nil
		}
		if err := config.DefinirData(ctx, grupo.chave, rel.Agora()); err != nil {
			return mudaram, err
		}
		if regras == nil {
			if regras, err = Carregar(ctx, q); err != nil {
				return mudaram, err
			}
		}
		for _, regra := range grupo.textos {
			// Só cria a que ainda não existe (o mesmo Texto, comparado como está, sem normalizar).
			_, err := q.ExecContext(ctx, `INSERT INTO "RegraCategorizacao" ("Texto", "TipoCorrespondencia", "CategoriaId",
				"Prioridade", "Ativa", "CriadoEm") SELECT ?, ?, ?, ?, 1, ?
				WHERE NOT EXISTS (SELECT 1 FROM "RegraCategorizacao" WHERE "Texto" = ?)`,
				regra, Contem, *investimentos, prioridadeDaRegraPadrao, agora, regra)
			if err != nil {
				return mudaram, fmt.Errorf("criar a regra padrão %s: %w", regra, err)
			}
		}
		n, err := passarParaInvestimentos(ctx, q, regras, grupo, *investimentos, agora)
		if err != nil {
			return mudaram, err
		}
		mudaram += n
	}
	return mudaram, nil
}

// categoriaInvestimentos é a primeira categoria de transferência chamada "Investimentos" (sem acento nem caixa), ou nil.
func categoriaInvestimentos(ctx context.Context, q consultas.DBTX) (*int64, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Nome" FROM "Categoria" WHERE "Tipo" = ? ORDER BY "Id"`, CategoriaTransferencia)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias de transferência: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var id int64
		var nome string
		if err := linhas.Scan(&id, &nome); err != nil {
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		if texto.ParaRegras(nome) == "INVESTIMENTOS" {
			return &id, nil
		}
	}
	return nil, linhas.Err()
}

// passarParaInvestimentos muda para a categoria as transações que ainda não estão nela, sem categoria escolhida à mão e
// sem grupo de transferência, cujo texto (o do banco, ou o nome se não houver) contém uma das regras do grupo.
func passarParaInvestimentos(ctx context.Context, q consultas.DBTX, regras *Regras, grupo regraPadraoNova, categoria int64, agora tipos.DataHora) (int, error) {
	candidatas := map[int64]*transacoes.Transacao{}
	var ordem []int64 // a ordem em que cada uma apareceu (o Dictionary do C# guarda a ordem de inserção)
	for _, filtro := range grupo.filtros {
		achadas, err := transacoes.Listar(ctx, q, `WHERE NOT "CategoriaManual" AND "TransferenciaGrupo" IS NULL
			AND ("CategoriaId" <> ? OR "CategoriaId" IS NULL) AND COALESCE("DescricaoOriginal", "Descricao") LIKE ? ORDER BY "Id"`,
			categoria, filtro)
		if err != nil {
			return 0, err
		}
		for _, t := range achadas {
			if _, repetida := candidatas[t.Id]; !repetida {
				ordem = append(ordem, t.Id)
			}
			candidatas[t.Id] = t
		}
	}
	mudaram := 0
	for _, id := range ordem {
		t := candidatas[id]
		descricao := t.Descricao
		if t.DescricaoOriginal != nil {
			descricao = *t.DescricaoOriginal
		}
		normalizada := texto.ParaRegras(descricao)
		if !contemAlguma(normalizada, grupo.textos) {
			continue
		}
		t.CategoriaId = &categoria
		regras.AplicarTipo(t)
		t.AtualizadoEm = agora
		if err := transacoes.Gravar(ctx, q, t); err != nil {
			return mudaram, err
		}
		mudaram++
	}
	return mudaram, nil
}

func contemAlguma(texto string, trechos []string) bool {
	for _, trecho := range trechos {
		if strings.Contains(texto, trecho) {
			return true
		}
	}
	return false
}

// ReclassificarEntradasNosCartoes é o ReclassificarEntradasNosCartoesAsync (#166, ADR 0036): acerta as entradas nos
// cartões que já estão no banco (estornos que contavam como receita, pagamentos sem a categoria certa). Devolve quantas
// mudaram de tipo, de "fora dos relatórios" ou de categoria.
func ReclassificarEntradasNosCartoes(ctx context.Context, q consultas.DBTX, agora tipos.DataHora) (int, error) {
	regras, err := Carregar(ctx, q)
	if err != nil {
		return 0, err
	}
	// Valor > 0 é em Go: a coluna é TEXT e o SQL não compara dinheiro.
	entradas, err := transacoes.Listar(ctx, q, `WHERE "ContaId" IN (SELECT "Id" FROM "Conta" WHERE "Tipo" = ?)
		AND "TransferenciaGrupo" IS NULL AND ("Tipo" = ? OR ("Tipo" = ? AND NOT "CategoriaManual")) ORDER BY "Id"`,
		tipolancamento.CartaoCredito, transacoes.Receita, transacoes.Transferencia)
	if err != nil {
		return 0, err
	}
	mudaram := 0
	for _, t := range entradas {
		if !t.Valor.Decimal().IsPositive() {
			continue
		}
		tipo, ignorar, categoria := t.Tipo, t.IgnorarRelatorios, t.CategoriaId
		regras.AplicarTipo(t)
		if t.Tipo == tipo && t.IgnorarRelatorios == ignorar && mesmaCategoria(t.CategoriaId, categoria) {
			continue
		}
		t.AtualizadoEm = agora
		if err := transacoes.Gravar(ctx, q, t); err != nil {
			return mudaram, err
		}
		mudaram++
	}
	return mudaram, nil
}

func mesmaCategoria(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// IgnorarComprasParceladasEstornadas é o IgnorarComprasParceladasEstornadasAsync (#257, ADR 0036): compra parcelada
// devolvida inteira (o estorno do valor total) fica fora dos relatórios, com o estorno e as parcelas dela já lançadas: a
// compra foi cancelada e, no total, não custou nada. Olha os lançamentos dos cartões dos últimos 460 dias (400 do estorno
// da compra e 60 de folga). Devolve quantas transações passaram a ficar fora.
func IgnorarComprasParceladasEstornadas(ctx context.Context, q consultas.DBTX, hoje tipos.Data, agora tipos.DataHora) (int, error) {
	desde := hoje.SomarDias(-parcelas.DiasEstornoDaCompra - 60)
	// O C# só traz do banco o que tem valor positivo ou mais de uma parcela, para ler menos linhas. Aqui vem tudo da janela
	// e o ComprasEstornadas escolhe (ele já descarta o que não é estorno nem parcela): o resultado é o mesmo, e a coluna
	// do valor é TEXT, que o SQL não compara.
	lancamentos, err := transacoes.Listar(ctx, q, `WHERE "ContaId" IN (SELECT "Id" FROM "Conta" WHERE "Tipo" = ?)
		AND "Data" >= ? AND "TransferenciaGrupo" IS NULL AND "Tipo" = ? ORDER BY "Id"`,
		tipolancamento.CartaoCredito, desde, transacoes.Despesa)
	if err != nil {
		return 0, err
	}
	lista := make([]parcelas.Lancamento, len(lancamentos))
	for i, t := range lancamentos {
		descricao := t.Descricao
		if t.DescricaoOriginal != nil {
			descricao = *t.DescricaoOriginal
		}
		lista[i] = parcelas.Lancamento{Id: t.Id, ContaId: t.ContaId, Data: t.Data, Valor: t.Valor,
			Numero: t.ParcelaNumero, Total: t.ParcelaTotal, Descricao: descricao}
	}
	fora := parcelas.ComprasEstornadas(lista)
	mudaram := 0
	for _, t := range lancamentos {
		if !fora[t.Id] || t.IgnorarRelatorios {
			continue
		}
		t.IgnorarRelatorios = true
		t.AtualizadoEm = agora
		if err := transacoes.Gravar(ctx, q, t); err != nil {
			return mudaram, err
		}
		mudaram++
	}
	return mudaram, nil
}
