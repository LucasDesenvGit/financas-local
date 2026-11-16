import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { GraficoExplicadoComponent } from '../../compartilhado/graficos/grafico-explicado.component';
import { GraficoComponent } from '../../compartilhado/graficos/grafico.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { Carteira } from '../../core/modelos';
import {
  alturaGraficoAtivos, configAtivos, configDistribuicao, corDoTipo, posicoesAbertas, resumoAtivos, resumoDistribuicao
} from './investimentos';

/**
 * "Mais análises" do Resumo de Investimentos (#731, ADR 0059): a distribuição por tipo e, havendo posição aberta, a
 * rentabilidade de cada ativo x Selic. Componente de apresentação: só recebe a carteira. Os dois cards e a grade que os
 * junta ficam aqui dentro, para a regra `.grade-2 > .painel-bloco` do `styles.css` seguir valendo.
 */
@Component({
  selector: 'app-investimentos-analises',
  imports: [DecimalPipe, MoedaPipe, GraficoComponent, GraficoExplicadoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    /* #330: a distribuição sozinha ocupa a largura toda, com a rosca e a lista lado a lado. */
    .grade-2.grade-sozinha { grid-template-columns: minmax(0, 1fr); }
    .distribuicao-larga .distribuicao-corpo {
      /* A lista tem largura limitada e o par fica centralizado: em tela larga, o valor não vai para longe do nome. */
      display: grid; grid-template-columns: minmax(220px, 320px) minmax(0, 380px); gap: 8px 56px; align-items: center;
      justify-content: center;
    }
    .distribuicao-larga .lista-legenda { margin-top: 0; }
    @media (max-width: 760px) { .distribuicao-larga .distribuicao-corpo { grid-template-columns: minmax(0, 1fr); } }
  `,
  template: `
    @let d = dados();
    <!-- #330: sem a rentabilidade por ativo, a distribuição ocupa a largura toda (rosca à esquerda, lista à
         direita), em vez de meia largura com o canto vazio (Nielsen 8). -->
    <div class="grade-2" [class.grade-sozinha]="!abertas().length">
      <section class="painel-bloco" [class.distribuicao-larga]="!abertas().length">
        <h2>Distribuição</h2>
        @if (d.distribuicao.length) {
          <div class="distribuicao-corpo">
          <app-grafico-explicado [resumo]="resumoDistribuicao()"
            comoLer="Cada fatia é um tipo de investimento, do tamanho do valor dele hoje. A lista traz os valores.">
            <app-grafico [configuracao]="graficoDistribuicao()" [altura]="200" />
          </app-grafico-explicado>
          <ul class="lista-legenda">
            @for (g of d.distribuicao; track g.nome; let i = $index) {
              <li>
                <span class="ponto" [style.background]="cor(i)"></span>
                <span class="cresce">{{ g.nome }}</span>
                <span class="valor">{{ g.valor | currency }}</span>
                <span class="texto-suave pct">{{ d.totalGeral > 0 ? (g.valor / d.totalGeral * 100 | number: '1.0-0') : 0 }}%</span>
              </li>
            }
          </ul>
          </div>
        } @else { <p class="vazio">Nenhum investimento ainda.</p> }
      </section>
      @if (abertas().length) {
        <section class="painel-bloco">
          <h2>Rentabilidade por ativo x Selic no mesmo período</h2>
          <app-grafico-explicado [resumo]="resumoAtivos()"
            comoLer="Para cada ativo: resultado total (valorização, vendas e proventos) em verde quando ganhou da Selic e em rosa quando perdeu; em laranja-claro, a Selic com as mesmas datas e valores das suas compras.">
            <app-grafico [configuracao]="graficoAtivos()" [altura]="alturaGraficoAtivos()" />
          </app-grafico-explicado>
        </section>
      }
    </div>
  `
})
export class InvestimentosAnalisesComponent {
  readonly dados = input.required<Carteira>();

  readonly abertas = computed(() => posicoesAbertas(this.dados().posicoes));
  readonly alturaGraficoAtivos = computed(() => alturaGraficoAtivos(this.abertas().length));
  readonly resumoDistribuicao = computed(() => resumoDistribuicao(this.dados()));
  readonly resumoAtivos = computed(() => resumoAtivos(this.abertas()));
  readonly graficoDistribuicao = computed(() => configDistribuicao(this.dados().distribuicao));
  readonly graficoAtivos = computed(() => configAtivos(this.abertas()));

  readonly cor = corDoTipo;
}
