import { Directive, ElementRef, HostListener, effect, inject, input } from '@angular/core';

/**
 * Dica curta para botões e links só com ícone: aparece no hover **e no foco do teclado** (o `title=` não aparece
 * no foco nem no toque). Atende a WCAG 1.4.13: some com Esc sem mover o foco e continua visível enquanto o ponteiro
 * estiver sobre o elemento. O texto também deve estar no aria-label do elemento (a dica é só visual).
 * Uso: <a appDica="Ver transações de Mercado" aria-label="Ver transações de Mercado">…</a>
 */
@Directive({
  selector: '[appDica]',
  host: { class: 'com-dica' }
})
export class DicaDirective {
  readonly appDica = input.required<string>();
  /** Lado em que a dica abre (padrão: acima). */
  readonly dicaLado = input<'acima' | 'abaixo'>('acima');
  private readonly el = inject(ElementRef<HTMLElement>);

  constructor() {
    effect(() => {
      const el = this.el.nativeElement as HTMLElement;
      el.dataset['dica'] = this.appDica();
      el.dataset['dicaLado'] = this.dicaLado();
    });
  }

  @HostListener('keydown.escape')
  dispensar(): void {
    (this.el.nativeElement as HTMLElement).classList.add('dica-dispensada');
  }

  @HostListener('mouseleave')
  @HostListener('blur')
  restaurar(): void {
    (this.el.nativeElement as HTMLElement).classList.remove('dica-dispensada');
  }
}
