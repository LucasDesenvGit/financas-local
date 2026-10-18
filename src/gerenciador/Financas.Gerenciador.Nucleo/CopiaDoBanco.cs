using Microsoft.Data.Sqlite;

namespace Financas.Gerenciador;

/// <summary>"Salvar uma cópia do banco": cópia consistente (VACUUM INTO) com o site no ar.</summary>
public static class CopiaDoBanco
{
    /// <summary>Lança <see cref="FileNotFoundException"/>, <see cref="SqliteException"/> ou erro de arquivo.</summary>
    public static void Salvar(string banco, string destino)
    {
        if (!File.Exists(banco)) throw new FileNotFoundException("O banco não foi encontrado.", banco);
        if (File.Exists(destino)) File.Delete(destino);
        // A cópia tem todos os dados financeiros: no Linux, nasce legível só pelo dono (como a pasta dados/, chmod 700).
        // O VACUUM INTO aceita um arquivo vazio como destino e mantém a permissão dele.
        if (!OperatingSystem.IsWindows())
            new FileStream(destino, new FileStreamOptions
            {
                Mode = FileMode.CreateNew, Access = FileAccess.Write,
                UnixCreateMode = UnixFileMode.UserRead | UnixFileMode.UserWrite
            }).Dispose();
        using var conexao = new SqliteConnection(new SqliteConnectionStringBuilder
        {
            DataSource = banco, Pooling = false, DefaultTimeout = 30
        }.ToString());
        conexao.Open();
        using var comando = conexao.CreateCommand();
        comando.CommandText = "VACUUM INTO $destino;";
        comando.Parameters.AddWithValue("$destino", destino);
        comando.ExecuteNonQuery();
    }
}
