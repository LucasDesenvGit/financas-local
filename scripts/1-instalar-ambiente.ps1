# =====================================================================
# FinancasLocal - 1) Instalacao do ambiente
#
#  - Habilita o IIS (recurso do proprio Windows)
#  - .NET 10 Hosting Bundle (IIS) ............ dotnet.microsoft.com   (SHA-512 + assinatura Microsoft)
#  - .NET 10 SDK portatil em D:\...\ferramentas (SHA-512 publicado pela Microsoft)
#  - Node.js LTS portatil em D:\...\ferramentas (SHA-256 publicado em nodejs.org) - apenas para compilar o Angular
#  - SQL Server 2025 Express, instancia FINANCAS, dados no D: (download.microsoft.com + assinatura Microsoft)
#
# Uso (Windows PowerShell como Administrador):
#   Set-ExecutionPolicy -Scope Process Bypass
#   D:\FinancasLocal\scripts\1-instalar-ambiente.ps1
#
# Parametros opcionais:
#   -PularSqlServer          nao instala o SQL Server (use se for aproveitar uma instancia existente)
#   -InstaladorSql <caminho> usa um SQLEXPR_x64_ENU.exe ja baixado manualmente do site da Microsoft
#   -ReinstalarSql           remove uma instalacao do SQL Server que falhou e instala de novo
# =====================================================================
[CmdletBinding()]
param(
    [switch]$PularSqlServer,
    [string]$NomeInstancia = 'FINANCAS',
    [string]$InstaladorSql = '',
    [switch]$ReinstalarSql,
    [string]$UrlSqlExpress = 'https://download.microsoft.com/download/dea8c210-c44a-4a9d-9d80-0c81578860c5/ENU/SQLEXPR_x64_ENU.exe'
)

. "$PSScriptRoot\comum.ps1"
Testar-Administrador
Testar-WindowsPowerShell
Validar-NomeSimples $NomeInstancia 'Nome da instancia'

Write-Host "FinancasLocal - instalacao do ambiente em $Raiz" -ForegroundColor White

# ------------------------------------------------------------------ Pastas
Escrever-Etapa 'Criando pastas'
foreach ($p in @($Ferramentas, $Instaladores, $Dados, $PastaSql, $PastaChaves, $PastaBackups, $PastaPublicado)) { Garantir-Pasta $p }
# Dados e backups: somente Administradores, SYSTEM e voce. (O pool do IIS e o SQL recebem acesso nos proximos scripts.)
$usuarioAtual = [Security.Principal.WindowsIdentity]::GetCurrent().Name
Definir-PermissaoRestrita $Dados @("$($usuarioAtual):(OI)(CI)F")
Definir-PermissaoRestrita $PastaBackups @("$($usuarioAtual):(OI)(CI)F")
Escrever-Ok 'pastas criadas e protegidas'

# ------------------------------------------------------------------ IIS
Escrever-Etapa 'Habilitando o IIS'
$ehServidor = (Get-CimInstance Win32_OperatingSystem).ProductType -ne 1
if ($ehServidor) {
    Install-WindowsFeature Web-Server, Web-Static-Content, Web-Default-Doc, Web-Http-Errors, Web-Stat-Compression,
        Web-Filtering, Web-AppInit, Web-Mgmt-Console, Web-Http-Logging | Out-Null
} else {
    $recursos = 'IIS-WebServerRole', 'IIS-WebServer', 'IIS-CommonHttpFeatures', 'IIS-StaticContent', 'IIS-DefaultDocument',
        'IIS-HttpErrors', 'IIS-HttpCompressionStatic', 'IIS-RequestFiltering', 'IIS-ApplicationInit',
        'IIS-ManagementConsole', 'IIS-HttpLogging', 'IIS-Security'
    Enable-WindowsOptionalFeature -Online -FeatureName $recursos -All -NoRestart | Out-Null
}
Escrever-Ok 'IIS habilitado'

# ------------------------------------------------------------------ .NET 10
Escrever-Etapa 'Obtendo a versao mais recente do .NET 10 (metadados oficiais da Microsoft)'
$urlsMetadados = @(
    'https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json',
    'https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/10.0/releases.json'
)
$metadados = $null
foreach ($u in $urlsMetadados) {
    try { $metadados = Invoke-RestMethod -Uri $u -UseBasicParsing; break } catch { Escrever-Aviso "falha em $u" }
}
if (-not $metadados) { throw 'Nao foi possivel obter os metadados de versoes do .NET.' }

$versao = $metadados.'latest-release'
$release = $metadados.releases | Where-Object { $_.'release-version' -eq $versao } | Select-Object -First 1
$arquivos = @()
foreach ($grupo in @('aspnetcore-runtime', 'sdk', 'runtime')) {
    if ($release.PSObject.Properties.Name -contains $grupo -and $release.$grupo.files) { $arquivos += $release.$grupo.files }
}
$hosting = $arquivos | Where-Object { $_.name -eq 'dotnet-hosting-win.exe' } | Select-Object -First 1
$sdkZip  = $arquivos | Where-Object { $_.name -eq 'dotnet-sdk-win-x64.zip' } | Select-Object -First 1
if (-not $hosting -or -not $sdkZip) { throw "Arquivos do .NET $versao nao encontrados nos metadados." }
Escrever-Ok ".NET $versao"

foreach ($u in @($hosting.url, $sdkZip.url)) {
    $host_ = ([Uri]$u).Host
    if ($host_ -notmatch '(\.microsoft\.com|\.windows\.net|\.azureedge\.net)$') { throw "Host inesperado para download do .NET: $host_" }
}

Escrever-Etapa 'Instalando o ASP.NET Core Hosting Bundle (modulo do IIS)'
$arqHosting = Join-Path $Instaladores "dotnet-hosting-$versao-win.exe"
if (-not (Test-Path $arqHosting)) { Baixar-Arquivo $hosting.url $arqHosting }
Verificar-Sha $arqHosting $hosting.hash 'SHA512'
Verificar-AssinaturaMicrosoft $arqHosting
$proc = Start-Process -FilePath $arqHosting -ArgumentList '/install', '/quiet', '/norestart' -Wait -PassThru
if ($proc.ExitCode -notin 0, 1638, 1641, 3010) { throw "Hosting Bundle retornou codigo $($proc.ExitCode)." }
Escrever-Ok 'Hosting Bundle instalado'

Escrever-Etapa 'Instalando o SDK do .NET (portatil, no D:)'
$destinoDotnet = Join-Path $Ferramentas 'dotnet'
$sdkVersao = $release.sdk.version
if (Test-Path (Join-Path $destinoDotnet "sdk\$sdkVersao")) {
    Escrever-Ok "SDK $sdkVersao ja presente"
} else {
    $arqSdk = Join-Path $Instaladores "dotnet-sdk-$sdkVersao-win-x64.zip"
    if (-not (Test-Path $arqSdk)) { Baixar-Arquivo $sdkZip.url $arqSdk }
    Verificar-Sha $arqSdk $sdkZip.hash 'SHA512'
    if (Test-Path $destinoDotnet) { Remove-Item $destinoDotnet -Recurse -Force }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::ExtractToDirectory($arqSdk, $destinoDotnet)
    Verificar-AssinaturaMicrosoft (Join-Path $destinoDotnet 'dotnet.exe')
    Escrever-Ok "SDK $sdkVersao em $destinoDotnet"
}

# ------------------------------------------------------------------ Node.js (somente para compilar o front-end)
Escrever-Etapa 'Instalando o Node.js LTS (portatil, no D:)'
$indice = Invoke-RestMethod -Uri 'https://nodejs.org/dist/index.json' -UseBasicParsing
$node = $indice | Where-Object {
    $_.lts -and ($_.lts -ne $false) -and ($_.files -contains 'win-x64-zip') -and
    ([int]($_.version.TrimStart('v').Split('.')[0]) -in 20, 22, 24)
} | Select-Object -First 1
if (-not $node) { throw 'Nenhuma versao LTS compativel do Node.js encontrada.' }
$nodeVersao = $node.version
$destinoNode = Join-Path $Ferramentas 'node'
$versaoInstalada = if (Test-Path (Join-Path $destinoNode 'node.exe')) { (& (Join-Path $destinoNode 'node.exe') --version) } else { '' }
if ($versaoInstalada -eq $nodeVersao) {
    Escrever-Ok "Node.js $nodeVersao ja presente"
} else {
    $nomeZip = "node-$nodeVersao-win-x64.zip"
    $arqNode = Join-Path $Instaladores $nomeZip
    $arqShas = Join-Path $Instaladores "node-$nodeVersao-SHASUMS256.txt"
    if (-not (Test-Path $arqNode)) { Baixar-Arquivo "https://nodejs.org/dist/$nodeVersao/$nomeZip" $arqNode }
    Baixar-Arquivo "https://nodejs.org/dist/$nodeVersao/SHASUMS256.txt" $arqShas
    $linha = Get-Content $arqShas | Where-Object { $_ -match "\s$([regex]::Escape($nomeZip))$" } | Select-Object -First 1
    if (-not $linha) { throw 'Hash do Node.js nao encontrado no SHASUMS256.txt.' }
    Verificar-Sha $arqNode ($linha -split '\s+')[0] 'SHA256'
    $temp = Join-Path $Instaladores 'node-extracao'
    if (Test-Path $temp) { Remove-Item $temp -Recurse -Force }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::ExtractToDirectory($arqNode, $temp)
    if (Test-Path $destinoNode) { Remove-Item $destinoNode -Recurse -Force }
    Move-Item (Join-Path $temp "node-$nodeVersao-win-x64") $destinoNode
    Remove-Item $temp -Recurse -Force
    Escrever-Ok "Node.js $nodeVersao em $destinoNode"
}

# ------------------------------------------------------------------ SQL Server Express
if ($PularSqlServer) {
    Escrever-Aviso 'SQL Server ignorado (-PularSqlServer). Informe a instancia existente nos proximos scripts com -InstanciaSql.'
} else {
    Escrever-Etapa "Instalando o SQL Server Express (instancia $NomeInstancia)"
    $chaveInstancias = 'HKLM:\SOFTWARE\Microsoft\Microsoft SQL Server\Instance Names\SQL'
    $nomeServico = "MSSQL`$$NomeInstancia"

    function Obter-InstanciaRegistrada {
        (Test-Path $chaveInstancias) -and ((Get-ItemProperty $chaveInstancias).PSObject.Properties.Name -contains $NomeInstancia)
    }

    function Mostrar-LogsSql {
        $summary = Get-ChildItem 'C:\Program Files\Microsoft SQL Server\*\Setup Bootstrap\Log\Summary.txt' -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending | Select-Object -First 1
        if ($summary) {
            Write-Host "    --- $($summary.FullName) (trecho) ---" -ForegroundColor DarkGray
            Get-Content $summary.FullName | Select-String -Pattern 'Exit code|Exit message|Exit facility|Feature:|Status:|Reason|Message:|Error' |
                Select-Object -First 25 | ForEach-Object { Write-Host "    $($_.Line.Trim())" -ForegroundColor DarkGray }
        }
        $errorlog = Get-ChildItem $PastaSql -Recurse -Filter 'ERRORLOG' -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($errorlog) {
            Write-Host "    --- $($errorlog.FullName) (ultimas linhas) ---" -ForegroundColor DarkGray
            Get-Content $errorlog.FullName -Tail 25 | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
        }
    }

    function Obter-MaiorSetorFisico {
        # Windows 11 com alguns SSDs NVMe informa setor > 4 KB, o que impede o SQL Server de iniciar
        # (erro 0x851A001A "Wait on the Database Engine recovery handle failed").
        $maior = 0
        foreach ($unidade in @($env:SystemDrive, (Split-Path $Raiz -Qualifier)) | Select-Object -Unique) {
            $saida = & cmd.exe /c "fsutil fsinfo sectorinfo $unidade 2>nul"
            foreach ($linha in $saida) {
                if ($linha -match 'PhysicalBytesPerSector\w*\s*:\s*(\d+)') {
                    $valor = [int]$Matches[1]
                    if ($valor -gt $maior) { $maior = $valor }
                }
            }
        }
        # Alguns SSDs (ex.: WD SN3000) informam 4 KB de atomicidade mas 64 KB de setor fisico.
        foreach ($disco in @(Get-PhysicalDisk -ErrorAction SilentlyContinue)) {
            if ([int]$disco.PhysicalSectorSize -gt $maior) { $maior = [int]$disco.PhysicalSectorSize }
        }
        return $maior
    }

    # Correcao documentada pela Microsoft para discos com setor fisico maior que 4 KB.
    $setor = Obter-MaiorSetorFisico
    if ($setor -gt 4096) {
        $chaveNvme = 'HKLM:\SYSTEM\CurrentControlSet\Services\stornvme\Parameters\Device'
        $atual = (Get-ItemProperty -Path $chaveNvme -Name 'ForcedPhysicalSectorSizeInBytes' -ErrorAction SilentlyContinue).ForcedPhysicalSectorSizeInBytes
        if (-not ($atual -contains '* 4095')) {
            if (-not (Test-Path $chaveNvme)) { New-Item -Path $chaveNvme -Force | Out-Null }
            New-ItemProperty -Path $chaveNvme -Name 'ForcedPhysicalSectorSizeInBytes' -PropertyType MultiString -Value @('* 4095') -Force | Out-Null
            Escrever-Aviso "Seu SSD informa setor fisico de $setor bytes, incompativel com o SQL Server."
            Escrever-Aviso 'Apliquei a correcao oficial da Microsoft (ForcedPhysicalSectorSizeInBytes = "* 4095").'
            Write-Host ''
            Write-Host 'REINICIE O COMPUTADOR e depois rode novamente:' -ForegroundColor Yellow
            Write-Host "   $PSCommandPath -ReinstalarSql" -ForegroundColor Yellow
            return
        }
        Escrever-Ok "correcao de setor do NVMe ja aplicada (setor informado: $setor bytes)"
    }

    $instanciaExiste = Obter-InstanciaRegistrada
    $servico = Get-Service -Name $nomeServico -ErrorAction SilentlyContinue

    if ($instanciaExiste -and -not $ReinstalarSql) {
        if ($servico -and $servico.Status -ne 'Running') {
            try { Start-Service -Name $nomeServico -ErrorAction Stop } catch { }
            $servico.Refresh()
        }
        if ($servico -and $servico.Status -eq 'Running') {
            Escrever-Ok "instancia $NomeInstancia ja instalada e em execucao"
        } else {
            Mostrar-LogsSql
            throw "A instancia $NomeInstancia existe mas o servico nao inicia (instalacao anterior incompleta). Rode novamente com -ReinstalarSql."
        }
    } else {
        $exe = $InstaladorSql
        if (-not $exe) {
            $exe = Join-Path $Instaladores 'SQLEXPR_x64_ENU.exe'
            if (-not (Test-Path $exe)) {
                try {
                    Baixar-Arquivo $UrlSqlExpress $exe
                } catch {
                    throw ("Falha ao baixar o SQL Server Express. Baixe manualmente em https://www.microsoft.com/sql-server/sql-server-downloads " +
                        "(Express > Download Media > Express Core) e rode novamente com -InstaladorSql <caminho do SQLEXPR_x64_ENU.exe>.")
                }
            }
        }
        Verificar-AssinaturaMicrosoft $exe

        $midia = Join-Path $Instaladores 'sqlexpr'
        if (Test-Path $midia) { Remove-Item $midia -Recurse -Force }
        $proc = Start-Process -FilePath $exe -ArgumentList '/q', "/x:`"$midia`"" -Wait -PassThru
        if ($proc.ExitCode -ne 0 -or -not (Test-Path (Join-Path $midia 'setup.exe'))) { throw 'Falha ao extrair a midia do SQL Server.' }
        $setup = Join-Path $midia 'setup.exe'
        Verificar-AssinaturaMicrosoft $setup

        if ($instanciaExiste -or $servico) {
            Escrever-Etapa "Removendo a instalacao anterior da instancia $NomeInstancia"
            $proc = Start-Process -FilePath $setup -Wait -PassThru -ArgumentList @(
                '/Q', '/ACTION=Uninstall', '/FEATURES=SQLENGINE', "/INSTANCENAME=$NomeInstancia", '/SUPPRESSPRIVACYSTATEMENTNOTICE')
            if ($proc.ExitCode -notin 0, 3010) { Escrever-Aviso "Desinstalacao retornou o codigo $($proc.ExitCode); tentando instalar mesmo assim." }
            # Arquivos dos bancos de sistema que sobraram impediriam a nova instalacao.
            Get-ChildItem $PastaSql -Directory -Filter "MSSQL*.$NomeInstancia" -ErrorAction SilentlyContinue |
                ForEach-Object { Remove-Item $_.FullName -Recurse -Force }
            if (Test-Path "$PastaSql\tempdb") { Remove-Item "$PastaSql\tempdb" -Recurse -Force }
            Escrever-Ok 'instalacao anterior removida (o banco FinancasLocal ainda nao existia)'
            if ($proc.ExitCode -eq 3010) {
                Write-Host ''
                Write-Host 'A remocao pediu reinicializacao do Windows. REINICIE O COMPUTADOR e depois rode:' -ForegroundColor Yellow
                Write-Host "   $PSCommandPath" -ForegroundColor Yellow
                return
            }
        }

        $existeAlgumSql = Test-Path $chaveInstancias
        $pastaInstancia = Join-Path $Raiz 'sqlserver\instancia'
        Garantir-Pasta $pastaInstancia
        $argumentos = @(
            '/Q', '/ACTION=Install', '/IACCEPTSQLSERVERLICENSETERMS', '/SUPPRESSPRIVACYSTATEMENTNOTICE',
            '/FEATURES=SQLENGINE', "/INSTANCENAME=$NomeInstancia", "/INSTANCEID=$NomeInstancia",
            "/INSTANCEDIR=`"$pastaInstancia`"",
            "/INSTALLSQLDATADIR=`"$PastaSql`"",
            "/SQLUSERDBDIR=`"$PastaSql`"", "/SQLUSERDBLOGDIR=`"$PastaSql`"",
            "/SQLTEMPDBDIR=`"$PastaSql\tempdb`"", "/SQLTEMPDBLOGDIR=`"$PastaSql\tempdb`"",
            "/SQLBACKUPDIR=`"$PastaBackups`"",
            "/SQLSYSADMINACCOUNTS=`"$usuarioAtual`"",
            # Sem /SECURITYMODE=SQL: somente autenticacao do Windows (sem login "sa")
            '/TCPENABLED=0', '/NPENABLED=0',     # sem acesso pela rede: apenas conexoes locais (memoria compartilhada)
            '/SQLSVCSTARTUPTYPE=Automatic', '/BROWSERSVCSTARTUPTYPE=Disabled',
            '/UPDATEENABLED=0', '/INDICATEPROGRESS=0'
        )
        if (-not $existeAlgumSql) {
            # Componentes compartilhados so podem ir para o D: se nao houver outro SQL Server instalado.
            $argumentos += "/INSTALLSHAREDDIR=`"$(Join-Path $Raiz 'sqlserver\compartilhado')`""
            $argumentos += "/INSTALLSHAREDWOWDIR=`"$(Join-Path $Raiz 'sqlserver\compartilhado-x86')`""
        } else {
            Escrever-Aviso 'Ja existe outro SQL Server neste computador: os componentes compartilhados ficam onde ja estao.'
        }

        Write-Host '    instalando (pode levar de 5 a 15 minutos)...'
        $proc = Start-Process -FilePath $setup -ArgumentList $argumentos -Wait -PassThru
        if ($proc.ExitCode -notin 0, 3010) {
            Mostrar-LogsSql
            $hex = '0x{0:X8}' -f $proc.ExitCode
            throw ("O instalador do SQL Server retornou o codigo $($proc.ExitCode) ($hex). Copie as linhas acima e envie para analise. " +
                "Depois de corrigir a causa, rode novamente com -ReinstalarSql.")
        }
        Escrever-Ok "SQL Server Express instalado (instancia .\$NomeInstancia, sem acesso pela rede)"
        if ($proc.ExitCode -eq 3010) { Escrever-Aviso 'O Windows pede reinicializacao para concluir a instalacao.' }
    }
}

# ------------------------------------------------------------------ Finalizacao
Escrever-Etapa 'Reiniciando o IIS para carregar o modulo do ASP.NET Core'
& iisreset.exe /restart | Out-Null
Escrever-Ok 'IIS reiniciado'

Write-Host ''
Write-Host 'Ambiente pronto. Proximo passo:' -ForegroundColor Green
Write-Host "   $PSScriptRoot\2-criar-banco.ps1"
