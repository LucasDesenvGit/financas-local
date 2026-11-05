package instalacao

import "golang.org/x/sys/windows/registry"

// pastaDoServico lê a linha do serviço FinancasLocal no registro (legível sem administrador).
func pastaDoServico() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\FinancasLocal`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	linha, _, err := k.GetStringValue("ImagePath")
	if err != nil {
		return ""
	}
	return PastaDaLinhaDoServico(linha)
}
