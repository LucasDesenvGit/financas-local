import { Component, computed, input, ChangeDetectionStrategy } from '@angular/core';

let sequencia = 0;

/**
 * UX-9: padrão único para todo gráfico — a conclusão primeiro, o gráfico, e uma linha dizendo como ler.
 *
 *   <app-grafico-explicado [resumo]="resumoX()" comoLer="Barras: total do mês…">
 *     <app-grafico [configuracao]="graficoX()" />
 *   </app-grafico-explicado>
 *
 * O resumo é uma frase calculada pela tela (regra de negócio de cada gráfico) e precisa respeitar "Ocultar valores".
 * Resumo + "como ler" viram a alternativa em texto do canvas (role="img" + aria-label; WCAG 1.1.1), e o "como ler"
 * fica junto do gráfico em vez de depender da memória de quem lê (Nielsen 6). A legenda continua a do próprio gráfico.
 */
@Component({
  selector: 'app-grafico-explicado',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <figure class="grafico-explicado">
      @if (resumo()) { <p class="grafico-resumo" aria-hidden="true">{{ resumo() }}</p> }
      <div class="grafico-area" role="img" [attr.aria-label]="textoAlternativo()">
        <ng-content />
      </div>
      @if (comoLer()) { <figcaption class="grafico-como-ler" [id]="idComoLer">{{ comoLer() }}</figcaption> }
    </figure>
  `
})
export class GraficoExplicadoComponent {
  /** Conclusão principal, em uma frase (null enquanto não há dados). */
  readonly resumo = input<string | null>(null);
  /** Como ler: eixos, cores, linhas de referência (uma linha, texto suave). */
  readonly comoLer = input<string>('');
  readonly idComoLer = `grafico-como-ler-${++sequencia}`;
  readonly textoAlternativo = computed(() =>
    [this.resumo() ? `${this.resumo()}` : 'Gráfico sem dados.', this.comoLer()].filter(Boolean).join(' '));
}
