import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Emprestimo, NaturezaCredito, PainelEmprestimos, ResumoCredito } from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES_CURTOS } from '../compartilhado/utilidades';

/** Cores fixas por natureza, usadas nos cartões, etiquetas e no gráfico: financiamento sempre azul, empréstimo sempre laranja. */
const COR_FINANCIAMENTO = '#8fa8f0';
const COR_EMPRESTIMO = '#f6a467';

interface Grupo {
  natureza: NaturezaCredito; titulo: string; descricao: string; classe: string;
  resumo: ResumoCredito; contratos: Emprestimo[];
}

@Component({
  selector: 'app-emprestimos',
  imports: [MoedaPipe, DatePipe, DecimalPipe, RouterLink, GraficoComponent],
  template: `
    <header class="cabecalho-pagina">
      <div>
        <h1>Empréstimos e financiamentos</h1>
        <p class="texto-suave">Contratos de crédito informados pelo Open Finance. Financiamentos e empréstimos aparecem sempre separados.</p>
      </div>
    </header>

    @if (erro()) { <p class="erro">{{ erro() }}</p> }

    @if (dados(); as d) {
      @if (!d.contratos.length) {
        <section class="painel-bloco">
          <p class="vazio">Nenhum empréstimo ou financiamento encontrado.</p>
          <p class="texto-suave orientacao">
            Para importar, o consentimento da instituição no Meu Pluggy precisa incluir <strong>operações de crédito</strong>
            (empréstimos e financiamentos). Depois de autorizar, use <strong>Sincronizar bancos</strong>.
            Nem toda instituição compartilha esses dados. Acompanhe as conexões em <a routerLink="/configuracoes">Configurações</a>.
          </p>
        </section>
      } @else {
        <section class="kpis">
          <article class="kpi kpi-destaque">
            <span>Total devido</span>
            <strong>{{ d.financiamentos.saldoDevedor + d.emprestimos.saldoDevedor | currency }}</strong>
            <small>financiamentos + empréstimos (saldo devedor)</small>
          </article>
          <article class="kpi kpi-natureza kpi-financiamento">
            <span>Financiamentos</span>
            <strong class="negativo">{{ d.financiamentos.saldoDevedor | currency }}</strong>
            <small>{{ d.financiamentos.parcelaMensal | currency }}/mês · {{ rotuloContratos(d.financiamentos.contratosAbertos) }}</small>
          </article>
          <article class="kpi kpi-natureza kpi-emprestimo">
            <span>Empréstimos</span>
            <strong class="negativo">{{ d.emprestimos.saldoDevedor | currency }}</strong>
            <small>{{ d.emprestimos.parcelaMensal | currency }}/mês · {{ rotuloContratos(d.emprestimos.contratosAbertos) }}</small>
          </article>
          <article class="kpi">
            <span>Parcelas por mês</span>
            <strong>{{ d.financiamentos.parcelaMensal + d.emprestimos.parcelaMensal | currency }}</strong>
            <small>soma estimada pela última parcela paga</small>
          </article>
        </section>

        <section class="painel-bloco">
          <div class="bloco-titulo">
            <h2>Pagamentos por mês (12 meses)</h2>
            <span class="legenda-natureza">
              <span><i class="ponto" [style.background]="corFinanciamento"></i> Financiamentos</span>
              <span><i class="ponto" [style.background]="corEmprestimo"></i> Empréstimos</span>
            </span>
          </div>
          <app-grafico [configuracao]="graficoPagamentos()" [altura]="260" />
          <p class="texto-suave pequeno-texto">
            Esses pagamentos também aparecem em <a routerLink="/transacoes">Transações</a> como despesas. Aqui eles
            vêm dos contratos, separados por natureza, para você ver quanto vai para cada tipo de dívida.
          </p>
        </section>

        @for (g of grupos(); track g.natureza) {
          <section class="painel-bloco grupo" [class]="'painel-bloco grupo ' + g.classe">
            <div class="bloco-titulo">
              <div>
                <h2><span class="etiqueta etiqueta-natureza">{{ g.titulo }}</span></h2>
                <p class="texto-suave descricao-grupo">{{ g.descricao }}</p>
              </div>
              <div class="totais-grupo">
                <span>Saldo devedor <strong class="negativo">{{ g.resumo.saldoDevedor | currency }}</strong></span>
                <span>Parcela/mês <strong>{{ g.resumo.parcelaMensal | currency }}</strong></span>
                <span>Já pago <strong>{{ g.resumo.totalPago | currency }}</strong></span>
              </div>
            </div>

            @for (c of g.contratos; track c.id) {
              <article class="contrato" [class.quitado]="c.quitado">
                <div class="contrato-topo">
                  <div class="cresce">
                    <strong>{{ c.produto }}</strong>
                    <small class="texto-suave">
                      {{ c.nomeInstituicao ?? 'Instituição' }} · {{ c.modalidadeDescricao }}
                      @if (c.numeroContrato) { · contrato {{ c.numeroContrato }} }
                    </small>
                  </div>
                  @if (c.quitado) {
                    <span class="etiqueta etiqueta-sucesso">Quitado</span>
                  } @else if (c.parcelasEmAtraso > 0) {
                    <span class="etiqueta etiqueta-alerta">{{ c.parcelasEmAtraso }} parcela(s) em atraso</span>
                  }
                </div>

                <div class="contrato-valores">
                  <div><span>Saldo devedor</span><strong class="negativo">{{ c.saldoDevedor | currency }}</strong></div>
                  <div><span>Parcela</span><strong>{{ c.valorParcela !== null ? (c.valorParcela | currency) : '—' }}</strong></div>
                  <div><span>Valor contratado</span><strong>{{ c.valorContratado | currency }}</strong></div>
                  <div><span>Já pago</span><strong>{{ c.totalPago | currency }}</strong></div>
                  <div><span>Juros</span><strong>{{ taxa(c) }}</strong></div>
                  <div><span>CET</span><strong>{{ c.cet !== null ? (c.cet * 100 | number: '1.2-2') + '% a.a.' : '—' }}</strong></div>
                </div>

                @if (c.parcelasTotal) {
                  <div class="linha-flex texto-suave pequeno">
                    <span>{{ c.parcelasPagas ?? 0 }} de {{ c.parcelasTotal }} parcelas pagas</span>
                    <span>
                      @if (c.proximoVencimento) { próxima prevista em {{ c.proximoVencimento | date: 'dd/MM/yyyy' }} · }
                      @if (c.dataVencimentoFinal) { termina em {{ c.dataVencimentoFinal | date: 'MM/yyyy' }} }
                    </span>
                  </div>
                  <div class="trilho"><div class="progresso" [style.width.%]="progresso(c)"></div></div>
                }
                @if (c.sistemaAmortizacao || c.dataContratacao) {
                  <small class="texto-suave">
                    @if (c.dataContratacao) { Contratado em {{ c.dataContratacao | date: 'dd/MM/yyyy' }} }
                    @if (c.sistemaAmortizacao) { · amortização {{ c.sistemaAmortizacao }} }
                  </small>
                }
              </article>
            } @empty {
              <p class="vazio">Nenhum {{ g.natureza === Natureza.Financiamento ? 'financiamento' : 'empréstimo' }}.</p>
            }
          </section>
        }

        @if (atualizadoEm(); as a) {
          <p class="texto-suave pequeno-texto">Dados da instituição atualizados em {{ a | date: 'dd/MM/yyyy HH:mm' }}.</p>
        }
      }
    } @else if (!erro()) {
      <p class="texto-suave">Carregando…</p>
    }
  `,
  styles: [`
    .kpi-natureza { position: relative; overflow: hidden; }
    .kpi-natureza::before { content: ""; position: absolute; inset: 0 auto 0 0; width: 5px; }
    .kpi-financiamento::before { background: ${COR_FINANCIAMENTO}; }
    .kpi-emprestimo::before { background: ${COR_EMPRESTIMO}; }
    .legenda-natureza { display: flex; gap: 14px; font-size: 0.85rem; color: var(--texto-suave); }
    .legenda-natureza .ponto { display: inline-block; width: 10px; height: 10px; border-radius: 3px; margin-right: 4px; }
    .grupo { border-left: 5px solid transparent; }
    .grupo-financiamento { border-left-color: ${COR_FINANCIAMENTO}; }
    .grupo-emprestimo { border-left-color: ${COR_EMPRESTIMO}; }
    .grupo h2 { margin: 0; }
    .etiqueta-natureza { font-size: 0.95rem; padding: 4px 12px; color: #1f2937; }
    .grupo-financiamento .etiqueta-natureza { background: ${COR_FINANCIAMENTO}; }
    .grupo-emprestimo .etiqueta-natureza { background: ${COR_EMPRESTIMO}; }
    .grupo-financiamento .progresso { background: ${COR_FINANCIAMENTO}; }
    .grupo-emprestimo .progresso { background: ${COR_EMPRESTIMO}; }
    .descricao-grupo { margin: 6px 0 0; font-size: 0.85rem; }
    .totais-grupo { display: flex; flex-wrap: wrap; gap: 6px 18px; font-size: 0.85rem; color: var(--texto-suave); }
    .totais-grupo strong { margin-left: 4px; }
    .contrato { border-top: 1px solid var(--linha); padding: 14px 0 10px; }
    .contrato.quitado { opacity: 0.65; }
    .contrato-topo { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 10px; }
    .contrato-topo small { display: block; margin-top: 2px; }
    .contrato-valores { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 10px; margin-bottom: 10px; }
    .contrato-valores span { display: block; font-size: 0.78rem; color: var(--texto-suave); }
    .contrato-valores strong { font-size: 0.98rem; }
    .pequeno { font-size: 0.82rem; }
    .orientacao { max-width: 640px; margin: 0 auto 12px; text-align: center; }
  `]
})
export class EmprestimosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly Natureza = NaturezaCredito;
  readonly corFinanciamento = COR_FINANCIAMENTO;
  readonly corEmprestimo = COR_EMPRESTIMO;
  readonly dados = signal<PainelEmprestimos | null>(null);
  readonly erro = signal<string | null>(null);

  readonly grupos = computed<Grupo[]>(() => {
    const d = this.dados();
    if (!d) return [];
    return [
      {
        natureza: NaturezaCredito.Financiamento, titulo: 'Financiamentos', classe: 'grupo-financiamento',
        descricao: 'Crédito para comprar um bem: imóvel, veículo ou outros bens.',
        resumo: d.financiamentos, contratos: d.contratos.filter((c) => c.natureza === NaturezaCredito.Financiamento)
      },
      {
        natureza: NaturezaCredito.Emprestimo, titulo: 'Empréstimos', classe: 'grupo-emprestimo',
        descricao: 'Crédito pessoal, consignado, cheque especial e demais empréstimos.',
        resumo: d.emprestimos, contratos: d.contratos.filter((c) => c.natureza === NaturezaCredito.Emprestimo)
      }
    ];
  });

  readonly atualizadoEm = computed(() => {
    const datas = (this.dados()?.contratos ?? []).map((c) => c.atualizadoEm).sort();
    return datas.length ? datas[datas.length - 1] : null;
  });

  readonly graficoPagamentos = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.dados()?.pagamentosPorMes ?? [];
    const oculto = this.privacidade.oculto();
    return {
      type: 'bar',
      data: {
        labels: meses.map((m) => `${NOMES_MESES_CURTOS[m.mes - 1]}/${String(m.ano).slice(2)}`),
        datasets: [
          { label: 'Financiamentos', data: meses.map((m) => m.financiamentos), backgroundColor: COR_FINANCIAMENTO, borderRadius: 6, stack: 'pagos' },
          { label: 'Empréstimos', data: meses.map((m) => m.emprestimos), backgroundColor: COR_EMPRESTIMO, borderRadius: 6, stack: 'pagos' }
        ]
      },
      options: {
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (ctx) => `${ctx.dataset.label}: ${oculto ? 'R$ •••••' : Number(ctx.raw).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })}`
            }
          }
        },
        scales: {
          x: { stacked: true },
          y: {
            stacked: true,
            ticks: { callback: (v) => (oculto ? '•••' : Number(v).toLocaleString('pt-BR', { notation: 'compact' })) }
          }
        }
      }
    };
  });

  ngOnInit(): void {
    this.api.emprestimos().subscribe({
      next: (d) => this.dados.set(d),
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }

  rotuloContratos(n: number): string {
    return n === 1 ? '1 contrato' : `${n} contratos`;
  }

  progresso(c: Emprestimo): number {
    if (!c.parcelasTotal) return 0;
    return Math.min(100, ((c.parcelasPagas ?? 0) / c.parcelasTotal) * 100);
  }

  taxa(c: Emprestimo): string {
    if (c.taxaJuros === null) return c.indexador ?? '—';
    const periodo = c.periodicidadeTaxa?.toUpperCase() === 'AM' ? 'a.m.' : 'a.a.';
    const valor = `${(c.taxaJuros * 100).toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}% ${periodo}`;
    return c.indexador && c.indexador !== 'PRE_FIXADO' ? `${c.indexador} + ${valor}` : valor;
  }
}
