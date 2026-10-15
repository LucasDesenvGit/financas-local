# =====================================================================
# FinancasLocal - 3) Compilacao e publicacao no IIS
#
#  - Compila o front-end Angular e a API .NET (ferramentas portateis do D:)
#  - Publica em D:\FinancasLocal\publicado\api (front-end servido pela propria API)
#  - Cria certificado HTTPS local (autoassinado, confiavel apenas neste computador)
#  - Cria o site no IIS respondendo SOMENTE em https://localhost:<porta> (127.0.0.1)
#
# Rode de novo sempre que atualizar o codigo: a publicacao e feita sem perder dados.
#
# Uso (Windows PowerShell como Administrador):
#   D:\FinancasLocal\scripts\3-publicar.ps1
#   D:\FinancasLocal\scripts\3-publicar.ps1 -Porta 5443 -InstanciaSql ".\FINANCAS"
# =====================================================================
[CmdletBinding()]
param(
    [ValidateRange(1024, 65535)][int]$Porta = 5443,
    [string]$InstanciaSql = '.\FINANCAS',
    [string]$NomeSite = 'FinancasLocal',
    [string]$NomePool = 'FinancasLocal',
    [switch]$PularBuild
)

. "$PSScriptRoot\comum.ps1"
Testar-Administrador
Testar-WindowsPowerShell
Validar-InstanciaSql $InstanciaSql
Validar-NomeSimples $NomeSite 'Nome do site'
Validar-NomeSimples $NomePool 'Nome do pool'
Import-Module WebAdministration

$pastaFront = Join-Path $Raiz 'src\frontend'
$projetoApi = Join-Path $Raiz 'src\backend\Financas.Api\Financas.Api.csproj'
$preparo = Join-Path $PastaPublicado 'preparo'
$destino = Join-Path $PastaPublicado 'api'
$contaPool = "IIS APPPOOL\$NomePool"

# ------------------------------------------------------------------ Build
if (-not $PularBuild) {
    Configurar-AmbienteBuild

    Escrever-Etapa 'Compilando o front-end (Angular)'
    Push-Location $pastaFront
    try {
        & npm.cmd install --no-audit --no-fund
        if ($LASTEXITCODE -ne 0) { throw 'npm install falhou.' }
        & npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw 'Build do Angular falhou.' }
    } finally {
        Pop-Location
    }
    Escrever-Ok 'front-end compilado'

    Escrever-Etapa 'Compilando a API (.NET)'
    if (Test-Path $preparo) { Remove-Item $preparo -Recurse -Force }
    & dotnet.exe publish $projetoApi -c Release -o $preparo --nologo
    if ($LASTEXITCODE -ne 0) { throw 'dotnet publish falhou.' }

    $distFront = Join-Path $pastaFront 'dist\financas-web\browser'
    if (-not (Test-Path (Join-Path $distFront 'index.html'))) { throw "Saida do Angular nao encontrada em $distFront" }
    $wwwroot = Join-Path $preparo 'wwwroot'
    Garantir-Pasta $wwwroot
    Copy-Item (Join-Path $distFront '*') $wwwroot -Recurse -Force
    Escrever-Ok 'API compilada e front-end incorporado'
} elseif (-not (Test-Path $preparo)) {
    throw 'Nao ha build anterior em publicado\preparo. Rode sem -PularBuild.'
}

# ------------------------------------------------------------------ Configuracao de producao
Escrever-Etapa 'Gerando appsettings.Production.json'
$nivel = $null
try {
    $conexao = Abrir-ConexaoSql $InstanciaSql
    try { $nivel = Executar-Escalar $conexao "SELECT compatibility_level FROM sys.databases WHERE name = N'FinancasLocal'" }
    finally { $conexao.Dispose() }
} catch {
    Escrever-Aviso "Nao foi possivel consultar o banco ($($_.Exception.Message)). Rode o 2-criar-banco.ps1 antes."
}
$config = [ordered]@{
    ConnectionStrings = [ordered]@{
        Financas = "Server=$InstanciaSql;Database=FinancasLocal;Integrated Security=True;Encrypt=True;TrustServerCertificate=True;Application Name=Financas.Api"
    }
    Seguranca = [ordered]@{ PastaChaves = $PastaChaves }
}
if ($nivel -and $nivel -isnot [DBNull]) { $config['Banco'] = [ordered]@{ NivelCompatibilidade = [int]$nivel } }
$json = $config | ConvertTo-Json -Depth 5
[IO.File]::WriteAllText((Join-Path $preparo 'appsettings.Production.json'), $json, (New-Object Text.UTF8Encoding($false)))
Escrever-Ok 'configuracao gerada'

# ------------------------------------------------------------------ Pool e permissoes
Escrever-Etapa 'Configurando pool de aplicativos e permissoes'
Garantir-PoolAplicativo $NomePool
Garantir-Pasta $destino
Definir-PermissaoRestrita $PastaChaves @("$($contaPool):(OI)(CI)M")
Conceder-Permissao $Dados "$($contaPool):(X)"   # apenas atravessar a pasta ate as chaves
Escrever-Ok "chaves de criptografia acessiveis somente por Administradores, SYSTEM e $contaPool"

# ------------------------------------------------------------------ Copia dos arquivos
Escrever-Etapa "Publicando em $destino"
$offline = Join-Path $destino 'app_offline.htm'
if (Test-Path (Join-Path $destino 'Financas.Api.dll')) {
    Set-Content -Path $offline -Value '<!doctype html><meta charset="utf-8"><title>Atualizando</title><p>Atualizando o sistema, aguarde alguns segundos...</p>' -Encoding UTF8
    Start-Sleep -Seconds 3
}
& robocopy.exe $preparo $destino /MIR /XF app_offline.htm /XD logs /NFL /NDL /NJH /NJS /NP | Out-Null
if ($LASTEXITCODE -ge 8) { throw "Falha ao copiar os arquivos (robocopy $LASTEXITCODE)." }
& icacls.exe $destino '/grant' "$($contaPool):(OI)(CI)RX" | Out-Null
if (Test-Path $offline) { Remove-Item $offline -Force }
Escrever-Ok 'arquivos publicados'

# ------------------------------------------------------------------ Certificado HTTPS
Escrever-Etapa 'Certificado HTTPS local'
$cert = Get-ChildItem Cert:\LocalMachine\My |
    Where-Object { $_.FriendlyName -eq 'FinancasLocal' -and $_.NotAfter -gt (Get-Date).AddDays(30) -and $_.HasPrivateKey } |
    Sort-Object NotAfter -Descending | Select-Object -First 1
if (-not $cert) {
    $cert = New-SelfSignedCertificate -DnsName 'localhost' -FriendlyName 'FinancasLocal' `
        -CertStoreLocation 'Cert:\LocalMachine\My' -NotAfter (Get-Date).AddYears(3) `
        -KeyAlgorithm RSA -KeyLength 2048 -HashAlgorithm SHA256 -KeyExportPolicy NonExportable `
        -TextExtension @('2.5.29.37={text}1.3.6.1.5.5.7.3.1', '2.5.29.19={text}false')
    Escrever-Ok "certificado criado (valido ate $($cert.NotAfter.ToString('dd/MM/yyyy')))"
}
if (-not (Test-Path "Cert:\LocalMachine\Root\$($cert.Thumbprint)")) {
    # Confia no certificado somente neste computador (apenas a parte publica e copiada).
    $cer = Join-Path $env:TEMP 'FinancasLocal.cer'
    Export-Certificate -Cert $cert -FilePath $cer | Out-Null
    Import-Certificate -FilePath $cer -CertStoreLocation 'Cert:\LocalMachine\Root' | Out-Null
    Remove-Item $cer -Force
    Escrever-Ok 'certificado marcado como confiavel neste computador'
} else {
    Escrever-Ok 'certificado existente reaproveitado'
}

# ------------------------------------------------------------------ Site no IIS
Escrever-Etapa "Configurando o site '$NomeSite' no IIS"
$caminhoSite = "IIS:\Sites\$NomeSite"
if (-not (Test-Path $caminhoSite)) {
    New-Website -Name $NomeSite -PhysicalPath $destino -ApplicationPool $NomePool -IPAddress '127.0.0.1' -Port $Porta -Ssl | Out-Null
    Escrever-Ok 'site criado'
} else {
    Set-ItemProperty $caminhoSite -Name physicalPath -Value $destino
    Set-ItemProperty $caminhoSite -Name applicationPool -Value $NomePool
}
# Somente HTTPS no loopback (inacessivel pela rede).
# "localhost" resolve primeiro para o IPv6 ::1, entao o certificado precisa estar ligado em 127.0.0.1 E em [::1].
$ipsLoopback = @('127.0.0.1', '[::1]')
$permitidas = $ipsLoopback | ForEach-Object { "$($_):$($Porta):" }
Get-WebBinding -Name $NomeSite | Where-Object { $_.protocol -ne 'https' -or $permitidas -notcontains $_.bindingInformation } |
    ForEach-Object { Remove-WebBinding -Name $NomeSite -BindingInformation $_.bindingInformation -Protocol $_.protocol }
foreach ($ip in $ipsLoopback) {
    $existe = Get-WebBinding -Name $NomeSite -Protocol https | Where-Object { $_.bindingInformation -eq "$($ip):$($Porta):" }
    if (-not $existe) { New-WebBinding -Name $NomeSite -Protocol https -IPAddress $ip -Port $Porta | Out-Null }
    & cmd.exe /c "netsh http delete sslcert ipport=$($ip):$Porta >nul 2>&1"
}
foreach ($ligacao in @(Get-WebBinding -Name $NomeSite -Protocol https)) {
    $ligacao.AddSslCertificate($cert.Thumbprint, 'My')
}
Set-ItemProperty $caminhoSite -Name applicationDefaults.preloadEnabled -Value $true
Set-ItemProperty $caminhoSite -Name logFile.directory -Value (Join-Path $Dados 'logs-iis')
Garantir-Pasta (Join-Path $Dados 'logs-iis')
Escrever-Ok "https://localhost:$Porta (somente loopback: 127.0.0.1 e ::1)"

Restart-WebAppPool -Name $NomePool -ErrorAction SilentlyContinue
Start-Website -Name $NomeSite -ErrorAction SilentlyContinue

# ------------------------------------------------------------------ Teste
Escrever-Etapa 'Testando'
Start-Sleep -Seconds 3
try {
    $resp = Invoke-WebRequest -Uri "https://localhost:$Porta/api/auth/estado" -UseBasicParsing -TimeoutSec 60
    Escrever-Ok "API respondeu ($($resp.StatusCode)): $($resp.Content)"
} catch {
    Escrever-Aviso "A API nao respondeu: $($_.Exception.Message)"
    Escrever-Aviso 'Veja o Visualizador de Eventos > Logs do Windows > Aplicativo (origem "IIS AspNetCore Module V2" e ".NET Runtime").'
}

Write-Host ''
Write-Host "Pronto! Abra https://localhost:$Porta no navegador." -ForegroundColor Green
