package erros

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// filtro é o FiltroErrosDto, lido da query string ([FromQuery]).
type filtro struct {
	inicio, fim   *tipos.Data
	origem        *string
	nivel         *Nivel
	situacao      *string // abertos (padrão), resolvidos ou todos
	pagina        int
	tamanhoPagina int
}

var situacaoValida = regexp.MustCompile(`^(abertos|resolvidos|todos)$`)

// lerFiltro faz o model binding e a validação do FiltroErrosDto como o ASP.NET (o contrato 17-erros grava cada
// mensagem): devolve os erros por propriedade, ou nil.
//
//   - o nome do parâmetro não liga para maiúsculas, e repetido vale o primeiro;
//   - texto vazio ou só com espaços é nulo (ConvertEmptyStringToNull); num int, que não aceita nulo, é erro;
//   - int pelo Int32Converter: espaços em volta, sinal, e hexadecimal com 0x ou &h;
//   - nível pelo nome (sem diferença de maiúsculas) ou pelo número, e tem de ser um dos três;
//   - data pelo DateOnly.Parse da cultura invariante: 2026-10-03, 2026-10-3, 10/03/2026 (mês primeiro) e com hora;
//   - os atributos ([Range], [RegularExpression], [StringLength]) só olham o que foi lido.
func lerFiltro(consulta string) (filtro, map[string][]string) {
	f := filtro{pagina: 1, tamanhoPagina: 25}
	valores := primeirosValores(consulta)
	erros := map[string][]string{}
	falhou := func(propriedade, mensagem string) { erros[propriedade] = append(erros[propriedade], mensagem) }

	inteiro := func(propriedade string, destino *int) bool {
		bruto, ok := valores[strings.ToLower(propriedade)]
		if !ok {
			return true
		}
		if strings.TrimSpace(bruto) == "" {
			falhou(propriedade, fmt.Sprintf("The value '%s' is invalid.", bruto))
			return false
		}
		n, mensagem := lerInt32(bruto, propriedade)
		if mensagem != "" {
			falhou(propriedade, mensagem)
			return false
		}
		*destino = n
		return true
	}
	data := func(propriedade string) *tipos.Data {
		bruto, ok := valores[strings.ToLower(propriedade)]
		if !ok || strings.TrimSpace(bruto) == "" {
			return nil
		}
		d, ok := lerData(bruto)
		if !ok {
			falhou(propriedade, naoValido(bruto, propriedade))
			return nil
		}
		return &d
	}
	texto := func(propriedade string) *string {
		bruto, ok := valores[strings.ToLower(propriedade)]
		if !ok || strings.TrimSpace(bruto) == "" {
			return nil
		}
		return &bruto
	}

	f.inicio, f.fim = data("Inicio"), data("Fim")
	f.origem, f.situacao = texto("Origem"), texto("Situacao")
	if bruto, ok := valores["nivel"]; ok && strings.TrimSpace(bruto) != "" {
		n, mensagem := lerNivel(bruto)
		if mensagem != "" {
			falhou("Nivel", mensagem)
		} else {
			f.nivel = &n
		}
	}
	paginaLida := inteiro("Pagina", &f.pagina)
	tamanhoLido := inteiro("TamanhoPagina", &f.tamanhoPagina)

	if f.origem != nil && respostas.TamanhoUtf16(*f.origem) > 30 {
		falhou("Origem", "The field Origem must be a string with a maximum length of 30.")
	}
	if f.situacao != nil && !situacaoValida.MatchString(*f.situacao) {
		falhou("Situacao", "The field Situacao must match the regular expression '^(abertos|resolvidos|todos)$'.")
	}
	if paginaLida && f.pagina < 1 {
		falhou("Pagina", "The field Pagina must be between 1 and 2147483647.")
	}
	if tamanhoLido && (f.tamanhoPagina < 1 || f.tamanhoPagina > 100) {
		falhou("TamanhoPagina", "The field TamanhoPagina must be between 1 and 100.")
	}
	if len(erros) > 0 {
		return f, erros
	}
	return f, nil
}

// primeirosValores lê a query string na ordem do texto, com o nome em minúsculas: o primeiro valor de cada nome.
func primeirosValores(consulta string) map[string]string {
	valores := map[string]string{}
	for _, par := range strings.Split(consulta, "&") {
		if par == "" {
			continue
		}
		nome, valor, _ := strings.Cut(par, "=")
		nome, err1 := url.QueryUnescape(nome)
		valor, err2 := url.QueryUnescape(valor)
		if err1 != nil || err2 != nil {
			continue
		}
		nome = strings.ToLower(nome)
		if _, existe := valores[nome]; !existe {
			valores[nome] = valor
		}
	}
	return valores
}

func naoValido(bruto, propriedade string) string {
	return fmt.Sprintf("The value '%s' is not valid for %s.", bruto, propriedade)
}

// lerInt32 é o Int32Converter: o texto sem os espaços das pontas; com 0x ou &h, hexadecimal de até 8 dígitos (os 32
// bits, como o Convert.ToInt32(texto, 16)); senão, o int.Parse da cultura invariante. Hexadecimal vazio não é um
// FormatException no .NET, e a mensagem é a genérica.
func lerInt32(bruto, propriedade string) (int, string) {
	texto := strings.Trim(bruto, " \t\n\v\f\r")
	for _, prefixo := range []string{"0x", "0X", "&h", "&H"} {
		if hex, ok := strings.CutPrefix(texto, prefixo); ok {
			if hex == "" {
				return 0, "The input was not valid."
			}
			n, err := strconv.ParseUint(hex, 16, 32)
			if err != nil {
				return 0, naoValido(bruto, propriedade)
			}
			v := int64(n)
			if v > math.MaxInt32 {
				v -= 1 << 32 // os 32 bits em complemento de dois: 0xFFFFFFFF é -1
			}
			return int(v), ""
		}
	}
	n, ok := configuracao.LerInteiro(texto)
	if !ok {
		return 0, naoValido(bruto, propriedade)
	}
	return n, ""
}

var nomesDosNiveis = map[string]Nivel{"aviso": Aviso, "erro": Erro, "critico": Critico}

// lerNivel é o EnumConverter mais o EnumTypeModelBinder: nome (sem diferença de maiúsculas) ou número, vários
// separados por vírgula (que se juntam, como flags); o que não se lê é "not valid for", e o número que não é um dos
// três é "is invalid".
func lerNivel(bruto string) (Nivel, string) {
	total := 0
	for _, parte := range strings.Split(bruto, ",") {
		parte = strings.TrimSpace(parte)
		if n, ok := nomesDosNiveis[strings.ToLower(parte)]; ok {
			total |= int(n)
			continue
		}
		n, ok := configuracao.LerInteiro(parte)
		if !ok {
			return 0, naoValido(bruto, "Nivel")
		}
		total |= n
	}
	switch Nivel(total) {
	case Aviso, Erro, Critico:
		return Nivel(total), ""
	}
	return 0, fmt.Sprintf("The value '%s' is invalid.", bruto)
}

// Formatos de data que o DateOnly.Parse da cultura invariante aceita e que fazem sentido numa URL. A hora, se vier,
// é conferida e descartada.
var (
	formatosData = []string{"2006-1-2", "2006/1/2", "1/2/2006"}
	formatosHora = []string{"15:04", "15:04:05", "15:04:05.9999999", "3:04 PM", "3:04:05 PM"}
)

func lerData(bruto string) (tipos.Data, bool) {
	texto := strings.TrimSpace(bruto)
	parteData, parteHora, temHora := strings.Cut(strings.Replace(texto, "T", " ", 1), " ")
	for _, formato := range formatosData {
		d, err := time.Parse(formato, parteData)
		if err != nil {
			continue
		}
		if temHora && !horaValida(strings.TrimSpace(parteHora)) {
			return tipos.Data{}, false
		}
		return tipos.DataDe(d), true
	}
	return tipos.Data{}, false
}

func horaValida(texto string) bool {
	for _, formato := range formatosHora {
		if _, err := time.Parse(formato, texto); err == nil {
			return true
		}
	}
	return false
}

// condicoes é o Filtrar do controller em SQL: só texto fixo, e os valores em parâmetros "?". O período é pela
// última ocorrência, em dias de Brasília.
func (f filtro) condicoes() (string, []any) {
	var partes []string
	var args []any
	situacao := "abertos"
	if f.situacao != nil {
		situacao = *f.situacao
	}
	switch situacao {
	case "resolvidos":
		partes = append(partes, `"Resolvido" = 1`)
	case "todos":
	default:
		partes = append(partes, `"Resolvido" = 0`)
	}
	if f.origem != nil && strings.TrimSpace(*f.origem) != "" {
		partes = append(partes, `"Origem" = ?`)
		args = append(args, *f.origem)
	}
	if f.nivel != nil {
		partes = append(partes, `"Nivel" = ?`)
		args = append(args, int(*f.nivel))
	}
	if f.inicio != nil {
		partes = append(partes, `"UltimaEm" >= ?`)
		args = append(args, tipos.NovaDataHora(relogio.InicioDoDiaUtc(*f.inicio)))
	}
	if f.fim != nil {
		partes = append(partes, `"UltimaEm" < ?`)
		args = append(args, tipos.NovaDataHora(relogio.InicioDoDiaUtc(f.fim.SomarDias(1))))
	}
	if len(partes) == 0 {
		return "1 = 1", args
	}
	return strings.Join(partes, " AND "), args
}
