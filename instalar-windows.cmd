@echo off
rem =====================================================================
rem FinancasLocal - instalador para Windows (duplo clique)
rem
rem  - Pede elevacao (Administrador), necessaria para registrar o servico
rem  - Roda scripts\instalar-windows.ps1 no Windows PowerShell, que ja vem
rem    com o Windows: nao precisa de Git, bash nem winget
rem
rem Pode ser executado de novo para atualizar a instalacao.
rem Repassa os argumentos ao script (ex.: instalar-windows.cmd -PularBuild).
rem =====================================================================
setlocal
rem Caminho passado ao PowerShell por variavel de ambiente: evita problemas de
rem aspas/apostrofos no caminho da pasta.
set "ARQUIVO_CMD=%~f0"
set "ARGUMENTOS=%*"

net session >nul 2>&1
if errorlevel 1 (
    echo Pedindo permissao de Administrador...
    powershell.exe -NoProfile -Command "$a = @{FilePath=$env:ARQUIVO_CMD; Verb='RunAs'}; if ($env:ARGUMENTOS) { $a.ArgumentList = $env:ARGUMENTOS }; Start-Process @a" 2>nul
    if errorlevel 1 (
        echo Permissao de Administrador negada. Instalacao cancelada.
        pause
        exit /b 1
    )
    exit /b 0
)

cd /d "%~dp0"

rem -ExecutionPolicy Bypass vale so para este processo: nao muda a politica do computador.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\instalar-windows.ps1" %*
if errorlevel 1 (
    echo.
    echo A instalacao falhou. Veja a mensagem de ERRO acima e o log em instaladores\instalacao-windows.log.
    pause
    exit /b 1
)
echo.
pause
exit /b 0
