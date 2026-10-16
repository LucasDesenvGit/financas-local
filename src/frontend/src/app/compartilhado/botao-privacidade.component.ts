import { Component, inject } from '@angular/core';
import { PrivacidadeService } from '../core/privacidade.service';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

@Component({
  selector: 'app-botao-privacidade',
  imports: [IconeComponent],
  template: `
    <button type="button" class="botao-privacidade" (click)="privacidade.alternar()"
      [attr.aria-pressed]="privacidade.oculto()"
      [title]="privacidade.oculto() ? 'Mostrar valores' : 'Ocultar valores'">
      <app-icone [icone]="privacidade.oculto() ? Icones.mostrar : Icones.ocultar" [tamanho]="16" />
      <span class="botao-privacidade-rotulo">{{ privacidade.oculto() ? 'Mostrar valores' : 'Ocultar valores' }}</span>
    </button>
  `
})
export class BotaoPrivacidadeComponent {
  readonly privacidade = inject(PrivacidadeService);
  readonly Icones = Icones;
}
