package reservas

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
)

// Registrar põe as rotas de /api/reservas no mux (o ReservasController).
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/reservas", s.listar)
	mux.HandleFunc("PUT /api/reservas/conferencia", s.conferir)
	mux.HandleFunc("DELETE /api/reservas/conferencia", s.removerConferencia)
	mux.HandleFunc("POST /api/reservas/saida", s.registrarSaida)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nas reservas", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context())
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// padraoMoeda é o [RegularExpression] da moeda do ConferirReservaDto, com a mensagem própria.
var padraoMoeda = func() *respostas.Padrao {
	p := respostas.NovoPadrao(`^(BRL|USD)$`)
	p.MensagemPropria = "Moeda inválida: use BRL ou USD."
	return p
}()

// conferir é o PUT /api/reservas/conferencia: grava o saldo real da reserva; os lançamentos depois da data passam a
// somar a partir dele. Sem a data, hoje em Brasília; no futuro, não. Com RegistrarDiferenca (#691), o que a estimativa
// passa do conferido vira um gasto do cofrinho, na categoria CategoriaId (opcional).
func (s *Servico) conferir(w http.ResponseWriter, r *http.Request) {
	conta := respostas.CampoInteiro{Propriedade: "ContaId"}
	nome := respostas.CampoTexto{Propriedade: "Nome", Maximo: TamanhoMaximoNome}
	saldo := respostas.CampoDecimal{Propriedade: "Saldo", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 999999999999}}
	data := respostas.CampoDataOpcional{Propriedade: "Data"}
	moeda := respostas.CampoTexto{Propriedade: "Moeda", Maximo: math.MaxInt, Opcional: true, Padrao: padraoMoeda}
	diferenca := respostas.CampoBool{Propriedade: "RegistrarDiferenca"}
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.ConferirReservaDto", &conta, &nome, &saldo, &data, &moeda,
		&diferenca, &categoria) {
		return
	}
	hoje := relogio.Hoje(s.relogio)
	dia := hoje
	if data.Valor != nil {
		dia = *data.Valor
	}
	// O "Informe o nome da reserva." do C# não chega a valer: o [Required] já recusa o nome só com espaços.
	if dia.Depois(hoje) {
		respostas.Mensagem(w, http.StatusBadRequest, "A data da conferência não pode estar no futuro.")
		return
	}
	var err error
	if diferenca.Valor {
		err = s.ConferirComDiferenca(r.Context(), conta.Valor, *nome.Valor, saldo.Valor, dia, categoria.Valor)
	} else {
		err = s.Conferir(r.Context(), conta.Valor, *nome.Valor, saldo.Valor, dia, moeda.Valor)
	}
	s.responder(w, r, "conferir", err)
}

// responder é o 204, a mensagem dos erros de quem usa (400) ou o 500.
func (s *Servico) responder(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	switch {
	case errors.Is(err, ErrContaInvalida), errors.Is(err, ErrSemCotacao), errors.Is(err, ErrSoOpenFinance),
		errors.Is(err, ErrSaidaMeliDolar), errors.Is(err, ErrCategoriaInvalida), errors.Is(err, ErrDiferencaSoHoje),
		errors.Is(err, ErrSemDiferenca), errors.Is(err, ErrReservaInvalida):
		respostas.Mensagem(w, http.StatusBadRequest, err.Error())
	case err != nil:
		s.erroInterno(w, r, etapa, err)
	default:
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}

// registrarSaida é o POST /api/reservas/saida (#691): o gasto pago direto do cofrinho, que não chega pelo Open
// Finance. A data não pode estar no futuro.
func (s *Servico) registrarSaida(w http.ResponseWriter, r *http.Request) {
	conta := respostas.CampoInteiro{Propriedade: "ContaId"}
	nome := respostas.CampoTexto{Propriedade: "Nome", Maximo: TamanhoMaximoNome}
	data := respostas.CampoData{Propriedade: "Data", Obrigatoria: true}
	valor := respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}}
	descricao := respostas.CampoTexto{Propriedade: "Descricao", Maximo: 300}
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SaidaReservaDto", &conta, &nome, &data, &valor, &descricao,
		&categoria) {
		return
	}
	if data.Valor.Depois(relogio.Hoje(s.relogio)) {
		respostas.Mensagem(w, http.StatusBadRequest, "A data da saída não pode estar no futuro.")
		return
	}
	_, err := s.RegistrarSaida(r.Context(), s.db, Saida{ContaId: conta.Valor, Nome: *nome.Valor, Data: data.Valor,
		Valor: valor.Valor, Descricao: *descricao.Valor, CategoriaId: categoria.Valor})
	s.responder(w, r, "registrar a saída", err)
}

// removerConferencia é o DELETE /api/reservas/conferencia?contaId=&nome=: o saldo volta a ser só a soma dos
// lançamentos do extrato. O nome é obrigatório (o string sem "?" do C#, que o ASP.NET trata como [Required]; vazio ou
// só espaços vira null); a conta sem vir é 0.
func (s *Servico) removerConferencia(w http.ResponseWriter, r *http.Request) {
	erros := map[string][]string{}
	var conta int64
	if v, ok := respostas.ParametroDaQuery(r, "contaId"); ok {
		if n, ok := respostas.InteiroDaQuery(v); ok {
			conta = int64(n)
		} else {
			erros["contaId"] = []string{respostas.MensagemDaQuery(v)}
		}
	}
	nome, ok := respostas.ParametroDaQuery(r, "nome")
	if !ok || strings.TrimFunc(nome, unicode.IsSpace) == "" {
		erros["nome"] = []string{respostas.Obrigatorio("nome")}
	}
	if len(erros) > 0 {
		respostas.Validacao(w, erros)
		return
	}
	removeu, err := s.RemoverConferencia(r.Context(), conta, nome)
	switch {
	case err != nil:
		s.erroInterno(w, r, "tirar a conferência", err)
	case !removeu:
		respostas.Mensagem(w, http.StatusNotFound, "Registro não encontrado.")
	default:
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}
