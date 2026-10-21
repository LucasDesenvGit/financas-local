import { DecimalPipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Projecao, ProjecaoCategoria, ProjecaoMes } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES, NOMES_MESES_CURTOS, paramsTransacoes, reaisOuNada } from '../compartilhado/utilidades';
import { CORES_GRAFICO } from '../compartilhado/cores-graficos';
import { GraficoExplicadoComponent } from '../compartilhado/grafico-explicado.component';
import { PrivacidadeService } from '../core/privacidade.service';

@Component({
  selector: 'app-projecao',
  imports: [MoedaPipe, DecimalPipe, RouterLink, GraficoComponent, GraficoExplicadoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina">
      <div>
        <h1>Projeção</h1>
        <p class="texto-suave">Receitas, despesas e saldo previstos com base no seu histórico.</p>
      </div>
      <div class="segmentado" role="group" aria-label="Horizonte da projeção">
        @for (n of horizontes; track n) {
          <button type="button" [class.ativo]="meses() === n" (click)="mudar(n)">{{ n }} meses</button>
        }
      </div>
    </header>

    @if (dados(); as d) {
      @if (d.mesesConsiderados === 0) {
        <section class="painel-bloco">
          <p class="vazio">Ainda não há um mês completo de histórico. A projeção aparece depois da primeira sincronização
            com o Open Finance (que importa os últimos 12 meses) ou após um mês de lançamentos.</p>
        </section>
      } @else {
        <section class="kpis">
          <article class="kpi">
            <span>Receita média mensal</span>
            <strong class="positivo">{{ d.mediaReceitas | currency }}</strong>
            <small>últimos {{ d.mesesConsiderados }} meses completos</small>
          </article>
          <article class="kpi">
            <span>Despesa média mensal</span>
            <strong class="negativo">{{ d.mediaDespesas | currency }}</strong>
            <small>gastos à vista, sem parcelados</small>
          </article>
          <article class="kpi">
            <span>Disponível hoje</span>
            <strong [class.negativo]="d.saldoDisponivelAtual < 0">{{ d.saldoDisponivelAtual | currency }}</strong>
            <small>contas − faturas de cartão (as parcelas que vencem depois entram nos meses delas)</small>
          </article>
          <article class="kpi kpi-destaque">
            <span>Saldo previsto em {{ nomeMes(ultimo()?.mes, ultimo()?.ano) }}</span>
            <strong>{{ ultimo()?.saldoProjetado ?? 0 | currency }}</strong>
            <small>{{ resumoTendencia() }}</small>
          </article>
        </section>

        <section class="painel-bloco">
          <h2>Receitas, despesas e saldo previsto</h2>
          <app-grafico-explicado [resumo]="resumoGrafico()"
            comoLer="Verde: receitas; rosa: despesas. Barras claras são previsão (média dos últimos meses + parcelas já contratadas). A linha lilás, no eixo da direita, é o saldo disponível ao fim de cada mês.">
            <app-grafico [configuracao]="grafico()" [altura]="320" />
          </app-grafico-explicado>
        </section>

        <div class="grade-2">
          <!-- #312: a tabela ocupa a largura toda da grade; em meia largura, "Saldo previsto" ficava cortado a 1366 px. -->
          <section class="painel-bloco bloco-mes-a-mes">
            <h2>Mês a mês</h2>
            <div class="tabela-rolagem">
              <table class="tabela">
                <thead>
                  <tr><th>Mês</th><th class="direita">Receitas</th><th class="direita">Despesas</th>
                    <th class="direita">Resultado</th><th class="direita">Saldo previsto</th></tr>
                </thead>
                <tbody>
                  @for (m of d.meses; track m.ano * 100 + m.mes) {
                    <tr [class.linha-projetada]="m.situacao !== 'realizado'">
                      <td class="nowrap">
                        {{ nomeMesCurto(m.mes, m.ano) }}
                        @if (m.situacao === 'em andamento') { <span class="etiqueta">em andamento</span> }
                        @if (m.situacao === 'projetado') { <span class="etiqueta etiqueta-suave">previsão</span> }
                        @if (virada(); as v) {
                          @if (v.mes === m) {
                            <span class="etiqueta selo-virada" [class.selo-virada-positivo]="v.positivo" [class.selo-virada-negativo]="!v.positivo">
                              {{ v.positivo ? 'saldo volta ao positivo' : 'saldo fica negativo' }}
                            </span>
                          }
                        }
                      </td>
                      <td class="direita valor positivo">{{ m.receitas | currency }}</td>
                      <td class="direita valor negativo">
                        {{ m.despesas | currency }}
                        @if (m.parcelas > 0) { <br><small class="texto-suave nowrap">{{ m.parcelas | currency }} em parcelas</small> }
                      </td>
                      <td class="direita valor" [class.positivo]="m.resultado >= 0" [class.negativo]="m.resultado < 0">{{ m.resultado | currency }}</td>
                      <td class="direita valor" [class.negativo]="(m.saldoProjetado ?? 0) < 0">
                        {{ m.saldoProjetado !== null ? (m.saldoProjetado | currency) : '—' }}
                      </td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          </section>

          <!-- #314: na largura toda, com a lista em duas colunas (sozinho em meia largura sobrava a metade da direita). -->
          <section class="painel-bloco bloco-categorias">
            <h2>Gasto médio por categoria</h2>
            <ul class="lista-legenda lista-categorias-projecao">
              @for (c of d.categorias.slice(0, 12); track c.nome) {
                <li>
                  <span class="ponto" [style.background]="c.cor"></span>
                  <span class="cresce nome-categoria">
                    <!-- Detalhe por navegação (ADR 0018): as despesas da categoria nos mesmos meses da média. -->
                    @if (linkCategoria(c); as params) {
                      <a routerLink="/transacoes" [queryParams]="params"
                        [attr.aria-label]="'Ver as transações de ' + c.nome + ' dos meses da média'">{{ c.nome }}</a>
                    } @else { {{ c.nome }} }
                    <span class="trilho trilho-categoria" aria-hidden="true">
                      <span class="progresso" [style.background]="c.cor" [style.width.%]="c.mediaMensal / maiorMedia() * 100"></span>
                    </span>
                  </span>
                  <span class="valor">{{ c.mediaMensal | currency }}</span>
                  <span class="texto-suave variacao nowrap" [class.negativo]="variacao(c.ultimoMes, c.mediaMensal) > 10"
                    [class.positivo]="variacao(c.ultimoMes, c.mediaMensal) < -10">
                    último mês {{ sinal(variacao(c.ultimoMes, c.mediaMensal)) }}{{ abs(variacao(c.ultimoMes, c.mediaMensal)) | number: '1.0-0' }}%
                  </span>
                </li>
              } @empty {
                <li class="vazio">Sem despesas no período.</li>
              }
            </ul>
            <p class="texto-suave pequeno-texto">
              Média por mês das despesas à vista dos últimos {{ d.mesesConsiderados }} meses completos. "Último mês" compara o
              mês passado com essa média: em vermelho, mais de 10% acima; em verde, mais de 10% abaixo.
            </p>
          </section>
        </div>

        <details class="guia">
          <summary>Como a projeção é calculada</summary>
          <ul>
            <li><strong>Receitas:</strong> média mensal dos últimos meses completos (até 6).</li>
            <li><strong>Despesas:</strong> média mensal dos gastos à vista + as parcelas futuras de compras parceladas já feitas.</li>
            <li><strong>Mês atual:</strong> o que já aconteceu + a média proporcional aos dias que faltam.</li>
            <li><strong>Saldo previsto:</strong> parte do disponível hoje (contas menos faturas) e acumula o resultado de cada mês.</li>
            <li>Transferências entre contas e itens marcados para ignorar ficam de fora.</li>
          </ul>
        </details>
      }
    } @else if (!erro()) {
      <p class="texto-suave">Calculando…</p>
    }
    @if (erro()) { <p class="erro">{{ erro() }}</p> }
  `,
  styles: [`
    /* #312: "Mês a mês" na largura toda da grade (a coluna "Saldo previsto" não cabia em meia largura). */
    .bloco-mes-a-mes { grid-column: 1 / -1; }
    /* #314: o gasto por categoria também na largura toda, em duas colunas no computador. */
    .bloco-categorias { grid-column: 1 / -1; }
    .lista-categorias-projecao { columns: 2; column-gap: 40px; }
    .lista-categorias-projecao li { break-inside: avoid; }
    .nome-categoria { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
    /* Alvo de clique de pelo menos 24 px de altura (WCAG 2.5.8; conferido em telas.spec.ts). */
    .nome-categoria a { display: inline-flex; align-items: center; min-height: 24px; align-self: flex-start; }
    .trilho-categoria { display: block; height: 4px; max-width: 160px; margin: 0; }
    .trilho-categoria .progresso { display: block; height: 100%; }
    .variacao { min-width: 8.5em; text-align: right; }
    .selo-virada { font-weight: 600; }
    .selo-virada-positivo { color: var(--positivo); }
    .selo-virada-negativo { color: var(--negativo); }
    @media (max-width: 700px) {
      .lista-categorias-projecao { columns: 1; }
    }
  `]
})
export class ProjecaoComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly horizontes = [3, 6, 12];
  readonly meses = signal(6);
  readonly dados = signal<Projecao | null>(null);
  readonly erro = signal<string | null>(null);

  readonly ultimo = computed(() => {
    const m = this.dados()?.meses ?? [];
    return m.length ? m[m.length - 1] : null;
  });

  /** UX-9: para onde vai o saldo no horizonte escolhido, comparado com hoje. Sem R$ com "ocultar valores". */
  readonly resumoGrafico = computed(() => {
    const d = this.dados();
    const u = this.ultimo();
    if (!d || !u || u.saldoProjetado === null) return null;
    const oculto = this.privacidade.oculto();
    const quando = `${NOMES_MESES[u.mes - 1]}/${u.ano}`;
    const diferenca = u.saldoProjetado - d.saldoDisponivelAtual;
    const valor = reaisOuNada(u.saldoProjetado, oculto);
    const base = d.mesesConsiderados > 0 ? `Pela média dos últimos ${d.mesesConsiderados} meses, ` : 'Sem histórico completo ainda, ';
    // #314: o mês em que o saldo vira, sem valor (vale também com "ocultar valores").
    const v = this.virada();
    const virada = v ? ` O saldo ${v.positivo ? 'volta ao positivo' : 'fica negativo'} em ${NOMES_MESES[v.mes.mes - 1]}/${v.mes.ano}.` : '';
    if (Math.abs(diferenca) < 1) return `${base}o saldo deve ficar estável até ${quando}${valor ? ` (${valor})` : ''}.${virada}`;
    if (u.saldoProjetado < 0) return `${base}o saldo deve ficar negativo até ${quando}${valor ? ` (${valor})` : ''}.${virada}`;
    return `${base}o saldo deve ${diferenca > 0 ? 'subir' : 'cair'} até ${quando}${valor ? `, chegando a ${valor}` : ''}.${virada}`;
  });

  readonly resumoTendencia = computed(() => {
    const d = this.dados();
    const u = this.ultimo();
    if (!d || !u || u.saldoProjetado === null) return '';
    const diferenca = u.saldoProjetado - d.saldoDisponivelAtual;
    if (Math.abs(diferenca) < 1) return 'estável em relação a hoje';
    return diferenca > 0 ? 'tendência de alta' : 'tendência de queda';
  });

  readonly grafico = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.dados()?.meses ?? [];
    const realizado = (m: { situacao: string }) => m.situacao === 'realizado';
    return {
      type: 'bar',
      data: {
        labels: meses.map((m) => `${NOMES_MESES_CURTOS[m.mes - 1]}/${String(m.ano).slice(2)}`),
        datasets: [
          {
            label: 'Receitas', data: meses.map((m) => m.receitas),
            backgroundColor: meses.map((m) => (realizado(m) ? CORES_GRAFICO.receita : CORES_GRAFICO.receita + '66')),
            borderRadius: 8, borderSkipped: false, yAxisID: 'y'
          },
          {
            label: 'Despesas', data: meses.map((m) => m.despesas),
            backgroundColor: meses.map((m) => (realizado(m) ? CORES_GRAFICO.despesa : CORES_GRAFICO.despesa + '66')),
            borderRadius: 8, borderSkipped: false, yAxisID: 'y'
          },
          // O <app-grafico> põe a linha por cima das barras, com um ponto por mês (grafico-misto.ts, #282).
          {
            label: 'Saldo previsto', data: meses.map((m) => m.saldoProjetado),
            type: 'line' as never, borderColor: CORES_GRAFICO.saldo, backgroundColor: CORES_GRAFICO.saldo,
            borderWidth: 3, tension: 0.35, yAxisID: 'y1', spanGaps: true
          } as never
        ]
      },
      options: {
        plugins: { legend: { position: 'bottom' } },
        scales: {
          y: { position: 'left', ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { notation: 'compact' }) } },
          y1: {
            position: 'right', grid: { drawOnChartArea: false },
            ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { notation: 'compact' }) }
          }
        }
      }
    };
  });

  ngOnInit(): void {
    this.carregar();
  }

  mudar(n: number): void {
    this.meses.set(n);
    this.carregar();
  }

  nomeMes(mes: number | undefined, ano: number | undefined): string {
    return mes && ano ? `${NOMES_MESES[mes - 1]}/${ano}` : '';
  }

  nomeMesCurto(mes: number, ano: number): string {
    return `${NOMES_MESES_CURTOS[mes - 1]}/${String(ano).slice(2)}`;
  }

  /**
   * #314: o primeiro mês previsto em que o saldo troca de sinal em relação ao anterior (ou ao saldo de hoje, para o
   * primeiro). É a conclusão mais importante da tabela (Shneiderman, overview first): ganha um selo em texto.
   */
  readonly virada = computed<{ mes: ProjecaoMes; positivo: boolean } | null>(() => {
    const d = this.dados();
    if (!d) return null;
    let anterior = d.saldoDisponivelAtual;
    for (const m of d.meses) {
      if (m.situacao === 'realizado' || m.saldoProjetado === null) continue;
      if ((anterior < 0) !== (m.saldoProjetado < 0)) return { mes: m, positivo: m.saldoProjetado >= 0 };
      anterior = m.saldoProjetado;
    }
    return null;
  });

  readonly maiorMedia = computed(() => Math.max(1, ...(this.dados()?.categorias ?? []).map((c) => c.mediaMensal)));

  /**
   * #314: Transações com as despesas da categoria nos mesmos meses da média (os últimos `mesesConsiderados` completos,
   * até o fim do mês passado). "Sem categoria" (null) vira categoria=sem. Sem histórico, sem link.
   */
  linkCategoria(c: ProjecaoCategoria): Record<string, string> | null {
    const meses = this.dados()?.mesesConsiderados ?? 0;
    if (meses <= 0) return null;
    const hoje = new Date();
    const inicio = new Date(hoje.getFullYear(), hoje.getMonth() - meses, 1);
    const fim = new Date(hoje.getFullYear(), hoje.getMonth(), 0);
    const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    return paramsTransacoes({ categoriaId: c.categoriaId, inicio: iso(inicio), fim: iso(fim), tipo: 'despesa' });
  }

  readonly abs = Math.abs;

  /** "+", o menos tipográfico "−" (como no resto do site) ou nada, pelo valor já arredondado. */
  sinal(v: number): string {
    const arredondado = Math.round(v);
    return arredondado > 0 ? '+' : arredondado < 0 ? '−' : '';
  }

  variacao(ultimo: number, media: number): number {
    return media > 0 ? ((ultimo - media) / media) * 100 : 0;
  }

  private carregar(): void {
    this.erro.set(null);
    this.api.projecao(this.meses()).subscribe({
      next: (d) => this.dados.set(d),
      error: (e) => {
        this.erro.set(mensagemDeErro(e));
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }
}
