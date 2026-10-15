# =====================================================================
# FinancasLocal - Backup do banco de dados
#
# Gera D:\FinancasLocal\backups\FinancasLocal_AAAAMMDD_HHMMSS.bak (com CHECKSUM)
# e mantem somente os N backups mais recentes.
#
# Uso (Windows PowerShell como Administrador):
#   D:\FinancasLocal\scripts\4-backup.ps1                 # backup agora
#   D:\FinancasLocal\scripts\4-backup.ps1 -Agendar        # agenda backup diario as 12:30
#   D:\FinancasLocal\scripts\4-backup.ps1 -Manter 30
#
# Dica: copie a pasta backups para um disco externo ou nuvem de sua confianca de tempos em tempos.
# Restauracao: SSMS > Databases > Restore Database > Device > selecione o .bak.
# =====================================================================
[CmdletBinding()]
param(
    [string]$InstanciaSql = '.\FINANCAS',
    [ValidateRange(1, 365)][int]$Manter = 14,
    [switch]$Agendar,
    [string]$Horario = '12:30'
)

. "$PSScriptRoot\comum.ps1"
Testar-Administrador
Validar-InstanciaSql $InstanciaSql

if ($Agendar) {
    $acao = New-ScheduledTaskAction -Execute 'powershell.exe' `
        -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" -InstanciaSql `"$InstanciaSql`" -Manter $Manter"
    $gatilho = New-ScheduledTaskTrigger -Daily -At $Horario
    $usuario = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    $principal = New-ScheduledTaskPrincipal -UserId $usuario -LogonType Interactive -RunLevel Highest
    $config = New-ScheduledTaskSettingsSet -StartWhenAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    Register-ScheduledTask -TaskName 'FinancasLocal - Backup diario' -Action $acao -Trigger $gatilho -Principal $principal `
        -Settings $config -Force | Out-Null
    Escrever-Ok "Backup diario agendado para $Horario (Agendador de Tarefas > 'FinancasLocal - Backup diario')."
    return
}

Garantir-Pasta $PastaBackups
$arquivo = Join-Path $PastaBackups ("FinancasLocal_{0}.bak" -f (Get-Date -Format 'yyyyMMdd_HHmmss'))

Escrever-Etapa "Fazendo backup em $arquivo"
$conexao = Abrir-ConexaoSql $InstanciaSql
try {
    $cmd = $conexao.CreateCommand()
    $cmd.CommandTimeout = 1800
    $cmd.CommandText = 'BACKUP DATABASE [FinancasLocal] TO DISK = @arquivo WITH INIT, CHECKSUM, NAME = N''FinancasLocal - backup completo'''
    [void]$cmd.Parameters.AddWithValue('@arquivo', $arquivo)
    [void]$cmd.ExecuteNonQuery()

    $cmd = $conexao.CreateCommand()
    $cmd.CommandTimeout = 1800
    $cmd.CommandText = 'RESTORE VERIFYONLY FROM DISK = @arquivo WITH CHECKSUM'
    [void]$cmd.Parameters.AddWithValue('@arquivo', $arquivo)
    [void]$cmd.ExecuteNonQuery()
} finally {
    $conexao.Dispose()
}
Escrever-Ok 'backup concluido e verificado'

$antigos = Get-ChildItem $PastaBackups -Filter 'FinancasLocal_*.bak' | Sort-Object LastWriteTime -Descending | Select-Object -Skip $Manter
foreach ($a in $antigos) { Remove-Item $a.FullName -Force }
if ($antigos) { Escrever-Ok "$(@($antigos).Count) backup(s) antigo(s) removido(s)" }
