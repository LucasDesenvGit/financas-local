using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Data;

/// <summary>
/// Mapeamento das entidades para a estrutura criada pelos scripts em \banco.
/// Não utiliza migrations: o banco é criado/atualizado pelos scripts SQL.
/// </summary>
public class FinancasDbContext : DbContext
{
    public FinancasDbContext(DbContextOptions<FinancasDbContext> options) : base(options) { }

    public DbSet<Usuario> Usuarios => Set<Usuario>();
    public DbSet<AuditoriaAcesso> AuditoriasAcesso => Set<AuditoriaAcesso>();
    public DbSet<Configuracao> Configuracoes => Set<Configuracao>();
    public DbSet<ConexaoBancaria> ConexoesBancarias => Set<ConexaoBancaria>();
    public DbSet<SincronizacaoLog> SincronizacaoLogs => Set<SincronizacaoLog>();
    public DbSet<Categoria> Categorias => Set<Categoria>();
    public DbSet<CategoriaPluggy> CategoriasPluggy => Set<CategoriaPluggy>();
    public DbSet<RegraCategorizacao> RegrasCategorizacao => Set<RegraCategorizacao>();
    public DbSet<Conta> Contas => Set<Conta>();
    public DbSet<FaturaCartao> FaturasCartao => Set<FaturaCartao>();
    public DbSet<Transacao> Transacoes => Set<Transacao>();
    public DbSet<Investimento> Investimentos => Set<Investimento>();
    public DbSet<Emprestimo> Emprestimos => Set<Emprestimo>();
    public DbSet<PagamentoEmprestimo> PagamentosEmprestimo => Set<PagamentoEmprestimo>();
    public DbSet<Orcamento> Orcamentos => Set<Orcamento>();
    public DbSet<Meta> Metas => Set<Meta>();
    public DbSet<ContaAgendada> ContasAgendadas => Set<ContaAgendada>();
    public DbSet<Ativo> Ativos => Set<Ativo>();
    public DbSet<OperacaoAtivo> OperacoesAtivo => Set<OperacaoAtivo>();
    public DbSet<ProventoAtivo> ProventosAtivo => Set<ProventoAtivo>();
    public DbSet<TaxaSelicDiaria> TaxasSelic => Set<TaxaSelicDiaria>();
    public DbSet<PrecoMensalAtivo> PrecosMensaisAtivo => Set<PrecoMensalAtivo>();
    public DbSet<IndiceMensal> IndicesMensais => Set<IndiceMensal>();

    protected override void OnModelCreating(ModelBuilder mb)
    {
        mb.Entity<Usuario>(e =>
        {
            e.ToTable("Usuario");
            e.HasKey(x => x.Id);
            e.Property(x => x.Login).HasMaxLength(100).IsRequired();
            e.Property(x => x.SenhaHash).HasMaxLength(500).IsRequired();
            e.HasIndex(x => x.Login).IsUnique();
        });

        mb.Entity<AuditoriaAcesso>(e =>
        {
            e.ToTable("AuditoriaAcesso");
            e.HasKey(x => x.Id);
            e.Property(x => x.Evento).HasMaxLength(50).IsRequired();
            e.Property(x => x.Login).HasMaxLength(100);
            e.Property(x => x.Ip).HasMaxLength(64);
            e.Property(x => x.UserAgent).HasMaxLength(400);
        });

        mb.Entity<Configuracao>(e =>
        {
            e.ToTable("Configuracao");
            e.HasKey(x => x.Chave);
            e.Property(x => x.Chave).HasMaxLength(100);
        });

        mb.Entity<ConexaoBancaria>(e =>
        {
            e.ToTable("ConexaoBancaria");
            e.HasKey(x => x.Id);
            e.Property(x => x.PluggyItemId).HasMaxLength(64).IsRequired();
            e.Property(x => x.NomeInstituicao).HasMaxLength(150);
            e.Property(x => x.CorPrimaria).HasMaxLength(9);
            e.Property(x => x.Status).HasMaxLength(40);
            e.Property(x => x.StatusExecucao).HasMaxLength(60);
            e.Property(x => x.UltimoErro).HasMaxLength(2000);
            e.HasIndex(x => x.PluggyItemId).IsUnique();
        });

        mb.Entity<SincronizacaoLog>(e =>
        {
            e.ToTable("SincronizacaoLog");
            e.HasKey(x => x.Id);
            e.Property(x => x.Origem).HasMaxLength(20).IsRequired();
            e.Property(x => x.Mensagem).HasMaxLength(2000);
            e.HasOne(x => x.ConexaoBancaria).WithMany()
                .HasForeignKey(x => x.ConexaoBancariaId).OnDelete(DeleteBehavior.SetNull);
        });

        mb.Entity<Categoria>(e =>
        {
            e.ToTable("Categoria");
            e.HasKey(x => x.Id);
            e.Property(x => x.Nome).HasMaxLength(80).IsRequired();
            e.Property(x => x.Cor).HasMaxLength(9).IsRequired();
            e.Property(x => x.Icone).HasMaxLength(40);
            e.HasIndex(x => x.Nome).IsUnique();
            e.HasOne(x => x.CategoriaPai).WithMany(x => x.Subcategorias)
                .HasForeignKey(x => x.CategoriaPaiId).OnDelete(DeleteBehavior.Restrict);
        });

        mb.Entity<CategoriaPluggy>(e =>
        {
            e.ToTable("CategoriaPluggy");
            e.HasKey(x => x.PluggyCategoriaId);
            e.Property(x => x.PluggyCategoriaId).HasMaxLength(20);
            e.Property(x => x.Descricao).HasMaxLength(150).IsRequired();
            e.Property(x => x.PaiPluggyId).HasMaxLength(20);
            e.Property(x => x.PaiDescricao).HasMaxLength(150);
            e.HasOne(x => x.Categoria).WithMany()
                .HasForeignKey(x => x.CategoriaId).OnDelete(DeleteBehavior.SetNull);
        });

        mb.Entity<RegraCategorizacao>(e =>
        {
            e.ToTable("RegraCategorizacao");
            e.HasKey(x => x.Id);
            e.Property(x => x.Texto).HasMaxLength(200).IsRequired();
            e.HasOne(x => x.Categoria).WithMany()
                .HasForeignKey(x => x.CategoriaId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<Conta>(e =>
        {
            e.ToTable("Conta");
            e.HasKey(x => x.Id);
            e.Property(x => x.PluggyAccountId).HasMaxLength(64);
            e.Property(x => x.Nome).HasMaxLength(120).IsRequired();
            e.Property(x => x.Numero).HasMaxLength(60);
            e.Property(x => x.Moeda).HasMaxLength(3);
            e.Property(x => x.SaldoInicial).HasPrecision(18, 2);
            e.Property(x => x.Saldo).HasPrecision(18, 2);
            e.Property(x => x.LimiteCredito).HasPrecision(18, 2);
            e.Property(x => x.LimiteDisponivel).HasPrecision(18, 2);
            e.Property(x => x.Cor).HasMaxLength(9).IsRequired();
            e.HasIndex(x => x.PluggyAccountId).IsUnique().HasFilter("PluggyAccountId IS NOT NULL");
            e.HasOne(x => x.ConexaoBancaria).WithMany(x => x.Contas)
                .HasForeignKey(x => x.ConexaoBancariaId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<FaturaCartao>(e =>
        {
            e.ToTable("FaturaCartao");
            e.HasKey(x => x.Id);
            e.Property(x => x.PluggyBillId).HasMaxLength(64);
            e.Property(x => x.ValorTotal).HasPrecision(18, 2);
            e.Property(x => x.ValorMinimo).HasPrecision(18, 2);
            e.Property(x => x.ValorPago).HasPrecision(18, 2);
            e.HasIndex(x => x.PluggyBillId).IsUnique().HasFilter("PluggyBillId IS NOT NULL");
            e.HasOne(x => x.Conta).WithMany(x => x.Faturas)
                .HasForeignKey(x => x.ContaId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<Transacao>(e =>
        {
            e.ToTable("Transacao");
            e.HasKey(x => x.Id);
            e.Property(x => x.PluggyTransactionId).HasMaxLength(64);
            e.Property(x => x.Descricao).HasMaxLength(300).IsRequired();
            e.Property(x => x.DescricaoOriginal).HasMaxLength(300);
            e.Property(x => x.Estabelecimento).HasMaxLength(200);
            e.Property(x => x.Valor).HasPrecision(18, 2);
            e.Property(x => x.PluggyCategoriaId).HasMaxLength(20);
            e.Property(x => x.Observacao).HasMaxLength(1000);
            e.HasIndex(x => x.PluggyTransactionId).IsUnique().HasFilter("PluggyTransactionId IS NOT NULL");
            e.HasOne(x => x.Conta).WithMany(x => x.Transacoes)
                .HasForeignKey(x => x.ContaId).OnDelete(DeleteBehavior.Cascade);
            e.HasOne(x => x.Categoria).WithMany()
                .HasForeignKey(x => x.CategoriaId).OnDelete(DeleteBehavior.SetNull);
            e.HasOne(x => x.FaturaCartao).WithMany()
                .HasForeignKey(x => x.FaturaCartaoId).OnDelete(DeleteBehavior.ClientSetNull);
        });

        mb.Entity<Investimento>(e =>
        {
            e.ToTable("Investimento");
            e.HasKey(x => x.Id);
            e.Property(x => x.PluggyInvestmentId).HasMaxLength(64).IsRequired();
            e.Property(x => x.Nome).HasMaxLength(200).IsRequired();
            e.Property(x => x.Tipo).HasMaxLength(30).IsRequired();
            e.Property(x => x.Subtipo).HasMaxLength(60);
            e.Property(x => x.Codigo).HasMaxLength(60);
            e.Property(x => x.Emissor).HasMaxLength(200);
            e.Property(x => x.Moeda).HasMaxLength(3);
            e.Property(x => x.Saldo).HasPrecision(18, 2);
            e.Property(x => x.ValorAplicado).HasPrecision(18, 2);
            e.Property(x => x.Rendimento).HasPrecision(18, 2);
            e.Property(x => x.Quantidade).HasPrecision(28, 8);
            e.Property(x => x.TaxaAnual).HasPrecision(9, 4);
            e.Property(x => x.Status).HasMaxLength(30);
            e.HasIndex(x => x.PluggyInvestmentId).IsUnique();
            e.HasOne(x => x.ConexaoBancaria).WithMany(x => x.Investimentos)
                .HasForeignKey(x => x.ConexaoBancariaId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<Emprestimo>(e =>
        {
            e.ToTable("Emprestimo");
            e.HasKey(x => x.Id);
            e.Property(x => x.PluggyLoanId).HasMaxLength(64).IsRequired();
            e.Property(x => x.Modalidade).HasMaxLength(80);
            e.Property(x => x.Produto).HasMaxLength(200).IsRequired();
            e.Property(x => x.NumeroContrato).HasMaxLength(100);
            e.Property(x => x.Moeda).HasMaxLength(3);
            e.Property(x => x.ValorContratado).HasPrecision(18, 2);
            e.Property(x => x.SaldoDevedor).HasPrecision(18, 2);
            e.Property(x => x.ValorParcela).HasPrecision(18, 2);
            e.Property(x => x.Periodicidade).HasMaxLength(30);
            e.Property(x => x.SistemaAmortizacao).HasMaxLength(30);
            e.Property(x => x.TaxaJuros).HasPrecision(12, 8);
            e.Property(x => x.PeriodicidadeTaxa).HasMaxLength(10);
            e.Property(x => x.Indexador).HasMaxLength(60);
            e.Property(x => x.Cet).HasPrecision(12, 8);
            e.HasIndex(x => x.PluggyLoanId).IsUnique();
            e.HasOne(x => x.ConexaoBancaria).WithMany(x => x.Emprestimos)
                .HasForeignKey(x => x.ConexaoBancariaId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<PagamentoEmprestimo>(e =>
        {
            e.ToTable("PagamentoEmprestimo");
            e.HasKey(x => x.Id);
            e.Property(x => x.Valor).HasPrecision(18, 2);
            e.HasIndex(x => new { x.EmprestimoId, x.Data });
            e.HasOne(x => x.Emprestimo).WithMany(x => x.Pagamentos)
                .HasForeignKey(x => x.EmprestimoId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<Orcamento>(e =>
        {
            e.ToTable("Orcamento");
            e.HasKey(x => x.Id);
            e.Property(x => x.Valor).HasPrecision(18, 2);
            e.HasIndex(x => new { x.CategoriaId, x.Ano, x.Mes }).IsUnique();
            e.HasOne(x => x.Categoria).WithMany()
                .HasForeignKey(x => x.CategoriaId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<Meta>(e =>
        {
            e.ToTable("Meta");
            e.HasKey(x => x.Id);
            e.Property(x => x.Nome).HasMaxLength(120).IsRequired();
            e.Property(x => x.ValorAlvo).HasPrecision(18, 2);
            e.Property(x => x.ValorAtual).HasPrecision(18, 2);
            e.Property(x => x.Cor).HasMaxLength(9).IsRequired();
            e.HasOne(x => x.Conta).WithMany()
                .HasForeignKey(x => x.ContaId).OnDelete(DeleteBehavior.SetNull);
        });

        mb.Entity<Ativo>(e =>
        {
            e.ToTable("Ativo");
            e.HasKey(x => x.Id);
            e.Property(x => x.Ticker).HasMaxLength(12).IsRequired();
            e.Property(x => x.Nome).HasMaxLength(150);
            e.Property(x => x.PrecoAtual).HasPrecision(18, 4);
            e.Property(x => x.PrecoAnterior).HasPrecision(18, 4);
            e.Property(x => x.FonteCotacao).HasMaxLength(20);
            e.Property(x => x.Segmento).HasMaxLength(40);
            e.HasIndex(x => x.Ticker).IsUnique();
        });

        mb.Entity<OperacaoAtivo>(e =>
        {
            e.ToTable("OperacaoAtivo");
            e.HasKey(x => x.Id);
            e.Property(x => x.Quantidade).HasPrecision(18, 6);
            e.Property(x => x.PrecoUnitario).HasPrecision(18, 6);
            e.Property(x => x.Taxas).HasPrecision(18, 2);
            e.Property(x => x.Observacao).HasMaxLength(300);
            e.Property(x => x.PluggyTransacaoId).HasMaxLength(100);
            e.HasIndex(x => x.PluggyTransacaoId).IsUnique().HasFilter("PluggyTransacaoId IS NOT NULL");
            e.HasIndex(x => x.TransacaoId).IsUnique().HasFilter("TransacaoId IS NOT NULL");
            e.HasOne(x => x.Ativo).WithMany(x => x.Operacoes)
                .HasForeignKey(x => x.AtivoId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<ProventoAtivo>(e =>
        {
            e.ToTable("ProventoAtivo");
            e.HasKey(x => x.Id);
            e.Property(x => x.ValorPorCota).HasPrecision(18, 8);
            e.HasIndex(x => new { x.AtivoId, x.DataCom }).IsUnique();
            e.HasOne(x => x.Ativo).WithMany(x => x.Proventos)
                .HasForeignKey(x => x.AtivoId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<PrecoMensalAtivo>(e =>
        {
            e.ToTable("PrecoMensalAtivo");
            e.HasKey(x => new { x.AtivoId, x.Mes });
            e.Property(x => x.Fechamento).HasPrecision(18, 4);
            e.HasOne(x => x.Ativo).WithMany(x => x.PrecosMensais)
                .HasForeignKey(x => x.AtivoId).OnDelete(DeleteBehavior.Cascade);
        });

        mb.Entity<IndiceMensal>(e =>
        {
            e.ToTable("IndiceMensal");
            e.HasKey(x => new { x.Codigo, x.Mes });
            e.Property(x => x.Codigo).HasMaxLength(12);
            e.Property(x => x.Fechamento).HasPrecision(18, 4);
        });

        mb.Entity<TaxaSelicDiaria>(e =>
        {
            e.ToTable("TaxaSelicDiaria");
            e.HasKey(x => x.Data);
            e.Property(x => x.Taxa).HasPrecision(12, 8);
        });

        mb.Entity<ContaAgendada>(e =>
        {
            e.ToTable("ContaAgendada");
            e.HasKey(x => x.Id);
            e.Property(x => x.Descricao).HasMaxLength(150).IsRequired();
            e.Property(x => x.Valor).HasPrecision(18, 2);
            e.HasOne(x => x.Categoria).WithMany()
                .HasForeignKey(x => x.CategoriaId).OnDelete(DeleteBehavior.SetNull);
            e.HasOne(x => x.Conta).WithMany()
                .HasForeignKey(x => x.ContaId).OnDelete(DeleteBehavior.SetNull);
        });
    }
}
