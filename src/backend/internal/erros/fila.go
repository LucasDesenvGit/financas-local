// Pacote erros é o registro de erros do sistema em Go (#298, ADR 0022): o handler do slog que manda avisos e erros
// para uma fila, o gravador que os grava em ErroSistema (agrupados e limpos de dados sensíveis) e as rotas de
// api/erros.
//
// Só vale com o Go sozinho, que é como o site vai rodar depois da inversão (#85). Atrás do .NET, o log do Go sai em
// JSON na saída de erro e quem grava é o .NET (ADR 0049); /api/erros fica no .NET até a inversão.
package erros

import (
	"sync"
	"time"
)

// Nivel é o NivelErro do C# (os números do LogLevel do .NET). No JSON sai como número.
type Nivel int

const (
	Aviso   Nivel = 3
	Erro    Nivel = 4
	Critico Nivel = 5
)

// Origens da tela de erros. As que o Go usa no atributo "origem" do log são as de origensConhecidas.
const (
	OrigemAPI      = "API"
	OrigemFrontEnd = "Front-end"
	OrigemSistema  = "Sistema"
	OrigemCripto   = "Cripto" // #298: antes caía em "Sistema"
)

// origensConhecidas são as que o atributo "origem" do log pode trazer (as mesmas do ProcessoGo.OrigensConhecidas).
var origensConhecidas = map[string]bool{
	"API": true, "Open Finance": true, "Investimentos": true, "Câmbio": true, OrigemCripto: true, OrigemSistema: true,
}

// Registro é um erro ou aviso ainda não gravado (texto ainda sem limpeza): o RegistroErro do C#.
type Registro struct {
	Em        time.Time // UTC
	Nivel     Nivel
	Origem    string
	Categoria string
	Mensagem  string
	Detalhe   *string
	Rota      *string
	Usuario   *string
}

const (
	tamanhoFila           = 1000
	maximoFrontEndPorHora = 30 // a tela é do próprio usuário, mas um laço não pode lotar o banco
)

// Fila fica entre quem registra o erro (o handler do log, o endpoint do front-end) e o gravador. Registrar nunca
// espera: com a fila cheia (banco travado, erro em laço), o registro é descartado; o log continua recebendo tudo.
type Fila struct {
	canal    chan Registro
	trava    sync.Mutex
	frontEnd []time.Time // quando chegaram os erros do front-end da última hora
}

func NovaFila() *Fila {
	return &Fila{canal: make(chan Registro, tamanhoFila)}
}

// Registrar põe o registro na fila; false se ela está cheia (o BoundedChannelFullMode.DropWrite do .NET).
func (f *Fila) Registrar(r Registro) bool {
	select {
	case f.canal <- r:
		return true
	default:
		return false
	}
}

// RegistrarDoFrontEnd é o erro vindo do navegador, limitado a 30 por hora (contados pela hora do registro).
func (f *Fila) RegistrarDoFrontEnd(r Registro) bool {
	f.trava.Lock()
	limite := r.Em.Add(-time.Hour)
	for len(f.frontEnd) > 0 && f.frontEnd[0].Before(limite) {
		f.frontEnd = f.frontEnd[1:]
	}
	if len(f.frontEnd) >= maximoFrontEndPorHora {
		f.trava.Unlock()
		return false
	}
	f.frontEnd = append(f.frontEnd, r.Em)
	f.trava.Unlock()
	return f.Registrar(r)
}
