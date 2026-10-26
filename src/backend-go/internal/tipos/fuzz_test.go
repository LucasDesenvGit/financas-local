package tipos

import (
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"testing"
)

// Fuzz dos tipos (#372). O teste em tabela confere os casos que alguém pensou; o fuzz gera entradas que ninguém
// pensou e confere o que vale para qualquer uma: ida e volta, escala e o formato do EF Core. As sementes são os
// valores do gabarito (testdata/efcore.json). Entrada que achou bug fica em testdata/fuzz/<alvo>/ e roda em todo
// go test, como regressão. Para fuzzar de verdade (CONTRIBUINDO.md, "Testes do back-end (Go)"):
//
//	go test ./internal/tipos -run '^$' -fuzz '^FuzzDecimalNoBanco$' -fuzztime 5m

var (
	// Como o EF Core grava o decimal: sem zero à esquerda, pelo menos uma casa e nenhum zero à direita (só o ".0").
	decimalDoEfCore = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.([0-9]*[1-9]|0)$`)
	// Como o System.Text.Json escreve o decimal: número sem aspas e sem expoente.
	numeroDoJSON = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	// Como o EF Core grava o DateTime: a fração some quando é zero e não tem zero à direita.
	dataHoraDoEfCore = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{0,6}[1-9])?$`)
	// Os dois formatos de GUID que o .NET grava e escreve: 32 dígitos, ou 8-4-4-4-12 com hífens.
	guidN = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	guidD = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	limiteDoDecimal = new(big.Int).Lsh(big.NewInt(1), 96) // o decimal do C# guarda o número inteiro em 96 bits
)

// cabeNoDecimalDoCSharp diz se o texto (no formato do EF Core ou do JSON) é um valor que o decimal do C# guarda
// sem arredondar: até 28 casas e os dígitos em 96 bits. Só esses o EF Core e o .NET escrevem.
func cabeNoDecimalDoCSharp(texto string) bool {
	inteiro, casas, _ := strings.Cut(strings.TrimPrefix(texto, "-"), ".")
	if len(casas) > casasMaximas {
		return false
	}
	digitos, ok := new(big.Int).SetString(inteiro+casas, 10)
	return ok && digitos.Cmp(limiteDoDecimal) < 0
}

// zeroNegativo é "-0", "-0.00"...: o decimal do C# guarda o sinal do zero, o shopspring/decimal não (sai "0.00").
// Conta de dinheiro do projeto não grava zero negativo; a diferença fica de fora da comparação exata.
func zeroNegativo(texto string) bool {
	return strings.HasPrefix(texto, "-") && strings.Trim(texto, "-0.") == ""
}

func sementesDecimal(f *testing.F) []string {
	sementes := []string{"1e3", "1.5e-3", "-0.0", "0." + strings.Repeat("0", 27) + "25", "79228162514264337593543950335"}
	for _, c := range lerGabarito(f).Casos {
		for _, campo := range []string{"Valor", "Preciso", "Opcional"} {
			if v := c.NoBanco[campo]; v != nil {
				sementes = append(sementes, *v)
			}
		}
		for _, campo := range []string{"valor", "preciso", "opcional"} {
			if v, ok := c.JSON[campo]; ok && string(v) != "null" {
				sementes = append(sementes, string(v))
			}
		}
	}
	for _, v := range decimaisDaSonda {
		sementes = append(sementes, v)
	}
	return sementes
}

// FuzzDecimalNoBanco: o que o Decimal lê volta para o banco no formato do EF Core, e o texto gravado lê de volta o
// mesmo valor. O texto que já está no formato do EF Core (e cabe no decimal do C#) volta igual, letra por letra.
func FuzzDecimalNoBanco(f *testing.F) {
	for _, s := range sementesDecimal(f) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, texto string) {
		d, err := NovoDecimal(texto)
		if err != nil {
			return
		}
		gravado := d.TextoNoBanco()
		if !decimalDoEfCore.MatchString(gravado) {
			t.Fatalf("NovoDecimal(%q) grava %q, fora do formato do EF Core", texto, gravado)
		}
		var lido Decimal
		if err := lido.Scan(gravado); err != nil {
			t.Fatalf("o gravado %q não lê de volta: %v", gravado, err)
		}
		if !lido.Decimal().Equal(d.Decimal()) || lido.TextoNoBanco() != gravado {
			t.Fatalf("ida e volta pelo banco: %q → %q → %q", texto, gravado, lido.TextoNoBanco())
		}
		if decimalDoEfCore.MatchString(texto) && cabeNoDecimalDoCSharp(texto) && !zeroNegativo(texto) && gravado != texto {
			t.Fatalf("o texto do EF Core %q volta como %q", texto, gravado)
		}
	})
}

// FuzzDecimalJSON: o que o Decimal lê do JSON sai como número (sem aspas, sem expoente, nunca por float64) e com a
// mesma escala. O número que o .NET escreveria volta igual: os zeros à direita não se perdem.
func FuzzDecimalJSON(f *testing.F) {
	for _, s := range sementesDecimal(f) {
		f.Add(s)
		f.Add(`"` + s + `"`)
	}
	f.Fuzz(func(t *testing.T, entrada string) {
		var d Decimal
		if json.Unmarshal([]byte(entrada), &d) != nil {
			return
		}
		saida, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("Marshal de %q: %v", entrada, err)
		}
		if !numeroDoJSON.Match(saida) {
			t.Fatalf("%q sai no JSON como %s, que não é o número do System.Text.Json", entrada, saida)
		}
		var volta Decimal
		if err := json.Unmarshal(saida, &volta); err != nil {
			t.Fatalf("o JSON %s não lê de volta: %v", saida, err)
		}
		if volta.String() != d.String() || volta.Escala() != d.Escala() {
			t.Fatalf("ida e volta pelo JSON: %q → %s → %s (escala %d → %d)", entrada, saida, volta, d.Escala(), volta.Escala())
		}
		if numeroDoJSON.MatchString(entrada) && cabeNoDecimalDoCSharp(entrada) && !zeroNegativo(entrada) && string(saida) != entrada {
			t.Fatalf("o número %s sai como %s: a escala se perdeu", entrada, saida)
		}
	})
}

// FuzzDataNoBanco: a data só é lida no formato do EF Core ("2026-10-15", com dia que existe: 29/02 só em ano
// bissexto, 31 só nos meses de 31) e volta igual pelo banco e pelo JSON.
func FuzzDataNoBanco(f *testing.F) {
	for _, c := range lerGabarito(f).Casos {
		f.Add(*c.NoBanco["Dia"])
	}
	for _, s := range []string{"2024-02-29", "2026-02-28", "2026-04-30", "2026-12-31", "0001-01-01", "9999-12-31"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, texto string) {
		d, err := LerData(texto)
		if err != nil {
			return
		}
		if d.String() != strings.TrimSpace(texto) {
			t.Fatalf("LerData(%q) = %s: leu outra data", texto, d)
		}
		var lida Data
		gravado, _ := d.Value()
		if err := lida.Scan(gravado); err != nil || lida.String() != d.String() {
			t.Fatalf("ida e volta pelo banco: %s → %v → %s (%v)", d, gravado, lida, err)
		}
		var doJSON Data
		dados, _ := json.Marshal(d)
		if err := json.Unmarshal(dados, &doJSON); err != nil || doJSON.String() != d.String() {
			t.Fatalf("ida e volta pelo JSON: %s → %s → %s (%v)", d, dados, doJSON, err)
		}
		if outra := d.SomarDias(1).SomarDias(-1); outra.String() != d.String() {
			t.Fatalf("%s + 1 dia - 1 dia = %s", d, outra)
		}
	})
}

// FuzzDataHoraNoBanco: a data e hora gravada sai no formato do EF Core, com a precisão do C# (100 ns), e volta
// igual pelo banco e pelo JSON. O texto que já está no formato do EF Core volta igual, letra por letra.
func FuzzDataHoraNoBanco(f *testing.F) {
	for _, c := range lerGabarito(f).Casos {
		f.Add(*c.NoBanco["Momento"])
		var noJSON string
		if json.Unmarshal(c.JSON["momento"], &noJSON) == nil {
			f.Add(noJSON)
		}
	}
	f.Add("2026-10-15T22:55:31.3481471Z")
	f.Add("2026-10-15 22:55:31.123456789")
	f.Fuzz(func(t *testing.T, texto string) {
		d, err := LerDataHora(texto)
		if err != nil {
			return
		}
		gravado := d.String()
		if !dataHoraDoEfCore.MatchString(gravado) {
			t.Fatalf("LerDataHora(%q) grava %q, fora do formato do EF Core", texto, gravado)
		}
		if d.Tempo().Nanosecond()%100 != 0 {
			t.Fatalf("%q guarda %d ns: o C# só tem 100 ns de precisão", texto, d.Tempo().Nanosecond())
		}
		var lida DataHora
		if err := lida.Scan(gravado); err != nil || !lida.Tempo().Equal(d.Tempo()) || lida.String() != gravado {
			t.Fatalf("ida e volta pelo banco: %q → %q → %s (%v)", texto, gravado, lida, err)
		}
		var doJSON DataHora
		dados, _ := json.Marshal(d)
		if err := json.Unmarshal(dados, &doJSON); err != nil || !doJSON.Tempo().Equal(d.Tempo()) {
			t.Fatalf("ida e volta pelo JSON: %q → %s → %s (%v)", texto, dados, doJSON, err)
		}
		if dataHoraDoEfCore.MatchString(texto) && gravado != texto {
			t.Fatalf("o texto do EF Core %q volta como %q", texto, gravado)
		}
	})
}

// FuzzGuid: o GUID só é lido nos formatos que o .NET grava e escreve (32 dígitos, ou 8-4-4-4-12), vai para o banco
// em maiúsculas (como o EF Core), para o JSON em minúsculas (como o System.Text.Json), e volta igual.
func FuzzGuid(f *testing.F) {
	for _, c := range lerGabarito(f).Casos {
		f.Add(*c.NoBanco["Identificador"])
		var noJSON string
		if json.Unmarshal(c.JSON["identificador"], &noJSON) == nil {
			f.Add(noJSON)
		}
	}
	f.Add("2f00defb672e45ef87d0702995ae8d2e")
	f.Add("00000000-0000-0000-0000-000000000000")
	f.Fuzz(func(t *testing.T, texto string) {
		g, err := LerGuid(texto)
		if err != nil {
			return
		}
		if limpo := strings.TrimSpace(texto); !guidN.MatchString(limpo) && !guidD.MatchString(limpo) {
			t.Fatalf("LerGuid aceitou %q, que o .NET não grava nem escreve", texto)
		}
		gravado, _ := g.Value()
		if gravado != strings.ToUpper(g.String()) {
			t.Fatalf("%q vai para o banco como %v: o EF Core grava em maiúsculas", texto, gravado)
		}
		if g.String() != strings.ToLower(g.String()) || !guidD.MatchString(g.String()) {
			t.Fatalf("%q sai no JSON como %s: o System.Text.Json escreve 8-4-4-4-12 em minúsculas", texto, g)
		}
		var lido Guid
		if err := lido.Scan(gravado); err != nil || lido != g {
			t.Fatalf("ida e volta pelo banco: %q → %v → %s (%v)", texto, gravado, lido, err)
		}
		var doJSON Guid
		dados, _ := json.Marshal(g)
		if err := json.Unmarshal(dados, &doJSON); err != nil || doJSON != g {
			t.Fatalf("ida e volta pelo JSON: %q → %s → %s (%v)", texto, dados, doJSON, err)
		}
	})
}
