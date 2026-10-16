using System.Data;
using System.Reflection;
using Financas.Api.Data;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Investimentos;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata;

namespace Financas.MigrarSqlite;

/// <summary>
/// Ferramenta pontual de migração: copia os dados de uma base SQL Server (o sistema antigo) para o
/// arquivo .db SQLite usado pela versão atual do FinancasLocal. Reaproveita o FinancasDbContext e as
/// classes de Entidades/ do Financas.Api, então não duplica o modelo nem precisa ser atualizada
/// manualmente quando o esquema evoluir.
///
/// Uso:
///   dotnet run --project src/backend/Financas.MigrarSqlite -- "<connection string SQL Server origem>" "<caminho do arquivo .db destino>"
///
/// É seguro rodar mesmo sem uma base SQL Server disponível: nesse caso, a ferramenta apenas avisa e
/// termina com sucesso (não é um erro, é o caso esperado enquanto não existir uma base antiga à mão).
/// </summary>
public static class Program
{
    private static readonly MethodInfo MetodoCopiarTabela =
        typeof(Program).GetMethod(nameof(CopiarTabelaAsync), BindingFlags.NonPublic | BindingFlags.Static)!;

    private static readonly MethodInfo MetodoContar =
        typeof(Program).GetMethod(nameof(ContarAsync), BindingFlags.NonPublic | BindingFlags.Static)!;

    // Mesma convenção terse de scripts/comum.sh (etapa/ok/aviso/erro_sai), só que sem cores: aqui é
    // Console.WriteLine simples, já que a saída deste utilitário costuma ir para um terminal qualquer
    // (inclusive no Windows) ou ser redirecionada para log.
    private static void Etapa(string texto)
    {
        Console.WriteLine();
        Console.WriteLine($"==> {texto}");
    }

    private static void Ok(string texto) => Console.WriteLine($"    OK  {texto}");
    private static void Aviso(string texto) => Console.WriteLine($"    !!  {texto}");
    private static void ErroLinha(string texto) => Console.Error.WriteLine($"    ERRO  {texto}");

    public static async Task<int> Main(string[] args)
    {
        if (args.Length < 2)
        {
            Console.WriteLine("Uso: dotnet run --project src/backend/Financas.MigrarSqlite -- \"<connection string SQL Server origem>\" \"<caminho do arquivo .db destino>\"");
            return 1;
        }

        var origemCs = args[0];
        var destinoCaminho = args[1];

        Etapa("Verificando base de origem (SQL Server)");

        var opcoesOrigem = new DbContextOptionsBuilder<FinancasDbContext>()
            .UseSqlServer(origemCs)
            .Options;

        await using var origem = new FinancasDbContext(opcoesOrigem);

        bool conectou;
        try
        {
            // CanConnectAsync() respeita o Connection Timeout da connection string: se o servidor não
            // existir, não estiver no ar, ou a autenticação/o banco falhar, cai aqui sem travar.
            conectou = await origem.Database.CanConnectAsync();
        }
        catch
        {
            // Captura ampla de propósito: qualquer falha ao tentar falar com o SQL Server (servidor
            // inalcançável, banco inexistente, timeout, erro de autenticação, etc.) significa a mesma
            // coisa para este fluxo: não há nada para migrar agora.
            conectou = false;
        }

        if (!conectou)
        {
            Console.WriteLine($"Nenhuma base SQL Server encontrada em '{origemCs}' — nada a migrar.");
            return 0;
        }

        Ok("base de origem acessível");

        if (!File.Exists(destinoCaminho))
        {
            ErroLinha($"O arquivo de destino '{destinoCaminho}' não existe.");
            ErroLinha("Crie antes um banco vazio só com a estrutura: sqlite3 \"<destino>\" < banco/01-estrutura.sql");
            ErroLinha("(não use o 2-criar-banco.sh: os dados iniciais dele conflitam com os dados migrados).");
            return 1;
        }

        var opcoesDestino = new DbContextOptionsBuilder<FinancasDbContext>()
            .UseSqlite($"Data Source={destinoCaminho}")
            .Options;

        await using var destino = new FinancasDbContext(opcoesDestino);

        Etapa("Verificando estrutura do destino (SQLite)");
        var tabelasEsperadas = origem.Model.GetEntityTypes()
            .Select(e => e.GetTableName())
            .Where(nome => nome is not null)
            .Cast<string>()
            .Distinct()
            .ToList();
        var tabelasExistentes = await ObterTabelasSqliteAsync(destino);
        var tabelasFaltando = tabelasEsperadas.Where(t => !tabelasExistentes.Contains(t)).ToList();
        if (tabelasFaltando.Count > 0)
        {
            ErroLinha("O arquivo de destino existe, mas não tem a estrutura esperada.");
            ErroLinha("Tabelas faltando: " + string.Join(", ", tabelasFaltando));
            ErroLinha("Esta ferramenta não cria esquema: crie o destino com banco/01-estrutura.sql e tente de novo.");
            return 1;
        }
        Ok("estrutura compatível");

        // Os Ids da origem são copiados como estão: qualquer linha já presente no destino (ex.: as
        // categorias e configurações padrão do banco/02-dados-iniciais.sql) colidiria com eles.
        var tabelasComDados = new List<string>();
        foreach (var tabela in tabelasEsperadas)
        {
            if (await TabelaTemLinhasAsync(destino, tabela)) tabelasComDados.Add(tabela);
        }
        if (tabelasComDados.Count > 0)
        {
            ErroLinha("O arquivo de destino já tem dados em: " + string.Join(", ", tabelasComDados));
            ErroLinha("A migração precisa de um banco vazio, criado só com banco/01-estrutura.sql (sem o 02-dados-iniciais.sql).");
            return 1;
        }
        Ok("destino vazio");

        Etapa("Ordenando tabelas por dependência de chave estrangeira");
        var ordem = OrdenarPorDependencia(origem.Model);
        Console.WriteLine("    " + string.Join(" -> ", ordem.Select(t => t.GetTableName())));

        Etapa("Copiando dados para o destino (em uma única transação)");
        await using var tx = await destino.Database.BeginTransactionAsync();
        try
        {
            foreach (var entityType in ordem)
            {
                await CopiarEntidadeAsync(origem, destino, entityType);
            }

            await LimparSegredosAsync(destino);

            Etapa("Verificando integridade referencial (PRAGMA foreign_key_check)");
            var violacoes = await ObterViolacoesForeignKeyAsync(destino);
            if (violacoes.Count > 0)
            {
                ErroLinha("Violações de integridade referencial encontradas (a migração será desfeita):");
                foreach (var v in violacoes) Console.Error.WriteLine("        " + v);
                await tx.RollbackAsync();
                return 1;
            }
            Ok("nenhuma violação de chave estrangeira");

            await tx.CommitAsync();
        }
        catch (Exception ex)
        {
            await tx.RollbackAsync();
            ErroLinha($"Falha ao migrar, nada foi gravado: {ex.Message}");
            return 1;
        }

        Etapa("Comparando contagem de linhas (origem x destino)");
        var divergiu = false;
        foreach (var entityType in ordem)
        {
            if (!await ImprimirContagemAsync(origem, destino, entityType)) divergiu = true;
        }

        Console.WriteLine();
        Aviso("Lembrete: o Client Secret da Pluggy e o token da brapi foram apagados no banco de destino.");
        Console.WriteLine("        Eles foram criptografados com a chave de Data Protection da máquina/instalação de origem");
        Console.WriteLine("        (protegida por DPAPI) e não podem ser decifrados em outro lugar. Informe-os de novo em");
        Console.WriteLine("        Configurações → Open Finance / Investimentos depois de abrir o novo banco.");

        return divergiu ? 1 : 0;
    }

    /// <summary>
    /// Ordena os tipos de entidade por dependência de chave estrangeira (Kahn's algorithm): um tipo só
    /// aparece depois de todos os tipos para os quais ele tem uma FK (precisam existir primeiro para a
    /// linha ser inserida sem violar a restrição). FKs auto-referenciais (ex.: Categoria.CategoriaPaiId
    /// apontando para a própria Categoria) são ignoradas aqui de propósito: elas não definem uma ordem
    /// entre *tabelas* diferentes, e são tratadas à parte em CopiarTabelaAsync (inserção em duas
    /// passagens) para qualquer entidade que tenha uma FK cujo tipo principal seja ela mesma.
    /// </summary>
    private static List<IEntityType> OrdenarPorDependencia(IModel model)
    {
        var tipos = model.GetEntityTypes().ToList();
        var grauEntrada = tipos.ToDictionary(t => t, _ => 0);
        var dependentes = tipos.ToDictionary(t => t, _ => new List<IEntityType>());

        foreach (var tipo in tipos)
        {
            foreach (var fk in tipo.GetForeignKeys())
            {
                if (fk.PrincipalEntityType == fk.DeclaringEntityType) continue; // auto-referencial: tratado à parte

                dependentes[fk.PrincipalEntityType].Add(tipo);
                grauEntrada[tipo]++;
            }
        }

        // Ordem alfabética pelo nome da tabela só para tornar a saída determinística entre execuções;
        // não tem efeito na correção do algoritmo.
        var fila = new Queue<IEntityType>(tipos.Where(t => grauEntrada[t] == 0).OrderBy(t => t.GetTableName()));
        var resultado = new List<IEntityType>();

        while (fila.Count > 0)
        {
            var atual = fila.Dequeue();
            resultado.Add(atual);
            foreach (var dependente in dependentes[atual].OrderBy(t => t.GetTableName()))
            {
                if (--grauEntrada[dependente] == 0) fila.Enqueue(dependente);
            }
        }

        if (resultado.Count != tipos.Count)
        {
            var faltando = tipos.Except(resultado).Select(t => t.GetTableName());
            throw new InvalidOperationException(
                "Ciclo de dependência entre tabelas diferentes (não auto-referencial) detectado envolvendo: " +
                string.Join(", ", faltando));
        }

        return resultado;
    }

    private static async Task CopiarEntidadeAsync(FinancasDbContext origem, FinancasDbContext destino, IEntityType entityType)
    {
        var metodo = MetodoCopiarTabela.MakeGenericMethod(entityType.ClrType);
        await (Task)metodo.Invoke(null, [origem, destino])!;
    }

    /// <summary>
    /// Copia uma tabela inteira de origem para destino, lendo tudo em memória de uma vez
    /// (AsNoTracking().ToListAsync()) — aceitável aqui porque esta é uma base pessoal, não big data.
    ///
    /// Preservação das chaves primárias explícitas: as linhas vêm de origem já com o Id real (não
    /// default/0), e são adicionadas em destino com Set&lt;T&gt;().AddRange(...) sem Attach prévio. Isso
    /// basta: o EF Core decide se gera um novo valor para uma propriedade de chave comparando o valor
    /// atual dela com o valor-sentinela do tipo (0 para int) no momento em que a entidade entra no
    /// estado Added — não importa se ela chegou lá via Add() direto ou via Attach()+Modified. Como os
    /// Ids aqui já são não-default, o EF marca a propriedade como "já definida" (IsTemporary = false) e
    /// inclui o valor explícito no INSERT; o SQLite aceita valores explícitos em uma coluna
    /// INTEGER PRIMARY KEY normalmente (diferente do SQL Server, que exigiria IDENTITY_INSERT ON). Esse
    /// comportamento foi confirmado empiricamente contra o provider Sqlite do EF Core 10 antes de
    /// escrever este código; não é necessário nenhum truque extra (como marcar a propriedade como não
    /// gerada pelo store por chamada) para preservar os Ids originais.
    /// </summary>
    private static async Task CopiarTabelaAsync<TEntity>(FinancasDbContext origem, FinancasDbContext destino)
        where TEntity : class
    {
        var entityType = origem.Model.FindEntityType(typeof(TEntity))!;
        var nomeTabela = entityType.GetTableName();

        var linhas = await origem.Set<TEntity>().AsNoTracking().ToListAsync();
        if (linhas.Count == 0)
        {
            Ok($"{nomeTabela}: 0 linhas (nada a copiar)");
            return;
        }

        // FK auto-referencial: uma FK cujo tipo principal é o próprio tipo declarante (ex.:
        // Categoria.CategoriaPaiId -> Categoria.Id). Detectada genericamente por metadado, não pelo
        // nome literal "Categoria", para continuar funcionando se outra entidade ganhar uma FK assim.
        var fkAutoReferencial = entityType.GetForeignKeys()
            .FirstOrDefault(fk => fk.PrincipalEntityType == fk.DeclaringEntityType);

        if (fkAutoReferencial is null)
        {
            destino.Set<TEntity>().AddRange(linhas);
            await destino.SaveChangesAsync();
            Ok($"{nomeTabela}: {linhas.Count} linhas copiadas");
            return;
        }

        if (fkAutoReferencial.Properties.Count != 1)
        {
            throw new InvalidOperationException(
                $"FK auto-referencial composta (mais de uma coluna) em {nomeTabela} não é suportada por esta ferramenta.");
        }

        var propriedadeFk = fkAutoReferencial.Properties[0];
        var propInfo = propriedadeFk.PropertyInfo
            ?? throw new InvalidOperationException($"Não foi possível obter o PropertyInfo da FK auto-referencial em {nomeTabela}.");

        if (Nullable.GetUnderlyingType(propInfo.PropertyType) is null && propInfo.PropertyType.IsValueType)
        {
            throw new InvalidOperationException(
                $"FK auto-referencial não anulável em {nomeTabela} ({propInfo.Name}) não é suportada: " +
                "a inserção em duas passagens depende de poder zerá-la temporariamente.");
        }

        // 1ª passagem: insere todas as linhas com a FK auto-referencial temporariamente nula, para não
        // depender da ordem entre pai/filho dentro da própria tabela.
        var valoresOriginais = new List<(TEntity Entidade, object? Valor)>(linhas.Count);
        foreach (var linha in linhas)
        {
            valoresOriginais.Add((linha, propInfo.GetValue(linha)));
            propInfo.SetValue(linha, null);
        }

        destino.Set<TEntity>().AddRange(linhas);
        await destino.SaveChangesAsync();

        // 2ª passagem: agora que todas as linhas já existem no destino, restaura o valor real da FK
        // auto-referencial. As entidades continuam rastreadas pelo destino desde o AddRange acima, então
        // a mudança de propriedade via reflexão é detectada normalmente no próximo SaveChanges.
        var comValorOriginal = 0;
        foreach (var (linha, valor) in valoresOriginais)
        {
            if (valor is null) continue;
            propInfo.SetValue(linha, valor);
            comValorOriginal++;
        }

        if (comValorOriginal > 0) await destino.SaveChangesAsync();

        Ok($"{nomeTabela}: {linhas.Count} linhas copiadas ({comValorOriginal} com FK auto-referencial restaurada)");
    }

    /// <summary>
    /// Limpa, no destino, os segredos protegidos por ASP.NET Data Protection (DPAPI da máquina de
    /// origem): o Client Secret da Pluggy e o token da brapi. Foram copiados como texto cifrado pela
    /// cópia genérica de Configuracao acima, mas são indecifráveis em outra máquina/instalação — o app
    /// já tem esse fluxo de recuperação documentado (usuário reinformando o Client Secret em
    /// Configurações quando a chave DPAPI muda, ver README.md "Problemas comuns").
    /// </summary>
    private static async Task LimparSegredosAsync(FinancasDbContext destino)
    {
        string[] chavesProtegidas = [ConfiguracaoService.PluggyClientSecret, CotacaoService.ChaveTokenBrapi];

        var linhas = await destino.Configuracoes
            .Where(c => chavesProtegidas.Contains(c.Chave))
            .ToListAsync();

        if (linhas.Count == 0) return;

        foreach (var linha in linhas)
        {
            linha.Valor = null;
            linha.Protegido = false;
        }

        await destino.SaveChangesAsync();
        Ok($"{linhas.Count} segredo(s) protegido(s) por DPAPI zerado(s) em Configuracao ({string.Join(", ", chavesProtegidas)})");
    }

    private static async Task<bool> ImprimirContagemAsync(FinancasDbContext origem, FinancasDbContext destino, IEntityType entityType)
    {
        var metodo = MetodoContar.MakeGenericMethod(entityType.ClrType);
        var tarefa = (Task<(int Origem, int Destino)>)metodo.Invoke(null, [origem, destino])!;
        var (qtdOrigem, qtdDestino) = await tarefa;
        var bateu = qtdOrigem == qtdDestino;
        var mensagem = $"{entityType.GetTableName()}: origem={qtdOrigem} destino={qtdDestino}";
        if (bateu) Ok(mensagem); else Aviso(mensagem);
        return bateu;
    }

    private static async Task<(int Origem, int Destino)> ContarAsync<TEntity>(FinancasDbContext origem, FinancasDbContext destino)
        where TEntity : class
    {
        var qtdOrigem = await origem.Set<TEntity>().AsNoTracking().CountAsync();
        var qtdDestino = await destino.Set<TEntity>().AsNoTracking().CountAsync();
        return (qtdOrigem, qtdDestino);
    }

    private static async Task<HashSet<string>> ObterTabelasSqliteAsync(FinancasDbContext destino)
    {
        var tabelas = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        var conexao = destino.Database.GetDbConnection();
        var precisaAbrir = conexao.State != ConnectionState.Open;
        if (precisaAbrir) await conexao.OpenAsync();
        try
        {
            await using var cmd = conexao.CreateCommand();
            cmd.CommandText = "SELECT name FROM sqlite_master WHERE type = 'table';";
            await using var reader = await cmd.ExecuteReaderAsync();
            while (await reader.ReadAsync()) tabelas.Add(reader.GetString(0));
        }
        finally
        {
            if (precisaAbrir) await conexao.CloseAsync();
        }

        return tabelas;
    }

    private static async Task<bool> TabelaTemLinhasAsync(FinancasDbContext destino, string tabela)
    {
        var conexao = destino.Database.GetDbConnection();
        var precisaAbrir = conexao.State != ConnectionState.Open;
        if (precisaAbrir) await conexao.OpenAsync();
        try
        {
            await using var cmd = conexao.CreateCommand();
            // Nome vem do modelo do EF (não de entrada do usuário); aspas duplas são o escape de identificador do SQLite.
            cmd.CommandText = $"SELECT EXISTS (SELECT 1 FROM \"{tabela.Replace("\"", "\"\"")}\");";
            return Convert.ToInt64(await cmd.ExecuteScalarAsync()) == 1;
        }
        finally
        {
            if (precisaAbrir) await conexao.CloseAsync();
        }
    }

    private static async Task<List<string>> ObterViolacoesForeignKeyAsync(FinancasDbContext destino)
    {
        var violacoes = new List<string>();
        var conexao = destino.Database.GetDbConnection();
        var precisaAbrir = conexao.State != ConnectionState.Open;
        if (precisaAbrir) await conexao.OpenAsync();
        try
        {
            await using var cmd = conexao.CreateCommand();
            cmd.CommandText = "PRAGMA foreign_key_check;";
            await using var reader = await cmd.ExecuteReaderAsync();
            // Colunas do PRAGMA foreign_key_check: table, rowid, parent, fkid.
            while (await reader.ReadAsync())
            {
                var tabela = reader.GetString(0);
                var rowid = reader.IsDBNull(1) ? "?" : reader.GetValue(1).ToString();
                var tabelaPai = reader.GetString(2);
                violacoes.Add($"{tabela} (rowid={rowid}) referencia {tabelaPai} de forma inválida");
            }
        }
        finally
        {
            if (precisaAbrir) await conexao.CloseAsync();
        }

        return violacoes;
    }
}
