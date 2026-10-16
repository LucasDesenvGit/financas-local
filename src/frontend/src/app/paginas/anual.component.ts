import { DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { NaturezaGasto, PainelAnual } from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES_CURTOS } from '../compartilhado/utilidades';

type Status = 'ok' | 'atencao' | 'alerta' | 'neutro';

interface Indicador {
  titulo: string;
  valor: string;
  meta: string;
  explicacao: string;
  status: Status;
  progresso: number | null; // 0–100 em relação à meta
}

const COR_ATUAL = '#a7a3f5';
const COR_ANTERIOR = '#c9c6dd';
const COR_RECEITA = '#8fd5b4';
const COR_RECEITA_ANTERIOR = '#cfe9dc';

const compacto = (v: string | number) => Number(v).toLocaleString('pt-BR', { notation: 'compact' });

@Component({
  selector: 'app-anual',
  imports: [FormsModule, MoedaPipe, DecimalPipe, GraficoComponent],
  template: `
    <header class="cabecalho-pagina">
      <div>
        <h1>Visão anual</h1>
        <p class="texto-suave">Compare anos e acompanhe os indicadores da sua saúde financeira.</p>
      </div>
      <div class="acoes">
        <label class="rotulo-inline">Ano
          <select [ngModel]="ano()" (ngModelChange)="mudarAno($event)">
            @for (a of anos(); track a) { <option [ngValue]="a">{{ a }}</option> }
          </select>
        </label>
        <label class="rotulo-inline">comparar com
          <select [ngModel]="comparar()" (ngModelChange)="mudarComparacao($event)">
            @for (a of anosComparacao(); track a) { <option [ngValue]="a">{{ a }}</option> }
          </select>
        </label>
      </div>
    </header>

    @if (erro()) { <p class="erro">{{ erro() }}</p> }

    @if (dados(); as d) {
      <!-- Saúde financeira -->
      <section class="indicadores">
        @for (i of indicadores(); track i.titulo) {
          <article class="indicador" [class]="'indicador indicador-' + i.status">
            <div class="indicador-topo">
              <span>{{ i.titulo }}</span>
              <span class="indicador-selo">{{ rotuloStatus(i.status) }}</span>
            </div>
            <strong>{{ i.valor }}</strong>
            @if (i.progresso !== null) {
              <div class="trilho"><div class="progresso" [style.width.%]="i.progresso"></div></div>
            }
            <small class="texto-suave">{{ i.meta }}</small>
            <p class="indicador-explicacao">{{ i.explicacao }}</p>
          </article>
        }
      </section>

      <!-- Comparativo -->
      <section class="kpis">
        @for (k of comparativos(); track k.titulo) {
          <article class="kpi">
            <span>{{ k.titulo }} {{ d.ano }}{{ d.mesLimite < 12 && d.mesLimite > 0 ? ' (até ' + nomeMes(d.mesLimite) + ')' : '' }}</span>
            <strong [class.positivo]="k.positivo === true" [class.negativo]="k.positivo === false">{{ k.atual | currency }}</strong>
            <small>
              {{ d.anoComparacao }} no mesmo período: {{ k.anterior | currency }}
              @if (k.variacao !== null) {
                <span class="variacao-selo" [class.bom]="k.variacaoBoa" [class.ruim]="!k.variacaoBoa">
                  {{ k.variacao > 0 ? '▲' : '▼' }} {{ (k.variacao < 0 ? -k.variacao : k.variacao) | number: '1.0-1' }}%
                </span>
              }
            </small>
          </article>
        }
      </section>

      <div class="grade-2">
        <section class="painel-bloco">
          <h2>Despesas mês a mês</h2>
          <app-grafico [configuracao]="graficoDespesas()" [altura]="260" />
        </section>
        <section class="painel-bloco">
          <h2>Receitas mês a mês</h2>
          <app-grafico [configuracao]="graficoReceitas()" [altura]="260" />
        </section>
      </div>

      <div class="grade-2">
        <section class="painel-bloco">
          <h2>Poupança acumulada no ano</h2>
          <p class="texto-suave subtitulo-grafico">Quanto sobrou somando mês a mês: a linha subindo mostra que você está guardando.</p>
          <app-grafico [configuracao]="graficoAcumulado()" [altura]="250" />
        </section>
        <section class="painel-bloco">
          <h2>Taxa de poupança por mês</h2>
          <p class="texto-suave subtitulo-grafico">Parte da renda que sobrou em cada mês. A linha tracejada é a referência de 20%.</p>
          <app-grafico [configuracao]="graficoTaxa()" [altura]="250" />
        </section>
      </div>

      <div class="grade-2">
        <section class="painel-bloco">
          <h2>Para onde vai a sua renda (regra 50/30/20)</h2>
          <p class="texto-suave subtitulo-grafico">Referência: até 50% em essenciais, até 30% em estilo de vida e pelo menos 20% guardados.
            Classifique as categorias em Categorias e regras.</p>
          <app-grafico [configuracao]="grafico503020()" [altura]="170" />
          @if (d.distribuicao.naoClassificado > 0) {
            <p class="texto-suave pequeno-texto">{{ d.distribuicao.naoClassificado | currency }} em categorias sem classificação.</p>
          }
        </section>
        <section class="painel-bloco">
          <h2>Patrimônio estimado</h2>
          <p class="texto-suave subtitulo-grafico">Reconstruído a partir do patrimônio de hoje e do resultado de cada mês.</p>
          <app-grafico [configuracao]="graficoPatrimonio()" [altura]="230" />
        </section>
      </div>

      <section class="painel-bloco">
        <h2>Categorias: {{ d.ano }} × {{ d.anoComparacao }} <small class="texto-suave">(mesmo período)</small></h2>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead>
              <tr><th>Categoria</th><th class="direita">{{ d.ano }}</th><th class="direita">{{ d.anoComparacao }}</th>
                <th class="direita">Variação</th><th style="width: 30%">Peso nos gastos</th></tr>
            </thead>
            <tbody>
              @for (c of d.categorias.slice(0, 15); track c.nome) {
                <tr>
                  <td>
                    <span class="chip"><span class="ponto" [style.background]="c.cor"></span>{{ c.nome }}</span>
                    @if (c.natureza === NaturezaGasto.Essencial) { <span class="etiqueta etiqueta-suave">essencial</span> }
                  </td>
                  <td class="direita valor">{{ c.valor | currency }}</td>
                  <td class="direita texto-suave">{{ c.valorAnterior | currency }}</td>
                  <td class="direita">
                    @if (c.valorAnterior > 0) {
                      <span class="variacao-selo" [class.ruim]="c.valor > c.valorAnterior" [class.bom]="c.valor <= c.valorAnterior">
                        {{ c.valor >= c.valorAnterior ? '▲' : '▼' }} {{ variacaoAbs(c.valor, c.valorAnterior) | number: '1.0-0' }}%
                      </span>
                    } @else { <span class="texto-suave">novo</span> }
                  </td>
                  <td>
                    <div class="trilho"><div class="progresso" [style.background]="c.cor"
                      [style.width.%]="d.resumo.despesas > 0 ? c.valor / d.resumo.despesas * 100 : 0"></div></div>
                  </td>
                </tr>
              } @empty {
                <tr><td colspan="5" class="vazio">Sem despesas no período.</td></tr>
              }
            </tbody>
          </table>
        </div>
      </section>

      @if (mapa().linhas.length) {
        <section class="painel-bloco">
          <h2>Mapa de gastos {{ d.ano }}</h2>
          <p class="texto-suave subtitulo-grafico">Quanto mais forte a cor, maior o gasto da categoria naquele mês.</p>
          <div class="tabela-rolagem">
            <table class="tabela mapa-calor">
              <thead>
                <tr><th>Categoria</th>
                  @for (m of mapa().meses; track m) { <th class="direita">{{ nomeMes(m) }}</th> }
                </tr>
              </thead>
              <tbody>
                @for (l of mapa().linhas; track l.nome) {
                  <tr>
                    <td class="nowrap"><span class="ponto" [style.background]="l.cor"></span>{{ l.nome }}</td>
                    @for (v of l.valores; track $index) {
                      <td class="direita celula-calor" [style.background]="corCalor(l.cor, v, l.maximo)">
                        {{ v > 0 ? (v | currency) : '' }}
                      </td>
                    }
                  </tr>
                }
              </tbody>
            </table>
          </div>
        </section>
      }
    } @else if (!erro()) {
      <p class="texto-suave">Carregando…</p>
    }
  `
})
export class AnualComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly NaturezaGasto = NaturezaGasto;
  readonly ano = signal(new Date().getFullYear());
  readonly comparar = signal(new Date().getFullYear() - 1);
  readonly dados = signal<PainelAnual | null>(null);
  readonly erro = signal<string | null>(null);

  readonly anos = computed(() => {
    const lista = this.dados()?.anosDisponiveis ?? [this.ano()];
    return lista.includes(this.ano()) ? lista : [this.ano(), ...lista];
  });
  readonly anosComparacao = computed(() => {
    const lista = new Set<number>([...this.anos(), this.ano() - 1, this.comparar()]);
    return [...lista].filter((a) => a !== this.ano()).sort((a, b) => b - a);
  });

  readonly indicadores = computed<Indicador[]>(() => {
    const d = this.dados();
    if (!d) return [];
    const oculto = this.privacidade.oculto();
    const s = d.saude;
    const taxa = d.resumo.taxaPoupanca;
    const reserva = s.mesesDeReserva;
    const comp = s.comprometimentoParcelas;
    const variacaoGastos = this.variacao(d.resumo.despesas, d.resumoAnteriorMesmoPeriodo.despesas);

    return [
      {
        titulo: 'Taxa de poupança',
        valor: d.resumo.receitas > 0 ? `${this.fmt(taxa)}%` : '—',
        meta: 'Meta: guardar pelo menos 20% da renda',
        explicacao: taxa >= 20 ? 'Ótimo: você está guardando uma boa parte do que ganha.'
          : taxa >= 10 ? 'Razoável. Tente reduzir gastos de estilo de vida para chegar a 20%.'
          : taxa >= 0 ? 'Baixa: quase tudo que entra está saindo.' : 'Você gastou mais do que ganhou neste período.',
        status: d.resumo.receitas <= 0 ? 'neutro' : taxa >= 20 ? 'ok' : taxa >= 10 ? 'atencao' : 'alerta',
        progresso: d.resumo.receitas > 0 ? Math.max(0, Math.min(100, (taxa / 20) * 100)) : null
      },
      {
        titulo: 'Reserva de emergência',
        valor: reserva === null ? '—' : `${this.fmt(reserva)} meses`,
        meta: `Meta: 6 meses de gastos${s.mesesDeReservaEssencial !== null ? ` · só essenciais: ${this.fmt(s.mesesDeReservaEssencial)} meses` : ''}`,
        explicacao: reserva === null ? 'Sem histórico de gastos suficiente.'
          : reserva >= 6 ? 'Você aguentaria mais de 6 meses sem renda mantendo o padrão atual.'
          : reserva >= 3 ? 'Boa base. Continue até cobrir 6 meses de gastos.'
          : 'Reserva curta: priorize guardar dinheiro em aplicação de liquidez diária.',
        status: reserva === null ? 'neutro' : reserva >= 6 ? 'ok' : reserva >= 3 ? 'atencao' : 'alerta',
        progresso: reserva === null ? null : Math.min(100, (reserva / 6) * 100)
      },
      {
        titulo: 'Comprometimento com parcelas',
        valor: comp === null ? '—' : `${this.fmt(comp)}% da renda`,
        meta: oculto ? 'Ideal: abaixo de 30%' : `Ideal: abaixo de 30% · cartão: ${this.moeda(s.parcelasMensais)}`
          + (s.parcelasFinanciamentos > 0 ? ` · financiamentos: ${this.moeda(s.parcelasFinanciamentos)}` : '')
          + (s.parcelasEmprestimos > 0 ? ` · empréstimos: ${this.moeda(s.parcelasEmprestimos)}` : ''),
        explicacao: comp === null ? 'Sem renda registrada no período.'
          : comp < 15 ? 'Parcelas sob controle.'
          : comp < 30 ? 'Atenção a novas compras parceladas.'
          : 'Muita renda presa em parcelas: evite novos parcelamentos.',
        status: comp === null ? 'neutro' : comp < 15 ? 'ok' : comp < 30 ? 'atencao' : 'alerta',
        progresso: comp === null ? null : Math.min(100, (comp / 30) * 100)
      },
      {
        titulo: `Gastos × ${d.anoComparacao}`,
        valor: variacaoGastos === null ? '—' : `${variacaoGastos > 0 ? '+' : ''}${this.fmt(variacaoGastos)}%`,
        meta: 'Comparação no mesmo período do ano',
        explicacao: variacaoGastos === null ? `Sem dados de ${d.anoComparacao} para comparar.`
          : variacaoGastos <= 0 ? 'Você está gastando menos que no ano anterior.'
          : variacaoGastos <= 5 ? 'Gastos praticamente estáveis (dentro da inflação).'
          : 'Gastos crescendo acima da inflação: veja quais categorias subiram.',
        status: variacaoGastos === null ? 'neutro' : variacaoGastos <= 0 ? 'ok' : variacaoGastos <= 5 ? 'atencao' : 'alerta',
        progresso: null
      }
    ];
  });

  readonly comparativos = computed(() => {
    const d = this.dados();
    if (!d) return [];
    const a = d.resumo;
    const b = d.resumoAnteriorMesmoPeriodo;
    const item = (titulo: string, atual: number, anterior: number, maiorEhMelhor: boolean, positivo?: boolean) => {
      const variacao = this.variacao(atual, anterior);
      return { titulo, atual, anterior, variacao, variacaoBoa: variacao === null ? true : maiorEhMelhor ? variacao >= 0 : variacao <= 0, positivo };
    };
    return [
      item('Receitas', a.receitas, b.receitas, true, true),
      item('Despesas', a.despesas, b.despesas, false, false),
      item('Sobrou', a.resultado, b.resultado, true, a.resultado >= 0)
    ];
  });

  private readonly rotulosMeses = NOMES_MESES_CURTOS.map((m) => m);

  readonly graficoDespesas = computed<ChartConfiguration<'line'>>(() => this.linhasComparativas(
    (m) => m.despesas, (m) => m.despesasAnterior, COR_ATUAL, COR_ANTERIOR));

  readonly graficoReceitas = computed<ChartConfiguration<'line'>>(() => this.linhasComparativas(
    (m) => m.receitas, (m) => m.receitasAnterior, COR_RECEITA, COR_RECEITA_ANTERIOR));

  readonly graficoAcumulado = computed<ChartConfiguration<'line'>>(() => {
    const d = this.dados();
    const meses = d?.meses ?? [];
    let a = 0;
    let b = 0;
    const atual = meses.map((m) => (m.futuro ? null : (a += m.receitas - m.despesas)));
    const anterior = meses.map((m) => (b += m.receitasAnterior - m.despesasAnterior));
    return {
      type: 'line',
      data: {
        labels: this.rotulosMeses,
        datasets: [
          { label: String(d?.ano ?? ''), data: atual, borderColor: COR_ATUAL, backgroundColor: COR_ATUAL + '33', fill: true, tension: 0.35, borderWidth: 3, pointRadius: 2 },
          { label: String(d?.anoComparacao ?? ''), data: anterior, borderColor: COR_ANTERIOR, borderDash: [6, 5], tension: 0.35, borderWidth: 2, pointRadius: 0 }
        ]
      },
      options: { plugins: { legend: { position: 'bottom' } }, scales: { y: { ticks: { callback: compacto } } } }
    };
  });

  readonly graficoTaxa = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.dados()?.meses ?? [];
    const taxas = meses.map((m) => (m.futuro || m.receitas <= 0 ? null : Math.round(((m.receitas - m.despesas) / m.receitas) * 1000) / 10));
    return {
      type: 'bar',
      data: {
        labels: this.rotulosMeses,
        datasets: [
          {
            label: 'Taxa de poupança (%)', data: taxas,
            backgroundColor: taxas.map((t) => (t === null ? 'transparent' : t >= 20 ? '#8fd5b4' : t >= 0 ? '#f6d38a' : '#f5a3b0')),
            borderRadius: 8, borderSkipped: false
          },
          {
            label: 'Referência 20%', data: meses.map(() => 20), type: 'line' as never,
            borderColor: '#a7a3f5', borderDash: [6, 5], borderWidth: 2, pointRadius: 0
          } as never
        ]
      },
      options: {
        plugins: { legend: { display: false } },
        scales: { y: { ticks: { callback: (v) => `${v}%` } } }
      }
    };
  });

  readonly grafico503020 = computed<ChartConfiguration<'bar'>>(() => {
    const d = this.dados()?.distribuicao;
    const renda = d && d.receitas > 0 ? d.receitas : 0;
    const pct = (v: number) => (renda > 0 ? Math.round((v / renda) * 1000) / 10 : 0);
    const essenciais = d ? pct(d.essenciais) : 0;
    const estilo = d ? pct(d.estiloDeVida + d.naoClassificado) : 0;
    const poupanca = d ? Math.max(0, pct(d.poupanca)) : 0;
    return {
      type: 'bar',
      data: {
        labels: ['Você', 'Referência'],
        datasets: [
          { label: 'Essenciais', data: [essenciais, 50], backgroundColor: '#92c8f0', borderRadius: 6 },
          { label: 'Estilo de vida', data: [estilo, 30], backgroundColor: '#f3b3d3', borderRadius: 6 },
          { label: 'Guardado', data: [poupanca, 20], backgroundColor: '#8fd5b4', borderRadius: 6 }
        ]
      },
      options: {
        indexAxis: 'y',
        plugins: {
          legend: { position: 'bottom' },
          tooltip: { callbacks: { label: (c) => `${c.dataset.label}: ${c.parsed.x}%` } }
        },
        scales: {
          x: { stacked: true, ticks: { callback: (v) => `${v}%` } },
          y: { stacked: true, grid: { display: false } }
        }
      }
    };
  });

  readonly graficoPatrimonio = computed<ChartConfiguration<'line'>>(() => {
    const meses = this.dados()?.meses ?? [];
    return {
      type: 'line',
      data: {
        labels: this.rotulosMeses,
        datasets: [{
          label: 'Patrimônio estimado', data: meses.map((m) => m.patrimonioEstimado),
          borderColor: '#92c8f0', backgroundColor: '#92c8f033', fill: true, tension: 0.35, borderWidth: 3, pointRadius: 2
        }]
      },
      options: { plugins: { legend: { display: false } }, scales: { y: { ticks: { callback: compacto } } } }
    };
  });

  readonly mapa = computed(() => {
    const d = this.dados();
    if (!d || d.mesLimite === 0) return { meses: [] as number[], linhas: [] as { nome: string; cor: string; valores: number[]; maximo: number }[] };
    const meses = Array.from({ length: d.mesLimite }, (_, i) => i + 1);
    const linhas = d.categorias.filter((c) => c.valor > 0).slice(0, 10).map((c) => {
      const valores = c.meses.slice(0, d.mesLimite);
      return { nome: c.nome, cor: c.cor, valores, maximo: Math.max(...valores, 1) };
    });
    return { meses, linhas };
  });

  ngOnInit(): void {
    this.carregar();
  }

  mudarAno(ano: number): void {
    this.ano.set(Number(ano));
    if (this.comparar() === this.ano()) this.comparar.set(this.ano() - 1);
    this.carregar();
  }

  mudarComparacao(ano: number): void {
    this.comparar.set(Number(ano));
    this.carregar();
  }

  nomeMes(mes: number): string {
    return NOMES_MESES_CURTOS[mes - 1];
  }

  rotuloStatus(s: Status): string {
    return { ok: 'saudável', atencao: 'atenção', alerta: 'cuidado', neutro: 'sem dados' }[s];
  }

  variacaoAbs(atual: number, anterior: number): number {
    return Math.abs(this.variacao(atual, anterior) ?? 0);
  }

  corCalor(cor: string, valor: number, maximo: number): string {
    if (valor <= 0) return 'transparent';
    const intensidade = Math.round(15 + (valor / maximo) * 70);
    return `color-mix(in srgb, ${cor} ${intensidade}%, transparent)`;
  }

  private variacao(atual: number, anterior: number): number | null {
    return anterior > 0 ? ((atual - anterior) / anterior) * 100 : null;
  }

  private fmt(n: number): string {
    return n.toLocaleString('pt-BR', { maximumFractionDigits: 1 });
  }

  private moeda(n: number): string {
    return n.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
  }

  private linhasComparativas(atual: (m: PainelAnual['meses'][number]) => number, anterior: (m: PainelAnual['meses'][number]) => number,
    corAtual: string, corAnterior: string): ChartConfiguration<'line'> {
    const d = this.dados();
    const meses = d?.meses ?? [];
    return {
      type: 'line',
      data: {
        labels: this.rotulosMeses,
        datasets: [
          { label: String(d?.ano ?? ''), data: meses.map((m) => (m.futuro ? null : atual(m))), borderColor: corAtual,
            backgroundColor: corAtual, tension: 0.35, borderWidth: 3, pointRadius: 3 },
          { label: String(d?.anoComparacao ?? ''), data: meses.map((m) => anterior(m)), borderColor: corAnterior,
            backgroundColor: corAnterior, borderDash: [6, 5], tension: 0.35, borderWidth: 2, pointRadius: 0 }
        ]
      },
      options: { plugins: { legend: { position: 'bottom' } }, scales: { y: { ticks: { callback: compacto } } } }
    };
  }

  private carregar(): void {
    this.erro.set(null);
    this.api.painelAnual(this.ano(), this.comparar()).subscribe({
      next: (d) => this.dados.set(d),
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }
}

