import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { AssinaturasRecorrentesComponent } from './assinaturas-recorrentes.component';
import { ContasAgendadasFormularioComponent } from './contas-agendadas-formulario.component';
import { ContasAgendadasPagamentoComponent } from './contas-agendadas-pagamento.component';
import { ContasAgendadasSugestoesComponent } from './contas-agendadas-sugestoes.component';
import { FormsModule } from '@angular/forms';
import { LogoBancoComponent } from '../../compartilhado/bancos/logo-banco.component';
import { SeloServicoComponent } from '../../compartilhado/bancos/selo-servico.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { dataIso } from '../../compartilhado/utilidades';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import {
  AvisoContaAgendada, Categoria, Conta, ContaAgendada, FormaCobranca, Reserva, SalvarContaAgendada, SugestaoContaAgendada
} from '../../core/modelos';
import {
  bancoDe, categoriasDeDespesa, corpoDoAviso, corpoParaSalvar, chavesIgnoradasDe, chavesParaGuardar, ehCartao, formularioDaConta,
  formularioDaSugestao, formularioVazio, nomeForma, nomeRecorrencia, separarSugestoes, textoDiasParaVencer, totalProximos30,
  totalVencido
} from './contas-agendadas';
import {
  AberturaPagamento, PagamentoParaApi, aberturaPeloAviso, baixaSozinha, cofrinhoEncontrado, mostrarPagar, textoDesfazerPagamento,
  textoLancarPeloCofrinho, textoPagamentoCofrinho
} from './pago-com-cofrinho';

@Component({
  selector: 'app-contas-agendadas',
  imports: [FormsModule, MoedaPipe, DatePipe, ModalComponent, CampoValorComponent, AssinaturasRecorrentesComponent,
    ContasAgendadasFormularioComponent, ContasAgendadasPagamentoComponent, ContasAgendadasSugestoesComponent, LogoBancoComponent,
    SeloServicoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styleUrl: './contas-agendadas-cobranca.css',
  template: `
    <header class="cabecalho-pagina">
      <h1>Contas a pagar</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova conta</button>
    </header>

    <!-- #158: avisos das recorrentes, com a ação ao lado (Nielsen 1 e 9); "não veio" e "cartão arquivado" também no Painel. -->
    @if (avisosRecorrentes().length) {
      <section class="painel-bloco avisos-recorrentes" aria-labelledby="titulo-avisos">
        <div class="bloco-titulo"><h2 id="titulo-avisos">Avisos das recorrentes</h2></div>
        <ul class="lista-avisos">
          @for (a of avisosRecorrentes(); track a.contaAgendadaId + a.tipo) {
            <li>
              <span aria-hidden="true">⚠</span> {{ a.texto }}
              @if (a.tipo === 'aumentou' && a.valorNovo !== null) {
                <span class="nowrap">({{ a.valorAnterior | currency }} → {{ a.valorNovo | currency }})</span>
                <button type="button" class="botao botao-pequeno" (click)="atualizarPeloAviso(a)"
                  [attr.aria-label]="'Atualizar o valor de ' + a.descricao">Atualizar o valor</button>
              }
              @if (a.tipo === 'mudou-de-cartao' && a.contaNovaId !== null) {
                <button type="button" class="botao botao-pequeno" (click)="atualizarPeloAviso(a)"
                  [attr.aria-label]="'Atualizar o catálogo de ' + a.descricao + ': cobra em ' + a.contaNovaNome">Atualizar o catálogo</button>
              }
              <!-- #691: a cobrança não chegou pelo extrato depois de sincronizar: pode ter saído direto de um cofrinho. -->
              @if (a.tipo === 'nao-veio' && a.podeSerCofrinho) {
                <button type="button" class="botao botao-pequeno" (click)="pagarPeloAviso(a)"
                  [attr.aria-label]="'Foi paga com um cofrinho: ' + a.descricao">Foi paga com um cofrinho</button>
              }
            </li>
          }
        </ul>
      </section>
    }

    <!-- #158: o de-para das cobranças que se repetem, como aba da mesma tela (Nielsen 4); a aba fica na URL (ADR 0018). -->
    <div class="segmentado barra-abas" role="tablist" aria-label="Contas a pagar">
      <button type="button" role="tab" [class.ativo]="aba() === 'contas'" [attr.aria-selected]="aba() === 'contas'"
        (click)="mudarAba('contas')">Contas a pagar</button>
      <button type="button" role="tab" [class.ativo]="aba() === 'recorrentes'" [attr.aria-selected]="aba() === 'recorrentes'"
        (click)="mudarAba('recorrentes')">Assinaturas e recorrentes</button>
    </div>

    @if (aba() === 'recorrentes') {
      <app-assinaturas-recorrentes [itens]="itens()" [contas]="contas()" />
    } @else {
    <section class="kpis">
      <article class="kpi"><span>Vencidas</span><strong class="negativo">{{ totalVencido() | currency }}</strong></article>
      <article class="kpi"><span>Próximos 30 dias</span><strong>{{ total30() | currency }}</strong></article>
    </section>

    <div class="tabela-rolagem">
      <table class="tabela">
        <thead>
          <tr><th>Vencimento</th><th>Descrição</th><th>Recorrência</th><th>Categoria</th><th class="direita">Valor</th><th><span class="so-leitor">Ações</span></th></tr>
        </thead>
        <tbody>
          @for (c of itens(); track c.id) {
            <tr [class.ignorada]="!c.ativa">
              <td class="nowrap">
                @if (c.encerrada) {
                  <span class="etiqueta etiqueta-suave">encerrada</span>
                } @else {
                  {{ c.proximoVencimento | date: 'dd/MM/yyyy' }}
                }
                @if (c.ativa) {
                  <br><small [class.negativo]="c.diasParaVencer < 0" class="texto-suave">
                    {{ textoDiasParaVencer(c.diasParaVencer) }}
                  </small>
                }
              </td>
              <!-- #265: selo do serviço (ou ícone da categoria) ao lado do nome da cobrança. #379: o selo numa coluna, e o
                   nome, a conta, a forma de cobrança e a observação empilhados ao lado, todos começando no mesmo ponto. -->
              <td><div class="cobranca"><app-selo-servico [texto]="c.textoCobranca" [nome]="c.descricao"
                [categoria]="c.categoriaNome" [tamanho]="24" />
                <div class="cobranca-texto">
                  <span class="nome-cobranca">{{ c.descricao }}</span>
                  <!-- #242: selo do banco ou do cartão ao lado do nome (decorativo: o nome está escrito, ADR 0024). -->
                  @if (c.contaNome) {
                    <small class="texto-suave conta-da-cobranca"><app-logo-banco [codigo]="bancoDe(c.contaId)"
                      [cartao]="ehCartao(c.contaId) || c.formaCobranca === FormaCobranca.CartaoCredito" [tamanho]="16" />{{ c.contaNome }}</small>
                  }
                  <!-- #158: como é cobrada, em texto (Nielsen 6: onde cai fica escrito na tela). -->
                  <small class="texto-suave forma-cobranca">{{ nomeForma(c.formaCobranca) }}
                    <!-- #691: de onde sai o dinheiro fica escrito (Nielsen 6). -->
                    @if (c.reservaNome) { · cofrinho {{ c.reservaNome }}@if (c.contaNome) { ({{ c.contaNome }})} }
                    @if (c.reservaNome && c.formaCobranca === FormaCobranca.DebitoAutomatico) { · o app lança a despesa no vencimento }
                    @else if (baixaSozinha(c)) { · baixa sozinha quando a cobrança com “{{ c.textoCobranca }}” chega }</small>
                  @if (!cofrinhoEncontrado(c)) {
                    <small class="texto-alerta">Cofrinho “{{ c.reservaNome }}” não encontrado nas reservas de {{ c.contaNome }}:
                      a despesa continua sendo lançada com esse nome.</small>
                  }
                  <!-- #691: pagamento dado antes de marcar o cofrinho, sem a despesa: o app oferece lançar (Nielsen 1). -->
                  @if (c.ultimoSemDespesa && c.reservaNome) {
                    <small class="texto-alerta">o pagamento de {{ c.ultimoSemDespesa | date: 'dd/MM' }} está sem a despesa
                      <button type="button" class="link" (click)="lancarUltimoPeloCofrinho(c)"
                        [attr.aria-label]="'Lançar pelo cofrinho a despesa de ' + c.descricao">lançar pelo cofrinho</button></small>
                  }
                  <!-- #691: o que o app lançou sozinho fica à vista, com saída fácil (Nielsen 1 e 3). -->
                  @if (c.ultimoPeloCofrinho; as u) {
                    <small class="texto-suave pago-cofrinho">{{ textoPagamentoCofrinho(u) }} · {{ u.valor | currency }}
                      <button type="button" class="link" (click)="abrirAjuste(c)"
                        [attr.aria-label]="'Ajustar o valor pago de ' + c.descricao">ajustar valor</button>
                      @if (u.podeDesfazer) {
                        <button type="button" class="link" (click)="desfazerPagamento(c)"
                          [attr.aria-label]="'Desfazer o pagamento de ' + c.descricao">desfazer</button>
                      }</small>
                  }
                  @if (c.observacao) { <small class="texto-suave">{{ c.observacao }}</small> }
                </div></div></td>
              <td>{{ nomeRecorrencia(c.recorrencia) }}
                @if (c.encerrada) {
                  <br><small class="texto-suave">encerrada@if (c.ultimoPagamentoEm) { em {{ c.ultimoPagamentoEm | date: 'dd/MM/yyyy' }}}</small>
                } @else if (c.ativa && c.dataLimite) {
                  <br><small class="texto-suave">até {{ c.dataLimite | date: 'MMM/yyyy' }} ·
                    {{ c.pagamentosRestantes === 1 ? 'último pagamento' : 'faltam ' + c.pagamentosRestantes }}</small>
                }
              </td>
              <td>{{ c.categoriaNome ?? '—' }}</td>
              <td class="direita valor">{{ c.valor | currency }}</td>
              <td class="direita nowrap">
                @if (mostrarPagar(c)) {
                  <button type="button" class="botao botao-pequeno" (click)="abrirPagamento(c)"
                    [attr.aria-label]="'Pagar ' + c.descricao">Pagar</button>
                }
                <button type="button" class="link" (click)="editar(c)">editar</button>
              </td>
            </tr>
          } @empty {
            <tr><td colspan="6" class="vazio">Cadastre aluguel, condomínio, escola, assinaturas… para ser lembrado no painel.</td></tr>
          }
        </tbody>
      </table>
    </div>
    <app-contas-agendadas-sugestoes [sugestoes]="separadas().visiveis" [ignoradas]="separadas().ignoradas.length" [contas]="contas()"
      (cadastrar)="cadastrarSugestao($event)" (mesma)="eAMesma($event)" (ignorar)="ignorar($event)" (restaurar)="restaurarIgnoradas()" />
    }

    <app-contas-agendadas-formulario [formulario]="form" [editando]="editandoId !== null" [contas]="contas()"
      [categorias]="categoriasDespesa()" [reservas]="reservas()" (salvar)="salvar($event)" (excluir)="excluir()"
      (fechar)="form = null" />

    <app-contas-agendadas-pagamento [abertura]="pagando" [contas]="contas()" [reservas]="reservas()"
      (confirmar)="confirmarPagamento($event)" (fechar)="pagando = null" />

    <!-- #691: "Ajustar valor" da última baixa pelo cofrinho (conta de consumo que veio diferente). -->
    <app-modal [aberto]="ajustando !== null" [titulo]="'Ajustar valor · ' + (ajustando?.descricao ?? '')" (fechar)="ajustando = null">
      @if (ajustando; as aj) {
        <form (ngSubmit)="confirmarAjuste()" #fa="ngForm" class="formulario">
          <label>Valor pago <app-campo-valor name="valorPago" [(ngModel)]="valorAjuste" obrigatorio [minimo]="0.01" /></label>
          <p class="texto-suave">Muda a despesa lançada em {{ aj.ultimoPeloCofrinho?.data | date: 'dd/MM/yyyy' }}. O valor da
            conta a pagar continua o mesmo para os próximos meses.</p>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="ajustando = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fa.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class ContasAgendadasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  /** #158: aba aberta (?aba=recorrentes). */
  readonly aba = signal<'contas' | 'recorrentes'>(
    this.rota.snapshot.queryParamMap.get('aba') === 'recorrentes' ? 'recorrentes' : 'contas');

  constructor() {
    this.rota.queryParamMap.pipe(takeUntilDestroyed())
      .subscribe((p) => this.aba.set(p.get('aba') === 'recorrentes' ? 'recorrentes' : 'contas'));
  }

  /** Trocar de aba limpa os filtros da outra (replaceUrl, para o voltar do navegador não passar por cada clique). */
  mudarAba(aba: 'contas' | 'recorrentes'): void {
    this.aba.set(aba);
    void this.router.navigate([], { relativeTo: this.rota, replaceUrl: true,
      queryParams: { aba: aba === 'recorrentes' ? 'recorrentes' : null } });
  }

  readonly FormaCobranca = FormaCobranca;
  readonly itens = signal<ContaAgendada[]>([]);
  readonly contas = signal<Conta[]>([]);
  readonly categorias = signal<Categoria[]>([]);
  /** #691: as reservas (cofrinhos) das contas, para o "Pago com". */
  readonly reservas = signal<Reserva[]>([]);
  readonly sugestoes = signal<SugestaoContaAgendada[]>([]);
  /** #158: avisos das recorrentes. */
  readonly avisosRecorrentes = signal<AvisoContaAgendada[]>([]);
  readonly ignoradas = signal<string[]>(chavesIgnoradasDe(ContasAgendadasComponent.lerGuardadas()));
  readonly separadas = computed(() => separarSugestoes(this.sugestoes(), this.ignoradas()));
  readonly categoriasDespesa = computed(() => categoriasDeDespesa(this.categorias()));
  readonly totalVencido = computed(() => totalVencido(this.itens()));
  readonly total30 = computed(() => totalProximos30(this.itens()));

  /** O formulário aberto (null = fechado) e o id da conta que ele edita (null = conta nova). */
  form: SalvarContaAgendada | null = null;
  editandoId: number | null = null;
  /** Sugestão que originou o formulário aberto: ao salvar, ela sai da lista mesmo se a descrição for editada. */
  private chaveSugestaoEmCadastro: string | null = null;
  /** O "Pagar" aberto (null = fechado). */
  pagando: AberturaPagamento | null = null;
  /** #691: conta a pagar com o valor pago pelo cofrinho em ajuste. */
  ajustando: ContaAgendada | null = null;
  valorAjuste = 0;

  readonly nomeForma = nomeForma;
  readonly nomeRecorrencia = nomeRecorrencia;
  readonly textoDiasParaVencer = textoDiasParaVencer;
  readonly baixaSozinha = baixaSozinha;
  readonly mostrarPagar = mostrarPagar;
  readonly textoPagamentoCofrinho = textoPagamentoCofrinho;

  ngOnInit(): void {
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
    this.api.categorias().subscribe((c) => this.categorias.set(c));
    // Complementar: sem as reservas, o "Pago com o cofrinho" mostra só o "Nenhum" e o "Outro…".
    this.api.reservas().subscribe({ next: (r) => this.reservas.set(r), error: () => this.reservas.set([]) });
  }

  /** #242 (ADR 0024): o banco que vale para a conta (null = selo neutro). */
  bancoDe(contaId: number | null): string | null {
    return bancoDe(this.contas(), contaId);
  }

  ehCartao(contaId: number | null): boolean {
    return ehCartao(this.contas(), contaId);
  }

  cofrinhoEncontrado(c: ContaAgendada): boolean {
    return cofrinhoEncontrado(c, this.reservas());
  }

  carregar(): void {
    this.api.contasAgendadas().subscribe({
      next: (i) => this.itens.set(i),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
    // Sugestão e aviso são complementares: se falharem, a tela segue sem eles.
    this.api.avisosContasAgendadas().subscribe({ next: (a) => this.avisosRecorrentes.set(a), error: () => this.avisosRecorrentes.set([]) });
    this.api.sugestoesContasAgendadas().subscribe({ next: (s) => this.sugestoes.set(s), error: () => this.sugestoes.set([]) });
  }

  /* -------------------------- formulário -------------------------- */

  nova(): void {
    this.editandoId = null;
    this.chaveSugestaoEmCadastro = null;
    this.form = formularioVazio();
  }

  editar(c: ContaAgendada): void {
    this.editandoId = c.id;
    this.chaveSugestaoEmCadastro = null;
    this.form = formularioDaConta(c);
  }

  cadastrarSugestao(s: SugestaoContaAgendada): void {
    this.editandoId = null;
    this.chaveSugestaoEmCadastro = s.chave;
    this.form = formularioDaSugestao(s);
  }

  salvar(fm: SalvarContaAgendada): void {
    const dto = corpoParaSalvar(fm, this.contas());
    const req = this.editandoId ? this.api.atualizarContaAgendada(this.editandoId, dto) : this.api.criarContaAgendada(dto);
    req.subscribe({
      next: () => {
        this.form = null;
        if (this.chaveSugestaoEmCadastro) this.gravarIgnoradas([...this.ignoradas(), this.chaveSugestaoEmCadastro]);
        this.chaveSugestaoEmCadastro = null;
        this.avisos.sucesso('Conta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  async excluir(): Promise<void> {
    if (!this.editandoId || !(await this.confirmacao.confirmar({ titulo: 'Excluir conta a pagar', mensagem: 'Excluir esta conta a pagar?', confirmar: 'Excluir', perigo: true }))) return;
    this.api.excluirContaAgendada(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* -------------------------- avisos das recorrentes -------------------------- */

  /** #158: "Atualizar o valor" ou "Atualizar o catálogo": a conta a pagar fica como a última cobrança. */
  atualizarPeloAviso(a: AvisoContaAgendada): void {
    const c = this.itens().find((x) => x.id === a.contaAgendadaId);
    if (!c) return;
    this.api.atualizarContaAgendada(c.id, corpoDoAviso(c, a)).subscribe({
      next: () => {
        this.avisos.sucesso(a.tipo === 'aumentou' ? 'Valor atualizado.' : `Agora cobra em ${a.contaNovaNome}.`);
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** #691: "Foi paga com um cofrinho": o "Pagar" com a data do vencimento e o primeiro cofrinho da conta já escolhidos. */
  pagarPeloAviso(a: AvisoContaAgendada): void {
    const c = this.itens().find((x) => x.id === a.contaAgendadaId);
    if (!c) return;
    this.pagando = aberturaPeloAviso(c, a, this.reservas());
  }

  /* -------------------------- sugestões do extrato -------------------------- */

  /**
   * #205: "é a mesma": grava a ligação da sugestão com a conta a pagar parecida, e a sugestão sai da lista. O aviso tem
   * "Desfazer" (Nielsen 3: saída clara para o clique por engano).
   */
  eAMesma(s: SugestaoContaAgendada): void {
    const id = s.pareceSerId;
    if (id === null || !s.chaveExtrato) return;
    this.api.ligarExtratoContaAgendada(id, s.chaveExtrato).subscribe({
      next: () => {
        this.sugestoes.update((lista) => lista.filter((x) => x.chaveExtrato !== s.chaveExtrato));
        this.avisos.sucesso(`Ligada a ${s.pareceSerNome}: essa sugestão não aparece mais.`, {
          rotulo: 'Desfazer',
          executar: () => this.api.desligarExtratoContaAgendada(id).subscribe({
            next: () => this.carregar(),
            error: (e) => this.avisos.erro(mensagemDeErro(e))
          })
        });
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  ignorar(s: SugestaoContaAgendada): void {
    this.gravarIgnoradas([...this.ignoradas(), s.chave]);
  }

  restaurarIgnoradas(): void {
    const naLista = new Set(this.separadas().ignoradas.map((s) => s.chave));
    this.gravarIgnoradas(this.ignoradas().filter((c) => !naLista.has(c)));
  }

  /** Lembrado só neste navegador: é conveniência de tela, não dado financeiro. */
  private static readonly CHAVE_IGNORADAS = 'financas.sugestoesContasIgnoradas';

  private static lerGuardadas(): string | null {
    try {
      return localStorage.getItem(ContasAgendadasComponent.CHAVE_IGNORADAS);
    } catch {
      return null;
    }
  }

  private gravarIgnoradas(chaves: string[]): void {
    const unicas = chavesParaGuardar(chaves);
    this.ignoradas.set(unicas);
    try {
      localStorage.setItem(ContasAgendadasComponent.CHAVE_IGNORADAS, JSON.stringify(unicas));
    } catch {
      // Sem armazenamento (janela anônima, bloqueado): vale só até recarregar a tela.
    }
  }

  /* -------------------------- pagar, ajustar e desfazer -------------------------- */

  abrirPagamento(c: ContaAgendada): void {
    this.pagando = { conta: c, data: dataIso(), cofrinho: null };
  }

  confirmarPagamento(p: PagamentoParaApi): void {
    if (!this.pagando) return;
    this.api.pagarContaAgendada(this.pagando.conta.id, p.lancar, p.data, p.cofrinho).subscribe({
      next: () => {
        this.pagando = null;
        this.avisos.sucesso('Pagamento registrado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirAjuste(c: ContaAgendada): void {
    this.ajustando = c;
    this.valorAjuste = c.ultimoPeloCofrinho?.valor ?? c.valor;
  }

  confirmarAjuste(): void {
    if (!this.ajustando) return;
    this.api.ajustarPagamentoCofrinho(this.ajustando.id, Number(this.valorAjuste)).subscribe({
      next: () => {
        this.ajustando = null;
        this.avisos.sucesso('Valor pago ajustado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** O lançamento sai e o vencimento volta: pede confirmação (Nielsen 5), e a baixa automática não se repete. */
  async desfazerPagamento(c: ContaAgendada): Promise<void> {
    const u = c.ultimoPeloCofrinho;
    if (!u || !(await this.confirmacao.confirmar({ titulo: 'Desfazer pagamento', mensagem: textoDesfazerPagamento(c, u),
      confirmar: 'Desfazer', perigo: true }))) return;
    this.api.desfazerPagamentoCofrinho(c.id).subscribe({
      next: () => {
        this.avisos.sucesso('Pagamento desfeito.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** #691: lança, pelo cofrinho, a despesa do último pagamento que ficou sem ela (confirma antes: Nielsen 5). */
  async lancarUltimoPeloCofrinho(c: ContaAgendada): Promise<void> {
    const data = c.ultimoSemDespesa;
    if (!data || !c.reservaNome || c.contaId === null) return;
    if (!(await this.confirmacao.confirmar({ titulo: 'Lançar pelo cofrinho', mensagem: textoLancarPeloCofrinho(c, data),
      confirmar: 'Lançar' }))) return;
    this.api.registrarSaidaReserva({ contaId: c.contaId, nome: c.reservaNome, data, valor: c.valor, descricao: c.descricao,
      categoriaId: c.categoriaId }).subscribe({
      next: () => {
        this.avisos.sucesso('Despesa lançada pelo cofrinho.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
