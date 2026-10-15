# =====================================================================
# FinancasLocal - funcoes compartilhadas pelos scripts de instalacao.
# Carregado com ". $PSScriptRoot\comum.ps1" (nao execute diretamente).
# =====================================================================

Set-StrictMode -Version 1.0
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # deixa o Invoke-WebRequest muito mais rapido no PowerShell 5.1

# TLS 1.2 obrigatorio para os servidores da Microsoft e do Node.js
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$script:Raiz          = Split-Path $PSScriptRoot -Parent
$script:Ferramentas   = Join-Path $Raiz 'ferramentas'
$script:Instaladores  = Join-Path $Raiz 'instaladores'
$script:Dados         = Join-Path $Raiz 'dados'
$script:PastaSql      = Join-Path $Dados 'sql'
$script:PastaChaves   = Join-Path $Dados 'chaves'
$script:PastaBackups  = Join-Path $Raiz 'backups'
$script:PastaPublicado = Join-Path $Raiz 'publicado'

# SIDs bem conhecidos (funcionam em Windows em qualquer idioma)
$script:SidAdministradores = '*S-1-5-32-544'
$script:SidSystem          = '*S-1-5-18'

function Escrever-Etapa([string]$texto) {
    Write-Host ''
    Write-Host "==> $texto" -ForegroundColor Cyan
}

function Escrever-Ok([string]$texto) { Write-Host "    OK  $texto" -ForegroundColor Green }
function Escrever-Aviso([string]$texto) { Write-Host "    !!  $texto" -ForegroundColor Yellow }

function Testar-Administrador {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($id)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Execute este script em um "Windows PowerShell" aberto como Administrador.'
    }
}

function Testar-WindowsPowerShell {
    if ($PSVersionTable.PSEdition -ne 'Desktop') {
        throw 'Use o "Windows PowerShell" (versao 5.1, que acompanha o Windows), nao o PowerShell 7. O modulo do IIS depende dele.'
    }
}

function Garantir-Pasta([string]$caminho) {
    if (-not (Test-Path $caminho)) { New-Item -ItemType Directory -Path $caminho -Force | Out-Null }
}

function Validar-NomeSimples([string]$valor, [string]$campo) {
    if ($valor -notmatch '^[A-Za-z0-9_\-]{1,64}$') { throw "$campo invalido: use apenas letras, numeros, - e _." }
}

function Validar-InstanciaSql([string]$valor) {
    if ($valor -notmatch '^[A-Za-z0-9_\.\-]+(\\[A-Za-z0-9_\$\-]+)?$') { throw "Nome de instancia SQL invalido: $valor" }
}

function Baixar-Arquivo([string]$url, [string]$destino) {
    if (-not $url.StartsWith('https://')) { throw "Somente downloads via HTTPS sao permitidos: $url" }
    Write-Host "    baixando $url"
    Invoke-WebRequest -Uri $url -OutFile $destino -UseBasicParsing
}

function Verificar-Sha([string]$arquivo, [string]$esperado, [string]$algoritmo) {
    $hash = (Get-FileHash -Path $arquivo -Algorithm $algoritmo).Hash
    if ($hash -ne $esperado.ToUpperInvariant()) {
        Remove-Item $arquivo -Force -ErrorAction SilentlyContinue
        throw "Hash $algoritmo nao confere para $(Split-Path $arquivo -Leaf). O arquivo foi apagado por seguranca."
    }
    Escrever-Ok "hash $algoritmo conferido: $(Split-Path $arquivo -Leaf)"
}

function Verificar-AssinaturaMicrosoft([string]$arquivo) {
    $assinatura = Get-AuthenticodeSignature -FilePath $arquivo
    $assinante = if ($assinatura.SignerCertificate) { $assinatura.SignerCertificate.Subject } else { '' }
    if ($assinatura.Status -ne 'Valid' -or $assinante -notmatch 'O=Microsoft Corporation') {
        Remove-Item $arquivo -Force -ErrorAction SilentlyContinue
        throw "Assinatura digital invalida ou nao emitida pela Microsoft: $(Split-Path $arquivo -Leaf) ($($assinatura.Status))."
    }
    Escrever-Ok "assinatura digital da Microsoft valida: $(Split-Path $arquivo -Leaf)"
}

function Definir-PermissaoRestrita([string]$pasta, [string[]]$concessoes) {
    # Remove heranca e deixa somente Administradores, SYSTEM e as contas informadas.
    Garantir-Pasta $pasta
    $argumentos = @($pasta, '/inheritance:r', '/grant:r', "$($SidAdministradores):(OI)(CI)F", '/grant:r', "$($SidSystem):(OI)(CI)F")
    foreach ($c in $concessoes) { $argumentos += @('/grant:r', $c) }
    & icacls.exe @argumentos | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Falha ao ajustar permissoes de $pasta" }
}

function Conceder-Permissao([string]$pasta, [string]$concessao) {
    Garantir-Pasta $pasta
    & icacls.exe $pasta '/grant' $concessao | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Falha ao conceder $concessao em $pasta" }
}

function Obter-ContaServicoSql([string]$instancia) {
    if ($instancia -match '\\(.+)$') { return "NT SERVICE\MSSQL`$$($Matches[1])" }
    return 'NT SERVICE\MSSQLSERVER'
}

function Garantir-PoolAplicativo([string]$nomePool) {
    Import-Module WebAdministration
    $caminho = "IIS:\AppPools\$nomePool"
    if (-not (Test-Path $caminho)) {
        New-WebAppPool -Name $nomePool | Out-Null
        Escrever-Ok "pool de aplicativos '$nomePool' criado"
    }
    Set-ItemProperty $caminho -Name managedRuntimeVersion -Value ''
    Set-ItemProperty $caminho -Name startMode -Value 'AlwaysRunning'
    Set-ItemProperty $caminho -Name processModel.identityType -Value 'ApplicationPoolIdentity'
    Set-ItemProperty $caminho -Name processModel.loadUserProfile -Value $true
    Set-ItemProperty $caminho -Name processModel.idleTimeout -Value ([TimeSpan]::Zero)
    Set-ItemProperty $caminho -Name enable32BitAppOnWin64 -Value $false
}

function Abrir-ConexaoSql([string]$instancia, [string]$banco = 'master') {
    $cs = "Server=$instancia;Database=$banco;Integrated Security=True;Encrypt=True;TrustServerCertificate=True;Application Name=FinancasLocal-Instalador"
    $conexao = New-Object System.Data.SqlClient.SqlConnection $cs
    $conexao.Open()
    return $conexao
}

function Executar-ScriptSql($conexao, [string]$texto) {
    # Divide o script nos separadores "GO" (linha sozinha), como o SSMS/sqlcmd.
    $lotes = [regex]::Split($texto, '^\s*GO\s*$', [Text.RegularExpressions.RegexOptions]::Multiline)
    foreach ($lote in $lotes) {
        if ([string]::IsNullOrWhiteSpace($lote)) { continue }
        $cmd = $conexao.CreateCommand()
        $cmd.CommandText = $lote
        $cmd.CommandTimeout = 300
        [void]$cmd.ExecuteNonQuery()
    }
}

function Executar-Escalar($conexao, [string]$sql) {
    $cmd = $conexao.CreateCommand()
    $cmd.CommandText = $sql
    return $cmd.ExecuteScalar()
}

function Configurar-AmbienteBuild {
    # Tudo o que o build baixa/gera fica no D:, nao no perfil do usuario no C:
    $dotnet = Join-Path $Ferramentas 'dotnet'
    $node = Join-Path $Ferramentas 'node'
    if (-not (Test-Path (Join-Path $dotnet 'dotnet.exe'))) { throw 'SDK do .NET nao encontrado. Rode antes o 1-instalar-ambiente.ps1.' }
    if (-not (Test-Path (Join-Path $node 'node.exe'))) { throw 'Node.js nao encontrado. Rode antes o 1-instalar-ambiente.ps1.' }
    $env:PATH = "$dotnet;$node;$env:PATH"
    $env:DOTNET_ROOT = $dotnet
    $env:DOTNET_CLI_HOME = Join-Path $Ferramentas 'dotnet-home'
    $env:NUGET_PACKAGES = Join-Path $Ferramentas 'nuget-pacotes'
    $env:DOTNET_CLI_TELEMETRY_OPTOUT = '1'
    $env:DOTNET_NOLOGO = '1'
    $env:DOTNET_SKIP_FIRST_TIME_EXPERIENCE = '1'
    $env:npm_config_cache = Join-Path $Ferramentas 'npm-cache'
    $env:npm_config_update_notifier = 'false'
    $env:NG_CLI_ANALYTICS = 'false'
}
