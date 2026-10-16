@echo off
rem =====================================================================
rem FinancasLocal - instalador para Windows (duplo clique)
rem
rem  - Pede elevacao (Administrador), necessaria para registrar o servico
rem  - Roda scripts\instalar-windows.ps1 no Windows PowerShell, que ja vem
rem    com o Windows: nao precisa de Git, bash nem winget
rem  - Deu certo: abre o site no navegador e fecha sozinho em 10 s.
rem    Deu errado: a janela fica aberta com o erro.
rem
rem Pode ser executado de novo para atualizar a instalacao.
rem Repassa os argumentos ao script (ex.: instalar-windows.cmd -PularBuild).
rem Sem ninguem na frente da tela (ex.: CI), defina FINANCAS_SEM_INTERACAO=1:
rem nao abre o navegador nem espera tecla/tempo no final.
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
if errorlevel 1 goto falhou

if "%FINANCAS_SEM_INTERACAO%"=="1" exit /b 0
echo.
echo Abrindo https://localhost:5443 no navegador...
rem explorer.exe abre o navegador padrao sem herdar a elevacao desta janela
rem (um "start" direto abriria o navegador como Administrador).
explorer.exe "https://localhost:5443"
echo Esta janela fecha em 10 segundos.
timeout /t 10 >nul
exit /b 0

:falhou
echo.
echo A instalacao falhou. Veja a mensagem de ERRO acima e o log em instaladores\instalacao-windows.log.
if not "%FINANCAS_SEM_INTERACAO%"=="1" pause
exit /b 1
