import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { InvestimentoComparado } from '../../core/modelos';
import { nomeTipoPluggy, percentual } from './investimentos';

/**
 * A aba "Open Finance" de Investimentos, sem o rendimento por mês (que tem componente próprio): a barra de "Registrar
 * compras na carteira" e a grade dos investimentos do Open Finance (#731, ADR 0059). Componente de apresentação: recebe
 * os investimentos e avisa os cliques; a grade recolhe pelo título (#331, Disclosure), e a escolha, que fica neste
 * navegador (ADR 0017), é guardada pela tela.
 */
@Component({
  selector: 'app-investimentos-open-finance',
  imports: [MoedaPipe, DatePipe, DecimalPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    /* #331: o título da grade do Open Finance, que recolhe. Alvo de 32 px (WCAG 2.5.8 pede 24). */
    .cabecalho-grade-of { display: flex; align-items: baseline; gap: 4px 12px; flex-wrap: wrap; margin: 4px 0 8px; }
    .cabecalho-grade-of h2 { margin: 0; font-size: 1rem; }
    .botao-grade-of {
      font: inherit; color: var(--texto-forte); background: none; border: 0; padding: 4px 6px 4px 0; min-height: 32px;
      cursor: pointer; display: inline-flex; align-items: center; gap: 6px;
    }
    .botao-grade-of:focus-visible { outline: 2px solid var(--primaria); outline-offset: 2px; border-radius: 6px; }
    tr.encerrada td { opacity: 0.6; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
    .barra-importacao { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
  `,
  template: `
    <div class="barra-importacao">
      <span class="texto-suave">
        @if (naoRegistrados() > 0) {
          {{ naoRegistrados() }} FII(s)/ação(ões) ainda não registrados como compras na carteira.
        } @else {
          FIIs, ações e ETFs do Open Finance já estão registrados como compras na carteira.
        }
      </span>
      <span class="cresce"></span>
      <button type="button" class="botao botao-primario" (click)="registrarCompras.emit()" [disabled]="registrando()">
        {{ registrando() ? 'Registrando…' : 'Registrar compras na carteira' }}</button>
    </div>
    <!-- #331: a grade recolhe pelo título (Disclosure: botão com aria-expanded e aria-controls), e a escolha fica
         neste navegador (ADR 0017). Recolhida, o total e quantos são continuam à vista. -->
    <div class="cabecalho-grade-of">
      <h2><button type="button" class="botao-grade-of" [attr.aria-expanded]="!recolhida()" aria-controls="grade-of"
        (click)="alternarGrade.emit()"><span aria-hidden="true">{{ recolhida() ? '▸' : '▾' }}</span>
        Investimentos do Open Finance</button></h2>
      <span class="texto-suave">{{ investimentos().length }} {{ investimentos().length === 1 ? 'investimento' : 'investimentos' }} ·
        {{ total() | currency }}</span>
    </div>
    <div id="grade-of">
    @if (!recolhida()) {
    <div class="tabela-rolagem">
      <table class="tabela">
        <thead>
          <tr><th>Investimento</th><th>Instituição</th><th class="direita">Aplicado</th><th class="direita">Saldo</th>
            <th class="direita">Rentabilidade</th><th class="direita"><app-termo chave="selic">Selic</app-termo> no período</th><th class="direita">% da Selic</th>
            <th>Vencimento</th></tr>
        </thead>
        <tbody>
          @for (i of investimentos(); track i.id) {
            <tr [class.encerrada]="i.jaNaCarteira">
              <!-- #170: o nome que se reconhece (Nielsen 2 e 6), com o do banco embaixo para achar no app. -->
              <td>{{ i.nomeExibicao }} <span class="etiqueta etiqueta-suave">{{ nomeTipoPluggy(i.tipo, i.subtipo, i.codigo) }}</span>
                @if (i.codigo) { <small class="texto-suave">{{ i.codigo }}</small> }
                <button type="button" class="link" (click)="renomear.emit(i)"
                  [attr.aria-label]="'Renomear ' + i.nomeExibicao">renomear</button>
                @if (i.nomeExibicao !== i.nome) { <br><small class="texto-suave">no banco: {{ i.nome }}</small> }
                @if (i.taxaAnual) { <br><small class="texto-suave">{{ i.taxaAnual | number: '1.2-2' }}% a.a.</small> }
                @if (i.jaNaCarteira) { <br><small class="etiqueta etiqueta-alerta">já lançado na carteira — não somado de novo</small> }
              </td>
              <td class="texto-suave">{{ i.nomeInstituicao }}</td>
              <td class="direita">{{ i.valorAplicado !== null ? (i.valorAplicado | currency: i.moeda) : '—' }}
                @if (i.dataAplicacao) { <br><small class="texto-suave">{{ i.dataAplicacao | date: 'dd/MM/yy' }}</small> }</td>
              <td class="direita valor">{{ i.saldo | currency: i.moeda }}
                @if (i.moeda !== 'BRL') {
                  <br><small class="texto-suave">{{ i.saldoEmReais !== null ? (i.saldoEmReais | currency) : 'sem cotação' }}</small>
                }</td>
              <td class="direita" [class.positivo]="(i.rentabilidade ?? 0) > 0" [class.negativo]="(i.rentabilidade ?? 0) < 0">
                {{ pct(i.rentabilidade) }}</td>
              <td class="direita texto-suave">{{ pct(i.selicNoPeriodo, false) }}</td>
              <td class="direita">
                @if (i.percentualDaSelic !== null) {
                  <span class="etiqueta" [class.etiqueta-sucesso]="i.percentualDaSelic >= 100"
                    [class.etiqueta-alerta]="i.percentualDaSelic < 100">{{ i.percentualDaSelic | number: '1.0-0' }}%</span>
                } @else {
                  <span class="texto-suave">—</span>
                }
              </td>
              <td>{{ i.dataVencimento ? (i.dataVencimento | date: 'dd/MM/yyyy') : '—' }}</td>
            </tr>
          } @empty {
            <tr><td colspan="8" class="vazio">Nenhum investimento importado pelo Open Finance.</td></tr>
          }
        </tbody>
      </table>
    </div>
    <p class="texto-suave nota">
      A comparação usa a data e o valor aplicado informados pela instituição. Em produtos com vários aportes (fundos,
      previdência) a instituição pode informar só o total, e o resultado fica aproximado. Valores brutos, antes do IR.
    </p>
    }
    </div>
  `
})
export class InvestimentosOpenFinanceComponent {
  readonly investimentos = input.required<InvestimentoComparado[]>();
  readonly total = input.required<number>();
  /** Quantos FIIs e ações do Open Finance ainda não viraram compra na carteira. */
  readonly naoRegistrados = input(0);
  readonly registrando = input(false);
  /** A grade recolhida (a escolha fica neste navegador, guardada pela tela). */
  readonly recolhida = input(false);
  readonly registrarCompras = output<void>();
  readonly alternarGrade = output<void>();
  readonly renomear = output<InvestimentoComparado>();

  readonly pct = percentual;
  readonly nomeTipoPluggy = nomeTipoPluggy;
}
