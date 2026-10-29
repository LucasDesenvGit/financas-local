package sugestoes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	diasHistorico            = 730
	maximoGrupos             = 300
	maximoTransacoes         = 5000
	conflitosParaAviso       = 3
	fracaoConflitosParaAviso = 0.2
)

// palavrasChave: palavra-chave (sem acento, maiúscula) → o nome da categoria padrão, na ordem do C# (vale a primeira
// que aparece).
var palavrasChave = [][2]string{
	{"SUPERMERCADO", "Mercado"}, {"MERCADO", "Mercado"}, {"ATACADAO", "Mercado"}, {"ASSAI", "Mercado"},
	{"CARREFOUR", "Mercado"}, {"PAO DE ACUCAR", "Mercado"}, {"HORTIFRUTI", "Mercado"}, {"SAMS CLUB", "Mercado"},
	{"ATACADISTA", "Mercado"}, {"SACOLAO", "Mercado"}, {"EMPORIO", "Mercado"},
	{"IFOOD", "Alimentação"}, {"IFD", "Alimentação"}, {"RAPPI", "Alimentação"}, {"RESTAURANTE", "Alimentação"},
	{"LANCHONETE", "Alimentação"}, {"PADARIA", "Alimentação"}, {"PANIFICADORA", "Alimentação"},
	{"BURGER", "Alimentação"}, {"MCDONALDS", "Alimentação"}, {"PIZZA", "Alimentação"}, {"SUSHI", "Alimentação"},
	{"CAFETERIA", "Alimentação"}, {"STARBUCKS", "Alimentação"}, {"OUTBACK", "Alimentação"}, {"BAR ", "Alimentação"},
	{"UBER", "Transporte"}, {"99APP", "Transporte"}, {"99 TAXI", "Transporte"}, {"99POP", "Transporte"},
	{"POSTO", "Transporte"}, {"SHELL", "Transporte"}, {"IPIRANGA", "Transporte"}, {"PETROBRAS", "Transporte"},
	{"COMBUSTIVEL", "Transporte"}, {"ESTACIONAMENTO", "Transporte"}, {"ESTAPAR", "Transporte"},
	{"SEM PARAR", "Transporte"}, {"CONECTCAR", "Transporte"}, {"VELOE", "Transporte"}, {"PEDAGIO", "Transporte"},
	{"METRO", "Transporte"}, {"BILHETE UNICO", "Transporte"}, {"AUTO PECAS", "Transporte"},
	{"FARMACIA", "Saúde"}, {"DROGASIL", "Saúde"}, {"DROGA RAIA", "Saúde"}, {"RAIA", "Saúde"},
	{"PAGUE MENOS", "Saúde"}, {"DROGARIA", "Saúde"}, {"HOSPITAL", "Saúde"}, {"CLINICA", "Saúde"},
	{"LABORATORIO", "Saúde"}, {"UNIMED", "Saúde"}, {"ODONTO", "Saúde"}, {"DENTISTA", "Saúde"},
	{"NETFLIX", "Serviços e assinaturas"}, {"SPOTIFY", "Serviços e assinaturas"},
	{"AMAZON PRIME", "Serviços e assinaturas"}, {"PRIME VIDEO", "Serviços e assinaturas"},
	{"DISNEY", "Serviços e assinaturas"}, {"HBO", "Serviços e assinaturas"}, {"MAX.COM", "Serviços e assinaturas"},
	{"YOUTUBE", "Serviços e assinaturas"}, {"APPLE.COM", "Serviços e assinaturas"},
	{"GOOGLE", "Serviços e assinaturas"}, {"MICROSOFT", "Serviços e assinaturas"},
	{"CLARO", "Serviços e assinaturas"}, {"VIVO", "Serviços e assinaturas"}, {"TIM ", "Serviços e assinaturas"},
	{"OPENAI", "Serviços e assinaturas"}, {"CHATGPT", "Serviços e assinaturas"}, {"CLAUDE", "Serviços e assinaturas"},
	{"ANTHROPIC", "Serviços e assinaturas"}, {"GLOBOPLAY", "Serviços e assinaturas"},
	{"ALUGUEL", "Moradia"}, {"CONDOMINIO", "Moradia"}, {"ENEL", "Moradia"}, {"SABESP", "Moradia"},
	{"CEMIG", "Moradia"}, {"COPEL", "Moradia"}, {"LIGHT", "Moradia"}, {"COMGAS", "Moradia"},
	{"ENERGIA", "Moradia"}, {"SANEAMENTO", "Moradia"}, {"CPFL", "Moradia"}, {"NEOENERGIA", "Moradia"},
	{"AMAZON", "Compras"}, {"MERCADOLIVRE", "Compras"}, {"MERCADO LIVRE", "Compras"}, {"MAGALU", "Compras"},
	{"MAGAZINE LUIZA", "Compras"}, {"SHOPEE", "Compras"}, {"ALIEXPRESS", "Compras"}, {"SHEIN", "Compras"},
	{"RENNER", "Compras"}, {"RIACHUELO", "Compras"}, {"CENTAURO", "Compras"}, {"KABUM", "Compras"},
	{"CASAS BAHIA", "Compras"}, {"AMERICANAS", "Compras"}, {"LEROY", "Compras"},
	{"ESCOLA", "Educação"}, {"FACULDADE", "Educação"}, {"UNIVERSIDADE", "Educação"}, {"CURSO", "Educação"},
	{"UDEMY", "Educação"}, {"ALURA", "Educação"}, {"LIVRARIA", "Educação"},
	{"CINEMA", "Lazer"}, {"CINEMARK", "Lazer"}, {"INGRESSO", "Lazer"}, {"SYMPLA", "Lazer"}, {"STEAM", "Lazer"},
	{"PLAYSTATION", "Lazer"}, {"XBOX", "Lazer"}, {"NINTENDO", "Lazer"},
	{"HOTEL", "Viagens"}, {"AIRBNB", "Viagens"}, {"BOOKING", "Viagens"}, {"LATAM", "Viagens"},
	{"GOL LINHAS", "Viagens"}, {"AZUL LINHAS", "Viagens"}, {"DECOLAR", "Viagens"}, {"123MILHAS", "Viagens"},
	{"PETZ", "Pets"}, {"COBASI", "Pets"}, {"PET SHOP", "Pets"}, {"PETSHOP", "Pets"}, {"VETERINAR", "Pets"},
	{"SALAO", "Cuidados pessoais"}, {"BARBEARIA", "Cuidados pessoais"}, {"ACADEMIA", "Cuidados pessoais"},
	{"SMART FIT", "Cuidados pessoais"}, {"BOTICARIO", "Cuidados pessoais"}, {"NATURA", "Cuidados pessoais"},
	{"IPVA", "Impostos e taxas"}, {"IPTU", "Impostos e taxas"}, {"DARF", "Impostos e taxas"},
	{"DETRAN", "Impostos e taxas"}, {"LICENCIAMENTO", "Impostos e taxas"},
	{"TARIFA", "Tarifas bancárias"}, {"ANUIDADE", "Tarifas bancárias"}, {"IOF", "Tarifas bancárias"},
	{"CESTA DE SERVICOS", "Tarifas bancárias"},
	{"SEGURO", "Seguros"}, {"PORTO SEGURO", "Seguros"},
	{"JUROS", "Juros e empréstimos"}, {"ENCARGOS", "Juros e empréstimos"}, {"EMPRESTIMO", "Juros e empréstimos"},
	{"SALARIO", "Salário"}, {"PROVENTOS", "Rendimentos"}, {"RENDIMENTO", "Rendimentos"},
	{"PAGAMENTO FATURA", "Pagamento de fatura"}, {"PAG FATURA", "Pagamento de fatura"},
}

// votos são os pontos de cada categoria, na ordem em que cada uma apareceu: o Dictionary do C# enumera na ordem de
// inserção, e essa ordem desempata as categorias de mesmos pontos.
type votos struct {
	ordem  []int64
	pontos map[int64]float64
}

func (v *votos) somar(categoria int64, peso float64) {
	if _, ok := v.pontos[categoria]; !ok {
		v.ordem = append(v.ordem, categoria)
	}
	v.pontos[categoria] += peso
}

func (v *votos) total() float64 {
	t := 0.0
	for _, c := range v.ordem {
		t += v.pontos[c]
	}
	return t
}

// emOrdem: as categorias dos mais pontos para os menos (OrderByDescending, estável na ordem de inserção).
func (v *votos) emOrdem() []int64 {
	ordem := slices.Clone(v.ordem)
	slices.SortStableFunc(ordem, func(a, b int64) int {
		switch pa, pb := v.pontos[a], v.pontos[b]; {
		case pa > pb:
			return -1
		case pa < pb:
			return 1
		}
		return 0
	})
	return ordem
}

type categoria struct {
	id        int64
	nome, cor string
	tipo      int64
}

type textoDaCategoria struct {
	texto     string
	categoria int64
}

// indice é o histórico de como você categorizou: por chave (as 3 primeiras palavras) e pela primeira palavra.
type indice struct {
	porChave   map[string]*votos
	porPalavra map[string]*votos
	categorias map[int64]categoria
	porNome    map[string]int64
	textos     []textoDaCategoria // #254: o texto que as regras testam e a categoria de cada transação
	regras     map[string]int64   // #254: as regras "contém" que já existem: texto normalizado → categoria
}

func somar(mapa map[string]*votos, chave string, categoria int64, peso float64) {
	v, ok := mapa[chave]
	if !ok {
		v = &votos{pontos: map[int64]float64{}}
		mapa[chave] = v
	}
	v.somar(categoria, peso)
}

func primeiro(s ...*string) string {
	for _, v := range s {
		if v != nil {
			return *v
		}
	}
	return ""
}

// montarIndice é o MontarIndiceAsync: as categorias, as regras "contém" e as transações categorizadas dos últimos 730
// dias. As consultas são as do C#, sem ORDER BY: a ordem do banco desempata as categorias de mesmos pontos, e a mesma
// consulta no mesmo SQLite dá a mesma ordem.
func montarIndice(ctx context.Context, q consultas.DBTX, hoje tipos.Data) (*indice, error) {
	ix := &indice{porChave: map[string]*votos{}, porPalavra: map[string]*votos{}, categorias: map[int64]categoria{},
		porNome: map[string]int64{}, regras: map[string]int64{}}
	linhas, err := q.QueryContext(ctx, `SELECT "c"."Id", "c"."Nome", "c"."Cor", "c"."Tipo" FROM "Categoria" AS "c"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias: %w", err)
	}
	for linhas.Next() {
		var c categoria
		if err := linhas.Scan(&c.id, &c.nome, &c.cor, &c.tipo); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		ix.categorias[c.id] = c
		if _, ok := ix.porNome[texto.ParaRegras(c.nome)]; !ok {
			ix.porNome[texto.ParaRegras(c.nome)] = c.id
		}
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as categorias: %w", err)
	}

	linhas, err = q.QueryContext(ctx, `SELECT "r"."Texto", "r"."CategoriaId" FROM "RegraCategorizacao" AS "r"
		WHERE "r"."TipoCorrespondencia" = 1`)
	if err != nil {
		return nil, fmt.Errorf("ler as regras: %w", err)
	}
	for linhas.Next() {
		var t string
		var c int64
		if err := linhas.Scan(&t, &c); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler regra: %w", err)
		}
		if _, ok := ix.regras[texto.ParaRegras(t)]; !ok {
			ix.regras[texto.ParaRegras(t)] = c
		}
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as regras: %w", err)
	}

	linhas, err = q.QueryContext(ctx, `SELECT "t"."Descricao", "t"."DescricaoOriginal", "t"."Estabelecimento",
		"t"."CategoriaId", "t"."CategoriaManual" FROM "Transacao" AS "t" WHERE "t"."Data" >= ? AND "t"."CategoriaId" IS NOT NULL`,
		hoje.SomarDias(-diasHistorico))
	if err != nil {
		return nil, fmt.Errorf("ler o histórico: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var descricao string
		var original, estabelecimento *string
		var c int64
		var manual bool
		if err := linhas.Scan(&descricao, &original, &estabelecimento, &c, &manual); err != nil {
			return nil, fmt.Errorf("ler transação do histórico: %w", err)
		}
		peso := 1.0
		if manual {
			peso = 3 // escolhas suas valem mais que as automáticas
		}
		t := primeiro(estabelecimento, original, &descricao)
		if chave := Chave(t); chave != "" {
			somar(ix.porChave, chave, c, peso)
		}
		if palavras := Palavras(t); len(palavras) > 0 && tamanho(palavras[0]) >= 4 {
			somar(ix.porPalavra, palavras[0], c, peso)
		}
		ix.textos = append(ix.textos, textoDaCategoria{texto.ParaRegras(primeiro(original, &descricao)), c})
	}
	return ix, linhas.Err()
}

// OpcaoDto é o OpcaoCategoriaDto do C#.
type OpcaoDto struct {
	CategoriaId int64  `json:"categoriaId"`
	Nome        string `json:"nome"`
	Cor         string `json:"cor"`
	Motivo      string `json:"motivo"`
}

type opcao struct {
	id     int64
	pontos float64
	motivo string
}

// avaliar é o Avaliar do C#: as até 3 opções (a melhor primeiro) e a confiança na primeira. Pesam o histórico da
// mesma chave (3), o da primeira palavra (1,5) e a primeira palavra-chave conhecida (1). Receita não sugere categoria
// de despesa, e despesa não sugere de receita.
func (ix *indice) avaliar(t string, receita bool, ignorar *int64) ([]OpcaoDto, string) {
	var opcoes []opcao
	confianca := "nenhuma"
	adicionar := func(v *votos, origem string, fator float64) {
		total := v.total()
		parecidas := max(1, int(math.RoundToEven(total)))
		for _, id := range v.emOrdem() {
			razao := v.pontos[id] / total
			opcoes = append(opcoes, opcao{id, razao * fator,
				origem + " (" + percentual(razao) + " das " + strconv.Itoa(parecidas) + " parecidas)"})
		}
	}
	if chave := Chave(t); chave != "" {
		if v, ok := ix.porChave[chave]; ok {
			adicionar(v, "Você já usou para transações iguais", 3)
			total, melhor := v.total(), 0.0
			for _, c := range v.ordem {
				melhor = max(melhor, v.pontos[c])
			}
			confianca = "media"
			if melhor/total >= 0.7 && total >= 2 {
				confianca = "alta"
			}
		}
	}
	if palavras := Palavras(t); len(palavras) > 0 {
		if v, ok := ix.porPalavra[palavras[0]]; ok {
			adicionar(v, `Usada em transações com "`+palavras[0]+`"`, 1.5)
			if confianca == "nenhuma" {
				confianca = "media"
			}
		}
	}
	normalizado := " " + texto.ParaRegras(t) + " "
	for _, pc := range palavrasChave {
		if !strings.Contains(normalizado, pc[0]) {
			continue
		}
		id, ok := ix.porNome[texto.ParaRegras(pc[1])]
		if !ok {
			continue
		}
		opcoes = append(opcoes, opcao{id, 1, `Palavra-chave "` + strings.TrimSpace(pc[0]) + `"`})
		if confianca == "nenhuma" {
			confianca = "baixa"
		}
		break
	}

	tipoQueNaoServe := int64(2) // receita
	if receita {
		tipoQueNaoServe = 1 // despesa
	}
	// GroupBy (na ordem da primeira) com a soma dos pontos, na ordem em que vieram.
	var grupos []opcao
	for _, o := range opcoes {
		c, ok := ix.categorias[o.id]
		if !ok || (ignorar != nil && o.id == *ignorar) || c.tipo == tipoQueNaoServe {
			continue
		}
		i := slices.IndexFunc(grupos, func(g opcao) bool { return g.id == o.id })
		if i < 0 {
			grupos = append(grupos, opcao{o.id, o.pontos, o.motivo})
			continue
		}
		grupos[i].pontos += o.pontos
	}
	// O motivo do grupo é o da opção de mais pontos dele.
	for i := range grupos {
		maior := -1.0
		for _, o := range opcoes {
			if o.id == grupos[i].id && o.pontos > maior {
				maior, grupos[i].motivo = o.pontos, o.motivo
			}
		}
	}
	slices.SortStableFunc(grupos, func(a, b opcao) int {
		switch {
		case a.pontos > b.pontos:
			return -1
		case a.pontos < b.pontos:
			return 1
		}
		return 0
	})
	lista := []OpcaoDto{}
	for _, g := range grupos[:min(3, len(grupos))] {
		c := ix.categorias[g.id]
		lista = append(lista, OpcaoDto{c.id, c.nome, c.cor, g.motivo})
	}
	if len(lista) == 0 {
		return lista, "nenhuma"
	}
	return lista, confianca
}

// avisoRegra (#254): por que não criar a regra sugerida sem olhar: ela já existe, ou o texto também aparece em
// transações de outras categorias (a regra mudaria as próximas assim). Nil quando não há nada a avisar.
func (ix *indice) avisoRegra(t string, categoria *int64) *string {
	if t == "" {
		return nil
	}
	aviso := func(s string) *string { return &s }
	if existente, ok := ix.regras[t]; ok {
		if categoria != nil && existente == *categoria {
			return aviso(`A regra "` + t + `" já existe.`)
		}
		return aviso(`Já existe a regra "` + t + `" para ` + ix.nomeCategoria(existente) + ".")
	}
	achadas := 0
	outras := &votos{pontos: map[int64]float64{}}
	for _, tc := range ix.textos {
		if !strings.Contains(tc.texto, t) {
			continue
		}
		achadas++
		if categoria == nil || tc.categoria != *categoria {
			outras.somar(tc.categoria, 1)
		}
	}
	conflitos := int(outras.total())
	if conflitos < conflitosParaAviso || float64(conflitos) < float64(achadas)*fracaoConflitosParaAviso {
		return nil
	}
	var exemplos []string
	for _, c := range outras.emOrdem()[:min(2, len(outras.ordem))] {
		exemplos = append(exemplos, ix.nomeCategoria(c))
	}
	return aviso(`"` + t + `" também aparece em ` + strconv.Itoa(conflitos) + " transações de outras categorias (" +
		strings.Join(exemplos, ", ") + ").")
}

func (ix *indice) nomeCategoria(id int64) string {
	if c, ok := ix.categorias[id]; ok {
		return c.nome
	}
	return "outra categoria"
}

// SugestaoDto é o SugestaoCategoriaDto do C#.
type SugestaoDto struct {
	Chave         string        `json:"chave"`
	Exemplo       string        `json:"exemplo"`
	Quantidade    int           `json:"quantidade"`
	Total         tipos.Decimal `json:"total"`
	TransacaoIds  []int64       `json:"transacaoIds"`
	CategoriaId   *int64        `json:"categoriaId"`
	CategoriaNome *string       `json:"categoriaNome"`
	CategoriaCor  *string       `json:"categoriaCor"`
	Confianca     string        `json:"confianca"`
	Motivo        *string       `json:"motivo"`
	TextoRegra    string        `json:"textoRegra"`
	Alternativas  []OpcaoDto    `json:"alternativas"`
	AvisoRegra    *string       `json:"avisoRegra"`
}

type transacao struct {
	id                        int64
	descricao                 string
	original, estabelecimento *string
	valor                     tipos.Decimal
	categoria                 *int64
}

// Grupos é o SugerirGruposAsync: as transações dos últimos dias (sem as transferências do sistema e as de categoria
// escolhida à mão), agrupadas pela descrição parecida, com a sugestão de cada grupo. Primeiro o que dá para aplicar
// com segurança (a confiança alta), depois os grupos maiores.
func Grupos(ctx context.Context, q consultas.DBTX, hoje tipos.Data, somenteSemCategoria bool, dias int) ([]SugestaoDto, error) {
	ix, err := montarIndice(ctx, q, hoje)
	if err != nil {
		return nil, err
	}
	filtro := ""
	if somenteSemCategoria {
		filtro = ` AND "t"."CategoriaId" IS NULL`
	}
	linhas, err := q.QueryContext(ctx, `SELECT "t"."Id", "t"."Descricao", "t"."DescricaoOriginal", "t"."Estabelecimento",
		"t"."Valor", "t"."CategoriaId" FROM "Transacao" AS "t"
		WHERE "t"."Data" >= ? AND "t"."TransferenciaGrupo" IS NULL AND NOT ("t"."CategoriaManual")`+filtro+`
		ORDER BY "t"."Data" DESC LIMIT ?`, hoje.SomarDias(-dias), maximoTransacoes)
	if err != nil {
		return nil, fmt.Errorf("ler as transações para sugerir: %w", err)
	}
	type grupo struct {
		chave string
		itens []transacao
		soma  tipos.Decimal // dos valores sem o sinal, para desempatar
	}
	var grupos []*grupo
	porChave := map[string]*grupo{}
	for linhas.Next() {
		var t transacao
		if err := linhas.Scan(&t.id, &t.descricao, &t.original, &t.estabelecimento, &t.valor, &t.categoria); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler transação para sugerir: %w", err)
		}
		chave := Chave(primeiro(t.estabelecimento, t.original, &t.descricao))
		if chave == "" {
			chave = texto.MaiusculasInvariantes(strings.TrimSpace(t.descricao))
		}
		g, ok := porChave[chave]
		if !ok {
			g = &grupo{chave: chave}
			porChave[chave] = g
			grupos = append(grupos, g)
		}
		g.itens = append(g.itens, t)
		g.soma = tipos.Somar(g.soma, tipos.DeDecimal(t.valor.Decimal().Abs()))
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as transações para sugerir: %w", err)
	}
	slices.SortStableFunc(grupos, func(a, b *grupo) int {
		if c := len(b.itens) - len(a.itens); c != 0 {
			return c
		}
		return b.soma.Decimal().Cmp(a.soma.Decimal())
	})
	grupos = grupos[:min(maximoGrupos, len(grupos))]

	resultado := []SugestaoDto{}
	for _, g := range grupos {
		exemplo := g.itens[0]
		// Se todas já estão na categoria sugerida, não há o que sugerir.
		var categoriaAtual *int64
		unica := true
		for _, t := range g.itens[1:] {
			if !mesmoId(t.categoria, exemplo.categoria) {
				unica = false
				break
			}
		}
		if unica {
			categoriaAtual = exemplo.categoria
		}
		opcoes, confianca := ix.avaliar(primeiro(exemplo.estabelecimento, exemplo.original, &exemplo.descricao),
			exemplo.valor.Decimal().IsPositive(), nil)
		if !somenteSemCategoria && (len(opcoes) == 0 || (categoriaAtual != nil && opcoes[0].CategoriaId == *categoriaAtual)) {
			continue
		}
		// #254: a regra testa a descrição do banco (o Resolver das regras), e não o estabelecimento da chave.
		descricoes := make([]string, len(g.itens))
		total := tipos.Zero
		ids := []int64{}
		for i, t := range g.itens {
			descricoes[i] = primeiro(t.original, &t.descricao)
			total = tipos.Somar(total, t.valor)
			if i < 1000 {
				ids = append(ids, t.id)
			}
		}
		textoRegra := TextoRegra(descricoes)
		d := SugestaoDto{Chave: g.chave, Exemplo: exemplo.descricao, Quantidade: len(g.itens), Total: tipos.Arredondar(total, 2),
			TransacaoIds: ids, Confianca: confianca, TextoRegra: textoRegra, Alternativas: []OpcaoDto{}}
		var principal *int64
		if len(opcoes) > 0 {
			o := opcoes[0]
			principal = &o.CategoriaId
			d.CategoriaId, d.CategoriaNome, d.CategoriaCor, d.Motivo = &o.CategoriaId, &o.Nome, &o.Cor, &o.Motivo
			d.Alternativas = opcoes[1:]
		}
		d.AvisoRegra = ix.avisoRegra(textoRegra, principal)
		resultado = append(resultado, d)
	}
	// O C# ordena pela confiança e depois pela quantidade (ThenByDescending). A segunda não muda nada: os grupos já
	// vêm do maior para o menor, e a ordenação é estável.
	ordem := map[string]int{"alta": 0, "media": 1, "baixa": 2, "nenhuma": 3}
	slices.SortStableFunc(resultado, func(a, b SugestaoDto) int { return ordem[a.Confianca] - ordem[b.Confianca] })
	return resultado, nil
}

func mesmoId(a, b *int64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// ErrNaoEncontrada: a transação não existe.
var ErrNaoEncontrada = errors.New("transação não encontrada")

// ParaTransacao é o SugerirParaTransacaoAsync: até 3 categorias para uma transação, menos a que ela já tem.
func ParaTransacao(ctx context.Context, q consultas.DBTX, hoje tipos.Data, id int64) ([]OpcaoDto, error) {
	var t transacao
	err := q.QueryRowContext(ctx, `SELECT "Descricao", "DescricaoOriginal", "Estabelecimento", "Valor", "CategoriaId"
		FROM "Transacao" WHERE "Id" = ?`, id).Scan(&t.descricao, &t.original, &t.estabelecimento, &t.valor, &t.categoria)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNaoEncontrada
	}
	if err != nil {
		return nil, fmt.Errorf("ler a transação: %w", err)
	}
	ix, err := montarIndice(ctx, q, hoje)
	if err != nil {
		return nil, err
	}
	opcoes, _ := ix.avaliar(primeiro(t.estabelecimento, t.original, &t.descricao), t.valor.Decimal().IsPositive(), t.categoria)
	return opcoes, nil
}
