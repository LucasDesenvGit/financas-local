import { Component, ChangeDetectionStrategy, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Conta } from '../../core/modelos';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { FormTransferencia } from './transacoes';

/**
 * Janela "Transferência entre contas" da tela de Transações. Componente de apresentação (#475, ADR 0059): recebe a
 * transferência e as contas, edita uma cópia e avisa em `salvar` com ela; não chama a API.
 */
@Component({
  selector: 'app-transacoes-transferencia',
  imports: [ListaSuspensaDirective, FormsModule, ModalComponent, CampoDataComponent, CampoValorComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="transferencia() !== null" titulo="Transferência entre contas" (fechar)="fechar.emit()">
      @if (dados(); as tf) {
        <form (ngSubmit)="salvar.emit(tf)" #ft="ngForm" class="formulario">
          <p class="texto-suave">Os lançamentos são criados apenas nas contas manuais. O lado do Open Finance chega pela sincronização.</p>
          <div class="grade-campos">
            <label>De
              <select name="origem" [(ngModel)]="tf.contaOrigemId" required>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Para
              <select name="destino" [(ngModel)]="tf.contaDestinoId" required>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Data <app-campo-data name="data" [(ngModel)]="tf.data" obrigatorio /></label>
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="tf.valor" obrigatorio [minimo]="0.01" /></label>
          </div>
          <label>Descrição <input name="descricao" [(ngModel)]="tf.descricao" maxlength="200" placeholder="Transferência"></label>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="ft.invalid || salvando()">Transferir</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class TransacoesTransferenciaComponent {
  /** A transferência da janela (null = fechada). */
  readonly transferencia = input<FormTransferencia | null>(null);
  readonly contas = input<Conta[]>([]);
  readonly salvando = input(false);

  /** "Transferir", com a cópia editada. */
  readonly salvar = output<FormTransferencia>();
  readonly fechar = output<void>();

  /** A cópia que a janela edita: refeita a cada transferência nova que chega. */
  readonly dados = linkedSignal(() => {
    const t = this.transferencia();
    return t ? { ...t } : null;
  });
}
