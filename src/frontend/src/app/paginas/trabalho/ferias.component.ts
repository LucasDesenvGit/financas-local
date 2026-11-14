import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { CalculoFerias, Emprego, PeriodoFerias } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { empregoAtual } from './salario';
import { diferencaDaVenda, periodoParaAgendar, primeiroDiaPossivel, situacaoDoPeriodo } from './ferias';

/**
 * #213 (ADR 0065): Trabalho › Férias. Os períodos aquisitivos com o direito, o que já foi agendado e o que sobra, com o
 * aviso de férias vencidas ou vencendo (Nielsen 1); o agendamento de uma fração com a venda de dias e o 13º junto; a
 * conta de cada fração (pagamento até 2 dias antes); e "Comparar 30 dias × vender 10" lado a lado. Estimativa.
 */
@Component({
  selector: 'app-trabalho-ferias',
  imports: [FormsModule, DatePipe, RouterLink, MoedaPipe, CampoDataComponent, ListaSuspensaDirective],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina"><h1>Férias</h1></header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (!emprego()) {
      <section class="painel-bloco"><p>Cadastre o emprego antes, em <a routerLink="/trabalho/salario">Trabalho › Salário</a>.</p></section>
    } @else if (erro(); as msg) {
      <section class="painel-bloco"><p class="aviso-inline">{{ msg }}</p></section>
    } @else {
      <p class="texto-suave aviso-estimativa">Estimativa, não holerite: confira com o da empresa. Nada daqui entra nos relatórios.</p>
      <div class="grade-2">
        <section class="painel-bloco" aria-labelledby="titulo-periodos">
          <h2 id="titulo-periodos">Períodos</h2>
          <ul class="periodos">
            @for (p of periodos(); track p.inicio) {
              <li class="periodo">
                <div class="periodo-topo">
                  <strong>{{ p.inicio | date: 'dd/MM/yyyy' }} a {{ p.fim | date: 'dd/MM/yyyy' }}</strong>
                  <span [class]="'etiqueta tom-' + situacao(p).tom">{{ situacao(p).texto }}</span>
                </div>
                <p class="texto-suave">{{ p.diasDireito }} dias de direito · {{ p.diasAgendados }} agendados · {{ p.diasVendidos }} vendidos ·
                  <strong>{{ p.saldo }} para tirar</strong>@if (p.completo) {, até {{ p.fimConcessivo | date: 'dd/MM/yyyy' }}}.</p>
                @if (p.vencida) { <p class="aviso-inline">Venceu sem tirar: a empresa paga essas férias em dobro (CLT, art. 137).</p> }
                @if (p.vencendo) { <p class="aviso-inline">Tire até {{ p.fimConcessivo | date: 'dd/MM/yyyy' }}, ou elas vencem.</p> }
                <label class="faltas">Faltas sem justificativa no período
                  <input type="number" inputmode="numeric" min="0" max="366" [ngModel]="p.faltas" #f="ngModel"
                    [name]="'faltas-' + p.inicio" (change)="salvarFaltas(p, f.value)"></label>
                @for (a of p.agendamentos; track a.id) {
                  <div class="agendamento">
                    <div><strong>{{ a.inicio | date: 'dd/MM/yyyy' }}</strong>, {{ a.dias }} dias@if (a.diasVendidos) {, vende {{ a.diasVendidos }}}@if (a.adiantarDecimo) {, com o 13º}</div>
                    @if (a.calculo; as c) {
                      <div class="texto-suave">Recebe {{ c.liquido | currency }} até {{ c.pagamento | date: 'dd/MM' }}; volta em {{ c.retorno | date: 'dd/MM/yyyy' }}.</div>
                    }
                    <button type="button" class="botao botao-pequeno" (click)="desmarcar(a.id, a.inicio)">Desmarcar<span class="so-leitor"> as férias de {{ a.inicio | date: 'dd/MM/yyyy' }}</span></button>
                  </div>
                }
              </li>
            }
          </ul>
        </section>

        <section class="painel-bloco" aria-labelledby="titulo-agendar">
          <h2 id="titulo-agendar">Agendar</h2>
          @if (periodos().length) {
            <form class="formulario" (ngSubmit)="agendar()" #fa="ngForm">
              <label>Período
                <select name="periodo" [(ngModel)]="periodoInicio" (ngModelChange)="limparSimulacao()">
                  @for (p of periodos(); track p.inicio) {
                    <option [ngValue]="p.inicio" [disabled]="p.saldo === 0">{{ p.inicio | date: 'dd/MM/yyyy' }} a {{ p.fim | date: 'dd/MM/yyyy' }} ({{ p.saldo }} dias)</option>
                  }
                </select>
              </label>
              <label>Começa em <app-campo-data name="inicio" [(ngModel)]="inicio" obrigatorio (ngModelChange)="limparSimulacao()" /></label>
              <div class="dias">
                <label>Dias de férias <input name="dias" type="number" inputmode="numeric" min="5" max="30" [(ngModel)]="dias" required (ngModelChange)="limparSimulacao()"></label>
                <label>Dias vendidos <input name="vendidos" type="number" inputmode="numeric" min="0" max="10" [(ngModel)]="vendidos" (ngModelChange)="limparSimulacao()"></label>
              </div>
              <label class="caixa"><input type="checkbox" name="decimo" [(ngModel)]="adiantarDecimo" (ngModelChange)="limparSimulacao()"> Receber a 1ª parcela do 13º junto</label>
              <p class="texto-suave">Até 3 frações: uma de 14 dias ou mais, as outras de 5 ou mais. Vender até 1/3 (10 de 30 dias).</p>
              <div class="rodape-form">
                <button type="button" class="botao" [disabled]="!inicio || ocupado()" (click)="simular()">Ver a conta</button>
                <button type="button" class="botao" [disabled]="!inicio || ocupado()" (click)="comparar()">Comparar 30 dias × vender 10</button>
                <span class="cresce"></span>
                <button class="botao botao-primario" [disabled]="fa.invalid || !inicio || ocupado()">Agendar</button>
              </div>
            </form>
            @if (simulacao(); as c) {
              <div class="conta" aria-live="polite">
                <h3>A conta</h3>
                <dl class="resumo">
                  <dt>Férias ({{ c.dias }} dias)</dt><dd>{{ c.ferias | currency }}</dd>
                  <dt>1/3 das férias</dt><dd>{{ c.tercoFerias | currency }}</dd>
                  @if (c.diasVendidos) {
                    <dt>Abono ({{ c.diasVendidos }} dias vendidos)</dt><dd>{{ c.abono | currency }}</dd>
                    <dt>1/3 do abono</dt><dd>{{ c.tercoAbono | currency }}</dd>
                  }
                  <dt>INSS</dt><dd>− {{ c.inss.valor | currency }}</dd>
                  <dt>IRRF</dt><dd>− {{ c.irrf.valor | currency }}</dd>
                  @if (c.adiantamentoDecimo) { <dt>1ª parcela do 13º</dt><dd>{{ c.adiantamentoDecimo | currency }}</dd> }
                  <dt class="total">Cai na conta até {{ c.pagamento | date: 'dd/MM' }}</dt><dd class="total">{{ c.liquido | currency }}</dd>
                </dl>
                <p class="texto-suave">Remuneração de {{ c.remuneracao | currency }} (o salário e a média das horas extras dos 12 meses
                  antes: {{ c.mediaVariaveis | currency }}). O INSS e o IRRF são só sobre as férias e o 1/3; o abono e o 1/3 dele não têm
                  desconto. Volta ao trabalho em {{ c.retorno | date: 'dd/MM/yyyy' }}; o salário do mês seguinte vem menor, porque os dias de
                  férias já foram pagos.</p>
              </div>
            }
            @if (comparacao(); as cmp) {
              <div class="comparacao" aria-live="polite">
                <h3>30 dias × vender 10</h3>
                <table class="tabela">
                  <thead><tr><th scope="col"></th><th scope="col" class="direita">Tirar 30</th><th scope="col" class="direita">Tirar 20, vender 10</th></tr></thead>
                  <tbody>
                    <tr><th scope="row">Dias de descanso</th><td class="direita">30</td><td class="direita">20</td></tr>
                    <tr><th scope="row">Cai na conta</th><td class="direita">{{ cmp.trinta.liquido | currency }}</td><td class="direita">{{ cmp.vendendo.liquido | currency }}</td></tr>
                  </tbody>
                </table>
                <p class="texto-suave">Vendendo 10 dias, entram {{ cmp.diferenca | currency }} a mais agora, em troca de 10 dias de trabalho.</p>
              </div>
            }
          } @else {
            <p class="vazio">O emprego ainda não tem período aquisitivo.</p>
          }
        </section>
      </div>
    }
  `,
  styles: [`
    .periodos { list-style: none; margin: 0; padding: 0; display: grid; gap: 12px; }
    .periodo { border: 1px solid var(--linha); border-radius: 10px; padding: 12px; display: grid; gap: 6px; }
    .periodo p { margin: 0; }
    .periodo-topo { display: flex; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
    .tom-erro { background: var(--negativo-suave); color: var(--negativo); }
    .faltas input { width: 7em; }
    .agendamento { display: grid; gap: 4px; border-top: 1px solid var(--linha); padding-top: 6px; justify-items: start; }
    .dias { display: flex; gap: 12px; flex-wrap: wrap; }
    .dias input { width: 7em; }
    .caixa { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; }
    .resumo { display: grid; grid-template-columns: 1fr auto; row-gap: 6px; margin: 0; }
    .resumo dt { padding-right: 16px; }
    .resumo dd { margin: 0; text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .resumo .total { font-weight: 600; border-top: 1px solid var(--linha); padding-top: 6px; }
    .conta, .comparacao { margin-top: 16px; }
    .conta h3, .comparacao h3 { font-size: 1rem; margin: 0 0 8px; }
    .aviso-estimativa { margin-top: 0; }
  `]
})
export class TrabalhoFeriasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);

  readonly situacao = situacaoDoPeriodo;
  readonly carregando = signal(true);
  readonly ocupado = signal(false);
  readonly emprego = signal<Emprego | null>(null);
  readonly periodos = signal<PeriodoFerias[]>([]);
  readonly erro = signal<string | null>(null);
  readonly simulacao = signal<CalculoFerias | null>(null);
  readonly comparacao = signal<{ trinta: CalculoFerias; vendendo: CalculoFerias; diferenca: number } | null>(null);
  periodoInicio: string | null = null;
  inicio: string | null = null;
  dias = 30;
  vendidos = 0;
  adiantarDecimo = false;

  ngOnInit(): void {
    this.api.empregos().subscribe({
      next: (lista) => { this.emprego.set(empregoAtual(lista)); this.carregando.set(false); this.carregar(); },
      error: (e) => { this.carregando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  private mostrar(lista: PeriodoFerias[]): void {
    this.periodos.set(lista);
    this.erro.set(null);
    const p = periodoParaAgendar(lista);
    if (p && (!this.periodoInicio || !lista.some((x) => x.inicio === this.periodoInicio && x.saldo > 0))) {
      this.periodoInicio = p.inicio;
      this.inicio ??= primeiroDiaPossivel(p) > hoje() ? primeiroDiaPossivel(p) : null;
    }
  }

  private carregar(): void {
    const e = this.emprego();
    if (!e) return;
    this.api.ferias(e.id).subscribe({ next: (l) => this.mostrar(l), error: (err) => this.erro.set(mensagemDeErro(err)) });
  }

  limparSimulacao(): void {
    this.simulacao.set(null);
    this.comparacao.set(null);
  }

  simular(): void {
    const e = this.emprego();
    if (!e || !this.inicio) return;
    this.api.simularFerias(e.id, this.inicio, this.dias, this.vendidos ?? 0, this.adiantarDecimo).subscribe({
      next: (c) => { this.comparacao.set(null); this.simulacao.set(c); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  comparar(): void {
    const e = this.emprego();
    const inicio = this.inicio;
    if (!e || !inicio) return;
    this.api.simularFerias(e.id, inicio, 30, 0, this.adiantarDecimo).subscribe({
      next: (trinta) => this.api.simularFerias(e.id, inicio, 20, 10, this.adiantarDecimo).subscribe({
        next: (vendendo) => {
          this.simulacao.set(null);
          this.comparacao.set({ trinta, vendendo, diferenca: diferencaDaVenda(trinta.liquido, vendendo.liquido) });
        },
        error: (err) => this.avisos.erro(mensagemDeErro(err))
      }),
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  agendar(): void {
    const e = this.emprego();
    if (!e || !this.inicio || !this.periodoInicio) return;
    this.ocupado.set(true);
    this.api.agendarFerias(e.id, { periodoInicio: this.periodoInicio, inicio: this.inicio, dias: this.dias,
      diasVendidos: this.vendidos ?? 0, adiantarDecimo: this.adiantarDecimo }).subscribe({
      next: (l) => {
        this.ocupado.set(false);
        this.avisos.sucesso('Férias agendadas.');
        this.inicio = null;
        this.limparSimulacao();
        this.mostrar(l);
      },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  salvarFaltas(p: PeriodoFerias, valor: number | string | null): void {
    const e = this.emprego();
    const faltas = Number(valor);
    if (!e || !Number.isInteger(faltas) || faltas === p.faltas) return;
    this.api.definirFaltas(e.id, p.inicio, faltas).subscribe({
      next: (l) => { this.mostrar(l); this.avisos.sucesso('Faltas gravadas: os dias de direito foram recalculados.'); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  async desmarcar(id: number, inicio: string): Promise<void> {
    const e = this.emprego();
    if (!e) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Desmarcar férias',
      mensagem: `Desmarcar as férias que começam em ${inicio.split('-').reverse().join('/')}?`, confirmar: 'Desmarcar', perigo: true });
    if (!ok) return;
    this.api.desmarcarFerias(e.id, id).subscribe({
      next: (l) => { this.mostrar(l); this.avisos.sucesso('Férias desmarcadas.'); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }
}

function hoje(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
