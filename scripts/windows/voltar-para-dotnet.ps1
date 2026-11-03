<#
    Volta o servico do Financas Local para o .NET (#85, ADR 0049), numa instalacao pelos scripts.

    Depois da troca (instalar-windows.ps1 -Backend go), o Go fica na frente. Este script poe o .NET de volta, sem
    restaurar o banco: os dois gravam o mesmo formato. O .NET continua publicado ao lado do Go ate a #86.

    O .NET nao le os segredos que o Go gravou ("v2:"): com eles, a sincronizacao falharia a cada ciclo com a tela
    dizendo "configurado". Por isso o script apaga o Client Secret da Pluggy e o token da brapi gravados pelo Go,
    como o MigrarSqlite faz, e avisa que e preciso informa-los de novo. Os outros dados ficam como estao.

    Num PowerShell como Administrador:
    .\scripts\windows\voltar-para-dotnet.ps1

    Compativel com o Windows PowerShell 5.1 e sem acentos no arquivo.
#>
$ErrorActionPreference = "Stop"
$Raiz = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$banco = Join-Path $Raiz "dados\sql\FinancasLocal.db"
if (-not (Test-Path -LiteralPath $banco)) { throw "Banco nao encontrado em $banco." }

$sqlite = Join-Path $Raiz "ferramentas\sqlite\sqlite3.exe"
if (-not (Test-Path -LiteralPath $sqlite)) {
    $comando = Get-Command sqlite3 -ErrorAction SilentlyContinue
    if (-not $comando) { throw "sqlite3 nao encontrado. Rode antes o instalar-windows.cmd." }
    $sqlite = $comando.Source
}

Write-Host "==> Segredos que so o Go le" -ForegroundColor Cyan
$sql = "UPDATE Configuracao SET Valor = NULL, Protegido = 0 WHERE Chave IN ('Pluggy.ClientSecret', 'Cotacao.BrapiToken') AND Valor LIKE 'v2:%'; SELECT changes();"
$apagados = & $sqlite -cmd ".timeout 15000" $banco $sql
if ($LASTEXITCODE -ne 0) { throw "sqlite3 falhou ao apagar os segredos do Go." }
if ([int]$apagados -gt 0) {
    Write-Warning "$apagados segredo(s) gravado(s) pelo Go foram apagados: o .NET nao os le."
    Write-Warning "Informe de novo em Configuracoes: o Client Secret da Pluggy (Open Finance) e o token da brapi (Investimentos)."
} else {
    Write-Host "nenhum segredo do Go a apagar"
}

Write-Host "==> Voltando o servico para o .NET" -ForegroundColor Cyan
& (Join-Path $PSScriptRoot "instalar-windows.ps1") -PularBuild -Backend dotnet
