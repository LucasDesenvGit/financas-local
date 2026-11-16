import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { GraficoExplicadoComponent } from '../../compartilhado/graficos/grafico-explicado.component';
import { GraficoComponent } from '../../compartilhado/graficos/grafico.component';
import { EvolucaoCarteira } from '../../core/modelos';
import { configEvolucao, resumoEvolucao } from './investimentos';

/**
 * "Seu dinheiro na renda variável x na Selic", o bloco do gráfico do Resumo de Investimentos (#731, ADR 0059).
 * Componente de apresentação: recebe a evolução e se "Ocultar valores" está ligado (a frase-resumo some com os R$, UX-9);
 * a conta e o gráfico vêm de `investimentos.ts`.
 */
@Component({
  selector: 'app-investimentos-evolucao',
  imports: [GraficoComponent, GraficoExplicadoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `:host { display: contents; }`,
  template: `
    <section class="painel-bloco">
      <h2>Seu dinheiro na renda variável x na Selic</h2>
      @if (evolucao().length) {
        <app-grafico-explicado [resumo]="resumo()"
          comoLer="Área lilás: o que você aplicou (compras menos vendas). Tracejado: quanto esse mesmo dinheiro teria hoje na Selic. Ponto verde: valor de mercado atual (proventos não entram neste gráfico).">
          <app-grafico [configuracao]="grafico()" [altura]="270" />
        </app-grafico-explicado>
      } @else { <p class="vazio">Lance suas compras para ver a comparação.</p> }
    </section>
  `
})
export class InvestimentosEvolucaoComponent {
  readonly evolucao = input.required<EvolucaoCarteira[]>();
  readonly oculto = input(false);

  readonly resumo = computed(() => resumoEvolucao(this.evolucao(), this.oculto()));
  readonly grafico = computed(() => configEvolucao(this.evolucao()));
}
