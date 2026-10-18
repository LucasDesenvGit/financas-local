<#
    Gera o FinancasLocal-<versao>.msi (#119, ADR 0029). Usado pelo workflow .github/workflows/versao.yml e, se quiser,
    no seu computador (precisa do .NET 10 SDK, do Node.js e do WiX 5: dotnet tool install --global wix --version 5.0.2;
    wix extension add -g WixToolset.UI.wixext/5.0.2 WixToolset.Util.wixext/5.0.2).

    .\instalador\gerar-msi.ps1                      # versao do Directory.Build.props
    .\instalador\gerar-msi.ps1 -Versao 2.1.1        # outra versao (ex.: testar a atualizacao)
    .\instalador\gerar-msi.ps1 -PularFront          # front-end ja compilado em src\frontend\dist
    .\instalador\gerar-msi.ps1 -SoPacote            # reaproveita a pasta publicada e so gera o .msi

    Saida: saida-instalador\FinancasLocal-<versao>.msi e saida-instalador\SHA256SUMS.txt.
#>
param(
    [string]$Versao = "",
    [switch]$PularFront,
    [switch]$SoPacote,
    [string]$Saida = ""
)

$ErrorActionPreference = "Stop"
$Raiz = Split-Path -Parent $PSScriptRoot
if (-not $Saida) { $Saida = Join-Path $Raiz "saida-instalador" }
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
Etapa "Financas Local $Versao"

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

    # API e gerenciador na mesma pasta, self-contained: o computador nao precisa do .NET instalado.
    # Os dois usam o mesmo .NET 10, entao os arquivos do runtime em comum sao os mesmos.
    foreach ($projeto in "src\backend\Financas.Api\Financas.Api.csproj", "src\gerenciador\Financas.Gerenciador\Financas.Gerenciador.csproj") {
        Etapa "Publicando $projeto"
        Executar "dotnet" @("publish", (Join-Path $Raiz $projeto), "-c", "Release", "-r", "win-x64", "--self-contained", "true",
            "-p:Version=$Versao", "-o", $Publicado, "--nologo")
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
    Copy-Item -LiteralPath (Join-Path $Raiz "src\gerenciador\Financas.Gerenciador\icone.ico") -Destination $pastaInstalador -Force

    # Nao vao para o computador: simbolos de depuracao e o web.config do IIS (o site roda direto pelo Kestrel).
    Get-ChildItem -LiteralPath $Publicado -Recurse -Include *.pdb, web.config | Remove-Item -Force
}

Etapa "Gerando o .msi (WiX)"
$msi = Join-Path $Saida "FinancasLocal-$Versao.msi"
Executar "wix" @("build", (Join-Path $PSScriptRoot "FinancasLocal.wxs"), "-arch", "x64", "-culture", "pt-BR",
    "-ext", "WixToolset.UI.wixext", "-ext", "WixToolset.Util.wixext",
    "-d", "Versao=$Versao", "-bindpath", "app=$Publicado", "-o", $msi)

$hash = (Get-FileHash -LiteralPath $msi -Algorithm SHA256).Hash.ToLowerInvariant()
$somas = Join-Path $Saida "SHA256SUMS.txt"
[IO.File]::WriteAllText($somas, "$hash  $(Split-Path -Leaf $msi)`n", (New-Object Text.UTF8Encoding $false))
$tamanho = [math]::Round((Get-Item -LiteralPath $msi).Length / 1MB, 1)
Write-Host "Pronto: $msi ($tamanho MB)" -ForegroundColor Green
Write-Host "SHA256: $hash"
