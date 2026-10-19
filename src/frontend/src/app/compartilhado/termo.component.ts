import { Component, ElementRef, HostListener, computed, effect, inject, input, signal, viewChild, ChangeDetectionStrategy } from '@angular/core';
import { ChaveGlossario, DefinicaoTermo, GLOSSARIO } from './glossario';

let sequencia = 0;

/**
 * UX-10: termo financeiro com definição no contexto. O texto do termo continua normal; ao lado, um ⓘ que abre a
 * definição por clique, toque ou teclado (Enter/Espaço) — nunca só por hover, que não existe em toque nem no teclado.
 * Esc ou clique fora fecham sem mover o foco, e o balão não some com o ponteiro sobre ele (WCAG 1.4.13);
 * o ⓘ tem 24×24 px (WCAG 2.5.8). Uso: <app-termo chave="dy">DY</app-termo>.
 */
@Component({
  selector: 'app-termo',
  host: { class: 'termo' },
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `<ng-content /><button type="button" class="termo-botao" [attr.aria-label]="'O que é ' + definicao().termo + '?'"
      [attr.aria-expanded]="aberto()" [attr.aria-controls]="id" [attr.aria-describedby]="aberto() ? id : null"
      (click)="alternar($event)">ⓘ</button>@if (aberto()) {
      <span #balao class="termo-balao" [id]="id" role="note" popover="manual" animate.leave="sair" [style.top.px]="posicao().top"
        [style.left.px]="posicao().left" [style.right.px]="posicao().right">
        <strong>{{ definicao().termo }}</strong>
        <span>{{ definicao().definicao }}</span>
        @if (definicao().exemplo) { <em>{{ definicao().exemplo }}</em> }
      </span>
    }`
})
export class TermoComponent {
  readonly chave = input.required<ChaveGlossario>();
  readonly definicao = computed<DefinicaoTermo>(() => GLOSSARIO[this.chave()]);
  readonly aberto = signal(false);
  /** Posição na tela; o balão vai para a top layer (popover), então nenhum ancestral com transform/overflow o corta. */
  readonly posicao = signal<{ top: number; left: number | null; right: number | null }>({ top: 0, left: 0, right: null });
  readonly id = `termo-${++sequencia}`;
  private readonly el = inject(ElementRef<HTMLElement>);
  private readonly balao = viewChild<ElementRef<HTMLElement>>('balao');

  constructor() {
    effect(() => {
      const b = this.balao()?.nativeElement;
      if (!b || b.matches(':popover-open')) return;
      b.showPopover();
      // Balão mais largo que o espaço ao lado do ⓘ (celular): encosta na margem em vez de sair da tela.
      const r = b.getBoundingClientRect();
      if (r.left < 8) this.posicao.update((p) => ({ ...p, left: 8, right: null }));
      else if (r.right > window.innerWidth - 8) this.posicao.update((p) => ({ ...p, left: null, right: 8 }));
    });
  }

  alternar(ev: Event): void {
    ev.stopPropagation(); // não dispara cliques da linha/cabeçalho onde o termo está
    if (!this.aberto()) {
      // Abaixo do ⓘ; perto da borda direita, alinhado pela direita para não sair da tela.
      const botao = (this.el.nativeElement as HTMLElement).querySelector('.termo-botao')!.getBoundingClientRect();
      const direita = botao.left > window.innerWidth / 2;
      this.posicao.set({
        top: botao.bottom + 6,
        left: direita ? null : Math.max(8, botao.left),
        right: direita ? Math.max(8, window.innerWidth - botao.right) : null
      });
    }
    this.aberto.update((v) => !v);
  }

  @HostListener('document:keydown.escape')
  fecharComEsc(): void {
    this.aberto.set(false);
  }

  /** Posição fixa não acompanha a rolagem: fecha para não ficar solto na tela. */
  @HostListener('window:scroll')
  @HostListener('window:resize')
  fecharAoRolar(): void {
    this.aberto.set(false);
  }

  @HostListener('document:click', ['$event'])
  cliqueFora(ev: MouseEvent): void {
    if (this.aberto() && !(this.el.nativeElement as HTMLElement).contains(ev.target as Node)) this.aberto.set(false);
  }
}
