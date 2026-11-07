// Pacote pluggy é o cliente HTTP da Pluggy (ADR 0006), sem SDK: o PluggyClient do .NET. Só fala com a Pluggy: quem grava
// no banco é a sincronização. A API key só vai ao host configurado, e nem ela nem os ids de item e de conta entram em log
// ou em mensagem de erro de rede.
package pluggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// URLPadrao é a mesma do Program.cs, usada quando Pluggy:UrlBase não está na configuração.
const URLPadrao = "https://api.pluggy.ai/"

// Limites iguais aos do PluggyClient e do AddHttpClient<PluggyClient> do Program.cs, mais um teto para o tamanho da resposta.
const (
	tempoLimite           = 100 * time.Second
	agenteDoCliente       = "FinancasLocal/1.0"
	tamanhoPagina         = 500
	paginasMaximas        = 200
	paginasDeCursor       = 1000
	tentativasDeRede      = 3
	tentativasTransitorio = 4
	esperaMaxima          = 60 * time.Second
	corpoNoErro           = 300
	tamanhoDoCorpo        = 64 << 20 // 64 MB: uma página tem até 500 itens
	redirecionamentos     = 10
)

// Erro é a PluggyException do .NET: a Pluggy respondeu, e a mensagem é para quem usa. Status 0 é sem status HTTP.
type Erro struct {
	Mensagem string
	Status   int
}

func (e *Erro) Error() string { return e.Mensagem }

var (
	// ErrRede é a HttpRequestException do .NET: o pedido não chegou até o fim, depois das tentativas.
	ErrRede = errors.New("falha de rede ao chamar a Pluggy")
	// ErrResposta é a JsonException do .NET: a Pluggy respondeu fora do formato.
	ErrResposta = errors.New("resposta da Pluggy fora do formato")
	// ErrTempoEsgotado é o limite de 100 s: como no .NET, não tenta de novo e não é ErrRede.
	ErrTempoEsgotado = errors.New("tempo esgotado ao chamar a Pluggy")
)

// Sessao é a API key, válida por cerca de 2 horas. Não aparece em log nem em mensagem.
type Sessao struct{ chave string }

func (Sessao) String() string { return "<sessão da Pluggy>" }

// Cliente consulta a Pluggy. Só HTTPS: a URL base é configurável para os testes.
type Cliente struct {
	base          *url.URL
	http          *http.Client
	log           *slog.Logger
	esperar       func(context.Context, time.Duration) error
	tamanhoMaximo int64
}

// NovoCliente cria o cliente. urlBase vazia usa a URLPadrao, e cliente nil usa um com o tempo limite de 100 s.
func NovoCliente(urlBase string, cliente *http.Client, log *slog.Logger) (*Cliente, error) {
	if urlBase == "" {
		urlBase = URLPadrao
	}
	base, err := url.Parse(urlBase)
	if err != nil {
		return nil, fmt.Errorf("URL da Pluggy inválida %q: %w", urlBase, err)
	}
	if base.Scheme != "https" {
		return nil, fmt.Errorf("a URL da Pluggy deve usar HTTPS: %q", urlBase)
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/" // como o BaseAddress do HttpClient: o caminho relativo vem depois da última barra
	}
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimite}
	}
	c := &Cliente{base: base, log: log, esperar: dormir, tamanhoMaximo: tamanhoDoCorpo}
	semFugir := *cliente
	semFugir.CheckRedirect = c.soNoMesmoHost
	c.http = &semFugir
	return c, nil
}

// soNoMesmoHost barra o redirecionamento para fora do host configurado, que levaria a API key junto: o cabeçalho
// X-API-KEY não é dos que o net/http tira ao trocar de host. O 3xx vira um erro da Pluggy.
func (c *Cliente) soNoMesmoHost(req *http.Request, anteriores []*http.Request) error {
	if req.URL.Scheme != "https" || req.URL.Host != c.base.Host {
		return http.ErrUseLastResponse
	}
	if len(anteriores) >= redirecionamentos {
		return errors.New("a Pluggy redirecionou demais")
	}
	return nil
}

// dormir é o Task.Delay: espera o tempo, ou o cancelamento.
func dormir(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// resposta é o status e o corpo já lido: como no HttpClient, falha no meio do corpo é falha do pedido.
type resposta struct {
	status     int
	retryAfter string
	corpo      []byte
}

func (r *resposta) sucesso() bool { return r.status >= 200 && r.status <= 299 }

func (c *Cliente) novoPedido(ctx context.Context, metodo, caminho string, corpo io.Reader) (*http.Request, error) {
	relativo, err := url.Parse(caminho)
	if err != nil {
		return nil, errors.New("montar o endereço da Pluggy: caminho inválido") // sem o caminho, que traz os ids
	}
	req, err := http.NewRequestWithContext(ctx, metodo, c.base.ResolveReference(relativo).String(), corpo)
	if err != nil {
		return nil, fmt.Errorf("montar o pedido à Pluggy: %w", err)
	}
	req.Header.Set("User-Agent", agenteDoCliente)
	return req, nil
}

func (c *Cliente) enviar(req *http.Request) (*resposta, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classificar(req.Context(), err)
	}
	defer resp.Body.Close()
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, c.tamanhoMaximo+1))
	if err != nil {
		return nil, classificar(req.Context(), err)
	}
	if int64(len(corpo)) > c.tamanhoMaximo {
		return nil, fmt.Errorf("%w: a resposta passou do tamanho máximo", ErrResposta)
	}
	return &resposta{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After"), corpo: corpo}, nil
}

// classificar separa a falha como o .NET: cancelamento, tempo esgotado e rede. O *url.Error sai, porque o texto dele
// traz a URL inteira, com os ids de item e de conta.
func classificar(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var rede net.Error
	if errors.As(err, &rede) && rede.Timeout() {
		return ErrTempoEsgotado
	}
	var endereco *url.Error
	if errors.As(err, &endereco) {
		err = endereco.Err
	}
	return fmt.Errorf("%w: %w", ErrRede, err)
}

// CriarSessao autentica na Pluggy. Sem nova tentativa, como no .NET.
func (c *Cliente) CriarSessao(ctx context.Context, clientId, clientSecret string) (Sessao, error) {
	credenciais, err := json.Marshal(struct {
		ClientId     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}{clientId, clientSecret})
	if err != nil {
		return Sessao{}, errors.New("montar as credenciais da Pluggy")
	}
	req, err := c.novoPedido(ctx, http.MethodPost, "auth", bytes.NewReader(credenciais))
	if err != nil {
		return Sessao{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	r, err := c.enviar(req)
	if err != nil {
		return Sessao{}, err
	}
	if !r.sucesso() {
		mensagem := fmt.Sprintf("Falha ao autenticar na Pluggy (HTTP %d).", r.status)
		if r.status == http.StatusBadRequest || r.status == http.StatusUnauthorized || r.status == http.StatusForbidden {
			mensagem = "Pluggy recusou o Client ID / Client Secret. Verifique as credenciais em Configurações."
		}
		return Sessao{}, &Erro{Mensagem: mensagem, Status: r.status}
	}
	var dados struct {
		ApiKey string `json:"apiKey"`
	}
	if err := json.Unmarshal(r.corpo, &dados); err != nil {
		return Sessao{}, fmt.Errorf("%w: %w", ErrResposta, err)
	}
	if strings.TrimSpace(dados.ApiKey) == "" {
		return Sessao{}, &Erro{Mensagem: "A Pluggy não retornou uma API key."}
	}
	return Sessao{chave: dados.ApiKey}, nil
}

// ObterItem consulta um item (uma conexão do Meu Pluggy).
func (c *Cliente) ObterItem(ctx context.Context, s Sessao, itemId string) (*Item, error) {
	var item Item
	if err := c.obter(ctx, s, "items/"+escapar(itemId), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// ListarContas devolve as contas do item.
func (c *Cliente) ListarContas(ctx context.Context, s Sessao, itemId string) ([]Conta, error) {
	var pagina paginaNumerada[Conta]
	if err := c.obter(ctx, s, "accounts?itemId="+escapar(itemId), &pagina); err != nil {
		return nil, err
	}
	return pagina.Results, nil
}

// ListarCategorias devolve as categorias da Pluggy.
func (c *Cliente) ListarCategorias(ctx context.Context, s Sessao) ([]Categoria, error) {
	var pagina paginaNumerada[Categoria]
	if err := c.obter(ctx, s, "categories", &pagina); err != nil {
		return nil, err
	}
	return pagina.Results, nil
}

// ListarFaturas devolve as faturas do cartão.
func (c *Cliente) ListarFaturas(ctx context.Context, s Sessao, contaId string) ([]Fatura, error) {
	return listarPaginado[Fatura](ctx, c, s, "bills?accountId="+escapar(contaId))
}

// ListarInvestimentos devolve os investimentos do item.
func (c *Cliente) ListarInvestimentos(ctx context.Context, s Sessao, itemId string) ([]Investimento, error) {
	return listarPaginado[Investimento](ctx, c, s, "investments?itemId="+escapar(itemId))
}

// ListarEmprestimos devolve os contratos de crédito do item.
func (c *Cliente) ListarEmprestimos(ctx context.Context, s Sessao, itemId string) ([]Emprestimo, error) {
	return listarPaginado[Emprestimo](ctx, c, s, "loans?itemId="+escapar(itemId))
}

// ListarMovimentosInvestimento devolve as movimentações de um investimento.
func (c *Cliente) ListarMovimentosInvestimento(ctx context.Context, s Sessao, investimentoId string) ([]MovimentoInvestimento, error) {
	return listarPaginado[MovimentoInvestimento](ctx, c, s, "investments/"+escapar(investimentoId)+"/transactions")
}

// ListarTransacoes usa o GET /v2/transactions, com paginação por cursor: o endpoint v1 será desativado em 2026-12-31.
func (c *Cliente) ListarTransacoes(ctx context.Context, s Sessao, contaId string, dataInicial tipos.Data) ([]Transacao, error) {
	todas := []Transacao{}
	cursor := ""
	for paginas := 1; ; paginas++ {
		caminho := "v2/transactions?accountId=" + escapar(contaId) + "&dateFrom=" + dataInicial.String()
		if cursor != "" {
			caminho += "&after=" + escapar(cursor)
		}
		var pagina paginaComCursor
		if err := c.obter(ctx, s, caminho, &pagina); err != nil {
			return nil, err
		}
		todas = append(todas, pagina.Results...)
		cursor = extrairCursor(pagina.proximo())
		if paginas > paginasDeCursor {
			return nil, &Erro{Mensagem: "Paginação de transações excedeu o limite de segurança."}
		}
		if cursor == "" {
			return todas, nil
		}
	}
}

func listarPaginado[T any](ctx context.Context, c *Cliente, s Sessao, caminho string) ([]T, error) {
	resultado := []T{}
	separador := "?"
	if strings.Contains(caminho, "?") {
		separador = "&"
	}
	for pagina := 1; ; pagina++ {
		var dados paginaNumerada[T]
		endereco := fmt.Sprintf("%s%spage=%d&pageSize=%d", caminho, separador, pagina, tamanhoPagina)
		if err := c.obter(ctx, s, endereco, &dados); err != nil {
			return nil, err
		}
		resultado = append(resultado, dados.Results...)
		if int(dados.TotalPages) <= pagina || len(dados.Results) == 0 || pagina >= paginasMaximas {
			return resultado, nil
		}
	}
}

// extrairCursor tira só o parâmetro "after" do link "next". A URL recebida nunca é seguida: assim, a API key só vai ao
// host configurado.
func extrairCursor(proximo string) string {
	_, query, tem := strings.Cut(proximo, "?")
	if !tem {
		return ""
	}
	for par := range strings.SplitSeq(query, "&") {
		chave, valor, _ := strings.Cut(par, "=")
		if !strings.EqualFold(chave, "after") {
			continue
		}
		decodificado, err := url.QueryUnescape(valor)
		if err != nil {
			decodificado = valor
		}
		if strings.TrimSpace(decodificado) == "" {
			return ""
		}
		return decodificado
	}
	return ""
}

// obter é o GET com as novas tentativas do .NET: a falha de rede tenta 3 vezes; o 429 e o 5xx, 4 vezes.
func (c *Cliente) obter(ctx context.Context, s Sessao, caminho string, destino any) error {
	for tentativa := 1; ; tentativa++ {
		req, err := c.novoPedido(ctx, http.MethodGet, caminho, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-API-KEY", s.chave)
		req.Header.Set("Accept", "application/json")

		r, err := c.enviar(req)
		if errors.Is(err, ErrRede) && tentativa < tentativasDeRede {
			c.log.Warn("falha de rede ao chamar a Pluggy", "origem", "Open Finance",
				"recurso", recursoParaLog(caminho), "erro", err.Error(), "tentativa", tentativa)
			if err := c.esperar(ctx, time.Duration(2*tentativa)*time.Second); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if r.sucesso() {
			return lerCorpo(r.corpo, caminho, destino)
		}
		if (r.status == http.StatusTooManyRequests || r.status >= 500) && tentativa < tentativasTransitorio {
			if err := c.esperar(ctx, esperaDoTransitorio(r.retryAfter, tentativa)); err != nil {
				return err
			}
			continue
		}
		return erroDeStatus(r, caminho)
	}
}

// esperaDoTransitorio é o Retry-After em segundos, ou 3 s vezes a tentativa, até 1 minuto. O Retry-After em data é
// ignorado, como o RetryAfter.Delta do .NET.
func esperaDoTransitorio(retryAfter string, tentativa int) time.Duration {
	espera := time.Duration(3*tentativa) * time.Second
	if segundos, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && segundos >= 0 {
		espera = time.Duration(min(segundos, int(esperaMaxima/time.Second))) * time.Second
	}
	return min(espera, esperaMaxima)
}

func lerCorpo(corpo []byte, caminho string, destino any) error {
	if string(bytes.TrimSpace(corpo)) == "null" {
		return &Erro{Mensagem: "Resposta vazia da Pluggy em " + caminhoSemQuery(caminho) + "."}
	}
	if err := json.Unmarshal(corpo, destino); err != nil {
		return fmt.Errorf("%w: %w", ErrResposta, err)
	}
	return nil
}

func erroDeStatus(r *resposta, caminho string) *Erro {
	var mensagem string
	limpo := sanitizarParaLog(caminhoSemQuery(caminho))
	switch r.status {
	case http.StatusUnauthorized, http.StatusForbidden:
		mensagem = "Acesso negado pela Pluggy. Verifique as credenciais e a autorização do Meu Pluggy."
	case http.StatusNotFound:
		mensagem = "Recurso não encontrado na Pluggy (" + limpo + "). Confira o Item ID."
	default:
		mensagem = fmt.Sprintf("Erro HTTP %d da Pluggy em %s: %s", r.status, limpo, respostas.CortarUtf16(string(r.corpo), corpoNoErro))
	}
	return &Erro{Mensagem: mensagem, Status: r.status}
}

func caminhoSemQuery(caminho string) string {
	semQuery, _, _ := strings.Cut(caminho, "?")
	return semQuery
}

// recursoParaLog é só o nome do recurso, sem os ids de item e de conta que vão no caminho.
func recursoParaLog(caminho string) string {
	recurso, _, _ := strings.Cut(caminhoSemQuery(caminho), "/")
	switch recurso {
	case "items", "accounts", "categories", "investments", "loans":
		return recurso
	case "v2":
		return "v2/transactions"
	}
	return "outro"
}

func sanitizarParaLog(valor string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(valor)
}

// escapar é o Uri.EscapeDataString do .NET: só as letras, os dígitos e "-_.~" passam. O net/url escapa menos no caminho
// e troca o espaço por "+" na query.
func escapar(valor string) string {
	var b strings.Builder
	for _, c := range []byte(valor) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
