package atualizacao

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repositorio de onde vêm as versões novas (Releases públicos; nada do usuário é enviado).
const Repositorio = "LucasDesenvGit/financas-local"

// Disponivel é uma versão no GitHub com instalador (.msi) e, de preferência, as somas (SHA256SUMS.txt).
type Disponivel struct {
	Versao  Versao
	Tag     string
	Notas   string
	Msi     string
	NomeMsi string
	Somas   string // "" quando a versão não publicou o SHA256SUMS.txt
	Pagina  string
}

// Cliente procura e baixa no GitHub. Base é a API do repositório (trocada nos testes).
type Cliente struct {
	HTTP   *http.Client
	Base   string
	Versao string // a do gerenciador, no User-Agent (o GitHub exige um)
	// Pasta onde o .msi é baixado (a temporária do sistema, em FinancasLocal).
	Pasta string
}

// NovoCliente é o cliente do GitHub de verdade.
func NovoCliente(versao string) Cliente {
	return Cliente{HTTP: &http.Client{Timeout: 15 * time.Minute}, Base: "https://api.github.com/repos/" + Repositorio + "/",
		Versao: versao, Pasta: filepath.Join(os.TempDir(), "FinancasLocal")}
}

// Procurar devolve a última versão publicada (ou, com desenvolvimento, a última de desenvolvimento), ou nil se não
// houver nenhuma com instalador.
func (c Cliente) Procurar(ctx context.Context, desenvolvimento bool) (*Disponivel, error) {
	endereco := c.Base + "releases/latest"
	if desenvolvimento {
		endereco = c.Base + "releases/tags/desenvolvimento"
	}
	resposta, err := c.pedir(ctx, endereco, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resposta.Body.Close()
	if resposta.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err := sucesso(resposta); err != nil {
		return nil, err
	}
	var release struct {
		Tag    string `json:"tag_name"`
		Nome   string `json:"name"`
		Corpo  string `json:"body"`
		Pagina string `json:"html_url"`
		Assets []struct {
			Nome string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resposta.Body, 4<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("ler a versão do GitHub: %w", err)
	}
	// A de desenvolvimento tem tag fixa ("desenvolvimento"): o número vem do título, "Finanças Local 2.2.1-dev.60 (…)".
	texto := release.Tag
	if desenvolvimento {
		texto = strings.TrimSpace(strings.Replace(release.Nome, "Finanças Local", "", 1))
	}
	versao, ok := LerVersao(texto)
	if !ok || versao.EhDesenvolvimento() != desenvolvimento {
		return nil, nil
	}
	d := Disponivel{Versao: versao, Tag: release.Tag, Notas: release.Corpo, Pagina: release.Pagina}
	for _, a := range release.Assets {
		switch {
		case strings.HasSuffix(strings.ToLower(a.Nome), ".msi"):
			d.Msi, d.NomeMsi = a.URL, a.Nome
		case strings.EqualFold(a.Nome, "SHA256SUMS.txt"):
			d.Somas = a.URL
		}
	}
	if d.Msi == "" {
		return nil, nil // versão só com o código, sem instalador
	}
	if d.Pagina == "" {
		d.Pagina = "https://github.com/" + Repositorio + "/releases"
	}
	return &d, nil
}

// ErrSemSomas e ErrHashDiferente: nada é instalado sem conferir o arquivo.
var (
	ErrSemSomas      = errors.New("a versão publicada não tem o SHA256SUMS.txt. Sem como conferir o arquivo, nada foi baixado")
	ErrHashDiferente = errors.New("o arquivo baixado não confere com o publicado (SHA256). Nada foi instalado; tente de novo")
)

// Baixar baixa o .msi para a pasta temporária e confere o SHA-256; se não bater, apaga o arquivo e devolve o erro.
// progresso recebe de 0 a 1 (só quando o servidor diz o tamanho).
func (c Cliente) Baixar(ctx context.Context, d Disponivel, progresso func(float64)) (string, error) {
	if d.Somas == "" {
		return "", ErrSemSomas
	}
	esperado, err := c.somaEsperada(ctx, d)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.Pasta, 0o700); err != nil {
		return "", err
	}
	destino := filepath.Join(c.Pasta, filepath.Base(d.NomeMsi))
	if err := c.gravar(ctx, d.Msi, destino, esperado, progresso); err != nil {
		_ = os.Remove(destino)
		return "", err
	}
	return destino, nil
}

func (c Cliente) somaEsperada(ctx context.Context, d Disponivel) (string, error) {
	resposta, err := c.pedir(ctx, d.Somas, "")
	if err != nil {
		return "", err
	}
	defer resposta.Body.Close()
	if err := sucesso(resposta); err != nil {
		return "", err
	}
	somas, err := io.ReadAll(io.LimitReader(resposta.Body, 64<<10))
	if err != nil {
		return "", err
	}
	esperado := Esperado(string(somas), d.NomeMsi)
	if esperado == "" {
		return "", fmt.Errorf("o SHA256SUMS.txt não tem o %s. Nada foi baixado", d.NomeMsi)
	}
	return esperado, nil
}

func (c Cliente) gravar(ctx context.Context, endereco, destino, esperado string, progresso func(float64)) error {
	resposta, err := c.pedir(ctx, endereco, "")
	if err != nil {
		return err
	}
	defer resposta.Body.Close()
	if err := sucesso(resposta); err != nil {
		return err
	}
	arquivo, err := os.Create(destino)
	if err != nil {
		return err
	}
	soma := sha256.New()
	_, err = io.Copy(io.MultiWriter(arquivo, soma), &leitorComProgresso{r: resposta.Body, total: resposta.ContentLength, avisar: progresso})
	if fechar := arquivo.Close(); err == nil {
		err = fechar
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(soma.Sum(nil)), esperado) {
		return ErrHashDiferente
	}
	return nil
}

// Esperado é o hash da linha "hash  nome" (formato do sha256sum; o "*" do modo binário é aceito) do arquivo pedido.
func Esperado(somas, nome string) string {
	for linha := range strings.SplitSeq(somas, "\n") {
		partes := strings.SplitN(strings.TrimSpace(linha), " ", 2)
		if len(partes) == 2 && strings.TrimLeft(strings.TrimSpace(partes[1]), "*") == nome {
			return partes[0]
		}
	}
	return ""
}

// Instalar abre o instalador com a janela dele (que pede o UAC e mostra o progresso), com o log ao lado do .msi. O
// gerenciador fecha logo depois, para o instalador poder trocá-lo.
func Instalar(msi string) error { return comandoDeInstalar(msi).Start() }

func comandoDeInstalar(msi string) *exec.Cmd {
	log := strings.TrimSuffix(msi, filepath.Ext(msi)) + ".log"
	return exec.Command("msiexec.exe", "/i", msi, "/l*v", log)
}

func (c Cliente) pedir(ctx context.Context, endereco, aceita string) (*http.Response, error) {
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, endereco, nil)
	if err != nil {
		return nil, err
	}
	pedido.Header.Set("User-Agent", "FinancasLocal-Gerenciador/"+c.Versao)
	if aceita != "" {
		pedido.Header.Set("Accept", aceita)
	}
	return c.HTTP.Do(pedido)
}

func sucesso(r *http.Response) error {
	if r.StatusCode < 200 || r.StatusCode > 299 {
		return fmt.Errorf("o GitHub respondeu %s", r.Status)
	}
	return nil
}

type leitorComProgresso struct {
	r      io.Reader
	total  int64
	lidos  int64
	avisar func(float64)
}

func (l *leitorComProgresso) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	l.lidos += int64(n)
	if l.total > 0 && l.avisar != nil {
		l.avisar(float64(l.lidos) / float64(l.total))
	}
	return n, err
}
