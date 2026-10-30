package extrato

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/desfazer"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/regrasdescricao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Os tipos de correspondência das regras (o enum TipoCorrespondencia do C#), com os nomes da query em minúsculas.
const (
	contem    = 1
	comecaCom = 2
	igual     = 3
)

var nomesDaCorrespondencia = map[string]int64{"contem": contem, "comecacom": comecaCom, "igual": igual}

// PreviaRenomearDto é o do C#.
type PreviaRenomearDto struct {
	DescricaoOriginal *string  `json:"descricaoOriginal"`
	TextoSugerido     string   `json:"textoSugerido"`
	Quantidade        int      `json:"quantidade"`
	Exemplos          []string `json:"exemplos"`
}

// ResultadoRenomearDto é o do C#: quantas mudaram, a regra criada e a chave do Desfazer.
type ResultadoRenomearDto struct {
	Alteradas int     `json:"alteradas"`
	RegraId   *int64  `json:"regraId"`
	Desfazer  *string `json:"desfazer"`
}

// ResultadoRenomearEmLoteDto é o do C#.
type ResultadoRenomearEmLoteDto struct {
	Alteradas       int     `json:"alteradas"`
	RegrasCriadas   int     `json:"regrasCriadas"`
	RegrasAlteradas int     `json:"regrasAlteradas"`
	Desfazer        *string `json:"desfazer"`
}

// nomeAntes é o EstadoAnterior do RegraDescricaoService: o nome de uma transação antes de mudar.
type nomeAntes struct {
	id              int64
	descricao       string
	regra           *int64
	descricaoManual bool
}

func nomeDe(t *transacoes.Transacao) nomeAntes {
	return nomeAntes{t.Id, t.Descricao, t.RegraDescricaoId, t.DescricaoManual}
}

func (a nomeAntes) mudou(t *transacoes.Transacao) bool {
	return a.descricao != t.Descricao || !mesmoId(a.regra, t.RegraDescricaoId) || a.descricaoManual != t.DescricaoManual
}

// regraAntes é a regra de descrição que o renomear em lote reaproveitou, com o nome e a situação de antes.
type regraAntes struct {
	id            int64
	novaDescricao string
	ativa         bool
}

// desfazerNome é o que o Desfazer de um renomear volta: os nomes, as regras criadas e as regras alteradas.
type desfazerNome struct {
	anteriores []nomeAntes
	criadas    []int64
	alteradas  []regraAntes
}

func mesmoId(a, b *int64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// reaplicar é o ReaplicarAsync do C# com as transações que a rota já leu: o EF Core devolve a mesma instância
// rastreada, com o que a rota já mudou na memória, para a linha que a consulta acha. A consulta é a do banco (o que
// ainda não foi gravado não muda quais linhas vêm). Devolve o nome de antes das outras que mudaram.
func reaplicar(r *http.Request, tx *sql.Tx, agora tipos.DataHora, proprias []*transacoes.Transacao) ([]nomeAntes, error) {
	rs, err := regrasdescricao.Carregar(r.Context(), tx)
	if err != nil {
		return nil, err
	}
	lista, err := regrasdescricao.NomeAutomaticoNoBanco(r.Context(), tx)
	if err != nil {
		return nil, err
	}
	porId := map[int64]*transacoes.Transacao{}
	for _, t := range proprias {
		porId[t.Id] = t
	}
	var mudaram []nomeAntes
	var gravar []*transacoes.Transacao
	for _, t := range lista {
		if p, ok := porId[t.Id]; ok {
			t = p
		}
		antes := nomeDe(t)
		if !rs.Aplicar(t) {
			continue
		}
		t.AtualizadoEm = agora
		if _, ok := porId[t.Id]; !ok {
			mudaram = append(mudaram, antes)
			gravar = append(gravar, t)
		}
	}
	return mudaram, transacoes.GravarTodas(r.Context(), tx, gravar)
}

// previaRenomear é o GET /api/transacoes/{id:long}/renomear/previa: quantas e quais (até 3) transações de nome
// automático o "usar também nas outras" renomearia, com o texto e o tipo escolhidos (sem texto: a descrição do banco,
// com "é igual a", ou o texto sugerido, com os outros tipos).
func (s *Servico) previaRenomear(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	tipo := int64(igual)
	// Vazio também é erro: o tipo não aceita null ("The value '' is invalid.").
	if v, ok := respostas.ParametroDaQuery(r, "tipo"); ok {
		switch n, converte, definido := respostas.EnumDaQuery(v, nomesDaCorrespondencia); {
		case !converte:
			respostas.Validacao(w, map[string][]string{"tipo": {respostas.MensagemDaQuery(v)}})
			return
		case !definido:
			respostas.Validacao(w, map[string][]string{"tipo": {respostas.MensagemDeValorInvalido(v)}})
			return
		default:
			tipo = n
		}
	}
	var original *string
	err := s.db.QueryRowContext(r.Context(), `SELECT "DescricaoOriginal" FROM "Transacao" WHERE "Id" = ?`, id).Scan(&original)
	if errors.Is(err, sql.ErrNoRows) {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	if err != nil {
		s.erroInterno(w, r, "ler a transação", err)
		return
	}
	if original == nil {
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, PreviaRenomearDto{Exemplos: []string{}})
		return
	}
	sugerido := regrasdescricao.SugerirTexto(*original)
	alvo := sugerido
	if v, _ := respostas.ParametroDaQuery(r, "texto"); strings.TrimSpace(v) != "" {
		alvo = strings.TrimSpace(v)
	} else if tipo == igual {
		alvo = *original
	}
	quantidade, exemplos, err := regrasdescricao.Previa(r.Context(), s.db, alvo, tipo, id, nil, nil)
	if err != nil {
		s.erroInterno(w, r, "montar a prévia", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, PreviaRenomearDto{original, sugerido, quantidade, exemplos})
}

// letrasOuDigitos conta, no texto normalizado como nas regras, os chars (UTF-16) que são letra ou dígito decimal (o
// Count(char.IsLetterOrDigit) do C#).
func letrasOuDigitos(t string) int {
	n := 0
	for _, r := range texto.ParaRegras(t) {
		if r < 0x10000 && (unicode.IsLetter(r) || unicode.Is(unicode.Nd, r)) {
			n++
		}
	}
	return n
}

// erroDeRenomear é a mensagem do 400 que o renomear devolve depois de abrir a transação do banco (que volta: a regra
// criada sai junto).
type erroDeRenomear string

func (e erroDeRenomear) Error() string { return string(e) }

// renomear é o PUT /api/transacoes/{id:long}/descricao (#123, ADR 0030): o nome novo só nesta (escolhido à mão), de
// volta ao do banco, ou como regra que vale nas outras de nome automático e nas próximas. Tudo numa transação do banco:
// o erro depois de criar a regra a desfaz junto. O Desfazer guarda o nome de antes de cada transação que mudou e a
// regra criada.
func (s *Servico) renomear(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	descricaoCampo := respostas.CampoTexto{Propriedade: "Descricao", Maximo: 300, Opcional: true}
	voltar := respostas.CampoBool{Propriedade: "VoltarOriginal"}
	nasOutras := respostas.CampoBool{Propriedade: "AplicarNasOutras"}
	tipoCampo := respostas.CampoEnum{Propriedade: "TipoCorrespondencia"}
	textoCampo := respostas.CampoTexto{Propriedade: "Texto", Maximo: 200, Opcional: true}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.RenomearTransacaoDto", &descricaoCampo, &voltar, &nasOutras,
		&tipoCampo, &textoCampo) {
		return
	}
	t, err := ler(r.Context(), s.db, id)
	if err != nil {
		s.erroInterno(w, r, "ler a transação", err)
		return
	}
	if t == nil {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	descricao := ""
	if descricaoCampo.Valor != nil {
		descricao = strings.TrimSpace(*descricaoCampo.Valor)
	}
	tipo := tipoCampo.Ou(igual)
	switch {
	case !voltar.Valor && descricao == "":
		respostas.Mensagem(w, http.StatusBadRequest, "Escreva a nova descrição.")
		return
	case nasOutras.Valor && t.DescricaoOriginal == nil:
		respostas.Mensagem(w, http.StatusBadRequest, "Só transações que vieram do banco podem virar regra: esta foi lançada à mão.")
		return
	case tipo < contem || tipo > igual:
		respostas.Mensagem(w, http.StatusBadRequest, "Tipo de correspondência inválido.")
		return
	}
	var resultado ResultadoRenomearDto
	var estado desfazerNome
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		agora := tipos.NovaDataHora(s.relogio.Agora())
		antes := nomeDe(t)
		var regra *int64
		var outras []nomeAntes
		switch {
		case voltar.Valor:
			if t.DescricaoOriginal != nil {
				t.Descricao = *t.DescricaoOriginal
			}
			t.DescricaoManual, t.RegraDescricaoId = true, nil
		case nasOutras.Valor:
			textoRegra := *t.DescricaoOriginal
			if textoCampo.Valor != nil && strings.TrimSpace(*textoCampo.Valor) != "" {
				textoRegra = strings.TrimSpace(*textoCampo.Valor)
			}
			if letrasOuDigitos(textoRegra) < 2 {
				return erroDeRenomear("O texto da regra precisa de pelo menos 2 letras ou números.")
			}
			res, err := tx.ExecContext(r.Context(), `INSERT INTO "RegraDescricao" ("Texto", "TipoCorrespondencia",
				"NovaDescricao", "ValorMinimo", "ValorMaximo", "Ativa", "CriadoEm") VALUES (?, ?, ?, NULL, NULL, 1, ?)`,
				textoRegra, tipo, descricao, agora)
			if err != nil {
				return err
			}
			novo, err := res.LastInsertId()
			if err != nil {
				return err
			}
			regra = &novo
			// A própria transação passa a ter o nome pela regra, que tem de valer para ela.
			t.DescricaoManual, t.RegraDescricaoId, t.Descricao = false, nil, *t.DescricaoOriginal
			if outras, err = reaplicar(r, tx, agora, []*transacoes.Transacao{t}); err != nil {
				return err
			}
			// #250: a regra nova pode ser seguida de outra na cadeia; o que importa é ela valer para a transação.
			rs, err := regrasdescricao.Carregar(r.Context(), tx)
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(rs.Cadeia(t.DescricaoOriginal, t.Valor), func(p regrasdescricao.Passo) bool { return p.Id == novo }) {
				if t.RegraDescricaoId == nil {
					return erroDeRenomear(`A descrição do banco desta transação ("` + *t.DescricaoOriginal + `") não bate com "` + textoRegra + `".`)
				}
				var outra string
				if err := tx.QueryRowContext(r.Context(), `SELECT "Texto" FROM "RegraDescricao" WHERE "Id" = ?`,
					*t.RegraDescricaoId).Scan(&outra); err != nil {
					return err
				}
				return erroDeRenomear(`Já existe uma regra mais específica para esta descrição ("` + outra + `"). Edite-a em Categorias › Regras automáticas.`)
			}
		default:
			t.Descricao, t.DescricaoManual, t.RegraDescricaoId = descricao, true, nil
		}
		mudaram := outras
		if antes.mudou(t) {
			t.AtualizadoEm = agora
			mudaram = append([]nomeAntes{antes}, outras...)
		}
		evento := "TransacaoRenomeada"
		if regra != nil {
			evento = "RegraDescricaoCriada (" + strconv.Itoa(len(mudaram)) + ")"
		}
		q := consultas.New(tx)
		login, err := auditoria.Login(r.Context(), q)
		if err != nil {
			return err
		}
		if err := auditoria.Registrar(r.Context(), q, r, agora, evento, login); err != nil {
			return err
		}
		if err := transacoes.Gravar(r.Context(), tx, t); err != nil {
			return err
		}
		resultado = ResultadoRenomearDto{Alteradas: len(mudaram), RegraId: regra}
		estado = desfazerNome{anteriores: mudaram}
		if regra != nil {
			estado.criadas = []int64{*regra}
		}
		return nil
	})
	var mensagem erroDeRenomear
	switch {
	case errors.As(err, &mensagem):
		respostas.Mensagem(w, http.StatusBadRequest, string(mensagem))
		return
	case err != nil:
		s.erroInterno(w, r, "renomear a transação", err)
		return
	}
	// A chave só depois de gravar, como no C#.
	if resultado.Alteradas > 0 || resultado.RegraId != nil {
		chave := s.memoriaNomes.Guardar(estado)
		resultado.Desfazer = &chave
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
}

// maximoRegrasPorLote é o MaximoRegrasPorLote do C#: regras criadas de uma vez pelo renomear em lote.
const maximoRegrasPorLote = 50

// renomearEmLote é o PATCH /api/transacoes/descricao/em-lote (#123): o mesmo nome em várias transações, só nelas
// (escolhido à mão) ou como regras "é igual a", uma por descrição do banco, que valem nas outras de nome automático e
// nas próximas. A regra que já existe com esse texto (sem faixa de valor, #241) passa a usar o nome novo.
func (s *Servico) renomearEmLote(w http.ResponseWriter, r *http.Request) {
	descricaoCampo := respostas.CampoTexto{Propriedade: "Descricao", Minimo: 1, Maximo: 300}
	criar := respostas.CampoBool{Propriedade: "CriarRegras"}
	ids, ok := lerIds(w, r, "Financas.Api.Dtos.RenomearEmLoteDto", &descricaoCampo, &criar)
	if !ok {
		return
	}
	em, argumentos := emLista(ids)
	selecionadas, err := transacoes.Listar(r.Context(), s.db, `WHERE "Id" IN `+em, argumentos...)
	if err != nil {
		s.erroInterno(w, r, "ler as transações", err)
		return
	}
	if len(selecionadas) != len(ids) {
		respostas.Mensagem(w, http.StatusNotFound, "Alguma transação não existe mais. Atualize a lista.")
		return
	}
	// O "Escreva o novo nome." do C# não chega a valer: o [Required] já recusa o nome só com espaços.
	nome := strings.TrimSpace(*descricaoCampo.Valor)
	// Uma descrição do banco por texto normalizado, a primeira de cada (o GroupBy do C#).
	var originais []string
	if criar.Valor {
		vistos := map[string]bool{}
		for _, t := range selecionadas {
			if t.DescricaoOriginal == nil {
				continue
			}
			if n := texto.ParaRegras(*t.DescricaoOriginal); !vistos[n] {
				vistos[n] = true
				originais = append(originais, *t.DescricaoOriginal)
			}
		}
	}
	if len(originais) > maximoRegrasPorLote {
		respostas.Mensagem(w, http.StatusBadRequest, "As selecionadas têm "+strconv.Itoa(len(originais))+
			" descrições do banco diferentes; o limite é "+strconv.Itoa(maximoRegrasPorLote)+" regras de uma vez. "+
			`Selecione menos, ou crie uma regra "contém" em Categorias › Regras automáticas.`)
		return
	}
	var estado desfazerNome
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		agora := tipos.NovaDataHora(s.relogio.Agora())
		anteriores := map[int64]nomeAntes{}
		for _, t := range selecionadas {
			anteriores[t.Id] = nomeDe(t)
		}
		if err := s.regrasDoLote(r, tx, agora, originais, nome, &estado); err != nil {
			return err
		}
		for _, t := range selecionadas {
			if len(originais) > 0 && t.DescricaoOriginal != nil {
				// O nome vem da regra: a transação volta ao nome automático e a reaplicação abaixo dá o nome.
				t.DescricaoManual, t.RegraDescricaoId, t.Descricao = false, nil, *t.DescricaoOriginal
			} else {
				t.Descricao, t.DescricaoManual, t.RegraDescricaoId = nome, true, nil
			}
		}
		var outras []nomeAntes
		if len(originais) > 0 {
			var err error
			if outras, err = reaplicar(r, tx, agora, selecionadas); err != nil {
				return err
			}
		}
		for _, t := range selecionadas {
			if a := anteriores[t.Id]; a.mudou(t) {
				t.AtualizadoEm = agora
				estado.anteriores = append(estado.anteriores, a)
			}
		}
		estado.anteriores = append(estado.anteriores, outras...)
		q := consultas.New(tx)
		login, err := auditoria.Login(r.Context(), q)
		if err != nil {
			return err
		}
		evento := "RenomearEmLote (" + strconv.Itoa(len(estado.anteriores)) + ", " + strconv.Itoa(len(estado.criadas)) + " regra)"
		if err := auditoria.Registrar(r.Context(), q, r, agora, evento, login); err != nil {
			return err
		}
		return transacoes.GravarTodas(r.Context(), tx, selecionadas)
	})
	if err != nil {
		s.erroInterno(w, r, "renomear em lote", err)
		return
	}
	resultado := ResultadoRenomearEmLoteDto{len(estado.anteriores), len(estado.criadas), len(estado.alteradas), nil}
	if resultado.Alteradas > 0 || resultado.RegrasCriadas > 0 || resultado.RegrasAlteradas > 0 {
		chave := s.memoriaNomes.Guardar(estado)
		resultado.Desfazer = &chave
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
}

// regrasDoLote cria uma regra "é igual a" por descrição do banco, ou reaproveita a que já existe com o mesmo texto
// (sem faixa de valor, #241), que passa a usar o nome novo e fica ativa. Guarda no estado o que criou e o que mudou.
func (s *Servico) regrasDoLote(r *http.Request, tx *sql.Tx, agora tipos.DataHora, originais []string, nome string, estado *desfazerNome) error {
	if len(originais) == 0 { // sem criar regras, ou só lançamentos manuais
		return nil
	}
	type existente struct {
		id            int64
		texto         string
		novaDescricao string
		ativa         bool
	}
	linhas, err := tx.QueryContext(r.Context(), `SELECT "Id", "Texto", "NovaDescricao", "Ativa" FROM "RegraDescricao"
		WHERE "TipoCorrespondencia" = ? AND "ValorMinimo" IS NULL AND "ValorMaximo" IS NULL`, igual)
	if err != nil {
		return err
	}
	var existentes []existente
	for linhas.Next() {
		var e existente
		if err := linhas.Scan(&e.id, &e.texto, &e.novaDescricao, &e.ativa); err != nil {
			linhas.Close()
			return err
		}
		existentes = append(existentes, e)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return err
	}
	for _, original := range originais {
		normalizado := texto.ParaRegras(original)
		i := slices.IndexFunc(existentes, func(e existente) bool { return texto.ParaRegras(e.texto) == normalizado })
		switch {
		case i < 0:
			res, err := tx.ExecContext(r.Context(), `INSERT INTO "RegraDescricao" ("Texto", "TipoCorrespondencia",
				"NovaDescricao", "ValorMinimo", "ValorMaximo", "Ativa", "CriadoEm") VALUES (?, ?, ?, NULL, NULL, 1, ?)`,
				original, igual, nome, agora)
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			estado.criadas = append(estado.criadas, id)
		case existentes[i].novaDescricao != nome || !existentes[i].ativa:
			e := existentes[i]
			estado.alteradas = append(estado.alteradas, regraAntes{e.id, e.novaDescricao, e.ativa})
			if _, err := tx.ExecContext(r.Context(), `UPDATE "RegraDescricao" SET "NovaDescricao" = ?, "Ativa" = 1 WHERE "Id" = ?`,
				nome, e.id); err != nil {
				return err
			}
			// A mesma regra pode voltar para outra descrição do lote com o mesmo texto: já está com o nome novo.
			existentes[i].novaDescricao, existentes[i].ativa = nome, true
		}
	}
	return nil
}

// desfazerRenomear é o POST /api/transacoes/descricao/desfazer/{chave}: volta o nome de antes de cada transação,
// apaga as regras criadas e volta as reaproveitadas. A chave vale uma vez só.
func (s *Servico) desfazerRenomear(w http.ResponseWriter, r *http.Request) {
	estado, ok := s.memoriaNomes.Tirar(r.PathValue("chave"))
	if !ok {
		respostas.Mensagem(w, http.StatusNotFound, desfazer.MensagemVencida)
		return
	}
	var restauradas int
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		agora := tipos.NovaDataHora(s.relogio.Agora())
		porId := map[int64]nomeAntes{}
		ids := make([]int64, len(estado.anteriores))
		for i, a := range estado.anteriores {
			porId[a.id], ids[i] = a, a.id
		}
		em, argumentos := emLista(ids)
		lista, err := transacoes.Listar(r.Context(), tx, `WHERE "Id" IN `+em, argumentos...)
		if err != nil {
			return err
		}
		// No C#, as regras criadas saem antes (o ExecuteDelete não espera o SaveChanges).
		if len(estado.criadas) > 0 {
			em, argumentos := emLista(estado.criadas)
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM "RegraDescricao" WHERE "Id" IN `+em, argumentos...); err != nil {
				return err
			}
		}
		for _, t := range lista {
			a := porId[t.Id]
			t.Descricao, t.RegraDescricaoId, t.DescricaoManual, t.AtualizadoEm = a.descricao, a.regra, a.descricaoManual, agora
		}
		for _, antes := range estado.alteradas {
			if _, err := tx.ExecContext(r.Context(), `UPDATE "RegraDescricao" SET "NovaDescricao" = ?, "Ativa" = ? WHERE "Id" = ?`,
				antes.novaDescricao, antes.ativa, antes.id); err != nil {
				return err
			}
		}
		q := consultas.New(tx)
		login, err := auditoria.Login(r.Context(), q)
		if err != nil {
			return err
		}
		if err := auditoria.Registrar(r.Context(), q, r, agora, "RenomearDesfeito ("+strconv.Itoa(len(lista))+")", login); err != nil {
			return err
		}
		restauradas = len(lista)
		return transacoes.GravarTodas(r.Context(), tx, lista)
	})
	if err != nil {
		s.erroInterno(w, r, "desfazer o renomear", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoAplicacaoDto{int64(restauradas)})
}
