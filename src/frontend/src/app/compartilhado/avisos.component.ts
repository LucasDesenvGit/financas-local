import { Component, Injectable, inject, signal } from '@angular/core';

export interface Aviso { id: number; texto: string; tipo: 'sucesso' | 'erro' | 'info'; }

@Injectable({ providedIn: 'root' })
export class AvisosService {
  private sequencia = 0;
  readonly avisos = signal<Aviso[]>([]);

  sucesso(texto: string): void { this.adicionar(texto, 'sucesso'); }
  erro(texto: string): void { this.adicionar(texto, 'erro', 7000); }
  info(texto: string): void { this.adicionar(texto, 'info'); }

  fechar(id: number): void { this.avisos.update((lista) => lista.filter((a) => a.id !== id)); }

  private adicionar(texto: string, tipo: Aviso['tipo'], duracao = 4000): void {
    const id = ++this.sequencia;
    this.avisos.update((lista) => [...lista, { id, texto, tipo }]);
    setTimeout(() => this.fechar(id), duracao);
  }
}

@Component({
  selector: 'app-avisos',
  template: `
    <div class="avisos" aria-live="polite">
      @for (a of servico.avisos(); track a.id) {
        <div class="aviso" [class]="'aviso aviso-' + a.tipo" role="status">
          <span>{{ a.texto }}</span>
          <button type="button" class="aviso-fechar" (click)="servico.fechar(a.id)" aria-label="Fechar">×</button>
        </div>
      }
    </div>
  `
})
export class AvisosComponent {
  readonly servico = inject(AvisosService);
}
