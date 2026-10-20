import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, computed, effect, inject, input, output, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Fii, NOMES_ORIGEM_OPERACAO, OrigemOperacao, PainelFiis, TipoAtivo } from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { CORES_GRAFICO } from '../compartilhado/cores-graficos';
import { GraficoExplicadoComponent } from '../compartilhado/grafico-explicado.component';
import { TermoComponent } from '../compartilhado/termo.component';
import { MaisAnalisesComponent } from '../compartilhado/mais-analises.component';
import { pct, reaisOuNada } from '../compartilhado/utilidades';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';

// Paleta dos fundos: sem o verde/rosa de ganhou/perdeu nem o laranja da Selic, que têm significado (UX-9).
const CORES = ['#a7a3f5', '#92c8f0', '#c3b1e1', '#8fd8d8', '#f3b3d3', '#f2d98b', '#b5cf9a', '#d4a5e8', '#a9b4c6',
  '#b8c7f0', '#e6c9a8', '#9fb8c8'];
const COR_FII = '#a7a3f5';
const COR_SELIC = CORES_GRAFICO.selic;
const COR_IFIX = '#92c8f0';
const COR_POSITIVO = CORES_GRAFICO.ganhou;
const COR_NEGATIVO = CORES_GRAFICO.perdeu;
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const rotulo = (ano: number, mes: number) => `${MESES[mes - 1]}/${String(ano).slice(2)}`;
const compacto = (v: string | number) => Number(v).toLocaleString('pt-BR', { notation: 'compact' });

/** Aba de FIIs da página de investimentos (inspirada nos painéis de carteira de sites como o Investidor10). */
@Component({
  selector: 'app-investimentos-fiis',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, DatePipe, DecimalPipe, GraficoComponent, GraficoExplicadoComponent, TermoComponent, MaisAnalisesComponent],
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
            <app-grafico-explicado [resumo]="resumoProventos()"
              comoLer="Proventos de cada mês pela data-com, com as cotas que você tinha em cada data; cada cor é um fundo. A linha tracejada é a média do período.">
              <app-grafico [configuracao]="graficoProventos()" [altura]="280" />
            </app-grafico-explicado>
          </section>
          <section class="painel-bloco">
            <h2>Rentabilidade acumulada</h2>
            <app-grafico-explicado [resumo]="resumoRentabilidade()"
              comoLer="Rentabilidade acumulada em %: área lilás, seus FIIs (cotação + proventos, ponderado pelos aportes); tracejado, a Selic; azul, o IFIX.">
              <app-grafico [configuracao]="graficoRentabilidade()" [altura]="280" />
            </app-grafico-explicado>
          </section>
        </div>

        @if (mesAMes().length) {
          <section class="painel-bloco mes-a-mes" aria-labelledby="titulo-mes-a-mes">
            <div class="cabecalho-bloco">
              <h2 id="titulo-mes-a-mes">Mês a mês</h2>
              <span class="cresce"></span>
              <div class="segmentado" role="group" aria-label="Período do mês a mês">
                <button type="button" [class.ativo]="mesesMesAMes() === 12" [attr.aria-pressed]="mesesMesAMes() === 12"
                  (click)="mesesMesAMes.set(12)">12 meses</button>
                <button type="button" [class.ativo]="mesesMesAMes() === 24" [attr.aria-pressed]="mesesMesAMes() === 24"
                  (click)="mesesMesAMes.set(24)">24 meses</button>
              </div>
            </div>
            <app-grafico-explicado [resumo]="resumoMesAMes()"
              comoLer="O que a carteira rendeu em cada mês: em azul, os rendimentos que caíram na conta; a valorização das cotas, em verde quando subiram e em rosa quando caíram. Os aportes estão na tabela abaixo.">
              <app-grafico [configuracao]="graficoMesAMes()" [altura]="260" />
            </app-grafico-explicado>
            <div class="tabela-rolagem">
              <table class="tabela tabela-fiis tabela-mes-a-mes">
                <caption class="so-leitor">Aportes, rendimentos, valorização e saldo dos FIIs em cada mês, do mais recente ao mais antigo</caption>
                <thead>
                  <tr>
                    <th scope="col">Mês</th>
                    <th scope="col" class="direita">Aportes</th>
                    <th scope="col" class="direita">Rendimentos</th>
                    <th scope="col" class="direita"><app-termo chave="valorizacaoMes">Valorização</app-termo></th>
                    <th scope="col" class="direita">Saldo no fim</th>
                  </tr>
                </thead>
                <tbody>
                  @for (m of mesAMesRecentes(); track m.ano * 100 + m.mes; let primeiro = $first) {
                    <tr>
                      <th scope="row">{{ rotuloMes(m) }}@if (primeiro && mesAtual(m)) { <br><small class="texto-suave">até hoje</small> }</th>
                      <td class="direita">{{ m.aportes | currency }}
                        @if (m.subscricoes > 0) { <br><small class="texto-suave">+ {{ m.subscricoes | currency }} em subscrição</small> }
                        @if (m.resgates > 0) { <br><small class="texto-suave">− {{ m.resgates | currency }} em vendas</small> }</td>
                      <td class="direita positivo">{{ m.rendimentosRecebidos | currency }}
                        @if (mesAtual(m) && m.rendimentosEstimados > 0) {
                          <br><small class="texto-suave">{{ m.rendimentosEstimados | currency }} a receber (data-com)</small>
                        }</td>
                      <td class="direita" [class.positivo]="m.valorizacao >= 0" [class.negativo]="m.valorizacao < 0">{{ m.valorizacao | currency }}</td>
                      <td class="direita">{{ m.saldo | currency }}</td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
            <p class="texto-suave nota">
              Aportes são as compras (cotas × preço); a taxa da B3, de centavos por ordem, só entra quando foi lançada.
              Rendimentos são os créditos de provento no extrato, sem o troco das compras nem o valor das vendas.
              A valorização usa o fechamento de cada mês (no mês atual, a cotação de hoje); por isso pode diferir um pouco
              do app da corretora, que usa outra fonte e outro dia de preço.
            </p>
          </section>
        }

        <section class="painel-bloco">
          <h2>Meus FIIs</h2>
          <div class="tabela-rolagem">
            <table class="tabela tabela-fiis">
              <thead>
                <tr>
                  <th>Fundo</th><th>Segmento</th><th class="direita">Cotas</th><th class="direita"><app-termo chave="precoMedio">Preço médio</app-termo></th>
                  <th class="direita">Cotação</th><th class="direita">Valorização</th><th class="direita">Último rend.</th>
                  <th class="direita"><app-termo chave="dy">DY</app-termo> mês / 12m</th><th class="direita"><app-termo chave="yieldOnCost">Yield on cost</app-termo></th><th class="direita">Renda/mês</th>
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

        <app-mais-analises chave="fiis" conteudo="evolução do patrimônio, cotação x compras, segmentos, yield on cost x DY, preço médio e cada compra">
          <ng-template>
            <div class="grade-2">
              <section class="painel-bloco">
                <h2>Evolução do patrimônio</h2>
                <app-grafico-explicado [resumo]="resumoEvolucao()"
                  comoLer="Lilás-claro: valor aplicado (custo). Lilás: valor de mercado no fim de cada mês. Lilás acima do claro = os fundos valem mais do que você pagou.">
                  <app-grafico [configuracao]="graficoEvolucao()" [altura]="280" />
                </app-grafico-explicado>
              </section>
              <section class="painel-bloco">
                <div class="cabecalho-bloco">
                  <h2>Cotação x suas compras</h2>
                  <span class="cresce"></span>
                  <select [ngModel]="tickerHistorico()" (ngModelChange)="tickerHistorico.set($event)" aria-label="Fundo">
                    @for (f of d.fiis; track f.id) { <option [ngValue]="f.ticker">{{ f.ticker }}</option> }
                  </select>
                </div>
                <app-grafico-explicado [resumo]="resumoHistorico()"
                  comoLer="Área lilás: fechamento mensal da cota. Pontos verdes: preço pago em cada compra. Tracejado: seu preço médio.">
                  <app-grafico [configuracao]="graficoHistorico()" [altura]="280" />
                </app-grafico-explicado>
              </section>
            </div>

            <div class="grade-3">
              <section class="painel-bloco">
                <h2>Por segmento</h2>
                <app-grafico-explicado [resumo]="resumoSegmentos()" comoLer="Cada fatia é um segmento, pelo valor de mercado dos seus fundos.">
                  <app-grafico [configuracao]="graficoSegmentos()" [altura]="190" />
                </app-grafico-explicado>
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
                <h2><app-termo chave="yieldOnCost">Yield on cost</app-termo> x <app-termo chave="dy">DY</app-termo> atual (12m)</h2>
                <app-grafico-explicado [resumo]="resumoYield()"
                  comoLer="Por fundo: yield on cost (proventos de 12 meses sobre o que você pagou) em verde quando é maior que o DY atual e em rosa quando é menor; em azul, o DY atual sobre a cotação de hoje.">
                  <app-grafico [configuracao]="graficoYield()" [altura]="alturaPorFundo()" />
                </app-grafico-explicado>
              </section>
              <section class="painel-bloco">
                <h2><app-termo chave="precoMedio">Preço médio</app-termo> x cotação</h2>
                <app-grafico-explicado [resumo]="resumoPreco()"
                  comoLer="Por fundo: lilás-claro, seu preço médio; a cotação de hoje em verde quando está acima dele e em rosa quando está abaixo.">
                  <app-grafico [configuracao]="graficoPreco()" [altura]="alturaPorFundo()" />
                </app-grafico-explicado>
              </section>
            </div>

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
                      <th class="direita"><app-termo chave="dyNaCompra">DY na compra</app-termo></th>
                      <th class="direita"><app-termo chave="yieldOnCost">Yield on cost</app-termo> hoje</th>
                      <th class="direita"><app-termo chave="dy">DY</app-termo> atual</th>
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
          </ng-template>
        </app-mais-analises>
      }
    } @else {
      <p class="vazio">Carregando FIIs…</p>
    }
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
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
  readonly mesesMesAMes = signal<12 | 24>(12);
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

  /** #173: os meses do período escolhido, do mais antigo ao mais recente (o gráfico lê da esquerda para a direita). */
  readonly mesAMes = computed(() => (this.dados()?.mesAMes ?? []).slice(-this.mesesMesAMes()));
  /** Na tabela, o mês mais recente primeiro (como no app). */
  readonly mesAMesRecentes = computed(() => [...this.mesAMes()].reverse());

  readonly resumoMesAMes = computed(() => {
    const m = this.mesAMes().at(-1);
    if (!m) return null;
    const oculto = this.privacidade.oculto();
    const quando = `Em ${rotulo(m.ano, m.mes)}${this.mesAtual(m) ? ' (até hoje)' : ''}`;
    const subiu = m.valorizacao >= 0 ? 'subiram' : 'caíram';
    if (oculto) {
      const partes = [m.aportes > 0 ? 'houve aportes' : 'não houve aportes',
        m.rendimentosRecebidos > 0 ? 'caíram rendimentos' : 'não caíram rendimentos'];
      return `${quando}, ${partes.join(', ')} e as cotas ${subiu} de preço.`;
    }
    const r = (v: number) => reaisOuNada(Math.abs(v), false);
    return `${quando}: ${r(m.aportes)} de aportes, ${r(m.rendimentosRecebidos)} de rendimentos, e as cotas ${subiu} ${r(m.valorizacao)}.`;
  });

  readonly graficoMesAMes = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.mesAMes();
    return {
      type: 'bar',
      data: {
        labels: meses.map((m) => rotulo(m.ano, m.mes)),
        datasets: [
          // Os aportes ficam na tabela e na frase: no gráfico, um aporte de milhares achatava os rendimentos e a
          // valorização (dezenas de reais), e o gráfico deixava de mostrar o que a carteira rendeu no mês.
          { label: 'Rendimentos', data: meses.map((m) => m.rendimentosRecebidos), backgroundColor: COR_IFIX, borderRadius: 4 },
          { label: 'Valorização', data: meses.map((m) => m.valorizacao),
            backgroundColor: meses.map((m) => (m.valorizacao >= 0 ? COR_POSITIVO : COR_NEGATIVO)), borderRadius: 4 }
        ]
      },
      options: {
        interaction: { mode: 'index', intersect: false },
        plugins: { legend: { position: 'bottom', labels: { boxWidth: 10 } } },
        scales: { y: { ticks: { callback: compacto } } }
      }
    };
  });

  rotuloMes(m: { ano: number; mes: number }): string {
    return rotulo(m.ano, m.mes);
  }

  mesAtual(m: { ano: number; mes: number }): boolean {
    const hoje = new Date();
    return m.ano === hoje.getFullYear() && m.mes === hoje.getMonth() + 1;
  }

  readonly alturaPorFundo = computed(() => Math.max(190, (this.dados()?.fiis.length ?? 0) * 34 + 60));

  /* ---------------- UX-9: frases-resumo (sem R$ quando "ocultar valores" está ligado) ---------------- */

  readonly resumoProventos = computed(() => {
    const meses = (this.dados()?.proventos ?? []).slice(-this.mesesProventos());
    if (!meses.length) return null;
    const media = meses.reduce((s, m) => s + m.total, 0) / meses.length;
    if (media <= 0) return 'Nenhum provento no período.';
    const ultimo = meses[meses.length - 1];
    const dif = ((ultimo.total - media) / media) * 100;
    const valor = reaisOuNada(media, this.privacidade.oculto());
    const inicio = valor ? `Média de ${valor} por mês em proventos` : `Proventos de ${meses.length} meses`;
    if (Math.abs(dif) < 1) return `${inicio}; o último mês ficou na média.`;
    return `${inicio}; o último mês ficou ${pct(dif)} ${dif > 0 ? 'acima' : 'abaixo'} da média.`;
  });

  readonly resumoEvolucao = computed(() => {
    const ev = (this.dados()?.evolucao ?? []).filter((m) => m.valorMercado !== null && m.aplicado > 0);
    const u = ev[ev.length - 1];
    if (!u) return null;
    const dif = ((u.valorMercado! - u.aplicado) / u.aplicado) * 100;
    if (Math.abs(dif) < 0.5) return 'Seus FIIs valem praticamente o que você aplicou.';
    return `Seus FIIs valem ${pct(dif)} ${dif > 0 ? 'acima' : 'abaixo'} do valor aplicado.`;
  });

  readonly resumoRentabilidade = computed(() => {
    const r = this.dados()?.resumo;
    if (!r || r.rentabilidadeTotal === null) return null;
    const fmt = (v: number) => `${v < 0 ? '−' : ''}${pct(v)}`;
    const comparacoes = [r.rentabilidadeSelic !== null ? `${fmt(r.rentabilidadeSelic)} da Selic` : null,
      r.rentabilidadeIfix !== null ? `${fmt(r.rentabilidadeIfix)} do IFIX` : null].filter(Boolean);
    return `Seus FIIs renderam ${fmt(r.rentabilidadeTotal)}${comparacoes.length ? `, contra ${comparacoes.join(' e ')}` : ''} no período.`;
  });

  readonly resumoHistorico = computed(() => {
    const f = this.dados()?.fiis.find((x) => x.ticker === this.tickerHistorico());
    if (!f || f.precoAtual === null || f.precoMedio <= 0) return null;
    const dif = ((f.precoAtual - f.precoMedio) / f.precoMedio) * 100;
    if (Math.abs(dif) < 0.5) return `${f.ticker} está cotado praticamente no seu preço médio.`;
    return `${f.ticker} está cotado ${pct(dif)} ${dif > 0 ? 'acima' : 'abaixo'} do seu preço médio.`;
  });

  readonly resumoSegmentos = computed(() => {
    const d = this.dados();
    const s = [...(d?.segmentos ?? [])].sort((a, b) => b.valor - a.valor);
    if (!d || !s.length || d.resumo.valorMercado <= 0) return null;
    return `${s[0].nome} é ${pct(s[0].valor / d.resumo.valorMercado * 100)} da sua carteira de FIIs.`;
  });

  readonly resumoYield = computed(() => {
    const f = (this.dados()?.fiis ?? []).filter((x) => x.yieldOnCost12Meses !== null && x.dy12Meses !== null);
    if (!f.length) return null;
    const melhores = f.filter((x) => x.yieldOnCost12Meses! >= x.dy12Meses!).length;
    return `Em ${melhores} de ${f.length} fundos, o que você recebe sobre o preço que pagou (yield on cost) é maior que o DY atual.`;
  });

  readonly resumoPreco = computed(() => {
    const f = (this.dados()?.fiis ?? []).filter((x) => x.precoAtual !== null);
    if (!f.length) return null;
    const acima = f.filter((x) => x.precoAtual! >= x.precoMedio).length;
    if (acima === f.length) return `Todos os ${f.length} fundos estão cotados acima do seu preço médio.`;
    if (acima === 0) return `Todos os ${f.length} fundos estão cotados abaixo do seu preço médio.`;
    return `${acima} de ${f.length} fundos estão cotados acima do seu preço médio.`;
  });

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
