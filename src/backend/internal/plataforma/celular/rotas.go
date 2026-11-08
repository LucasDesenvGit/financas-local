package celular

import (
	"errors"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/erros"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
)

// RotaAutoridade é a única rota sem login (a autenticação a deixa passar): o celular baixa a autoridade antes de
// confiar no site e de entrar. É só o certificado público.
const RotaAutoridade = "/api/celular/autoridade"

// NomeDoArquivo é o nome do .crt que o celular baixa.
const NomeDoArquivo = "financas-local-celular.crt"

// Mensagens das falhas que a pessoa pode resolver.
const (
	MensagemEnderecoInvalido = "Escolha um dos endereços deste computador na rede de casa."
	MensagemEscutar          = "Não deu para abrir o site nesse endereço: outro programa pode estar usando a porta, ou o computador saiu dessa rede. Os detalhes estão em Configurações › Erros."
	MensagemFirewall         = "Não deu para abrir a porta no firewall do Windows. O site precisa rodar como serviço, que é o normal da instalação. Os detalhes estão em Configurações › Erros."
)

// Registrar põe as rotas do acesso pelo celular no mux. Login e CSRF são de quem está na frente (autenticacao), menos
// na RotaAutoridade.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/celular", s.situacaoHTTP)
	mux.HandleFunc("POST /api/celular/ligar", s.ligarHTTP)
	mux.HandleFunc("POST /api/celular/desligar", s.desligarHTTP)
	mux.HandleFunc("GET "+RotaAutoridade, s.autoridadeHTTP)
}

func (s *Servico) situacaoHTTP(w http.ResponseWriter, r *http.Request) {
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, s.Situacao(r.Context()))
}

func (s *Servico) ligarHTTP(w http.ResponseWriter, r *http.Request) {
	ip := &respostas.CampoTexto{Propriedade: "Ip", Minimo: 7, Maximo: 15}
	if !respostas.LerTextos(w, r, "dto", "LigarAcessoCelularDto", ip) {
		return
	}
	sit, err := s.Ligar(r.Context(), r, *ip.Valor)
	s.responder(w, r, sit, err, "ligar")
}

func (s *Servico) desligarHTTP(w http.ResponseWriter, r *http.Request) {
	sit, err := s.Desligar(r.Context(), r)
	s.responder(w, r, sit, err, "desligar")
}

func (s *Servico) responder(w http.ResponseWriter, r *http.Request, sit Situacao, err error, etapa string) {
	switch {
	case err == nil:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, sit)
	case errors.Is(err, ErrEnderecoInvalido):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemEnderecoInvalido)
	case errors.Is(err, ErrEscutar):
		s.o.Log.WarnContext(r.Context(), "acesso pelo celular: "+etapa, "origem", erros.OrigemAPI, "erro", err)
		respostas.Mensagem(w, http.StatusConflict, MensagemEscutar)
	case errors.Is(err, ErrFirewall):
		s.o.Log.WarnContext(r.Context(), "acesso pelo celular: "+etapa, "origem", erros.OrigemAPI, "erro", err)
		respostas.Mensagem(w, http.StatusConflict, MensagemFirewall)
	default:
		s.o.Log.ErrorContext(r.Context(), "acesso pelo celular: "+etapa, "origem", erros.OrigemAPI, "erro", err)
		servidor.ResponderErroInterno(w)
	}
}

// autoridadeHTTP entrega o .crt da autoridade, para o celular instalar. Desligado, 404: não há o que instalar.
func (s *Servico) autoridadeHTTP(w http.ResponseWriter, r *http.Request) {
	der, ok := s.Autoridade()
	if !ok {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="`+NomeDoArquivo+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(der)
}
