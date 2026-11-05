package desenvolvimento

import (
	"path"
	"strings"
)

// O "Atualizar o gerenciador e reabrir" (#430), como o AtualizarGerenciadorEReabrir do C#: o gerenciador não troca o
// próprio executável aberto. Ele deixa um comando rodando à parte, que espera ele fechar, roda o instalador do
// gerenciador do clone e o abre de novo. Os comandos ficam aqui, puros e com o separador de cada sistema (e não o do
// filepath, que depende de onde roda), para o teste valer nos dois.

// ComandoReabrirWindows é o -Command do PowerShell, numa janela que mostra o andamento; se o instalador falhar, a
// janela fica aberta com o aviso (o mesmo texto do FormPrincipal.cs).
func ComandoReabrirWindows(raiz, executavel string) string {
	script := strings.TrimRight(raiz, `\/`) + `\scripts\windows\instalar-gerenciador.ps1`
	return "Start-Sleep -Seconds 2; & " + literalPowerShell(script) + "; if ($LASTEXITCODE -eq 0) { Start-Process " +
		literalPowerShell(executavel) + " } else { Read-Host 'Nao deu para atualizar o gerenciador. Enter para fechar' }"
}

// ComandoReabrirLinux é o bash -c, num processo à parte (setsid) que continua depois de o gerenciador fechar, como o
// JanelaPrincipal.axaml.cs. O gerenciador reaberto é o do mesmo caminho: o em Go, que o script compila no lugar.
func ComandoReabrirLinux(raiz, executavel string) string {
	script := path.Join(raiz, "scripts", "linux", "instalar-gerenciador.sh")
	return "sleep 2; bash " + aspasShell(script) + " && exec " + aspasShell(executavel)
}

// literalPowerShell: texto entre aspas simples, com a aspa simples dobrada (o Literal do C#).
func literalPowerShell(texto string) string { return "'" + strings.ReplaceAll(texto, "'", "''") + "'" }

// aspasShell: texto entre aspas simples do bash, com a aspa simples fechada, escapada e reaberta.
func aspasShell(texto string) string { return "'" + strings.ReplaceAll(texto, "'", `'\''`) + "'" }
