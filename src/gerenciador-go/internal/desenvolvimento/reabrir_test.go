package desenvolvimento

import "testing"

// Regra: o comando do Windows espera o gerenciador fechar, roda o instalar-gerenciador.ps1 do clone e reabre; se
// falhar, segura a janela com o aviso do C#. Aspa simples no caminho é dobrada (#430).
func TestComandoReabrirWindows(t *testing.T) {
	casos := []struct{ nome, raiz, exe, quero string }{
		{"caminho comum", `C:\FinancasLocal`, `C:\FinancasLocal\publicado\gerenciador\Financas.Gerenciador.exe`,
			`Start-Sleep -Seconds 2; & 'C:\FinancasLocal\scripts\windows\instalar-gerenciador.ps1'; ` +
				`if ($LASTEXITCODE -eq 0) { Start-Process 'C:\FinancasLocal\publicado\gerenciador\Financas.Gerenciador.exe' } ` +
				`else { Read-Host 'Nao deu para atualizar o gerenciador. Enter para fechar' }`},
		{"barra no fim e aspa no nome", `D:\d'Ávila\fl\`, `D:\d'Ávila\fl\publicado\gerenciador\Financas.Gerenciador.exe`,
			`Start-Sleep -Seconds 2; & 'D:\d''Ávila\fl\scripts\windows\instalar-gerenciador.ps1'; ` +
				`if ($LASTEXITCODE -eq 0) { Start-Process 'D:\d''Ávila\fl\publicado\gerenciador\Financas.Gerenciador.exe' } ` +
				`else { Read-Host 'Nao deu para atualizar o gerenciador. Enter para fechar' }`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ComandoReabrirWindows(c.raiz, c.exe); got != c.quero {
				t.Errorf("\ngot  %s\nquero %s", got, c.quero)
			}
		})
	}
}

// Regra: o comando do Linux espera, roda o instalar-gerenciador.sh do clone e só reabre se ele der certo (&&); aspa
// simples no caminho fecha, escapa e reabre (#430).
func TestComandoReabrirLinux(t *testing.T) {
	casos := []struct{ nome, raiz, exe, quero string }{
		{"caminho comum", "/home/ana/financas-local", "/home/ana/financas-local/publicado/gerenciador-go/financas-gerenciador",
			"sleep 2; bash '/home/ana/financas-local/scripts/linux/instalar-gerenciador.sh' && " +
				"exec '/home/ana/financas-local/publicado/gerenciador-go/financas-gerenciador'"},
		{"aspa e espaço", "/home/d'avila/meu fl", "/home/d'avila/meu fl/publicado/gerenciador-go/financas-gerenciador",
			`sleep 2; bash '/home/d'\''avila/meu fl/scripts/linux/instalar-gerenciador.sh' && ` +
				`exec '/home/d'\''avila/meu fl/publicado/gerenciador-go/financas-gerenciador'`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ComandoReabrirLinux(c.raiz, c.exe); got != c.quero {
				t.Errorf("\ngot  %s\nquero %s", got, c.quero)
			}
		})
	}
}
