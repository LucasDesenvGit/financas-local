<#
    Gera o FinancasLocal-<versao>.msi (#119, ADR 0029). Usado pelo workflow .github/workflows/versao.yml e, se quiser,
    no seu computador. Precisa do Go da versao do src\backend-go\go.mod e do src\gerenciador-go\go.mod, do
    Node.js e do WiX 5.0.2, pelo
    wix-cli-x64.msi do GitHub Releases do WiX, que ja traz as extensoes UI e Util e nao pede o .NET, #446:
    https://github.com/wixtoolset/wix/releases/tag/v5.0.2; confira o SHA-256 do WIX_SHA256 do versao.yml).

    .\instalador\gerar-msi.ps1                      # versao do Directory.Build.props
    .\instalador\gerar-msi.ps1 -Versao 2.1.1        # outra versao (ex.: testar a atualizacao)
    .\instalador\gerar-msi.ps1 -PularFront          # front-end ja compilado em src\frontend\dist
    .\instalador\gerar-msi.ps1 -SoPacote            # reaproveita a pasta publicada e so gera o .msi
    .\instalador\gerar-msi.ps1 -Revisao 57          # versao de desenvolvimento: X.Y.Z-dev.57 no site e no
                                                    # gerenciador, X.Y.Z.57 no .msi (a CI usa no develop)

    Saida: saida-instalador\FinancasLocal-<versao>.msi (ou -<versao>-dev.<revisao>.msi) e saida-instalador\SHA256SUMS.txt.
    O .msi troca qualquer instalacao da mesma versao X.Y.Z (AllowSameVersionUpgrades): a de desenvolvimento e a do
    Release substituem uma a outra sem ficarem as duas instaladas.
#>
param(
    [string]$Versao = "",
    [switch]$PularFront,
    [switch]$SoPacote,
    [string]$Saida = "",
    [int]$Revisao = 0
)

$ErrorActionPreference = "Stop"
$Raiz = Split-Path -Parent $PSScriptRoot
if (-not $Saida) { $Saida = Join-Path $Raiz "saida-instalador" }
# Caminho absoluto: o WiX resolve o -bindpath relativo a partir da pasta do .wxs, e nao da pasta atual.
if (-not [IO.Path]::IsPathRooted($Saida)) { $Saida = Join-Path (Get-Location).Path $Saida }
$Saida = [IO.Path]::GetFullPath($Saida)
$Publicado = Join-Path $Saida "app"

function Etapa([string]$texto) { Write-Host "==> $texto" -ForegroundColor Cyan }
function Executar([string]$programa, [string[]]$argumentos) {
    & $programa @argumentos
    if ($LASTEXITCODE -ne 0) { throw "$programa falhou ($LASTEXITCODE): $($argumentos -join ' ')" }
}

if (-not $Versao) {
    $props = [xml](Get-Content -LiteralPath (Join-Path $Raiz "Directory.Build.props") -Raw)
    $Versao = ($props.Project.PropertyGroup | Where-Object { $_.Version } | Select-Object -First 1).Version
}
if ($Versao -notmatch '^\d+\.\d+\.\d+$') { throw "Versao invalida: '$Versao' (use X.Y.Z)." }
# O Windows Installer so aceita numeros ate 65535 em cada parte e compara so X.Y.Z (a revisao so identifica o pacote).
if ($Revisao -lt 0 -or $Revisao -gt 65535) { throw "Revisao invalida: $Revisao (de 1 a 65535)." }
$Rotulo = if ($Revisao) { "$Versao-dev.$Revisao" } else { $Versao }
$VersaoMsi = if ($Revisao) { "$Versao.$Revisao" } else { $Versao }
Etapa "Financas Local $Rotulo"

if (-not $SoPacote) {
    if (Test-Path -LiteralPath $Publicado) { Remove-Item -LiteralPath $Publicado -Recurse -Force }
    New-Item -ItemType Directory -Path $Publicado -Force | Out-Null

    $front = Join-Path $Raiz "src\frontend"
    if (-not $PularFront) {
        Etapa "Front-end (Angular)"
        Push-Location $front
        try {
            Executar "npm" @("ci", "--no-audit", "--no-fund")
            Executar "npm" @("run", "build")
        } finally { Pop-Location }
    }
    $dist = Join-Path $front "dist\financas-web\browser"
    if (-not (Test-Path -LiteralPath (Join-Path $dist "index.html"))) { throw "Front-end compilado nao encontrado em $dist." }

    # #86: sem a API em .NET. O Go le o appsettings.json dele, o Production (abaixo) e o configuracao.json da
    # pasta de dados.
    Copy-Item -LiteralPath (Join-Path $Raiz "src\backend-go\appsettings.json") -Destination $Publicado -Force

    # #277 (ADR 0052): o gerenciador e o em Go (src\gerenciador-go), um .exe so, no lugar do C# self-contained. Mantem o
    # nome Financas.Gerenciador.exe: os atalhos e a entrada "Iniciar com o Windows" de quem atualiza continuam valendo.
    # Como o README do modulo: sem cgo, sem F12 nem menu do botao direito (-tags production), sem console (windowsgui).
    Etapa "Compilando o gerenciador em Go"
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go nao encontrado no PATH (veja src\gerenciador-go\go.mod para a versao)." }
    $env:CGO_ENABLED = "0"; $env:GOOS = "windows"; $env:GOARCH = "amd64"
    Push-Location (Join-Path $Raiz "src\gerenciador-go")
    try {
        Executar "go" @("build", "-tags", "production", "-trimpath", "-ldflags", "-s -w -H=windowsgui -X main.versao=$Rotulo",
            "-o", (Join-Path $Publicado "Financas.Gerenciador.exe"), ".")
    } finally {
        Pop-Location
        Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    }

    # O back-end em Go e o site e o servico (desde a #85; sem o .NET desde a #86). Sem cgo
    # (o SQLite e o modernc.org/sqlite, em Go puro), para Windows x64. A versao entra no executavel pelo -ldflags.
    Etapa "Compilando o back-end em Go"
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go nao encontrado no PATH (veja src\backend-go\go.mod para a versao)." }
    $env:CGO_ENABLED = "0"; $env:GOOS = "windows"; $env:GOARCH = "amd64"
    Push-Location (Join-Path $Raiz "src\backend-go")
    try {
        Executar "go" @("build", "-trimpath", "-ldflags",
            "-s -w -X github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao.Atual=$Rotulo",
            "-o", (Join-Path $Publicado "financas-go.exe"), "./cmd/financas")
    } finally {
        Pop-Location
        Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    }

    Etapa "Montando a pasta do instalador"
    $wwwroot = Join-Path $Publicado "wwwroot"
    New-Item -ItemType Directory -Path $wwwroot -Force | Out-Null
    Copy-Item -Path (Join-Path $dist "*") -Destination $wwwroot -Recurse -Force

    # Dados fora da pasta do programa (atualizar e desinstalar nao mexem neles). O resto vem do configuracao.json
    # que o configurar.ps1 grava na pasta de dados (certificado e senha dele).
    $producao = [ordered]@{ Dados = [ordered]@{ Pasta = "%ProgramData%\FinancasLocal\dados" } }
    [IO.File]::WriteAllText((Join-Path $Publicado "appsettings.Production.json"), (($producao | ConvertTo-Json -Depth 5) + "`n"),
        (New-Object Text.UTF8Encoding $false))

    $pastaInstalador = Join-Path $Publicado "instalador"
    New-Item -ItemType Directory -Path $pastaInstalador -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "configurar.ps1") -Destination $pastaInstalador -Force
    Copy-Item -LiteralPath (Join-Path $Raiz "src\gerenciador-go\icone.ico") -Destination $pastaInstalador -Force

}

Etapa "Gerando o .msi (WiX)"
$msi = Join-Path $Saida "FinancasLocal-$Rotulo.msi"
Executar "wix" @("build", (Join-Path $PSScriptRoot "FinancasLocal.wxs"), "-arch", "x64", "-culture", "pt-BR",
    "-ext", "WixToolset.UI.wixext", "-ext", "WixToolset.Util.wixext",
    "-d", "Versao=$VersaoMsi", "-bindpath", "app=$Publicado", "-o", $msi)

$hash = (Get-FileHash -LiteralPath $msi -Algorithm SHA256).Hash.ToLowerInvariant()
$somas = Join-Path $Saida "SHA256SUMS.txt"
[IO.File]::WriteAllText($somas, "$hash  $(Split-Path -Leaf $msi)`n", (New-Object Text.UTF8Encoding $false))
$tamanho = [math]::Round((Get-Item -LiteralPath $msi).Length / 1MB, 1)
Write-Host "Pronto: $msi ($tamanho MB)" -ForegroundColor Green
Write-Host "SHA256: $hash"
