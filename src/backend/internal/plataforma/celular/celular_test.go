package celular

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/config"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// registroDeLog guarda as mensagens do log: os caminhos que só avisam no log também têm teste.
type registroDeLog struct {
	mu        sync.Mutex
	mensagens []string
}

func (r *registroDeLog) Enabled(context.Context, slog.Level) bool { return true }
func (r *registroDeLog) WithAttrs([]slog.Attr) slog.Handler       { return r }
func (r *registroDeLog) WithGroup(string) slog.Handler            { return r }
func (r *registroDeLog) Handle(_ context.Context, registro slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mensagens = append(r.mensagens, registro.Message)
	return nil
}

// tem diz se alguma mensagem do log contém o trecho.
func (r *registroDeLog) tem(trecho string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.mensagens {
		if strings.Contains(m, trecho) {
			return true
		}
	}
	return false
}

// firewallFalso grava o que pediram e falha quando o teste manda.
type firewallFalso struct {
	mu                  sync.Mutex
	abertos             []string // "IP:porta" de cada Abrir
	fechados            int
	errAbrir, errFechar error
	avisos              []string
}

func (f *firewallFalso) Abrir(_ context.Context, e Endereco, porta int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.abertos = append(f.abertos, net.JoinHostPort(e.IP, strconv.Itoa(porta)))
	return f.errAbrir
}

func (f *firewallFalso) Fechar(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fechados++
	return f.errFechar
}

func (f *firewallFalso) Avisos(context.Context, Endereco, int) []string { return f.avisos }

var (
	wifi     = Endereco{IP: "192.168.0.10", Interface: "Wi-Fi", Rede: "192.168.0.0/24"}
	ethernet = Endereco{IP: "10.0.0.20", Interface: "Ethernet", Rede: "10.0.0.0/8"}
)

// ambiente é o serviço com o banco de verdade, o servidor do site de verdade (o mesmo pipeline, com o filtro de hosts)
// e a porta "da rede" aberta no 127.0.0.1, porque o computador do teste não tem o IP 192.168.0.10.
type ambiente struct {
	t          *testing.T
	s          *Servico
	log        *registroDeLog
	db         *sql.DB
	fw         *firewallFalso
	pasta      string
	mu         sync.Mutex
	enderecos  []Endereco
	errLista   error
	pedidos    []string                // os endereços que o serviço pediu para escutar
	ouvintes   map[string]net.Listener // pelo endereço pedido
	errEscutar error
	errMontar  error
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	a := &ambiente{t: t, db: testeapoio.NovoBanco(t), fw: &firewallFalso{}, pasta: filepath.Join(t.TempDir(), "chaves"),
		enderecos: []Endereco{wifi, ethernet}, ouvintes: map[string]net.Listener{}, log: &registroDeLog{}}
	// O site fica em testdata, e não no t.TempDir(): o servidor deixa a pasta aberta (os.Root), e o Windows não apaga.
	site := filepath.Join("testdata", "site")
	api := http.NewServeMux()
	api.HandleFunc("GET /api/eco", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "eco "+r.Host) })
	log := slog.New(a.log)
	a.s = Novo(Opcoes{
		DB: a.db, Relogio: relogio.Fixo{Instante: agoraDosTestes}, Log: log, PastaChaves: a.pasta, Porta: 5443, Firewall: a.fw,
		NovoServidor: func(ip string, c tls.Certificate) (*http.Server, error) {
			if a.errMontar != nil {
				return nil, a.errMontar
			}
			return servidor.NovoServidor(servidor.Opcoes{
				Config: config.Config{HostsPermitidos: []string{ip}}, PastaSite: site, Certificado: c, Log: log, API: api,
			})
		},
		Enderecos: func() ([]Endereco, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.enderecos, a.errLista
		},
		Escutar: func(endereco string) (net.Listener, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			a.pedidos = append(a.pedidos, endereco)
			if a.errEscutar != nil {
				return nil, a.errEscutar
			}
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return nil, err
			}
			a.ouvintes[endereco] = l
			return l, nil
		},
	})
	t.Cleanup(func() { a.s.Parar(context.Background()) })
	return a
}

// pedido é a requisição de quem liga, com alguém logado (a auditoria grava o IP e o navegador).
func (a *ambiente) pedido() *http.Request {
	r := httptest.NewRequestWithContext(sessao.ComUsuario(a.t.Context(), testeapoio.UsuarioDosTestes), "POST", "/api/celular/ligar", nil)
	r.RemoteAddr = "127.0.0.1:50000"
	return r
}

// celular é o navegador do celular: confia só na autoridade e chega ao site pelo IP da rede (a conexão vai para a
// porta aberta no 127.0.0.1, mas o nome conferido no certificado e o Host são os do IP).
func (a *ambiente) celular(ip string, raiz []byte) *http.Client {
	a.t.Helper()
	raizes := x509.NewCertPool()
	if raiz != nil {
		c, err := x509.ParseCertificate(raiz)
		if err != nil {
			a.t.Fatal(err)
		}
		raizes.AddCert(c)
	}
	a.mu.Lock()
	ouvinte := a.ouvintes[net.JoinHostPort(ip, "5443")]
	a.mu.Unlock()
	if ouvinte == nil {
		a.t.Fatalf("não escutou em %s", ip)
	}
	destino := ouvinte.Addr().String()
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: raizes, Time: func() time.Time { return agoraDosTestes }},
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", destino)
		},
		DisableKeepAlives: true,
	}}
}

func (a *ambiente) ipGravado() *string {
	a.t.Helper()
	ip, err := configuracao.Novo(a.db, relogio.Fixo{Instante: agoraDosTestes}).Obter(a.t.Context(), ChaveIP)
	if err != nil {
		a.t.Fatal(err)
	}
	return ip
}

func (a *ambiente) eventos() []string {
	a.t.Helper()
	linhas, err := a.db.QueryContext(a.t.Context(), "SELECT Evento || ' ' || COALESCE(Ip, '') FROM AuditoriaAcesso ORDER BY Id")
	if err != nil {
		a.t.Fatal(err)
	}
	defer linhas.Close()
	lista := []string{}
	for linhas.Next() {
		var e string
		if err := linhas.Scan(&e); err != nil {
			a.t.Fatal(err)
		}
		lista = append(lista, e)
	}
	return lista
}

func pedir(t *testing.T, c *http.Client, url string) (int, string, error) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	corpo, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(corpo), nil
}

// Regra (#328, ADR 0061): ligar escuta no IP da rede na porta do site, abre o firewall para ele e grava a escolha
// com a auditoria. O celular que instalou a autoridade abre o site pelo IP com HTTPS válido, e só pelo IP: outro nome
// no Host recebe 400 (o filtro de hosts, contra DNS rebinding). Desligar fecha a porta na hora e apaga a escolha.
func TestLigarEDesligar(t *testing.T) {
	a := novoAmbiente(t)
	sit, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10")
	if err != nil {
		t.Fatal(err)
	}
	impressao := sit.ImpressaoDigital
	if !sit.Ligado || *sit.IP != "192.168.0.10" || *sit.Endereco != "https://192.168.0.10:5443" || sit.IPIndisponivel ||
		impressao == nil || len(*impressao) != 95 || !reflect.DeepEqual(sit.Enderecos, []Endereco{wifi, ethernet}) {
		t.Errorf("situação %+v", sit)
	}
	if !reflect.DeepEqual(a.pedidos, []string{"192.168.0.10:5443"}) || !reflect.DeepEqual(a.fw.abertos, []string{"192.168.0.10:5443"}) {
		t.Errorf("escutou em %v, firewall %v", a.pedidos, a.fw.abertos)
	}
	if ip := a.ipGravado(); ip == nil || *ip != "192.168.0.10" {
		t.Errorf("gravou %v", ip)
	}
	if e := a.eventos(); !reflect.DeepEqual(e, []string{"AcessoCelularLigado 127.0.0.1"}) {
		t.Errorf("auditoria %v", e)
	}

	raiz, ok := a.s.Autoridade()
	if !ok {
		t.Fatal("ligado, e sem a autoridade")
	}
	celular := a.celular("192.168.0.10", raiz)
	status, corpo, err := pedir(t, celular, "https://192.168.0.10:5443/api/eco")
	if err != nil || status != 200 || corpo != "eco 192.168.0.10:5443" {
		t.Errorf("pelo IP: %d %q %v", status, corpo, err)
	}
	if status, corpo, err := pedir(t, celular, "https://192.168.0.10:5443/"); err != nil || status != 200 || !strings.Contains(corpo, "site") {
		t.Errorf("a página: %d %q %v", status, corpo, err)
	}
	// Outro nome no Host (um site de fora apontando um domínio para o IP): 400.
	r, _ := http.NewRequest("GET", "https://192.168.0.10:5443/api/eco", nil)
	r.Host = "malicioso.example"
	if resp, err := celular.Do(r); err != nil || resp.StatusCode != 400 {
		t.Errorf("outro host: %v %v", resp, err)
	} else {
		resp.Body.Close()
	}
	// O celular que não instalou a autoridade não confia.
	if _, _, err := pedir(t, a.celular("192.168.0.10", nil), "https://192.168.0.10:5443/"); err == nil ||
		!errors.As(err, new(*tls.CertificateVerificationError)) {
		t.Errorf("sem a autoridade: %v", err)
	}

	sit, err = a.s.Desligar(t.Context(), a.pedido())
	if err != nil {
		t.Fatal(err)
	}
	if sit.Ligado || sit.IP != nil || sit.Endereco != nil || sit.ImpressaoDigital != nil || sit.IPIndisponivel {
		t.Errorf("desligado: %+v", sit)
	}
	if _, _, err := pedir(t, celular, "https://192.168.0.10:5443/"); err == nil {
		t.Error("desligado, e o site ainda responde pelo IP")
	}
	if a.fw.fechados != 1 || a.ipGravado() != nil {
		t.Errorf("firewall fechado %d vezes, IP gravado %v", a.fw.fechados, a.ipGravado())
	}
	if e := a.eventos(); !reflect.DeepEqual(e, []string{"AcessoCelularLigado 127.0.0.1", "AcessoCelularDesligado 127.0.0.1"}) {
		t.Errorf("auditoria %v", e)
	}
	if _, ok := a.s.Autoridade(); ok {
		t.Error("desligado, e ainda entrega a autoridade")
	}
	if a.log.tem("o servidor da rede parou") || a.log.tem("ler o IP gravado") {
		t.Errorf("desligar normal não é erro: %v", a.log.mensagens)
	}
	// Desligar de novo não é erro.
	if _, err := a.s.Desligar(t.Context(), a.pedido()); err != nil {
		t.Error(err)
	}
}

// Regra (#328): só um IP deste computador na rede de casa. O pedido não abre porta nem grava nada.
func TestLigarRecusaEndereco(t *testing.T) {
	a := novoAmbiente(t)
	for _, ip := range []string{"", "lixo", "8.8.8.8", "127.0.0.1", "192.168.0.99", "::1", "0.0.0.0", "192.168.0.10:5443"} {
		if _, err := a.s.Ligar(t.Context(), a.pedido(), ip); !errors.Is(err, ErrEnderecoInvalido) {
			t.Errorf("%q: %v", ip, err)
		}
	}
	// Mesmo que a lista trouxesse um IP público, ele não vale: a regra da rede de casa é conferida de novo.
	a.enderecos = append(a.enderecos, Endereco{IP: "200.1.2.3", Interface: "Modem", Rede: "200.1.2.0/24"})
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "200.1.2.3"); !errors.Is(err, ErrEnderecoInvalido) {
		t.Errorf("IP público na lista: %v", err)
	}
	a.errLista = errors.New("sem interfaces")
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); !errors.Is(err, ErrEnderecoInvalido) {
		t.Errorf("sem a lista: %v", err)
	}
	if len(a.pedidos) != 0 || len(a.fw.abertos) != 0 || a.ipGravado() != nil || len(a.eventos()) != 0 {
		t.Errorf("mexeu em algo: %v %v", a.pedidos, a.fw.abertos)
	}
}

// Regra (#328): falhou no meio, nada fica ligado nem gravado: a porta que abriu fecha, a regra do firewall sai.
func TestLigarQueFalha(t *testing.T) {
	t.Run("a porta não abre", func(t *testing.T) {
		a := novoAmbiente(t)
		a.errEscutar = errors.New("address already in use")
		if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); !errors.Is(err, ErrEscutar) ||
			!strings.Contains(err.Error(), "already in use") {
			t.Errorf("erro %v", err)
		}
		if len(a.fw.abertos) != 0 || a.ipGravado() != nil || a.s.Situacao(t.Context()).Ligado {
			t.Errorf("firewall %v", a.fw.abertos)
		}
	})
	t.Run("o firewall não abre", func(t *testing.T) {
		a := novoAmbiente(t)
		a.fw.errAbrir = errors.New("requer elevação")
		if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); !errors.Is(err, ErrFirewall) ||
			!strings.Contains(err.Error(), "requer elevação") {
			t.Errorf("erro %v", err)
		}
		// A porta que tinha aberto fechou.
		if _, err := net.Dial("tcp", a.ouvintes["192.168.0.10:5443"].Addr().String()); err == nil {
			t.Error("a porta ficou aberta")
		}
		if a.ipGravado() != nil || a.s.Situacao(t.Context()).Ligado || len(a.eventos()) != 0 {
			t.Error("ficou ligado")
		}
	})
	t.Run("o servidor não monta", func(t *testing.T) {
		a := novoAmbiente(t)
		a.errMontar = errors.New("sem index.html")
		if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err == nil || !strings.Contains(err.Error(), "sem index.html") {
			t.Errorf("erro %v", err)
		}
		if len(a.pedidos) != 0 {
			t.Errorf("escutou: %v", a.pedidos)
		}
	})
	t.Run("a autoridade não abre", func(t *testing.T) {
		a := novoAmbiente(t)
		if err := os.MkdirAll(filepath.Join(a.pasta, ArquivoDaAutoridade), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err == nil || !strings.Contains(err.Error(), "ler a autoridade") {
			t.Errorf("erro %v", err)
		}
	})
	t.Run("não grava", func(t *testing.T) {
		a := novoAmbiente(t)
		if _, err := a.db.ExecContext(t.Context(), "DROP TABLE AuditoriaAcesso"); err != nil {
			t.Fatal(err)
		}
		if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err == nil {
			t.Fatal("devia falhar")
		}
		if a.fw.fechados != 1 || a.s.Situacao(t.Context()).Ligado || a.ipGravado() != nil {
			t.Errorf("firewall fechado %d vezes", a.fw.fechados)
		}
		if a.log.tem("desfazer a regra do firewall") {
			t.Error("desfazer deu certo, e o log diz que falhou")
		}
		if _, err := net.Dial("tcp", a.ouvintes["192.168.0.10:5443"].Addr().String()); err == nil {
			t.Error("a porta ficou aberta")
		}
	})
}

// Regra (#328): o Ligar que não grava desfaz a regra do firewall; se nem isso der, o log diz.
func TestLigarQueNaoGravaNemDesfaz(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.db.ExecContext(t.Context(), "DROP TABLE AuditoriaAcesso"); err != nil {
		t.Fatal(err)
	}
	a.fw.errFechar = errors.New("negado")
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err == nil {
		t.Fatal("devia falhar")
	}
	if !a.log.tem("desfazer a regra do firewall") {
		t.Errorf("o log não diz que a regra ficou: %v", a.log.mensagens)
	}
}

// Regra (#328): ligar em outro IP troca (a porta do antigo fecha); se o novo falha, o antigo volta a funcionar.
func TestLigarEmOutroIP(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err != nil {
		t.Fatal(err)
	}
	antigo := a.ouvintes["192.168.0.10:5443"].Addr().String()
	sit, err := a.s.Ligar(t.Context(), a.pedido(), "10.0.0.20")
	if err != nil {
		t.Fatal(err)
	}
	if *sit.IP != "10.0.0.20" || *a.ipGravado() != "10.0.0.20" {
		t.Errorf("situação %+v", sit)
	}
	if _, err := net.Dial("tcp", antigo); err == nil {
		t.Error("o IP antigo continua escutando")
	}
	raiz, _ := a.s.Autoridade()
	if status, _, err := pedir(t, a.celular("10.0.0.20", raiz), "https://10.0.0.20:5443/api/eco"); err != nil || status != 200 {
		t.Errorf("pelo IP novo: %d %v", status, err)
	}

	// O próximo falha no firewall: volta o 10.0.0.20, que funcionava.
	primeiro := true
	a.s.o.Firewall = &firewallQueFalhaUmaVez{firewallFalso: a.fw, falhar: &primeiro}
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); !errors.Is(err, ErrFirewall) {
		t.Fatalf("erro %v", err)
	}
	sit = a.s.Situacao(t.Context())
	if !sit.Ligado || *sit.IP != "10.0.0.20" || *a.ipGravado() != "10.0.0.20" {
		t.Errorf("não voltou o anterior: %+v", sit)
	}
	if a.log.tem("não deu para voltar o IP anterior") {
		t.Error("voltou, e o log diz que não")
	}
	raiz, _ = a.s.Autoridade()
	if status, _, err := pedir(t, a.celular("10.0.0.20", raiz), "https://10.0.0.20:5443/api/eco"); err != nil || status != 200 {
		t.Errorf("o anterior não responde: %d %v", status, err)
	}

	// O novo e o anterior falham: fica desligado, e o log diz (não há o que voltar).
	a.s.o.Firewall = &firewallQueFalhaSempre{a.fw}
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); !errors.Is(err, ErrFirewall) {
		t.Fatalf("erro %v", err)
	}
	if a.s.Situacao(t.Context()).Ligado {
		t.Error("ficou ligado sem firewall")
	}
	if !a.log.tem("não deu para voltar o IP anterior") {
		t.Errorf("o log não diz que ficou desligado: %v", a.log.mensagens)
	}
}

type firewallQueFalhaUmaVez struct {
	*firewallFalso
	falhar *bool
}

func (f *firewallQueFalhaUmaVez) Abrir(ctx context.Context, e Endereco, porta int) error {
	if *f.falhar {
		*f.falhar = false
		return errors.New("falhou uma vez")
	}
	return f.firewallFalso.Abrir(ctx, e, porta)
}

type firewallQueFalhaSempre struct{ *firewallFalso }

func (f *firewallQueFalhaSempre) Abrir(context.Context, Endereco, int) error {
	return errors.New("falha sempre")
}

// Regra (#328): ao subir, o site liga o que estava ligado. Se o IP gravado saiu deste computador (o roteador deu
// outro), não escuta, e a tela mostra o IP antigo como indisponível para escolher de novo.
func TestIniciar(t *testing.T) {
	t.Run("desligado", func(t *testing.T) {
		a := novoAmbiente(t)
		a.s.Iniciar(t.Context())
		if len(a.pedidos) != 0 || a.s.Situacao(t.Context()).Ligado {
			t.Error("ligou")
		}
	})
	t.Run("ligado", func(t *testing.T) {
		a := novoAmbiente(t)
		testeapoio.NovaFabrica(t, a.db, relogio.Fixo{Instante: agoraDosTestes}).Configuracao(ChaveIP, "10.0.0.20")
		a.s.Iniciar(t.Context())
		sit := a.s.Situacao(t.Context())
		if !sit.Ligado || *sit.IP != "10.0.0.20" || !reflect.DeepEqual(a.fw.abertos, []string{"10.0.0.20:5443"}) {
			t.Errorf("situação %+v, firewall %v", sit, a.fw.abertos)
		}
		raiz, _ := a.s.Autoridade()
		if status, _, err := pedir(t, a.celular("10.0.0.20", raiz), "https://10.0.0.20:5443/api/eco"); err != nil || status != 200 {
			t.Errorf("%d %v", status, err)
		}
		// O IP sai do computador com o site rodando: a tela avisa.
		a.mu.Lock()
		a.enderecos = []Endereco{wifi}
		a.mu.Unlock()
		if sit := a.s.Situacao(t.Context()); !sit.Ligado || !sit.IPIndisponivel {
			t.Errorf("situação %+v", sit)
		}
		// Parar fecha a porta e guarda a escolha.
		a.s.Parar(t.Context())
		if a.s.Situacao(t.Context()).Ligado || *a.ipGravado() != "10.0.0.20" {
			t.Error("não parou, ou apagou a escolha")
		}
		if a.log.tem("não deu para ligar ao subir") || a.log.tem("parar o servidor da rede") || a.log.tem("o servidor da rede parou") {
			t.Errorf("subir e parar sem erro, e o log tem erro: %v", a.log.mensagens)
		}
		a.s.Parar(t.Context()) // de novo, sem nada ligado
	})
	t.Run("o IP saiu do computador", func(t *testing.T) {
		a := novoAmbiente(t)
		testeapoio.NovaFabrica(t, a.db, relogio.Fixo{Instante: agoraDosTestes}).Configuracao(ChaveIP, "192.168.1.50")
		a.s.Iniciar(t.Context())
		sit := a.s.Situacao(t.Context())
		if sit.Ligado || sit.IP == nil || *sit.IP != "192.168.1.50" || !sit.IPIndisponivel || len(a.pedidos) != 0 {
			t.Errorf("situação %+v", sit)
		}
	})
	t.Run("a porta não abre ao subir", func(t *testing.T) {
		a := novoAmbiente(t)
		testeapoio.NovaFabrica(t, a.db, relogio.Fixo{Instante: agoraDosTestes}).Configuracao(ChaveIP, "192.168.0.10")
		a.errEscutar = errors.New("em uso")
		a.s.Iniciar(t.Context())
		sit := a.s.Situacao(t.Context())
		if sit.Ligado || *sit.IP != "192.168.0.10" || sit.IPIndisponivel {
			t.Errorf("situação %+v", sit)
		}
		if !a.log.tem("não deu para ligar ao subir") {
			t.Errorf("o log não diz: %v", a.log.mensagens)
		}
	})
	t.Run("o banco não abre", func(t *testing.T) {
		a := novoAmbiente(t)
		if _, err := a.db.ExecContext(t.Context(), "DROP TABLE Configuracao"); err != nil {
			t.Fatal(err)
		}
		a.s.Iniciar(t.Context())
		if sit := a.s.Situacao(t.Context()); sit.Ligado || sit.IP != nil {
			t.Errorf("situação %+v", sit)
		}
		if !a.log.tem("ler o IP gravado") {
			t.Errorf("o log não diz: %v", a.log.mensagens)
		}
	})
}

// A situação lista os IPs para escolher ([] e não null no JSON) e traz os avisos do firewall só ligado.
func TestSituacao(t *testing.T) {
	a := novoAmbiente(t)
	a.fw.avisos = []string{"a rede é Pública"}
	if sit := a.s.Situacao(t.Context()); sit.Avisos == nil || len(sit.Avisos) != 0 || sit.Ligado {
		t.Errorf("desligado: %+v", sit)
	}
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err != nil {
		t.Fatal(err)
	}
	if sit := a.s.Situacao(t.Context()); !reflect.DeepEqual(sit.Avisos, []string{"a rede é Pública"}) {
		t.Errorf("avisos %v", sit.Avisos)
	}
	a.mu.Lock()
	a.errLista = errors.New("sem interfaces")
	a.mu.Unlock()
	if sit := a.s.Situacao(t.Context()); sit.Enderecos == nil || len(sit.Enderecos) != 0 || !sit.IPIndisponivel {
		t.Errorf("sem a lista: %+v", sit)
	}
}

// Regra (#328): desligar com o firewall que não deixa apagar a regra é erro, mas a porta já fechou (cortar o acesso
// vem primeiro) e a escolha fica, para tentar de novo.
func TestDesligarComFirewallQueFalha(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err != nil {
		t.Fatal(err)
	}
	a.fw.errFechar = errors.New("negado")
	if _, err := a.s.Desligar(t.Context(), a.pedido()); !errors.Is(err, ErrFirewall) {
		t.Errorf("erro %v", err)
	}
	if a.s.Situacao(t.Context()).Ligado {
		t.Error("a porta continua aberta")
	}
	if ip := a.ipGravado(); ip == nil {
		t.Error("apagou a escolha sem apagar a regra")
	}
	// Banco que não grava o desligar.
	a.fw.errFechar = nil
	if _, err := a.db.ExecContext(t.Context(), "DROP TABLE AuditoriaAcesso"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.s.Desligar(t.Context(), a.pedido()); err == nil {
		t.Error("devia falhar")
	}
}

// Sem Enderecos nem Escutar, o serviço usa os do computador e o net.Listen.
func TestNovoComOsPadroes(t *testing.T) {
	s := Novo(Opcoes{DB: testeapoio.NovoBanco(t), Relogio: relogio.Fixo{Instante: agoraDosTestes}, Log: slog.New(slog.DiscardHandler)})
	if _, err := s.o.Enderecos(); err != nil {
		t.Error(err)
	}
	l, err := s.o.Escutar("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

// Regra (#328): o servidor da rede que cai sozinho (a porta fechada por fora) fica no log; o que para porque o site
// desligou, não.
func TestServidorDaRedeQueCai(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err != nil {
		t.Fatal(err)
	}
	if err := a.ouvintes["192.168.0.10:5443"].Close(); err != nil {
		t.Fatal(err)
	}
	a.s.Parar(t.Context()) // espera o servidor terminar
	if !a.log.tem("o servidor da rede parou") {
		t.Errorf("o log não diz: %v", a.log.mensagens)
	}
}
