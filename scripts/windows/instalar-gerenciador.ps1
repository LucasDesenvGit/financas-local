<#
    Publica o Gerenciador do Financas Local numa instalacao feita pelos scripts (instalar-windows.cmd ou
    3-publicar-atualizar.sh) e cria os atalhos no Menu Iniciar e na area de trabalho do usuario atual. Nao precisa de
    administrador. Quem instalou pelo .msi ja tem o gerenciador (ADR 0028).

    .\scripts\windows\instalar-gerenciador.ps1                         # publica em publicado\gerenciador
    .\scripts\windows\instalar-gerenciador.ps1 -Destino D:\outra\pasta
    .\scripts\windows\instalar-gerenciador.ps1 -SemAtalhoNaAreaDeTrabalho

    O gerenciador acha o site pelo servico FinancasLocal (a pasta da API e o appsettings dela), entao funciona
    de qualquer pasta. Fica fora de publicado\api porque a publicacao espelha aquela pasta (robocopy /MIR).
    Rode de novo para atualizar o gerenciador; se ele estiver aberto, e fechado antes.
    Compativel com o Windows PowerShell 5.1 e sem acentos no arquivo.
#>
param([string]$Destino = "", [switch]$SemAtalhoNaAreaDeTrabalho)

$ErrorActionPreference = "Stop"
$Raiz = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $Destino) { $Destino = Join-Path $Raiz "publicado\gerenciador" }
$Projeto = Join-Path $Raiz "src\gerenciador\Financas.Gerenciador\Financas.Gerenciador.csproj"
$Nome = "Finan" + [char]0x00E7 + "as Local"

function Etapa([string]$texto) { Write-Host "==> $texto" -ForegroundColor Cyan }

# .NET: o portatil do projeto (ferramentas\dotnet), se houver; senao, o do sistema.
$dotnet = Join-Path $Raiz "ferramentas\dotnet\dotnet.exe"
if (Test-Path -LiteralPath $dotnet) {
    $env:DOTNET_ROOT = Split-Path -Parent $dotnet
    $env:DOTNET_CLI_HOME = Join-Path $Raiz "ferramentas\dotnet-home"
    $env:NUGET_PACKAGES = Join-Path $Raiz "ferramentas\nuget-pacotes"
} else {
    $comando = Get-Command dotnet -ErrorAction SilentlyContinue
    if (-not $comando) { throw "dotnet nao encontrado. Rode antes o instalar-windows.cmd (ele baixa o .NET em ferramentas\dotnet)." }
    $dotnet = $comando.Source
}
$env:DOTNET_NOLOGO = "1"; $env:DOTNET_CLI_TELEMETRY_OPTOUT = "1"

if (-not (Get-Service -Name "FinancasLocal" -ErrorAction SilentlyContinue)) {
    Write-Warning "O servico FinancasLocal nao esta registrado: o gerenciador vai dizer que o site nao esta instalado."
}

# Gerenciador aberto trava os arquivos: fecha so o que roda desta pasta.
$aberto = Get-Process -Name "Financas.Gerenciador" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -and $_.Path.StartsWith($Destino, [StringComparison]::OrdinalIgnoreCase) }
if ($aberto) {
    Etapa "Fechando o gerenciador aberto"
    $aberto | Stop-Process -Force
    Start-Sleep -Seconds 1
}

# Self-contained: o gerenciador nao depende de .NET instalado no Windows (o do sistema pode ser outra versao).
Etapa "Publicando o gerenciador em $Destino"
& $dotnet publish $Projeto -c Release -r win-x64 --self-contained true -o $Destino --nologo
if ($LASTEXITCODE -ne 0) { throw "dotnet publish falhou ($LASTEXITCODE)." }
Get-ChildItem -LiteralPath $Destino -Filter *.pdb -ErrorAction SilentlyContinue | Remove-Item -Force

$shell = New-Object -ComObject WScript.Shell
function Criar-Atalho([string]$pasta) {
    New-Item -ItemType Directory -Path $pasta -Force | Out-Null
    $atalho = $shell.CreateShortcut((Join-Path $pasta ("Gerenciador do " + $Nome + ".lnk")))
    $atalho.TargetPath = Join-Path $Destino "Financas.Gerenciador.exe"
    $atalho.WorkingDirectory = $Destino
    $atalho.IconLocation = (Join-Path $Destino "Financas.Gerenciador.exe") + ",0"
    $atalho.Description = "Ver se o site esta no ar, ligar, desligar e copiar o banco"
    $atalho.Save()
}

Etapa "Criando o atalho no Menu Iniciar"
Criar-Atalho (Join-Path ([Environment]::GetFolderPath("Programs")) $Nome)
if (-not $SemAtalhoNaAreaDeTrabalho) {
    Etapa "Criando o atalho na area de trabalho"
    Criar-Atalho ([Environment]::GetFolderPath("Desktop"))
}

Write-Host ("Pronto: Menu Iniciar > " + $Nome + " > Gerenciador do " + $Nome) -ForegroundColor Green
if (-not $SemAtalhoNaAreaDeTrabalho) { Write-Host ("e na area de trabalho: Gerenciador do " + $Nome) -ForegroundColor Green }
