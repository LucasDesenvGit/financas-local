import { Component, input, ChangeDetectionStrategy } from '@angular/core';
import { LucideDynamicIcon, LucideIcon } from '@lucide/angular';

/** Ícone decorativo (aria-hidden): o texto ao lado é quem descreve a ação. Herda a cor do texto. */
@Component({
  selector: 'app-icone',
  imports: [LucideDynamicIcon],
  template: `<svg [lucideIcon]="icone()" [size]="tamanho()" [strokeWidth]="traco()" aria-hidden="true"></svg>`,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [`:host { display: inline-flex; flex-shrink: 0; line-height: 0; }`]
})
export class IconeComponent {
  readonly icone = input.required<LucideIcon>();
  readonly tamanho = input(18);
  readonly traco = input(1.9);
}
