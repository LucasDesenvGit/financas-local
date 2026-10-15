/* =====================================================================
   FinancasLocal - Permissões de acesso da API
   A API roda no IIS com a identidade do pool de aplicativos
   (IIS APPPOOL\{{POOL}}). Ela recebe somente leitura/escrita de dados
   (sem permissão para alterar estrutura, criar logins ou fazer backup).
   ===================================================================== */
USE [master];
GO
IF NOT EXISTS (SELECT 1 FROM sys.server_principals WHERE name = N'IIS APPPOOL\{{POOL}}')
    CREATE LOGIN [IIS APPPOOL\{{POOL}}] FROM WINDOWS WITH DEFAULT_DATABASE = [FinancasLocal];
GO

USE [FinancasLocal];
GO
IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = N'FinancasApi')
    CREATE USER [FinancasApi] FOR LOGIN [IIS APPPOOL\{{POOL}}];
GO
ALTER ROLE db_datareader ADD MEMBER [FinancasApi];
GO
ALTER ROLE db_datawriter ADD MEMBER [FinancasApi];
GO
/* A auditoria de acesso é apenas inserida/consultada pela API, nunca alterada ou apagada */
DENY UPDATE, DELETE ON dbo.AuditoriaAcesso TO [FinancasApi];
GO
