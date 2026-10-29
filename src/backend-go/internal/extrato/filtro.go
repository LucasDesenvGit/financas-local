package extrato

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// filtro é o FiltroTransacoesDto do C#, lido da query ([FromQuery]).
type filtro struct {
	Inicio, Fim        *tipos.Data
	ContaId            *int64
	CategoriaId        *int64
	CategoriaIds       []int64
	SemCategoria       bool
	Naturezas          []int64
	SemNatureza        bool
	SomenteEmergencias bool
	PoucoInformativa   bool
	Lancamento         *string
	Tipo               *int64
	Busca              *string
	Ids                []int64
	Pagina             int64
	TamanhoPagina      int64
	Ordem, Direcao     *string
}

// Os enums que o filtro lê da query, com os nomes em minúsculas (Entidades/Enums.cs).
var (
	nomesDaNatureza = map[string]int64{"essencial": 1, "estilodevida": 2}
	nomesDoTipo     = map[string]int64{"despesa": 1, "receita": 2, "transferencia": 3}
)

// Os [RegularExpression] do DTO, com o texto que aparece na mensagem.
var (
	padraoLancamento = respostas.NovoPadrao(`^(cartao|debito|pix|transferencia|entre-contas|fatura|boleto|saque-deposito|tarifas|investimentos|salario|credito|dinheiro|outros)$`)
	padraoOrdem      = respostas.NovoPadrao(`^(data|categoria|conta|valor)$`)
	padraoDirecao    = respostas.NovoPadrao(`^(asc|desc)$`)
)

// lerFiltro é o model binding do FiltroTransacoesDto: cada propriedade pelo nome (sem diferença de maiúsculas), as
// listas repetidas ou com índice, e os atributos de validação. O erro de conversão de uma propriedade cita o nome dela
// ("…is not valid for Pagina."); o de um item de lista, não ("…is not valid.", na chave da lista). A propriedade que não
// converte fica com o padrão e não passa pela validação.
func lerFiltro(r *http.Request) (filtro, map[string][]string) {
	f := filtro{Pagina: 1, TamanhoPagina: 50}
	erros := map[string][]string{}
	erro := func(propriedade, valor string) {
		erros[propriedade] = append(erros[propriedade], respostas.MensagemDaPropriedade(valor, propriedade))
	}

	data := func(nome string, destino **tipos.Data) {
		if v, ok := respostas.ParametroDaQuery(r, nome); ok && strings.TrimSpace(v) != "" {
			if d, ok := respostas.DataDaQuery(v); ok {
				*destino = &d
			} else {
				erro(nome, v)
			}
		}
	}
	data("Inicio", &f.Inicio)
	data("Fim", &f.Fim)

	inteiroOpcional := func(nome string, destino **int64) {
		if v, ok := respostas.ParametroDaQuery(r, nome); ok && strings.TrimSpace(v) != "" {
			if n, ok := respostas.InteiroDaQuery(v); ok {
				x := int64(n)
				*destino = &x
			} else {
				erro(nome, v)
			}
		}
	}
	inteiroOpcional("ContaId", &f.ContaId)
	inteiroOpcional("CategoriaId", &f.CategoriaId)

	inteiro := func(nome string, destino *int64) bool {
		v, ok := respostas.ParametroDaQuery(r, nome)
		if !ok {
			return true
		}
		n, ok := respostas.InteiroDaQuery(v)
		if !ok {
			erro(nome, v)
			return false
		}
		*destino = int64(n)
		return true
	}
	validarPagina := inteiro("Pagina", &f.Pagina)
	validarTamanho := inteiro("TamanhoPagina", &f.TamanhoPagina)

	booleano := func(nome string, destino *bool) {
		if v, ok := respostas.ParametroDaQuery(r, nome); ok {
			if b, ok := respostas.BoolDaQuery(v); ok {
				*destino = b
			} else {
				erro(nome, v)
			}
		}
	}
	booleano("SemCategoria", &f.SemCategoria)
	booleano("SemNatureza", &f.SemNatureza)
	booleano("SomenteEmergencias", &f.SomenteEmergencias)
	booleano("PoucoInformativa", &f.PoucoInformativa)

	texto := func(nome string) *string {
		// string vazia vira null (ConvertEmptyStringToNull).
		if v, ok := respostas.ParametroDaQuery(r, nome); ok && v != "" {
			return &v
		}
		return nil
	}
	f.Lancamento, f.Busca, f.Ordem, f.Direcao = texto("Lancamento"), texto("Busca"), texto("Ordem"), texto("Direcao")

	if v, ok := respostas.ParametroDaQuery(r, "Tipo"); ok && strings.TrimSpace(v) != "" {
		switch n, converte, definido := respostas.EnumDaQuery(v, nomesDoTipo); {
		case !converte:
			erro("Tipo", v)
		case !definido:
			erros["Tipo"] = append(erros["Tipo"], respostas.MensagemDeValorInvalido(v))
		default:
			f.Tipo = &n
		}
	}

	// As listas: cada item que não converte é um erro na chave da lista, sem o nome no texto.
	lista := func(nome string, ler func(string) (int64, string)) []int64 {
		var valores []int64
		for _, v := range respostas.ValoresDaQuery(r, nome) {
			n, mensagem := ler(v)
			if mensagem != "" {
				erros[nome] = append(erros[nome], mensagem)
				continue
			}
			valores = append(valores, n)
		}
		return valores
	}
	f.CategoriaIds = lista("CategoriaIds", func(v string) (int64, string) {
		if n, ok := respostas.InteiroDaQuery(v); ok {
			return int64(n), ""
		}
		return 0, respostas.MensagemDaQuery(v)
	})
	f.Ids = lista("Ids", func(v string) (int64, string) {
		if n, ok := respostas.LongDaQuery(v); ok {
			return n, ""
		}
		return 0, respostas.MensagemDaQuery(v)
	})
	f.Naturezas = lista("Naturezas", func(v string) (int64, string) {
		switch n, converte, definido := respostas.EnumDaQuery(v, nomesDaNatureza); {
		case !converte:
			return 0, respostas.MensagemDaQuery(v)
		case !definido:
			return 0, respostas.MensagemDeValorInvalido(v)
		default:
			return n, ""
		}
	})

	// Validação (os atributos do DTO), só no que converteu.
	validar := func(propriedade string, invalido bool, mensagem string) {
		if invalido && erros[propriedade] == nil {
			erros[propriedade] = []string{mensagem}
		}
	}
	maximoDaLista := func(propriedade string, n, maximo int) {
		validar(propriedade, n > maximo, "The field "+propriedade+" must be a string or array type with a maximum length of '"+
			strconv.Itoa(maximo)+"'.")
	}
	maximoDaLista("CategoriaIds", len(f.CategoriaIds), 200)
	maximoDaLista("Naturezas", len(f.Naturezas), 2)
	maximoDaLista("Ids", len(f.Ids), 200)
	padrao := func(propriedade string, valor *string, p *respostas.Padrao) {
		if valor != nil && !p.Casa(*valor) {
			validar(propriedade, true, p.Mensagem(propriedade))
		}
	}
	padrao("Lancamento", f.Lancamento, padraoLancamento)
	padrao("Ordem", f.Ordem, padraoOrdem)
	padrao("Direcao", f.Direcao, padraoDirecao)
	if f.Busca != nil {
		validar("Busca", respostas.TamanhoUtf16(*f.Busca) > 100, "The field Busca must be a string with a maximum length of 100.")
	}
	if validarPagina {
		validar("Pagina", f.Pagina < 1, "The field Pagina must be between 1 and 2147483647.")
	}
	if validarTamanho {
		validar("TamanhoPagina", f.TamanhoPagina < 1 || f.TamanhoPagina > 500, "The field TamanhoPagina must be between 1 and 500.")
	}
	return f, erros
}
