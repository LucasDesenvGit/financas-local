using System.Text;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Sugere categorias para transações a partir de:
/// 1) histórico: como você categorizou transações com descrição parecida (o que você escolheu à mão pesa mais);
/// 2) palavras-chave conhecidas (UBER → Transporte, IFOOD → Alimentação...).
/// As transações são agrupadas pela descrição "limpa" (sem números, datas e prefixos como PIX ENVIADO / COMPRA CARTAO).
/// </summary>
public class SugestaoCategoriaService
{
    private const int DiasHistorico = 730;
    private const int MaximoGrupos = 300;

    /// <summary>Palavras que não identificam o estabelecimento.</summary>
    private static readonly HashSet<string> Ruido = new(StringComparer.Ordinal)
    {
        "PIX", "ENVIADO", "ENVIADA", "RECEBIDO", "RECEBIDA", "TRANSF", "TRANSFERENCIA", "COMPRA", "COMPRAS", "CARTAO",
        "CRED", "CREDITO", "DEB", "DEBITO", "PAG", "PAGTO", "PAGAMENTO", "PGTO", "TED", "DOC", "BOLETO", "PARC",
        "PARCELA", "PARCELADO", "NACIONAL", "INTERNACIONAL", "ELO", "VISA", "MASTERCARD", "MASTER", "LTDA", "EIRELI",
        "COM", "BRA", "BRASIL", "WWW", "HTTPS", "HTTP", "APP", "DES", "REM", "SAQUE", "QRCODE", "QR", "CODE", "AUT",
        "PARA", "DAS", "DOS", "COMERCIO", "SERVICOS", "SERV", "EMPRESA", "EPP", "ONLINE", "INTERNET", "AGENDADO",
        "AUTOMATICO", "CONTA", "VALOR", "REF", "NO", "NA", "EM", "DE", "DA", "DO", "E", "O", "A",
        "SAO", "PAULO", "SP", "RJ", "MG", "PR", "SC", "RS", "BR"
    };

    /// <summary>
    /// #254: palavras que aparecem em descrições de todo tipo. Servem para agrupar, mas sozinhas viram regra "contém" que
    /// pega o que não devia (ex.: "BANCO" levava Pix, tarifas e transferências para "Pagamento de fatura").
    /// </summary>
    private static readonly HashSet<string> GenericasParaRegra = new(StringComparer.Ordinal)
    {
        "BANCO", "BCO", "BANK", "FATURA", "PAGAMENTOS", "RECEBIMENTO", "DEPOSITO", "APLICACAO", "RESGATE", "SALDO",
        "LOJA", "LOJAS", "MENSALIDADE", "SERVICO", "CENTRO", "GRUPO", "INSTITUICAO", "PAGADORA", "CLIENTE", "TITULAR",
        "MESMA", "TITULARIDADE", "OUTRA", "OUTROS", "DIVERSOS", "ESTORNO", "TAXA"
    };

    /// <summary>Uma regra sugerida que pegaria pelo menos isto de transações de outras categorias ganha um aviso.</summary>
    private const int ConflitosParaAviso = 3;
    private const double FracaoConflitosParaAviso = 0.2;

    /// <summary>Palavra-chave (sem acento, maiúscula) → nome da categoria padrão.</summary>
    private static readonly (string Palavra, string Categoria)[] PalavrasChave =
    {
        ("SUPERMERCADO", "Mercado"), ("MERCADO", "Mercado"), ("ATACADAO", "Mercado"), ("ASSAI", "Mercado"),
        ("CARREFOUR", "Mercado"), ("PAO DE ACUCAR", "Mercado"), ("HORTIFRUTI", "Mercado"), ("SAMS CLUB", "Mercado"),
        ("ATACADISTA", "Mercado"), ("SACOLAO", "Mercado"), ("EMPORIO", "Mercado"),
        ("IFOOD", "Alimentação"), ("IFD", "Alimentação"), ("RAPPI", "Alimentação"), ("RESTAURANTE", "Alimentação"),
        ("LANCHONETE", "Alimentação"), ("PADARIA", "Alimentação"), ("PANIFICADORA", "Alimentação"),
        ("BURGER", "Alimentação"), ("MCDONALDS", "Alimentação"), ("PIZZA", "Alimentação"), ("SUSHI", "Alimentação"),
        ("CAFETERIA", "Alimentação"), ("STARBUCKS", "Alimentação"), ("OUTBACK", "Alimentação"), ("BAR ", "Alimentação"),
        ("UBER", "Transporte"), ("99APP", "Transporte"), ("99 TAXI", "Transporte"), ("99POP", "Transporte"),
        ("POSTO", "Transporte"), ("SHELL", "Transporte"), ("IPIRANGA", "Transporte"), ("PETROBRAS", "Transporte"),
        ("COMBUSTIVEL", "Transporte"), ("ESTACIONAMENTO", "Transporte"), ("ESTAPAR", "Transporte"),
        ("SEM PARAR", "Transporte"), ("CONECTCAR", "Transporte"), ("VELOE", "Transporte"), ("PEDAGIO", "Transporte"),
        ("METRO", "Transporte"), ("BILHETE UNICO", "Transporte"), ("AUTO PECAS", "Transporte"),
        ("FARMACIA", "Saúde"), ("DROGASIL", "Saúde"), ("DROGA RAIA", "Saúde"), ("RAIA", "Saúde"),
        ("PAGUE MENOS", "Saúde"), ("DROGARIA", "Saúde"), ("HOSPITAL", "Saúde"), ("CLINICA", "Saúde"),
        ("LABORATORIO", "Saúde"), ("UNIMED", "Saúde"), ("ODONTO", "Saúde"), ("DENTISTA", "Saúde"),
        ("NETFLIX", "Serviços e assinaturas"), ("SPOTIFY", "Serviços e assinaturas"),
        ("AMAZON PRIME", "Serviços e assinaturas"), ("PRIME VIDEO", "Serviços e assinaturas"),
        ("DISNEY", "Serviços e assinaturas"), ("HBO", "Serviços e assinaturas"), ("MAX.COM", "Serviços e assinaturas"),
        ("YOUTUBE", "Serviços e assinaturas"), ("APPLE.COM", "Serviços e assinaturas"),
        ("GOOGLE", "Serviços e assinaturas"), ("MICROSOFT", "Serviços e assinaturas"),
        ("CLARO", "Serviços e assinaturas"), ("VIVO", "Serviços e assinaturas"), ("TIM ", "Serviços e assinaturas"),
        ("OPENAI", "Serviços e assinaturas"), ("CHATGPT", "Serviços e assinaturas"), ("CLAUDE", "Serviços e assinaturas"),
        ("ANTHROPIC", "Serviços e assinaturas"), ("GLOBOPLAY", "Serviços e assinaturas"),
        ("ALUGUEL", "Moradia"), ("CONDOMINIO", "Moradia"), ("ENEL", "Moradia"), ("SABESP", "Moradia"),
        ("CEMIG", "Moradia"), ("COPEL", "Moradia"), ("LIGHT", "Moradia"), ("COMGAS", "Moradia"),
        ("ENERGIA", "Moradia"), ("SANEAMENTO", "Moradia"), ("CPFL", "Moradia"), ("NEOENERGIA", "Moradia"),
        ("AMAZON", "Compras"), ("MERCADOLIVRE", "Compras"), ("MERCADO LIVRE", "Compras"), ("MAGALU", "Compras"),
        ("MAGAZINE LUIZA", "Compras"), ("SHOPEE", "Compras"), ("ALIEXPRESS", "Compras"), ("SHEIN", "Compras"),
        ("RENNER", "Compras"), ("RIACHUELO", "Compras"), ("CENTAURO", "Compras"), ("KABUM", "Compras"),
        ("CASAS BAHIA", "Compras"), ("AMERICANAS", "Compras"), ("LEROY", "Compras"),
        ("ESCOLA", "Educação"), ("FACULDADE", "Educação"), ("UNIVERSIDADE", "Educação"), ("CURSO", "Educação"),
        ("UDEMY", "Educação"), ("ALURA", "Educação"), ("LIVRARIA", "Educação"),
        ("CINEMA", "Lazer"), ("CINEMARK", "Lazer"), ("INGRESSO", "Lazer"), ("SYMPLA", "Lazer"), ("STEAM", "Lazer"),
        ("PLAYSTATION", "Lazer"), ("XBOX", "Lazer"), ("NINTENDO", "Lazer"),
        ("HOTEL", "Viagens"), ("AIRBNB", "Viagens"), ("BOOKING", "Viagens"), ("LATAM", "Viagens"),
        ("GOL LINHAS", "Viagens"), ("AZUL LINHAS", "Viagens"), ("DECOLAR", "Viagens"), ("123MILHAS", "Viagens"),
        ("PETZ", "Pets"), ("COBASI", "Pets"), ("PET SHOP", "Pets"), ("PETSHOP", "Pets"), ("VETERINAR", "Pets"),
        ("SALAO", "Cuidados pessoais"), ("BARBEARIA", "Cuidados pessoais"), ("ACADEMIA", "Cuidados pessoais"),
        ("SMART FIT", "Cuidados pessoais"), ("BOTICARIO", "Cuidados pessoais"), ("NATURA", "Cuidados pessoais"),
        ("IPVA", "Impostos e taxas"), ("IPTU", "Impostos e taxas"), ("DARF", "Impostos e taxas"),
        ("DETRAN", "Impostos e taxas"), ("LICENCIAMENTO", "Impostos e taxas"),
        ("TARIFA", "Tarifas bancárias"), ("ANUIDADE", "Tarifas bancárias"), ("IOF", "Tarifas bancárias"),
        ("CESTA DE SERVICOS", "Tarifas bancárias"),
        ("SEGURO", "Seguros"), ("PORTO SEGURO", "Seguros"),
        ("JUROS", "Juros e empréstimos"), ("ENCARGOS", "Juros e empréstimos"), ("EMPRESTIMO", "Juros e empréstimos"),
        ("SALARIO", "Salário"), ("PROVENTOS", "Rendimentos"), ("RENDIMENTO", "Rendimentos"),
        ("PAGAMENTO FATURA", "Pagamento de fatura"), ("PAG FATURA", "Pagamento de fatura")
    };

    private readonly FinancasDbContext _db;

    public SugestaoCategoriaService(FinancasDbContext db) => _db = db;

    private sealed record Categoria(int Id, string Nome, string Cor, TipoCategoria Tipo);

    private sealed class Indice
    {
        public Dictionary<string, Dictionary<int, double>> PorChave { get; } = new();
        public Dictionary<string, Dictionary<int, double>> PorPalavra { get; } = new();
        public Dictionary<int, Categoria> Categorias { get; init; } = new();
        public Dictionary<string, int> PorNome { get; init; } = new();
        /// <summary>#254: o texto que as regras testam (descrição do banco, normalizada) e a categoria de cada transação.</summary>
        public List<(string Texto, int CategoriaId)> Textos { get; } = new();
        /// <summary>#254: regras "contém" que já existem: texto normalizado → categoria.</summary>
        public Dictionary<string, int> Regras { get; init; } = new();
    }

    /* ------------------------------ normalização ------------------------------ */

    /// <summary>Palavras significativas da descrição (sem números, acentos e ruído).</summary>
    public static List<string> Palavras(string? descricao)
    {
        var texto = CategorizacaoService.Normalizar(descricao);
        var sb = new StringBuilder(texto.Length);
        foreach (var c in texto) sb.Append(char.IsLetter(c) ? c : ' ');
        return sb.ToString().Split(' ', StringSplitOptions.RemoveEmptyEntries)
            .Where(p => p.Length >= 3 && !Ruido.Contains(p))
            .ToList();
    }

    /// <summary>Chave de agrupamento: até 3 primeiras palavras significativas.</summary>
    public static string Chave(string? descricao)
    {
        var palavras = Palavras(descricao);
        return palavras.Count == 0 ? string.Empty : string.Join(' ', palavras.Take(3));
    }

    /* ------------------------------ índice do histórico ------------------------------ */

    private async Task<Indice> MontarIndiceAsync(CancellationToken ct)
    {
        var categorias = await _db.Categorias.AsNoTracking()
            .Select(c => new Categoria(c.Id, c.Nome, c.Cor, c.Tipo)).ToListAsync(ct);
        var regras = await _db.RegrasCategorizacao.AsNoTracking()
            .Where(r => r.TipoCorrespondencia == TipoCorrespondencia.Contem)
            .Select(r => new { r.Texto, r.CategoriaId }).ToListAsync(ct);
        var indice = new Indice
        {
            Categorias = categorias.ToDictionary(c => c.Id),
            PorNome = categorias.GroupBy(c => CategorizacaoService.Normalizar(c.Nome))
                .ToDictionary(g => g.Key, g => g.First().Id),
            Regras = regras.GroupBy(r => CategorizacaoService.Normalizar(r.Texto))
                .ToDictionary(g => g.Key, g => g.First().CategoriaId)
        };

        var desde = Relogio.Hoje.AddDays(-DiasHistorico);
        var historico = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= desde && t.CategoriaId != null)
            .Select(t => new { t.Descricao, t.DescricaoOriginal, t.Estabelecimento, CategoriaId = t.CategoriaId!.Value, t.CategoriaManual })
            .ToListAsync(ct);

        foreach (var h in historico)
        {
            var peso = h.CategoriaManual ? 3d : 1d; // escolhas suas valem mais que as automáticas
            var texto = h.Estabelecimento ?? h.DescricaoOriginal ?? h.Descricao;
            var chave = Chave(texto);
            if (chave.Length > 0) Somar(indice.PorChave, chave, h.CategoriaId, peso);
            var primeira = Palavras(texto).FirstOrDefault();
            if (primeira is not null && primeira.Length >= 4) Somar(indice.PorPalavra, primeira, h.CategoriaId, peso);
            indice.Textos.Add((CategorizacaoService.Normalizar(h.DescricaoOriginal ?? h.Descricao), h.CategoriaId));
        }
        return indice;
    }

    private static void Somar(Dictionary<string, Dictionary<int, double>> mapa, string chave, int categoria, double peso)
    {
        if (!mapa.TryGetValue(chave, out var porCategoria)) mapa[chave] = porCategoria = new();
        porCategoria[categoria] = porCategoria.GetValueOrDefault(categoria) + peso;
    }

    /// <summary>Opções ordenadas (melhor primeiro) com o nível de confiança da primeira.</summary>
    private static (List<OpcaoCategoriaDto> Opcoes, string Confianca) Avaliar(Indice indice, string texto, bool receita,
        int? ignorarCategoria = null)
    {
        var opcoes = new List<(int Id, double Pontos, string Motivo)>();
        var confianca = "nenhuma";

        void Adicionar(Dictionary<int, double>? votos, string origem, double fator)
        {
            if (votos is null) return;
            var total = votos.Values.Sum();
            foreach (var (id, pontos) in votos.OrderByDescending(v => v.Value))
            {
                opcoes.Add((id, pontos / total * fator, $"{origem} ({pontos / total:P0} das {Math.Max(1, (int)Math.Round(total))} parecidas)"));
            }
        }

        var chave = Chave(texto);
        if (chave.Length > 0 && indice.PorChave.TryGetValue(chave, out var porChave))
        {
            Adicionar(porChave, "Você já usou para transações iguais", 3);
            var total = porChave.Values.Sum();
            var melhor = porChave.Values.Max();
            confianca = melhor / total >= 0.7 && total >= 2 ? "alta" : "media";
        }

        var primeira = Palavras(texto).FirstOrDefault();
        if (primeira is not null && indice.PorPalavra.TryGetValue(primeira, out var porPalavra))
        {
            Adicionar(porPalavra, $"Usada em transações com \"{primeira}\"", 1.5);
            if (confianca == "nenhuma") confianca = "media";
        }

        var normalizado = " " + CategorizacaoService.Normalizar(texto) + " ";
        foreach (var (palavra, categoria) in PalavrasChave)
        {
            if (!normalizado.Contains(palavra, StringComparison.Ordinal)) continue;
            if (!indice.PorNome.TryGetValue(CategorizacaoService.Normalizar(categoria), out var id)) continue;
            opcoes.Add((id, 1, $"Palavra-chave \"{palavra.Trim()}\""));
            if (confianca == "nenhuma") confianca = "baixa";
            break;
        }

        var lista = opcoes
            .Where(o => indice.Categorias.TryGetValue(o.Id, out var cat) && o.Id != ignorarCategoria
                        && cat.Tipo != (receita ? TipoCategoria.Despesa : TipoCategoria.Receita))
            .GroupBy(o => o.Id)
            .Select(g => (Id: g.Key, Pontos: g.Sum(x => x.Pontos), Motivo: g.OrderByDescending(x => x.Pontos).First().Motivo))
            .OrderByDescending(o => o.Pontos)
            .Take(3)
            .Select(o =>
            {
                var c = indice.Categorias[o.Id];
                return new OpcaoCategoriaDto(c.Id, c.Nome, c.Cor, o.Motivo);
            })
            .ToList();
        return (lista, lista.Count == 0 ? "nenhuma" : confianca);
    }

    /* ------------------------------ API ------------------------------ */

    public async Task<List<SugestaoCategoriaDto>> SugerirGruposAsync(bool somenteSemCategoria, int dias, CancellationToken ct)
    {
        var indice = await MontarIndiceAsync(ct);
        var desde = Relogio.Hoje.AddDays(-dias);
        var consulta = _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= desde && t.TransferenciaGrupo == null && !t.CategoriaManual);
        if (somenteSemCategoria) consulta = consulta.Where(t => t.CategoriaId == null);
        var transacoes = await consulta
            .OrderByDescending(t => t.Data)
            .Select(t => new { t.Id, t.Descricao, t.DescricaoOriginal, t.Estabelecimento, t.Valor, t.CategoriaId })
            .Take(5000)
            .ToListAsync(ct);

        var grupos = transacoes
            .GroupBy(t => Chave(t.Estabelecimento ?? t.DescricaoOriginal ?? t.Descricao) is { Length: > 0 } k ? k : t.Descricao.Trim().ToUpperInvariant())
            .Select(g => new { Chave = g.Key, Itens = g.ToList() })
            .OrderByDescending(g => g.Itens.Count).ThenByDescending(g => g.Itens.Sum(t => Math.Abs(t.Valor)))
            .Take(MaximoGrupos)
            .ToList();

        var resultado = new List<SugestaoCategoriaDto>();
        foreach (var g in grupos)
        {
            var exemplo = g.Itens[0];
            var texto = exemplo.Estabelecimento ?? exemplo.DescricaoOriginal ?? exemplo.Descricao;
            // Se todas já estão na categoria sugerida, não há o que sugerir.
            var categoriaAtual = g.Itens.Select(t => t.CategoriaId).Distinct().Count() == 1 ? exemplo.CategoriaId : null;
            var (opcoes, confianca) = Avaliar(indice, texto, exemplo.Valor > 0);
            if (!somenteSemCategoria && (opcoes.Count == 0 || opcoes[0].CategoriaId == categoriaAtual)) continue;

            var principal = opcoes.FirstOrDefault();
            // #254: a regra testa a descrição do banco (CategorizacaoService.Resolver), e não o estabelecimento da chave.
            var textoRegra = TextoRegra(g.Itens.Select(t => t.DescricaoOriginal ?? t.Descricao));
            resultado.Add(new SugestaoCategoriaDto(g.Chave, exemplo.Descricao, g.Itens.Count,
                Math.Round(g.Itens.Sum(t => t.Valor), 2), g.Itens.Select(t => t.Id).Take(1000).ToList(),
                principal?.CategoriaId, principal?.Nome, principal?.Cor, confianca, principal?.Motivo,
                textoRegra, opcoes.Skip(1).ToList(), AvisoRegra(indice, textoRegra, principal?.CategoriaId)));
        }

        // Primeiro o que dá para aplicar com segurança; depois os grupos maiores.
        var ordem = new Dictionary<string, int> { ["alta"] = 0, ["media"] = 1, ["baixa"] = 2, ["nenhuma"] = 3 };
        return resultado.OrderBy(r => ordem.GetValueOrDefault(r.Confianca, 3)).ThenByDescending(r => r.Quantidade).ToList();
    }

    public async Task<List<OpcaoCategoriaDto>?> SugerirParaTransacaoAsync(long id, CancellationToken ct)
    {
        var t = await _db.Transacoes.AsNoTracking().Where(x => x.Id == id)
            .Select(x => new { x.Descricao, x.DescricaoOriginal, x.Estabelecimento, x.Valor, x.CategoriaId })
            .FirstOrDefaultAsync(ct);
        if (t is null) return null;
        var indice = await MontarIndiceAsync(ct);
        var (opcoes, _) = Avaliar(indice, t.Estabelecimento ?? t.DescricaoOriginal ?? t.Descricao, t.Valor > 0, t.CategoriaId);
        return opcoes;
    }

    /// <summary>
    /// Texto sugerido para a regra "contém", tirado das descrições do banco do grupo (o que a regra testa, #254): a primeira
    /// palavra que identifica (não genérica, com 4 letras ou mais) presente em quase todas; curta, vai junto com a seguinte
    /// (ex.: "UBER EATS"). Sem palavra assim, o começo comum das descrições, se for longo e tiver alguma palavra que
    /// identifique (ex.: "PAGAMENTO CARTAO CREDITO BCE"); senão, vazio (sem regra).
    /// </summary>
    public static string TextoRegra(IEnumerable<string?> descricoes)
    {
        var textos = descricoes.Select(CategorizacaoService.Normalizar).Where(t => t.Length > 0).Distinct().ToList();
        if (textos.Count == 0) return string.Empty;
        var listas = textos.Select(Palavras).ToList();
        var minimo = Math.Max(1, (int)Math.Ceiling(textos.Count * 0.8));
        var primeiras = listas[0];
        var palavra = primeiras
            .Where(p => p.Length >= 4 && !GenericasParaRegra.Contains(p))
            .FirstOrDefault(p => listas.Count(l => l.Contains(p)) >= minimo);
        if (palavra is not null)
        {
            var i = primeiras.IndexOf(palavra);
            if (palavra.Length <= 4 && i + 1 < primeiras.Count)
            {
                var dupla = $"{palavra} {primeiras[i + 1]}";
                if (textos.Count(t => t.Contains(dupla, StringComparison.Ordinal)) >= minimo) return dupla;
            }
            return palavra;
        }
        // O começo comum só serve se tiver alguma palavra que identifique: "PIX ENVIADO" sozinho pegaria todo Pix.
        var prefixo = PrefixoComum(textos);
        return prefixo.Length >= 10 && Palavras(prefixo).Any(p => !GenericasParaRegra.Contains(p)) ? prefixo : string.Empty;
    }

    /// <summary>Começo comum dos textos, cortado no fim da última palavra inteira.</summary>
    private static string PrefixoComum(List<string> textos)
    {
        var tamanho = textos.Min(t => t.Length);
        var n = 0;
        while (n < tamanho && textos.All(t => t[n] == textos[0][n])) n++;
        var prefixo = textos[0][..n];
        if (n < textos[0].Length && char.IsLetterOrDigit(textos[0][n]) && textos.All(t => t.Length > n))
            prefixo = prefixo[..Math.Max(0, prefixo.LastIndexOf(' '))];
        return prefixo.Trim();
    }

    /// <summary>
    /// #254: por que não criar a regra sugerida sem olhar: ela já existe, ou o texto também aparece em transações de
    /// outras categorias (a regra mudaria as próximas assim). Nulo quando não há nada a avisar.
    /// </summary>
    private static string? AvisoRegra(Indice indice, string texto, int? categoria)
    {
        if (texto.Length == 0) return null;
        if (indice.Regras.TryGetValue(texto, out var existente))
            return existente == categoria
                ? $"A regra \"{texto}\" já existe."
                : $"Já existe a regra \"{texto}\" para {NomeCategoria(indice, existente)}.";
        var achadas = 0;
        var outras = new Dictionary<int, int>();
        foreach (var (t, c) in indice.Textos)
        {
            if (!t.Contains(texto, StringComparison.Ordinal)) continue;
            achadas++;
            if (c != categoria) outras[c] = outras.GetValueOrDefault(c) + 1;
        }
        var conflitos = outras.Values.Sum();
        if (conflitos < ConflitosParaAviso || conflitos < achadas * FracaoConflitosParaAviso) return null;
        var exemplos = string.Join(", ", outras.OrderByDescending(o => o.Value).Take(2).Select(o => NomeCategoria(indice, o.Key)));
        return $"\"{texto}\" também aparece em {conflitos} transações de outras categorias ({exemplos}).";
    }

    private static string NomeCategoria(Indice indice, int id) =>
        indice.Categorias.TryGetValue(id, out var c) ? c.Nome : "outra categoria";
}
