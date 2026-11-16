import { Component, ChangeDetectionStrategy, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { Meta } from '../../core/modelos';

/**
 * Janela "Aporte" da tela de Metas (#732, ADR 0059). Componente de apresentação: recebe a meta, guarda o valor digitado
 * e avisa em `registrar`; não chama a API. O valor negativo é uma retirada. Fechar sem registrar não muda nada.
 */
@Component({
  selector: 'app-metas-aporte',
  imports: [FormsModule, ModalComponent, CampoValorComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="meta() !== null" [titulo]="'Aporte · ' + (meta()?.nome ?? '')" (fechar)="fechar.emit()">
      <form (ngSubmit)="registrarValor()" class="formulario">
        <label>Valor — use negativo para retirada
          <app-campo-valor name="aporte" [ngModel]="valor()" (ngModelChange)="valor.set($event)" obrigatorio negativo />
        </label>
        <div class="rodape-form">
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
          <button class="botao botao-primario">Registrar</button>
        </div>
      </form>
    </app-modal>
  `
})
export class MetasAporteComponent {
  /** A meta que recebe o aporte (null = janela fechada). */
  readonly meta = input<Meta | null>(null);

  /** "Registrar", com o valor (negativo = retirada). */
  readonly registrar = output<number>();
  readonly fechar = output<void>();

  /** O valor digitado: vazio a cada meta que abre a janela. */
  readonly valor = linkedSignal<Meta | null, number | null>({ source: this.meta, computation: () => null });

  registrarValor(): void {
    const valor = this.valor();
    if (!this.meta() || !valor) return;
    this.registrar.emit(Number(valor));
  }
}
