import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
  OpcaoCategoria, SugestaoCategoria,
  AtualizarAtivo, Carteira, CotacaoMoeda, MoedaCambio, ResultadoImportacaoCarteira, ImportarExtratoItem, OperacaoAtivo, PainelFiis, ResultadoCotacoes, SalvarOperacao,
  SugestaoExtrato,
  AjusteTransacao, Auditoria, Categoria, CategoriaPluggy, Conexao, ConfiguracaoPluggy, Conta, ContaAgendada,
  Fatura, FiltroTransacoes, Investimento, Mensagem, MesValores, Meta, Orcamento, PaginaTransacoes, Painel, Regra,
  PainelAnual, Projecao, ResultadoAplicacao, ResultadoSincronizacao, SalvarCategoria, SalvarConfiguracaoPluggy, SalvarConta,
  SalvarContaAgendada, SalvarMeta, SugestaoContaAgendada, SugestaoOrcamento, SalvarRegra, SalvarTransacao, SincronizacaoLog, Transacao, Transferencia,
  TipoTransacao, ValorCategoria, PainelEmprestimos, Parametro, InfoSistema, StatusSincronizacao, Reserva, PlanoMetas,
  ErroFrontEnd, ErroSistemaDetalhe, FiltroErros, PaginaErros
} from './modelos';

/** Todas as chamadas usam URLs relativas: mesma origem do IIS (sem CORS) e o Angular anexa o token CSRF. */
@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly base = '/api';

  private params(obj: object): HttpParams {
    let p = new HttpParams();
    for (const [chave, valor] of Object.entries(obj)) {
      // Lista vira a chave repetida (?categoriaIds=1&categoriaIds=2), que é o que o ASP.NET entende como List<int>.
      if (Array.isArray(valor)) for (const v of valor) p = p.append(chave, String(v));
      else if (valor !== null && valor !== undefined && valor !== '') p = p.set(chave, String(valor));
    }
    return p;
  }

  // Painel e relatórios
  painel(ano: number, mes: number): Observable<Painel> {
    return this.http.get<Painel>(`${this.base}/painel`, { params: this.params({ ano, mes }) });
  }
  relatorioCategorias(inicio: string, fim: string, tipo: TipoTransacao, agrupar = true): Observable<ValorCategoria[]> {
    return this.http.get<ValorCategoria[]>(`${this.base}/relatorios/categorias`, { params: this.params({ inicio, fim, tipo, agrupar }) });
  }
  relatorioFluxo(inicio: string, fim: string): Observable<MesValores[]> {
    return this.http.get<MesValores[]>(`${this.base}/relatorios/fluxo`, { params: this.params({ inicio, fim }) });
  }
  painelAnual(ano: number, comparar: number): Observable<PainelAnual> {
    return this.http.get<PainelAnual>(`${this.base}/relatorios/anual`, { params: this.params({ ano, comparar }) });
  }
  projecao(meses: number): Observable<Projecao> {
    return this.http.get<Projecao>(`${this.base}/relatorios/projecao`, { params: this.params({ meses }) });
  }
  maioresDespesas(inicio: string, fim: string): Observable<Transacao[]> {
    return this.http.get<Transacao[]>(`${this.base}/relatorios/maiores-despesas`, { params: this.params({ inicio, fim }) });
  }

  // Contas
  contas(incluirArquivadas = false): Observable<Conta[]> {
    return this.http.get<Conta[]>(`${this.base}/contas`, { params: this.params({ incluirArquivadas }) });
  }
  criarConta(dto: SalvarConta): Observable<Conta> { return this.http.post<Conta>(`${this.base}/contas`, dto); }
  atualizarConta(id: number, dto: SalvarConta): Observable<Conta> { return this.http.put<Conta>(`${this.base}/contas/${id}`, dto); }
  excluirConta(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/contas/${id}`); }
  faturas(contaId: number): Observable<Fatura[]> { return this.http.get<Fatura[]>(`${this.base}/contas/${contaId}/faturas`); }

  // Transações
  transacoes(filtro: FiltroTransacoes): Observable<PaginaTransacoes> {
    return this.http.get<PaginaTransacoes>(`${this.base}/transacoes`, { params: this.params(filtro) });
  }
  criarTransacao(dto: SalvarTransacao): Observable<Transacao> { return this.http.post<Transacao>(`${this.base}/transacoes`, dto); }
  atualizarTransacao(id: number, dto: SalvarTransacao): Observable<Transacao> {
    return this.http.put<Transacao>(`${this.base}/transacoes/${id}`, dto);
  }
  ajustarTransacao(id: number, dto: AjusteTransacao): Observable<Transacao> {
    return this.http.patch<Transacao>(`${this.base}/transacoes/${id}`, dto);
  }
  excluirTransacao(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/transacoes/${id}`); }
  marcarEmergencia(ids: number[], emergencia: boolean): Observable<ResultadoAplicacao> {
    return this.http.put<ResultadoAplicacao>(`${this.base}/transacoes/emergencia-em-lote`, { ids, emergencia });
  }
  sugestoesCategoria(somenteSemCategoria: boolean, dias: number): Observable<SugestaoCategoria[]> {
    return this.http.get<SugestaoCategoria[]>(`${this.base}/transacoes/sugestoes-categoria`,
      { params: this.params({ somenteSemCategoria, dias }) });
  }
  sugestoesDaTransacao(id: number): Observable<OpcaoCategoria[]> {
    return this.http.get<OpcaoCategoria[]>(`${this.base}/transacoes/${id}/sugestoes-categoria`);
  }
  categorizarEmLote(ids: number[], categoriaId: number | null): Observable<ResultadoAplicacao> {
    return this.http.put<ResultadoAplicacao>(`${this.base}/transacoes/categoria-em-lote`, { ids, categoriaId });
  }
  transferir(dto: Transferencia): Observable<void> { return this.http.post<void>(`${this.base}/transacoes/transferencia`, dto); }

  // Categorias e regras
  categorias(): Observable<Categoria[]> { return this.http.get<Categoria[]>(`${this.base}/categorias`); }
  criarCategoria(dto: SalvarCategoria): Observable<Categoria> { return this.http.post<Categoria>(`${this.base}/categorias`, dto); }
  atualizarCategoria(id: number, dto: SalvarCategoria): Observable<Categoria> {
    return this.http.put<Categoria>(`${this.base}/categorias/${id}`, dto);
  }
  excluirCategoria(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/categorias/${id}`); }
  regras(): Observable<Regra[]> { return this.http.get<Regra[]>(`${this.base}/regras`); }
  criarRegra(dto: SalvarRegra): Observable<Regra> { return this.http.post<Regra>(`${this.base}/regras`, dto); }
  atualizarRegra(id: number, dto: SalvarRegra): Observable<Regra> { return this.http.put<Regra>(`${this.base}/regras/${id}`, dto); }
  excluirRegra(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/regras/${id}`); }
  reaplicarRegras(): Observable<ResultadoAplicacao> { return this.http.post<ResultadoAplicacao>(`${this.base}/regras/reaplicar`, {}); }

  // Orçamentos
  orcamentos(ano: number, mes: number): Observable<Orcamento[]> {
    return this.http.get<Orcamento[]>(`${this.base}/orcamentos`, { params: this.params({ ano, mes }) });
  }
  salvarOrcamento(categoriaId: number, ano: number, mes: number, valor: number): Observable<void> {
    return this.http.put<void>(`${this.base}/orcamentos`, { categoriaId, ano, mes, valor });
  }
  excluirOrcamento(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/orcamentos/${id}`); }
  copiarOrcamentos(ano: number, mes: number): Observable<ResultadoAplicacao> {
    return this.http.post<ResultadoAplicacao>(`${this.base}/orcamentos/copiar-mes-anterior`, { ano, mes });
  }
  sugestaoOrcamentos(ano: number, mes: number): Observable<SugestaoOrcamento> {
    return this.http.get<SugestaoOrcamento>(`${this.base}/orcamentos/sugestao`, { params: this.params({ ano, mes }) });
  }
  aplicarSugestaoOrcamentos(ano: number, mes: number, itens: { categoriaId: number; valor: number }[]): Observable<ResultadoAplicacao> {
    return this.http.post<ResultadoAplicacao>(`${this.base}/orcamentos/aplicar-sugestao`, { ano, mes, itens });
  }

  // Metas
  metas(): Observable<Meta[]> { return this.http.get<Meta[]>(`${this.base}/metas`); }
  reservas(): Observable<Reserva[]> { return this.http.get<Reserva[]>(`${this.base}/reservas`); }
  planoMetas(): Observable<PlanoMetas> { return this.http.get<PlanoMetas>(`${this.base}/metas/plano`); }
  ordenarMetas(ids: number[]): Observable<void> { return this.http.put<void>(`${this.base}/metas/prioridades`, { ids }); }
  criarMeta(dto: SalvarMeta): Observable<Meta> { return this.http.post<Meta>(`${this.base}/metas`, dto); }
  atualizarMeta(id: number, dto: SalvarMeta): Observable<Meta> { return this.http.put<Meta>(`${this.base}/metas/${id}`, dto); }
  aporteMeta(id: number, valor: number): Observable<Meta> { return this.http.post<Meta>(`${this.base}/metas/${id}/aporte`, { valor }); }
  excluirMeta(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/metas/${id}`); }

  // Contas a pagar
  contasAgendadas(): Observable<ContaAgendada[]> { return this.http.get<ContaAgendada[]>(`${this.base}/contas-agendadas`); }
  sugestoesContasAgendadas(): Observable<SugestaoContaAgendada[]> {
    return this.http.get<SugestaoContaAgendada[]>(`${this.base}/contas-agendadas/sugestoes`);
  }
  criarContaAgendada(dto: SalvarContaAgendada): Observable<void> { return this.http.post<void>(`${this.base}/contas-agendadas`, dto); }
  atualizarContaAgendada(id: number, dto: SalvarContaAgendada): Observable<void> {
    return this.http.put<void>(`${this.base}/contas-agendadas/${id}`, dto);
  }
  excluirContaAgendada(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/contas-agendadas/${id}`); }
  pagarContaAgendada(id: number, lancarTransacao: boolean, data: string | null): Observable<void> {
    return this.http.post<void>(`${this.base}/contas-agendadas/${id}/pagar`, { lancarTransacao, data });
  }

  // Investimentos
  investimentos(): Observable<Investimento[]> { return this.http.get<Investimento[]>(`${this.base}/investimentos`); }
  emprestimos(): Observable<PainelEmprestimos> { return this.http.get<PainelEmprestimos>(`${this.base}/emprestimos`); }

  // Carteira (renda variável + comparação com a Selic)
  moedasCambio(): Observable<MoedaCambio[]> { return this.http.get<MoedaCambio[]>(`${this.base}/cambio/moedas`); }
  cotacaoCambio(moeda: string, data: string): Observable<CotacaoMoeda> {
    return this.http.get<CotacaoMoeda>(`${this.base}/cambio/cotacao`, { params: this.params({ moeda, data }) });
  }

  carteira(): Observable<Carteira> { return this.http.get<Carteira>(`${this.base}/carteira`); }
  atualizarCotacoes(ativoId?: number): Observable<ResultadoCotacoes> {
    return this.http.post<ResultadoCotacoes>(`${this.base}/carteira/atualizar-cotacoes`, {}, { params: this.params({ ativoId }) });
  }
  operacoesAtivo(ativoId: number): Observable<OperacaoAtivo[]> {
    return this.http.get<OperacaoAtivo[]>(`${this.base}/carteira/ativos/${ativoId}/operacoes`);
  }
  criarOperacao(dto: SalvarOperacao): Observable<{ id: number; ticker: string; ativoId: number }> {
    return this.http.post<{ id: number; ticker: string; ativoId: number }>(`${this.base}/carteira/operacoes`, dto);
  }
  atualizarOperacao(id: number, dto: SalvarOperacao): Observable<void> {
    return this.http.put<void>(`${this.base}/carteira/operacoes/${id}`, dto);
  }
  excluirOperacao(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/carteira/operacoes/${id}`); }
  atualizarAtivo(id: number, dto: AtualizarAtivo): Observable<void> {
    return this.http.put<void>(`${this.base}/carteira/ativos/${id}`, dto);
  }
  excluirAtivo(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/carteira/ativos/${id}`); }
  importarOpenFinance(): Observable<ResultadoImportacaoCarteira> {
    return this.http.post<ResultadoImportacaoCarteira>(`${this.base}/carteira/importar-open-finance`, {});
  }
  conferirReserva(contaId: number, nome: string, saldo: number, data: string): Observable<void> {
    return this.http.put<void>(`${this.base}/reservas/conferencia`, { contaId, nome, saldo, data });
  }
  removerConferenciaReserva(contaId: number, nome: string): Observable<void> {
    return this.http.delete<void>(`${this.base}/reservas/conferencia`, { params: this.params({ contaId, nome }) });
  }
  fiis(): Observable<PainelFiis> { return this.http.get<PainelFiis>(`${this.base}/carteira/fiis`); }
  sugestoesExtrato(): Observable<SugestaoExtrato[]> {
    return this.http.get<SugestaoExtrato[]>(`${this.base}/carteira/extrato/sugestoes`);
  }
  importarExtrato(itens: ImportarExtratoItem[]): Observable<ResultadoAplicacao> {
    return this.http.post<ResultadoAplicacao>(`${this.base}/carteira/extrato/importar`, { itens });
  }
  configuracaoCotacao(): Observable<{ tokenBrapiConfigurado: boolean }> {
    return this.http.get<{ tokenBrapiConfigurado: boolean }>(`${this.base}/carteira/configuracao`);
  }
  salvarConfiguracaoCotacao(tokenBrapi: string | null, remover: boolean): Observable<void> {
    return this.http.put<void>(`${this.base}/carteira/configuracao`, { tokenBrapi, remover });
  }

  // Open Finance
  parametros(): Observable<Parametro[]> { return this.http.get<Parametro[]>(`${this.base}/parametros`); }
  salvarParametros(valores: Record<string, number>): Observable<Parametro[]> {
    return this.http.put<Parametro[]>(`${this.base}/parametros`, { valores });
  }
  infoSistema(): Observable<InfoSistema> { return this.http.get<InfoSistema>(`${this.base}/parametros/sistema`); }

  configuracaoPluggy(): Observable<ConfiguracaoPluggy> {
    return this.http.get<ConfiguracaoPluggy>(`${this.base}/open-finance/configuracao`);
  }
  salvarConfiguracaoPluggy(dto: SalvarConfiguracaoPluggy): Observable<void> {
    return this.http.put<void>(`${this.base}/open-finance/configuracao`, dto);
  }
  removerSecretPluggy(): Observable<void> { return this.http.delete<void>(`${this.base}/open-finance/configuracao/secret`); }
  testarPluggy(): Observable<Mensagem> { return this.http.post<Mensagem>(`${this.base}/open-finance/testar`, {}); }
  conexoes(): Observable<Conexao[]> { return this.http.get<Conexao[]>(`${this.base}/open-finance/conexoes`); }
  adicionarConexao(itemId: string): Observable<ResultadoSincronizacao> {
    return this.http.post<ResultadoSincronizacao>(`${this.base}/open-finance/conexoes`, { itemId });
  }
  alterarConexaoAtiva(id: number, ativa: boolean): Observable<void> {
    return this.http.put<void>(`${this.base}/open-finance/conexoes/${id}/ativa`, {}, { params: this.params({ ativa }) });
  }
  removerConexao(id: number): Observable<void> { return this.http.delete<void>(`${this.base}/open-finance/conexoes/${id}`); }
  sincronizar(conexaoId?: number): Observable<ResultadoSincronizacao> {
    return this.http.post<ResultadoSincronizacao>(`${this.base}/open-finance/sincronizar`, {},
      { params: this.params({ conexaoId }) });
  }
  statusSincronizacao(): Observable<StatusSincronizacao> {
    return this.http.get<StatusSincronizacao>(`${this.base}/open-finance/status`);
  }
  logsSincronizacao(): Observable<SincronizacaoLog[]> {
    return this.http.get<SincronizacaoLog[]>(`${this.base}/open-finance/logs`);
  }
  categoriasPluggy(): Observable<CategoriaPluggy[]> {
    return this.http.get<CategoriaPluggy[]>(`${this.base}/open-finance/categorias`);
  }
  mapearCategoriaPluggy(pluggyCategoriaId: string, categoriaId: number | null, aplicarExistentes: boolean): Observable<ResultadoAplicacao> {
    return this.http.put<ResultadoAplicacao>(
      `${this.base}/open-finance/categorias/${encodeURIComponent(pluggyCategoriaId)}`, { categoriaId, aplicarExistentes });
  }

  // Segurança
  alterarSenha(senhaAtual: string, novaSenha: string): Observable<void> {
    return this.http.post<void>(`${this.base}/auth/alterar-senha`, { senhaAtual, novaSenha });
  }
  auditoria(): Observable<Auditoria[]> { return this.http.get<Auditoria[]>(`${this.base}/auth/auditoria`); }

  /* ----- Erros do sistema (#11) ----- */
  erros(filtro: FiltroErros): Observable<PaginaErros> {
    return this.http.get<PaginaErros>(`${this.base}/erros`, { params: this.params(filtro) });
  }
  erroDetalhe(id: number): Observable<ErroSistemaDetalhe> { return this.http.get<ErroSistemaDetalhe>(`${this.base}/erros/${id}`); }
  situacaoErro(id: number, resolvido: boolean): Observable<void> {
    return this.http.put<void>(`${this.base}/erros/${id}/situacao`, { resolvido });
  }
  /** Marca como resolvidos todos os em aberto do filtro (todas as páginas). */
  resolverErros(filtro: FiltroErros): Observable<{ quantidade: number }> {
    const { pagina: _p, tamanhoPagina: _t, ...resto } = filtro;
    return this.http.post<{ quantidade: number }>(`${this.base}/erros/resolver`, {}, { params: this.params(resto) });
  }
  registrarErroFrontEnd(dto: ErroFrontEnd): Observable<void> { return this.http.post<void>(`${this.base}/erros/front-end`, dto); }
}
