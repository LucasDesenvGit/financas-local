package tipos

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Guid é o Guid do C#. O EF Core grava em MAIÚSCULAS no banco ("2F00DEFB-672E-…"), e o System.Text.Json
// escreve em minúsculas no JSON ("2f00defb-672e-…"). Os dois formatos foram confirmados pela sonda.
type Guid [16]byte

// GuidVazio é o Guid.Empty do C#.
var GuidVazio Guid

// NovoGuid gera um GUID aleatório (versão 4), como Guid.NewGuid().
func NovoGuid() Guid {
	var g Guid
	_, _ = rand.Read(g[:]) // crypto/rand.Read nunca falha (documentado desde o Go 1.24)
	g[6] = g[6]&0x0f | 0x40
	g[8] = g[8]&0x3f | 0x80
	return g
}

// LerGuid lê os dois formatos que o .NET grava e escreve: 8-4-4-4-12 ("2f00defb-672e-45ef-87d0-702995ae8d2e") ou
// os 32 dígitos sem hífen, em maiúsculas ou minúsculas, com os espaços em volta ignorados, como o Guid.Parse. Hífen
// fora do lugar é recusado, como no .NET (#372, achado pelo fuzz). Os formatos com chaves e parênteses, que o
// Guid.Parse também lê, ficam de fora: o .NET do projeto não os escreve.
func LerGuid(texto string) (Guid, error) {
	var g Guid
	limpo := strings.TrimSpace(texto)
	if len(limpo) == 36 && limpo[8] == '-' && limpo[13] == '-' && limpo[18] == '-' && limpo[23] == '-' {
		limpo = limpo[:8] + limpo[9:13] + limpo[14:18] + limpo[19:23] + limpo[24:]
	}
	if len(limpo) != 32 {
		return g, fmt.Errorf("guid inválido %q", texto)
	}
	if _, err := hex.Decode(g[:], []byte(limpo)); err != nil {
		return g, fmt.Errorf("guid inválido %q: %w", texto, err)
	}
	return g, nil
}

// String escreve em minúsculas, como o C# e o JSON.
func (g Guid) String() string {
	h := hex.EncodeToString(g[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// Value grava em maiúsculas, como o EF Core (driver.Valuer).
func (g Guid) Value() (driver.Value, error) { return strings.ToUpper(g.String()), nil }

// Scan lê do banco (sql.Scanner).
func (g *Guid) Scan(origem any) error {
	var texto string
	switch v := origem.(type) {
	case string:
		texto = v
	case []byte:
		texto = string(v)
	default:
		return fmt.Errorf("guid gravado como %T, esperado TEXT", origem)
	}
	lido, err := LerGuid(texto)
	if err != nil {
		return err
	}
	*g = lido
	return nil
}

func (g Guid) MarshalJSON() ([]byte, error) { return json.Marshal(g.String()) }

func (g *Guid) UnmarshalJSON(dados []byte) error {
	var texto string
	if err := json.Unmarshal(dados, &texto); err != nil {
		return fmt.Errorf("guid deve vir como texto: %w", err)
	}
	lido, err := LerGuid(texto)
	if err != nil {
		return err
	}
	*g = lido
	return nil
}
