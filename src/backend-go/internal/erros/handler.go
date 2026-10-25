package erros

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
)

// NivelCritico é o nível do slog para o LogLevel.Critical do .NET (o slog não tem um).
const NivelCritico = slog.LevelError + 4

// CategoriaLog é a categoria dos registros do Go, a mesma que o .NET usa para o log do Go repassado (ProcessoGo):
// "Financas.Go", ou "Financas.Go.<origem>" quando o log diz a origem. A assinatura (agrupamento) usa a categoria, e
// assim o mesmo aviso continua agrupado na troca do .NET para o Go.
const CategoriaLog = "Financas.Go"

// Handler é o RegistroErrosLoggerProvider: repassa tudo ao próximo handler (o log de sempre) e, dos avisos para
// cima, põe um Registro na fila. O que o gravador loga vai direto para o próximo handler, para uma falha ao gravar não
// virar outro erro a gravar.
type Handler struct {
	proximo slog.Handler
	fila    *Fila
	relogio relogio.Relogio
	attrs   []slog.Attr // os do Logger.With, já com o nome do grupo
	grupo   string
}

func NovoHandler(proximo slog.Handler, fila *Fila, r relogio.Relogio) *Handler {
	return &Handler{proximo: proximo, fila: fila, relogio: r}
}

func (h *Handler) Enabled(ctx context.Context, nivel slog.Level) bool {
	return nivel >= slog.LevelWarn || h.proximo.Enabled(ctx, nivel)
}

func (h *Handler) Handle(ctx context.Context, rec slog.Record) error {
	if rec.Level >= slog.LevelWarn {
		h.fila.Registrar(h.registro(ctx, rec))
	}
	if !h.proximo.Enabled(ctx, rec.Level) {
		return nil
	}
	return h.proximo.Handle(ctx, rec)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	novo := *h
	novo.proximo = h.proximo.WithAttrs(attrs)
	novo.attrs = append(append([]slog.Attr(nil), h.attrs...), comGrupo(h.grupo, attrs)...)
	return &novo
}

func (h *Handler) WithGroup(nome string) slog.Handler {
	if nome == "" {
		return h
	}
	novo := *h
	novo.proximo = h.proximo.WithGroup(nome)
	novo.grupo = h.grupo + nome + "."
	return &novo
}

// comGrupo põe os atributos dentro do grupo aberto por WithGroup ("g." vira slog.Group("g", ...)).
func comGrupo(grupo string, attrs []slog.Attr) []slog.Attr {
	if grupo == "" {
		return attrs
	}
	nomes := strings.Split(strings.TrimSuffix(grupo, "."), ".")
	valor := slog.GroupValue(attrs...)
	for i := len(nomes) - 1; i >= 0; i-- {
		valor = slog.GroupValue(slog.Attr{Key: nomes[i], Value: valor})
	}
	return valor.Group()
}

// registro monta o Registro como o .NET monta o do log do Go (ProcessoGo e RegistroErrosLogger):
//   - origem: o atributo "origem", se for uma das conhecidas; senão "Sistema";
//   - mensagem: a do log mais os outros atributos, "msg (chave=valor ...)", como a linha JSON vira no .NET;
//   - detalhe: os atributos que são error, com o tipo do erro mais de dentro (o que foi embrulhado com %w) na
//     primeira linha. É o "tipo da exceção" da assinatura: o mesmo erro de E/S agrupa, com mensagens diferentes;
//   - rota e usuário: os da requisição, quando o log foi feito com o contexto dela (ComRequisicao).
func (h *Handler) registro(ctx context.Context, rec slog.Record) Registro {
	origem, categoria := OrigemSistema, CategoriaLog
	var partes, detalhes []string
	tratar := func(a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Key == "origem" && a.Value.Kind() == slog.KindString {
			if o := a.Value.String(); origensConhecidas[o] {
				origem, categoria = o, CategoriaLog+"."+o
			}
			return
		}
		if err, ok := a.Value.Any().(error); ok && a.Value.Kind() == slog.KindAny {
			detalhes = append(detalhes, detalheDoErro(err))
			return
		}
		partes = append(partes, a.Key+"="+textoDoValor(a.Value))
	}
	for _, a := range h.attrs {
		tratar(a)
	}
	rec.Attrs(func(a slog.Attr) bool {
		for _, a := range comGrupo(h.grupo, []slog.Attr{a}) {
			tratar(a)
		}
		return true
	})

	mensagem := rec.Message
	if len(partes) > 0 {
		mensagem += " (" + strings.Join(partes, " ") + ")"
	}
	if strings.TrimSpace(mensagem) == "" {
		mensagem = "(sem mensagem)"
	}
	r := Registro{Em: h.relogio.Agora(), Nivel: nivelDe(rec.Level), Origem: origem, Categoria: categoria, Mensagem: mensagem}
	if len(detalhes) > 0 {
		d := strings.Join(detalhes, "\n")
		r.Detalhe = &d
	}
	if req, ok := ctx.Value(chaveRequisicao{}).(requisicao); ok {
		rota := req.rota
		r.Rota, r.Usuario = &rota, req.usuario
	}
	return r
}

func nivelDe(n slog.Level) Nivel {
	switch {
	case n >= NivelCritico:
		return Critico
	case n >= slog.LevelError:
		return Erro
	default:
		return Aviso
	}
}

// detalheDoErro é o "tipo: mensagem" do erro, com a cadeia dos tipos na segunda linha. O tipo, que vai até o
// primeiro ":" e entra na assinatura, é o do erro mais de dentro (errors.Unwrap) que diz alguma coisa: os genéricos
// (errors.New, os embrulhos do fmt.Errorf e do errors.Join) não contam, porque todo erro teria o mesmo. Só com
// genéricos, vale o de fora.
func detalheDoErro(err error) string {
	tipos := []string{fmt.Sprintf("%T", err)}
	tipo := ""
	if !tiposGenericos[tipos[0]] {
		tipo = tipos[0]
	}
	for dentro := desembrulhar(err); dentro != nil; dentro = desembrulhar(dentro) {
		nome := fmt.Sprintf("%T", dentro)
		tipos = append(tipos, nome)
		if !tiposGenericos[nome] {
			tipo = nome
		}
	}
	if tipo == "" {
		tipo = tipos[0]
	}
	texto := tipo + ": " + err.Error()
	if len(tipos) > 1 {
		texto += "\n   cadeia: " + strings.Join(tipos, " → ")
	}
	return texto
}

var tiposGenericos = map[string]bool{
	"*errors.errorString": true, "*fmt.wrapError": true, "*fmt.wrapErrors": true, "*errors.joinError": true,
}

// desembrulhar é o errors.Unwrap, e o primeiro da lista no erro de errors.Join.
func desembrulhar(err error) error {
	switch e := err.(type) {
	case interface{ Unwrap() error }:
		return e.Unwrap()
	case interface{ Unwrap() []error }:
		if lista := e.Unwrap(); len(lista) > 0 {
			return lista[0]
		}
	}
	return nil
}

// textoDoValor escreve o valor como ele sai no slog.JSONHandler e o .NET o lê: texto cru, e o resto em JSON.
func textoDoValor(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindDuration:
		return strconv.FormatInt(int64(v.Duration()), 10)
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindGroup:
		objeto := map[string]any{}
		for _, a := range v.Group() {
			objeto[a.Key] = textoDoValor(a.Value.Resolve())
		}
		b, _ := json.Marshal(objeto)
		return string(b)
	}
	b, err := json.Marshal(v.Any())
	if err != nil {
		return fmt.Sprint(v.Any())
	}
	var texto string
	if json.Unmarshal(b, &texto) == nil {
		return texto
	}
	return string(b)
}

type chaveRequisicao struct{}

type requisicao struct {
	rota    string
	usuario *string
}

// ComRequisicao guarda no contexto a rota ("GET /api/contas", sem a query string) e o login de quem está na sessão:
// um log feito com esse contexto (log.ErrorContext) leva os dois para a tabela, como o HttpContext no .NET.
func ComRequisicao(ctx context.Context, metodo, caminho string, usuario *string) context.Context {
	return context.WithValue(ctx, chaveRequisicao{}, requisicao{rota: metodo + " " + caminho, usuario: usuario})
}

var _ slog.Handler = (*Handler)(nil)
