import { Component, ElementRef, HostListener, computed, inject, input, output, signal } from '@angular/core';

let sequencia = 0;

/** Uma opção da lista; `nivel` 1 aparece recuada (ex.: subcategoria logo abaixo da categoria). */
export interface OpcaoMultipla {
  valor: string;
  rotulo: string;
  cor?: string;
  nivel?: 0 | 1;
}

/**
 * Seleção de vários valores com caixas de marcar, no lugar do <select multiple> (que pede Ctrl+clique e não mostra o
 * que está marcado). O botão resume a escolha ("Todas", "Mercado", "3 categorias") e abre a lista; cada caixa vale
 * na hora. Botão nativo com aria-expanded/aria-controls (WCAG 4.1.2), lista em role="group" com nome, Esc fecha e
 * devolve o foco ao botão, clique fora fecha (WCAG 2.1.1 e 1.4.13). Nielsen 1: o resumo mostra o estado sem abrir.
 *
 *   <app-seletor-multiplo rotulo="Categoria" [opcoes]="opcoes()" [selecionadas]="valores" (mudou)="aplicar($event)" />
 */
@Component({
  selector: 'app-seletor-multiplo',
  host: { class: 'seletor-multiplo' },
  template: `
    <span class="seletor-multiplo-rotulo" [id]="id + '-rotulo'">{{ rotulo() }}</span>
    <button type="button" class="seletor-multiplo-botao" [attr.aria-expanded]="aberto()" [attr.aria-controls]="id"
      [attr.aria-labelledby]="id + '-rotulo ' + id + '-resumo'" (click)="alternar()">
      <span class="seletor-multiplo-resumo" [id]="id + '-resumo'">{{ resumo() }}</span>
      <span class="seletor-multiplo-seta" aria-hidden="true">▾</span>
    </button>
    @if (aberto()) {
      <div class="seletor-multiplo-painel" [id]="id" role="group" [attr.aria-labelledby]="id + '-rotulo'" animate.leave="sair">
        <button type="button" class="link seletor-multiplo-limpar" [disabled]="!selecionadas().length" (click)="limpar()">
          {{ rotuloVazio() }}
        </button>
        @for (o of opcoes(); track o.valor) {
          <label class="checkbox seletor-multiplo-opcao" [class.sub]="o.nivel === 1">
            <input type="checkbox" [checked]="marcadas().has(o.valor)" (change)="alternarOpcao(o.valor)">
            @if (o.cor) { <span class="ponto" [style.background]="o.cor"></span> }
            <span>{{ o.rotulo }}</span>
          </label>
        }
      </div>
    }
  `
})
export class SeletorMultiploComponent {
  readonly rotulo = input.required<string>();
  readonly opcoes = input.required<OpcaoMultipla[]>();
  readonly selecionadas = input<string[]>([]);
  /** Texto quando nada está marcado (e do botão que desmarca tudo). */
  readonly rotuloVazio = input('Todas');
  /** Plural usado no resumo com mais de uma marcada ("3 categorias"). */
  readonly plural = input('categorias');
  readonly mudou = output<string[]>();

  readonly id = `seletor-multiplo-${++sequencia}`;
  readonly aberto = signal(false);
  readonly marcadas = computed(() => new Set(this.selecionadas()));
  readonly resumo = computed(() => {
    const sel = this.selecionadas();
    if (!sel.length) return this.rotuloVazio();
    if (sel.length === 1) return this.opcoes().find((o) => o.valor === sel[0])?.rotulo ?? '1 selecionada';
    return `${sel.length} ${this.plural()}`;
  });
  private readonly el = inject(ElementRef<HTMLElement>);

  alternar(): void {
    this.aberto.update((v) => !v);
  }

  alternarOpcao(valor: string): void {
    const atual = this.selecionadas();
    this.mudou.emit(atual.includes(valor) ? atual.filter((v) => v !== valor) : [...atual, valor]);
  }

  limpar(): void {
    this.mudou.emit([]);
  }

  @HostListener('keydown.escape')
  fecharComEsc(): void {
    if (!this.aberto()) return;
    this.aberto.set(false);
    (this.el.nativeElement as HTMLElement).querySelector<HTMLButtonElement>('.seletor-multiplo-botao')?.focus();
  }

  @HostListener('document:click', ['$event'])
  cliqueFora(ev: MouseEvent): void {
    if (this.aberto() && !(this.el.nativeElement as HTMLElement).contains(ev.target as Node)) this.aberto.set(false);
  }
}
