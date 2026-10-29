package categorias

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/regrasdescricao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// RegraDescricaoDto, na ordem do record do C#, com quantas transações têm o nome dado pela regra.
type RegraDescricaoDto struct {
	Id                  int64          `json:"id"`
	Texto               string         `json:"texto"`
	TipoCorrespondencia int64          `json:"tipoCorrespondencia"`
	NovaDescricao       string         `json:"novaDescricao"`
	Ativa               bool           `json:"ativa"`
	Transacoes          int64          `json:"transacoes"`
	ValorMinimo         *tipos.Decimal `json:"valorMinimo"`
	ValorMaximo         *tipos.Decimal `json:"valorMaximo"`
}

// listarRegrasDescricao: pela nova descrição e Id (ORDER BY do SQLite), com a contagem de cada uma.
func (s *Servico) listarRegrasDescricao(w http.ResponseWriter, r *http.Request) {
	contagem := map[int64]int64{}
	linhas, err := s.db.QueryContext(r.Context(), `SELECT "RegraDescricaoId", COUNT(*) FROM "Transacao"
		WHERE "RegraDescricaoId" IS NOT NULL GROUP BY "RegraDescricaoId"`)
	if err != nil {
		s.erroInterno(w, r, "contar as transações das regras de descrição", err)
		return
	}
	for linhas.Next() {
		var id, n int64
		if err := linhas.Scan(&id, &n); err != nil {
			linhas.Close()
			s.erroInterno(w, r, "contar as transações das regras de descrição", err)
			return
		}
		contagem[id] = n
	}
	linhas.Close()
	linhas, err = s.db.QueryContext(r.Context(), `SELECT "Id", "Texto", "TipoCorrespondencia", "NovaDescricao", "Ativa",
		"ValorMinimo", "ValorMaximo" FROM "RegraDescricao" ORDER BY "NovaDescricao", "Id"`)
	if err != nil {
		s.erroInterno(w, r, "listar regras de descrição", err)
		return
	}
	defer linhas.Close()
	lista := []RegraDescricaoDto{}
	for linhas.Next() {
		var d RegraDescricaoDto
		if err := linhas.Scan(&d.Id, &d.Texto, &d.TipoCorrespondencia, &d.NovaDescricao, &d.Ativa, &d.ValorMinimo, &d.ValorMaximo); err != nil {
			s.erroInterno(w, r, "ler regra de descrição", err)
			return
		}
		d.Transacoes = contagem[d.Id]
		lista = append(lista, d)
	}
	if err := linhas.Err(); err != nil {
		s.erroInterno(w, r, "listar regras de descrição", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// previa é o GET /api/regras-descricao/previa?texto=&tipo=&valorMinimo=&valorMaximo=: quantas transações de nome
// automático uma regra nova renomearia, e alguns exemplos. Os parâmetros são lidos como o model binding do ASP.NET lê a
// query (cultura invariante); o que não vira o tipo é o 400 automático, com o nome do parâmetro.
func (s *Servico) previa(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	erros := map[string][]string{}
	tipo := int64(categorizacao.Contem)
	if valor, ok := parametro(q, "tipo"); ok {
		if v, ok := lerEnumDaQuery(valor, nomesDoTipoCorrespondencia); ok {
			tipo = v
		} else {
			erros["tipo"] = []string{mensagemDaQuery(valor)}
		}
	}
	var minimo, maximo *tipos.Decimal
	for _, p := range []struct {
		nome  string
		valor **tipos.Decimal
	}{{"valorMinimo", &minimo}, {"valorMaximo", &maximo}} {
		valor, ok := parametro(q, p.nome)
		if !ok || strings.TrimSpace(valor) == "" {
			continue
		}
		if d, ok := lerDecimalDaQuery(valor); ok {
			*p.valor = &d
		} else {
			erros[p.nome] = []string{mensagemDaQuery(valor)}
		}
	}
	if len(erros) > 0 {
		respostas.Validacao(w, erros)
		return
	}
	texto, _ := parametro(q, "texto")
	if strings.TrimSpace(texto) == "" {
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, previaDto{Exemplos: []string{}})
		return
	}
	quantidade, exemplos, err := regrasdescricao.Previa(r.Context(), s.db, strings.TrimSpace(texto), tipo, 0, minimo, maximo)
	if err != nil {
		s.erroInterno(w, r, "prévia da regra de descrição", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, previaDto{Quantidade: quantidade, Exemplos: exemplos})
}

type previaDto struct {
	Quantidade int      `json:"quantidade"`
	Exemplos   []string `json:"exemplos"`
}

// parametro é o [FromQuery] do ASP.NET: o nome não diferencia maiúsculas, e vale o primeiro valor.
func parametro(q url.Values, nome string) (string, bool) {
	for chave, valores := range q {
		if strings.EqualFold(chave, nome) && len(valores) > 0 {
			return valores[0], true
		}
	}
	return "", false
}

// mensagemDaQuery é a do 400 automático para o valor da query que não vira o tipo do parâmetro: "The value 'abc' is
// not valid." (contrato 06-regras e 02-cambio). Vazio num tipo que não aceita null é "The value ” is invalid.".
func mensagemDaQuery(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return "The value '" + valor + "' is invalid."
	}
	return "The value '" + valor + "' is not valid."
}

var nomesDoTipoCorrespondencia = map[string]int64{"contem": 1, "comecacom": 2, "igual": 3}

// lerEnumDaQuery é o EnumConverter do .NET na query: o nome (sem diferença de maiúsculas) ou o número, e vários
// separados por vírgula viram a combinação (o OR dos valores). Número que não existe no enum passa.
func lerEnumDaQuery(valor string, nomes map[string]int64) (int64, bool) {
	var resultado int64
	partes := strings.Split(valor, ",")
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if v, ok := nomes[strings.ToLower(p)]; ok {
			resultado |= v
			continue
		}
		n, err := strconv.ParseUint(p, 10, 8) // o enum é ": byte"
		if err != nil {
			return 0, false
		}
		resultado |= int64(n)
	}
	return resultado, true
}

// lerDecimalDaQuery é o decimal da query na cultura invariante (NumberStyles.Number): espaços nas pontas, sinal,
// ponto decimal e vírgula de milhar.
func lerDecimalDaQuery(valor string) (tipos.Decimal, bool) {
	limpo := strings.ReplaceAll(strings.TrimSpace(valor), ",", "")
	if limpo == "" || strings.ContainsAny(limpo, "eE") {
		return tipos.Decimal{}, false
	}
	d, err := tipos.NovoDecimal(limpo)
	return d, err == nil
}

// lerRegraDescricao é o model binding do SalvarRegraDescricaoDto: [Required, StringLength(200, MinimumLength = 2)]
// Texto, [Required, StringLength(300, MinimumLength = 1)] NovaDescricao e [Range(0.01, 999999999)] nas duas pontas da
// faixa (#241); Ativa vale true se não vier. Depois, o Invalida do controller.
func lerRegraDescricao(w http.ResponseWriter, r *http.Request) (RegraDescricaoDto, bool) {
	faixa := &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}
	texto := respostas.CampoTexto{Propriedade: "Texto", Minimo: 2, Maximo: 200}
	tipo := respostas.CampoEnum{Propriedade: "TipoCorrespondencia"}
	nova := respostas.CampoTexto{Propriedade: "NovaDescricao", Minimo: 1, Maximo: 300}
	ativa := respostas.CampoBool{Propriedade: "Ativa", Valor: true}
	minimo := respostas.CampoDecimalOpcional{Propriedade: "ValorMinimo", Faixa: faixa}
	maximo := respostas.CampoDecimalOpcional{Propriedade: "ValorMaximo", Faixa: faixa}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarRegraDescricaoDto", &texto, &tipo, &nova, &ativa, &minimo, &maximo) {
		return RegraDescricaoDto{}, false
	}
	d := RegraDescricaoDto{Texto: strings.TrimSpace(*texto.Valor), TipoCorrespondencia: tipo.Ou(0),
		NovaDescricao: strings.TrimSpace(*nova.Valor), Ativa: ativa.Valor, ValorMinimo: minimo.Valor, ValorMaximo: maximo.Valor}
	switch {
	case d.TipoCorrespondencia < categorizacao.Contem || d.TipoCorrespondencia > categorizacao.Igual:
		respostas.Mensagem(w, http.StatusBadRequest, "Tipo de correspondência inválido.")
		return d, false
	case d.ValorMinimo != nil && d.ValorMaximo != nil && d.ValorMinimo.Decimal().GreaterThan(d.ValorMaximo.Decimal()):
		// #241: as duas pontas são opcionais, mas a de baixo não pode passar a de cima.
		respostas.Mensagem(w, http.StatusBadRequest, `O valor "de" é maior que o valor "até": troque os dois.`)
		return d, false
	}
	return d, true
}

func (s *Servico) criarRegraDescricao(w http.ResponseWriter, r *http.Request) {
	d, ok := lerRegraDescricao(w, r)
	if !ok {
		return
	}
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `INSERT INTO "RegraDescricao" ("Texto", "TipoCorrespondencia", "NovaDescricao",
			"ValorMinimo", "ValorMaximo", "Ativa", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?)`,
			d.Texto, d.TipoCorrespondencia, d.NovaDescricao, d.ValorMinimo, d.ValorMaximo, d.Ativa, s.agora())
		if err != nil {
			return err
		}
		if d.Id, err = res.LastInsertId(); err != nil {
			return err
		}
		return s.reaplicarDescricoes(r.Context(), tx, nil, &d)
	})
	if err != nil {
		s.erroInterno(w, r, "criar regra de descrição", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
}

func (s *Servico) atualizarRegraDescricao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	// O C# procura a regra (404) antes do Invalida (400); os atributos, o model binding confere antes de tudo.
	var existe bool
	if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "RegraDescricao" WHERE "Id" = ?)`, id).Scan(&existe); err != nil {
		s.erroInterno(w, r, "editar regra de descrição", err)
		return
	}
	if !existe {
		if _, ok := lerAtributosDaRegraDescricao(w, r); ok {
			respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		}
		return
	}
	d, valido := lerRegraDescricao(w, r)
	if !valido {
		return
	}
	d.Id = id
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE "RegraDescricao" SET "Texto" = ?, "TipoCorrespondencia" = ?,
			"NovaDescricao" = ?, "ValorMinimo" = ?, "ValorMaximo" = ?, "Ativa" = ? WHERE "Id" = ?`,
			d.Texto, d.TipoCorrespondencia, d.NovaDescricao, d.ValorMinimo, d.ValorMaximo, d.Ativa, id); err != nil {
			return err
		}
		return s.reaplicarDescricoes(r.Context(), tx, nil, &d)
	})
	if err != nil {
		s.erroInterno(w, r, "editar regra de descrição", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
}

// lerAtributosDaRegraDescricao é só a parte do model binding (os atributos), para o 404 da regra que não existe vir
// depois do 400 dos atributos e antes do Invalida, como no C#.
func lerAtributosDaRegraDescricao(w http.ResponseWriter, r *http.Request) (struct{}, bool) {
	faixa := &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}
	return struct{}{}, respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarRegraDescricaoDto",
		&respostas.CampoTexto{Propriedade: "Texto", Minimo: 2, Maximo: 200},
		&respostas.CampoEnum{Propriedade: "TipoCorrespondencia"},
		&respostas.CampoTexto{Propriedade: "NovaDescricao", Minimo: 1, Maximo: 300},
		&respostas.CampoBool{Propriedade: "Ativa", Valor: true},
		&respostas.CampoDecimalOpcional{Propriedade: "ValorMinimo", Faixa: faixa},
		&respostas.CampoDecimalOpcional{Propriedade: "ValorMaximo", Faixa: faixa})
}

// excluirRegraDescricao: as transações com o nome dado pela regra voltam ao nome do banco (ou de outra regra).
func (s *Servico) excluirRegraDescricao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	var naoExiste bool
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var existe bool
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "RegraDescricao" WHERE "Id" = ?)`, id).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			naoExiste = true
			return nil
		}
		voltaram, err := transacoes.Listar(r.Context(), tx, `WHERE "RegraDescricaoId" = ?`, id)
		if err != nil {
			return err
		}
		for _, t := range voltaram {
			if t.DescricaoOriginal != nil {
				t.Descricao = *t.DescricaoOriginal
			}
			t.RegraDescricaoId = nil
			t.AtualizadoEm = s.agora()
		}
		if err := transacoes.GravarTodas(r.Context(), tx, voltaram); err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM "RegraDescricao" WHERE "Id" = ?`, id); err != nil {
			return err
		}
		return s.reaplicarDescricoes(r.Context(), tx, voltaram, nil)
	})
	switch {
	case err != nil:
		s.erroInterno(w, r, "excluir regra de descrição", err)
	case naoExiste:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// reaplicarDescricoes reaplica as regras de descrição nas transações de nome automático e recategoriza (#250) as que
// mudaram de nome e as de antes (as que voltaram ao nome do banco ao apagar uma regra); grava tudo. Com d, conta
// quantas transações ficaram com o nome dado pela regra d.
func (s *Servico) reaplicarDescricoes(ctx context.Context, tx *sql.Tx, antes []*transacoes.Transacao, d *RegraDescricaoDto) error {
	mudaram, err := regrasdescricao.Reaplicar(ctx, tx, s.agora())
	if err != nil {
		return err
	}
	// O estado mais novo de cada transação: o que o Reaplicar leu depois de gravar as de antes.
	porId := map[int64]*transacoes.Transacao{}
	var ordem []int64
	for _, lista := range [][]*transacoes.Transacao{antes, mudaram} {
		for _, t := range lista {
			if _, visto := porId[t.Id]; !visto {
				ordem = append(ordem, t.Id)
			}
			porId[t.Id] = t
		}
	}
	lista := make([]*transacoes.Transacao, 0, len(ordem))
	for _, id := range ordem {
		lista = append(lista, porId[id])
	}
	if err := s.recategorizar(ctx, tx, lista); err != nil {
		return err
	}
	if err := transacoes.GravarTodas(ctx, tx, lista); err != nil {
		return err
	}
	if d != nil {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Transacao" WHERE "RegraDescricaoId" = ?`, d.Id).Scan(&d.Transacoes)
	}
	return nil
}

// recategorizar (#250): depois de uma regra de descrição mudar nomes, a categoria automática dessas transações é
// resolvida de novo (a regra de categoria olha o nome novo); categoria escolhida à mão fica. Se alguma mudou, grava e
// acerta as transferências (ADR 0031), como o "reaplicar".
func (s *Servico) recategorizar(ctx context.Context, tx *sql.Tx, lista []*transacoes.Transacao) error {
	if len(lista) == 0 {
		return nil
	}
	rs, err := categorizacao.Carregar(ctx, tx)
	if err != nil {
		return err
	}
	mudou := false
	for _, t := range lista {
		if t.CategoriaManual {
			continue
		}
		nova := rs.Resolver(t)
		if mesmoId(nova, t.CategoriaId) {
			continue
		}
		t.CategoriaId = nova
		rs.AplicarTipo(t)
		t.AtualizadoEm = s.agora()
		mudou = true
	}
	if !mudou {
		return nil
	}
	if err := transacoes.GravarTodas(ctx, tx, lista); err != nil {
		return err
	}
	_, err = transferencias.Reclassificar(ctx, tx, s.agora())
	return err
}
