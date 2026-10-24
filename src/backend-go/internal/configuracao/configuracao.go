// Pacote configuracao é o ConfiguracaoService do .NET: lê e grava a tabela Configuracao (chave e valor em texto).
//
// Segredo (Protegido = 1) não passa pelo Obter (devolve ErrProtegido): é lido e gravado por ObterSegredo e
// DefinirSegredo, com o cofre do pacote segredos (#76). Até a inversão (#85) nenhuma rota do Go usa os dois.
package configuracao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Chaves usadas pelo Go (as constantes do ConfiguracaoService).
const (
	UltimaSincronizacaoGeral = "Sincronizacao.UltimaExecucao"
)

// ErrProtegido é a leitura de um segredo por Obter: quem lê segredo usa o pacote segredos.
var ErrProtegido = errors.New("a configuração é um segredo cifrado: leia pelo pacote segredos")

// Servico lê e grava a Configuracao. Para gravar junto com outras tabelas, numa transação, use ComTx.
type Servico struct {
	q       *consultas.Queries
	relogio relogio.Relogio
}

func Novo(db consultas.DBTX, r relogio.Relogio) *Servico {
	return &Servico{q: consultas.New(db), relogio: r}
}

// ComTx devolve o mesmo serviço gravando na transação tx (o SaveChanges único do .NET).
func (s *Servico) ComTx(tx *sql.Tx) *Servico {
	return &Servico{q: s.q.WithTx(tx), relogio: s.relogio}
}

// Obter é o ObterAsync: o valor, ou nil se a chave não existe ou está sem valor.
func (s *Servico) Obter(ctx context.Context, chave string) (*string, error) {
	c, err := s.q.ObterConfiguracao(ctx, chave)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("ler a configuração %s: %w", chave, err)
	case c.Valor == nil:
		return nil, nil
	case c.Protegido:
		return nil, fmt.Errorf("%s: %w", chave, ErrProtegido)
	}
	return c.Valor, nil
}

// Existe é o ExisteAsync: a chave existe e tem valor (vale também para segredo).
func (s *Servico) Existe(ctx context.Context, chave string) (bool, error) {
	existe, err := s.q.ExisteConfiguracao(ctx, chave)
	if err != nil {
		return false, fmt.Errorf("conferir a configuração %s: %w", chave, err)
	}
	return existe, nil
}

// ObterInteiro é o ObterInteiroAsync: o valor como inteiro (int.TryParse), ou o padrão.
func (s *Servico) ObterInteiro(ctx context.Context, chave string, padrao int) (int, error) {
	valor, err := s.Obter(ctx, chave)
	if err != nil || valor == nil {
		return padrao, err
	}
	if n, ok := LerInteiro(*valor); ok {
		return n, nil
	}
	return padrao, nil
}

// ObterBool é o ObterBoolAsync: o valor como booleano (bool.TryParse), ou o padrão.
func (s *Servico) ObterBool(ctx context.Context, chave string, padrao bool) (bool, error) {
	valor, err := s.Obter(ctx, chave)
	if err != nil || valor == nil {
		return padrao, err
	}
	if b, ok := LerBool(*valor); ok {
		return b, nil
	}
	return padrao, nil
}

// ObterData é o ObterDataAsync: o valor como data e hora (DateTime.TryParse com RoundtripKind), ou nil.
func (s *Servico) ObterData(ctx context.Context, chave string) (*Instante, error) {
	valor, err := s.Obter(ctx, chave)
	if err != nil || valor == nil {
		return nil, err
	}
	if i, ok := LerInstante(*valor); ok {
		return &i, nil
	}
	return nil, nil
}

// Definir é o DefinirAsync para valor que não é segredo: cria a chave ou troca o valor, e sempre regrava
// AtualizadoEm (em UTC), mesmo com o mesmo valor.
func (s *Servico) Definir(ctx context.Context, chave string, valor *string) error {
	err := s.q.DefinirConfiguracao(ctx, consultas.DefinirConfiguracaoParams{
		Chave: chave, Valor: valor, Protegido: false, AtualizadoEm: tipos.NovaDataHora(s.relogio.Agora()),
	})
	if err != nil {
		return fmt.Errorf("gravar a configuração %s: %w", chave, err)
	}
	return nil
}

// Cofre é o que o pacote segredos faz (declarado aqui, por quem usa).
type Cofre interface {
	Cifrar(chave, segredo string) (string, error)
	Decifrar(chave, gravado string) (string, error)
}

// ObterSegredo é o ObterAsync de um valor protegido: nil se não há valor. O valor antigo, do Data Protection, volta
// como o erro do cofre (segredos.ErrInformarDeNovo) e a linha fica como está.
func (s *Servico) ObterSegredo(ctx context.Context, cofre Cofre, chave string) (*string, error) {
	c, err := s.q.ObterConfiguracao(ctx, chave)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("ler a configuração %s: %w", chave, err)
	case c.Valor == nil:
		return nil, nil
	case !c.Protegido:
		return c.Valor, nil // como o .NET: sem a marca, o valor é texto puro
	}
	segredo, err := cofre.Decifrar(chave, *c.Valor)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", chave, err)
	}
	return &segredo, nil
}

// DefinirSegredo é o DefinirAsync(chave, valor, protegido: true): grava o valor cifrado pelo cofre, com
// Protegido = 1. nil apaga o valor (fica a linha, sem valor), como no .NET.
func (s *Servico) DefinirSegredo(ctx context.Context, cofre Cofre, chave string, segredo *string) error {
	var valor *string
	if segredo != nil {
		cifrado, err := cofre.Cifrar(chave, *segredo)
		if err != nil {
			return err
		}
		valor = &cifrado
	}
	err := s.q.DefinirConfiguracao(ctx, consultas.DefinirConfiguracaoParams{
		Chave: chave, Valor: valor, Protegido: true, AtualizadoEm: tipos.NovaDataHora(s.relogio.Agora()),
	})
	if err != nil {
		return fmt.Errorf("gravar a configuração %s: %w", chave, err)
	}
	return nil
}

// LerInteiro é o int.TryParse do C# (NumberStyles.Integer): espaços antes e depois, sinal + ou -, só dígitos ASCII,
// dentro de um int de 32 bits. Os "\0" do fim são ignorados, como no .NET.
func LerInteiro(texto string) (int, bool) {
	texto = strings.TrimRight(texto, "\x00")
	texto = strings.Trim(texto, " \t\n\v\f\r")
	digitos := strings.TrimPrefix(strings.TrimPrefix(texto, "+"), "-")
	if digitos == "" || len(texto)-len(digitos) > 1 {
		return 0, false
	}
	for _, c := range digitos {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(texto, "+"), 10, 32)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

// LerBool é o bool.TryParse do C#: "true" ou "false" sem diferença de maiúsculas, com espaços em volta (e "\0" no
// fim) ignorados.
func LerBool(texto string) (bool, bool) {
	texto = strings.TrimLeftFunc(texto, unicode.IsSpace)
	texto = strings.TrimRightFunc(texto, func(r rune) bool { return unicode.IsSpace(r) || r == 0 })
	switch {
	case strings.EqualFold(texto, "true"):
		return true, true
	case strings.EqualFold(texto, "false"):
		return false, true
	}
	return false, false
}

// Instante é o DateTime lido com RoundtripKind: o Kind vem do texto ("Z" é UTC; com deslocamento, a hora local do
// computador; sem nada, sem fuso), e o JSON sai como o System.Text.Json escreve cada Kind.
type Instante struct {
	t    time.Time
	tipo tipoInstante
}

type tipoInstante int

const (
	semFuso tipoInstante = iota
	utc
	local
)

// Formatos ISO 8601 que o DateTime.TryParse aceita e que o .NET grava (ToString("O") e o do EF Core). Outros textos
// que o .NET lê pela cultura (como "14/10/2026 09:00") não são lidos aqui: viram nil, como texto inválido.
var formatosInstante = []string{
	"2006-01-02T15:04:05.9999999", "2006-01-02 15:04:05.9999999", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02",
}

// LerInstante lê o texto como o DateTime.TryParse com RoundtripKind, nos formatos ISO 8601.
func LerInstante(texto string) (Instante, bool) {
	texto = strings.TrimSpace(texto)
	tipo, fuso := semFuso, time.UTC
	switch {
	case strings.HasSuffix(texto, "Z"):
		texto, tipo = strings.TrimSuffix(texto, "Z"), utc
	case len(texto) > 6 && (texto[len(texto)-6] == '+' || texto[len(texto)-6] == '-') && texto[len(texto)-3] == ':':
		deslocamento, err := time.Parse("-07:00", texto[len(texto)-6:])
		if err != nil {
			return Instante{}, false
		}
		_, segundos := deslocamento.Zone()
		texto, tipo, fuso = texto[:len(texto)-6], local, time.FixedZone("", segundos)
	}
	for _, formato := range formatosInstante {
		if t, err := time.ParseInLocation(formato, texto, fuso); err == nil {
			if tipo == local {
				t = t.Local()
			}
			return Instante{t: t.Truncate(100 * time.Nanosecond), tipo: tipo}, true
		}
	}
	return Instante{}, false
}

// InstanteUTC é o DateTime com Kind Utc (o DateTime.UtcNow, ou o StartTime.ToUniversalTime()).
func InstanteUTC(t time.Time) Instante {
	return Instante{t: t.UTC().Truncate(100 * time.Nanosecond), tipo: utc}
}

// MarshalJSON escreve como o System.Text.Json: até 7 casas, sem os zeros do fim, e "Z" (UTC), o deslocamento (hora
// local) ou nada (sem fuso).
func (i Instante) MarshalJSON() ([]byte, error) {
	texto := i.t.Format("2006-01-02T15:04:05.9999999")
	switch i.tipo {
	case utc:
		texto += "Z"
	case local:
		texto += i.t.Format("-07:00")
	}
	return []byte(`"` + texto + `"`), nil
}
