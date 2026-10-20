<#
.SYNOPSIS
    Para tudo o que o FinancasLocal deixou rodando no Windows, sem bash.

.DESCRIPTION
    Equivalente, no Windows, a scripts/geral/parar.sh (que repassa para este script quando roda no Git Bash):
      1) Servico do Windows "FinancasLocal", que hospeda a API e o front publicado
      2) API rodando fora do servico (dotnet run, Visual Studio, publicado\api na mao)
      3) Front em desenvolvimento (npm start / ng serve e seus processos filhos)
      4) Banco: SQLite nao e servico, e so o arquivo dados\sql\FinancasLocal.db. O script apenas
         confere se algum processo ainda esta com ele aberto.

    Mexer no servico precisa de Administrador: o parar-windows.cmd, na raiz, pede a elevacao.
    Sai com codigo 1 se algo ficou rodando (ou o banco continua aberto), para quem automatiza saber.
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

$Raiz = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Ferramentas = Join-Path $Raiz "ferramentas"
$PastaSql = Join-Path (Join-Path $Raiz "dados") "sql"
$NomeServico = "FinancasLocal"
$script:Problemas = 0

function Etapa([string]$texto) { Write-Host ""; Write-Host "==> $texto" -ForegroundColor Cyan }
function Ok([string]$texto) { Write-Host "    OK  $texto" -ForegroundColor Green }
function Aviso([string]$texto) { Write-Host "    !!  $texto" -ForegroundColor Yellow }
function Problema([string]$texto) { $script:Problemas++; Aviso $texto }

$ehAdministrador = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)

# ---------------------------------------------------------------- 1. Servico do Windows
Etapa "Servico do Windows"
$servico = Get-Service -Name $NomeServico -ErrorAction SilentlyContinue
if (-not $servico) {
    Ok "servico $NomeServico nao instalado"
} elseif (-not $ehAdministrador) {
    if ($servico.Status -eq "Stopped") { Ok "servico $NomeServico ja estava parado" }
    else { Problema "Sem permissao de Administrador para parar o servico. Use o parar-windows.cmd (pede a permissao)." }
} else {
    if ($servico.Status -eq "Stopped") {
        Ok "servico $NomeServico ja estava parado"
    } else {
        try {
            Stop-Service -Name $NomeServico -Force
            $servico.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
            Ok "servico $NomeServico parado"
        } catch {
            Problema "O servico $NomeServico nao parou em 30 s ($($_.Exception.Message)). Tente de novo ou reinicie o computador."
        }
    }
    if ($Desativar) {
        Set-Service -Name $NomeServico -StartupType Manual
        Ok "removido da inicializacao automatica (inicio manual)"
    }
}

# ---------------------------------------------------------------- 2 e 3. Processos fora do servico
# So processos DESTE projeto: a linha de comando ou o executavel tem de estar dentro da pasta do projeto,
# com barra invertida (D:\FinancasLocal), normal (D:/FinancasLocal) ou no formato do Git Bash (/d/FinancasLocal).
# Assim um "Financas.Api" de outra pasta, ou um "dotnet build" com caminho relativo, nao e derrubado.
Etapa "Processos da API e do front fora do servico"
$raizMsys = "\" + $Raiz.Substring(0, 1).ToLowerInvariant() + $Raiz.Substring(2)   # D:\FinancasLocal -> \d\FinancasLocal

function Texto-Processo($p) { return (("" + $p.CommandLine + " " + $p.ExecutablePath) -replace '/', '\') }

function No-Projeto([string]$texto, [string]$subpasta) {
    foreach ($base in @($Raiz, $raizMsys)) {
        # Barra no fim: D:\FinancasLocal\ nao pode casar com D:\FinancasLocal-dev\.
        $pasta = (Join-Path $base $subpasta).TrimEnd('\') + '\'
        if ($texto.IndexOf($pasta, [StringComparison]::OrdinalIgnoreCase) -ge 0) { return $true }
    }
    return $false
}

function Parar-Processos([string]$descricao, [scriptblock]$filtro) {
    $alvos = @(Get-CimInstance Win32_Process |
        Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and (& $filtro (Texto-Processo $_)) })
    if ($alvos.Count -eq 0) { Ok "${descricao}: nada rodando"; return }
    $falhas = 0
    foreach ($p in $alvos) {
        try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop }
        catch { if (Get-Process -Id $p.ProcessId -ErrorAction SilentlyContinue) { $falhas++ } }
    }
    if ($falhas -gt 0) { Problema "${descricao}: $falhas processo(s) nao puderam ser encerrados (rode como Administrador)" }
    else { Ok "${descricao}: $($alvos.Count) processo(s) encerrado(s)" }
}

Parar-Processos "API" { param($t) ($t -match 'Financas\.Api(\.dll|\.exe)?("|\s|$)') -and (No-Projeto $t "") }
Parar-Processos "Front (ng serve)" { param($t) No-Projeto $t "src\frontend\node_modules" }

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
    } catch [UnauthorizedAccessException] {
        Aviso "Sem permissao para conferir o banco (a pasta dados e do servico). Rode pelo parar-windows.cmd para conferir."
    } catch {
        Problema "Ainda ha processo com o banco aberto (ex.: DB Browser, sqlite3, IDE). Feche-o antes de copiar ou restaurar o .db."
    }
}

Write-Host ""
if ($script:Problemas -gt 0) {
    Aviso "Terminou com $($script:Problemas) pendencia(s): veja os avisos acima."
    exit 1
}
Ok "Pronto."
exit 0
