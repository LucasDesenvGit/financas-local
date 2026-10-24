// Pacote cambio é o câmbio (ADR 0007, #81): a PTAX do Banco Central guardada no banco para funcionar sem internet,
// o conversor para reais e as rotas de /api/cambio. Porte do CambioService, do ConversorMoeda, do CambioController e
// do CambioHostedService do .NET, com o mesmo comportamento (o contrato, casos/02-cambio.json, confere).
package cambio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// MoedaBase é o real: as outras moedas são convertidas para ele.
const MoedaBase = "BRL"

// Chaves da Configuracao, as mesmas do CambioService.
const (
	chaveMoedas            = "Cambio.Moedas"
	chaveMoedasEm          = "Cambio.MoedasConsultadasEm"
	prefixoUltimaConsulta  = "Cambio.UltimaConsulta."
	ChaveUltimaAtualizacao = "Cambio.UltimaAtualizacao"
)

const (
	// diasRetroativos: quantos dias para trás procurar o último dia útil (fim de semana + feriado prolongado).
	diasRetroativos = 7
	// validadeDasMoedas: a lista de moedas é consultada no máximo uma vez por semana.
	validadeDasMoedas = 7 * 24 * time.Hour
	// intervaloDeConsultaHoje: hoje sem boletim (antes das ~13h, fim de semana, feriado), a API só é consultada de novo
	// depois deste tempo, para não bater nela a cada chamada.
	intervaloDeConsultaHoje = 30 * time.Minute
	// pausaEntreMoedas: gentileza com a API pública, como o Task.Delay(200) do .NET.
	pausaEntreMoedas = 200 * time.Millisecond
)

var formatoMoeda = regexp.MustCompile(`^[A-Z]{3}$`)

// NormalizarMoeda valida um código ISO 4217, como o CambioService.NormalizarMoeda: " usd" vira "USD"; o que não tem
// 3 letras de A a Z devolve false.
func NormalizarMoeda(moeda string) (string, bool) {
	m := strings.ToUpper(strings.TrimSpace(moeda))
	return m, formatoMoeda.MatchString(m)
}

// Fontes, como aparecem no JSON.
const (
	fontePTAX = "PTAX/Banco Central"
	fonteFixa = "Fixa"
)

// Cotacao é o CotacaoMoedaDto: Data é o dia útil da cotação, que pode ser anterior a DataSolicitada.
type Cotacao struct {
	Moeda          string        `json:"moeda"`
	DataSolicitada tipos.Data    `json:"dataSolicitada"`
	Data           tipos.Data    `json:"data"`
	Compra         tipos.Decimal `json:"compra"`
	Venda          tipos.Decimal `json:"venda"`
	Fonte          string        `json:"fonte"`
}

// CotacoesAtuais é o CotacoesAtuaisDto: a última cotação guardada de cada moeda.
type CotacoesAtuais struct {
	AtualizadoEm *configuracao.Instante `json:"atualizadoEm"`
	Cotacoes     []Cotacao              `json:"cotacoes"`
}

// Guardada é uma linha de CotacaoMoeda.
type Guardada struct {
	Moeda  string
	Data   tipos.Data
	Compra tipos.Decimal
	Venda  tipos.Decimal
}

// ResultadoAtualizacao é o ResultadoAtualizacaoCambio do .NET.
type ResultadoAtualizacao struct {
	Novas, Corrigidas int
	Falhas            []string
}

// fontePTAXInterface é o que o serviço usa do cliente (a interface fica com quem usa, para os testes trocarem).
type fonteDeCotacoes interface {
	ListarMoedas(ctx context.Context) ([]Moeda, error)
	BuscarFechamentos(ctx context.Context, moeda string, de, ate tipos.Data) ([]Fechamento, error)
}

// Servico é o CambioService.
type Servico struct {
	db      *sql.DB
	config  *configuracao.Servico
	fonte   fonteDeCotacoes
	relogio relogio.Relogio
	log     *slog.Logger
	esperar func(ctx context.Context, d time.Duration) error // a pausa entre moedas; nos testes, nada
}

// NovoServico cria o serviço. fonte é o cliente da PTAX (NovoClientePTAX).
func NovoServico(db *sql.DB, fonte fonteDeCotacoes, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, config: configuracao.Novo(db, r), fonte: fonte, relogio: r, log: log, esperar: esperarComContexto}
}

func esperarComContexto(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (s *Servico) avisar(msg string, args ...any) {
	s.log.Warn(msg, append([]any{"origem", "Câmbio"}, args...)...)
}

// ListarMoedas é o ListarMoedasAsync: a lista guardada, se tiver menos de 7 dias; senão, a da API (gravada de novo).
// Sem internet, a guardada, mesmo velha (ou vazia).
func (s *Servico) ListarMoedas(ctx context.Context) ([]Moeda, error) {
	emCache, err := s.moedasGuardadas(ctx)
	if err != nil {
		return nil, err
	}
	consultadaEm, err := s.config.ObterData(ctx, chaveMoedasEm)
	if err != nil {
		return nil, err
	}
	agora := s.relogio.Agora()
	if len(emCache) > 0 && consultadaEm != nil && agora.Sub(consultadaEm.Tempo()) < validadeDasMoedas {
		return emCache, nil
	}

	moedas, err := s.fonte.ListarMoedas(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s.avisar("Não foi possível consultar a lista de moedas no Banco Central.", "erro", err)
		return emCache, nil
	}
	if len(moedas) == 0 {
		return emCache, nil
	}
	texto, err := jsonDasMoedas(moedas)
	if err != nil {
		return nil, err
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		config := s.config.ComTx(tx)
		if err := config.Definir(ctx, chaveMoedas, &texto); err != nil {
			return err
		}
		return config.DefinirData(ctx, chaveMoedasEm, agora)
	})
	if err != nil {
		// No .NET, a gravação está no mesmo try da consulta: o erro vira aviso, e segue a lista guardada.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s.avisar("Não foi possível consultar a lista de moedas no Banco Central.", "erro", err)
		return emCache, nil
	}
	return moedas, nil
}

// moedasGuardadas lê a lista guardada; texto que não é a lista vale lista vazia, como o Desserializar do .NET.
func (s *Servico) moedasGuardadas(ctx context.Context) ([]Moeda, error) {
	texto, err := s.config.Obter(ctx, chaveMoedas)
	if err != nil {
		return nil, err
	}
	moedas := []Moeda{}
	if texto == nil || strings.TrimSpace(*texto) == "" {
		return moedas, nil
	}
	// O .NET grava com o JsonSerializer padrão: chaves "Codigo" e "Nome", com maiúscula.
	var guardadas []struct{ Codigo, Nome string }
	if err := json.Unmarshal([]byte(*texto), &guardadas); err != nil {
		return moedas, nil
	}
	for _, m := range guardadas {
		moedas = append(moedas, Moeda(m))
	}
	return moedas, nil
}

// jsonDasMoedas escreve a lista como o JsonSerializer.Serialize(moedas) do .NET: "Codigo" e "Nome" com maiúscula e
// o que não é ASCII escapado (o "ó" de "Dólar" vira a barra, a letra u e 00F3), para o .NET ler e o texto no banco ser o mesmo.
func jsonDasMoedas(moedas []Moeda) (string, error) {
	type comoNoDotnet struct{ Codigo, Nome string }
	lista := make([]comoNoDotnet, len(moedas))
	for i, m := range moedas {
		lista[i] = comoNoDotnet(m)
	}
	corpo, err := json.Marshal(lista)
	if err != nil {
		return "", fmt.Errorf("montar a lista de moedas: %w", err)
	}
	return servidor.EscaparComoDotnet(string(corpo)), nil
}

func ordenarMoedas(moedas []Moeda) {
	slices.SortStableFunc(moedas, func(a, b Moeda) int { return strings.Compare(a.Codigo, b.Codigo) })
}
