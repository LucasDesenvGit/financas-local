import { Component, HostListener, input, output } from '@angular/core';

/** Janela modal simples. O conteúdo vem por projeção. */
@Component({
  selector: 'app-modal',
  template: `
    @if (aberto()) {
      <div class="modal-fundo" (click)="fechar.emit()"></div>
      <section class="modal" role="dialog" aria-modal="true" [attr.aria-label]="titulo()">
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
export class ModalComponent {
  readonly titulo = input('');
  readonly aberto = input(false);
  readonly fechar = output<void>();

  @HostListener('document:keydown.escape')
  aoPressionarEsc(): void {
    if (this.aberto()) this.fechar.emit();
  }
}
