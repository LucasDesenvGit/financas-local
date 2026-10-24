using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using Microsoft.Data.Sqlite;

namespace Financas.Api.Data;

/// <summary>
/// Cria e atualiza o banco ao iniciar a API (ADR 0003). Usa os mesmos scripts idempotentes de <c>banco/</c> e as
/// mesmas colunas novas das linhas <c>adicionar_coluna_se_ausente</c> do <c>scripts/geral/2-criar-banco.sh</c>, embutidos no
/// build: a lista de colunas continua num lugar só. Banco novo recebe também os dados iniciais. Antes de mexer num
/// banco que já existe, se os scripts mudaram desde a última vez, faz uma cópia em <c>dados/backups</c>.
/// Assim a instalação pelo .msi (#119) não precisa de sqlite3 nem de bash; a instalação por scripts continua igual
/// (rodar os scripts antes não atrapalha: tudo aqui é idempotente).
/// </summary>
public static partial class EsquemaBanco
{
    /// <summary>Cópias "antes da atualização" guardadas; as mais antigas são apagadas.</summary>
    private const int BackupsGuardados = 5;
    private const string PrefixoBackup = "FinancasLocal-antes-atualizacao-";

    /// <summary>A mesma linha que o <c>instalar-windows.ps1</c> lê (três argumentos entre aspas duplas).</summary>
    [GeneratedRegex(@"^\s*adicionar_coluna_se_ausente\s+""([A-Za-z0-9_]+)""\s+""([A-Za-z0-9_]+)""\s+""([^""]+)""",
        RegexOptions.Multiline)]
    private static partial Regex LinhaColuna();

    public static void Aplicar(string caminhoBanco, ILogger logger)
    {
        var estrutura = Ler("01-estrutura.sql");
        var colunas = LinhaColuna().Matches(Ler("2-criar-banco.sh"))
            .Select(m => (Tabela: m.Groups[1].Value, Coluna: m.Groups[2].Value, Definicao: m.Groups[3].Value))
            .ToList();
        if (colunas.Count == 0) throw new InvalidOperationException("Nenhuma coluna nova encontrada no 2-criar-banco.sh embutido.");
        var assinatura = Assinatura(estrutura, colunas);

        var caminho = Path.GetFullPath(caminhoBanco);
        Directory.CreateDirectory(Path.GetDirectoryName(caminho)!);
        var novo = !File.Exists(caminho) || new FileInfo(caminho).Length == 0;

        using var conexao = new SqliteConnection(new SqliteConnectionStringBuilder
        {
            DataSource = caminho, Pooling = false, DefaultTimeout = 30
        }.ToString());
        conexao.Open();
        PassarParaWal(conexao, logger);

        if (!novo)
        {
            // Mesmos scripts da última vez: nada a fazer (o comum, a cada reinício do serviço).
            if (Convert.ToInt32(Escalar(conexao, "PRAGMA user_version;")) == assinatura) return;
            Backup(conexao, caminho, logger);
        }

        using (var transacao = conexao.BeginTransaction())
        {
            Executar(conexao, transacao, estrutura);
            foreach (var (tabela, coluna, definicao) in colunas)
            {
                if (ColunaExiste(conexao, transacao, tabela, coluna)) continue;
                Executar(conexao, transacao, $"ALTER TABLE {tabela} ADD COLUMN {coluna} {definicao};");
                logger.LogInformation("Banco: coluna {Coluna} adicionada em {Tabela}.", coluna, tabela);
            }
            // Dados iniciais só no banco novo: rodar de novo recriaria categorias que a pessoa apagou.
            if (novo) Executar(conexao, transacao, Ler("02-dados-iniciais.sql"));
            transacao.Commit();
        }
        Executar(conexao, null, $"PRAGMA user_version = {assinatura};");
        logger.LogInformation(novo ? "Banco criado em {Caminho}." : "Estrutura do banco conferida em {Caminho}.", caminho);
    }

    /// <summary>
    /// #231 (ADR 0049): o .NET e o Go usam o mesmo arquivo. Em WAL, quem lê não espera quem grava, e a espera de banco
    /// ocupado fica só entre dois que gravam (o Go espera até 30 s, o Microsoft.Data.Sqlite também). O modo fica
    /// gravado no arquivo; rodar de novo não muda nada. Só dá para trocar sem outra conexão aberta: se não deu
    /// (ex.: o DB Browser aberto), avisa e segue no modo de antes, como até a 2.37.
    /// </summary>
    private static void PassarParaWal(SqliteConnection conexao, ILogger logger)
    {
        string? modo;
        try
        {
            modo = Convert.ToString(Escalar(conexao, "PRAGMA journal_mode = WAL;"));
        }
        catch (SqliteException ex)
        {
            modo = ex.Message;
        }
        if (!string.Equals(modo, "wal", StringComparison.OrdinalIgnoreCase))
            logger.LogWarning("Banco: não deu para passar para o modo WAL ({Modo}); feche outros programas com o banco aberto "
                + "e reinicie o serviço.", modo);
    }

    /// <summary>
    /// Número que muda quando a estrutura ou a lista de colunas muda (guardado no <c>PRAGMA user_version</c>, que o
    /// projeto não usa para mais nada). Positivo de 31 bits: o PRAGMA guarda um inteiro de 32 bits com sinal.
    /// </summary>
    private static int Assinatura(string estrutura, IEnumerable<(string Tabela, string Coluna, string Definicao)> colunas)
    {
        var texto = estrutura.Replace("\r\n", "\n") + "\n" + string.Join("\n", colunas.Select(c => $"{c.Tabela}|{c.Coluna}|{c.Definicao}"));
        var hash = SHA256.HashData(Encoding.UTF8.GetBytes(texto));
        var valor = BitConverter.ToInt32(hash, 0) & int.MaxValue;
        return valor == 0 ? 1 : valor;
    }

    /// <summary>
    /// Cópia consistente (VACUUM INTO) em <c>dados/backups</c>, guardando as 5 mais recentes.
    /// <para>#366: o nome vai até o milissegundo. Só até o segundo, duas atualizações no mesmo segundo (o serviço
    /// reiniciando logo depois de uma atualização que falhou) davam o mesmo nome, e o VACUUM INTO recusa um destino que
    /// já existe. A ordem é byte a byte (<see cref="StringComparer.Ordinal"/>): o "." vem antes do "_", e o nome antigo
    /// de um segundo (<c>_HHmmss.db</c>) fica antes dos novos do mesmo segundo. A ordem da cultura (a padrão do
    /// <c>OrderBy</c>) põe o "_" antes do "." e apagaria o mais novo.</para>
    /// <para>#365: uma cópia antiga que não dá para apagar (outro dono, só leitura) fica para a próxima, com um aviso.
    /// Sem permissão, o <see cref="FileInfo.Delete"/> lança <see cref="UnauthorizedAccessException"/>, que não é
    /// <see cref="IOException"/>, e a API não subia.</para>
    /// </summary>
    private static void Backup(SqliteConnection conexao, string caminho, ILogger logger)
    {
        var pastaSql = Path.GetDirectoryName(caminho)!;
        var pastaDados = string.Equals(Path.GetFileName(pastaSql), "sql", StringComparison.OrdinalIgnoreCase)
            ? Path.GetDirectoryName(pastaSql)! : pastaSql;
        var pastaBackups = Path.Combine(pastaDados, "backups");
        Directory.CreateDirectory(pastaBackups);
        var destino = Path.Combine(pastaBackups, $"{PrefixoBackup}{DateTime.Now:yyyyMMdd_HHmmss_fff}.db");
        using (var comando = conexao.CreateCommand())
        {
            comando.CommandText = "VACUUM INTO $destino;";
            comando.Parameters.AddWithValue("$destino", destino);
            comando.ExecuteNonQuery();
        }
        logger.LogInformation("Banco: cópia antes da atualização em {Destino}.", destino);

        foreach (var antigo in new DirectoryInfo(pastaBackups).GetFiles(PrefixoBackup + "*.db")
                     .OrderByDescending(f => f.Name, StringComparer.Ordinal).Skip(BackupsGuardados))
        {
            try { antigo.Delete(); }
            catch (Exception ex)
            {
                logger.LogWarning(ex, "Banco: não deu para apagar a cópia antiga {Arquivo}; fica para a próxima.", antigo.Name);
            }
        }
    }

    private static bool ColunaExiste(SqliteConnection conexao, SqliteTransaction transacao, string tabela, string coluna)
    {
        using var comando = conexao.CreateCommand();
        comando.Transaction = transacao;
        comando.CommandText = "SELECT COUNT(*) FROM pragma_table_info($tabela) WHERE name = $coluna;";
        comando.Parameters.AddWithValue("$tabela", tabela);
        comando.Parameters.AddWithValue("$coluna", coluna);
        return Convert.ToInt32(comando.ExecuteScalar()) > 0;
    }

    private static void Executar(SqliteConnection conexao, SqliteTransaction? transacao, string sql)
    {
        using var comando = conexao.CreateCommand();
        comando.Transaction = transacao;
        comando.CommandText = sql;
        comando.ExecuteNonQuery();
    }

    private static object? Escalar(SqliteConnection conexao, string sql)
    {
        using var comando = conexao.CreateCommand();
        comando.CommandText = sql;
        return comando.ExecuteScalar();
    }

    private static string Ler(string nome)
    {
        using var fluxo = Assembly.GetExecutingAssembly().GetManifestResourceStream("banco." + nome)
            ?? throw new InvalidOperationException($"Script {nome} não foi embutido no build (Financas.Api.csproj).");
        using var leitor = new StreamReader(fluxo, Encoding.UTF8);
        return leitor.ReadToEnd();
    }
}
