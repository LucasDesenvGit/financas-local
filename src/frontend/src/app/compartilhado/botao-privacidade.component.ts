import { Component, inject } from '@angular/core';
import { PrivacidadeService } from '../core/privacidade.service';
import { DicaDirective } from './dica.directive';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

/**
 * Ocultar/mostrar valores (UX-7: ícone sempre visível na barra superior, no mesmo lugar em qualquer tela, porque o
 * uso é repentino — alguém chegou perto da tela). aria-pressed indica o estado para leitores de tela.
 */
@Component({
  selector: 'app-botao-privacidade',
  imports: [IconeComponent, DicaDirective],
  template: `
    <button type="button" class="botao-barra botao-privacidade" (click)="privacidade.alternar()"
      [attr.aria-pressed]="privacidade.oculto()" aria-label="Ocultar valores"
      [appDica]="privacidade.oculto() ? 'Mostrar valores' : 'Ocultar valores'" dicaLado="abaixo">
      <app-icone [icone]="privacidade.oculto() ? Icones.mostrar : Icones.ocultar" [tamanho]="18" />
    </button>
  `
})
export class BotaoPrivacidadeComponent {
  readonly privacidade = inject(PrivacidadeService);
  readonly Icones = Icones;
}
