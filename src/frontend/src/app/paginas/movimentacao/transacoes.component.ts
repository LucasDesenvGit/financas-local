import { DatePipe } from '@angular/common';
import { Component, ElementRef, HostListener, Injector, OnInit, afterNextRender, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { FormsModule } from '@angular/forms';
import { PrivacidadeService } from '../../core/privacidade.service';
import { ActivatedRoute, Router } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ApiService } from '../../core/api.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import {
  Categoria, Conta, FiltroTransacoes, OpcaoCategoria, OrigemConta, PaginaTransacoes, ResultadoCategoriasEmLote, StatusTransacao,
  TipoConta, TipoCorrespondencia, TipoLancamento, Transacao, TipoTransacao, OrdemTransacoes
} from '../../core/modelos';
import { AvisosService } from '../../core/avisos.service';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { PaginacaoComponent } from '../../compartilhado/ui/paginacao.component';
import { OpcaoMultipla, SeletorMultiploComponent } from '../../compartilhado/ui/seletor-multiplo.component';
import { OrigemIdsUrl, dataIso } from '../../compartilhado/utilidades';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { IconeComponent } from '../../compartilhado/ui/icone.component';
import { LogoBancoComponent } from '../../compartilhado/bancos/logo-banco.component';
import { GRUPOS_LANCAMENTO, TIPOS_LANCAMENTO } from '../../compartilhado/lancamentos/tipos-lancamento';
import { RenomearTransacaoComponent } from './renomear-transacao.component';
import { RenomearEmLoteComponent } from './renomear-em-lote.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { SeloServicoComponent } from '../../compartilhado/bancos/selo-servico.component';
import { TransacoesLancamentoComponent } from './transacoes-lancamento.component';
import { TransacoesTransferenciaComponent } from './transacoes-transferencia.component';
import { TransacoesBarraEdicaoComponent } from './transacoes-barra-edicao.component';
import { TransacoesBarraLoteComponent } from './transacoes-barra-lote.component';
import {
  COLUNAS_ORDENAVEIS, ChipFiltro, ParametrosUrl, EscolhaCategoria, FormTransacao, FormTransferencia, LinhaSugestao, SugestaoRegra,
  ajusteDoForm, alternarPagina, ariaSortDe, camposDeCategorias, categoriasRotuladas, categoriasDoFiltro, chipsDoFiltro, comEscolha, direcaoAoClicar,
  direcaoDe, filtroInicial, foraDosGastos, formDaTransacao, formNovo, itensDoLote, lerFiltroDaUrl, linhasDeSugestao,
  marcarIntervalo, nomeConfianca, nomeSelecao, opcoesCategoria, ordemDe, paginaDaUrl, parametrosDoFiltro, regrasSugeridas,
  rotuloCategoria, rotuloCategoriaPorId, rotuloPeriodo, semOChip, setaDe, somandoNosTotais, tamanhoPaginaValido, textoSelecao,
  conflitoRegra, transacaoDoForm, trocouFiltro, ultimaPagina, valorCategoria
} from './transacoes';

/**
 * Tela de Transações. A lógica (filtro ↔ URL, chips, ordenação, seleção, modo edição, sugestões, formulário) está em
 * funções puras em `./transacoes.ts` (#475, ADR 0059); aqui ficam os dados, o estado e as chamadas da API.
 */
@Component({
  selector: 'app-transacoes',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, DatePipe, ModalComponent, PaginacaoComponent, SeletorMultiploComponent, TermoComponent,
    IconeComponent, LogoBancoComponent, RenomearTransacaoComponent, RenomearEmLoteComponent, SeloServicoComponent,
    CampoDataComponent, TransacoesLancamentoComponent, TransacoesTransferenciaComponent,
    TransacoesBarraEdicaoComponent, TransacoesBarraLoteComponent],
  templateUrl: './transacoes.component.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  styleUrl: './transacoes.component.css'
})
export class TransacoesComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly elemento = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly TipoTransacao = TipoTransacao;
  readonly StatusTransacao = StatusTransacao;

  /**
   * #141: transação que veio no link (?transacao=id), destacada na lista. Vale só para o filtro e a página com que
   * a tela abriu: trocar qualquer um tira o destaque (e o parâmetro da URL).
   */
  readonly transacaoDestacada = signal<number | null>(null);
  /** #173 d): de que tela vieram os ?ids= (o chip diz "Rendimentos de FIIs: 2 lançamentos"). */
  readonly origemIds = signal<OrigemIdsUrl | null>(null);
  private filtroDoDestaque: string | null = null;
  /** Rolar até a linha e pôr o foco nela só na primeira vez, não a cada recarga (ex.: depois de editar). */
  private destaqueMostrado = false;

  readonly contas = signal<Conta[]>([]);
  /** Todas as categorias (inclusive inativas), para validar a categoria que chega pela URL. */
  readonly todasCategorias = signal<Categoria[]>([]);
  /** Categorias oferecidas nos selects: as ativas e, se for o caso, as inativas que estão no filtro. */
  readonly categorias = computed(() =>
    this.todasCategorias().filter((c) => c.ativa || this.categoriasFiltradas().includes(String(c.id))));
  private readonly categoriasFiltradas = signal<string[]>([]);
  /** Opções do filtro de categorias: "Sem categoria" e cada categoria seguida das subcategorias (recuadas). */
  readonly opcoesCategoria = computed<OpcaoMultipla[]>(() => opcoesCategoria(this.categorias(), this.todasCategorias()));
  /** As categorias oferecidas com o rótulo pronto ("Lazer › Cinema"), para os componentes de apresentação. */
  readonly categoriasRotuladas = computed(() => categoriasRotuladas(this.categorias(), this.todasCategorias()));
  readonly pagina = signal<PaginaTransacoes | null>(null);
  readonly selecionadas = signal<Set<number>>(new Set());
  readonly salvando = signal(false);
  readonly sugestoesForm = signal<OpcaoCategoria[]>([]);
  readonly sugestoesAbertas = signal(false);
  readonly carregandoSugestoes = signal(false);
  readonly somenteSemCategoria = signal(true);
  readonly linhasSugestao = signal<LinhaSugestao[]>([]);
  /** #254: regras criadas nesta janela de sugestões (texto em maiúsculas → categoria), para não criar outra igual. */
  private readonly regrasCriadas = new Map<string, number>();
  private sugestoesAlteraram = false;

  readonly contasManuais = computed(() => this.contas().filter((c) => c.origem === OrigemConta.Manual));
  readonly totalPaginas = computed(() => {
    const p = this.pagina();
    return p ? ultimaPagina(p.total, p.tamanhoPagina) : 1;
  });
  readonly todasSelecionadas = computed(() => {
    const itens = this.pagina()?.itens ?? [];
    return itens.length > 0 && itens.every((t) => this.selecionadas().has(t.id));
  });
  /** #133: quantas das selecionadas estão nesta página (o resto está em outras: a seleção continua ao trocar). */
  private readonly selecionadasNaPagina = computed(() =>
    (this.pagina()?.itens ?? []).filter((t) => this.selecionadas().has(t.id)).length);
  readonly algumaSelecionadaNaPagina = computed(() => this.selecionadasNaPagina() > 0);
  readonly textoSelecao = computed(() => textoSelecao(this.selecionadas().size, this.selecionadasNaPagina()));
  /** Shift+clique: a última caixa marcada/desmarcada (índice na página) e se o clique veio com Shift. */
  private ultimoMarcado: number | null = null;
  shiftNoClique = false;

  /* ------------------------------ #87: modo edição ------------------------------ */
  /** Modo edição de categorias (?editar=1). */
  readonly modoEdicao = signal(false);
  /** Trocas de categoria ainda não gravadas: id da transação → escolha. */
  readonly pendentes = signal<Map<number, EscolhaCategoria>>(new Map());
  readonly quantidadePendentes = computed(() => this.pendentes().size);
  readonly idsCategoriasOferecidas = computed(() => new Set(this.categorias().map((c) => c.id)));
  readonly sugestoesRegra = signal<SugestaoRegra[]>([]);
  readonly soSemCategoriaAtivo = computed(() => {
    const f = this.filtroAplicado();
    return !!f.semCategoria && !(f.categoriaIds ?? []).length;
  });
  /** Escolha para "todas do filtro" (null = nada escolhido). */
  categoriaTodas: EscolhaCategoria | null = null;

  filtro: FiltroTransacoes = this.filtroInicial();
  /** Categorias marcadas no filtro: 'sem' ou o id, na ordem em que foram marcadas. */
  filtroCategorias: string[] = [];
  /** Filtro da última busca (o que está na tela), base dos chips de "Filtrando:". */
  readonly filtroAplicado = signal<FiltroTransacoes>(this.filtroInicial());

  /** UX-2: um chip para cada filtro diferente do padrão (`chipsDoFiltro`). */
  readonly chipsFiltro = computed<ChipFiltro[]>(() =>
    chipsDoFiltro(this.filtroAplicado(), this.filtroInicial(), this.todasCategorias(), this.contas(), this.origemIds()));

  removerFiltro(id: ChipFiltro['id']): void {
    const r = semOChip(this.filtro, this.filtroCategorias, id, this.filtroInicial());
    this.filtro = r.filtro;
    if (r.categorias !== this.filtroCategorias) {
      this.filtroCategorias = r.categorias;
      this.sincronizarFiltroCategoria();
    }
    if (id === 'ids') this.origemIds.set(null);
    this.buscar(1);
  }

  limparFiltros(): void {
    this.filtro = this.filtroInicial();
    this.origemIds.set(null);
    this.filtroCategorias = [];
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }
  categoriaLote: number | null = null;
  form: FormTransacao | null = null;
  transferencia: FormTransferencia | null = null;

  ngOnInit(): void {
    const params = this.rota.snapshot.queryParamMap;
    // Categoria/conta da URL só são aplicadas depois de conferir que existem; por isso, quando vierem,
    // a primeira busca espera as listas (uma requisição só, sem a lista piscar).
    const precisaListas = params.has('categoria') || params.has('conta');
    forkJoin({ contas: this.api.contas(), categorias: this.api.categorias() }).subscribe({
      next: ({ contas, categorias }) => {
        this.contas.set(contas);
        this.todasCategorias.set(categorias);
        if (precisaListas) {
          this.aplicarParametrosDaUrl(params);
          this.buscar(paginaDaUrl(params));
        }
      },
      error: (e) => {
        this.avisos.erro(mensagemDeErro(e));
        if (precisaListas) this.buscar(paginaDaUrl(params));
      }
    });
    if (!precisaListas) {
      this.aplicarParametrosDaUrl(params);
      this.buscar(paginaDaUrl(params));
    }
  }

  /** UX-1 e UX-5: o filtro, o modo edição, a origem dos ids e o destaque que vêm na URL (`lerFiltroDaUrl`). */
  private aplicarParametrosDaUrl(params: ParametrosUrl): void {
    const lido = lerFiltroDaUrl(params, this.filtro, this.todasCategorias(), this.contas());
    this.filtro = lido.filtro;
    this.filtroCategorias = lido.categorias;
    this.sincronizarFiltroCategoria();
    if (lido.modoEdicao) this.modoEdicao.set(true);
    if (lido.origemIds) this.origemIds.set(lido.origemIds);
    if (lido.transacaoDestacada !== null) this.transacaoDestacada.set(lido.transacaoDestacada);
  }

  rotuloCategoria(c: Categoria): string {
    return rotuloCategoria(c, this.todasCategorias());
  }

  aplicarFiltroCategorias(valores: string[]): void {
    this.filtroCategorias = valores;
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }

  /** Leva as categorias marcadas ('sem' | id) para o filtro da API. */
  private sincronizarFiltroCategoria(): void {
    Object.assign(this.filtro, camposDeCategorias(this.filtroCategorias));
    this.categoriasFiltradas.set(this.filtroCategorias);
  }

  buscar(pagina: number): void {
    // #87: trocar de página ou de filtro perde as trocas não salvas; pergunta antes (Nielsen 5). Sem pendência, a busca
    // continua na hora (síncrona), como antes do #631.
    if (this.pendentes().size) {
      void this.confirmacao.confirmar({ titulo: 'Descartar alterações', mensagem: `Descartar ${this.pendentes().size} alteração(ões) de categoria não salva(s)?`, confirmar: 'Descartar', perigo: true })
        .then((ok) => {
          if (!ok) {
            this.voltarAoFiltroAplicado();
            return;
          }
          this.pendentes.set(new Map());
          this.executarBusca(pagina);
        });
      return;
    }
    this.executarBusca(pagina);
  }

  private executarBusca(pagina: number): void {
    this.filtro.pagina = pagina;
    if (this.transacaoDestacada() !== null) {
      const chave = JSON.stringify(this.filtro);
      this.filtroDoDestaque ??= chave;
      if (chave !== this.filtroDoDestaque) this.transacaoDestacada.set(null);
    }
    // #133: trocar só de página mantém a seleção; trocar o filtro limpa (`trocouFiltro`).
    if (trocouFiltro(this.filtroAplicado(), this.filtro) && this.selecionadas().size) {
      this.limparSelecao();
      this.avisos.info('Seleção limpa ao trocar o filtro.');
    }
    this.filtroAplicado.set({ ...this.filtro });
    this.gravarFiltroNaUrl();
    this.api.transacoes(this.filtro).subscribe({
      next: (p) => {
        // Página da URL que não existe mais (link antigo, menos transações agora): vai para a última que existe.
        const ultima = ultimaPagina(p.total, p.tamanhoPagina);
        if (!p.itens.length && p.pagina > ultima) {
          this.buscar(ultima);
          return;
        }
        const trocouPagina = this.pagina() !== null && this.pagina()!.pagina !== p.pagina;
        this.pagina.set(p);
        this.ultimoMarcado = null;   // o intervalo do Shift vale dentro de uma página
        if (trocouPagina) this.rolarParaTopo();
        this.mostrarDestaque(p);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /**
   * UX-5: o filtro da busca vai para a URL (só o que difere do padrão), para o F5 e o link salvo manterem a lista.
   * replaceUrl: o voltar do navegador leva à tela anterior, não ao filtro anterior (julgamento próprio; Nielsen 4).
   */
  private gravarFiltroNaUrl(): void {
    const params = parametrosDoFiltro(this.filtro, this.filtroInicial(), {
      categorias: this.filtroCategorias, transacaoDestacada: this.transacaoDestacada(), origemIds: this.origemIds(),
      modoEdicao: this.modoEdicao()
    });
    this.router.navigate([], { relativeTo: this.rota, queryParams: params, replaceUrl: true });
  }

  /* ------------------------------ #244: ordenar pelo cabeçalho ------------------------------ */

  readonly colunasOrdenaveis = COLUNAS_ORDENAVEIS;

  ordemAtual(): OrdemTransacoes {
    return ordemDe(this.filtroAplicado());
  }

  direcaoAtual(): 'asc' | 'desc' {
    return direcaoDe(this.filtroAplicado());
  }

  ariaSort(coluna: OrdemTransacoes): 'ascending' | 'descending' | null {
    return ariaSortDe(this.filtroAplicado(), coluna);
  }

  seta(coluna: OrdemTransacoes): string {
    return setaDe(this.filtroAplicado(), coluna);
  }

  /** Clicar na coluna ordena por ela; clicar de novo inverte (`direcaoAoClicar`). Volta para a página 1. */
  ordenar(coluna: OrdemTransacoes): void {
    this.filtro.direcao = direcaoAoClicar(this.filtroAplicado(), coluna);
    this.filtro.ordem = coluna;
    this.buscar(1);
  }

  mudarTamanhoPagina(tamanho: number): void {
    this.filtro.tamanhoPagina = tamanho;
    try { localStorage.setItem('financas.tamanhoPagina', String(tamanho)); } catch { /* ignora */ }
    this.buscar(1);
  }

  /** Volta ao topo da lista ao trocar de página. */
  private rolarParaTopo(): void {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  /**
   * #141: rola até a transação do link e põe o foco nela (teclado e leitor de tela caem direto nela). Se ela não
   * está na lista, avisa e tira o destaque, para o F5 não repetir o aviso.
   */
  private mostrarDestaque(p: PaginaTransacoes): void {
    const id = this.transacaoDestacada();
    if (id === null || this.destaqueMostrado) return;
    this.destaqueMostrado = true;
    if (!p.itens.some((t) => t.id === id)) {
      this.transacaoDestacada.set(null);
      this.gravarFiltroNaUrl();
      this.avisos.info('A transação do link não aparece neste filtro: ela pode ter sido apagada ou mudado de data ou de conta.');
      return;
    }
    afterNextRender(() => {
      const linha = this.elemento.nativeElement.querySelector<HTMLElement>(`tr[data-transacao="${id}"]`);
      if (!linha) return;
      const semAnimacao = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
      linha.scrollIntoView({ block: 'center', behavior: semAnimacao ? 'auto' : 'smooth' });
      linha.focus({ preventScroll: true });
    }, { injector: this.injector });
  }

  alternar(id: number): void {
    this.selecionadas.update((s) => {
      const novo = new Set(s);
      if (novo.has(id)) novo.delete(id); else novo.add(id);
      return novo;
    });
  }

  /** #133: marca/desmarca a caixa da linha `indice`; com Shift (clique ou Shift+Espaço), o intervalo (`marcarIntervalo`). */
  marcar(indice: number, e: MouseEvent): void {
    const comShift = e.shiftKey || this.shiftNoClique;
    this.shiftNoClique = false;
    const marcar = (e.target as HTMLInputElement).checked;
    this.selecionadas.update((s) => marcarIntervalo(s, this.pagina()?.itens ?? [], indice, this.ultimoMarcado, comShift, marcar));
    this.ultimoMarcado = indice;
  }

  /** "Selecionar todas desta página" (`alternarPagina`). */
  alternarTodas(): void {
    this.selecionadas.update((s) => alternarPagina(s, this.pagina()?.itens ?? []));
    this.ultimoMarcado = null;
  }

  limparSelecao(): void {
    this.selecionadas.set(new Set());
    this.ultimoMarcado = null;
  }

  nomeSelecao(t: Transacao): string {
    return nomeSelecao(t, this.privacidade.oculto());
  }

  categorizarLote(): void {
    if (this.modoEdicao()) {
      // No modo edição a barra de seleção só marca a troca; grava junto com as outras em "Salvar alterações".
      const escolha: EscolhaCategoria = this.categoriaLote === null ? 'auto' : `${this.categoriaLote}`;
      const itens = (this.pagina()?.itens ?? []).filter((t) => this.selecionadas().has(t.id));
      for (const t of itens) this.mudarCategoria(t, escolha);
      this.avisos.info(`${itens.length} troca(s) marcada(s). Salve para gravar.`);
      this.limparSelecao();
      return;
    }
    this.api.categorizarEmLote([...this.selecionadas()], this.categoriaLote).subscribe({
      next: (r) => {
        this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) atualizada(s).`);
        this.limparSelecao();
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  marcarEmergenciaLote(emergencia: boolean): void {
    this.api.marcarEmergencia([...this.selecionadas()], emergencia).subscribe({
      next: (r) => {
        this.avisos.sucesso(emergencia
          ? `${r.transacoesAtualizadas} despesa(s) marcada(s) como emergência.`
          : `${r.transacoesAtualizadas} transação(ões) desmarcada(s).`);
        this.limparSelecao();
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  marcarRevisadasSelecionadas(): void {
    this.marcarRevisadas([...this.selecionadas()]);
  }

  /* ------------------------------ #111: tipo de lançamento ------------------------------ */
  readonly gruposLancamento = GRUPOS_LANCAMENTO;

  rotuloLancamento(t: Transacao): string {
    return TIPOS_LANCAMENTO[t.tipoLancamento]?.rotulo ?? 'Outro';
  }

  iconeLancamento(t: Transacao) {
    return (TIPOS_LANCAMENTO[t.tipoLancamento] ?? TIPOS_LANCAMENTO[TipoLancamento.Outro]).icone;
  }

  /** #123: transação na janela "Renomear" (null = fechada). */
  readonly renomeando = signal<Transacao | null>(null);

  aoRenomear(): void {
    this.renomeando.set(null);
    this.buscar(this.filtro.pagina ?? 1);
  }

  /** #123: o mesmo nome nas selecionadas (as desta página). */
  readonly renomeandoLote = signal<Transacao[] | null>(null);

  renomearSelecionadas(): void {
    const selecionadas = (this.pagina()?.itens ?? []).filter((t) => this.selecionadas().has(t.id));
    if (selecionadas.length) this.renomeandoLote.set(selecionadas);
  }

  aoRenomearLote(): void {
    this.renomeandoLote.set(null);
    this.limparSelecao();
    this.buscar(this.filtro.pagina ?? 1);
  }

  readonly foraDosGastos = foraDosGastos;
  readonly somandoNosTotais = somandoNosTotais;

  contaDe(t: Transacao): Conta | undefined {
    return this.contas().find((c) => c.id === t.contaId);
  }

  ehDeCartao(t: Transacao): boolean {
    return this.contaDe(t)?.tipo === TipoConta.CartaoCredito;
  }

  /** #9: revisada sai do filtro "Descrição pouco informativa"; o aviso permite desfazer. */
  marcarRevisadas(ids: number[]): void {
    if (!ids.length) return;
    this.api.marcarRevisada(ids, true).subscribe({
      next: (r) => {
        this.limparSelecao();
        this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) marcada(s) como revisada(s).`, {
          rotulo: 'Desfazer',
          executar: () => this.api.marcarRevisada(ids, false).subscribe({
            next: () => this.buscar(this.filtro.pagina ?? 1),
            error: (e) => this.avisos.erro(mensagemDeErro(e))
          })
        });
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ #87: modo edição ------------------------------ */

  async alternarModoEdicao(): Promise<void> {
    if (this.modoEdicao() && this.pendentes().size
      && !(await this.confirmacao.confirmar({ titulo: 'Sair da edição', mensagem: `Sair da edição e descartar ${this.pendentes().size} alteração(ões) não salva(s)?`, confirmar: 'Sair e descartar', perigo: true }))) return;
    this.pendentes.set(new Map());
    this.sugestoesRegra.set([]);
    this.categoriaTodas = null;
    this.modoEdicao.update((v) => !v);
    this.gravarFiltroNaUrl();
  }

  valorCategoria(t: Transacao): string {
    return valorCategoria(t, this.pendentes());
  }

  mudarCategoria(t: Transacao, valor: string): void {
    this.pendentes.update((m) => comEscolha(m, t, valor));
  }

  desfazerLinha(t: Transacao): void {
    this.pendentes.update((m) => {
      const novo = new Map(m);
      novo.delete(t.id);
      return novo;
    });
  }

  async descartarPendentes(): Promise<void> {
    if (!this.pendentes().size || !(await this.confirmacao.confirmar({ titulo: 'Descartar alterações', mensagem: `Descartar ${this.pendentes().size} alteração(ões)?`, confirmar: 'Descartar', perigo: true }))) return;
    this.pendentes.set(new Map());
  }

  @HostListener('window:beforeunload', ['$event'])
  avisarAntesDeSair(evento: BeforeUnloadEvent): void {
    if (this.pendentes().size) evento.preventDefault();
  }

  soSemCategoria(): void {
    this.filtroCategorias = ['sem'];
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }

  salvarPendentes(): void {
    const pendentes = this.pendentes();
    if (!pendentes.size) return;
    const sugestoes = regrasSugeridas(this.pagina()?.itens ?? [], pendentes, this.todasCategorias());
    this.salvando.set(true);
    this.api.alterarCategoriasEmLote({ itens: itensDoLote(pendentes) }).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.pendentes.set(new Map());
        this.sugestoesRegra.set(sugestoes);
        this.avisarResultado(r);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** "Todas as N do filtro", inclusive as das outras páginas, com confirmação (Nielsen 5). */
  async aplicarATodasDoFiltro(): Promise<void> {
    const escolha = this.categoriaTodas;
    const total = this.pagina()?.total ?? 0;
    if (!escolha || !total) return;
    const f = this.filtroAplicado();
    const nome = escolha === 'auto' ? 'a categorização automática'
      : `“${rotuloCategoriaPorId(Number(escolha), this.todasCategorias())}”`;
    if (!(await this.confirmacao.confirmar({ titulo: 'Trocar a categoria', mensagem: `Trocar a categoria das ${total} transações do filtro (${rotuloPeriodo(f.inicio, f.fim)}) para ${nome}?`, confirmar: 'Trocar a categoria' }))) return;
    this.salvando.set(true);
    this.api.alterarCategoriasEmLote({
      filtro: { ...f, pagina: 1 }, categoriaId: escolha === 'auto' ? null : Number(escolha), automatica: escolha === 'auto'
    }).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.categoriaTodas = null;
        this.pendentes.set(new Map());
        this.avisarResultado(r);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** Aviso do que mudou, com "Desfazer" (a API guarda o estado anterior por alguns minutos). */
  private avisarResultado(r: ResultadoCategoriasEmLote): void {
    if (!r.alteradas || !r.desfazer) {
      this.avisos.info('Nada mudou: as transações já estavam com essa categoria.');
      return;
    }
    const chave = r.desfazer;
    this.avisos.sucesso(`${r.alteradas} transação(ões) atualizada(s).`, {
      rotulo: 'Desfazer',
      executar: () => this.api.desfazerCategoriasEmLote(chave).subscribe({
        next: (d) => {
          this.sugestoesRegra.set([]);
          this.avisos.sucesso(`Desfeito: ${d.transacoesAtualizadas} transação(ões) voltaram como estavam.`);
          this.buscar(this.filtro.pagina ?? 1);
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      })
    });
  }

  criarRegraSugerida(r: SugestaoRegra): void {
    this.api.criarRegra({
      texto: r.texto, tipoCorrespondencia: TipoCorrespondencia.Contem, categoriaId: r.categoriaId, prioridade: 100, ativa: true
    }).subscribe({
      next: () => {
        this.dispensarRegra(r);
        this.avisos.sucesso(`Regra criada: as próximas “${r.texto}” entram em ${r.categoriaNome}.`);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  dispensarRegra(r: SugestaoRegra): void {
    this.sugestoesRegra.update((lista) => lista.filter((x) => x !== r));
  }

  /** Cancelou o descarte: os campos de filtro voltam ao que está na tela. */
  private voltarAoFiltroAplicado(): void {
    const f = this.filtroAplicado();
    this.filtro = { ...f };
    this.filtroCategorias = categoriasDoFiltro(f);
    this.categoriasFiltradas.set(this.filtroCategorias);
  }

  /* ------------------------------ sugestões de categoria ------------------------------ */

  readonly quantidadeAlta = computed(() =>
    this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.categoriaId).length);

  abrirSugestoes(): void {
    this.sugestoesAbertas.set(true);
    this.sugestoesAlteraram = false;
    this.carregarSugestoes();
  }

  trocarModoSugestoes(semCategoria: boolean): void {
    this.somenteSemCategoria.set(semCategoria);
    this.carregarSugestoes();
  }

  private carregarSugestoes(): void {
    this.carregandoSugestoes.set(true);
    this.api.sugestoesCategoria(this.somenteSemCategoria(), 365).subscribe({
      next: (s) => {
        this.carregandoSugestoes.set(false);
        this.regrasCriadas.clear();
        this.linhasSugestao.set(linhasDeSugestao(s));
      },
      error: (e) => {
        this.carregandoSugestoes.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  readonly nomeConfianca = nomeConfianca;

  aplicarSugestao(l: LinhaSugestao, silencioso = false): void {
    if (!l.escolhida) return;
    const categoriaId = l.escolhida;
    this.salvando.set(true);
    this.api.categorizarEmLote(l.transacaoIds, categoriaId).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.sugestoesAlteraram = true;
        this.linhasSugestao.update((linhas) => linhas.map((x) => (x.chave === l.chave ? { ...x, aplicada: true } : x)));
        const conflito = this.conflitoRegra({ ...l, criarRegra: true });
        if (l.criarRegra && l.textoRegra.trim() && conflito) {
          this.avisos.erro(`A regra "${l.textoRegra}" não foi criada: outra linha a usa para ${conflito}. Categorize sem regra, ou crie a regra certa em Categorias e regras.`);
        } else if (l.criarRegra && l.textoRegra.trim()) {
          this.regrasCriadas.set(l.textoRegra.trim().toUpperCase(), categoriaId);
          this.api.criarRegra({
            texto: l.textoRegra.trim(), tipoCorrespondencia: TipoCorrespondencia.Contem, categoriaId, prioridade: 100, ativa: true
          }).subscribe({ error: (e) => this.avisos.erro(`Regra "${l.textoRegra}": ${mensagemDeErro(e)}`) });
        }
        if (!silencioso) this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) categorizada(s).`);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  async aplicarConfiancaAlta(): Promise<void> {
    const linhas = this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.escolhida);
    if (!linhas.length) return;
    // #254: duas linhas com a mesma regra e categorias diferentes: categoriza as duas, mas nenhuma cria a regra.
    const semRegra = linhas.filter((l) => l.criarRegra && this.conflitoRegra(l));
    const nota = semRegra.length
      ? `\n\nSem regra (o mesmo texto com categorias diferentes): ${[...new Set(semRegra.map((l) => `"${l.textoRegra}"`))].join(', ')}.`
      : '';
    if (!(await this.confirmacao.confirmar({ titulo: 'Aplicar sugestões', mensagem: `Aplicar ${linhas.length} sugestão(ões) de confiança alta?${nota}`, confirmar: 'Aplicar' }))) return;
    for (const l of linhas) this.aplicarSugestao(semRegra.includes(l) ? { ...l, criarRegra: false } : l, true);
    this.avisos.sucesso(`${linhas.length} grupo(s) categorizado(s)` + (semRegra.length ? `; ${semRegra.length} sem regra, por conflito.` : '.'));
  }

  /** #254: a outra categoria que usa a mesma regra desta linha (`conflitoRegra`), ou null. */
  conflitoRegra(l: LinhaSugestao): string | null {
    return conflitoRegra(l, this.linhasSugestao(), this.regrasCriadas, this.categorias());
  }

  fecharSugestoes(): void {
    this.sugestoesAbertas.set(false);
    if (this.sugestoesAlteraram) this.buscar(this.filtro.pagina ?? 1);
  }

  novo(): void {
    this.form = formNovo(this.contasManuais()[0]?.id ?? null, dataIso());
    this.sugestoesForm.set([]);
  }

  editar(t: Transacao): void {
    this.form = formDaTransacao(t);
    this.sugestoesForm.set([]);
    this.api.sugestoesDaTransacao(t.id).subscribe({ next: (s) => this.sugestoesForm.set(s), error: () => { /* opcional */ } });
  }

  /** "Salvar" da janela do lançamento, com o que foi editado nela. */
  salvar(f: FormTransacao): void {
    this.salvando.set(true);

    const requisicao = f.openFinance && f.id
      ? this.api.ajustarTransacao(f.id, ajusteDoForm(f))
      : f.id ? this.api.atualizarTransacao(f.id, transacaoDoForm(f)) : this.api.criarTransacao(transacaoDoForm(f));

    requisicao.subscribe({
      next: () => {
        this.salvando.set(false);
        this.form = null;
        this.avisos.sucesso('Transação salva.');
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  async excluir(): Promise<void> {
    const f = this.form;
    if (!f?.id || !(await this.confirmacao.confirmar({ titulo: 'Excluir transação', mensagem: 'Excluir esta transação?', confirmar: 'Excluir', perigo: true }))) return;
    this.api.excluirTransacao(f.id).subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Transação excluída.');
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirTransferencia(): void {
    this.transferencia = { contaOrigemId: null, contaDestinoId: null, valor: null, data: dataIso(), descricao: '' };
  }

  /** "Transferir" da janela da transferência, com o que foi editado nela. */
  salvarTransferencia(t: FormTransferencia): void {
    if (!t.contaOrigemId || !t.contaDestinoId || !t.valor) return;
    this.salvando.set(true);
    this.api.transferir({
      contaOrigemId: t.contaOrigemId, contaDestinoId: t.contaDestinoId, valor: Number(t.valor), data: t.data,
      descricao: t.descricao || null
    }).subscribe({
      next: () => {
        this.salvando.set(false);
        this.transferencia = null;
        this.avisos.sucesso('Transferência registrada.');
        this.buscar(1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** O filtro padrão: o mês atual, com o tamanho de página guardado no navegador (ADR 0017). */
  private filtroInicial(): FiltroTransacoes {
    let guardado: string | null = null;
    try { guardado = localStorage.getItem('financas.tamanhoPagina'); } catch { /* sem armazenamento: o padrão */ }
    return filtroInicial(new Date(), tamanhoPaginaValido(guardado));
  }
}
