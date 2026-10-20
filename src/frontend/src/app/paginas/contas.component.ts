import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Banco, Conta, Fatura, NOMES_TIPO_CONTA, OrigemConta, SalvarConta, SituacaoFatura, TipoConta } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { visualBanco } from '../compartilhado/bancos';
import { LogoBancoComponent } from '../compartilhado/logo-banco.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { TermoComponent } from '../compartilhado/termo.component';
import { CORES_SUGERIDAS, NOMES_MESES } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';

/** Quantas faturas o histórico mostra antes de "mostrar mais antigas". */
const FATURAS_VISIVEIS = 6;

/**
 * Contas e cartões. Shneiderman: o resumo primeiro (saldo, faturas do mês, dívida, limite), depois cada conta e
 * cartão, e o histórico de faturas só sob demanda (Progressive Disclosure). ADR 0024: cada conta e cartão mostra o
 * selo do seu banco (escolhido ou detectado pelo nome; o cartão herda o da conta a que pertence).
 */
@Component({
  selector: 'app-contas',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, DatePipe, RouterLink, ModalComponent, LogoBancoComponent, TermoComponent, CampoValorComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Contas e cartões</h1>
      <div class="acoes">
        <!-- #130: efeito imediato → interruptor; o estado fica na URL (?arquivadas=1). -->
        <label class="interruptor"><input type="checkbox" role="switch" [(ngModel)]="mostrarArquivadas"
          (change)="alternarArquivadas()"> Mostrar arquivadas</label>
        <!-- #172: comparar com os apps dos bancos (Nielsen 1). -->
        <a class="botao" routerLink="/conferir-banco">Conferir com o banco</a>
        <button type="button" class="botao botao-primario" (click)="nova()">+ Conta manual</button>
      </div>
    </header>

    <section class="kpis kpis-contas" aria-label="Resumo das contas e cartões">
      <article class="kpi">
        <span>Saldo em contas</span>
        <strong [class.negativo]="totalContas() < 0">{{ totalContas() | currency }}</strong>
        <small>{{ contasBancarias().length }} conta(s)</small>
      </article>
      <article class="kpi">
        <span><app-termo chave="faturaDoMes">Faturas de {{ nomeMesAtual }}</app-termo></span>
        <strong class="negativo">{{ totalFaturasMes() | currency }}</strong>
        <small>lançado nos cartões neste mês</small>
      </article>
      <article class="kpi">
        <span><app-termo chave="dividaCartoes">Dívida total nos cartões</app-termo></span>
        <strong class="negativo">{{ totalDivida() | currency }}</strong>
        <small>com parcelas futuras</small>
      </article>
      <article class="kpi">
        <span>Limite disponível</span>
        <strong>{{ totalDisponivel() | currency }}</strong>
        <small>{{ cartoes().length }} cartão(ões)</small>
      </article>
    </section>

    <p class="texto-suave pequeno-texto">Contas do Open Finance são criadas pela sincronização
      (<a routerLink="/configuracoes">Configurações › Open Finance</a>); contas manuais servem para dinheiro em espécie,
      bancos sem Open Finance ou controles separados. O banco de cada conta é reconhecido pelo nome: se não estiver
      certo, escolha em "editar".</p>

    <!-- Contas (dinheiro disponível) -->
    <section class="secao-contas" aria-labelledby="titulo-contas">
      <header class="secao-cabecalho">
        <div>
          <h2 id="titulo-contas">Contas</h2>
          <small class="texto-suave">{{ contasBancarias().length }} conta(s)</small>
        </div>
        <div class="secao-total">
          <span class="texto-suave">Saldo total</span>
          <strong [class.negativo]="totalContas() < 0">{{ totalContas() | currency }}</strong>
        </div>
      </header>
      <div class="grade-cartoes">
        @for (c of contasBancarias(); track c.id) {
          <article class="cartao-conta" [class.arquivada]="c.arquivada" [style.--cor-conta]="corDe(c)">
            <header>
              <app-logo-banco [codigo]="c.banco" [tamanho]="40" />
              <div class="cresce">
                <strong>{{ c.nome }}</strong>
                <small class="texto-suave">{{ nomeBanco(c) }} · {{ nomeTipo(c.tipo) }}</small>
              </div>
              <span class="etiquetas-conta">
                <!-- #130: "Arquivada" em texto, não só o visual apagado (WCAG 1.4.1). -->
                @if (c.arquivada) { <span class="etiqueta etiqueta-arquivada">Arquivada</span> }
                <span class="etiqueta" [class.etiqueta-suave]="c.origem !== OrigemConta.OpenFinance">
                  {{ c.origem === OrigemConta.OpenFinance ? 'open finance' : 'manual' }}</span>
              </span>
            </header>
            <div class="saldo" [class.negativo]="c.saldo < 0">{{ c.saldo | currency }}</div>
            <small class="texto-suave">{{ c.incluirNoSaldo ? 'saldo atual' : 'saldo atual · fora do total' }}</small>
            @if (cartoesDa(c).length) {
              <small class="texto-suave cartoes-da-conta">Cartões: {{ nomesCartoes(c) }}</small>
            }
            <footer>
              <small class="texto-suave">{{ c.numero ?? '' }}</small>
              <span class="cresce"></span>
              <a class="link" routerLink="/transacoes" [queryParams]="{ conta: c.id }" [attr.aria-label]="'Ver transações de ' + c.nome">transações</a>
              <button type="button" class="link" (click)="editar(c)" [attr.aria-label]="'Editar ' + c.nome">editar</button>
            </footer>
          </article>
        } @empty {
          <p class="vazio">Nenhuma conta ainda. Configure o Open Finance ou crie uma conta manual.</p>
        }
      </div>
    </section>

    <!-- Cartões de crédito -->
    @if (cartoes().length) {
      <section class="secao-contas" aria-labelledby="titulo-cartoes">
        <header class="secao-cabecalho">
          <div>
            <h2 id="titulo-cartoes">Cartões de crédito</h2>
            <small class="texto-suave">{{ cartoes().length }} cartão(ões)</small>
          </div>
          <div class="secao-total">
            <span class="texto-suave">Faturas de {{ nomeMesAtual }}</span>
            <strong class="negativo">{{ totalFaturasMes() | currency }}</strong>
          </div>
        </header>
        <div class="grade-cartoes grade-cartoes-credito">
          @for (c of cartoes(); track c.id) {
            <article class="cartao-credito-bloco" [class.arquivada]="c.arquivada" [id]="'cartao-' + c.id">
              <div class="cartao-credito" [style.--cor-conta]="corDe(c)">
                <div class="cartao-credito-topo">
                  <span class="cartao-credito-instituicao">
                    <app-logo-banco [codigo]="c.banco" [tamanho]="26" [cartao]="true" />
                    {{ c.banco ? nomeBanco(c) : (c.nomeInstituicao ?? 'Cartão') }}
                  </span>
                  <span class="cartao-credito-tipo">crédito</span>
                </div>
                <span class="cartao-credito-chip" aria-hidden="true"></span>
                <div class="cartao-credito-rodape">
                  <strong>{{ c.nome }}</strong>
                  <span>•••• {{ finalCartao(c) }}</span>
                </div>
              </div>

              <div class="cartao-credito-detalhes">
                <div class="linha-valores">
                  <div class="valor-fatura-mes">
                    <span class="texto-suave">Fatura de {{ nomeMesAtual }}</span>
                    <strong class="negativo">{{ faturaDoMes(c) | currency }}</strong>
                    @if (c.faturaMes?.vencimento && c.faturaMes?.situacao; as situacao) {
                      <small class="texto-suave">@if (c.faturaMes!.fechamento) {fecha {{ c.faturaMes!.fechamento | date: 'dd/MM' }},}
                        vence {{ c.faturaMes!.vencimento | date: 'dd/MM' }}
                        <span class="etiqueta" [class.etiqueta-sucesso]="situacao === 'paga'"
                          [class.etiqueta-alerta]="situacao === 'fechada'">{{ nomeSituacao(situacao) }}</span></small>
                    } @else {
                      <small class="texto-suave">lançado no mês</small>
                    }
                  </div>
                  <div class="direita valor-divida">
                    <span class="texto-suave"><app-termo chave="dividaCartao">Dívida total</app-termo></span>
                    <strong class="negativo">{{ dividaTotal(c) | currency }}</strong>
                    <small class="texto-suave">com parcelas futuras</small>
                  </div>
                </div>
                @if (limiteEfetivo(c); as limite) {
                  <div class="trilho" [title]="usoLimite(c) + '% do limite em uso'">
                    <div class="progresso" [style.width.%]="usoLimite(c)"
                      [class.atencao]="usoLimite(c) >= 70 && usoLimite(c) < 90" [class.estourado]="usoLimite(c) >= 90"></div>
                  </div>
                  <small class="texto-suave">
                    {{ usoLimite(c) }}% de {{ limite | currency }} em uso ·
                    @if ((disponivel(c) ?? 0) < 0) { acima do limite em {{ -disponivel(c)! | currency }} }
                    @else { disponível {{ disponivel(c) !== null ? (disponivel(c) | currency) : '—' }} }
                    @if (limiteAprovadoMaior(c)) { · aprovado pelo banco: {{ c.limiteCredito | currency }} }
                  </small>
                }
                @if (c.diaFechamento || c.diaVencimento) {
                  <small class="texto-suave">
                    @if (c.diaFechamento) { fecha dia {{ c.diaFechamento }} }
                    @if (c.diaFechamento && c.diaVencimento) { · }
                    @if (c.diaVencimento) { vence dia {{ c.diaVencimento }} }
                    @if (melhorDia(c); as dia) {
                      · <app-termo chave="melhorDiaCompra">melhor dia de compra: {{ dia }}</app-termo>
                    }
                  </small>
                }
                @if (contaDoCartao(c); as conta) {
                  <small class="texto-suave">da conta {{ conta.nome }}</small>
                }
                <footer>
                  @if (c.arquivada) { <span class="etiqueta etiqueta-arquivada">Arquivada</span> }
                  <span class="etiqueta" [class.etiqueta-suave]="c.origem !== OrigemConta.OpenFinance">
                    {{ c.origem === OrigemConta.OpenFinance ? 'open finance' : 'manual' }}</span>
                  <span class="cresce"></span>
                  <a class="link" routerLink="/transacoes" [queryParams]="{ conta: c.id }" [attr.aria-label]="'Ver transações de ' + c.nome">transações</a>
                  <button type="button" class="link" (click)="editar(c)" [attr.aria-label]="'Editar ' + c.nome">editar</button>
                </footer>

                <button type="button" class="botao-historico" [attr.aria-expanded]="historicoAberto(c.id)"
                  [attr.aria-controls]="'historico-' + c.id" (click)="alternarHistorico(c)">
                  <span aria-hidden="true">{{ historicoAberto(c.id) ? '▾' : '▸' }}</span> Histórico de faturas
                </button>
                <div class="historico-faturas" [id]="'historico-' + c.id">
                  @if (historicoAberto(c.id)) {
                    @if (faturasDe(c.id); as lista) {
                      @if (lista.length) {
                        <table class="tabela tabela-historico expandir" animate.leave="recolher">
                          <caption class="so-leitor">Faturas de {{ c.nome }}, da mais recente para a mais antiga</caption>
                          <thead><tr><th>Vencimento</th><th class="direita">Total</th><th>Situação</th></tr></thead>
                          <tbody>
                            @for (f of visiveis(c.id, lista); track f.id) {
                              <tr>
                                <td class="nowrap">{{ f.dataVencimento | date: 'dd/MM/yy' }}
                                  @if (f.dataFechamento) { <br><small class="texto-suave">fechou {{ f.dataFechamento | date: 'dd/MM' }}</small> }
                                </td>
                                <td class="direita nowrap">{{ f.valorTotal | currency }}
                                  @if (f.valorPago !== null && f.situacao !== 'paga' && f.valorPago > 0) {
                                    <br><small class="texto-suave">pago {{ f.valorPago | currency }}</small>
                                  }
                                </td>
                                <td><span class="etiqueta" [class.etiqueta-sucesso]="f.situacao === 'paga'"
                                  [class.etiqueta-alerta]="f.situacao === 'fechada'">{{ nomeSituacao(f.situacao) }}</span></td>
                              </tr>
                            }
                          </tbody>
                        </table>
                        @if (lista.length > visiveis(c.id, lista).length) {
                          <button type="button" class="link" (click)="mostrarTodas(c.id)">
                            Mostrar mais antigas ({{ lista.length - visiveis(c.id, lista).length }})</button>
                        }
                      } @else {
                        <p class="texto-suave pequeno-texto">Sem faturas registradas. O histórico vem do banco pelo Open Finance;
                          cartão manual não tem.</p>
                      }
                    } @else {
                      <p class="texto-suave pequeno-texto" role="status">Carregando…</p>
                    }
                  }
                </div>
              </div>
            </article>
          }
        </div>
      </section>
    }

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar conta' : 'Nova conta manual'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          @if (editandoOpenFinance) {
            <p class="aviso-inline expandir" animate.leave="recolher">Conta do Open Finance: saldo, limite e tipo vêm do banco. Você pode alterar nome, banco,
              cor e visibilidade.</p>
          }
          <label>Nome <input name="nome" [(ngModel)]="fm.nome" required maxlength="120"></label>
          <div class="campo-banco">
            <app-logo-banco [codigo]="previaBanco(fm)" [tamanho]="40" [cartao]="ehCartao(fm)" />
            <label class="cresce">Banco
              <select name="banco" [(ngModel)]="fm.banco" aria-describedby="ajuda-banco">
                <option [ngValue]="null">{{ textoDetectar() }}</option>
                @for (b of bancos(); track b.codigo) { <option [ngValue]="b.codigo">{{ b.nome }}</option> }
              </select>
            </label>
          </div>
          <p class="texto-suave pequeno-texto" id="ajuda-banco">
            Mostra o selo do banco na conta{{ ehCartao(fm) ? ' e no cartão' : '' }}.
            @if (ehCartao(fm)) { Sem banco escolhido, o cartão usa o da conta dele. }
          </p>
          @if (ehCartao(fm)) {
            <label>Conta do cartão
              <select name="contaVinculada" [(ngModel)]="fm.contaVinculadaId" aria-describedby="ajuda-conta-cartao">
                <option [ngValue]="null">Nenhuma</option>
                @for (c of contasParaVincular(); track c.id) { <option [ngValue]="c.id">{{ rotuloContaVinculada(c) }}</option> }
              </select>
            </label>
            <p class="texto-suave pequeno-texto" id="ajuda-conta-cartao">A conta do mesmo banco, de onde a fatura é paga.</p>
          }
          @if (!editandoOpenFinance) {
            <div class="grade-campos">
              <label>Tipo
                <select name="tipo" [(ngModel)]="fm.tipo">
                  @for (t of tipos; track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
                </select>
              </label>
              <label>Saldo inicial
                <app-campo-valor name="saldoInicial" [(ngModel)]="fm.saldoInicial" negativo />
              </label>
              <label>Número / identificação <input name="numero" [(ngModel)]="fm.numero" maxlength="60"></label>
              @if (fm.tipo === TipoConta.CartaoCredito) {
                <label>Limite <app-campo-valor name="limite" [(ngModel)]="fm.limiteCredito" /></label>
                <label>Dia de fechamento <input type="number" name="fech" [(ngModel)]="fm.diaFechamento" min="1" max="31"></label>
                <label>Dia de vencimento <input type="number" name="venc" [(ngModel)]="fm.diaVencimento" min="1" max="31"></label>
              }
            </div>
            @if (fm.tipo === TipoConta.CartaoCredito) {
              <p class="texto-suave">Para cartão, use saldo inicial negativo para uma fatura já em aberto.</p>
            }
          }
          <fieldset class="campo-cor">
            <legend>Cor <small class="texto-suave">(vale quando o banco não é identificado)</small></legend>
            <div class="cores">
              @for (cor of cores; track cor) {
                <button type="button" class="cor" [style.background]="cor" [class.selecionada]="fm.cor === cor"
                  (click)="fm.cor = cor" [attr.aria-label]="'Cor ' + cor" [attr.aria-pressed]="fm.cor === cor"></button>
              }
            </div>
          </fieldset>
          <label class="checkbox"><input type="checkbox" name="incluir" [(ngModel)]="fm.incluirNoSaldo"> Incluir no saldo total</label>
          <label class="checkbox"><input type="checkbox" name="arquivada" [(ngModel)]="fm.arquivada"> Arquivada</label>
          <div class="rodape-form">
            @if (editandoId && !editandoOpenFinance) {
              <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    .kpis-contas { margin-bottom: 10px; }
    .cartoes-da-conta { display: block; margin-top: 4px; }
    .cartao-credito-instituicao { display: inline-flex; align-items: center; gap: 8px; }
    .valor-fatura-mes small, .valor-divida small { font-size: 0.78rem; }
    .botao-historico {
      font: inherit; font-size: 0.88rem; font-weight: 600; color: var(--primaria-forte); background: none; border: 0;
      cursor: pointer; padding: 8px 0 0; min-height: 32px; text-align: left; border-top: 1px solid var(--linha); margin-top: 6px;
    }
    .tabela-historico { font-size: 0.85rem; }
    .tabela-historico th, .tabela-historico td { padding: 6px 8px; }
    .historico-faturas > .link { margin-top: 6px; }
    .campo-banco { display: flex; gap: 12px; align-items: flex-end; }
    .campo-cor { border: 0; padding: 0; margin: 0; }
    .campo-cor legend { font-size: 0.85rem; font-weight: 600; margin-bottom: 6px; padding: 0; }
  `
})
export class ContasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);
  /** UX-12: /contas?faturas=ID abre o histórico de faturas do cartão (link de "Faturas do mês" no Painel). */
  private faturasPelaUrl = Number(this.rota.snapshot.queryParamMap.get('faturas')) || null;

  readonly TipoConta = TipoConta;
  readonly OrigemConta = OrigemConta;
  readonly cores = CORES_SUGERIDAS;
  readonly tipos = [TipoConta.ContaCorrente, TipoConta.Poupanca, TipoConta.CartaoCredito, TipoConta.Investimento,
    TipoConta.Dinheiro, TipoConta.Outro];
  readonly nomeMesAtual = NOMES_MESES[new Date().getMonth()];

  readonly contas = signal<Conta[]>([]);
  readonly bancos = signal<Banco[]>([]);
  /** Históricos abertos e as faturas já carregadas de cada cartão (carrega só ao abrir). */
  private readonly abertos = signal<Set<number>>(new Set());
  private readonly faturas = signal<Map<number, Fatura[]>>(new Map());
  private readonly todasVisiveis = signal<Set<number>>(new Set());

  /** Ordem: pelo banco (contas do mesmo banco ficam juntas) e depois pelo nome. */
  private readonly ordenadas = computed(() => [...this.contas()].sort((a, b) =>
    (this.nomeBanco(a, '~')).localeCompare(this.nomeBanco(b, '~'), 'pt-BR') || a.nome.localeCompare(b.nome, 'pt-BR')));
  readonly contasBancarias = computed(() => this.ordenadas().filter((c) => c.tipo !== TipoConta.CartaoCredito));
  readonly cartoes = computed(() => this.ordenadas().filter((c) => c.tipo === TipoConta.CartaoCredito));
  readonly totalContas = computed(() =>
    this.contasBancarias().reduce((s, c) => s + (c.incluirNoSaldo && !c.arquivada ? c.saldo : 0), 0));
  readonly totalFaturasMes = computed(() =>
    this.cartoes().reduce((s, c) => s + (c.arquivada ? 0 : this.faturaDoMes(c)), 0));
  readonly totalDivida = computed(() =>
    this.cartoes().reduce((s, c) => s + (c.arquivada ? 0 : this.dividaTotal(c)), 0));
  readonly totalDisponivel = computed(() =>
    this.cartoes().reduce((s, c) => s + (c.arquivada ? 0 : Math.max(0, this.disponivel(c) ?? 0)), 0));
  /** Contas que podem ser "a conta do cartão": as que não são cartão (nem o próprio cartão em edição). */
  readonly contasParaVincular = computed(() => this.contasBancarias().filter((c) => c.id !== this.editandoId));

  /** #130: estado na URL (?arquivadas=1): voltar do navegador e link salvo abrem do mesmo jeito (ADR 0018). */
  mostrarArquivadas = this.rota.snapshot.queryParamMap.get('arquivadas') === '1';
  form: SalvarConta | null = null;
  editandoId: number | null = null;
  editandoOpenFinance = false;
  /** Tipo da conta do Open Finance em edição (o formulário não mostra o campo, mas o cartão tem conta vinculada). */
  private tipoEditando: TipoConta | null = null;
  /** Banco que vale hoje para a conta em edição (para o "Detectar pelo nome (Nubank)"). */
  private bancoAtual: string | null = null;

  ngOnInit(): void {
    this.carregar();
    this.api.bancos().subscribe({ next: (b) => this.bancos.set(b), error: () => this.bancos.set([]) });
  }

  nomeTipo(t: TipoConta): string {
    return NOMES_TIPO_CONTA[t];
  }

  /** Nome do banco da conta (o que vale), ou um texto para quando não foi identificado. */
  nomeBanco(c: Conta, semBanco = 'Banco não identificado'): string {
    return this.nomeBancoCodigo(c.banco) ?? semBanco;
  }

  nomeBancoCodigo(codigo: string | null): string | null {
    if (!codigo) return null;
    return this.bancos().find((b) => b.codigo === codigo)?.nome ?? visualBanco(codigo)?.nome ?? null;
  }

  /** A cor do banco quando ele é conhecido (o cartão "fica com a cara" do banco); senão, a cor escolhida. */
  corDe(c: Conta): string {
    return visualBanco(c.banco)?.fundo ?? c.cor;
  }

  cartoesDa(conta: Conta): Conta[] {
    return this.cartoes().filter((c) => c.contaVinculadaId === conta.id);
  }

  nomesCartoes(conta: Conta): string {
    return this.cartoesDa(conta).map((c) => c.nome).join(', ');
  }

  /** "Conta corrente · Itaú"; sem repetir quando o nome já diz o banco ("Nubank"). */
  rotuloContaVinculada(c: Conta): string {
    const banco = this.nomeBancoCodigo(c.banco);
    const semAcento = (t: string) => t.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
    return banco && !semAcento(c.nome).includes(semAcento(banco)) ? `${c.nome} · ${banco}` : c.nome;
  }

  contaDoCartao(cartao: Conta): Conta | null {
    return cartao.contaVinculadaId ? this.contas().find((c) => c.id === cartao.contaVinculadaId) ?? null : null;
  }

  finalCartao(c: Conta): string {
    const digitos = (c.numero ?? '').replace(/\D/g, '');
    return digitos.length >= 4 ? digitos.slice(-4) : '••••';
  }

  /** A fatura do cartão que fecha neste mês, a mesma conta do Painel (#144). */
  faturaDoMes(c: Conta): number {
    return c.faturaMes?.valorMes ?? 0;
  }

  /** Tudo o que se deve no cartão hoje, com parcelas futuras (o saldo do cartão é negativo quando há dívida). */
  dividaTotal(c: Conta): number {
    return Math.max(0, -c.saldo);
  }

  disponivel(c: Conta): number | null {
    return c.limiteDisponivel;
  }

  /**
   * Limite realmente em uso. Muitos bancos informam pelo Open Finance o limite APROVADO (ex.: R$ 10.150),
   * mas o disponível já reflete o limite que você ajustou no app. Por isso usamos dívida + disponível.
   */
  limiteEfetivo(c: Conta): number | null {
    if (c.limiteDisponivel !== null) {
      const efetivo = this.dividaTotal(c) + Math.max(0, c.limiteDisponivel);
      if (efetivo > 0) return c.limiteCredito ? Math.min(efetivo, c.limiteCredito) : efetivo;
    }
    return c.limiteCredito;
  }

  usoLimite(c: Conta): number {
    const limite = this.limiteEfetivo(c);
    if (!limite) return 0;
    return Math.round(Math.min(100, Math.max(0, (this.dividaTotal(c) / limite) * 100)));
  }

  limiteAprovadoMaior(c: Conta): boolean {
    const efetivo = this.limiteEfetivo(c);
    return !!c.limiteCredito && !!efetivo && c.limiteCredito > efetivo * 1.05;
  }

  /** Dia seguinte ao fechamento (compra a partir dele cai na fatura seguinte). */
  melhorDia(c: Conta): number | null {
    if (!c.diaFechamento) return null;
    return c.diaFechamento >= 31 ? 1 : c.diaFechamento + 1;
  }

  nomeSituacao(s: SituacaoFatura): string {
    return s === 'paga' ? 'paga' : s === 'fechada' ? 'fechada' : 'em aberto';
  }

  /* ------------------------------ histórico de faturas ------------------------------ */

  historicoAberto(id: number): boolean {
    return this.abertos().has(id);
  }

  faturasDe(id: number): Fatura[] | null {
    return this.faturas().get(id) ?? null;
  }

  visiveis(id: number, lista: Fatura[]): Fatura[] {
    return this.todasVisiveis().has(id) ? lista : lista.slice(0, FATURAS_VISIVEIS);
  }

  mostrarTodas(id: number): void {
    this.todasVisiveis.update((s) => new Set(s).add(id));
  }

  /** Abre ou fecha o histórico; o aberto por último fica na URL (?faturas=ID), para o F5 e o link do Painel. */
  alternarHistorico(c: Conta): void {
    const abrir = !this.abertos().has(c.id);
    this.abertos.update((s) => {
      const novo = new Set(s);
      if (abrir) novo.add(c.id); else novo.delete(c.id);
      return novo;
    });
    if (abrir) this.carregarFaturas(c.id);
    const naUrl = Number(this.rota.snapshot.queryParamMap.get('faturas')) || null;
    if (abrir || naUrl === c.id) {
      void this.router.navigate([], {
        relativeTo: this.rota, queryParams: { faturas: abrir ? c.id : null }, queryParamsHandling: 'merge', replaceUrl: true
      });
    }
  }

  private carregarFaturas(id: number): void {
    if (this.faturas().has(id)) return;
    this.api.faturas(id).subscribe({
      next: (f) => this.faturas.update((m) => new Map(m).set(id, f)),
      error: (e) => {
        this.faturas.update((m) => new Map(m).set(id, []));
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  alternarArquivadas(): void {
    this.router.navigate([], {
      relativeTo: this.rota, queryParams: { arquivadas: this.mostrarArquivadas ? 1 : null },
      queryParamsHandling: 'merge', replaceUrl: true
    });
    this.carregar();
  }

  carregar(): void {
    this.api.contas(this.mostrarArquivadas).subscribe({
      next: (c) => {
        this.contas.set(c);
        const cartao = this.faturasPelaUrl ? c.find((x) => x.id === this.faturasPelaUrl && x.tipo === TipoConta.CartaoCredito) : undefined;
        this.faturasPelaUrl = null;
        if (cartao && !this.abertos().has(cartao.id)) {
          this.alternarHistorico(cartao);
          // Leva até o cartão quando a tela abre por um link (ex.: "Faturas do mês" no Painel).
          setTimeout(() => document.getElementById('cartao-' + cartao.id)?.scrollIntoView?.({ block: 'start' }));
        }
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ formulário ------------------------------ */

  ehCartao(fm: SalvarConta): boolean {
    return (this.editandoOpenFinance ? this.tipoEditando : fm.tipo) === TipoConta.CartaoCredito;
  }

  /** "Detectar pelo nome", com o banco que ficaria quando dá para saber (conta já salva, sem banco escolhido). */
  textoDetectar(): string {
    const detectado = this.form && this.form.banco === null && this.bancoAtual ? this.nomeBancoCodigo(this.bancoAtual) : null;
    return detectado ? `Detectar pelo nome (${detectado})` : 'Detectar pelo nome';
  }

  /** O selo que a conta vai ter com o que está no formulário. */
  previaBanco(fm: SalvarConta): string | null {
    if (fm.banco === 'outro') return null;
    if (fm.banco) return fm.banco;
    if (this.ehCartao(fm) && fm.contaVinculadaId) {
      const conta = this.contas().find((c) => c.id === fm.contaVinculadaId);
      if (conta?.banco) return conta.banco;
    }
    return this.bancoAtual;
  }

  nova(): void {
    this.editandoId = null;
    this.editandoOpenFinance = false;
    this.tipoEditando = null;
    this.bancoAtual = null;
    this.form = {
      nome: '', tipo: TipoConta.Dinheiro, numero: null, saldoInicial: 0, limiteCredito: null, diaFechamento: null,
      diaVencimento: null, cor: CORES_SUGERIDAS[0], incluirNoSaldo: true, arquivada: false, banco: null, contaVinculadaId: null
    };
  }

  editar(c: Conta): void {
    this.editandoId = c.id;
    this.editandoOpenFinance = c.origem === OrigemConta.OpenFinance;
    this.tipoEditando = c.tipo;
    this.bancoAtual = c.bancoDefinido ? null : c.banco;
    this.form = {
      nome: c.nome, tipo: c.tipo, numero: c.numero, saldoInicial: c.saldoInicial, limiteCredito: c.limiteCredito,
      diaFechamento: c.diaFechamento, diaVencimento: c.diaVencimento, cor: c.cor, incluirNoSaldo: c.incluirNoSaldo,
      arquivada: c.arquivada, banco: c.bancoDefinido ? (c.banco ?? 'outro') : null, contaVinculadaId: c.contaVinculadaId
    };
  }

  salvar(): void {
    if (!this.form) return;
    const dto: SalvarConta = {
      ...this.form,
      saldoInicial: Number(this.form.saldoInicial) || 0,
      limiteCredito: this.form.limiteCredito ? Number(this.form.limiteCredito) : null,
      diaFechamento: this.form.diaFechamento ? Number(this.form.diaFechamento) : null,
      diaVencimento: this.form.diaVencimento ? Number(this.form.diaVencimento) : null,
      numero: this.form.numero || null,
      contaVinculadaId: this.ehCartao(this.form) ? this.form.contaVinculadaId : null
    };
    const req = this.editandoId ? this.api.atualizarConta(this.editandoId, dto) : this.api.criarConta(dto);
    req.subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Conta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluir(): void {
    if (!this.editandoId || !confirm('Excluir a conta e TODAS as suas transações? Esta ação não pode ser desfeita.')) return;
    this.api.excluirConta(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Conta excluída.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
