import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import type { LucideIcon } from '@lucide/angular';
import { IconeComponent } from './icone.component';
import { IconesCategoria } from './icones';
import { palavras } from './servicos';

/** #336: o ícone de uma categoria pelo nome (o mesmo do selo das transações, #265); null se não é uma das padrão. */
export function iconeDaCategoria(nome: string | null | undefined): LucideIcon | null {
  return IconesCategoria[palavras(nome).join(' ')] ?? null;
}

/**
 * #336: ícone da categoria num quadrado arredondado, na cor dela, com o fundo dessa cor suave, no lugar da bolinha de
 * cor (Painel, Análises, Categorias e regras, Orçamentos). Categoria sem ícone conhecido: a primeira letra do nome.
 * Decorativo (o nome vem escrito ao lado, WCAG 1.1.1).
 *
 * As cores das categorias são claras: no tema claro, o desenho mistura a cor com a do texto forte (meio a meio); no
 * escuro, vai na cor pura. Nos dois, pelo menos 3:1 sobre o fundo suave (WCAG 1.4.11), conferido em
 * `contraste-tema.spec.ts`.
 *
 * Uso: <app-icone-categoria [nome]="c.nome" [cor]="c.cor" />.
 */
@Component({
  selector: 'app-icone-categoria',
  imports: [IconeComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <span class="icone-categoria" [style.--cor]="cor() || '#a9b4c6'" [style.width.px]="tamanho()" [style.height.px]="tamanho()"
      [style.border-radius.px]="tamanho() * 0.3" aria-hidden="true">
      @if (icone(); as i) {
        <app-icone [icone]="i" [tamanho]="tamanhoDesenho()" [traco]="2" />
      } @else {
        <span class="inicial" [style.font-size.px]="tamanho() * 0.46">{{ inicial() }}</span>
      }
    </span>
  `,
  styles: `
    :host { display: inline-flex; flex: none; vertical-align: middle; }
    .icone-categoria {
      display: inline-grid; place-items: center;
      background: color-mix(in srgb, var(--cor) 22%, var(--superficie));
      color: color-mix(in srgb, var(--cor) 50%, var(--texto-forte));
    }
    :host-context([data-tema="escuro"]) .icone-categoria { color: var(--cor); }
    .inicial { font-weight: 700; line-height: 1; }
  `
})
export class IconeCategoriaComponent {
  readonly nome = input<string | null | undefined>('');
  readonly cor = input<string | null | undefined>(null);
  readonly tamanho = input(28);

  readonly icone = computed(() => iconeDaCategoria(this.nome()));
  readonly tamanhoDesenho = computed(() => Math.round(this.tamanho() * 0.57));
  readonly inicial = computed(() => (this.nome() ?? '').trim().charAt(0).toUpperCase() || '?');
}
