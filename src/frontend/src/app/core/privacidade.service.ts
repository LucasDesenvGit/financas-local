import { Injectable, signal } from '@angular/core';

/** "Ocultar valores": esconde quantias na tela (preferência salva só neste navegador). */
@Injectable({ providedIn: 'root' })
export class PrivacidadeService {
  private static readonly CHAVE = 'financas.ocultarValores';
  readonly oculto = signal<boolean>(this.ler());

  constructor() {
    this.aplicarClasse();
  }

  alternar(): void {
    this.oculto.update((v) => !v);
    try {
      localStorage.setItem(PrivacidadeService.CHAVE, this.oculto() ? '1' : '0');
    } catch {
      /* armazenamento indisponível */
    }
    this.aplicarClasse();
  }

  private aplicarClasse(): void {
    document.documentElement.classList.toggle('valores-ocultos', this.oculto());
  }

  private ler(): boolean {
    try {
      return localStorage.getItem(PrivacidadeService.CHAVE) === '1';
    } catch {
      return false;
    }
  }
}
