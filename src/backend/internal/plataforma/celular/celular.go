// Pacote celular é o acesso pelo celular na rede de casa (#328, ADR 0061): desligado por padrão, e o site continua
// escutando só em 127.0.0.1. Ligado, escuta também no IP da rede de casa escolhido, com um servidor à parte:
//   - o mesmo pipeline do site (login, CSRF, cabeçalhos), mas só com esse IP nos hosts permitidos;
//   - um certificado para o IP, assinado por uma autoridade local que só pode assinar IPs da rede de casa;
//   - a porta aberta no firewall só para a sub-rede local, no perfil Particular (Windows).
//
// O certificado do computador (o .pfx da instalação) não muda: quem abre pelo computador não percebe nada.
package celular

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// ChaveIP é a chave da Configuracao com o IP escolhido. Sem valor: desligado.
const ChaveIP = "AcessoCelular.Ip"

// Eventos da auditoria de acesso: ligar e desligar mudam quem chega ao site.
const (
	EventoLigado    = "AcessoCelularLigado"
	EventoDesligado = "AcessoCelularDesligado"
)

var (
	// ErrEnderecoInvalido: o IP pedido não é um IP deste computador na rede de casa.
	ErrEnderecoInvalido = errors.New("o endereço não é deste computador na rede de casa")
	// ErrEscutar: a porta no IP da rede não abriu (outro programa nela, ou o IP saiu da rede).
	ErrEscutar = errors.New("não deu para escutar no endereço da rede")
	// ErrFirewall: o firewall não deixou abrir a porta (no Windows, o site precisa rodar como serviço).
	ErrFirewall = errors.New("não deu para abrir a porta no firewall")
)

// Opcoes do acesso pelo celular. Os campos com padrão (Enderecos, Escutar) só mudam no teste.
type Opcoes struct {
	DB          *sql.DB
	Relogio     relogio.Relogio
	Log         *slog.Logger
	PastaChaves string // onde fica a autoridade (dados/chaves)
	Porta       int    // a mesma do site (5443)
	Firewall    Firewall
	// NovoServidor monta o servidor do site para o IP: o mesmo pipeline, só com o IP nos hosts permitidos e o
	// certificado dado (servidor.NovoServidor, no cmd/financas).
	NovoServidor func(ip string, certificado tls.Certificate) (*http.Server, error)
	Enderecos    func() ([]Endereco, error)                  // padrão: EnderecosDoComputador
	Escutar      func(endereco string) (net.Listener, error) // padrão: net.Listen("tcp", ...)
	// ChaveGerenciador é o segredo do gerenciador (CarregarChaveDoGerenciador, #684); vazio: as rotas dele dão 404.
	ChaveGerenciador string
}

// Servico liga e desliga o acesso, e guarda o servidor da rede enquanto ele está ligado.
type Servico struct {
	o    Opcoes
	q    *consultas.Queries
	conf *configuracao.Servico

	mu     sync.Mutex
	escuta *escuta // nil: desligado
}

type escuta struct {
	endereco   Endereco
	servidor   *http.Server
	autoridade *Autoridade
	parou      chan struct{}
}

// Situacao é o que a tela mostra.
type Situacao struct {
	Ligado bool `json:"ligado"`
	// IP é o escolhido, mesmo que não esteja mais neste computador (IPIndisponivel); nil: desligado.
	IP             *string    `json:"ip"`
	Endereco       *string    `json:"endereco"` // https://IP:porta, só ligado
	IPIndisponivel bool       `json:"ipIndisponivel"`
	Enderecos      []Endereco `json:"enderecos"` // os IPs deste computador na rede de casa, para escolher
	// ImpressaoDigital é o SHA-256 da autoridade, para conferir no celular; só ligado.
	ImpressaoDigital *string  `json:"impressaoDigital"`
	Avisos           []string `json:"avisos"`
}

// Novo cria o serviço, desligado. Iniciar liga o que estava ligado.
func Novo(o Opcoes) *Servico {
	if o.Enderecos == nil {
		o.Enderecos = EnderecosDoComputador
	}
	if o.Escutar == nil {
		o.Escutar = func(endereco string) (net.Listener, error) { return net.Listen("tcp", endereco) }
	}
	return &Servico{o: o, q: consultas.New(o.DB), conf: configuracao.Novo(o.DB, o.Relogio)}
}

// Iniciar liga o acesso que estava ligado quando o site parou. Se o IP gravado não está mais neste computador (o
// roteador deu outro), não escuta: a tela mostra e a pessoa escolhe de novo. Erro aqui não derruba o site.
func (s *Servico) Iniciar(ctx context.Context) {
	ip, err := s.conf.Obter(ctx, ChaveIP)
	if err != nil {
		s.o.Log.ErrorContext(ctx, "acesso pelo celular: ler o IP gravado", "erro", err)
		return
	}
	if ip == nil {
		return
	}
	e, ok := s.doComputador(*ip)
	if !ok {
		s.o.Log.WarnContext(ctx, "acesso pelo celular: o IP escolhido não está mais neste computador; escolha de novo em Configurações › Segurança", "ip", *ip)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.escutar(ctx, e); err != nil {
		s.o.Log.ErrorContext(ctx, "acesso pelo celular: não deu para ligar ao subir", "ip", e.IP, "erro", err)
	}
}

// Ligar escuta no IP (um dos deste computador na rede de casa), abre o firewall e grava a escolha, com a auditoria.
// Já ligado em outro IP, troca. Falhou no meio: desfaz o que fez e nada fica gravado.
func (s *Servico) Ligar(ctx context.Context, r *http.Request, ip string) (Situacao, error) {
	e, ok := s.doComputador(ip)
	if !ok {
		return Situacao{}, ErrEnderecoInvalido
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	anterior := s.escuta
	if anterior != nil {
		s.pararEscuta()
	}
	if err := s.escutar(ctx, e); err != nil {
		if anterior != nil {
			// Volta o que estava ligado, para a falha no IP novo não desligar o que funcionava.
			if errAntigo := s.escutar(ctx, anterior.endereco); errAntigo != nil {
				s.o.Log.ErrorContext(ctx, "acesso pelo celular: não deu para voltar o IP anterior", "ip", anterior.endereco.IP, "erro", errAntigo)
			}
		}
		return Situacao{}, err
	}
	if err := s.gravar(ctx, r, &e.IP, EventoLigado); err != nil {
		s.pararEscuta()
		s.fecharFirewall(ctx)
		return Situacao{}, err
	}
	return s.situacao(ctx), nil
}

// Desligar para de escutar na rede na hora (as conexões dos celulares caem), apaga a regra do firewall e a escolha.
func (s *Servico) Desligar(ctx context.Context, r *http.Request) (Situacao, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pararEscuta()
	if err := s.o.Firewall.Fechar(ctx); err != nil {
		return Situacao{}, fmt.Errorf("%w: %w", ErrFirewall, err)
	}
	if err := s.gravar(ctx, r, nil, EventoDesligado); err != nil {
		return Situacao{}, err
	}
	return s.situacao(ctx), nil
}

// Parar fecha o servidor da rede quando o site para, esperando as requisições em andamento até o prazo do ctx. A
// escolha e o firewall ficam: o site liga de novo ao subir.
func (s *Servico) Parar(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.escuta == nil {
		return
	}
	if err := s.escuta.servidor.Shutdown(ctx); err != nil {
		s.o.Log.WarnContext(ctx, "acesso pelo celular: parar o servidor da rede", "erro", err)
		_ = s.escuta.servidor.Close()
	}
	<-s.escuta.parou
	s.escuta = nil
}

// Situacao é o estado para a tela.
func (s *Servico) Situacao(ctx context.Context) Situacao {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.situacao(ctx)
}

// EstadoCurto é o que o gerenciador mostra (#679): só se está ligado e o endereço. Não lista a rede nem pergunta ao
// firewall, porque o gerenciador consulta a cada poucos segundos.
type EstadoCurto struct {
	Ligado   bool    `json:"ligado"`
	Endereco *string `json:"endereco"` // https://IP:porta, só ligado
}

// Estado é o EstadoCurto de agora.
func (s *Servico) Estado() EstadoCurto {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.escuta == nil {
		return EstadoCurto{}
	}
	endereco := "https://" + net.JoinHostPort(s.escuta.endereco.IP, strconv.Itoa(s.o.Porta))
	return EstadoCurto{Ligado: true, Endereco: &endereco}
}

// Autoridade é o certificado público da autoridade (DER), para o celular baixar; só com o acesso ligado.
func (s *Servico) Autoridade() ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.escuta == nil {
		return nil, false
	}
	return s.escuta.autoridade.DER(), true
}

// situacao monta a Situacao; quem chama segura o mu.
func (s *Servico) situacao(ctx context.Context) Situacao {
	enderecos, err := s.o.Enderecos()
	if err != nil {
		s.o.Log.WarnContext(ctx, "acesso pelo celular: listar os IPs deste computador", "erro", err)
		enderecos = []Endereco{}
	}
	sit := Situacao{Enderecos: enderecos, Avisos: []string{}}
	if s.escuta != nil {
		e := s.escuta.endereco
		endereco := "https://" + net.JoinHostPort(e.IP, strconv.Itoa(s.o.Porta))
		impressao := s.escuta.autoridade.ImpressaoDigital()
		sit.Ligado, sit.IP, sit.Endereco, sit.ImpressaoDigital = true, &e.IP, &endereco, &impressao
		sit.IPIndisponivel = !slices.ContainsFunc(enderecos, func(x Endereco) bool { return x.IP == e.IP })
		sit.Avisos = append(sit.Avisos, s.o.Firewall.Avisos(ctx, e, s.o.Porta)...)
		return sit
	}
	gravado, err := s.conf.Obter(ctx, ChaveIP)
	if err != nil {
		s.o.Log.WarnContext(ctx, "acesso pelo celular: ler o IP gravado", "erro", err)
	}
	if gravado != nil {
		// Gravado e sem escutar: o IP saiu deste computador, ou a porta não abriu ao subir (o log diz qual).
		sit.IP = gravado
		sit.IPIndisponivel = !slices.ContainsFunc(enderecos, func(x Endereco) bool { return x.IP == *gravado })
	}
	return sit
}

// doComputador confere que o IP é um dos deste computador na rede de casa, e devolve o Endereco dele.
func (s *Servico) doComputador(ip string) (Endereco, bool) {
	pedido := net.ParseIP(ip)
	if pedido == nil || !daRedeDeCasa(pedido) {
		return Endereco{}, false
	}
	enderecos, err := s.o.Enderecos()
	if err != nil {
		return Endereco{}, false
	}
	for _, e := range enderecos {
		if e.IP == pedido.To4().String() {
			return e, true
		}
	}
	return Endereco{}, false
}

// escutar abre a porta no IP, abre o firewall e põe o servidor da rede para atender; quem chama segura o mu. Falhou:
// fecha o que abriu.
func (s *Servico) escutar(ctx context.Context, e Endereco) error {
	agora := s.o.Relogio.Agora()
	autoridade, err := CarregarAutoridade(s.o.PastaChaves, agora)
	if err != nil {
		return err
	}
	certificado, err := autoridade.CertificadoPara(net.ParseIP(e.IP).To4(), agora)
	if err != nil {
		return err
	}
	servidor, err := s.o.NovoServidor(e.IP, certificado)
	if err != nil {
		return err
	}
	ouvinte, err := s.o.Escutar(net.JoinHostPort(e.IP, strconv.Itoa(s.o.Porta)))
	if err != nil {
		return fmt.Errorf("%w %s: %w", ErrEscutar, e.IP, err)
	}
	if err := s.o.Firewall.Abrir(ctx, e, s.o.Porta); err != nil {
		_ = ouvinte.Close()
		return fmt.Errorf("%w: %w", ErrFirewall, err)
	}
	parou := make(chan struct{})
	go func() {
		defer close(parou)
		if err := servidor.ServeTLS(ouvinte, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.o.Log.Error("acesso pelo celular: o servidor da rede parou", "ip", e.IP, "erro", err)
		}
	}()
	s.escuta = &escuta{endereco: e, servidor: servidor, autoridade: autoridade, parou: parou}
	s.o.Log.InfoContext(ctx, "acesso pelo celular ligado", "endereco", "https://"+net.JoinHostPort(e.IP, strconv.Itoa(s.o.Porta)))
	return nil
}

// pararEscuta fecha o servidor da rede na hora, com as conexões abertas; quem chama segura o mu.
func (s *Servico) pararEscuta() {
	if s.escuta == nil {
		return
	}
	_ = s.escuta.servidor.Close()
	<-s.escuta.parou
	s.escuta = nil
}

// fecharFirewall desfaz o Abrir de um Ligar que falhou depois; o erro só vai para o log.
func (s *Servico) fecharFirewall(ctx context.Context) {
	if err := s.o.Firewall.Fechar(ctx); err != nil {
		s.o.Log.WarnContext(ctx, "acesso pelo celular: desfazer a regra do firewall", "erro", err)
	}
}

// gravar grava a escolha (nil: desligado) e o evento da auditoria juntos, numa transação.
func (s *Servico) gravar(ctx context.Context, r *http.Request, ip *string, evento string) error {
	return banco.EmTransacao(ctx, s.o.DB, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		login, err := auditoria.Login(ctx, q)
		if err != nil {
			return err
		}
		if err := auditoria.Registrar(ctx, q, r, tipos.NovaDataHora(s.o.Relogio.Agora()), evento, login); err != nil {
			return err
		}
		return s.conf.ComTx(tx).Definir(ctx, ChaveIP, ip)
	})
}
