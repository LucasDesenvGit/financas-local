<#
.SYNOPSIS
    Cria o repositorio no GitHub, envia as branches main e develop e aplica as protecoes.

.DESCRIPTION
    Se o GitHub CLI (gh) nao estiver instalado, o script instala pelo winget.
    Na primeira vez, o script pede o login (gh auth login) pelo navegador.

    O que e configurado:
      - repositorio PRIVADO (dados financeiros nunca sobem: veja o .gitignore)
      - branch padrao: main; develop para integracao
      - main e develop protegidas: so recebem alteracoes por pull request, com o CI (build) passando,
        sem force push e sem exclusao. Voce (administrador) pode contornar quando precisar.
      - exclusao automatica da branch depois do merge
      - alertas de vulnerabilidade e correcoes automaticas do Dependabot

    Observacao: no plano gratuito do GitHub, protecao de branch em repositorio PRIVADO nao esta disponivel.
    Nesse caso o script avisa e segue com o resto (as regras ficam documentadas em CONTRIBUINDO.md).

.EXAMPLE
    .\configurar-github.ps1 -Usuario meu-usuario
    .\configurar-github.ps1 -Usuario meu-usuario -Repositorio financas-local
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Usuario,
    [string]$Repositorio = "financas-local"
)

$ErrorActionPreference = "Stop"
$raiz = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $raiz

function Executar([string]$descricao, [scriptblock]$comando) {
    Write-Host "-> $descricao" -ForegroundColor Cyan
    & $comando
    if ($LASTEXITCODE -ne 0) { throw "Falhou: $descricao (codigo $LASTEXITCODE)" }
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw "Git nao encontrado. Instale: winget install --id Git.Git -e" }
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    # Instala o GitHub CLI pelo winget (pacote oficial publicado pela GitHub no repositorio da Microsoft).
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        throw "GitHub CLI nao encontrado e o winget nao esta disponivel. Baixe em https://cli.github.com e rode o script de novo."
    }
    Executar "Instalando o GitHub CLI (winget)" {
        winget install --id GitHub.cli --exact --source winget --accept-package-agreements --accept-source-agreements --silent
    }
    # Recarrega o PATH desta sessao para encontrar o gh recem-instalado.
    $env:PATH = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        $padrao = Join-Path $env:ProgramFiles "GitHub CLI"
        if (Test-Path (Join-Path $padrao "gh.exe")) { $env:PATH = "$padrao;$env:PATH" }
        else { throw "GitHub CLI instalado, mas nao encontrado no PATH. Abra um novo PowerShell e rode o script de novo." }
    }
}
if (-not (Test-Path (Join-Path $raiz ".git"))) { throw "Esta pasta nao e um repositorio Git: $raiz" }

# Repositorio criado fora do Windows: ajustes para o Git do Windows.
git config core.autocrlf false
git config core.filemode false
git config core.symlinks false
git config core.ignorecase true
$seguro = $raiz -replace '\\', '/'
cmd /c "git config --global --add safe.directory `"$seguro`" >nul 2>&1"

cmd /c "gh auth status >nul 2>&1"
if ($LASTEXITCODE -ne 0) { Executar "Login no GitHub" { gh auth login --hostname github.com --git-protocol https --web } }

$nomeCompleto = "$Usuario/$Repositorio"
cmd /c "gh repo view $nomeCompleto >nul 2>&1"
if ($LASTEXITCODE -ne 0) {
    Executar "Criando repositorio privado $nomeCompleto" {
        gh repo create $nomeCompleto --private --disable-wiki --description "Gerenciador financeiro pessoal local (.NET + Angular + SQL Server) com Open Finance"
    }
}

$remoto = "https://github.com/$nomeCompleto.git"
cmd /c "git remote get-url origin >nul 2>&1"
if ($LASTEXITCODE -eq 0) { git remote set-url origin $remoto } else { git remote add origin $remoto }

Executar "Enviando main" { git push -u origin main }
Executar "Enviando develop" { git push -u origin develop }
Executar "Enviando tags" { git push origin --tags }

Executar "Configuracoes do repositorio" {
    gh repo edit $nomeCompleto --default-branch main --delete-branch-on-merge --enable-squash-merge `
        --enable-merge-commit --enable-rebase-merge=false --enable-projects=false --enable-wiki=false
}

# Protecao das branches (exige o check "build" do CI).
$protecao = @{
    required_status_checks        = @{ strict = $true; contexts = @("build") }
    enforce_admins                = $false
    required_pull_request_reviews = @{ required_approving_review_count = 0; dismiss_stale_reviews = $true }
    restrictions                  = $null
    allow_force_pushes            = $false
    allow_deletions               = $false
    required_conversation_resolution = $true
} | ConvertTo-Json -Depth 5
$arquivo = Join-Path $env:TEMP "protecao-branch.json"
[IO.File]::WriteAllText($arquivo, $protecao)

foreach ($branch in @("main", "develop")) {
    Write-Host "-> Protegendo $branch" -ForegroundColor Cyan
    $saida = cmd /c "gh api -X PUT repos/$nomeCompleto/branches/$branch/protection --input `"$arquivo`" 2>&1"
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "Nao foi possivel proteger '$branch' (no plano gratuito, repositorio privado nao tem protecao de branch). Detalhe: $saida"
    }
}
Remove-Item $arquivo -ErrorAction SilentlyContinue

Write-Host "-> Alertas de seguranca do Dependabot" -ForegroundColor Cyan
cmd /c "gh api -X PUT repos/$nomeCompleto/vulnerability-alerts >nul 2>&1"
cmd /c "gh api -X PUT repos/$nomeCompleto/automated-security-fixes >nul 2>&1"

Write-Host ""
Write-Host "Pronto: https://github.com/$nomeCompleto" -ForegroundColor Green
Write-Host "Fluxo de trabalho: CONTRIBUINDO.md"
