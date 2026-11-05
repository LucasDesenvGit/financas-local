import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { AvisosService } from '../../core/avisos.service';

/** Os avisos rápidos (toasts) do AvisosService, no canto da tela. Fica uma vez só, no app.component. */
@Component({
  selector: 'app-avisos',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="avisos" aria-live="polite">
      @for (a of servico.avisos(); track a.id) {
        <div class="aviso" [class]="'aviso aviso-' + a.tipo" role="status" animate.leave="sair"
          (mouseenter)="servico.pausar(a.id)" (mouseleave)="servico.retomar(a.id)"
          (focusin)="servico.pausar(a.id)" (focusout)="servico.retomar(a.id)">
          <span>{{ a.texto }}</span>
          @if (a.acao) {
            <button type="button" class="aviso-acao" (click)="servico.executar(a)">{{ a.acao.rotulo }}</button>
          }
          <button type="button" class="aviso-fechar" (click)="servico.fechar(a.id)" aria-label="Fechar">×</button>
        </div>
      }
    </div>
  `
})
export class AvisosComponent {
  readonly servico = inject(AvisosService);
}
