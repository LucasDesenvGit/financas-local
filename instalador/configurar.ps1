<#
    Acao personalizada do FinancasLocal.msi (#119, ADR 0029). O instalador roda este script como SYSTEM
    (WixQuietExec, sem janela; a saida vai para o log do msiexec).

    -Acao instalar     (instalacao, reparo e atualizacao; antes de o servico ser registrado e iniciado)
        1. Pasta de dados %ProgramData%\FinancasLocal\dados, so para SYSTEM e Administradores.
        2. Instalacao antiga pelos scripts (servico FinancasLocal apontando para outra pasta): para o servico,
           copia banco e chaves para a pasta nova (sem apagar nada da antiga) e remove o servico antigo,
           para o .msi poder registrar o dele.
        3. Certificado HTTPS de 127.0.0.1/localhost: reaproveita o atual ou gera um novo (5 anos), confia
           nele nas Autoridades Raiz da maquina e grava dados\configuracao.json (caminho e senha do .pfx).
        4. Porta 5443 (#300): se outro programa estiver nela, para com a mensagem no log do msiexec. O servico
           e vital no .msi: sem isso, ele nao sobe e a instalacao falha com um erro generico do Windows Installer.
    -Acao desinstalar  (so na desinstalacao de verdade, nao na troca de versao)
        Tira o certificado das Autoridades Raiz. Os dados ficam, salvo -ApagarDados 1.

    Compativel com o Windows PowerShell 5.1 e sem acentos (o arquivo e lido na pagina de codigo do sistema).
#>
param(
    [ValidateSet("instalar", "desinstalar")][string]$Acao = "instalar",
    [string]$ApagarDados = ""
)

$ErrorActionPreference = "Stop"
$NomeServico = "FinancasLocal"
$Raiz = Join-Path $env:ProgramData "FinancasLocal"
$Dados = Join-Path $Raiz "dados"
$PastaPrograma = Split-Path -Parent $PSScriptRoot
$Configuracao = Join-Path $Dados "configuracao.json"
$Pfx = Join-Path $Dados "certificado\financaslocal.pfx"
$Url = "https://127.0.0.1:5443"

function Log([string]$texto) { Write-Output ("[configurar] " + $texto) }

function Garantir-Pasta([string]$pasta) {
    if (-not (Test-Path -LiteralPath $pasta)) { New-Item -ItemType Directory -Path $pasta -Force | Out-Null }
}

function Gravar-Json([string]$caminho, $objeto) {
    $json = ($objeto | ConvertTo-Json -Depth 10) + "`n"
    [IO.File]::WriteAllText($caminho, $json, (New-Object Text.UTF8Encoding $false))
}

# ------------------------------------------------------------------ 1) Pasta de dados
function Preparar-Dados {
    foreach ($p in $Raiz, $Dados, (Join-Path $Dados "sql"), (Join-Path $Dados "chaves"), (Join-Path $Dados "backups"),
                   (Join-Path $Dados "certificado")) { Garantir-Pasta $p }
    # So SYSTEM (S-1-5-18) e Administradores (S-1-5-32-544), por SID: o nome do grupo muda com o idioma.
    # So na raiz e sem /T: o que ja existe la dentro herda sozinho. Com /T, os arquivos de uma instalacao anterior
    # perdiam a heranca e ficavam sem permissao nenhuma (nem o SYSTEM lia o banco e o certificado).
    & icacls.exe $Raiz /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" /Q | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "icacls falhou ($LASTEXITCODE) ao proteger $Raiz." }
    Log "pasta de dados protegida: $Dados"
}

# ------------------------------------------------------------------ 2) Instalacao antiga pelos scripts
function Caminho-Do-Servico {
    $servico = Get-CimInstance -ClassName Win32_Service -Filter "Name='$NomeServico'" -ErrorAction SilentlyContinue
    if (-not $servico) { return $null }
    return $servico.PathName
}

# Caminho do appsettings antigo: relativo a pasta da API antiga quando nao for absoluto.
function Caminho-Absoluto([string]$caminho, [string]$base) {
    if ([IO.Path]::IsPathRooted($caminho)) { return $caminho }
    return [IO.Path]::GetFullPath((Join-Path $base $caminho))
}

function Migrar-Instalacao-Antiga {
    $linha = Caminho-Do-Servico
    if (-not $linha) { return }
    if ($linha -like ("*" + $PastaPrograma.TrimEnd('\') + "*")) { return }   # ja e o servico do .msi

    Log "servico $NomeServico da instalacao pelos scripts encontrado: $linha"
    # Ex.: "D:\...\dotnet.exe" "D:\FinancasLocal\publicado\api\Financas.Api.dll"  ou  ...\Financas.Api.exe
    $m = [regex]::Match($linha, '([A-Za-z]:\\[^"]*?Financas\.Api\.(dll|exe))')
    $bancoAntigo = $null; $chavesAntigas = $null
    if ($m.Success) {
        $pastaAntiga = Split-Path -Parent $m.Groups[1].Value
        $config = Join-Path $pastaAntiga "appsettings.Production.json"
        if (Test-Path -LiteralPath $config) {
            try {
                $json = Get-Content -LiteralPath $config -Raw -Encoding UTF8 | ConvertFrom-Json
                if ($json.Banco -and $json.Banco.Caminho) { $bancoAntigo = Caminho-Absoluto ($json.Banco.Caminho -replace '/', '\') $pastaAntiga }
                if ($json.Seguranca -and $json.Seguranca.PastaChaves) { $chavesAntigas = Caminho-Absoluto ($json.Seguranca.PastaChaves -replace '/', '\') $pastaAntiga }
            } catch { Log "aviso: nao deu para ler $config ($($_.Exception.Message))" }
        }
    }

    # Sem o banco antigo, nao remove nada: o servico antigo volta a rodar (o msiexec desfaz a instalacao) e os
    # dados continuam onde estavam. Melhor do que comecar com um banco vazio sem a pessoa entender por que.
    $bancoNovo = Join-Path $Dados "sql\FinancasLocal.db"
    if (-not ($bancoAntigo -and (Test-Path -LiteralPath $bancoAntigo)) -and -not (Test-Path -LiteralPath $bancoNovo)) {
        throw ("Ha uma instalacao do Financas Local feita pelos scripts ($linha), mas o banco dela nao foi encontrado " +
               "(procurado em: $bancoAntigo). Nada foi mudado. Confira o appsettings.Production.json da instalacao antiga " +
               "ou faca um backup do banco e remova o servico antigo antes de instalar.")
    }

    $servico = Get-Service -Name $NomeServico -ErrorAction SilentlyContinue
    if ($servico -and $servico.Status -ne "Stopped") {
        Stop-Service -Name $NomeServico -Force
        (Get-Service -Name $NomeServico).WaitForStatus("Stopped", [TimeSpan]::FromSeconds(60))
        Log "servico antigo parado"
    }

    if ($bancoAntigo -and (Test-Path -LiteralPath $bancoAntigo)) {
        if (Test-Path -LiteralPath $bancoNovo) {
            Log "banco ja existe em ${bancoNovo}; o da instalacao antiga nao foi copiado"
        } else {
            foreach ($sufixo in "", "-wal", "-shm") {
                if (Test-Path -LiteralPath ($bancoAntigo + $sufixo)) { Copy-Item -LiteralPath ($bancoAntigo + $sufixo) -Destination ($bancoNovo + $sufixo) -Force }
            }
            Log "banco copiado de $bancoAntigo"
        }
    } else {
        Log "banco da instalacao antiga nao encontrado; fica o que ja estava em $bancoNovo"
    }

    $chavesNovas = Join-Path $Dados "chaves"
    if ($chavesAntigas -and (Test-Path -LiteralPath $chavesAntigas) -and -not (Get-ChildItem -LiteralPath $chavesNovas -Filter "key-*.xml" -ErrorAction SilentlyContinue)) {
        Copy-Item -Path (Join-Path $chavesAntigas "key-*.xml") -Destination $chavesNovas -Force -ErrorAction SilentlyContinue
        Log "chaves de protecao de dados copiadas de $chavesAntigas"
    }

    # O .msi registra o servico com o mesmo nome; a pasta antiga fica como estava.
    & sc.exe delete $NomeServico | Out-Null
    for ($i = 0; $i -lt 30 -and (Caminho-Do-Servico); $i++) { Start-Sleep -Seconds 1 }
    $nota = "Migrado em $(Get-Date -Format s) da instalacao pelos scripts.`r`nServico antigo: $linha`r`nBanco antigo: $bancoAntigo`r`nChaves antigas: $chavesAntigas`r`nA pasta antiga nao foi apagada.`r`n"
    [IO.File]::WriteAllText((Join-Path $Dados "migrado-da-instalacao-por-scripts.txt"), $nota, (New-Object Text.UTF8Encoding $false))
    Log "servico antigo removido (a pasta antiga nao foi apagada)"
}

# ------------------------------------------------------------------ 3) Certificado HTTPS
# So a parte publica interessa aqui: carregar sem gravar a chave privada em nenhum repositorio de chaves do Windows.
function Ler-Pfx([string]$senha) {
    $flags = [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::EphemeralKeySet
    return New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($Pfx, $senha, $flags)
}

function Senha-Atual {
    if (-not (Test-Path -LiteralPath $Configuracao)) { return $null }
    try { return (Get-Content -LiteralPath $Configuracao -Raw -Encoding UTF8 | ConvertFrom-Json).Kestrel.Endpoints.Https.Certificate.Password }
    catch { return $null }
}

function Preparar-Certificado {
    $senha = Senha-Atual
    $reaproveitar = $false
    if ($senha -and (Test-Path -LiteralPath $Pfx)) {
        try {
            $atual = Ler-Pfx $senha
            $reaproveitar = $atual.NotAfter -gt (Get-Date).AddDays(30)
        } catch { $reaproveitar = $false }
    }

    if ($reaproveitar) {
        Log "certificado atual reaproveitado"
    } else {
        $bytes = New-Object byte[] 24
        [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
        $senha = [Convert]::ToBase64String($bytes)
        # IP no SAN: o navegador exige para https://127.0.0.1.
        $cert = New-SelfSignedCertificate -Subject "CN=localhost" -FriendlyName $NomeServico `
            -TextExtension @("2.5.29.17={text}DNS=localhost&IPAddress=127.0.0.1&IPAddress=::1") `
            -KeyAlgorithm RSA -KeyLength 2048 -HashAlgorithm SHA256 -KeyExportPolicy Exportable `
            -Type SSLServerAuthentication -NotAfter (Get-Date).AddYears(5) -CertStoreLocation "Cert:\LocalMachine\My"
        try {
            $segura = ConvertTo-SecureString $senha -AsPlainText -Force
            if (Test-Path -LiteralPath $Pfx) { Remove-Item -LiteralPath $Pfx -Force }
            Export-PfxCertificate -Cert $cert -FilePath $Pfx -Password $segura | Out-Null
        } finally {
            Remove-Item -LiteralPath ("Cert:\LocalMachine\My\" + $cert.Thumbprint) -Force -ErrorAction SilentlyContinue
        }
        Log "certificado novo gerado ($($cert.Thumbprint))"
    }

    # Confia so na parte publica, na loja da maquina (vale para todas as contas, como no instalar-windows.ps1).
    $completo = Ler-Pfx $senha
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
    Log "certificado confiavel para este computador"

    $config = [ordered]@{
        Kestrel = [ordered]@{
            Endpoints = [ordered]@{
                Https = [ordered]@{
                    Url = $Url
                    Certificate = [ordered]@{ Path = ($Pfx -replace '\\', '/'); Password = $senha }
                }
            }
        }
    }
    Gravar-Json $Configuracao $config
    Log "configuracao gravada em $Configuracao"
}

# ------------------------------------------------------------------ 4) Porta ocupada (#300)
# O servico do Financas Local ja parou (troca de versao) ou foi removido (migracao): quem estiver na porta e outro
# programa. Sem acento nem dois-pontos colado em variavel (o arquivo e lido na pagina de codigo do sistema).
function Conferir-Porta {
    $conexao = Get-NetTCPConnection -LocalPort 5443 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $conexao) { Log "porta 5443 livre"; return }
    $processo = Get-Process -Id $conexao.OwningProcess -ErrorAction SilentlyContinue
    $nome = if ($processo) { $processo.ProcessName } else { "programa desconhecido" }
    if ($nome -eq "Financas.Api") { Log "porta 5443 com o proprio Financas Local (PID $($conexao.OwningProcess))"; return }
    throw ("A porta 5443 esta em uso por $nome (PID $($conexao.OwningProcess)). O site do Financas Local nao sobe " +
           "enquanto ela estiver ocupada. Feche esse programa (ou pare o servico dele) e rode o instalador de novo.")
}

# ------------------------------------------------------------------ Desinstalar
function Desinstalar {
    $loja = New-Object System.Security.Cryptography.X509Certificates.X509Store("Root", "LocalMachine")
    $loja.Open("ReadWrite")
    try {
        foreach ($c in @($loja.Certificates)) { if ($c.FriendlyName -eq $NomeServico) { $loja.Remove($c) } }
    } finally { $loja.Close() }
    Log "certificado retirado das Autoridades Raiz"

    if ($ApagarDados -eq "1") {
        Remove-Item -LiteralPath $Raiz -Recurse -Force -ErrorAction SilentlyContinue
        Log "dados apagados (APAGARDADOS=1): $Raiz"
    } else {
        Log "dados mantidos em $Dados (para apagar: msiexec /x FinancasLocal.msi APAGARDADOS=1)"
    }
}

try {
    if ($Acao -eq "instalar") {
        Preparar-Dados
        Migrar-Instalacao-Antiga
        Conferir-Porta
        Preparar-Certificado
    } else {
        Desinstalar
    }
    exit 0
} catch {
    Write-Output ("[configurar] ERRO: " + $_.Exception.Message)
    Write-Output ("[configurar] onde: " + $_.InvocationInfo.PositionMessage)
    Write-Output ("[configurar] pilha: " + $_.ScriptStackTrace)
    exit 1
}
