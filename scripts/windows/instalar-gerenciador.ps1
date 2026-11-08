<#
    Compila o Gerenciador do Financas Local (o em Go, #430, ADR 0052) numa instalacao feita pelos scripts
    (instalar-windows.cmd ou 3-publicar-atualizar.sh) e cria os atalhos no Menu Iniciar e na area de trabalho do
    usuario atual. Nao precisa de administrador. Quem instalou pelo .msi ja tem o gerenciador (ADR 0029).

    .\scripts\windows\instalar-gerenciador.ps1                         # compila em publicado\gerenciador
    .\scripts\windows\instalar-gerenciador.ps1 -Destino D:\outra\pasta
    .\scripts\windows\instalar-gerenciador.ps1 -SemAtalhoNaAreaDeTrabalho

    O gerenciador acha o site pelo servico FinancasLocal (a pasta da API e o appsettings dela), entao funciona
    de qualquer pasta. Fica fora de publicado\api porque a publicacao espelha aquela pasta (robocopy /MIR).
    Rode de novo para atualizar o gerenciador; se ele estiver aberto, e fechado antes.
    Compativel com o Windows PowerShell 5.1 e sem acentos no arquivo.
#>
param([string]$Destino = "", [switch]$SemAtalhoNaAreaDeTrabalho)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "comum.ps1")
$Padrao = Join-Path $Raiz "publicado\gerenciador"
if (-not $Destino) { $Destino = $Padrao }
$Nome = "Finan" + [char]0x00E7 + "as Local"

# #430 (ADR 0052): o gerenciador em Go (src\gerenciador-go), compilado sem cgo. O Go e o portatil do projeto
# (ferramentas\go), se houver, com o mesmo ambiente do instalar-windows.ps1: modulos e cache em ferramentas\, nunca
# baixar outra versao do Go sozinho (GOTOOLCHAIN=local).
$go = Join-Path $Ferramentas "go\bin\go.exe"
if (-not (Test-Path -LiteralPath $go)) {
    $comando = Get-Command go -ErrorAction SilentlyContinue
    if (-not $comando) { throw "Go nao encontrado. Rode antes o instalar-windows.cmd (ele baixa o Go em ferramentas\go)." }
    $go = $comando.Source
}
$env:GOPATH = Join-Path $Ferramentas "go-caminho"
$env:GOCACHE = Join-Path $Ferramentas "go-cache"
$env:GOTOOLCHAIN = "local"
$env:GOFLAGS = "-modcacherw"
$env:CGO_ENABLED = "0"

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

# A publicacao em C# (self-contained, ~240 arquivos) sai da pasta padrao, que so este script escreve. Numa pasta
# escolhida com -Destino, nada e apagado: so o aviso.
if (Test-Path -LiteralPath (Join-Path $Destino "Financas.Gerenciador.dll")) {
    if ($Destino.TrimEnd('\') -ieq $Padrao) {
        Etapa "Tirando o gerenciador em C# de $Destino"
        Remove-Item -LiteralPath $Destino -Recurse -Force
    } else {
        Write-Warning "$Destino tem o gerenciador em C#: os arquivos dele ficam ao lado do novo (pode apaga-los)."
    }
}
New-Item -ItemType Directory -Path $Destino -Force | Out-Null

# Um .exe so, com o mesmo nome do em C#: os atalhos e a entrada "Iniciar com o Windows" continuam valendo. Como o
# README do modulo: sem F12 nem menu do botao direito (-tags production), sem console (windowsgui). A versao e a do
# Directory.Build.props, como a do site.
$props = [xml](Get-Content -LiteralPath (Join-Path $Raiz "Directory.Build.props") -Raw)
$versao = ($props.Project.PropertyGroup | Where-Object { $_.Version } | Select-Object -First 1).Version
Etapa "Compilando o gerenciador em $Destino"
Push-Location (Join-Path $Raiz "src\gerenciador-go")
try {
    & $go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.versao=$versao" -o (Join-Path $Destino "Financas.Gerenciador.exe") .
    if ($LASTEXITCODE -ne 0) { throw "go build falhou ($LASTEXITCODE)." }
} finally { Pop-Location }

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
