import { Component, inject } from '@angular/core';
import { PrivacidadeService } from '../core/privacidade.service';
import { DicaDirective } from './dica.directive';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

/**
 * Ocultar/mostrar valores (UX-7: ícone sempre visível na barra superior, no mesmo lugar em qualquer tela, porque o
 * uso é repentino — alguém chegou perto da tela). aria-pressed indica o estado para leitores de tela.
 * O ícone mostra o ESTADO, como nos apps de banco: olho riscado (e botão destacado) com os valores ocultos, olho
 * aberto com eles à mostra (Nielsen 2 e 4). A dica diz a ação do clique. Antes o ícone mostrava a ação e brigava
 * com o destaque: olho aberto destacado parecia "valores à mostra".
 */
@Component({
  selector: 'app-botao-privacidade',
  imports: [IconeComponent, DicaDirective],
  template: `
    <button type="button" class="botao-barra botao-privacidade" (click)="privacidade.alternar()"
      [attr.aria-pressed]="privacidade.oculto()" aria-label="Ocultar valores"
      [appDica]="privacidade.oculto() ? 'Mostrar valores' : 'Ocultar valores'" dicaLado="abaixo">
      <app-icone [icone]="privacidade.oculto() ? Icones.ocultar : Icones.mostrar" [tamanho]="18" />
    </button>
  `
})
export class BotaoPrivacidadeComponent {
  readonly privacidade = inject(PrivacidadeService);
  readonly Icones = Icones;
}
