package contrato

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// InicioPadrao é o instante em que o relógio do servidor sob teste começa (faketime no .NET). Em Brasília ainda é
// 14/10 (22h): a data de hoje em Brasília e a data em UTC são diferentes, e quem usar a data UTC como "hoje" erra.
const InicioPadrao = "2026-10-15T01:00:00Z"

var (
	alvo      = flag.String("alvo", "", "endereço do servidor sob teste (ex.: https://127.0.0.1:5451); vazio pula o contrato")
	banco     = flag.String("banco", "", "arquivo SQLite que o servidor sob teste usa (para comparar o banco)")
	inicio    = flag.String("inicio", InicioPadrao, "instante em que o relógio do servidor começou (RFC 3339)")
	atualizar = flag.Bool("atualizar", false, "grava os esperados a partir do servidor (só contra o .NET: ele é a referência)")
	// #75: o Go sozinho (com o login dele) atende só os assuntos já portados; os outros arquivos ficam de fora.
	somente = flag.String("somente", "", "arquivos de casos a rodar, separados por vírgula (ex.: 00-auth); vazio roda todos")
)

// Resultado é o que um caso produziu, já normalizado: é o que fica em esperado/*.json.
type Resultado struct {
	Nome       string                      `json:"nome"`
	Requisicao string                      `json:"requisicao"`
	Status     int                         `json:"status"`
	Tipo       string                      `json:"tipo,omitempty"`
	Cookies    []string                    `json:"cookies,omitempty"`
	Corpo      any                         `json:"corpo,omitempty"`
	Texto      *string                     `json:"texto,omitempty"`
	Banco      map[string]*DiferencaTabela `json:"banco,omitempty"`
}

// TestContrato roda casos/*.json em ordem contra -alvo e compara cada resposta e cada mudança no banco com
// esperado/*.json. Com -atualizar, grava os esperados. Uso: README.md (ou rodar-dotnet.sh, que monta tudo).
func TestContrato(t *testing.T) {
	if *alvo == "" {
		t.Skip("sem -alvo: o contrato roda contra um servidor de pé (veja testes/contrato/README.md)")
	}
	if *banco == "" {
		t.Fatal("-banco é obrigatório com -alvo: o contrato também compara o banco")
	}
	enderecoAlvo, err := url.Parse(*alvo)
	if err != nil || enderecoAlvo.Scheme != "https" || enderecoAlvo.Host == "" {
		t.Fatalf("-alvo %q: use https://host:porta", *alvo)
	}
	instanteInicio, err := time.Parse(time.RFC3339, *inicio)
	if err != nil {
		t.Fatalf("-inicio: %v", err)
	}
	caminhoBanco, err := filepath.Abs(*banco)
	if err != nil {
		t.Fatal(err)
	}
	versao, err := lerVersao(filepath.Join("..", "..", "..", "..", "Directory.Build.props"))
	if err != nil {
		t.Fatal(err)
	}
	n := Normalizador{Inicio: instanteInicio, Versao: versao, PastaDados: filepath.Dir(filepath.Dir(caminhoBanco))}

	nomes, arquivos, err := lerCasos("casos")
	if err != nil {
		t.Fatal(err)
	}
	db, err := abrirSoLeitura(caminhoBanco)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ex := &execucao{alvo: enderecoAlvo, n: n, db: db, sessoes: map[string]*Sessao{}, variaveis: map[string]string{}}
	escolhidos := map[string]bool{}
	for _, nome := range strings.Split(*somente, ",") {
		if nome = strings.TrimSpace(nome); nome != "" {
			escolhidos[nome] = true
		}
	}
	for _, nome := range nomes {
		if len(escolhidos) > 0 && !escolhidos[nome] {
			continue
		}
		arquivo := arquivos[nome]
		t.Run(nome, func(t *testing.T) { ex.rodarArquivo(t, nome, arquivo) })
	}
}

type execucao struct {
	alvo      *url.URL
	n         Normalizador
	db        *sql.DB
	sessoes   map[string]*Sessao
	variaveis map[string]string
}

func (ex *execucao) rodarArquivo(t *testing.T, nome string, arquivo ArquivoCasos) {
	caminhoEsperado := filepath.Join("esperado", nome+".json")
	esperados := map[string]any{}
	if !*atualizar {
		dados, err := os.ReadFile(caminhoEsperado)
		if err != nil {
			t.Fatalf("sem %s: grave os esperados contra o .NET (rodar-dotnet.sh --atualizar)", caminhoEsperado)
		}
		lista, err := decodificar(dados)
		if err != nil {
			t.Fatalf("%s: %v", caminhoEsperado, err)
		}
		itens, _ := lista.([]any)
		for _, item := range itens {
			if m, ok := item.(map[string]any); ok {
				if nomeCaso, ok := m["nome"].(string); ok {
					esperados[nomeCaso] = m
				}
			}
		}
		for nomeCaso := range esperados {
			if !temCaso(arquivo, nomeCaso) {
				t.Errorf("%s tem o caso %q, que não está mais em casos/%s.json: regrave os esperados", caminhoEsperado, nomeCaso, nome)
			}
		}
	}

	var resultados []Resultado
	for _, caso := range arquivo.Casos {
		t.Run(caso.Nome, func(t *testing.T) {
			r, err := ex.rodarCaso(t.Context(), arquivo, caso)
			if err != nil {
				t.Fatal(err)
			}
			resultados = append(resultados, *r)
			if *atualizar {
				return
			}
			esperado, ok := esperados[caso.Nome]
			if !ok {
				t.Fatalf("o caso não tem esperado em %s: grave contra o .NET (rodar-dotnet.sh --atualizar)", caminhoEsperado)
			}
			obtido, err := paraAny(r)
			if err != nil {
				t.Fatal(err)
			}
			if difs := comparar("", esperado, obtido); len(difs) > 0 {
				if len(difs) > 30 {
					difs = append(difs[:30], fmt.Sprintf("... e mais %d diferenças", len(difs)-30))
				}
				t.Errorf("%s %s: diferente do esperado:\n  %s", caso.Metodo, caso.Caminho, strings.Join(difs, "\n  "))
			}
		})
	}

	if *atualizar && !t.Failed() {
		dados, err := codificar(resultados)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("esperado", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminhoEsperado, dados, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("gravado %s (%d casos)", caminhoEsperado, len(resultados))
	}
}

func temCaso(a ArquivoCasos, nome string) bool {
	for _, c := range a.Casos {
		if c.Nome == nome {
			return true
		}
	}
	return false
}

func (ex *execucao) sessao(arquivo ArquivoCasos, nome string) (*Sessao, error) {
	if nome == "" {
		nome = "principal"
	}
	if s, ok := ex.sessoes[nome]; ok {
		return s, nil
	}
	s, err := novaSessao(ex.alvo, arquivo.Sessoes[nome].Origem)
	if err != nil {
		return nil, err
	}
	ex.sessoes[nome] = s
	return s, nil
}

func (ex *execucao) rodarCaso(ctx context.Context, arquivo ArquivoCasos, caso Caso) (*Resultado, error) {
	s, err := ex.sessao(arquivo, caso.Sessao)
	if err != nil {
		return nil, err
	}
	if caso.NovaSessao {
		if err := s.limpar(); err != nil {
			return nil, err
		}
	}
	caminho, err := substituir(caso.Caminho, ex.variaveis)
	if err != nil {
		return nil, err
	}
	var corpo []byte
	tipoCorpo := ""
	switch {
	case len(caso.Corpo) > 0:
		texto, err := substituir(string(caso.Corpo), ex.variaveis)
		if err != nil {
			return nil, err
		}
		corpo, tipoCorpo = []byte(texto), "application/json"
	case caso.CorpoTexto != nil:
		corpo, tipoCorpo = []byte(*caso.CorpoTexto), "application/json"
	}
	csrf := ""
	if m := caso.Metodo; m != http.MethodGet && m != http.MethodHead && !caso.SemCsrf {
		if csrf, err = s.tokenCsrf(ctx); err != nil {
			return nil, err
		}
	}

	antes, err := fotografar(ctx, ex.db)
	if err != nil {
		return nil, fmt.Errorf("ler o banco antes: %w", err)
	}
	resp, err := s.enviar(ctx, caso.Metodo, caminho, corpo, tipoCorpo, csrf)
	if err != nil {
		return nil, err
	}
	depois, err := fotografar(ctx, ex.db)
	if err != nil {
		return nil, fmt.Errorf("ler o banco depois: %w", err)
	}

	r := &Resultado{
		Nome:       caso.Nome,
		Requisicao: caso.Metodo + " " + caso.Caminho,
		Status:     resp.Status,
		Tipo:       resp.Tipo,
		Cookies:    resp.Cookies,
		Banco:      ex.n.diferencas(antes, depois),
	}
	if len(resp.Corpo) > 0 {
		if ehJSON(resp.Tipo) {
			// Captura do corpo cru, antes de normalizar (o valor de verdade, para usar no caso seguinte).
			cru, err := decodificar(resp.Corpo)
			if err != nil {
				return nil, fmt.Errorf("corpo com Content-Type JSON que não é JSON: %w", err)
			}
			if err := ex.capturar(caso, cru); err != nil {
				return nil, err
			}
			normalizado, _ := decodificar(resp.Corpo)
			if r.Corpo, err = ex.n.corpo(normalizado, caso.Normalizar); err != nil {
				return nil, err
			}
		} else {
			texto := string(resp.Corpo)
			r.Texto = &texto
		}
	}
	if len(caso.Capturar) > 0 && r.Corpo == nil {
		return nil, errors.New("o caso pede capturar, mas a resposta não tem corpo JSON")
	}
	if len(caso.Normalizar) > 0 && r.Corpo == nil {
		return nil, errors.New("o caso pede normalizar, mas a resposta não tem corpo JSON")
	}
	return r, nil
}

func (ex *execucao) capturar(caso Caso, corpo any) error {
	for nome, p := range caso.Capturar {
		v, _, err := ponteiro(corpo, p)
		if err != nil {
			return fmt.Errorf("capturar %s: %w", nome, err)
		}
		texto, err := comoTexto(v)
		if err != nil {
			return fmt.Errorf("capturar %s: %w", nome, err)
		}
		ex.variaveis[nome] = texto
	}
	return nil
}

var versaoNoProps = regexp.MustCompile(`<Version>([^<]+)</Version>`)

// lerVersao lê a versão do sistema no Directory.Build.props (a mesma que o .NET e o Go devolvem).
func lerVersao(caminho string) (string, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return "", fmt.Errorf("ler a versão: %w", err)
	}
	m := versaoNoProps.FindSubmatch(dados)
	if m == nil {
		return "", fmt.Errorf("%s sem <Version>", caminho)
	}
	return strings.TrimSpace(string(m[1])), nil
}
