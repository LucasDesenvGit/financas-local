import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { LucideIcon } from '@lucide/angular';
import { PreferenciaTema, TemaService } from '../core/tema.service';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

@Component({
  selector: 'app-seletor-tema',
  imports: [IconeComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="seletor-tema" role="radiogroup" aria-label="Tema">
      @for (o of opcoes; track o.valor) {
        <button type="button" role="radio" [attr.aria-checked]="tema.preferencia() === o.valor"
          [class.ativo]="tema.preferencia() === o.valor" (click)="tema.definir(o.valor)" [title]="o.titulo">
          <app-icone [icone]="o.icone" [tamanho]="14" />
          <span class="seletor-tema-rotulo">{{ o.rotulo }}</span>
        </button>
      }
    </div>
  `
})
export class SeletorTemaComponent {
  readonly tema = inject(TemaService);
  readonly opcoes: { valor: PreferenciaTema; rotulo: string; icone: LucideIcon; titulo: string }[] = [
    { valor: 'claro', rotulo: 'Claro', icone: Icones.claro, titulo: 'Tema claro' },
    { valor: 'escuro', rotulo: 'Escuro', icone: Icones.escuro, titulo: 'Tema escuro' },
    { valor: 'sistema', rotulo: 'Auto', icone: Icones.sistema, titulo: 'Seguir o sistema' }
  ];
}
