package banco

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
)

// Cópias "antes da atualização" em dados/backups: o nome (#366) e o rodízio das 5 mais recentes (#365, #366).

// #366: o nome vai até o milissegundo, como o yyyyMMdd_HHmmss_fff do .NET (que corta, não arredonda).
func TestNomeDaCopia(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome      string
		instante  time.Time
		resultado string
	}{
		{"segundo em ponto", time.Date(2026, 10, 22, 23, 30, 0, 0, relogio.Brasilia), PrefixoBackup + "20261022_233000_000.db"},
		{"um milissegundo", time.Date(2026, 10, 22, 23, 30, 0, int(time.Millisecond), relogio.Brasilia), PrefixoBackup + "20261022_233000_001.db"},
		{"corta a fração, não arredonda", time.Date(2026, 10, 22, 23, 30, 0, 999_600_000, relogio.Brasilia), PrefixoBackup + "20261022_233000_999.db"},
		{"meio-dia (24 horas, sem AM/PM)", time.Date(2026, 1, 2, 12, 5, 9, 50_000_000, relogio.Brasilia), PrefixoBackup + "20260102_120509_050.db"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			if obtido := nomeDaCopia(c.instante); obtido != c.resultado {
				t.Errorf("nomeDaCopia(%s) = %s, esperado %s", c.instante.Format(time.RFC3339Nano), obtido, c.resultado)
			}
		})
	}
}

// #366: com o nome novo (milissegundos) misturado aos nomes antigos (só até o segundo), a ordem pelo nome continua a
// ordem do tempo, e o rodízio apaga a mais antiga. O "." vem antes do "_" na tabela ASCII: o nome antigo de um
// segundo fica antes dos nomes novos do mesmo segundo.
func TestRodizioComNomesAntigosENovos(t *testing.T) {
	t.Parallel()
	antigo := func(segundo string) string { return PrefixoBackup + segundo + ".db" }
	novo := func(dia, segundo, ms int) string {
		return nomeDaCopia(time.Date(2020, 1, dia, 0, 0, segundo, ms*int(time.Millisecond), relogio.Brasilia))
	}
	casos := []struct {
		nome    string
		copias  []string
		apagada string // a única que sai: são sempre 6 cópias
	}{
		{
			"nome novo no mesmo segundo de um antigo é mais recente",
			[]string{antigo("20200101_000000"), novo(1, 0, 0), antigo("20200102_000000"), antigo("20200103_000000"), antigo("20200104_000000"), antigo("20200105_000000")},
			antigo("20200101_000000"),
		},
		{
			"nome antigo do segundo seguinte é mais recente que o novo com 999 ms",
			[]string{novo(1, 0, 999), antigo("20200101_000001"), antigo("20200102_000000"), antigo("20200103_000000"), antigo("20200104_000000"), antigo("20200105_000000")},
			novo(1, 0, 999),
		},
		{
			"só nomes novos, todos no mesmo segundo",
			[]string{novo(1, 0, 5), novo(1, 0, 40), novo(1, 0, 300), novo(1, 0, 2), novo(1, 0, 999), novo(1, 0, 10)},
			novo(1, 0, 2),
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			pasta := t.TempDir()
			for _, nome := range c.copias {
				if err := os.WriteFile(filepath.Join(pasta, nome), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := apagarCopiasAntigas(pasta, semLog()); err != nil {
				t.Fatal(err)
			}
			restantes := nomesNaPasta(t, pasta)
			esperadas := slices.DeleteFunc(slices.Clone(c.copias), func(n string) bool { return n == c.apagada })
			slices.Sort(esperadas)
			if !slices.Equal(restantes, esperadas) {
				t.Errorf("ficaram:\n%q\nesperado (sem %s):\n%q", restantes, c.apagada, esperadas)
			}
		})
	}
}

// #366: duas atualizações no mesmo segundo (a primeira falha depois da cópia e o serviço reinicia em seguida). Antes,
// a segunda parava a API: o nome repetia e o VACUUM INTO recusa um destino que já existe. Aqui, sete atualizações
// no mesmo segundo, com o relógio parado em cada uma: todas dão certo e ficam as 5 mais recentes.
func TestAtualizacoesNoMesmoSegundo(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	caminho, pasta := bancoParaAtualizar(t)
	var nomes []string
	for ms := 100; ms <= 700; ms += 100 {
		agora := relogio.Fixo{Instante: instanteDosTestes.Instante.Add(time.Duration(ms) * time.Millisecond)}
		executar(t, caminho, "PRAGMA user_version = 7;")
		if err := AplicarEsquema(ctx, caminho, agora, semLog()); err != nil {
			t.Fatalf("atualização às 23:30:00,%03d: %v", ms, err)
		}
		nomes = append(nomes, nomeDaCopia(relogio.AgoraLocal(agora)))
	}
	restantes := nomesNaPasta(t, pasta)
	if esperadas := nomes[len(nomes)-BackupsGuardados:]; !slices.Equal(restantes, esperadas) {
		t.Errorf("ficaram:\n%q\nesperado (as 5 mais recentes):\n%q", restantes, esperadas)
	}
}

// #365: uma cópia antiga que não dá para apagar (outro dono, só leitura) fica para a próxima, com um aviso, e não
// impede a API de subir. Antes, falta de permissão era erro, como o UnauthorizedAccessException no .NET.
func TestApagarCopiasAntigasSemPermissao(t *testing.T) {
	t.Parallel()
	pularSemPermissaoDePasta(t)
	pasta := t.TempDir()
	var copias []string
	for dia := 1; dia <= BackupsGuardados+2; dia++ {
		nome := PrefixoBackup + time.Date(2020, 1, dia, 0, 0, 0, 0, time.UTC).Format("20060102_150405") + ".db"
		copias = append(copias, nome)
		if err := os.WriteFile(filepath.Join(pasta, nome), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tirarPermissaoDeEscrita(t, pasta)

	log, avisos := registrarAvisos()
	if err := apagarCopiasAntigas(pasta, log); err != nil {
		t.Fatalf("apagarCopiasAntigas sem permissão = %v, esperado nil (fica para a próxima)", err)
	}
	if obtidas := nomesNaPasta(t, pasta); !slices.Equal(obtidas, copias) {
		t.Errorf("sem permissão, nada deveria sair; ficaram %q", obtidas)
	}
	obtidos, esperados := avisos.arquivos(), copias[:2]
	slices.Sort(obtidos)
	if !slices.Equal(obtidos, esperados) {
		t.Errorf("avisos com o arquivo = %q, esperado %q (as duas mais antigas)", obtidos, esperados)
	}
}

// #365, o caminho inteiro: a cópia nova é feita, a antiga não sai, o esquema é aplicado e a API sobe.
//
// A pasta das cópias fica sem permissão de escrita, como acontece com uma cópia de outro dono. Para a cópia nova
// entrar mesmo assim, o nome dela (conhecido, com o relógio parado) já está na pasta como um link para um arquivo
// noutra pasta, que pode ser escrita: o SQLite segue o link e grava lá (a sonda do .NET não tem como fazer isso,
// porque o nome lá vem do DateTime.Now).
func TestAplicarEsquemaComCopiaAntigaSemPermissao(t *testing.T) {
	t.Parallel()
	pularSemPermissaoDePasta(t)
	ctx := t.Context()
	caminho, pasta := bancoParaAtualizar(t)
	executar(t, caminho, "PRAGMA user_version = 7;")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	maisAntiga := PrefixoBackup + "20200101_000000.db"
	nomes := []string{maisAntiga, nomeDaCopia(relogio.AgoraLocal(instanteDosTestes))}
	for dia := 2; dia <= BackupsGuardados; dia++ {
		nomes = append(nomes, PrefixoBackup+time.Date(2020, 1, dia, 0, 0, 0, 0, time.UTC).Format("20060102_150405")+".db")
	}
	for _, nome := range slices.Delete(slices.Clone(nomes), 1, 2) {
		if err := os.WriteFile(filepath.Join(pasta, nome), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copiaNova := filepath.Join(t.TempDir(), "copia-nova.db")
	if err := os.Symlink(copiaNova, filepath.Join(pasta, nomes[1])); err != nil {
		t.Fatal(err)
	}
	tirarPermissaoDeEscrita(t, pasta)

	log, avisos := registrarAvisos()
	if err := AplicarEsquema(ctx, caminho, instanteDosTestes, log); err != nil {
		t.Fatalf("AplicarEsquema com uma cópia antiga sem permissão de apagar = %v, esperado nil", err)
	}
	if obtidos := avisos.arquivos(); !slices.Equal(obtidos, []string{maisAntiga}) {
		t.Errorf("avisos com o arquivo = %q, esperado %q", obtidos, []string{maisAntiga})
	}
	if versao := contar(t, caminho, "PRAGMA user_version"); versao == 7 {
		t.Error("o esquema não foi aplicado: o user_version continua 7")
	}
	if n := contar(t, copiaNova, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'"); n == 0 {
		t.Error("a cópia nova não tem as tabelas do banco")
	}
}

// Um arquivo que sumiu entre listar e apagar (outro processo apagou antes) não gera aviso: o FileInfo.Delete do .NET
// não lança exceção quando o arquivo não existe.
func TestApagarCopiaQueJaSumiu(t *testing.T) {
	t.Parallel()
	caminho := filepath.Join(t.TempDir(), PrefixoBackup+"20200101_000000.db")
	if err := apagarCopia(caminho); err != nil {
		t.Errorf("apagarCopia(arquivo que não existe) = %v, esperado nil", err)
	}
}

// bancoParaAtualizar cria um banco com os scripts embutidos em dados/sql e devolve o caminho dele e o de
// dados/backups.
func bancoParaAtualizar(t *testing.T) (caminho, pastaCopias string) {
	t.Helper()
	dados := filepath.Join(t.TempDir(), "dados")
	caminho = filepath.Join(dados, "sql", "FinancasLocal.db")
	if err := AplicarEsquema(t.Context(), caminho, instanteDosTestes, semLog()); err != nil {
		t.Fatal(err)
	}
	return caminho, filepath.Join(dados, "backups")
}

// nomesNaPasta lista as cópias "antes da atualização" da pasta, em ordem de nome.
func nomesNaPasta(t *testing.T, pasta string) []string {
	t.Helper()
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	nomes := []string{}
	for _, e := range entradas {
		nomes = append(nomes, e.Name())
	}
	return nomes // o os.ReadDir já devolve em ordem de nome
}

func pularSemPermissaoDePasta(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("usa a permissão de pasta do Unix (chmod); no Windows, o modo só-leitura de pasta não impede apagar")
	}
	if os.Geteuid() == 0 {
		t.Skip("rodando como root, que apaga sem permissão de escrita: o teste não provaria nada. Rode com outro usuário")
	}
}

// tirarPermissaoDeEscrita deixa a pasta só para ler (ninguém apaga nem cria arquivo nela) e devolve a permissão no
// fim do teste, para o t.TempDir conseguir limpar.
func tirarPermissaoDeEscrita(t *testing.T, pasta string) {
	t.Helper()
	if err := os.Chmod(pasta, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(pasta, 0o755); err != nil {
			t.Error(err)
		}
	})
	// Confere que a falta de permissão vale de fato (sistema de arquivos sem permissão Unix, por exemplo).
	teste := filepath.Join(pasta, "conferir-permissao")
	if err := os.WriteFile(teste, nil, 0o644); !errors.Is(err, fs.ErrPermission) {
		os.Remove(teste)
		t.Skipf("a pasta sem permissão de escrita aceitou um arquivo novo (%v): o teste não provaria nada", err)
	}
}

// avisosRegistrados guarda os registros de nível Warn ou acima.
type avisosRegistrados struct {
	mu        sync.Mutex
	registros []slog.Record
}

func registrarAvisos() (*slog.Logger, *avisosRegistrados) {
	a := &avisosRegistrados{}
	return slog.New(a), a
}

func (a *avisosRegistrados) Enabled(_ context.Context, nivel slog.Level) bool {
	return nivel >= slog.LevelWarn
}

func (a *avisosRegistrados) Handle(_ context.Context, r slog.Record) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.registros = append(a.registros, r.Clone())
	return nil
}

func (a *avisosRegistrados) WithAttrs([]slog.Attr) slog.Handler { return a }
func (a *avisosRegistrados) WithGroup(string) slog.Handler      { return a }

// arquivos devolve o atributo "arquivo" de cada aviso, na ordem em que saíram.
func (a *avisosRegistrados) arquivos() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	arquivos := []string{}
	for _, r := range a.registros {
		r.Attrs(func(atributo slog.Attr) bool {
			if atributo.Key == "arquivo" {
				arquivos = append(arquivos, atributo.Value.String())
			}
			return true
		})
	}
	return arquivos
}
