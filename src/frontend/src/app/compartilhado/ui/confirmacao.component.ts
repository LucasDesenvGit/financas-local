import { ChangeDetectionStrategy, Component, ElementRef, effect, inject, viewChild } from '@angular/core';
import { ModalComponent } from './modal.component';
import { ConfirmacaoService } from './confirmacao.service';

/**
 * #631 (ADR 0057): a janela de confirmação do site, no lugar do `confirm()` do navegador. Fica uma vez só no `layout`
 * (`shell.component.ts`) e mostra o pedido aberto no `ConfirmacaoService`.
 *
 * - WAI-ARIA, "Alert and Message Dialogs": `role="alertdialog"`, a pergunta ligada por `aria-describedby` e o foco
 *   inicial no botão que não faz nada (Cancelar), para um Enter sem olhar não apagar nada.
 * - Nielsen 4 (consistência): o mesmo `<app-modal>` e os mesmos botões do resto do site, no tema claro ou escuro.
 * - Nielsen 5 (prevenção de erros): o botão diz o verbo da ação ("Excluir", e não "OK") e sai vermelho quando a ação
 *   apaga ou não tem volta.
 * - Nielsen 3 (controle e liberdade): Cancelar, Esc e o "×" fecham sem fazer nada.
 */
@Component({
  selector: 'app-confirmacao',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [ModalComponent],
  template: `
    <app-modal [aberto]="!!servico.aberta()" [titulo]="servico.aberta()?.titulo || 'Confirmar'" papel="alertdialog"
               descricao="confirmacao-mensagem" (fechar)="responder(false)">
      @if (servico.aberta(); as p) {
        <p id="confirmacao-mensagem" class="confirmacao-mensagem">{{ p.mensagem }}</p>
        <div class="rodape-form">
          <span class="cresce"></span>
          <button #cancelar type="button" class="botao" (click)="responder(false)">Cancelar</button>
          <button type="button" class="botao" [class.botao-perigo]="p.perigo" [class.botao-primario]="!p.perigo"
                  (click)="responder(true)">{{ p.confirmar || 'Confirmar' }}</button>
        </div>
      }
    </app-modal>
  `,
  styles: [`.confirmacao-mensagem { margin: 0 0 16px; white-space: pre-line; line-height: 1.5; }`]
})
export class ConfirmacaoComponent {
  readonly servico = inject(ConfirmacaoService);
  private readonly cancelar = viewChild<ElementRef<HTMLButtonElement>>('cancelar');

  constructor() {
    // O foco vai para o Cancelar quando a janela abre (o botão só existe depois de ela aparecer).
    effect(() => this.cancelar()?.nativeElement.focus());
  }

  responder(ok: boolean): void {
    this.servico.aberta()?.responder(ok);
  }
}
