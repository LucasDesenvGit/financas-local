import { Component, inject } from '@angular/core';
import { PreferenciaTema, TemaService } from '../core/tema.service';

@Component({
  selector: 'app-seletor-tema',
  template: `
    <div class="seletor-tema" role="radiogroup" aria-label="Tema">
      @for (o of opcoes; track o.valor) {
        <button type="button" role="radio" [attr.aria-checked]="tema.preferencia() === o.valor"
          [class.ativo]="tema.preferencia() === o.valor" (click)="tema.definir(o.valor)" [title]="o.titulo">
          <span aria-hidden="true">{{ o.icone }}</span>
          <span class="seletor-tema-rotulo">{{ o.rotulo }}</span>
        </button>
      }
    </div>
  `
})
export class SeletorTemaComponent {
  readonly tema = inject(TemaService);
  readonly opcoes: { valor: PreferenciaTema; rotulo: string; icone: string; titulo: string }[] = [
    { valor: 'claro', rotulo: 'Claro', icone: '☀', titulo: 'Tema claro' },
    { valor: 'escuro', rotulo: 'Escuro', icone: '☾', titulo: 'Tema escuro' },
    { valor: 'sistema', rotulo: 'Auto', icone: '◐', titulo: 'Seguir o Windows' }
  ];
}
