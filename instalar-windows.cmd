@echo off
rem =====================================================================
rem FinancasLocal - instalador para Windows (duplo clique)
rem
rem  - Pede elevacao (Administrador), necessaria para registrar o servico
rem  - Procura o Git Bash; se nao houver, instala o Git for Windows via winget
rem  - Roda scripts/instalar.sh (as 3 etapas) no Git Bash
rem
rem Pode ser executado de novo para atualizar a instalacao.
rem =====================================================================
setlocal
rem Caminho passado ao PowerShell por variavel de ambiente: evita problemas de
rem aspas/apostrofos no caminho da pasta.
set "ARQUIVO_CMD=%~f0"

net session >nul 2>&1
if errorlevel 1 (
    echo Pedindo permissao de Administrador...
    powershell -NoProfile -Command "Start-Process -FilePath $env:ARQUIVO_CMD -Verb RunAs" 2>nul
    if errorlevel 1 (
        echo Permissao de Administrador negada. Instalacao cancelada.
        pause
        exit /b 1
    )
    exit /b 0
)

cd /d "%~dp0"

call :procurar_bash
if defined GIT_BASH goto rodar

echo Git Bash nao encontrado. Instalando o Git for Windows pelo winget...
where winget >nul 2>&1
if errorlevel 1 goto sem_winget
winget install --id Git.Git -e --source winget --accept-source-agreements --accept-package-agreements
call :procurar_bash
if defined GIT_BASH goto rodar
echo.
echo Nao foi possivel encontrar o Git Bash depois da instalacao.
echo Instale manualmente em https://git-scm.com/download/win e rode este arquivo de novo.
pause
exit /b 1

:sem_winget
echo.
echo O winget nao esta disponivel neste Windows.
echo Instale o Git for Windows em https://git-scm.com/download/win e rode este arquivo de novo.
pause
exit /b 1

:rodar
echo Usando %GIT_BASH%
"%GIT_BASH%" scripts/instalar.sh
if errorlevel 1 (
    echo.
    echo A instalacao falhou. Veja a mensagem de ERRO acima.
    pause
    exit /b 1
)
echo.
pause
exit /b 0

rem Procura o bash.exe do Git for Windows (nao o bash.exe do WSL em System32).
rem Fica fora de blocos com parenteses porque "Program Files (x86)" quebraria o bloco.
:procurar_bash
set "GIT_BASH="
if exist "%ProgramFiles%\Git\bin\bash.exe" set "GIT_BASH=%ProgramFiles%\Git\bin\bash.exe"
if defined GIT_BASH exit /b 0
if exist "%ProgramFiles(x86)%\Git\bin\bash.exe" set "GIT_BASH=%ProgramFiles(x86)%\Git\bin\bash.exe"
if defined GIT_BASH exit /b 0
if exist "%LocalAppData%\Programs\Git\bin\bash.exe" set "GIT_BASH=%LocalAppData%\Programs\Git\bin\bash.exe"
if defined GIT_BASH exit /b 0
for /f "delims=" %%G in ('where git.exe 2^>nul') do if not defined GIT_BASH if exist "%%~dpG..\bin\bash.exe" set "GIT_BASH=%%~dpG..\bin\bash.exe"
exit /b 0
