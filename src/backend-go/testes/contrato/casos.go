// Pacote contrato tem os testes de contrato da migração (#72, ADR 0049): o que o back-end .NET responde hoje fica
// gravado em esperado/*.json (golden files), e o mesmo roteiro de casos/*.json roda contra o Go para conferir que
// ele responde igual e deixa o banco igual. Como usar: README.md desta pasta.
package contrato

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ArquivoCasos é um casos/NN-assunto.json: os casos de um assunto, rodados em ordem.
type ArquivoCasos struct {
	Descricao string `json:"descricao"`
	// Sessoes dá opções às sessões (navegadores) usadas no arquivo. Sessão sem opção nenhuma não precisa estar aqui.
	Sessoes map[string]OpcoesSessao `json:"sessoes"`
	Casos   []Caso                  `json:"casos"`
}

// OpcoesSessao configura uma sessão.
type OpcoesSessao struct {
	// Origem é o IP de onde a sessão se conecta (127.0.0.x). O limite de tentativas de login é por IP
	// (Program.cs), então uma sessão em outro IP tem a própria cota.
	Origem string `json:"origem"`
}

// Caso é uma requisição do roteiro.
type Caso struct {
	Nome       string `json:"nome"`
	Comentario string `json:"comentario,omitempty"`
	Metodo     string `json:"metodo"`
	Caminho    string `json:"caminho"`
	// Sessao é o nome do navegador que faz a requisição: cada sessão tem os próprios cookies. Vazio é "principal",
	// a que fica logada para os assuntos seguintes.
	Sessao string `json:"sessao,omitempty"`
	// NovaSessao apaga os cookies da sessão antes da requisição (navegador recém-aberto).
	NovaSessao bool `json:"novaSessao,omitempty"`
	// Corpo é enviado como JSON. CorpoTexto é enviado como está (para JSON quebrado). No máximo um dos dois.
	Corpo      json.RawMessage `json:"corpo,omitempty"`
	CorpoTexto *string         `json:"corpoTexto,omitempty"`
	// SemCsrf não manda o cabeçalho X-XSRF-TOKEN (por padrão, todo método que não é GET manda).
	SemCsrf bool `json:"semCsrf,omitempty"`
	// Capturar guarda valores da resposta em variáveis, usadas nos casos seguintes como {{nome}} no caminho e no
	// corpo. A chave é o nome da variável; o valor, um JSON Pointer (RFC 6901) no corpo da resposta: "/itens/0/id".
	Capturar map[string]string `json:"capturar,omitempty"`
	// Normalizar troca campos do corpo da resposta que mudam a cada execução por um marcador. A chave é um JSON
	// Pointer; o valor, o tipo esperado: "texto", "numero" ou "datahora". Só para o que não é regra de negócio
	// (versão do runtime, hora em que o processo subiu): dinheiro e datas de negócio nunca.
	Normalizar map[string]string `json:"normalizar,omitempty"`
}

var nomeVariavel = regexp.MustCompile(`\{\{([A-Za-z0-9_.-]+)\}\}`)

// lerCasos lê todos os casos/*.json, em ordem de nome.
func lerCasos(pasta string) ([]string, map[string]ArquivoCasos, error) {
	arquivos, err := filepath.Glob(filepath.Join(pasta, "*.json"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(arquivos)
	nomes := make([]string, 0, len(arquivos))
	todos := map[string]ArquivoCasos{}
	for _, caminho := range arquivos {
		dados, err := os.ReadFile(caminho)
		if err != nil {
			return nil, nil, err
		}
		var a ArquivoCasos
		dec := json.NewDecoder(bytes.NewReader(dados))
		dec.DisallowUnknownFields() // campo com erro de digitação não passa em silêncio
		if err := dec.Decode(&a); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", caminho, err)
		}
		if err := validarArquivo(a); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", caminho, err)
		}
		nome := strings.TrimSuffix(filepath.Base(caminho), ".json")
		nomes = append(nomes, nome)
		todos[nome] = a
	}
	return nomes, todos, nil
}

func validarArquivo(a ArquivoCasos) error {
	vistos := map[string]bool{}
	for i, c := range a.Casos {
		switch {
		case c.Nome == "":
			return fmt.Errorf("caso %d sem nome", i+1)
		case vistos[c.Nome]:
			return fmt.Errorf("nome de caso repetido: %q", c.Nome)
		case c.Metodo == "" || c.Caminho == "":
			return fmt.Errorf("%q: metodo e caminho são obrigatórios", c.Nome)
		case !strings.HasPrefix(c.Caminho, "/"):
			return fmt.Errorf("%q: o caminho começa com /", c.Nome)
		case len(c.Corpo) > 0 && c.CorpoTexto != nil:
			return fmt.Errorf("%q: use corpo ou corpoTexto, não os dois", c.Nome)
		}
		for ponteiro, tipo := range c.Normalizar {
			if tipo != "texto" && tipo != "numero" && tipo != "datahora" {
				return fmt.Errorf("%q: normalizar %s: tipo %q desconhecido (texto, numero, datahora)", c.Nome, ponteiro, tipo)
			}
		}
		vistos[c.Nome] = true
	}
	return nil
}

// substituir troca {{nome}} pelo valor capturado num caso anterior.
func substituir(texto string, variaveis map[string]string) (string, error) {
	var faltando []string
	resultado := nomeVariavel.ReplaceAllStringFunc(texto, func(m string) string {
		nome := nomeVariavel.FindStringSubmatch(m)[1]
		valor, ok := variaveis[nome]
		if !ok {
			faltando = append(faltando, nome)
			return m
		}
		return valor
	})
	if len(faltando) > 0 {
		return "", fmt.Errorf("variável não capturada antes: %s", strings.Join(faltando, ", "))
	}
	return resultado, nil
}
