# =====================================================================
# FinancasLocal - 2) Criacao do banco de dados
#
#  - Cria o pool de aplicativos do IIS (a identidade dele e quem acessa o banco)
#  - Cria o banco FinancasLocal com a estrutura completa (sem migrations)
#  - Insere categorias padrao e configuracoes iniciais
#  - Da ao pool do IIS somente leitura/escrita de dados (sem permissao de alterar estrutura)
#
# Pode ser executado novamente: os scripts SQL sao idempotentes.
#
# Uso (Windows PowerShell como Administrador):
#   D:\FinancasLocal\scripts\2-criar-banco.ps1
#   D:\FinancasLocal\scripts\2-criar-banco.ps1 -InstanciaSql "localhost\SQLEXPRESS"   # outra instancia
# =====================================================================
[CmdletBinding()]
param(
    [string]$InstanciaSql = '.\FINANCAS',
    [string]$NomePool = 'FinancasLocal'
)

. "$PSScriptRoot\comum.ps1"
Testar-Administrador
Testar-WindowsPowerShell
Validar-InstanciaSql $InstanciaSql
Validar-NomeSimples $NomePool 'Nome do pool'

$pastaBanco = Join-Path $Raiz 'banco'

Escrever-Etapa 'Preparando o pool de aplicativos do IIS'
Garantir-PoolAplicativo $NomePool

Escrever-Etapa 'Ajustando permissoes das pastas de dados para o SQL Server'
$contaSql = Obter-ContaServicoSql $InstanciaSql
try {
    Conceder-Permissao $PastaSql "$($contaSql):(OI)(CI)F"
    Conceder-Permissao $PastaBackups "$($contaSql):(OI)(CI)M"
    Escrever-Ok "acesso concedido a $contaSql"
} catch {
    Escrever-Aviso "Nao foi possivel conceder acesso a $contaSql ($($_.Exception.Message)). Se a instancia usar outra conta de servico, conceda manualmente acesso a $PastaSql e $PastaBackups."
}

Escrever-Etapa "Conectando em $InstanciaSql"
$conexao = Abrir-ConexaoSql $InstanciaSql
try {
    $versao = Executar-Escalar $conexao "SELECT CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(50)) + ' ' + CAST(SERVERPROPERTY('Edition') AS nvarchar(100))"
    Escrever-Ok "SQL Server $versao"

    Escrever-Etapa 'Criando a estrutura (01-estrutura.sql)'
    $sql = [IO.File]::ReadAllText((Join-Path $pastaBanco '01-estrutura.sql'), [Text.Encoding]::UTF8)
    Executar-ScriptSql $conexao ($sql.Replace('{{PASTA_DADOS}}', $PastaSql))
    Escrever-Ok 'estrutura criada/atualizada'

    Escrever-Etapa 'Inserindo dados iniciais (02-dados-iniciais.sql)'
    $sql = [IO.File]::ReadAllText((Join-Path $pastaBanco '02-dados-iniciais.sql'), [Text.Encoding]::UTF8)
    Executar-ScriptSql $conexao $sql
    Escrever-Ok 'categorias e configuracoes padrao'

    Escrever-Etapa 'Concedendo acesso ao pool do IIS (03-permissoes.sql)'
    $sql = [IO.File]::ReadAllText((Join-Path $pastaBanco '03-permissoes.sql'), [Text.Encoding]::UTF8)
    Executar-ScriptSql $conexao ($sql.Replace('{{POOL}}', $NomePool))
    Escrever-Ok "IIS APPPOOL\$NomePool -> db_datareader + db_datawriter"

    $nivel = Executar-Escalar $conexao "SELECT compatibility_level FROM sys.databases WHERE name = N'FinancasLocal'"
    Escrever-Ok "nivel de compatibilidade do banco: $nivel"
} finally {
    $conexao.Dispose()
}

Write-Host ''
Write-Host 'Banco pronto. Proximo passo:' -ForegroundColor Green
$proximo = "$PSScriptRoot\3-publicar.ps1"
if ($InstanciaSql -ne '.\FINANCAS') { $proximo += ' -InstanciaSql "' + $InstanciaSql + '"' }
Write-Host "   $proximo"
