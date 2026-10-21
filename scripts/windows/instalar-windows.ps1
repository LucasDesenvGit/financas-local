<#
.SYNOPSIS
    Instala ou atualiza o FinancasLocal no Windows, sem bash: ambiente, banco e publicacao.

.DESCRIPTION
    Equivalente, no Windows, a scripts/geral/instalar.sh (etapas 1, 2 e 3). Usa so o que vem com o
    Windows (Windows PowerShell 5.1) e baixa o resto para ferramentas\:
      1) Node.js LTS, .NET 10 SDK e sqlite3 portateis (reaproveita dotnet 10.x e sqlite3 do PATH),
         todos por HTTPS e conferidos pelo hash publicado pela origem
      2) Banco SQLite: banco\01-estrutura.sql, colunas novas e banco\02-dados-iniciais.sql
         (a lista de colunas novas e lida do 2-criar-banco.sh, para existir num lugar so)
      3) Compila front-end e API, publica em publicado\api, gera/reaproveita o certificado HTTPS
         e registra o servico do Windows "FinancasLocal"

    Precisa ser executado como Administrador (registro do servico). O instalar-windows.cmd, na raiz,
    pede a elevacao e chama este script. Pode ser executado de novo para atualizar: nao apaga dados.
    A saida fica tambem em instaladores\instalacao-windows.log (acumula as execucoes). Numa
    atualizacao, o banco e copiado para backups\ antes de mudar a estrutura, e a versao publicada
    anterior volta sozinha se a nova nao subir.

    Ao migrar de uma instalacao com SQL Server, NAO use este script: siga docs/INSTALACAO-PELO-CODIGO.md.

.EXAMPLE
    .\instalar-windows.ps1
    .\instalar-windows.ps1 -PularBuild
    .\instalar-windows.ps1 -NaoConfiarCertificado   # sem ninguem na frente da tela (ex.: CI)
#>
[CmdletBinding()]
param(
    # Reaproveita o build anterior (publicado\preparo) e so republica.
    [switch]$PularBuild,
    # Nao poe o certificado nas Autoridades Raiz Confiaveis da maquina (o navegador vai avisar).
    [switch]$NaoConfiarCertificado
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"   # a barra de progresso deixa o Invoke-WebRequest muito lento no 5.1
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# ---------------------------------------------------------------- Pastas (mesma convencao do comum.sh)
$Raiz = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Ferramentas = Join-Path $Raiz "ferramentas"
$Instaladores = Join-Path $Raiz "instaladores"
$Dados = Join-Path $Raiz "dados"
$PastaSql = Join-Path $Dados "sql"
$PastaChaves = Join-Path $Dados "chaves"
$PastaBackups = Join-Path $Raiz "backups"
$PastaPublicado = Join-Path $Raiz "publicado"
$NomeServico = "FinancasLocal"

# ---------------------------------------------------------------- Mensagens
$script:EtapaAtual = "inicio"
function Etapa([string]$texto) { $script:EtapaAtual = $texto; Write-Host ""; Write-Host "==> $texto" -ForegroundColor Cyan }
function Ok([string]$texto) { Write-Host "    OK  $texto" -ForegroundColor Green }
function Aviso([string]$texto) { Write-Host "    !!  $texto" -ForegroundColor Yellow }
# Erros previstos (com mensagem pronta) sao marcados para o catch final nao mostrar a linha do
# script; os inesperados mostram, para achar a causa.
function ErroSai([string]$texto) {
    $erro = New-Object System.Exception $texto
    $erro.Data["Previsto"] = $true
    throw $erro
}

# ---------------------------------------------------------------- Utilitarios
function Garantir-Pasta([string]$caminho) {
    if (-not (Test-Path -LiteralPath $caminho)) { New-Item -ItemType Directory -Path $caminho -Force | Out-Null }
}

# Caminho com barras normais (C:/FinancasLocal), como o cygpath -m dos scripts bash.
function Caminho-Barras([string]$caminho) { return $caminho -replace '\\', '/' }


# Baixa para arquivo e le como UTF-8: dependendo do Content-Type, o .Content do Invoke-WebRequest
# do 5.1 vem como bytes em vez de texto.
function Obter-Texto([string]$url) {
    if (-not $url.StartsWith("https://")) { ErroSai "Somente downloads via HTTPS sao permitidos: $url" }
    $temporario = Join-Path $Instaladores "download.tmp"
    Invoke-WebRequest -Uri $url -OutFile $temporario -UseBasicParsing
    $texto = [IO.File]::ReadAllText($temporario, [Text.Encoding]::UTF8)
    Remove-Item -LiteralPath $temporario -Force
    return $texto
}

function Baixar-Arquivo([string]$url, [string]$destino) {
    if (-not $url.StartsWith("https://")) { ErroSai "Somente downloads via HTTPS sao permitidos: $url" }
    Write-Host "    baixando $url"
    Invoke-WebRequest -Uri $url -OutFile $destino -UseBasicParsing
}

# Verificar-Hash arquivo hash_esperado algoritmo(SHA256|SHA512|SHA3-256)
# SHA3-256 (usado pelo sqlite.org) nao existe no .NET Framework do PowerShell 5.1: e calculado
# pelo Node.js, que e instalado antes de precisar dele. O codigo JS nao usa aspas duplas: o 5.1
# as remove dos argumentos passados a programas externos.
function Verificar-Hash([string]$arquivo, [string]$esperado, [string]$algoritmo) {
    if ($algoritmo -eq "SHA3-256") {
        $hash = & node -e 'const c = require(''crypto''), fs = require(''fs''); process.stdout.write(c.createHash(''sha3-256'').update(fs.readFileSync(process.argv[1])).digest(''hex''));' $arquivo
        if ($LASTEXITCODE -ne 0) { ErroSai "Nao foi possivel calcular o SHA3-256 de $arquivo." }
    } else {
        $hash = (Get-FileHash -LiteralPath $arquivo -Algorithm $algoritmo).Hash
    }
    if ($hash -ne $esperado) {   # -ne do PowerShell ignora maiusculas/minusculas
        Remove-Item -LiteralPath $arquivo -Force
        ErroSai "Hash $algoritmo nao confere para $(Split-Path -Leaf $arquivo). O arquivo foi apagado por seguranca."
    }
    Ok "hash $algoritmo conferido: $(Split-Path -Leaf $arquivo)"
}

# ZipFile e bem mais rapido que o Expand-Archive do 5.1 (o SDK do .NET tem milhares de arquivos).
function Extrair-Zip([string]$arquivo, [string]$destino) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    Garantir-Pasta $destino
    [System.IO.Compression.ZipFile]::ExtractToDirectory($arquivo, $destino)
}

# Executa um programa externo e para se ele falhar (o PowerShell nao para sozinho com exit code).
# Junta stdout e stderr e passa pelo Out-Host para os dois entrarem no log (o Start-Transcript nao
# grava saida direta de programa externo); as ultimas linhas vao junto da mensagem de erro, para a
# causa aparecer ao lado do ERRO.
function Executar([string]$programa, [string[]]$argumentos) {
    # No 5.1, stderr redirecionado com ErrorActionPreference=Stop vira excecao na primeira linha.
    $ErrorActionPreference = "Continue"
    $ultimas = New-Object System.Collections.Generic.Queue[string]
    & $programa @argumentos 2>&1 | ForEach-Object {
        if ($_ -is [System.Management.Automation.ErrorRecord]) { $linha = $_.Exception.Message } else { $linha = "$_" }
        $ultimas.Enqueue($linha)
        if ($ultimas.Count -gt 20) { [void]$ultimas.Dequeue() }
        $linha
    } | Out-Host
    if ($LASTEXITCODE -ne 0) {
        ErroSai ("Falhou ($LASTEXITCODE): $programa $($argumentos -join ' '). Ultimas linhas:`n" + ($ultimas -join "`n"))
    }
}

function Adicionar-Path([string]$pasta) {
    if (($env:PATH -split ';') -notcontains $pasta) { $env:PATH = "$pasta;$env:PATH" }
}

function Comando-Existe([string]$nome) { return [bool](Get-Command $nome -ErrorAction SilentlyContinue) }

# #300: quem escuta na porta ("nome (PID n)"), ou $null se ela esta livre.
function Quem-UsaPorta([int]$porta) {
    $conexao = Get-NetTCPConnection -LocalPort $porta -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $conexao) { return $null }
    $processo = Get-Process -Id $conexao.OwningProcess -ErrorAction SilentlyContinue
    $nome = if ($processo) { $processo.ProcessName } else { "programa desconhecido" }
    return "$nome (PID $($conexao.OwningProcess))"
}

# ================================================================= 1) Ambiente
function Preparar-Ambiente {
    Garantir-Pasta $Ferramentas
    Garantir-Pasta $Instaladores

    # ------------------------------------------------------------ Node.js
    Etapa "[1/3] Verificando o Node.js"
    $destinoNode = Join-Path $Ferramentas "node"
    $chave = "win-$Arch-zip"
    # index.tab: versao, data, arquivos, ..., lts na 10a coluna. Angular 22 aceita Node 22 e 24 (o 26 ainda nao foi testado aqui).
    $versaoNode = $null
    foreach ($linha in ((Obter-Texto "https://nodejs.org/dist/index.tab") -split "`n" | Select-Object -Skip 1)) {
        $col = $linha.TrimEnd("`r") -split "`t"
        if ($col.Count -lt 10 -or $col[9] -eq "-") { continue }
        $maior = [int]($col[0].TrimStart("v").Split(".")[0])
        if (@(22, 24) -notcontains $maior) { continue }
        if (($col[2] -split ",") -contains $chave) { $versaoNode = $col[0]; break }
    }
    if (-not $versaoNode) { ErroSai "Nenhuma versao LTS compativel do Node.js encontrada." }

    $nodeExe = Join-Path $destinoNode "node.exe"
    $versaoInstalada = $null
    if (Test-Path -LiteralPath $nodeExe) { $versaoInstalada = (& $nodeExe --version) }
    if ($versaoInstalada -eq $versaoNode) {
        Ok "Node.js $versaoNode ja presente"
    } else {
        $nomePacote = "node-$versaoNode-win-$Arch.zip"
        $arqNode = Join-Path $Instaladores $nomePacote
        $arqShas = Join-Path $Instaladores "node-$versaoNode-SHASUMS256.txt"
        if (-not (Test-Path -LiteralPath $arqNode)) { Baixar-Arquivo "https://nodejs.org/dist/$versaoNode/$nomePacote" $arqNode }
        Baixar-Arquivo "https://nodejs.org/dist/$versaoNode/SHASUMS256.txt" $arqShas
        $linhaHash = Get-Content -LiteralPath $arqShas | Where-Object { $_ -match "\s$([regex]::Escape($nomePacote))$" } | Select-Object -First 1
        if (-not $linhaHash) { ErroSai "Hash do Node.js nao encontrado no SHASUMS256.txt." }
        Verificar-Hash $arqNode (($linhaHash -split '\s+')[0]) "SHA256"

        Etapa "Extraindo o Node.js"
        $tmp = Join-Path $Instaladores "node-extracao"
        if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Recurse -Force }
        Extrair-Zip $arqNode $tmp
        if (Test-Path -LiteralPath $destinoNode) { Remove-Item -LiteralPath $destinoNode -Recurse -Force }
        Move-Item -LiteralPath (Join-Path $tmp "node-$versaoNode-win-$Arch") -Destination $destinoNode
        Remove-Item -LiteralPath $tmp -Recurse -Force
        Ok "Node.js $versaoNode em $destinoNode"
    }
    Adicionar-Path $destinoNode

    # ------------------------------------------------------------ .NET 10 SDK
    Etapa "[1/3] Verificando o SDK do .NET"
    $temDotnet10 = $false
    if (Comando-Existe "dotnet") {
        $sdks = & dotnet --list-sdks
        if ($LASTEXITCODE -eq 0 -and ($sdks | Where-Object { $_ -match '^10\.' })) { $temDotnet10 = $true }
    }
    if ($temDotnet10) {
        Ok "SDK do .NET 10 ja disponivel no PATH ($((Get-Command dotnet).Source))"
    } else {
        $destinoDotnet = Join-Path $Ferramentas "dotnet"
        $nomePacote = "dotnet-sdk-win-$Arch.zip"
        Etapa "Obtendo a versao mais recente do .NET 10 (metadados oficiais da Microsoft)"
        $metadados = $null
        foreach ($url in @("https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json",
                           "https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/10.0/releases.json")) {
            try { $metadados = Invoke-RestMethod -Uri $url -UseBasicParsing; break } catch { Aviso "falha em $url" }
        }
        if (-not $metadados) { ErroSai "Nao foi possivel obter os metadados de versoes do .NET." }
        $versaoRelease = $metadados.'latest-release'
        $release = $metadados.releases | Where-Object { $_.'release-version' -eq $versaoRelease } | Select-Object -First 1
        $arquivo = $release.sdk.files | Where-Object { $_.name -eq $nomePacote } | Select-Object -First 1
        if (-not $arquivo) { ErroSai "Pacote $nomePacote nao encontrado nos metadados do .NET $versaoRelease." }
        $sdkVersao = $release.sdk.version
        Ok ".NET SDK $sdkVersao"

        $hostSdk = ([Uri]$arquivo.url).Host
        if ($hostSdk -notmatch '\.(microsoft\.com|windows\.net|azureedge\.net)$') { ErroSai "Host inesperado para download do .NET: $hostSdk" }

        if (Test-Path -LiteralPath (Join-Path $destinoDotnet "sdk\$sdkVersao")) {
            Ok "SDK $sdkVersao ja presente em $destinoDotnet"
        } else {
            $arqSdk = Join-Path $Instaladores "dotnet-sdk-$sdkVersao-windows-$Arch.zip"
            if (-not (Test-Path -LiteralPath $arqSdk)) { Baixar-Arquivo $arquivo.url $arqSdk }
            Verificar-Hash $arqSdk $arquivo.hash "SHA512"
            Etapa "Extraindo o SDK do .NET"
            if (Test-Path -LiteralPath $destinoDotnet) { Remove-Item -LiteralPath $destinoDotnet -Recurse -Force }
            Extrair-Zip $arqSdk $destinoDotnet
            Ok "SDK $sdkVersao extraido em $destinoDotnet"
        }
    }

    # ------------------------------------------------------------ sqlite3
    Etapa "[1/3] Verificando o sqlite3"
    $destinoSqlite = Join-Path $Ferramentas "sqlite"
    if (Comando-Existe "sqlite3") {
        Ok "sqlite3 disponivel ($((Get-Command sqlite3).Source))"
    } else {
        Etapa "Obtendo a versao mais recente do sqlite3 (pagina oficial de download)"
        # Bloco "para scripts lerem" da pagina: PRODUCT,VERSAO,ANO/arquivo.zip,TAMANHO,HASH-SHA3-256
        $pagina = Obter-Texto "https://www.sqlite.org/download.html"
        $m = [regex]::Match($pagina, "(?m)^PRODUCT,([^,]+),(\d{4}/sqlite-tools-win-$Arch-\d+\.zip),\d+,([0-9a-fA-F]+)")
        if (-not $m.Success) { ErroSai "Pacote sqlite-tools-win-$Arch nao encontrado em sqlite.org/download.html." }
        $versaoSqlite = $m.Groups[1].Value
        $relativo = $m.Groups[2].Value
        Ok "sqlite3 $versaoSqlite"

        $arqSqlite = Join-Path $Instaladores (Split-Path -Leaf $relativo)
        if (-not (Test-Path -LiteralPath $arqSqlite)) { Baixar-Arquivo "https://www.sqlite.org/$relativo" $arqSqlite }
        Verificar-Hash $arqSqlite $m.Groups[3].Value "SHA3-256"

        Etapa "Extraindo o sqlite3"
        $tmp = Join-Path $Instaladores "sqlite-extracao"
        if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Recurse -Force }
        Extrair-Zip $arqSqlite $tmp
        $exe = Get-ChildItem -LiteralPath $tmp -Recurse -Filter "sqlite3.exe" | Select-Object -First 1
        if (-not $exe) { ErroSai "sqlite3.exe nao encontrado dentro de $(Split-Path -Leaf $arqSqlite)." }
        if (Test-Path -LiteralPath $destinoSqlite) { Remove-Item -LiteralPath $destinoSqlite -Recurse -Force }
        Move-Item -LiteralPath $exe.DirectoryName -Destination $destinoSqlite
        if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Recurse -Force }
        Adicionar-Path $destinoSqlite
        Ok "sqlite3 $versaoSqlite em $destinoSqlite"
    }

    Etapa "[1/3] Conferindo o ambiente"
    Configurar-AmbienteBuild
    Ok "dotnet  $(& dotnet --version)"
    Ok "node    $(& node --version)"
    Ok "npm     $(& npm.cmd --version)"
    Ok "sqlite3 $((& sqlite3 --version).Split(' ')[0])"
}

# Mesmo que configurar_ambiente_build do comum.sh.
function Configurar-AmbienteBuild {
    $dotnetPortatil = Join-Path $Ferramentas "dotnet"
    if (Test-Path -LiteralPath (Join-Path $dotnetPortatil "dotnet.exe")) {
        Adicionar-Path $dotnetPortatil
        $env:DOTNET_ROOT = $dotnetPortatil
    } elseif (-not (Comando-Existe "dotnet")) {
        ErroSai "SDK do .NET nao encontrado."
    }
    $nodePortatil = Join-Path $Ferramentas "node"
    if (Test-Path -LiteralPath (Join-Path $nodePortatil "node.exe")) { Adicionar-Path $nodePortatil }
    elseif (-not (Comando-Existe "node")) { ErroSai "Node.js nao encontrado." }

    $env:DOTNET_CLI_HOME = Join-Path $Ferramentas "dotnet-home"
    $env:NUGET_PACKAGES = Join-Path $Ferramentas "nuget-pacotes"
    $env:DOTNET_CLI_TELEMETRY_OPTOUT = "1"
    $env:DOTNET_NOLOGO = "1"
    $env:DOTNET_SKIP_FIRST_TIME_EXPERIENCE = "1"
    $env:npm_config_cache = Join-Path $Ferramentas "npm-cache"
    $env:npm_config_update_notifier = "false"
    $env:NG_CLI_ANALYTICS = "false"
}

# ================================================================= 2) Banco
# .timeout: se a API (ainda no ar numa atualizacao) estiver gravando, espera em vez de falhar com
# "database is locked".
function Sqlite([string]$banco, [string]$comando) {
    $saida = & sqlite3 -cmd ".timeout 15000" $banco $comando
    if ($LASTEXITCODE -ne 0) { ErroSai "sqlite3 falhou: $comando" }
    return $saida
}

# Comando do sqlite3 sobre um arquivo (.read, .backup) rodado de dentro da pasta do arquivo, so
# com o nome: o caminho completo precisaria de aspas (espaco, apostrofo), e o shell do sqlite3 so
# tem escape dentro de "...", que o PowerShell 5.1 estraga ao passar para programa externo.
function Sqlite-NaPasta([string]$banco, [string]$comando, [string]$arquivo) {
    Push-Location -LiteralPath (Split-Path -Parent $arquivo)
    try { Sqlite $banco "$comando $(Split-Path -Leaf $arquivo)" | Out-Null } finally { Pop-Location }
}

function Criar-Banco {
    $banco = Join-Path $PastaSql "FinancasLocal.db"
    $pastaBancoSql = Join-Path $Raiz "banco"

    Etapa "[2/3] Criando pastas de dados"
    foreach ($p in @($Dados, $PastaSql, $PastaChaves, $PastaBackups)) { Garantir-Pasta $p }
    Ok "pastas criadas"

    # Atualizacao: copia o banco antes de mexer na estrutura. .backup e seguro com a API no ar.
    # Ficam as 5 copias mais recentes (fora da rotacao do 4-backup.sh, que usa outro nome).
    if (Test-Path -LiteralPath $banco) {
        Etapa "[2/3] Backup do banco antes da atualizacao"
        $copia = Join-Path $PastaBackups ("FinancasLocal-antes-instalacao-" + (Get-Date -Format "yyyyMMdd_HHmmss") + ".db")
        Sqlite-NaPasta $banco ".backup" $copia
        Get-ChildItem -LiteralPath $PastaBackups -Filter "FinancasLocal-antes-instalacao-*.db" |
            Sort-Object Name -Descending | Select-Object -Skip 5 | Remove-Item -Force
        Ok "copia em $copia"
    }

    # .read em vez de redirecionar o arquivo: o pipe do PowerShell 5.1 para programas externos
    # converte o texto para ASCII e estragaria os acentos de 02-dados-iniciais.sql.
    Etapa "[2/3] Criando a estrutura em $banco (01-estrutura.sql)"
    $estrutura = Join-Path $pastaBancoSql "01-estrutura.sql"
    if (-not (Test-Path -LiteralPath $estrutura)) { ErroSai "Arquivo nao encontrado: $estrutura" }
    Sqlite-NaPasta $banco ".read" $estrutura
    Ok "estrutura criada/atualizada"

    # A lista de colunas novas fica so no 2-criar-banco.sh (linhas adicionar_coluna_se_ausente),
    # para nao precisar manter duas copias.
    Etapa "[2/3] Adicionando colunas novas em tabelas existentes"
    $scriptBanco = Join-Path $Raiz "scripts\geral\2-criar-banco.sh"
    $padrao = '^\s*adicionar_coluna_se_ausente\s+"([A-Za-z0-9_]+)"\s+"([A-Za-z0-9_]+)"\s+"([^"]+)"'
    $total = 0
    foreach ($linha in (Get-Content -LiteralPath $scriptBanco)) {
        $m = [regex]::Match($linha, $padrao)
        if (-not $m.Success) { continue }
        $total++
        $tabela = $m.Groups[1].Value; $coluna = $m.Groups[2].Value; $definicao = $m.Groups[3].Value
        $colunas = Sqlite $banco "PRAGMA table_info($tabela);" | ForEach-Object { ($_ -split '\|')[1] }
        if (@($colunas) -notcontains $coluna) {
            Sqlite $banco "ALTER TABLE $tabela ADD COLUMN $coluna $definicao;" | Out-Null
            Ok "coluna $coluna adicionada em $tabela"
        }
    }
    if ($total -eq 0) { Aviso "nenhuma linha adicionar_coluna_se_ausente encontrada em 2-criar-banco.sh" }
    Ok "colunas conferidas"

    Etapa "[2/3] Inserindo dados iniciais (02-dados-iniciais.sql)"
    $iniciais = Join-Path $pastaBancoSql "02-dados-iniciais.sql"
    if (-not (Test-Path -LiteralPath $iniciais)) { ErroSai "Arquivo nao encontrado: $iniciais" }
    Sqlite-NaPasta $banco ".read" $iniciais
    Ok "categorias e configuracoes padrao"

    $tabelas = Sqlite $banco "SELECT COUNT(*) FROM sqlite_master WHERE type='table';"
    Ok "banco pronto ($tabelas tabela(s))"
}

# ================================================================= 3) Publicacao
# O Windows nao apaga um .exe em execucao: com um "npm start" aberto (ng serve + esbuild.exe de
# src\frontend\node_modules), o "npm ci" falha com EPERM. Encerra so o que roda de dentro dessa pasta.
function Encerrar-UsoDoFrontEnd([string]$pastaFront) {
    $modulos = (Join-Path $pastaFront "node_modules").Replace('/', '\').ToLowerInvariant()
    $emUso = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $_.ProcessId -ne $PID -and (
            ($_.ExecutablePath -and $_.ExecutablePath.ToLowerInvariant().StartsWith($modulos)) -or
            ($_.CommandLine -and $_.CommandLine.ToLowerInvariant().Contains($modulos)))
    })
    if ($emUso.Count -eq 0) { return }
    Aviso "Encerrando $($emUso.Count) processo(s) que usam o front-end (ex.: um 'npm start' aberto); sem isso o 'npm ci' falha:"
    foreach ($p in $emUso) {
        Aviso "  PID $($p.ProcessId): $($p.Name)"
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
    Start-Sleep -Seconds 1
}

function Publicar {
    $pastaFront = Join-Path $Raiz "src\frontend"
    $projetoApi = Join-Path $Raiz "src\backend\Financas.Api\Financas.Api.csproj"
    $preparo = Join-Path $PastaPublicado "preparo"
    $destino = Join-Path $PastaPublicado "api"

    Configurar-AmbienteBuild

    if (-not $PularBuild) {
        Etapa "[3/3] Compilando o front-end (Angular)"
        Encerrar-UsoDoFrontEnd $pastaFront
        Push-Location $pastaFront
        try {
            Executar "npm.cmd" @("ci", "--no-audit", "--no-fund")
            Executar "npm.cmd" @("run", "build")
        } finally { Pop-Location }
        Ok "front-end compilado"

        Etapa "[3/3] Compilando a API (.NET)"
        if (Test-Path -LiteralPath $preparo) { Remove-Item -LiteralPath $preparo -Recurse -Force }
        Executar "dotnet" @("publish", $projetoApi, "-c", "Release", "-o", $preparo, "--nologo")
        Ok "API compilada"

        $distFront = Join-Path $pastaFront "dist\financas-web\browser"
        if (-not (Test-Path -LiteralPath (Join-Path $distFront "index.html"))) { ErroSai "Saida do Angular nao encontrada em $distFront" }
        $wwwroot = Join-Path $preparo "wwwroot"
        Garantir-Pasta $wwwroot
        Copy-Item -Path (Join-Path $distFront "*") -Destination $wwwroot -Recurse -Force
        Ok "front-end incorporado ao wwwroot"
    } elseif (-not (Test-Path -LiteralPath $preparo)) {
        ErroSai "Nao ha build anterior em publicado\preparo. Rode sem -PularBuild."
    }

    # ------------------------------------------------------------ Certificado HTTPS
    Etapa "[3/3] Certificado HTTPS local"
    Garantir-Pasta $PastaChaves
    $pfx = Join-Path $PastaChaves "financaslocal.pfx"
    $configAtual = Join-Path $destino "appsettings.Production.json"

    # A senha do PFX so fica no appsettings.Production.json publicado (nunca na tela).
    $senhaPfx = $null
    if ((Test-Path -LiteralPath $pfx) -and (Test-Path -LiteralPath $configAtual)) {
        try {
            $senhaPfx = (Get-Content -LiteralPath $configAtual -Raw -Encoding UTF8 | ConvertFrom-Json).Kestrel.Endpoints.Https.Certificate.Password
        } catch { $senhaPfx = $null }
    }

    $reaproveitar = $false
    if ($senhaPfx) {
        try {
            $cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($pfx, $senhaPfx)
            $reaproveitar = $cert.NotAfter -gt (Get-Date).AddDays(30)
        } catch { $reaproveitar = $false }
    }

    if ($reaproveitar) {
        Ok "certificado existente reaproveitado"
    } else {
        Etapa "Gerando certificado HTTPS de desenvolvimento"
        $bytes = New-Object byte[] 24
        [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
        $senhaPfx = [Convert]::ToBase64String($bytes)
        if (Test-Path -LiteralPath $pfx) { Remove-Item -LiteralPath $pfx -Force }
        & dotnet dev-certs https -ep $pfx -p $senhaPfx | Out-Null
        if ($LASTEXITCODE -ne 0) { ErroSai "dotnet dev-certs falhou ao gerar o certificado." }
        Ok "certificado gerado em $pfx"
    }

    # Confia na loja da MAQUINA (Autoridades Raiz Confiaveis), sem janela: o "dotnet dev-certs
    # https --trust" pede um clique e, se quem elevou no UAC foi outra conta, poria o certificado so
    # na conta do administrador. Vai so a parte publica; o FriendlyName identifica o certificado
    # antigo para troca-lo quando houver renovacao.
    if ($NaoConfiarCertificado) {
        Aviso "Certificado nao marcado como confiavel (-NaoConfiarCertificado). O navegador vai mostrar um aviso de seguranca."
    } else {
        $completo = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($pfx, $senhaPfx)
        $publico = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2(, $completo.RawData)
        $publico.FriendlyName = $NomeServico
        $loja = New-Object System.Security.Cryptography.X509Certificates.X509Store("Root", "LocalMachine")
        $loja.Open("ReadWrite")
        try {
            foreach ($antigo in @($loja.Certificates)) {
                if ($antigo.FriendlyName -eq $NomeServico -and $antigo.Thumbprint -ne $publico.Thumbprint) { $loja.Remove($antigo) }
            }
            if (-not ($loja.Certificates | Where-Object { $_.Thumbprint -eq $publico.Thumbprint })) { $loja.Add($publico) }
        } finally { $loja.Close() }
        Ok "certificado confiavel para todas as contas deste computador (vale so para localhost)"
    }

    # ------------------------------------------------------------ Configuracao de producao
    Etapa "[3/3] Gerando appsettings.Production.json"
    $config = [ordered]@{
        Banco = [ordered]@{ Caminho = (Caminho-Barras $PastaSql) + "/FinancasLocal.db" }
        Seguranca = [ordered]@{ PastaChaves = (Caminho-Barras $PastaChaves) }
        Kestrel = [ordered]@{
            Endpoints = [ordered]@{
                Https = [ordered]@{
                    Certificate = [ordered]@{ Path = (Caminho-Barras $pfx); Password = $senhaPfx }
                }
            }
        }
    }
    $json = ($config | ConvertTo-Json -Depth 10) + "`n"
    [IO.File]::WriteAllText((Join-Path $preparo "appsettings.Production.json"), $json, (New-Object Text.UTF8Encoding $false))
    Ok "configuracao gerada"

    # ------------------------------------------------------------ Copia (com o servico parado)
    # O Windows trava as DLLs em uso: o servico tem de parar antes da copia.
    $servico = Get-Service -Name $NomeServico -ErrorAction SilentlyContinue
    if ($servico -and $servico.Status -ne "Stopped") {
        Etapa "Parando o servico $NomeServico"
        Stop-Service -Name $NomeServico -Force
        (Get-Service -Name $NomeServico).WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
        Ok "servico parado"
    }

    # #300: outro programa na porta impede o site de subir. O servico ja parou (acima), entao quem estiver nela
    # nao e o Financas Local. Para antes de trocar a versao: nada e mudado, e o servico volta a ligar.
    $ocupante = Quem-UsaPorta 5443
    if ($ocupante) {
        if ($servico) { Start-Service -Name $NomeServico -ErrorAction SilentlyContinue }
        ErroSai "A porta 5443 esta em uso por ${ocupante}: o site nao sobe enquanto ela estiver ocupada. Feche esse programa (ou pare o servico dele) e rode a instalacao de novo. Nada foi mudado."
    }

    # Guarda a versao publicada atual para voltar a ela se a nova nao subir.
    $anterior = Join-Path $PastaPublicado "api-anterior"
    $temAnterior = Test-Path -LiteralPath (Join-Path $destino "Financas.Api.dll")
    if ($temAnterior) {
        & robocopy.exe $destino $anterior /MIR /NFL /NDL /NJH /NJS /NP /R:3 /W:2 | Out-Null
        if ($LASTEXITCODE -ge 8) { ErroSai "robocopy falhou ($LASTEXITCODE) ao guardar a versao anterior em $anterior." }
        $global:LASTEXITCODE = 0
    }

    Etapa "[3/3] Publicando em $destino"
    Garantir-Pasta $destino
    # /MIR espelha (apaga o que nao existe mais no build). Codigos de saida < 8 sao sucesso.
    & robocopy.exe $preparo $destino /MIR /NFL /NDL /NJH /NJS /NP /R:3 /W:2 | Out-Null
    if ($LASTEXITCODE -ge 8) { ErroSai "robocopy falhou ($LASTEXITCODE) ao publicar em $destino." }
    $global:LASTEXITCODE = 0
    Ok "arquivos publicados"

    # ------------------------------------------------------------ Servico do Windows
    Etapa "[3/3] Registrando o servico $NomeServico"
    $dotnetPortatil = Join-Path $Ferramentas "dotnet\dotnet.exe"
    if (Test-Path -LiteralPath $dotnetPortatil) { $dotnetExec = $dotnetPortatil } else { $dotnetExec = (Get-Command dotnet).Source }
    $dll = Join-Path $destino "Financas.Api.dll"
    $binPath = "`"$dotnetExec`" `"$dll`""

    # New-Service e Win32_Service.Change recebem o binPath como esta (o sc.exe chamado pelo
    # PowerShell 5.1 perde as aspas internas).
    if ($servico) {
        $resultado = Get-CimInstance -ClassName Win32_Service -Filter "Name='$NomeServico'" |
            Invoke-CimMethod -MethodName Change -Arguments @{ PathName = $binPath; StartMode = "Automatic" }
        if ($resultado.ReturnValue -ne 0) { ErroSai "Nao foi possivel reconfigurar o servico (codigo $($resultado.ReturnValue))." }
        Ok "servico atualizado"
    } else {
        New-Service -Name $NomeServico -BinaryPathName $binPath -DisplayName $NomeServico -StartupType Automatic | Out-Null
        Ok "servico criado"
    }
    # ------------------------------------------------------------ Inicio e teste
    Etapa "[3/3] Iniciando e testando"
    $inicio = Get-Date
    $falha = Iniciar-E-Testar
    if (-not $falha) { return }

    # A nova versao nao subiu: mostra os eventos do Windows (a excecao real da API fica la) e, numa
    # atualizacao, volta a versao anterior para o site nao ficar fora do ar. O
    # appsettings.Production.json novo e mantido (o certificado pode ter sido renovado agora).
    Mostrar-EventosServico $inicio
    if ($temAnterior) {
        Aviso "Voltando a versao anterior..."
        Stop-Service -Name $NomeServico -Force -ErrorAction SilentlyContinue
        & robocopy.exe $anterior $destino /MIR /XF appsettings.Production.json /NFL /NDL /NJH /NJS /NP /R:3 /W:2 | Out-Null
        $global:LASTEXITCODE = 0
        if (-not (Iniciar-E-Testar)) { ErroSai "$falha A versao anterior foi restaurada e esta no ar; veja os eventos acima." }
        ErroSai "$falha A versao anterior tambem nao subiu; veja os eventos acima."
    }
    ErroSai "$falha Veja os eventos acima."
}

# Inicia o servico e espera a API responder (ate 30 s: a primeira subida gera as chaves do Data
# Protection). Devolve $null se deu certo, ou a descricao da falha.
function Iniciar-E-Testar {
    try {
        Start-Service -Name $NomeServico
    } catch {
        return "O servico $NomeServico nao iniciou ($($_.Exception.Message))."
    }
    $urlTeste = "https://127.0.0.1:5443/api/auth/estado"
    for ($i = 0; $i -lt 15; $i++) {
        $resposta = & curl.exe -k -fs $urlTeste   # sem -S: nada no stderr (vira erro no 5.1)
        if ($LASTEXITCODE -eq 0 -and $resposta) {
            $global:LASTEXITCODE = 0
            Ok "servico iniciado; API respondeu: $resposta"
            return $null
        }
        Start-Sleep -Seconds 2
    }
    $global:LASTEXITCODE = 0
    $ocupante = Quem-UsaPorta 5443
    if ($ocupante -and $ocupante -notmatch '^(dotnet|Financas\.Api) ') {
        return "A API nao respondeu em $urlTeste depois de 30 s. A porta 5443 esta em uso por ${ocupante}: feche esse programa."
    }
    return "A API nao respondeu em $urlTeste depois de 30 s."
}

# Copia para a tela (e o log) os eventos do Windows desde $desde: excecao da API (.NET Runtime,
# Application Error) e erros do Gerenciador de Servicos.
function Mostrar-EventosServico([datetime]$desde) {
    $eventos = @()
    $eventos += Get-WinEvent -FilterHashtable @{ LogName = "Application"; StartTime = $desde } -ErrorAction SilentlyContinue |
        Where-Object { $_.ProviderName -match "\.NET Runtime|Application Error|Financas" }
    $eventos += Get-WinEvent -FilterHashtable @{ LogName = "System"; StartTime = $desde; ProviderName = "Service Control Manager" } -ErrorAction SilentlyContinue
    $eventos = @($eventos | Where-Object { $_ })   # sem eventos, o Get-WinEvent devolve $null
    if ($eventos.Count -eq 0) { Aviso "Nenhum evento do Windows registrado para o servico."; return }
    Write-Host ""
    Write-Host "    Eventos do Windows desde $($desde.ToString('HH:mm:ss')):" -ForegroundColor Yellow
    $eventos | Sort-Object TimeCreated | Select-Object -Last 10 | ForEach-Object {
        Write-Host "    [$($_.TimeCreated.ToString('HH:mm:ss'))] $($_.ProviderName): $($_.Message)"
    }
}

# ================================================================= Principal
Garantir-Pasta $Instaladores
$log = Join-Path $Instaladores "instalacao-windows.log"
Start-Transcript -Path $log -Append | Out-Null   # -Append: rodar de novo nao apaga o log da falha
$codigo = 0

# Desliga o QuickEdit desta janela: um clique no console do Windows entra no modo "Selecionar" e
# congela tudo que escreve nela, e a instalacao parece travada. Sem console (ex.: CI), nao faz nada.
try {
    Add-Type -Namespace Financas -Name Console -MemberDefinition @'
[DllImport("kernel32.dll")] public static extern IntPtr GetStdHandle(int n);
[DllImport("kernel32.dll")] public static extern bool GetConsoleMode(IntPtr h, out uint m);
[DllImport("kernel32.dll")] public static extern bool SetConsoleMode(IntPtr h, uint m);
'@
    $entrada = [Financas.Console]::GetStdHandle(-10)
    $modo = 0
    if ([Financas.Console]::GetConsoleMode($entrada, [ref]$modo)) {
        # tira ENABLE_QUICK_EDIT_MODE (0x40), mantem ENABLE_EXTENDED_FLAGS (0x80)
        [void][Financas.Console]::SetConsoleMode($entrada, (($modo -band (-bnot 0x40)) -bor 0x80))
    }
} catch { }

try {
    $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        ErroSai "Rode como Administrador (o instalar-windows.cmd pede isso sozinho)."
    }

    $arquitetura = $env:PROCESSOR_ARCHITEW6432
    if (-not $arquitetura) { $arquitetura = $env:PROCESSOR_ARCHITECTURE }
    switch ($arquitetura) {
        "AMD64" { $Arch = "x64" }
        "ARM64" { $Arch = "arm64" }
        default { ErroSai "Arquitetura nao suportada: $arquitetura" }
    }

    Write-Host "FinancasLocal - instalacao no Windows em $Raiz ($Arch)"

    # Como no comum.sh, o sqlite3 portatil vale desde o inicio. O .NET e o Node portateis entram no
    # PATH depois da verificacao (Configurar-AmbienteBuild), para o .NET portatil ser atualizado
    # quando sair versao nova.
    $pastaSqlite = Join-Path $Ferramentas "sqlite"
    if (Test-Path -LiteralPath $pastaSqlite) { Adicionar-Path $pastaSqlite }

    Preparar-Ambiente
    Criar-Banco
    Publicar

    # #300: o Firefox nao usa os certificados do Windows por padrao e mostra um aviso no site.
    if (Test-Path -LiteralPath "HKLM:\SOFTWARE\Mozilla\Mozilla Firefox") {
        Aviso "Firefox instalado: por padrao ele nao usa os certificados do Windows e mostra um aviso no site."
        Aviso "Para aceitar: em about:config, deixe security.enterprise_roots.enabled como true (ou use o Edge/Chrome)."
    }

    Write-Host ""
    Write-Host "Instalacao concluida. Abra https://localhost:5443 no navegador." -ForegroundColor Green
    Write-Host "Gerenciador (ver se o site esta no ar, ligar, desligar, copiar o banco): powershell -ExecutionPolicy Bypass -File scripts\windows\instalar-gerenciador.ps1"

} catch {
    Write-Host ""
    Write-Host "    ERRO em `"$script:EtapaAtual`": $($_.Exception.Message)" -ForegroundColor Red
    if (-not $_.Exception.Data["Previsto"]) {
        # Erro inesperado (nao veio do ErroSai): a linha do script ajuda a achar a causa.
        Write-Host $_.InvocationInfo.PositionMessage -ForegroundColor DarkGray
    }
    Write-Host "    Log completo em $log"
    $codigo = 1
} finally {
    Stop-Transcript | Out-Null
}
exit $codigo
