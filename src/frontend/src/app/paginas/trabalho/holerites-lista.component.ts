import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { MesAno, SeletorMesComponent } from '../../compartilhado/ui/seletor-mes.component';
import { OrigemHolerite, ResumoHolerite, SituacaoHolerite } from '../../core/modelos';
import { textoSituacao } from './holerites';
import { nomeDoMes } from './salario';

/**
 * Coluna da esquerda de Holerites (#777, ADR 0059): escolher o mês para abrir, ler PDFs e a lista dos meses. Componente
 * de apresentação: recebe a lista e avisa o que a pessoa quer fazer; não chama a API nem lê a rota. O `section` segue
 * como filho direto da grade da página (`display: contents`).
 */
@Component({
  selector: 'app-holerites-lista',
  imports: [MoedaPipe, SeletorMesComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    .novo-holerite { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px; }
    .lista-meses { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; }
    .item-mes { width: 100%; display: grid; grid-template-columns: 1fr auto; gap: 2px 8px; text-align: left; padding: 8px 10px;
      border: 1px solid var(--linha); border-radius: 10px; background: var(--superficie); color: inherit; cursor: pointer; min-height: 44px; }
    .item-mes.ativo { border-color: var(--primaria); background: var(--primaria-suave); }
    .mes-liquido { grid-column: 1 / -1; font-variant-numeric: tabular-nums; color: var(--texto-suave); }
    .ler-pdf { margin-bottom: 12px; }
    .ler-pdf p { margin: 4px 0 0; }
    .seletor-pdf { position: relative; display: inline-flex; align-items: center; min-height: 44px; cursor: pointer; }
    .seletor-pdf:focus-within { outline: 2px solid var(--primaria); outline-offset: 2px; }
    .seletor-pdf.desabilitado { opacity: 0.6; cursor: default; }
  `,
  template: `
    <section class="painel-bloco" aria-labelledby="titulo-meses">
      <h2 id="titulo-meses">{{ empregador() }}</h2>
      <div class="novo-holerite">
        <app-seletor-mes [valor]="mesNovo()" (mudou)="mesNovoMudou.emit($event)" />
        <button type="button" class="botao botao-primario" [disabled]="ocupado()" (click)="criar.emit()">Abrir o holerite do mês</button>
      </div>
      <div class="ler-pdf">
        <label class="botao seletor-pdf" [class.desabilitado]="ocupado()">Ler holerite em PDF
          <input type="file" class="so-leitor" accept="application/pdf,.pdf" multiple [disabled]="ocupado()" (change)="escolherPdfs.emit($event)"></label>
        <p class="texto-suave">Um ou vários (o ano de uma vez). Lido no seu computador; o PDF não fica guardado.</p>
      </div>
      @if (lista().length) {
        <ul class="lista-meses">
          @for (r of lista(); track r.id) {
            <li>
              <button type="button" class="item-mes" [class.ativo]="r.id === abertoId()" [attr.aria-current]="r.id === abertoId() ? 'true' : null"
                (click)="abrir.emit(r.id)">
                <span class="mes-nome">{{ nomeDoMes(r.mes) }}</span>
                <span class="etiqueta" [class.etiqueta-suave]="r.situacao === Situacao.Aberto">{{ textoSituacao(r.situacao) }}@if (r.origem === Origem.LidoDoPdf) { · lido do PDF }</span>
                <span class="mes-liquido">{{ r.liquido === null ? 'sem cálculo' : (r.liquido | currency) }}</span>
              </button>
            </li>
          }
        </ul>
      } @else {
        <p class="vazio">Nenhum holerite ainda. Escolha o mês e clique em “Abrir o holerite do mês”.</p>
      }
    </section>
  `
})
export class HoleritesListaComponent {
  readonly empregador = input.required<string>();
  readonly lista = input.required<ResumoHolerite[]>();
  readonly abertoId = input<number | null>(null);
  readonly mesNovo = input.required<MesAno>();
  readonly ocupado = input(false);

  readonly mesNovoMudou = output<MesAno>();
  readonly criar = output<void>();
  readonly abrir = output<number>();
  readonly escolherPdfs = output<Event>();

  readonly Situacao = SituacaoHolerite;
  readonly Origem = OrigemHolerite;
  readonly textoSituacao = textoSituacao;
  readonly nomeDoMes = nomeDoMes;
}
