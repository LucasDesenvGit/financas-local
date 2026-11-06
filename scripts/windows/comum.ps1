# =====================================================================
# FinancasLocal - o que os scripts de scripts\windows\ repetiam: pastas e mensagens (#655).
# Carregado com '. (Join-Path $PSScriptRoot "comum.ps1")' (nao execute direto). Como o comum.sh no bash.
# So ASCII e compativel com o Windows PowerShell 5.1.
# O instalador do .msi (instalador\configurar.ps1) NAO usa este arquivo: o .msi teria de levar ele junto.
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
# Erros previstos (com mensagem pronta) sao marcados para o catch final nao mostrar a linha do
# script; os inesperados mostram, para achar a causa.
function ErroSai([string]$texto) {
    $erro = New-Object System.Exception $texto
    $erro.Data["Previsto"] = $true
    throw $erro
}
