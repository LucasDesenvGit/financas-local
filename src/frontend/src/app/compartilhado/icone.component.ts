import { Component, input } from '@angular/core';
import { LucideAngularModule, LucideIconData } from 'lucide-angular';

/** Ícone decorativo (aria-hidden): o texto ao lado é quem descreve a ação. Herda a cor do texto. */
@Component({
  selector: 'app-icone',
  imports: [LucideAngularModule],
  template: `<lucide-icon [img]="icone()" [size]="tamanho()" [strokeWidth]="traco()" aria-hidden="true" />`,
  styles: [`:host { display: inline-flex; flex-shrink: 0; line-height: 0; }`]
})
export class IconeComponent {
  readonly icone = input.required<LucideIconData>();
  readonly tamanho = input(18);
  readonly traco = input(1.9);
}
