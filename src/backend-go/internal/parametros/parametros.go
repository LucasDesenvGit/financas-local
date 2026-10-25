// Pacote parametros é o ParametroService e o ParametrosController do .NET (#76): a Parametrização do site (o
// catálogo de parâmetros, a gravação que valida tudo antes e o diagnóstico do sistema).
package parametros

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
)

// iniciadoEm é o Process.GetCurrentProcess().StartTime do .NET: a hora em que o processo subiu (do sistema, e não do
// relógio de negócio: é diagnóstico).
var iniciadoEm = time.Now()

// Opcoes do serviço, passadas pelo construtor.
type Opcoes struct {
	DB           *sql.DB
	Relogio      relogio.Relogio
	Log          *slog.Logger
	Versao       string
	CaminhoBanco string // Banco:Caminho
	PastaChaves  string // Seguranca:PastaChaves
	// Padroes são os valores do appsettings para os parâmetros que existiam lá antes (DoAppsettings), pela chave do
	// catálogo; sem a chave aqui, vale o padrão do catálogo.
	Padroes map[string]int
}

type Servico struct {
	o      Opcoes
	q      *consultas.Queries
	config *configuracao.Servico
}

func Novo(o Opcoes) *Servico {
	return &Servico{o: o, q: consultas.New(o.DB), config: configuracao.Novo(o.DB, o.Relogio)}
}

// ParametroDto, na ordem das propriedades do record do C#.
type ParametroDto struct {
	Chave     string  `json:"chave"`
	Grupo     string  `json:"grupo"`
	Nome      string  `json:"nome"`
	Descricao string  `json:"descricao"`
	Tipo      Tipo    `json:"tipo"`
	Valor     int     `json:"valor"`
	Padrao    int     `json:"padrao"`
	Minimo    *int    `json:"minimo"`
	Maximo    *int    `json:"maximo"`
	Unidade   *string `json:"unidade"`
}

// InfoSistemaDto, na ordem do record do C#. DotNet e SistemaOperacional dizem o ambiente do Go (a versão do Go e o
// sistema): o nome da propriedade fica o do .NET, que a tela lê.
type InfoSistemaDto struct {
	Versao              string                 `json:"versao"`
	DotNet              string                 `json:"dotNet"`
	SistemaOperacional  string                 `json:"sistemaOperacional"`
	IniciadoEm          configuracao.Instante  `json:"iniciadoEm"`
	CaminhoBanco        string                 `json:"caminhoBanco"`
	TamanhoBancoBytes   *int64                 `json:"tamanhoBancoBytes"`
	PastaChaves         string                 `json:"pastaChaves"`
	Contas              int64                  `json:"contas"`
	Transacoes          int64                  `json:"transacoes"`
	ConexoesAtivas      int64                  `json:"conexoesAtivas"`
	UltimaSincronizacao *configuracao.Instante `json:"ultimaSincronizacao"`
}

// Valor é um par chave e valor do dicionário Valores, na ordem em que veio no JSON.
type Valor struct {
	Chave string
	Valor int
}

// PadroesDoAppsettings são os valores do appsettings dos parâmetros que existiam lá antes (DoAppsettings).
func PadroesDoAppsettings(cfg config.Config) map[string]int {
	return map[string]int{
		"Seguranca.MaximoTentativasLogin": cfg.MaximoTentativasLogin,
		"Seguranca.MinutosBloqueio":       cfg.MinutosBloqueio,
	}
}

// Padrao é o Padrao(p) do C#: o do appsettings, quando o parâmetro existia lá antes, ou o do catálogo.
func (s *Servico) Padrao(d Definicao) int {
	if n, ok := s.o.Padroes[d.Chave]; ok && d.DoAppsettings {
		return n
	}
	return d.Padrao
}

// ObterInteiro é o ObterInteiroAsync: o valor atual de um parâmetro inteiro do catálogo, limitado à faixa válida.
func (s *Servico) ObterInteiro(ctx context.Context, chave string) (int, error) {
	d, ok := definicao(chave)
	if !ok {
		return 0, fmt.Errorf("parâmetro fora do catálogo: %s", chave)
	}
	valor, err := s.config.ObterInteiro(ctx, chave, s.Padrao(d))
	if err != nil {
		return 0, err
	}
	return limitar(valor, d.Minimo, d.Maximo), nil
}

// Listar é o ListarAsync: cada parâmetro do catálogo com o valor gravado (sem os segredos), ou o padrão quando não
// há valor ou ele não se lê.
func (s *Servico) Listar(ctx context.Context) ([]ParametroDto, error) {
	linhas, err := s.q.ListarConfiguracoes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar as configurações: %w", err)
	}
	gravados := map[string]*string{}
	for _, c := range linhas {
		if !c.Protegido {
			gravados[c.Chave] = c.Valor
		}
	}
	lista := make([]ParametroDto, 0, len(Catalogo))
	for _, d := range Catalogo {
		padrao := s.Padrao(d)
		valor := padrao
		if texto := gravados[d.Chave]; texto != nil {
			if n, ok := normalizar(d, *texto); ok {
				valor = n
			}
		}
		dto := ParametroDto{Chave: d.Chave, Grupo: d.Grupo, Nome: d.Nome, Descricao: d.Descricao, Tipo: d.Tipo,
			Valor: valor, Padrao: padrao, Unidade: d.Unidade}
		if d.Tipo == Inteiro {
			dto.Minimo, dto.Maximo = &d.Minimo, &d.Maximo
		}
		lista = append(lista, dto)
	}
	return lista, nil
}

// validar é a primeira metade do SalvarAsync: as mensagens de erro, na ordem dos valores enviados.
func validar(valores []Valor) []string {
	var erros []string
	for _, v := range valores {
		d, ok := definicao(v.Chave)
		switch {
		case !ok:
			erros = append(erros, fmt.Sprintf("Parâmetro desconhecido: %s.", v.Chave))
		case d.Tipo == Booleano && v.Valor != 0 && v.Valor != 1:
			erros = append(erros, fmt.Sprintf("%s: use ligado ou desligado.", d.Nome))
		case d.Tipo == Inteiro && (v.Valor < d.Minimo || v.Valor > d.Maximo):
			unidade := ""
			if d.Unidade != nil {
				unidade = " " + *d.Unidade
			}
			erros = append(erros, fmt.Sprintf("%s: informe um valor entre %d e %d%s.", d.Nome, d.Minimo, d.Maximo, unidade))
		}
	}
	return erros
}

// gravar é a segunda metade do SalvarAsync, dentro da transação de quem chama: booleano vira "true"/"false", inteiro
// vira o número sem separador.
func (s *Servico) gravar(ctx context.Context, tx *sql.Tx, valores []Valor) error {
	config := s.config.ComTx(tx)
	for _, v := range valores {
		d, _ := definicao(v.Chave)
		texto := strconv.Itoa(v.Valor)
		if d.Tipo == Booleano {
			texto = strconv.FormatBool(v.Valor == 1)
		}
		if err := config.Definir(ctx, v.Chave, &texto); err != nil {
			return err
		}
	}
	return nil
}

// InfoSistema é o InfoSistemaAsync: diagnóstico somente leitura (não expõe segredos).
func (s *Servico) InfoSistema(ctx context.Context) (InfoSistemaDto, error) {
	caminhoBanco, err := filepath.Abs(s.o.CaminhoBanco)
	if err != nil {
		return InfoSistemaDto{}, err
	}
	pastaChaves, err := filepath.Abs(s.o.PastaChaves)
	if err != nil {
		return InfoSistemaDto{}, err
	}
	info := InfoSistemaDto{
		Versao: s.o.Versao, DotNet: runtime.Version(), SistemaOperacional: descricaoDoSistema(),
		IniciadoEm: configuracao.InstanteUTC(iniciadoEm), CaminhoBanco: caminhoBanco, PastaChaves: pastaChaves,
	}
	if arquivo, err := os.Stat(caminhoBanco); err == nil && arquivo.Mode().IsRegular() {
		tamanho := arquivo.Size()
		info.TamanhoBancoBytes = &tamanho
	}
	if info.Contas, err = s.q.ContarContas(ctx); err != nil {
		return InfoSistemaDto{}, fmt.Errorf("contar as contas: %w", err)
	}
	if info.Transacoes, err = s.q.ContarTransacoes(ctx); err != nil {
		return InfoSistemaDto{}, fmt.Errorf("contar as transações: %w", err)
	}
	if info.ConexoesAtivas, err = s.q.ContarConexoesAtivas(ctx); err != nil {
		return InfoSistemaDto{}, fmt.Errorf("contar as conexões: %w", err)
	}
	if info.UltimaSincronizacao, err = s.config.ObterData(ctx, configuracao.UltimaSincronizacaoGeral); err != nil {
		return InfoSistemaDto{}, err
	}
	return info, nil
}

// normalizar é o Normalizar do C#: o texto gravado como 1/0 (booleano) ou como inteiro limitado à faixa; false se
// não se lê.
func normalizar(d Definicao, texto string) (int, bool) {
	if d.Tipo == Booleano {
		b, ok := configuracao.LerBool(texto)
		if !ok {
			return 0, false
		}
		if b {
			return 1, true
		}
		return 0, true
	}
	n, ok := configuracao.LerInteiro(texto)
	if !ok {
		return 0, false
	}
	return limitar(n, d.Minimo, d.Maximo), true
}

// descricaoDoSistema é o RuntimeInformation.OSDescription: "Microsoft Windows 10.0.26200" ou o "uname" no Linux.
func descricaoDoSistema() string {
	if d := strings.TrimSpace(descricaoNativa()); d != "" {
		return d
	}
	return runtime.GOOS
}
