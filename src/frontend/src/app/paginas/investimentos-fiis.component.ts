import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, computed, effect, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Fii, NOMES_ORIGEM_OPERACAO, OrigemOperacao, PainelFiis, TipoAtivo } from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

const CORES = ['#a7a3f5', '#92c8f0', '#8fd5b4', '#f8c09a', '#f3b3d3', '#8fd8d8', '#f2d98b', '#b5cf9a', '#d4a5e8',
  '#f4a89a', '#a9b4c6', '#f5a3b0'];
const COR_FII = '#a7a3f5';
const COR_SELIC = '#f8c09a';
const COR_IFIX = '#92c8f0';
const COR_POSITIVO = '#8fd5b4';
const COR_NEGATIVO = '#f5a3b0';
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const rotulo = (ano: number, mes: number) => `${MESES[mes - 1]}/${String(ano).slice(2)}`;
const compacto = (v: string | number) => Number(v).toLocaleString('pt-BR', { notation: 'compact' });

/** Aba de FIIs da página de investimentos (inspirada nos painéis de carteira de sites como o Investidor10). */
@Component({
  selector: 'app-investimentos-fiis',
  imports: [FormsModule, MoedaPipe, DatePipe, DecimalPipe, GraficoComponent],
  template: `
    @if (dados(); as d) {
      @if (!d.fiis.length) {
        <section class="painel-bloco">
          <p class="vazio">
            Nenhum FII na carteira ainda. Eles entram automaticamente quando a corretora está conectada no Meu Pluggy,
            ou use <strong>Importar do extrato</strong> / <strong>+ Lançar compra/venda</strong>.
          </p>
        </section>
      } @else {
        <section class="kpis">
          <article class="kpi kpi-destaque">
            <span>Patrimônio em FIIs</span>
            <strong>{{ d.resumo.valorMercado | currency }}</strong>
            <small>{{ d.resumo.quantidadeFiis }} fundos · custo {{ d.resumo.custo | currency }}
              ({{ pct(d.resumo.valorizacaoPercentual) }})</small>
          </article>
          <article class="kpi">
            <span>Renda mensal estimada</span>
            <strong class="positivo">{{ d.resumo.rendaMensalEstimada | currency }}</strong>
            <small>último rendimento de cada fundo × suas cotas</small>
          </article>
          <article class="kpi">
            <span>Renda em 12 meses</span>
            <strong>{{ d.resumo.renda12Meses | currency }}</strong>
            <small>média de {{ d.resumo.rendaMediaMensal | currency }}/mês
              @if (d.resumo.recebidoExtrato12Meses > 0) { · {{ d.resumo.recebidoExtrato12Meses | currency }} no extrato }</small>
          </article>
          <article class="kpi">
            <span>Dividend yield 12m</span>
            <strong>{{ pct(d.resumo.dy12Meses, false) }}</strong>
            <small>yield on cost (sobre o que você pagou): <strong>{{ pct(d.resumo.yieldOnCost12Meses, false) }}</strong></small>
          </article>
          <article class="kpi">
            <span>Rentabilidade (com proventos)</span>
            <strong [class.positivo]="(d.resumo.rentabilidadeTotal ?? 0) >= 0" [class.negativo]="(d.resumo.rentabilidadeTotal ?? 0) < 0">
              {{ pct(d.resumo.rentabilidadeTotal) }}</strong>
            <small>Selic {{ pct(d.resumo.rentabilidadeSelic) }}
              @if (d.resumo.rentabilidadeIfix !== null) { · IFIX {{ pct(d.resumo.rentabilidadeIfix) }} } no mesmo período</small>
          </article>
        </section>

        <div class="grade-2">
          <section class="painel-bloco">
            <div class="cabecalho-bloco">
              <h2>Proventos por mês</h2>
              <span class="cresce"></span>
              <div class="segmentado" role="group" aria-label="Período">
                <button type="button" [class.ativo]="mesesProventos() === 12" (click)="mesesProventos.set(12)">12 meses</button>
                <button type="button" [class.ativo]="mesesProventos() === 24" (click)="mesesProventos.set(24)">24 meses</button>
              </div>
            </div>
            <p class="texto-suave legenda-grafico">Pela data-com, com a quantidade de cotas que você tinha em cada data.</p>
            <app-grafico [configuracao]="graficoProventos()" [altura]="280" />
          </section>
          <section class="painel-bloco">
            <h2>Evolução do patrimônio</h2>
            <p class="texto-suave legenda-grafico">Valor aplicado (custo) x valor de mercado no fim de cada mês.</p>
            <app-grafico [configuracao]="graficoEvolucao()" [altura]="280" />
          </section>
        </div>

        <div class="grade-2">
          <section class="painel-bloco">
            <h2>Rentabilidade acumulada</h2>
            <p class="texto-suave legenda-grafico">Seus FIIs (cotação + proventos, ponderado pelos aportes) x Selic x IFIX.</p>
            <app-grafico [configuracao]="graficoRentabilidade()" [altura]="280" />
          </section>
          <section class="painel-bloco">
            <div class="cabecalho-bloco">
              <h2>Cotação x suas compras</h2>
              <span class="cresce"></span>
              <select [ngModel]="tickerHistorico()" (ngModelChange)="tickerHistorico.set($event)" aria-label="Fundo">
                @for (f of d.fiis; track f.id) { <option [ngValue]="f.ticker">{{ f.ticker }}</option> }
              </select>
            </div>
            <p class="texto-suave legenda-grafico">Fechamento mensal, preço pago em cada compra e seu preço médio.</p>
            <app-grafico [configuracao]="graficoHistorico()" [altura]="280" />
          </section>
        </div>

        <div class="grade-3">
          <section class="painel-bloco">
            <h2>Por segmento</h2>
            <app-grafico [configuracao]="graficoSegmentos()" [altura]="190" />
            <ul class="lista-legenda">
              @for (s of d.segmentos; track s.nome; let i = $index) {
                <li>
                  <span class="ponto" [style.background]="cor(i)"></span>
                  <span class="cresce">{{ s.nome }}</span>
                  <span class="texto-suave pct">{{ d.resumo.valorMercado > 0 ? (s.valor / d.resumo.valorMercado * 100 | number: '1.0-1') : 0 }}%</span>
                </li>
              }
            </ul>
          </section>
          <section class="painel-bloco">
            <h2>Yield on cost x DY atual (12m)</h2>
            <app-grafico [configuracao]="graficoYield()" [altura]="alturaPorFundo()" />
          </section>
          <section class="painel-bloco">
            <h2>Preço médio x cotação</h2>
            <app-grafico [configuracao]="graficoPreco()" [altura]="alturaPorFundo()" />
          </section>
        </div>

        <section class="painel-bloco">
          <h2>Meus FIIs</h2>
          <div class="tabela-rolagem">
            <table class="tabela tabela-fiis">
              <thead>
                <tr>
                  <th>Fundo</th><th>Segmento</th><th class="direita">Cotas</th><th class="direita">Preço médio</th>
                  <th class="direita">Cotação</th><th class="direita">Valorização</th><th class="direita">Último rend.</th>
                  <th class="direita">DY mês / 12m</th><th class="direita">Yield on cost</th><th class="direita">Renda/mês</th>
                  <th class="direita">Peso</th>
                </tr>
              </thead>
              <tbody>
                @for (f of d.fiis; track f.id) {
                  <tr>
                    <td><strong>{{ f.ticker }}</strong>@if (f.nome) { <br><small class="texto-suave">{{ f.nome }}</small> }</td>
                    <td>
                      <select class="select-compacto" [ngModel]="f.segmento" (ngModelChange)="mudarSegmento(f, $event)"
                        [attr.aria-label]="'Segmento de ' + f.ticker">
                        <option [ngValue]="null">—</option>
                        @for (s of d.segmentosDisponiveis; track s) { <option [ngValue]="s">{{ s }}</option> }
                      </select>
                      @if (f.segmentoSugerido) { <br><small class="texto-suave">sugerido</small> }
                    </td>
                    <td class="direita">{{ qtd(f.quantidade) }}</td>
                    <td class="direita">{{ f.precoMedio | currency }}</td>
                    <td class="direita">{{ f.precoAtual !== null ? (f.precoAtual | currency) : '—' }}
                      @if (f.variacaoDia !== null && !f.precoManual) {
                        <br><small [class.positivo]="f.variacaoDia >= 0" [class.negativo]="f.variacaoDia < 0">{{ pct(f.variacaoDia) }} hoje</small>
                      }</td>
                    <td class="direita" [class.positivo]="f.valorizacao >= 0" [class.negativo]="f.valorizacao < 0">
                      {{ pct(f.valorizacaoPercentual) }}<br><small>{{ f.valorizacao | currency }}</small></td>
                    <td class="direita">{{ f.ultimoRendimento !== null ? (f.ultimoRendimento | currency: 'BRL' : 'symbol' : '1.2-4') : '—' }}
                      @if (f.dataComUltimo) { <br><small class="texto-suave">data-com {{ f.dataComUltimo | date: 'dd/MM' }}</small> }</td>
                    <td class="direita">{{ pct(f.dyMensal, false) }}<br><small class="texto-suave">{{ pct(f.dy12Meses, false) }}</small></td>
                    <td class="direita"><strong>{{ pct(f.yieldOnCost12Meses, false) }}</strong>
                      <br><small class="texto-suave">{{ pct(f.yieldOnCostMensal, false) }} ao mês</small></td>
                    <td class="direita positivo">{{ f.rendaMensalEstimada | currency }}</td>
                    <td class="direita">{{ f.pesoCarteira | number: '1.0-1' }}%</td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
          <p class="texto-suave nota">
            DY = rendimentos por cota ÷ cotação atual. Yield on cost = os mesmos rendimentos ÷ seu preço médio, ou seja, quanto o
            fundo rende sobre o que você pagou.
          </p>
        </section>

        <section class="painel-bloco">
          <div class="cabecalho-bloco">
            <h2>Cada compra x hoje</h2>
            <span class="cresce"></span>
            <select [ngModel]="filtroLote()" (ngModelChange)="filtroLote.set($event)" aria-label="Filtrar fundo">
              <option [ngValue]="null">Todos os fundos</option>
              @for (f of d.fiis; track f.id) { <option [ngValue]="f.ticker">{{ f.ticker }}</option> }
            </select>
          </div>
          <div class="tabela-rolagem">
            <table class="tabela tabela-fiis">
              <thead>
                <tr>
                  <th>Data</th><th>Fundo</th><th class="direita">Cotas</th><th class="direita">Preço pago</th>
                  <th class="direita">Cotação hoje</th><th class="direita">Variação</th>
                  <th class="direita" title="Rendimentos dos 12 meses antes da compra ÷ preço pago">DY na compra</th>
                  <th class="direita" title="Rendimentos dos últimos 12 meses ÷ preço pago">Yield on cost hoje</th>
                  <th class="direita" title="Rendimentos dos últimos 12 meses ÷ cotação atual">DY atual</th>
                  <th class="direita">Renda/mês do lote</th><th class="direita">Já rendeu</th><th>Origem</th>
                </tr>
              </thead>
              <tbody>
                @for (l of lotesFiltrados(); track l.operacaoId) {
                  <tr>
                    <td>{{ l.data | date: 'dd/MM/yyyy' }}</td>
                    <td><strong>{{ l.ticker }}</strong></td>
                    <td class="direita">{{ qtd(l.quantidade) }}</td>
                    <td class="direita">{{ l.precoPago | currency: 'BRL' : 'symbol' : '1.2-4' }}<br>
                      <small class="texto-suave">{{ l.valorPago | currency }}</small></td>
                    <td class="direita">{{ l.precoAtual !== null ? (l.precoAtual | currency) : '—' }}
                      @if (l.valorAtual !== null) { <br><small class="texto-suave">{{ l.valorAtual | currency }}</small> }</td>
                    <td class="direita" [class.positivo]="(l.variacaoPercentual ?? 0) >= 0" [class.negativo]="(l.variacaoPercentual ?? 0) < 0">
                      {{ pct(l.variacaoPercentual) }}</td>
                    <td class="direita">{{ pct(l.dyNaCompra, false) }}</td>
                    <td class="direita"><strong [class.positivo]="(l.yieldOnCost12Meses ?? 0) >= (l.dyNaCompra ?? 0)"
                      [class.negativo]="l.dyNaCompra !== null && (l.yieldOnCost12Meses ?? 0) < l.dyNaCompra">{{ pct(l.yieldOnCost12Meses, false) }}</strong></td>
                    <td class="direita">{{ pct(l.dyAtual12Meses, false) }}</td>
                    <td class="direita">{{ l.rendaMensalLote | currency }}</td>
                    <td class="direita positivo">{{ l.proventosRecebidosLote | currency }}</td>
                    <td><span class="etiqueta etiqueta-suave">{{ nomeOrigem(l.origem) }}</span></td>
                  </tr>
                } @empty {
                  <tr><td colspan="12" class="vazio">Nenhuma compra.</td></tr>
                }
              </tbody>
            </table>
          </div>
          <p class="texto-suave nota">
            "DY na compra" mostra quanto o fundo pagava, nos 12 meses anteriores, sobre o preço que você pagou. Comparado ao
            "Yield on cost hoje", ele mostra se a renda dessa compra aumentou ou caiu.
          </p>
        </section>
      }
    } @else {
      <p class="vazio">Carregando FIIs…</p>
    }
  `,
  styles: `
    .legenda-grafico { margin: -6px 0 10px; font-size: 0.84rem; }
    .cabecalho-bloco { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; flex-wrap: wrap; }
    .cabecalho-bloco h2 { margin: 0; }
    .tabela-fiis td { white-space: nowrap; }
    .tabela-fiis td:first-child, .tabela-fiis td:nth-child(2) { white-space: normal; }
    .select-compacto { padding: 4px 8px; font-size: 0.85rem; min-width: 130px; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
  `
})
export class InvestimentosFiisComponent {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);

  /** Incrementado pela página quando a carteira muda, para recarregar. */
  readonly versao = input(0);
  readonly alterado = output<void>();

  readonly dados = signal<PainelFiis | null>(null);
  readonly mesesProventos = signal<12 | 24>(12);
  readonly tickerHistorico = signal<string | null>(null);
  readonly filtroLote = signal<string | null>(null);

  constructor() {
    effect(() => {
      this.versao();
      this.carregar();
    });
  }

  carregar(): void {
    this.api.fiis().subscribe({
      next: (d) => {
        this.dados.set(d);
        const atual = this.tickerHistorico();
        if (!atual || !d.fiis.some((f) => f.ticker === atual)) this.tickerHistorico.set(d.fiis[0]?.ticker ?? null);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  readonly lotesFiltrados = computed(() => {
    const filtro = this.filtroLote();
    const lotes = this.dados()?.lotes ?? [];
    return filtro ? lotes.filter((l) => l.ticker === filtro) : lotes;
  });

  readonly alturaPorFundo = computed(() => Math.max(190, (this.dados()?.fiis.length ?? 0) * 34 + 60));

  readonly graficoProventos = computed<ChartConfiguration<'bar'>>(() => {
    const todos = this.dados()?.proventos ?? [];
    const meses = todos.slice(-this.mesesProventos());
    const tickers = [...new Set(meses.flatMap((m) => m.porAtivo.map((p) => p.ticker)))];
    const media = meses.length ? meses.reduce((s, m) => s + m.total, 0) / meses.length : 0;
    const temExtrato = meses.some((m) => m.recebidoExtrato > 0);
    const datasets: ChartConfiguration<'bar'>['data']['datasets'] = tickers.map((t, i) => ({
      label: t,
      data: meses.map((m) => m.porAtivo.find((p) => p.ticker === t)?.valor ?? 0),
      backgroundColor: this.cor(i), stack: 'fiis', borderRadius: 4
    }));
    datasets.push({
      label: 'Média', data: meses.map(() => Math.round(media * 100) / 100), type: 'line' as never, stack: 'media',
      borderColor: '#6b7280', borderDash: [5, 5], borderWidth: 1.5, pointRadius: 0
    } as never);
    if (temExtrato) {
      datasets.push({
        label: 'Recebido no extrato', data: meses.map((m) => m.recebidoExtrato), type: 'line' as never, stack: 'extrato',
        borderColor: COR_POSITIVO, backgroundColor: COR_POSITIVO, borderWidth: 2, pointRadius: 3, tension: 0.3
      } as never);
    }
    return {
      type: 'bar',
      data: { labels: meses.map((m) => rotulo(m.ano, m.mes)), datasets },
      options: {
        interaction: { mode: 'index', intersect: false },
        plugins: { legend: { position: 'bottom', labels: { boxWidth: 10 } } },
        scales: { x: { stacked: true }, y: { stacked: true, ticks: { callback: compacto } } }
      }
    };
  });

  readonly graficoEvolucao = computed<ChartConfiguration<'bar'>>(() => {
    const ev = this.dados()?.evolucao ?? [];
    return {
      type: 'bar',
      data: {
        labels: ev.map((m) => rotulo(m.ano, m.mes)),
        datasets: [
          { label: 'Aplicado', data: ev.map((m) => m.aplicado), backgroundColor: '#d9d7fb', borderRadius: 4 },
          { label: 'Valor de mercado', data: ev.map((m) => m.valorMercado), backgroundColor: COR_FII, borderRadius: 4 }
        ]
      },
      options: {
        interaction: { mode: 'index', intersect: false },
        plugins: { legend: { position: 'bottom' } },
        scales: { y: { ticks: { callback: compacto } } }
      }
    };
  });

  readonly graficoRentabilidade = computed<ChartConfiguration<'line'>>(() => {
    const r = this.dados()?.rentabilidade ?? [];
    const datasets: ChartConfiguration<'line'>['data']['datasets'] = [
      { label: 'Meus FIIs', data: r.map((m) => m.carteira), borderColor: COR_FII, backgroundColor: COR_FII + '33',
        fill: true, tension: 0.3, borderWidth: 2.5, pointRadius: 0 },
      { label: 'Selic', data: r.map((m) => m.selic), borderColor: COR_SELIC, borderDash: [6, 5], tension: 0.3,
        borderWidth: 2, pointRadius: 0 }
    ];
    if (r.some((m) => m.ifix !== null)) {
      datasets.push({ label: 'IFIX', data: r.map((m) => m.ifix), borderColor: COR_IFIX, tension: 0.3, borderWidth: 2,
        pointRadius: 0, spanGaps: true });
    }
    return {
      type: 'line',
      data: { labels: r.map((m) => rotulo(m.ano, m.mes)), datasets },
      options: {
        interaction: { mode: 'index', intersect: false },
        plugins: { legend: { position: 'bottom' } },
        scales: { y: { ticks: { callback: (v) => `${v}%` } } }
      }
    };
  });

  readonly graficoHistorico = computed<ChartConfiguration<'line'>>(() => {
    const d = this.dados();
    const ticker = this.tickerHistorico();
    const historico = d?.historicos.find((h) => h.ticker === ticker)?.pontos ?? [];
    const fii = d?.fiis.find((f) => f.ticker === ticker);
    const chaves = historico.map((p) => `${p.ano}-${p.mes}`);
    // Preço médio pago em cada mês com compra.
    const compras = new Map<string, { soma: number; qtd: number }>();
    for (const l of d?.lotes ?? []) {
      if (l.ticker !== ticker) continue;
      const [a, m] = l.data.split('-').map(Number);
      const chave = `${a}-${m}`;
      const atual = compras.get(chave) ?? { soma: 0, qtd: 0 };
      compras.set(chave, { soma: atual.soma + l.precoPago * l.quantidade, qtd: atual.qtd + l.quantidade });
    }
    return {
      type: 'line',
      data: {
        labels: historico.map((p) => rotulo(p.ano, p.mes)),
        datasets: [
          { label: 'Cotação', data: historico.map((p) => p.fechamento), borderColor: COR_FII, backgroundColor: COR_FII + '22',
            fill: true, tension: 0.25, borderWidth: 2, pointRadius: 0 },
          { label: 'Suas compras', data: chaves.map((c) => { const x = compras.get(c); return x ? x.soma / x.qtd : null; }),
            borderColor: '#0f9f6e', backgroundColor: COR_POSITIVO, pointRadius: 6, pointHoverRadius: 8, showLine: false },
          { label: 'Preço médio', data: historico.map(() => fii?.precoMedio ?? null), borderColor: COR_SELIC,
            borderDash: [6, 5], borderWidth: 1.5, pointRadius: 0 }
        ]
      },
      options: {
        interaction: { mode: 'index', intersect: false },
        plugins: { legend: { position: 'bottom' } },
        scales: { y: { ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { minimumFractionDigits: 2 }) } } }
      }
    };
  });

  readonly graficoSegmentos = computed<ChartConfiguration<'doughnut'>>(() => {
    const s = this.dados()?.segmentos ?? [];
    return {
      type: 'doughnut',
      data: { labels: s.map((x) => x.nome), datasets: [{ data: s.map((x) => x.valor), backgroundColor: s.map((_, i) => this.cor(i)), borderWidth: 0 }] },
      options: { cutout: '70%', plugins: { legend: { display: false } } }
    };
  });

  readonly graficoYield = computed<ChartConfiguration<'bar'>>(() => {
    const f = this.dados()?.fiis ?? [];
    return {
      type: 'bar',
      data: {
        labels: f.map((x) => x.ticker),
        datasets: [
          { label: 'Yield on cost', data: f.map((x) => x.yieldOnCost12Meses ?? 0),
            backgroundColor: f.map((x) => ((x.yieldOnCost12Meses ?? 0) >= (x.dy12Meses ?? 0) ? COR_POSITIVO : COR_NEGATIVO)), borderRadius: 4 },
          { label: 'DY atual', data: f.map((x) => x.dy12Meses ?? 0), backgroundColor: COR_IFIX, borderRadius: 4 }
        ]
      },
      options: { indexAxis: 'y', plugins: { legend: { position: 'bottom' } }, scales: { x: { ticks: { callback: (v) => `${v}%` } } } }
    };
  });

  readonly graficoPreco = computed<ChartConfiguration<'bar'>>(() => {
    const f = this.dados()?.fiis ?? [];
    return {
      type: 'bar',
      data: {
        labels: f.map((x) => x.ticker),
        datasets: [
          { label: 'Preço médio', data: f.map((x) => x.precoMedio), backgroundColor: '#d9d7fb', borderRadius: 4 },
          { label: 'Cotação', data: f.map((x) => x.precoAtual ?? 0),
            backgroundColor: f.map((x) => ((x.precoAtual ?? 0) >= x.precoMedio ? COR_POSITIVO : COR_NEGATIVO)), borderRadius: 4 }
        ]
      },
      options: { indexAxis: 'y', plugins: { legend: { position: 'bottom' } } }
    };
  });

  mudarSegmento(f: Fii, segmento: string | null): void {
    this.api.atualizarAtivo(f.id, {
      nome: f.nome, tipo: TipoAtivo.Fii, segmento, precoManual: f.precoManual, precoAtual: f.precoManual ? f.precoAtual : null
    }).subscribe({
      next: () => {
        this.avisos.sucesso(`Segmento de ${f.ticker} atualizado.`);
        this.alterado.emit();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  pct(valor: number | null, sinal = true): string {
    if (valor === null || valor === undefined) return '—';
    const texto = valor.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    return `${sinal && valor > 0 ? '+' : ''}${texto}%`;
  }

  qtd(valor: number): string {
    if (this.privacidade.oculto()) return '•••';
    return valor.toLocaleString('pt-BR', { maximumFractionDigits: 6 });
  }

  nomeOrigem(o: OrigemOperacao): string {
    return NOMES_ORIGEM_OPERACAO[o] ?? '';
  }

  cor(i: number): string {
    return CORES[i % CORES.length];
  }
}
