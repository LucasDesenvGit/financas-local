@echo off
rem =====================================================================
rem FinancasLocal - para tudo no Windows (duplo clique)
rem
rem  - Pede elevacao (Administrador), necessaria para parar o servico
rem  - Roda scripts\parar-windows.ps1 no Windows PowerShell, que ja vem
rem    com o Windows: nao precisa de Git nem bash
rem  - Para o servico FinancasLocal, a API e o front de desenvolvimento e
rem    confere se o banco ficou livre. A janela fica aberta no final.
rem
rem Repassa os argumentos ao script, ex.:
rem   parar-windows.cmd -Desativar      (nao sobe sozinho no proximo boot)
rem   parar-windows.cmd -Compiladores   (tambem encerra os servidores de build)
rem Para subir de novo: instalar-windows.cmd
rem =====================================================================
setlocal
set "ARQUIVO_CMD=%~f0"
set "ARGUMENTOS=%*"

net session >nul 2>&1
if errorlevel 1 (
    echo Pedindo permissao de Administrador...
    powershell.exe -NoProfile -Command "$a = @{FilePath=$env:ARQUIVO_CMD; Verb='RunAs'}; if ($env:ARGUMENTOS) { $a.ArgumentList = $env:ARGUMENTOS }; Start-Process @a" 2>nul
    if errorlevel 1 (
        echo Permissao de Administrador negada. Nada foi parado.
        pause
        exit /b 1
    )
    exit /b 0
)

cd /d "%~dp0"

rem -ExecutionPolicy Bypass vale so para este processo: nao muda a politica do computador.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\parar-windows.ps1" %*
set "RESULTADO=%errorlevel%"
if not "%FINANCAS_SEM_INTERACAO%"=="1" pause
exit /b %RESULTADO%
