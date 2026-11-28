# =====================================================================
# FinancasLocal - o que os scripts de scripts\windows\ repetiam: pastas e mensagens (#655).
# Carregado com '. (Join-Path $PSScriptRoot "comum.ps1")' (nao execute direto). Como o comum.sh no bash.
# So ASCII e compativel com o Windows PowerShell 5.1.
# O instalador do .msi (instalador\configurar.ps1) carrega este arquivo so para o Confiar-No-Certificado (#775) e o
# Proteger-Pasta (#68): o gerar-msi.ps1 o leva para instalador\. As pastas daqui nao valem la: o configurar.ps1 define
# as dele depois.
# =====================================================================

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
# Versao do sistema (ADR 0070): calculada pelo Go (src\backend\cmd\versao) a partir da base do Directory.Build.props e
# dos merges do git; sem o historico do git (copia baixada em .zip), sai a base com o sufixo -sem-git. Como o
# versao_do_sistema do comum.sh. $go: o go.exe, se nao estiver no PATH. No 5.1, stderr redirecionado com
# ErrorActionPreference=Stop vira excecao, entao a funcao roda em Continue (o valor volta ao fim dela).
function VersaoDoSistema([string]$go = "go") {
    $ErrorActionPreference = "Continue"
    Push-Location (Join-Path $Raiz "src\backend")
    try {
        $saida = @(& $go run ./cmd/versao 2>&1)
        $codigo = $LASTEXITCODE
    } finally { Pop-Location }
    $linhas = @($saida | ForEach-Object { if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.Exception.Message } else { "$_" } })
    if ($codigo -ne 0) { ErroSai ("Nao consegui calcular a versao (go run ./cmd/versao, em src\backend): " + ($linhas -join " ")) }
    return ($linhas -join "").Trim()
}

# Confia no certificado do site na loja da MAQUINA (Autoridades Raiz Confiaveis), sem janela: a loja do usuario pede um
# clique e, se quem elevou no UAC foi outra conta, poria o certificado so na conta do administrador. $pem e a parte
# publica que o preparar-https grava ao lado do .pfx (sem a chave nem a senha). O FriendlyName identifica o certificado
# antigo para troca-lo quando houver renovacao (e para a desinstalacao do .msi tira-lo).
function Confiar-No-Certificado([string]$pem) {
    $der = [Convert]::FromBase64String(((Get-Content -LiteralPath $pem) -notmatch "-----") -join "")
    $publico = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2(, $der)
    $publico.FriendlyName = $NomeServico
    $loja = New-Object System.Security.Cryptography.X509Certificates.X509Store("Root", "LocalMachine")
    $loja.Open("ReadWrite")
    try {
        foreach ($antigo in @($loja.Certificates)) {
            if ($antigo.FriendlyName -eq $NomeServico -and $antigo.Thumbprint -ne $publico.Thumbprint) { $loja.Remove($antigo) }
        }
        if (-not ($loja.Certificates | Where-Object { $_.Thumbprint -eq $publico.Thumbprint })) { $loja.Add($publico) }
    } finally { $loja.Close() }
}

# Pasta do servico (#68, item 1): o servico roda como SYSTEM, entao so SYSTEM (S-1-5-18) e Administradores (S-1-5-32-544)
# mudam o que ele executa ou le. Por SID: o nome do grupo muda com o idioma. Com -UsuariosLeem, Usuarios (S-1-5-32-545)
# leem e executam (o gerenciador, na conta de quem usa, le os appsettings da pasta da API).
# A permissao so na raiz e sem /T: o que ja existe la dentro herda sozinho. Com /T, os arquivos de uma instalacao
# anterior perdiam a heranca e ficavam sem permissao nenhuma (nem o SYSTEM lia o banco e o certificado).
# O dono passa a ser Administradores, com /T: o dono de um arquivo sempre pode trocar a permissao dele, e um arquivo
# criado pela conta comum (ex.: o 2-criar-banco.sh sem elevacao) devolveria o acesso a ela.
function Proteger-Pasta([string]$pasta, [switch]$UsuariosLeem) {
    $permissoes = @("*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F")
    if ($UsuariosLeem) { $permissoes += "*S-1-5-32-545:(OI)(CI)RX" }
    & icacls.exe $pasta /inheritance:r /grant:r @permissoes /Q | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "icacls falhou ($LASTEXITCODE) ao proteger $pasta." }
    & icacls.exe $pasta /setowner "*S-1-5-32-544" /T /C /Q | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "icacls falhou ($LASTEXITCODE) ao passar $pasta para os Administradores." }
}

# Erros previstos (com mensagem pronta) sao marcados para o catch final nao mostrar a linha do
# script; os inesperados mostram, para achar a causa.
function ErroSai([string]$texto) {
    $erro = New-Object System.Exception $texto
    $erro.Data["Previsto"] = $true
    throw $erro
}
