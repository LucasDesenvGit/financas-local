package esquema

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"
)

// Copias diz de onde vem cada arquivo de embutido/ (caminhos a partir de src/backend-go). O //go:embed só alcança
// arquivos do próprio módulo (src/backend-go), e o banco/ fica fora dele: o go generate (esquema/gerar) copia os
// três arquivos para embutido/, e o TestCopiasEmDia e a CI ("go generate em dia") falham se uma cópia ficar
// diferente do original. O original continua sendo a fonte única (ADR 0003).
var Copias = map[string]string{
	"01-estrutura.sql":      ArquivoEstrutura,
	"02-dados-iniciais.sql": ArquivoDadosIniciais,
	"2-criar-banco.sh":      ArquivoColunas,
}

// Scripts são os três textos que o EsquemaBanco lê: a estrutura, os dados iniciais e o 2-criar-banco.sh (de onde
// saem as colunas novas).
type Scripts struct {
	Estrutura, DadosIniciais, CriarBanco string
}

// Texto lê o arquivo como o EsquemaBanco.Ler: o StreamReader em UTF-8 descarta a marca de ordem de bytes (BOM) do
// começo, que mudaria a assinatura.
func Texto(conteudo []byte) string {
	return strings.TrimPrefix(string(conteudo), "\uFEFF")
}

// Assinatura é o número que vai no PRAGMA user_version, calculado como o EsquemaBanco.Assinatura do .NET: SHA-256
// da estrutura (CRLF virando LF), de uma quebra de linha e das colunas "Tabela|Coluna|Definicao", uma por linha; os
// 4 primeiros bytes lidos como inteiro little-endian (o BitConverter.ToInt32), sem o bit de sinal, e 1 no lugar de 0.
// Igual dos dois lados, trocar entre o .NET e o Go não refaz a cópia do banco a cada partida.
func Assinatura(estrutura string, colunas []ColunaNova) int32 {
	linhas := make([]string, len(colunas))
	for i, c := range colunas {
		linhas[i] = c.Tabela + "|" + c.Coluna + "|" + c.Definicao
	}
	texto := strings.ReplaceAll(estrutura, "\r\n", "\n") + "\n" + strings.Join(linhas, "\n")
	hash := sha256.Sum256([]byte(texto))
	return assinaturaDoHash(hash)
}

func assinaturaDoHash(hash [sha256.Size]byte) int32 {
	valor := int32(binary.LittleEndian.Uint32(hash[:4]) & math.MaxInt32)
	if valor == 0 {
		return 1
	}
	return valor
}
