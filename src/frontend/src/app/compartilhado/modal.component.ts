import { Component, HostListener, OnDestroy, computed, effect, input, output, signal, untracked } from '@angular/core';

/** Modais abertos, na ordem em que foram abertos (o último fica por cima). Compartilhado por todas as instâncias. */
const pilha = signal<ModalComponent[]>([]);

/** z-index base do fundo (ver .modal-fundo em styles.css); cada modal empilhado sobe 2, abaixo dos avisos (60). */
const Z_BASE = 40;
const Z_MAXIMO = 58;

/**
 * Janela modal simples. O conteúdo vem por projeção.
 * Com mais de um aberto (ex.: editar uma operação a partir da lista de operações), o aberto por último fica por cima,
 * independentemente da ordem no template, e o Esc fecha só ele.
 */
@Component({
  selector: 'app-modal',
  template: `
    @if (aberto()) {
      <div class="modal-fundo" [style.z-index]="zIndice()" (click)="fechar.emit()"></div>
      <section class="modal" [style.z-index]="zIndice() + 1" role="dialog" aria-modal="true" [attr.aria-label]="titulo()">
        <header class="modal-cabecalho">
          <h2>{{ titulo() }}</h2>
          <button type="button" class="botao-icone" (click)="fechar.emit()" aria-label="Fechar">×</button>
        </header>
        <div class="modal-corpo">
          <ng-content />
        </div>
      </section>
    }
  `
})
export class ModalComponent implements OnDestroy {
  readonly titulo = input('');
  readonly aberto = input(false);
  readonly fechar = output<void>();

  private readonly nivel = computed(() => Math.max(0, pilha().indexOf(this)));
  readonly zIndice = computed(() => Math.min(Z_BASE + this.nivel() * 2, Z_MAXIMO));

  constructor() {
    effect(() => {
      const aberto = this.aberto();
      // Depende só do próprio "aberto": abrir/fechar outros modais não reexecuta este effect.
      const atual = untracked(pilha);
      if (aberto && !atual.includes(this)) pilha.set([...atual, this]);
      else if (!aberto && atual.includes(this)) pilha.set(atual.filter((m) => m !== this));
    });
  }

  ngOnDestroy(): void {
    pilha.update((atual) => atual.filter((m) => m !== this));
  }

  @HostListener('document:keydown.escape')
  aoPressionarEsc(): void {
    // Só o modal do topo responde: com dois abertos, um Esc fecha um de cada vez.
    const atual = pilha();
    if (this.aberto() && atual[atual.length - 1] === this) this.fechar.emit();
  }
}
