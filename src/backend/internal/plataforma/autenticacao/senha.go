package autenticacao

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // só para ler hash antigo (v2 e v3 com HMAC-SHA1), como o PasswordHasher
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"hash"
)

// O hash de senha do ASP.NET Identity (PasswordHasher<TUser>), que é o que está no banco: o Go lê e grava no mesmo
// formato, para dar para voltar ao .NET sem trocar a senha de ninguém.
//
// Versão 3 (a de hoje): 0x01 | PRF (uint32, big-endian) | iterações (uint32) | tamanho do sal (uint32) | sal | chave,
// em Base64. O .NET grava PBKDF2 com HMAC-SHA512, 100 mil iterações, sal de 16 bytes e chave de 32 bytes.
// Versão 2 (antiga): 0x00 | sal (16) | chave (32), PBKDF2 com HMAC-SHA1 e mil iterações.
// Fonte: https://github.com/dotnet/aspnetcore/blob/main/src/Identity/Extensions.Core/src/PasswordHasher.cs
const (
	iteracoesV3  = 100_000
	tamanhoSalV3 = 16
	tamanhoChave = 32
	cabecalhoV3  = 13 // 1 byte de formato e 3 uint32

	prfSHA1   = 0 // KeyDerivationPrf.HMACSHA1
	prfSHA256 = 1 // KeyDerivationPrf.HMACSHA256
	prfSHA512 = 2 // KeyDerivationPrf.HMACSHA512
)

// ResultadoSenha é o PasswordVerificationResult do .NET.
type ResultadoSenha int

const (
	SenhaErrada       ResultadoSenha = iota // Failed
	SenhaCerta                              // Success
	SenhaCertaRefazer                       // SuccessRehashNeeded: certa, mas num formato mais fraco que o de hoje
)

// GerarHash faz o hash da senha no formato v3 do .NET (HMAC-SHA512, 100 mil iterações, sal aleatório).
func GerarHash(senha string) (string, error) {
	sal := make([]byte, tamanhoSalV3)
	if _, err := rand.Read(sal); err != nil {
		return "", err
	}
	chave, err := pbkdf2.Key(sha512.New, senha, sal, iteracoesV3, tamanhoChave)
	if err != nil {
		return "", err
	}
	saida := make([]byte, cabecalhoV3+len(sal)+len(chave))
	saida[0] = 0x01
	binary.BigEndian.PutUint32(saida[1:], prfSHA512)
	binary.BigEndian.PutUint32(saida[5:], iteracoesV3)
	binary.BigEndian.PutUint32(saida[9:], tamanhoSalV3)
	copy(saida[cabecalhoV3:], sal)
	copy(saida[cabecalhoV3+len(sal):], chave)
	return base64.StdEncoding.EncodeToString(saida), nil
}

// VerificarHash confere a senha com o hash do banco, em tempo constante na comparação da chave. Hash inválido ou de
// formato desconhecido é senha errada, como no .NET.
func VerificarHash(hashBase64, senha string) ResultadoSenha {
	dados, err := base64.StdEncoding.DecodeString(hashBase64)
	if err != nil || len(dados) == 0 {
		return SenhaErrada
	}
	switch dados[0] {
	case 0x00:
		if verificarV2(dados, senha) {
			return SenhaCertaRefazer // o .NET também pede para refazer o hash v2
		}
	case 0x01:
		certa, prf, iteracoes := verificarV3(dados, senha)
		if !certa {
			return SenhaErrada
		}
		// Igual ao .NET: menos iterações que as de hoje, ou PRF mais fraca que SHA-512, pede para refazer o hash.
		if iteracoes < iteracoesV3 || prf != prfSHA512 {
			return SenhaCertaRefazer
		}
		return SenhaCerta
	}
	return SenhaErrada
}

func verificarV2(dados []byte, senha string) bool {
	const sal, chave = 16, 32
	if len(dados) != 1+sal+chave {
		return false
	}
	calculada, err := pbkdf2.Key(sha1.New, senha, dados[1:1+sal], 1000, chave)
	return err == nil && subtle.ConstantTimeCompare(calculada, dados[1+sal:]) == 1
}

func verificarV3(dados []byte, senha string) (certa bool, prf, iteracoes uint32) {
	if len(dados) < cabecalhoV3 {
		return false, 0, 0
	}
	prf = binary.BigEndian.Uint32(dados[1:])
	iteracoes = binary.BigEndian.Uint32(dados[5:])
	tamanhoSal := int(binary.BigEndian.Uint32(dados[9:]))
	// O .NET recusa sal ou chave com menos de 128 bits.
	if tamanhoSal < 16 || tamanhoSal > len(dados)-cabecalhoV3 || iteracoes == 0 {
		return false, prf, iteracoes
	}
	sal := dados[cabecalhoV3 : cabecalhoV3+tamanhoSal]
	esperada := dados[cabecalhoV3+tamanhoSal:]
	if len(esperada) < 16 {
		return false, prf, iteracoes
	}
	var funcao func() hash.Hash
	switch prf {
	case prfSHA1:
		funcao = sha1.New
	case prfSHA256:
		funcao = sha256.New
	case prfSHA512:
		funcao = sha512.New
	default:
		return false, prf, iteracoes
	}
	calculada, err := pbkdf2.Key(funcao, senha, sal, int(iteracoes), len(esperada))
	if err != nil {
		return false, prf, iteracoes
	}
	return subtle.ConstantTimeCompare(calculada, esperada) == 1, prf, iteracoes
}
