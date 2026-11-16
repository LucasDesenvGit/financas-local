import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { Carteira } from '../../core/modelos';
import { listaTickers, percentual } from './investimentos';

/**
 * Os avisos e os cinco números do topo de Investimentos (#731, ADR 0059). Componente de apresentação: recebe a carteira
 * e só avisa quando alguém pede "Conferir investimentos"; o elemento não desenha nada (`display: contents`), então os
 * avisos e os cards ficam no fluxo da página como antes.
 */
@Component({
  selector: 'app-investimentos-indicadores',
  imports: [MoedaPipe, DecimalPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `:host { display: contents; }`,
  template: `
    @let d = dados();
    @if (d.cotasDiferentes.length > 0) {
      <section class="alertas" role="status">
        <p>A corretora informa outra quantidade de cotas de {{ listaTickers(d.cotasDiferentes) }}.
          <button type="button" class="link" (click)="conferir.emit()">Conferir investimentos</button></p>
      </section>
    }
    @if (d.moedasSemCotacao.length > 0) {
      <section class="alertas">
        <p>Investimentos em {{ d.moedasSemCotacao.join(', ') }} ficaram fora dos totais: não há cotação de câmbio
          guardada para essa moeda.</p>
      </section>
    }
    <section class="kpis">
      <article class="kpi kpi-destaque">
        <span>Total investido</span>
        <strong>{{ d.totalGeral | currency }}</strong>
        <small>renda variável + Open Finance + contas de investimento + reservas</small>
      </article>
      <article class="kpi">
        <span>Renda variável (cotação atual)</span>
        <strong>{{ d.rendaVariavel.valorMercado | currency }}</strong>
        <small>custo {{ d.rendaVariavel.custo | currency }} ·
          <span [class.positivo]="d.rendaVariavel.valorizacao >= 0" [class.negativo]="d.rendaVariavel.valorizacao < 0">
            {{ pct(d.rendaVariavel.valorizacaoPercentual) }}</span></small>
      </article>
      <article class="kpi">
        <span>Carteira x Selic</span>
        <strong [class.positivo]="d.rendaVariavel.diferencaSelic >= 0" [class.negativo]="d.rendaVariavel.diferencaSelic < 0">
          {{ d.rendaVariavel.diferencaSelic >= 0 ? '+' : '' }}{{ d.rendaVariavel.diferencaSelic | currency }}</strong>
        <small>carteira {{ pct(d.rendaVariavel.resultadoTotalPercentual) }} · Selic {{ pct(d.rendaVariavel.resultadoSelicPercentual) }}
          no mesmo período</small>
      </article>
      <article class="kpi">
        <span>Proventos em 12 meses</span>
        <strong class="positivo">{{ d.rendaVariavel.proventos12Meses | currency }}</strong>
        <small>{{ d.rendaVariavel.proventosRecebidos | currency }} desde a 1ª compra</small>
      </article>
      <article class="kpi">
        <span><app-termo chave="selic">Selic</app-termo></span>
        <strong>{{ d.selic.metaAnual !== null ? (d.selic.metaAnual | number: '1.2-2') + '% a.a.' : '—' }}</strong>
        <small>12 meses: {{ pct(d.selic.acumulado12Meses, false) }} · no ano: {{ pct(d.selic.acumuladoAno, false) }}</small>
      </article>
    </section>
  `
})
export class InvestimentosIndicadoresComponent {
  readonly dados = input.required<Carteira>();
  /** "Conferir investimentos", no aviso de cotas diferentes. */
  readonly conferir = output<void>();

  readonly listaTickers = listaTickers;
  readonly pct = percentual;
}
