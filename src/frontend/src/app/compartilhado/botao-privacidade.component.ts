import { Component, inject } from '@angular/core';
import { PrivacidadeService } from '../core/privacidade.service';

@Component({
  selector: 'app-botao-privacidade',
  template: `
    <button type="button" class="botao-privacidade" (click)="privacidade.alternar()"
      [attr.aria-pressed]="privacidade.oculto()"
      [title]="privacidade.oculto() ? 'Mostrar valores' : 'Ocultar valores'">
      <span aria-hidden="true">{{ privacidade.oculto() ? '◡' : '◉' }}</span>
      <span class="botao-privacidade-rotulo">{{ privacidade.oculto() ? 'Mostrar valores' : 'Ocultar valores' }}</span>
    </button>
  `
})
export class BotaoPrivacidadeComponent {
  readonly privacidade = inject(PrivacidadeService);
}
