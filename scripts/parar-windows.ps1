<#
.SYNOPSIS
    Para tudo o que o FinancasLocal deixou rodando no Windows, sem bash.

.DESCRIPTION
    Equivalente, no Windows, a scripts/parar.sh (que repassa para este script quando roda no Git Bash):
      1) Servico do Windows "FinancasLocal", que hospeda a API e o front publicado
      2) API rodando fora do servico (dotnet run, Visual Studio, publicado\api na mao)
      3) Front em desenvolvimento (npm start / ng serve e seus processos filhos)
      4) Banco: SQLite nao e servico, e so o arquivo dados\sql\FinancasLocal.db. O script apenas
         confere se algum processo ainda esta com ele aberto.

    Mexer no servico precisa de Administrador: o parar-windows.cmd, na raiz, pede a elevacao.
    Para subir de novo: instalar-windows.cmd, ou (como Administrador)
      sc.exe config FinancasLocal start= auto ; sc.exe start FinancasLocal

.EXAMPLE
    .\parar-windows.ps1
    .\parar-windows.ps1 -Desativar      # tambem tira o servico da inicializacao automatica
    .\parar-windows.ps1 -Compiladores   # tambem encerra os servidores de build do .NET
#>
[CmdletBinding()]
param(
    # Muda o servico para inicio manual: nao sobe sozinho no proximo boot.
    [switch]$Desativar,
    # Encerra os servidores de build do .NET (MSBuild, VBCSCompiler, Razor).
    [switch]$Compiladores
)

$ErrorActionPreference = "Stop"

$Raiz = Split-Path -Parent $PSScriptRoot
$Ferramentas = Join-Path $Raiz "ferramentas"
$PastaSql = Join-Path (Join-Path $Raiz "dados") "sql"
$NomeServico = "FinancasLocal"

function Etapa([string]$texto) { Write-Host ""; Write-Host "==> $texto" -ForegroundColor Cyan }
function Ok([string]$texto) { Write-Host "    OK  $texto" -ForegroundColor Green }
function Aviso([string]$texto) { Write-Host "    !!  $texto" -ForegroundColor Yellow }

$ehAdministrador = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)

# ---------------------------------------------------------------- 1. Servico do Windows
Etapa "Servico do Windows"
$servico = Get-Service -Name $NomeServico -ErrorAction SilentlyContinue
if (-not $servico) {
    Ok "servico $NomeServico nao instalado"
} elseif (-not $ehAdministrador) {
    Aviso "Sem permissao de Administrador para parar o servico. Use o parar-windows.cmd (pede a permissao)."
} else {
    if ($servico.Status -eq "Stopped") {
        Ok "servico $NomeServico ja estava parado"
    } else {
        Stop-Service -Name $NomeServico -Force
        $servico.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
        Ok "servico $NomeServico parado"
    }
    if ($Desativar) {
        Set-Service -Name $NomeServico -StartupType Manual
        Ok "removido da inicializacao automatica (inicio manual)"
    }
}

# ---------------------------------------------------------------- 2 e 3. Processos fora do servico
# Filtros restritos ao projeto, para nao derrubar outros apps .NET/Node da maquina.
Etapa "Processos da API e do front fora do servico"
$pastaFront = Join-Path $Raiz "src\frontend\node_modules"

function Parar-Processos([string]$descricao, [scriptblock]$filtro) {
    $alvos = @(Get-CimInstance Win32_Process |
        Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and (& $filtro $_.CommandLine) })
    if ($alvos.Count -eq 0) { Ok "${descricao}: nada rodando"; return }
    $falhas = 0
    foreach ($p in $alvos) {
        try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop }
        catch { if (Get-Process -Id $p.ProcessId -ErrorAction SilentlyContinue) { $falhas++ } }
    }
    if ($falhas -gt 0) { Aviso "${descricao}: $falhas processo(s) nao puderam ser encerrados (rode como Administrador)" }
    else { Ok "${descricao}: $($alvos.Count) processo(s) encerrado(s)" }
}

Parar-Processos "API" { param($linha) $linha -match 'Financas\.Api(\.dll|\.exe)?("|\s|$)' }
Parar-Processos "Front (ng serve)" { param($linha) $linha.IndexOf($pastaFront, [StringComparison]::OrdinalIgnoreCase) -ge 0 }

# ---------------------------------------------------------------- Servidores de build (opcional)
if ($Compiladores) {
    Etapa "Servidores de build do .NET"
    $dotnet = Join-Path $Ferramentas "dotnet\dotnet.exe"
    if (-not (Test-Path $dotnet)) {
        $comando = Get-Command dotnet -ErrorAction SilentlyContinue
        $dotnet = if ($comando) { $comando.Source } else { $null }
    }
    if ($dotnet) {
        & $dotnet build-server shutdown *> $null
        Ok "servidores de build encerrados (MSBuild, VBCSCompiler, Razor)"
    } else {
        Aviso "dotnet nao encontrado; pulando."
    }
}

# ---------------------------------------------------------------- 4. Banco (SQLite)
Etapa "Banco (SQLite)"
$banco = Join-Path $PastaSql "FinancasLocal.db"
if (-not (Test-Path $banco)) {
    Ok "banco ainda nao criado"
} else {
    # Abrir sem compartilhamento so funciona se ninguem mais estiver com o arquivo aberto.
    try {
        $arquivo = [IO.File]::Open($banco, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
        $arquivo.Close()
        Ok "nenhum processo com o banco aberto (SQLite nao tem servico proprio)"
    } catch {
        Aviso "Ainda ha processo com o banco aberto (ex.: DB Browser, sqlite3, IDE). Feche-o antes de copiar ou restaurar o .db."
    }
}

Write-Host ""
Ok "Pronto."
