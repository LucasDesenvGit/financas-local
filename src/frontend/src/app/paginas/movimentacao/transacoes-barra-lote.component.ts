import { Component, ChangeDetectionStrategy, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { CategoriaRotulada } from './transacoes';

/**
 * Ações nas transações selecionadas de Transações (#133). Componente de apresentação (#475, ADR 0059): mostra quantas
 * estão selecionadas e avisa a tela de cada ação; não chama a API. O espaço que abre e fecha animando a altura (e o
 * `inert` sem seleção) fica na tela; aqui, `display: contents`. Esc limpa a seleção (WCAG 2.1.1).
 */
@Component({
  selector: 'app-transacoes-barra-lote',
  imports: [ListaSuspensaDirective, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="barra-lote" (keydown.escape)="limpar.emit()" aria-label="Ações nas transações selecionadas">
      <span class="contagem-selecao">{{ textoSelecao() }}</span>
      <select [(ngModel)]="categoria">
        <option [ngValue]="null">Categorização automática</option>
        @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ c.rotulo }}</option> }
      </select>
      <button type="button" class="botao botao-primario" (click)="categorizar.emit()">
        {{ modoEdicao() ? 'Trocar nas selecionadas' : 'Aplicar categoria' }}</button>
      <button type="button" class="botao" (click)="renomear.emit()">✎ Renomear</button>
      @if (poucoInformativa()) {
        <button type="button" class="botao" (click)="marcarRevisadas.emit()">Marcar como revisadas</button>
      }
      <button type="button" class="botao botao-emergencia" (click)="marcarEmergencia.emit(true)">Marcar emergência</button>
      <button type="button" class="botao" (click)="marcarEmergencia.emit(false)">Desmarcar emergência</button>
      <button type="button" class="botao" (click)="limpar.emit()">✕ Limpar seleção</button>
    </section>
  `,
  styles: `:host { display: contents; }`
})
export class TransacoesBarraLoteComponent {
  /** "5 selecionadas (2 em outras páginas)". */
  readonly textoSelecao = input('');
  readonly categorias = input<CategoriaRotulada[]>([]);
  /** No modo edição, a categoria só vira troca pendente ("Trocar nas selecionadas"). */
  readonly modoEdicao = input(false);
  /** Filtro "Descrição pouco informativa": aparece "Marcar como revisadas". */
  readonly poucoInformativa = input(false);
  /** A categoria escolhida para as selecionadas (null = automática). */
  readonly categoria = model<number | null>(null);

  readonly categorizar = output<void>();
  readonly renomear = output<void>();
  readonly marcarRevisadas = output<void>();
  readonly marcarEmergencia = output<boolean>();
  readonly limpar = output<void>();
}
