//go:build !windows

package segredos

// Fora do Windows a chave fica crua no arquivo, e quem a protege é a permissão 0600 (só a conta do serviço lê), como
// as chaves do Data Protection hoje no Linux (ADR 0005).
func proteger(chave []byte) ([]byte, error) { return chave, nil }

func desproteger(guardada []byte) ([]byte, error) { return guardada, nil }
