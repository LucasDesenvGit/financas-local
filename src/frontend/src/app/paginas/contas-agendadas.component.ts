import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { CampoDataComponent } from '../compartilhado/campo-data.component';
import { RouterLink } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  Categoria, Conta, ContaAgendada, NOMES_RECORRENCIA, OrigemConta, Recorrencia, SalvarContaAgendada,
  SugestaoContaAgendada, TipoCategoria
} from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { dataIso, dataIsoValida, paramsTransacoes, paraData, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';

@Component({
  selector: 'app-contas-agendadas',
  imports: [ListaSuspensaDirective, FormsModule, RouterLink, MoedaPipe, DatePipe, ModalComponent, CampoValorComponent, CampoDataComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Contas a pagar</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova conta</button>
    </header>

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
                    {{ c.diasParaVencer < 0 ? 'vencida há ' + (-c.diasParaVencer) + 'd' : c.diasParaVencer === 0 ? 'vence hoje' : 'em ' + c.diasParaVencer + ' dias' }}
                  </small>
                }
              </td>
              <td>{{ c.descricao }} @if (c.contaNome) { <small class="texto-suave">· {{ c.contaNome }}</small> }</td>
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
                @if (c.ativa) { <button type="button" class="botao botao-pequeno" (click)="abrirPagamento(c)">Pagar</button> }
                <button type="button" class="link" (click)="editar(c)">editar</button>
              </td>
            </tr>
          } @empty {
            <tr><td colspan="6" class="vazio">Cadastre aluguel, condomínio, escola, assinaturas… para ser lembrado no painel.</td></tr>
          }
        </tbody>
      </table>
    </div>

    @if (sugestoesVisiveis().length || ignoradasNaLista().length) {
      <section class="painel-bloco sugestoes expandir" aria-labelledby="titulo-sugestoes" animate.leave="recolher">
        <div class="bloco-titulo">
          <h2 id="titulo-sugestoes">Sugestões do extrato</h2>
        </div>
        <p class="texto-suave pequeno-texto">
          Pagamentos que se repetem todo mês nas suas contas e ainda não estão nesta lista. O banco não informa
          boletos futuros, então o vencimento é estimado pelo dia em que você costuma pagar. Confira antes de salvar.
        </p>
        @if (sugestoesVisiveis().length) {
          <div class="tabela-rolagem">
            <table class="tabela">
              <thead>
                <tr><th>Próximo (estimado)</th><th>Descrição</th><th>Categoria</th><th class="direita">Valor</th><th><span class="so-leitor">Ações</span></th></tr>
              </thead>
              <tbody>
                @for (s of sugestoesVisiveis(); track s.chave) {
                  <tr>
                    <td class="nowrap">{{ s.proximoVencimento | date: 'dd/MM/yyyy' }}<br>
                      <small class="texto-suave">costuma ser dia {{ s.diaHabitual }}</small>
                      @if (s.vencimentoSemPagamento) {
                        <br><small class="texto-alerta">o de {{ s.vencimentoSemPagamento | date: 'dd/MM' }} ainda não apareceu no extrato</small>
                      }</td>
                    <td>{{ s.descricao }}<br>
                      <small class="texto-suave">pago em {{ s.meses }} meses@if (s.pagamentos !== s.meses) { ({{ s.pagamentos }} pagamentos) } · último em {{ s.ultimoPagamento | date: 'dd/MM' }} · {{ s.contaNome }}</small></td>
                    <td>{{ s.categoriaNome ?? '—' }}</td>
                    <td class="direita valor nowrap">
                      @if (s.valorVariavel) {
                        ≈ {{ s.valor | currency }}<br>
                        <small class="texto-suave">varia de {{ s.valorMinimo | currency }} a {{ s.valorMaximo | currency }}</small>
                      } @else {
                        {{ s.valor | currency }}
                      }
                    </td>
                    <td class="direita nowrap">
                      <button type="button" class="botao botao-pequeno" (click)="cadastrarSugestao(s)"
                        [attr.aria-label]="'Cadastrar ' + s.descricao">Cadastrar</button>
                      <a class="link" routerLink="/transacoes" [queryParams]="linkPagamentos(s)"
                        [attr.aria-label]="'Ver pagamentos de ' + s.descricao">ver pagamentos</a>
                      <button type="button" class="link" (click)="ignorar(s)"
                        [attr.aria-label]="'Ignorar sugestão ' + s.descricao">ignorar</button>
                    </td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
        } @else {
          <p class="vazio">Nenhuma sugestão nova.</p>
        }
        @if (ignoradasNaLista().length) {
          <p class="pequeno-texto">
            <button type="button" class="link" (click)="restaurarIgnoradas()">
              Mostrar de novo {{ ignoradasNaLista().length }} sugestão(ões) ignorada(s)
            </button>
          </p>
        }
      </section>
    }

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar conta' : 'Nova conta a pagar'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="150"></label>
          <div class="grade-campos">
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>
            <label>Próximo vencimento <app-campo-data name="venc" [(ngModel)]="fm.proximoVencimento" obrigatorio /></label>
            <label>Recorrência
              <select name="rec" [(ngModel)]="fm.recorrencia">
                <option [ngValue]="Recorrencia.Mensal">Mensal</option>
                <option [ngValue]="Recorrencia.Anual">Anual</option>
                <option [ngValue]="Recorrencia.Unica">Única</option>
              </select>
            </label>
            <label>Categoria
              <select name="cat" [(ngModel)]="fm.categoriaId">
                <option [ngValue]="null">—</option>
                @for (c of categoriasDespesa(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
          </div>
          @if (fm.recorrencia !== Recorrencia.Unica) {
            <div class="grade-campos">
              <label>Até (opcional)
                <app-campo-data name="limite" [(ngModel)]="fm.dataLimite" [minimo]="fm.proximoVencimento"
                  ajuda="ajuda-limite" [invalido]="limiteInvalido(fm)" /></label>
              <label>ou quantos pagamentos faltam
                <input type="number" name="restantes" min="1" step="1" [ngModel]="restantesDoForm(fm)"
                  (ngModelChange)="definirRestantes(fm, $event)" aria-describedby="ajuda-limite"></label>
            </div>
            @if (limiteInvalido(fm)) {
              <p class="negativo pequeno-texto" role="alert">A data limite não pode ser antes do próximo vencimento.</p>
            }
            <p class="texto-suave pequeno-texto" id="ajuda-limite">
              Depois do último pagamento a conta é encerrada sozinha e sai do Painel. Em branco, ela se repete sem fim.</p>
          }
          <label>Conta de pagamento (opcional)
            <select name="conta" [(ngModel)]="fm.contaId">
              <option [ngValue]="null">—</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="fm.ativa"> Ativa</label>
          <div class="rodape-form">
            @if (editandoId) { <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || limiteInvalido(fm)">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="pagando !== null" [titulo]="'Pagar · ' + (pagando?.descricao ?? '')" (fechar)="pagando = null">
      @if (pagando; as pg) {
        <form (ngSubmit)="confirmarPagamento()" class="formulario">
          <label>Data do pagamento <app-campo-data name="data" [(ngModel)]="dataPagamento" obrigatorio /></label>
          @if (pg.pagamentosRestantes === 1) {
            <p class="texto-suave" role="status">É o último pagamento (até {{ pg.dataLimite | date: 'dd/MM/yyyy' }}): a conta será encerrada.</p>
          }
          @if (podeLancar(pg)) {
            <label class="checkbox"><input type="checkbox" name="lancar" [(ngModel)]="lancarTransacao">
              Lançar a despesa na conta {{ pg.contaNome }}</label>
          } @else {
            <p class="texto-suave">Se o pagamento sair de uma conta do Open Finance, a transação chega pela sincronização.</p>
          }
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="pagando = null">Cancelar</button>
            <button class="botao botao-primario">Confirmar pagamento</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class ContasAgendadasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly Recorrencia = Recorrencia;
  readonly itens = signal<ContaAgendada[]>([]);
  readonly contas = signal<Conta[]>([]);
  readonly categorias = signal<Categoria[]>([]);
  readonly sugestoes = signal<SugestaoContaAgendada[]>([]);
  readonly ignoradas = signal<string[]>(ContasAgendadasComponent.lerIgnoradas());
  readonly sugestoesVisiveis = computed(() => this.sugestoes().filter((s) => !this.ignoradas().includes(s.chave)));
  readonly ignoradasNaLista = computed(() => this.sugestoes().filter((s) => this.ignoradas().includes(s.chave)));
  readonly categoriasDespesa = computed(() => this.categorias().filter((c) => c.tipo === TipoCategoria.Despesa && c.ativa));
  readonly totalVencido = computed(() =>
    this.itens().filter((c) => c.ativa && c.diasParaVencer < 0).reduce((s, c) => s + c.valor, 0));
  readonly total30 = computed(() =>
    this.itens().filter((c) => c.ativa && c.diasParaVencer >= 0 && c.diasParaVencer <= 30).reduce((s, c) => s + c.valor, 0));

  form: SalvarContaAgendada | null = null;
  editandoId: number | null = null;
  /** Sugestão que originou o formulário aberto: ao salvar, ela sai da lista mesmo se a descrição for editada. */
  private chaveSugestaoEmCadastro: string | null = null;
  pagando: ContaAgendada | null = null;
  dataPagamento = dataIso();
  lancarTransacao = false;

  ngOnInit(): void {
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
    this.api.categorias().subscribe((c) => this.categorias.set(c));
  }

  nomeRecorrencia(r: Recorrencia): string {
    return NOMES_RECORRENCIA[r];
  }

  podeLancar(c: ContaAgendada): boolean {
    return this.contas().some((x) => x.id === c.contaId && x.origem === OrigemConta.Manual);
  }

  carregar(): void {
    this.api.contasAgendadas().subscribe({
      next: (i) => this.itens.set(i),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
    // Sugestão é complementar: se falhar, a tela segue sem ela.
    this.api.sugestoesContasAgendadas().subscribe({ next: (s) => this.sugestoes.set(s), error: () => this.sugestoes.set([]) });
  }

  cadastrarSugestao(s: SugestaoContaAgendada): void {
    this.editandoId = null;
    this.chaveSugestaoEmCadastro = s.chave;
    this.form = {
      descricao: s.descricao, valor: s.valor, proximoVencimento: s.proximoVencimento, recorrencia: Recorrencia.Mensal,
      categoriaId: s.categoriaId, contaId: s.contaId, ativa: true, dataLimite: null
    };
  }

  linkPagamentos(s: SugestaoContaAgendada): Record<string, string> {
    const inicio = paraData(s.primeiroPagamento);
    const fim = paraData(s.ultimoPagamento);
    return {
      ...paramsTransacoes({
        tipo: 'despesa',
        inicio: primeiroDiaMes(inicio.getFullYear(), inicio.getMonth() + 1),
        fim: ultimoDiaMes(fim.getFullYear(), fim.getMonth() + 1)
      }),
      busca: s.termoBusca
    };
  }

  ignorar(s: SugestaoContaAgendada): void {
    this.gravarIgnoradas([...this.ignoradas(), s.chave]);
  }

  restaurarIgnoradas(): void {
    const naLista = new Set(this.ignoradasNaLista().map((s) => s.chave));
    this.gravarIgnoradas(this.ignoradas().filter((c) => !naLista.has(c)));
  }

  /** Lembrado só neste navegador: é conveniência de tela, não dado financeiro. */
  private static readonly CHAVE_IGNORADAS = 'financas.sugestoesContasIgnoradas';

  private static lerIgnoradas(): string[] {
    try {
      const valor = JSON.parse(localStorage.getItem(ContasAgendadasComponent.CHAVE_IGNORADAS) ?? '[]');
      return Array.isArray(valor) ? valor.filter((c): c is string => typeof c === 'string') : [];
    } catch {
      return [];
    }
  }

  private gravarIgnoradas(chaves: string[]): void {
    const unicas = [...new Set(chaves)].slice(-200);
    this.ignoradas.set(unicas);
    try {
      localStorage.setItem(ContasAgendadasComponent.CHAVE_IGNORADAS, JSON.stringify(unicas));
    } catch {
      // Sem armazenamento (janela anônima, bloqueado): vale só até recarregar a tela.
    }
  }

  nova(): void {
    this.editandoId = null;
    this.chaveSugestaoEmCadastro = null;
    this.form = { descricao: '', valor: 0, proximoVencimento: dataIso(), recorrencia: Recorrencia.Mensal, categoriaId: null, contaId: null, ativa: true,
      dataLimite: null };
  }

  editar(c: ContaAgendada): void {
    this.editandoId = c.id;
    this.chaveSugestaoEmCadastro = null;
    this.form = {
      descricao: c.descricao, valor: c.valor, proximoVencimento: c.proximoVencimento, recorrencia: c.recorrencia,
      categoriaId: c.categoriaId, contaId: c.contaId, ativa: c.ativa, dataLimite: c.dataLimite
    };
  }

  /** #69: data limite antes do próximo vencimento (só importa com a conta ativa; a API recusa do mesmo jeito). */
  limiteInvalido(fm: SalvarContaAgendada): boolean {
    return fm.ativa && fm.recorrencia !== Recorrencia.Unica && !!fm.dataLimite && !!fm.proximoVencimento
      && fm.dataLimite < fm.proximoVencimento;
  }

  /** Pagamentos que faltam pela data limite, contando o próximo (o atalho do formulário mostra os dois jeitos). */
  restantesDoForm(fm: SalvarContaAgendada): number | null {
    if (!fm.dataLimite || !dataIsoValida(fm.proximoVencimento) || fm.dataLimite < fm.proximoVencimento) return null;
    let n = 0;
    for (let v = fm.proximoVencimento; v <= fm.dataLimite && n < 1200; v = avancarVencimento(v, fm.recorrencia)) n++;
    return n;
  }

  /** "Faltam N pagamentos" vira a data do N-ésimo vencimento a partir do próximo. */
  definirRestantes(fm: SalvarContaAgendada, valor: number | string | null): void {
    const n = Math.floor(Number(valor));
    if (!valor || !Number.isFinite(n) || n < 1 || !dataIsoValida(fm.proximoVencimento)) {
      fm.dataLimite = null;
      return;
    }
    let v = fm.proximoVencimento;
    for (let i = 1; i < Math.min(n, 1200); i++) v = avancarVencimento(v, fm.recorrencia);
    fm.dataLimite = v;
  }

  salvar(): void {
    if (!this.form) return;
    const dto = {
      ...this.form, valor: Number(this.form.valor),
      dataLimite: this.form.recorrencia === Recorrencia.Unica || !this.form.dataLimite ? null : this.form.dataLimite
    };
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

  excluir(): void {
    if (!this.editandoId || !confirm('Excluir esta conta a pagar?')) return;
    this.api.excluirContaAgendada(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirPagamento(c: ContaAgendada): void {
    this.pagando = c;
    this.dataPagamento = dataIso();
    this.lancarTransacao = this.podeLancar(c);
  }

  confirmarPagamento(): void {
    if (!this.pagando) return;
    this.api.pagarContaAgendada(this.pagando.id, this.lancarTransacao && this.podeLancar(this.pagando), this.dataPagamento)
      .subscribe({
        next: () => {
          this.pagando = null;
          this.avisos.sucesso('Pagamento registrado.');
          this.carregar();
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      });
  }
}

/**
 * Próximo vencimento como a API avança ao pagar (DateOnly.AddMonths/AddYears a partir do anterior): dia que não
 * existe no mês cai no último dia (31/01 → 28/02 → 28/03).
 */
export function avancarVencimento(iso: string, recorrencia: Recorrencia): string {
  const [a, m, d] = iso.split('-').map(Number);
  const meses = recorrencia === Recorrencia.Anual ? 12 : 1;
  const alvo = new Date(a, m - 1 + meses, 1);
  const ultimo = new Date(alvo.getFullYear(), alvo.getMonth() + 1, 0).getDate();
  return dataIso(new Date(alvo.getFullYear(), alvo.getMonth(), Math.min(d, ultimo)));
}
