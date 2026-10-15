import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ChartConfiguration } from 'chart.js';
import { forkJoin } from 'rxjs';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { MesValores, TipoTransacao, Transacao, ValorCategoria } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { NOMES_MESES_CURTOS, dataIso, primeiroDiaMes } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

@Component({
  selector: 'app-relatorios',
  imports: [FormsModule, MoedaPipe, DatePipe, DecimalPipe, GraficoComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Relatórios</h1>
      <div class="acoes">
        <div class="segmentado" role="group" aria-label="Período rápido">
          <button type="button" (click)="rapido(3)">3 meses</button>
          <button type="button" (click)="rapido(6)">6 meses</button>
          <button type="button" (click)="rapido(12)">12 meses</button>
          <button type="button" (click)="anoAtual()">Este ano</button>
        </div>
      </div>
    </header>

    <section class="filtros">
      <label>De <input type="date" [(ngModel)]="inicio" (change)="carregar()"></label>
      <label>Até <input type="date" [(ngModel)]="fim" (change)="carregar()"></label>
      <label>Visão
        <select [(ngModel)]="tipo" (change)="carregar()">
          <option [ngValue]="TipoTransacao.Despesa">Despesas</option>
          <option [ngValue]="TipoTransacao.Receita">Receitas</option>
        </select>
      </label>
      <label class="checkbox"><input type="checkbox" [(ngModel)]="agrupar" (change)="carregar()"> agrupar subcategorias</label>
    </section>

    <section class="kpis">
      <article class="kpi"><span>Receitas</span><strong class="positivo">{{ totalReceitas() | currency }}</strong></article>
      <article class="kpi"><span>Despesas</span><strong class="negativo">{{ totalDespesas() | currency }}</strong></article>
      <article class="kpi"><span>Saldo do período</span>
        <strong [class.positivo]="totalReceitas() >= totalDespesas()" [class.negativo]="totalReceitas() < totalDespesas()">
          {{ totalReceitas() - totalDespesas() | currency }}</strong></article>
      <article class="kpi"><span>Média mensal de despesas</span><strong>{{ mediaDespesas() | currency }}</strong></article>
      <article class="kpi"><span>Emergências</span>
        <strong class="texto-emergencia">{{ totalEmergencia() | currency }}</strong>
        <small>{{ totalDespesas() > 0 ? (totalEmergencia() / totalDespesas() * 100 | number: '1.0-1') : 0 }}% das despesas
          @if (mesesPorEmergencia() > 0) { · {{ mesesPorEmergencia() }} mês(es) no vermelho só por emergência }</small>
      </article>
    </section>

    <section class="painel-bloco">
      <h2>Fluxo de caixa mensal</h2>
      <p class="texto-suave legenda-relatorio">Em <span class="texto-emergencia">laranja</span>, as despesas marcadas como emergência.</p>
      <app-grafico [configuracao]="graficoFluxo()" [altura]="300" />
      <div class="tabela-rolagem">
        <table class="tabela tabela-meses">
          <thead>
            <tr><th>Mês</th><th class="direita">Receitas</th><th class="direita">Despesas</th>
              <th class="direita">Emergências</th><th class="direita">Resultado</th><th class="direita">Sem emergências</th></tr>
          </thead>
          <tbody>
            @for (m of fluxo(); track m.ano * 100 + m.mes) {
              <tr [class.linha-emergencia]="situacaoMes(m) === 'emergencia'" [class.linha-negativa]="situacaoMes(m) === 'negativo'">
                <td>{{ nomeMes(m) }}
                  @if (situacaoMes(m) === 'emergencia') { <span class="etiqueta etiqueta-emergencia">no vermelho por emergência</span> }
                </td>
                <td class="direita positivo">{{ m.receitas | currency }}</td>
                <td class="direita negativo">{{ m.despesas | currency }}</td>
                <td class="direita texto-emergencia">{{ m.despesasEmergencia > 0 ? (m.despesasEmergencia | currency) : '—' }}</td>
                <td class="direita valor" [class.positivo]="m.receitas - m.despesas >= 0"
                  [class.negativo]="situacaoMes(m) === 'negativo'" [class.texto-emergencia]="situacaoMes(m) === 'emergencia'">
                  {{ m.receitas - m.despesas | currency }}</td>
                <td class="direita texto-suave">{{ m.receitas - m.despesas + m.despesasEmergencia | currency }}</td>
              </tr>
            }
          </tbody>
        </table>
      </div>
    </section>

    <div class="grade-2">
      <section class="painel-bloco">
        <h2>{{ tipo === TipoTransacao.Despesa ? 'Despesas' : 'Receitas' }} por categoria</h2>
        <div class="tabela-rolagem">
          <table class="tabela">
            <tbody>
              @for (c of categorias(); track c.nome) {
                <tr>
                  <td><span class="chip"><span class="ponto" [style.background]="c.cor"></span>{{ c.nome }}</span></td>
                  <td class="direita valor">{{ c.valor | currency }}</td>
                  <td class="direita texto-suave">{{ c.valor / totalCategorias() * 100 | number: '1.1-1' }}%</td>
                  <td style="width: 35%"><div class="trilho"><div class="progresso" [style.background]="c.cor"
                    [style.width.%]="c.valor / maiorCategoria() * 100"></div></div></td>
                </tr>
              } @empty { <tr><td class="vazio">Sem dados no período.</td></tr> }
            </tbody>
          </table>
        </div>
      </section>

      <section class="painel-bloco">
        <h2>Maiores despesas</h2>
        <ul class="lista-simples">
          @for (t of maiores(); track t.id) {
            <li>
              <span class="data-curta">{{ t.data | date: 'dd/MM/yy' }}</span>
              <span class="cresce truncar" [title]="t.descricao">{{ t.descricao }}
                @if (t.emergencia) { <span class="etiqueta etiqueta-emergencia">emergência</span> }</span>
              <span class="valor negativo">{{ t.valor | currency }}</span>
            </li>
          } @empty { <li class="vazio">Sem despesas no período.</li> }
        </ul>
      </section>
    </div>
  `,
  styles: `
    .legenda-relatorio { margin: -6px 0 10px; font-size: 0.84rem; }
    .tabela-meses { margin-top: 14px; }
    .tabela-meses tr.linha-emergencia td { background: var(--emergencia-suave); }
    .tabela-meses tr.linha-negativa td:first-child { box-shadow: inset 3px 0 0 var(--negativo); }
    .tabela-meses tr.linha-emergencia td:first-child { box-shadow: inset 3px 0 0 var(--emergencia); }
  `
})
export class RelatoriosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly TipoTransacao = TipoTransacao;
  readonly fluxo = signal<MesValores[]>([]);
  readonly categorias = signal<ValorCategoria[]>([]);
  readonly maiores = signal<Transacao[]>([]);

  readonly totalReceitas = computed(() => this.fluxo().reduce((s, m) => s + m.receitas, 0));
  readonly totalDespesas = computed(() => this.fluxo().reduce((s, m) => s + m.despesas, 0));
  readonly totalEmergencia = computed(() => this.fluxo().reduce((s, m) => s + (m.despesasEmergencia ?? 0), 0));
  readonly mesesPorEmergencia = computed(() => this.fluxo().filter((m) => this.situacaoMes(m) === 'emergencia').length);
  readonly mediaDespesas = computed(() => (this.fluxo().length ? this.totalDespesas() / this.fluxo().length : 0));
  readonly totalCategorias = computed(() => this.categorias().reduce((s, c) => s + c.valor, 0) || 1);
  readonly maiorCategoria = computed(() => Math.max(1, ...this.categorias().map((c) => c.valor)));

  readonly graficoFluxo = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.fluxo();
    let acumulado = 0;
    const saldoAcumulado = meses.map((m) => (acumulado += m.receitas - m.despesas));
    return {
      type: 'bar',
      data: {
        labels: meses.map((m) => `${NOMES_MESES_CURTOS[m.mes - 1]}/${String(m.ano).slice(2)}`),
        datasets: [
          { label: 'Receitas', data: meses.map((m) => m.receitas), backgroundColor: '#8fd5b4', borderRadius: 8, borderSkipped: false,
            stack: 'receitas' },
          { label: 'Despesas', data: meses.map((m) => m.despesas - (m.despesasEmergencia ?? 0)), backgroundColor: '#f5a3b0',
            borderRadius: 8, borderSkipped: false, stack: 'despesas' },
          { label: 'Emergências', data: meses.map((m) => m.despesasEmergencia ?? 0), backgroundColor: '#f6a467',
            borderRadius: 8, borderSkipped: false, stack: 'despesas' },
          // Linha sobreposta do saldo acumulado
          { label: 'Saldo acumulado', data: saldoAcumulado, type: 'line' as never, borderColor: '#a7a3f5',
            backgroundColor: '#a7a3f5', borderWidth: 3, pointRadius: 0, tension: 0.35, stack: 'saldo' } as never
        ]
      },
      options: {
        plugins: { legend: { position: 'bottom' } },
        interaction: { mode: 'index', intersect: false },
        scales: {
          x: { stacked: true },
          y: { stacked: true, ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { notation: 'compact' }) } }
        }
      }
    };
  });

  /** "emergencia": mês no vermelho que ficaria positivo sem os gastos de emergência. */
  situacaoMes(m: MesValores): 'ok' | 'emergencia' | 'negativo' {
    const resultado = m.receitas - m.despesas;
    if (resultado >= 0) return 'ok';
    return resultado + (m.despesasEmergencia ?? 0) >= 0 ? 'emergencia' : 'negativo';
  }

  nomeMes(m: MesValores): string {
    return `${NOMES_MESES_CURTOS[m.mes - 1]}/${m.ano}`;
  }

  inicio = '';
  fim = '';
  tipo: TipoTransacao = TipoTransacao.Despesa;
  agrupar = true;

  ngOnInit(): void {
    this.rapido(6);
  }

  rapido(meses: number): void {
    const hoje = new Date();
    const inicio = new Date(hoje.getFullYear(), hoje.getMonth() - (meses - 1), 1);
    this.inicio = primeiroDiaMes(inicio.getFullYear(), inicio.getMonth() + 1);
    this.fim = dataIso(hoje);
    this.carregar();
  }

  anoAtual(): void {
    const hoje = new Date();
    this.inicio = `${hoje.getFullYear()}-01-01`;
    this.fim = dataIso(hoje);
    this.carregar();
  }

  carregar(): void {
    if (!this.inicio || !this.fim) return;
    forkJoin({
      fluxo: this.api.relatorioFluxo(this.inicio, this.fim),
      categorias: this.api.relatorioCategorias(this.inicio, this.fim, this.tipo, this.agrupar),
      maiores: this.api.maioresDespesas(this.inicio, this.fim)
    }).subscribe({
      next: (r) => {
        this.fluxo.set(r.fluxo);
        this.categorias.set(r.categorias);
        this.maiores.set(r.maiores);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
