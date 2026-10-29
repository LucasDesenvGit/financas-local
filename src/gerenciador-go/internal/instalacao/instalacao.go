// Package instalacao acha onde está o site, como o Instalacao.cs: a pasta da API (pelo serviço do sistema), o banco,
// a pasta de dados e o endereço. Lê os mesmos appsettings da API, para não ter uma segunda configuração.
//   - .msi (Windows): Dados:Pasta (ex.: %ProgramData%\FinancasLocal\dados) define o banco; atualiza pelo GitHub.
//   - scripts: Banco:Caminho define o banco; atualiza pelos scripts, então o gerenciador não oferece o .msi.
package instalacao

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const pastaDadosPadrao = `%ProgramData%\FinancasLocal\dados`

// Instalacao é onde está o site.
type Instalacao struct {
	// PastaPrograma é a pasta da API (a do serviço; sem o serviço, a do próprio gerenciador).
	PastaPrograma string
	PastaDados    string
	CaminhoBanco  string
	// PeloMsi: instalado pelo .msi; só então o gerenciador procura e instala versões novas do GitHub.
	PeloMsi bool
	// Site é o endereço com a barra no fim (https://127.0.0.1:5443/).
	Site string
}

// Pagina é o endereço de uma página do site ("" é o início).
func (i Instalacao) Pagina(caminho string) string { return i.Site + caminho }

// Ler monta a instalação a partir dos appsettings da pasta da API.
func Ler(pastaPrograma string) Instalacao {
	producao := lerJSON(filepath.Join(pastaPrograma, "appsettings.Production.json"))
	geral := lerJSON(filepath.Join(pastaPrograma, "appsettings.json"))

	i := Instalacao{PastaPrograma: pastaPrograma}
	dados, temDados := texto(producao, "Dados", "Pasta")
	banco, temBanco := texto(producao, "Banco", "Caminho")
	if !temBanco {
		banco, temBanco = texto(geral, "Banco", "Caminho")
	}
	i.PeloMsi = temDados
	if !temDados && temBanco {
		// Instalação pelos scripts: o banco fica onde o appsettings diz (relativo à pasta da API, se não for absoluto).
		i.CaminhoBanco = absoluto(pastaPrograma, strings.ReplaceAll(banco, "/", string(filepath.Separator)))
		pastaSQL := filepath.Dir(i.CaminhoBanco)
		i.PastaDados = pastaSQL
		if strings.EqualFold(filepath.Base(pastaSQL), "sql") {
			i.PastaDados = filepath.Dir(pastaSQL)
		}
	} else {
		if !temDados {
			dados = pastaDadosPadrao
		}
		i.PastaDados = absoluto(pastaPrograma, expandir(dados))
		i.CaminhoBanco = filepath.Join(i.PastaDados, "sql", "FinancasLocal.db")
	}

	url, ok := texto(producao, "Kestrel", "Endpoints", "Https", "Url")
	if !ok {
		url, ok = texto(geral, "Kestrel", "Endpoints", "Https", "Url")
	}
	if !ok {
		url = "https://127.0.0.1:5443"
	}
	i.Site = strings.TrimRight(url, "/") + "/"
	return i
}

// executavelDaApi acha a API na linha do serviço: "...\Financas.Api.exe" (.msi) ou "...\dotnet.exe"
// "...\Financas.Api.dll" (scripts).
var executavelDaApi = regexp.MustCompile(`(?i)([A-Za-z]:\\[^"]*?Financas\.Api\.(?:dll|exe))`)

// PastaDaLinhaDoServico é a pasta da API na linha de comando do serviço (ImagePath), ou "".
func PastaDaLinhaDoServico(linha string) string {
	achado := executavelDaApi.FindStringSubmatch(expandir(linha))
	if achado == nil {
		return ""
	}
	// O caminho é do Windows: a pasta vai até a última barra invertida, em qualquer sistema.
	return achado[1][:strings.LastIndex(achado[1], `\`)]
}

// dllDaApi acha a API no ExecStart da unidade do systemd (scripts no Linux): ".../dotnet .../Financas.Api.dll".
var dllDaApi = regexp.MustCompile(`(/\S*?Financas\.Api\.dll)`)

// PastaDaUnidade é a pasta da API pelas propriedades da unidade do systemd --user, como o PastaDaApi do
// ServicoLinux.cs: a WorkingDirectory; sem ela, a pasta do Financas.Api.dll do ExecStart; "" sem a unidade.
func PastaDaUnidade(p map[string]string) string {
	if p["LoadState"] != "loaded" {
		return ""
	}
	if pasta := p["WorkingDirectory"]; strings.HasPrefix(pasta, "/") {
		return pasta
	}
	achado := dllDaApi.FindStringSubmatch(p["ExecStart"])
	if achado == nil {
		return ""
	}
	return path.Dir(achado[1])
}

// absoluto é o Path.GetFullPath(Path.Combine(base, caminho)) do C#: caminho absoluto fica como está.
func absoluto(base, caminho string) string {
	if !filepath.IsAbs(caminho) {
		caminho = filepath.Join(base, caminho)
	}
	return filepath.Clean(caminho)
}

var variavel = regexp.MustCompile(`%([^%]+)%`)

// expandir troca %VARIAVEL% pelo valor, como o Environment.ExpandEnvironmentVariables: a que não existe fica como
// está.
func expandir(texto string) string {
	return variavel.ReplaceAllStringFunc(texto, func(v string) string {
		if valor, ok := os.LookupEnv(v[1 : len(v)-1]); ok {
			return valor
		}
		return v
	})
}

// lerJSON lê o appsettings aceitando comentários e vírgula sobrando, como o JsonDocumentOptions do C#; arquivo que
// falta ou não lê vira nil.
func lerJSON(caminho string) map[string]any {
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return nil
	}
	var raiz map[string]any
	if json.Unmarshal(limparJSON(conteudo), &raiz) != nil {
		return nil
	}
	return raiz
}

// limparJSON tira os comentários (// e /* */) e as vírgulas antes de } ou ], fora dos textos.
func limparJSON(entrada []byte) []byte {
	saida := make([]byte, 0, len(entrada))
	for i := 0; i < len(entrada); i++ {
		c := entrada[i]
		switch {
		case c == '"':
			fim := fimDoTexto(entrada, i)
			saida = append(saida, entrada[i:fim]...)
			i = fim - 1
		case c == '/' && i+1 < len(entrada) && entrada[i+1] == '/':
			for i < len(entrada) && entrada[i] != '\n' {
				i++
			}
			i--
		case c == '/' && i+1 < len(entrada) && entrada[i+1] == '*':
			fim := strings.Index(string(entrada[i+2:]), "*/")
			if fim < 0 {
				return saida
			}
			i += fim + 3
		case c == '}' || c == ']':
			// A vírgula sobrando é a última coisa (fora espaço) antes do fechamento.
			antes := bytes.TrimRight(saida, " \t\r\n")
			if n := len(antes); n > 0 && antes[n-1] == ',' {
				saida = append(saida[:n-1], saida[n:]...)
			}
			saida = append(saida, c)
		default:
			saida = append(saida, c)
		}
	}
	return saida
}

// fimDoTexto é a posição logo depois das aspas que fecham o texto que começa em inicio.
func fimDoTexto(entrada []byte, inicio int) int {
	for i := inicio + 1; i < len(entrada); i++ {
		switch entrada[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(entrada)
}

// texto é o valor de texto no caminho dado, se existir.
func texto(raiz map[string]any, caminho ...string) (string, bool) {
	var atual any = raiz
	for _, parte := range caminho {
		objeto, ok := atual.(map[string]any)
		if !ok {
			return "", false
		}
		if atual, ok = objeto[parte]; !ok {
			return "", false
		}
	}
	s, ok := atual.(string)
	return s, ok
}
