package protecao

import "testing"

// Regra (#68, item 14): pasta com o nome de um cliente de nuvem, em qualquer parte do caminho e sem diferença de
// maiúsculas, ou dentro da pasta das variáveis do OneDrive, está na nuvem; nome parecido só no meio de outro caminho
// também conta (uma pasta "OneDrive - Empresa"); o resto não.
func TestNaNuvem(t *testing.T) {
	t.Parallel()
	ambiente := map[string]string{"OneDriveCommercial": `D:\Nuvem da Empresa\`}
	casos := []struct {
		caminho string
		nuvem   bool
	}{
		{`C:\Users\ana\OneDrive\financas-local\dados`, true},
		{`C:\Users\ana\OneDrive - Empresa X\financas-local\dados`, true},
		{`C:\Users\ana\Dropbox\financas-local\dados`, true},
		{`G:\My Drive\financas-local\dados`, true},
		{`G:\Meu Drive\financas-local\dados`, true},
		{`C:\Users\ana\Google Drive\financas\dados`, true},
		{`C:\Users\ana\iCloudDrive\financas\dados`, true},
		{"/home/ana/dropbox/financas-local/dados", true},
		{"/home/ana/GoogleDrive/financas-local/dados", true},
		{`d:\nuvem da empresa\financas\dados`, true},
		{`D:\Nuvem da Empresa`, true},
		{`D:\Nuvem da Empresa Antiga\dados`, false},
		{`C:\ProgramData\FinancasLocal\dados`, false},
		{"/var/home/ana/Repositorios/financas-local/dados", false},
		{"/home/ana/drive/dados", false},
		{"", false},
	}
	for _, c := range casos {
		if obtido := NaNuvem(c.caminho, func(v string) string { return ambiente[v] }); obtido != c.nuvem {
			t.Errorf("%q: %v, esperado %v", c.caminho, obtido, c.nuvem)
		}
	}
}

// Regra: só a letra do disco local; pasta de rede e caminho sem letra não têm disco para olhar.
func TestLetraDoDisco(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		`C:\ProgramData\FinancasLocal\dados`: "C:",
		`d:\financas\dados`:                  "D:",
		`\\servidor\pasta\dados`:             "",
		"/home/ana/dados":                    "",
		"1:\\x":                              "",
		"C":                                  "",
		"":                                   "",
	}
	for caminho, esperado := range casos {
		if obtido := LetraDoDisco(caminho); obtido != esperado {
			t.Errorf("%q: %q, esperado %q", caminho, obtido, esperado)
		}
	}
}

// Regra: só o 2 ("desligado") é sem criptografia; ligado, de passagem, suspenso ou sem resposta não avisa.
func TestBitLockerDesligado(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{"2\r\n": true, "2": true, "1\r\n": false, "3": false, "4": false, "5": false, "6": false,
		"8": false, "": false, "\r\n": false, "0": false, "12": false}
	for saida, esperado := range casos {
		if obtido := BitLockerDesligado(saida); obtido != esperado {
			t.Errorf("%q: %v, esperado %v", saida, obtido, esperado)
		}
	}
}

// Regra: o dispositivo sem o subvolume do btrfs; o que não é dispositivo (tmpfs, rede, FUSE, ZFS) fica "".
func TestOrigemDoMonte(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		"/dev/nvme0n1p7[/home]\n":     "/dev/nvme0n1p7",
		"/dev/mapper/luks-abc[/root]": "/dev/mapper/luks-abc",
		"/dev/sda2\n":                 "/dev/sda2",
		"tmpfs\n":                     "",
		"servidor:/exportado\n":       "",
		"tanque/home\n":               "",
		"gocryptfs\n":                 "",
		"/dev/../etc/passwd":          "",
		"":                            "",
	}
	for saida, esperado := range casos {
		if obtido := OrigemDoMonte(saida); obtido != esperado {
			t.Errorf("%q: %q, esperado %q", saida, obtido, esperado)
		}
	}
}

// Regra: com um "crypt" em qualquer nível (LUKS direto ou por baixo do LVM), há criptografia; sem nenhum tipo, não dá
// para saber.
func TestTiposSemCriptografia(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{
		"part\ndisk\n":           true,
		"lvm\npart\ndisk\n":      true,
		"crypt\npart\ndisk\n":    false,
		"lvm\ncrypt\npart\ndisk": false,
		"crypt\nloop\n":          false,
		"":                       false,
		"\n":                     false,
	}
	for saida, esperado := range casos {
		if obtido := TiposSemCriptografia(saida); obtido != esperado {
			t.Errorf("%q: %v, esperado %v", saida, obtido, esperado)
		}
	}
}
