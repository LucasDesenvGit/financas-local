package segredos

import (
	"bytes"
	"testing"
)

// Regra: no Windows, a chave vai para o arquivo cifrada pelo DPAPI da conta (nunca como está) e volta igual (#459).
func TestDPAPIIdaEVolta(t *testing.T) {
	chave := bytes.Repeat([]byte{0x5a, 0x01, 0xc3}, 11)
	protegida, err := proteger(chave)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(protegida, chave) {
		t.Fatal("a chave aparece como está no que vai para o arquivo")
	}
	aberta, err := desproteger(protegida)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(aberta, chave) {
		t.Errorf("voltou %x, quer %x", aberta, chave)
	}
}

// Regra: o que não foi cifrado pelo DPAPI desta conta, com esta entropia, não abre (arquivo vazio, mexido, de outro
// programa ou de outro uso do DPAPI na mesma conta).
func TestDPAPIRecusa(t *testing.T) {
	protegida, err := proteger([]byte("chave de 32 bytes do cofre......"))
	if err != nil {
		t.Fatal(err)
	}
	mexida := append([]byte(nil), protegida...)
	mexida[len(mexida)/2] ^= 0xff
	casos := map[string][]byte{
		"vazio":          nil,
		"mexido no meio": mexida,
		"não é do DPAPI": []byte("isto não saiu do CryptProtectData"),
	}
	for nome, guardada := range casos {
		if _, err := desproteger(guardada); err == nil {
			t.Errorf("%s: abriu", nome)
		}
	}
	// O vazio diz o que houve, sem chegar ao DPAPI (que só diria "parâmetro incorreto").
	if _, err := desproteger(nil); err == nil || err.Error() != "arquivo vazio" {
		t.Errorf("vazio: %v", err)
	}

	antes := entropia
	entropia = []byte("outro uso do DPAPI na mesma conta")
	defer func() { entropia = antes }()
	if _, err := desproteger(protegida); err == nil {
		t.Error("abriu com outra entropia")
	}
}
