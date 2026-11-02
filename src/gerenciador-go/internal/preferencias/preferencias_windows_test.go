package preferencias

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// chavesDeTeste aponta as preferências para chaves só do teste (HKCU\Software\FinancasLocal\Testes\<teste>), apagadas
// no fim: o teste roda na máquina de quem desenvolve sem mexer no "Iniciar com o Windows" de verdade (#459).
func chavesDeTeste(t *testing.T) {
	t.Helper()
	testes := `Software\FinancasLocal\Testes`
	base := testes + `\` + t.Name()
	antes, antesIniciar := chave, chaveIniciar
	chave, chaveIniciar = base+`\Gerenciador`, base+`\Run`
	t.Cleanup(func() {
		chave, chaveIniciar = antes, antesIniciar
		// Das folhas para a raiz: o registro só apaga chave sem subchave. A de Testes fica se outro teste ainda a usa.
		for _, k := range []string{base + `\Gerenciador`, base + `\Run`, base, testes} {
			_ = registry.DeleteKey(registry.CURRENT_USER, k)
		}
	})
}

// Regra: sem nada gravado, valem os padrões do C# (não inicia com o Windows, procura versão nova, e o tipo da versão
// instalada decide receber as de desenvolvimento).
func TestPadroesSemNadaGravado(t *testing.T) {
	chavesDeTeste(t)
	if IniciarComSistema() {
		t.Error("IniciarComSistema: true sem nada gravado")
	}
	if !ProcurarAoAbrir() {
		t.Error("ProcurarAoAbrir: false sem nada gravado")
	}
	for _, padrao := range []bool{true, false} {
		if ReceberDesenvolvimento(padrao) != padrao {
			t.Errorf("ReceberDesenvolvimento(%v) sem nada gravado", padrao)
		}
	}
	// Desligar o que não existe não é erro.
	if err := DefinirIniciarComSistema(false); err != nil {
		t.Errorf("DefinirIniciarComSistema(false) sem a entrada: %v", err)
	}
}

// Regra: o que é gravado é o que se lê de volta, nos dois sentidos, e a escolha gravada vence o padrão.
func TestGravarELerDeVolta(t *testing.T) {
	chavesDeTeste(t)
	for _, ligado := range []bool{true, false, true} {
		if err := DefinirProcurarAoAbrir(ligado); err != nil {
			t.Fatal(err)
		}
		if ProcurarAoAbrir() != ligado {
			t.Errorf("ProcurarAoAbrir depois de gravar %v", ligado)
		}
		if err := DefinirReceberDesenvolvimento(ligado); err != nil {
			t.Fatal(err)
		}
		for _, padrao := range []bool{true, false} {
			if ReceberDesenvolvimento(padrao) != ligado {
				t.Errorf("ReceberDesenvolvimento(%v) depois de gravar %v", padrao, ligado)
			}
		}
		if err := DefinirIniciarComSistema(ligado); err != nil {
			t.Fatal(err)
		}
		if IniciarComSistema() != ligado {
			t.Errorf("IniciarComSistema depois de gravar %v", ligado)
		}
	}
}

// Regra: a entrada de iniciar com o Windows abre este executável, entre aspas, com --bandeja (como a do C#).
func TestEntradaDeIniciar(t *testing.T) {
	chavesDeTeste(t)
	if err := DefinirIniciarComSistema(true); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, chaveIniciar, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	valor, _, err := k.GetStringValue(nomeIniciar)
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	if quer := `"` + strings.TrimRight(exe, `\`) + `" --bandeja`; valor != quer {
		t.Errorf("entrada: %q, quer %q", valor, quer)
	}
}

// Regra: a cópia do banco começa numa pasta de verdade (Meus documentos).
func TestPastaDocumentos(t *testing.T) {
	pasta := PastaDocumentos()
	if !filepath.IsAbs(pasta) {
		t.Errorf("PastaDocumentos: %q não é um caminho completo", pasta)
	}
}
