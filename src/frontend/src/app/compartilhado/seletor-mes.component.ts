import { Component, computed, input, output } from '@angular/core';
import { NOMES_MESES } from './utilidades';

export interface MesAno { ano: number; mes: number; }

@Component({
  selector: 'app-seletor-mes',
  template: `
    <div class="seletor-mes" role="group" aria-label="Selecionar mês">
      <button type="button" class="botao-icone" (click)="mover(-1)" aria-label="Mês anterior">‹</button>
      <span class="seletor-mes-rotulo">{{ rotulo() }}</span>
      <button type="button" class="botao-icone" (click)="mover(1)" aria-label="Próximo mês">›</button>
    </div>
  `
})
export class SeletorMesComponent {
  readonly valor = input.required<MesAno>();
  readonly mudou = output<MesAno>();

  readonly rotulo = computed(() => `${NOMES_MESES[this.valor().mes - 1]} de ${this.valor().ano}`);

  mover(delta: number): void {
    const d = new Date(this.valor().ano, this.valor().mes - 1 + delta, 1);
    this.mudou.emit({ ano: d.getFullYear(), mes: d.getMonth() + 1 });
  }
}
