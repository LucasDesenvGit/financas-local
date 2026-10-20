<#
.SYNOPSIS
    Atualiza a instalacao de DESENVOLVIMENTO pelo git (#154, ADR 0040): o equivalente, no Windows, ao
    scripts/geral/atualizar.sh.

.DESCRIPTION
    So para quem desenvolve: a pasta do projeto e um clone git na branch develop. Traz o develop do GitHub (so
    avanca, nunca descarta nada) e, sem -SoCodigo, roda o instalar-windows.ps1. A instalacao de uma versao
    publicada nao usa este script.

    Recusa, sem mexer em nada: pasta que nao e clone git, git ausente, branch diferente de develop, alteracoes
    locais ou commits locais que nao estao no develop do GitHub.

    Codigos de saida (lidos pelo gerenciador): 0 atualizado; 10 ja estava na ultima; 2 recusado; 1 erro.

    O gerenciador roda com -SoCodigo, como o usuario (o git nunca roda como Administrador na pasta do usuario), e
    depois reinstala como Administrador, num passo separado (UAC).

.EXAMPLE
    .\scripts\windows\atualizar-windows.ps1 -SoCodigo
    .\scripts\windows\atualizar-windows.ps1              # num PowerShell como Administrador: busca e reinstala
#>
param(
    [switch]$SoCodigo
)

$ErrorActionPreference = "Stop"
$Raiz = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Branch = "develop"

function Etapa([string]$texto) { Write-Host ""; Write-Host "==> $texto" -ForegroundColor Cyan }
function Ok([string]$texto) { Write-Host "    OK  $texto" -ForegroundColor Green }
function Recusa([string]$motivo, [string]$oQueFazer) {
    Write-Host "    !!  $motivo" -ForegroundColor Yellow
    if ($oQueFazer) { Write-Host "        $oQueFazer" }
    exit 2
}
function Erro([string]$texto) { Write-Host "    ERRO  $texto" -ForegroundColor Red; exit 1 }

# git com a saida em texto; $LASTEXITCODE diz se deu certo (stderr nao vira excecao). O nome nao pode ser "Git":
# o PowerShell nao diferencia maiusculas, e a funcao esconderia o git.exe (chamaria a si mesma).
function RodarGit {
    $anterior = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try { $saida = & git.exe -C $Raiz @args 2>&1 | ForEach-Object { "$_" } }
    finally { $ErrorActionPreference = $anterior }
    return $saida
}

Etapa "Conferindo a pasta do projeto"
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Recusa "O git nao esta instalado." "Instale o Git for Windows, ou atualize pelos scripts a mao." }
RodarGit rev-parse --is-inside-work-tree | Out-Null
if ($LASTEXITCODE -ne 0) { Recusa "A pasta $Raiz nao e um clone git." "Esta atualizacao e so para a instalacao de desenvolvimento, feita a partir de um clone do repositorio." }

$atual = (RodarGit rev-parse --abbrev-ref HEAD | Select-Object -First 1)
if ($atual -ne $Branch) { Recusa "A pasta esta na branch `"$atual`", nao na $Branch." "Nada foi trocado. Para atualizar pelo develop: git switch $Branch (depois de guardar o que estiver fazendo)." }

$alteracoes = @(RodarGit status --porcelain --untracked-files=no | Where-Object { $_ })
if ($alteracoes.Count -gt 0) {
    $nomes = ($alteracoes | Select-Object -First 5 | ForEach-Object { $_.Substring(3) }) -join " "
    Recusa "Ha alteracoes locais em arquivos do repositorio; nada foi descartado." "Guarde (git stash) ou desfaca antes. Arquivos: $nomes"
}
Ok "clone na $Branch, sem alteracoes locais"

Etapa "Buscando o $Branch no GitHub"
RodarGit fetch --quiet origin $Branch | Out-Null
if ($LASTEXITCODE -ne 0) { Erro "Nao deu para buscar o develop (sem internet?). Nada mudou." }
$local = (RodarGit rev-parse HEAD | Select-Object -First 1)
$remoto = (RodarGit rev-parse "origin/$Branch" | Select-Object -First 1)
if ($local -eq $remoto) {
    Ok ("ja esta na ultima do $Branch (" + (RodarGit log -1 "--format=%h, %cd" --date=short | Select-Object -First 1) + ")")
    exit 10
}
RodarGit merge-base --is-ancestor HEAD "origin/$Branch" | Out-Null
if ($LASTEXITCODE -ne 0) { Recusa "Ha commits locais que nao estao no $Branch do GitHub; nada foi mudado." "Envie (git push) ou desfaca esses commits antes." }

Write-Host "    Novidades:"
RodarGit log --oneline --no-decorate "HEAD..origin/$Branch" | Select-Object -First 20 | ForEach-Object { Write-Host "      $_" }
RodarGit merge --ff-only --quiet "origin/$Branch" | Out-Null
if ($LASTEXITCODE -ne 0) { Erro "Nao deu para avancar para o develop novo." }
Ok ("codigo atualizado para " + (RodarGit log -1 "--format=%h" | Select-Object -First 1))

if ($SoCodigo) { exit 0 }

Etapa "Reinstalando (instalar-windows.ps1)"
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "instalar-windows.ps1")
exit $LASTEXITCODE
