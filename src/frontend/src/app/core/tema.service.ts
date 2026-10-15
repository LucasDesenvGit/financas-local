import { Injectable, signal } from '@angular/core';

export type PreferenciaTema = 'claro' | 'escuro' | 'sistema';
export type Tema = 'claro' | 'escuro';

/**
 * Tema claro/escuro. A preferência fica só neste navegador (localStorage);
 * "sistema" acompanha a configuração do Windows.
 */
@Injectable({ providedIn: 'root' })
export class TemaService {
  private static readonly CHAVE = 'financas.tema';
  private readonly midia = window.matchMedia('(prefers-color-scheme: dark)');

  readonly preferencia = signal<PreferenciaTema>(this.lerPreferencia());
  /** Tema efetivamente aplicado (os gráficos observam este sinal para redesenhar). */
  readonly tema = signal<Tema>('claro');

  constructor() {
    this.aplicar();
    this.midia.addEventListener('change', () => {
      if (this.preferencia() === 'sistema') this.aplicar();
    });
  }

  definir(preferencia: PreferenciaTema): void {
    this.preferencia.set(preferencia);
    try {
      localStorage.setItem(TemaService.CHAVE, preferencia);
    } catch {
      /* armazenamento indisponível: vale só nesta sessão */
    }
    this.aplicar();
  }

  alternar(): void {
    this.definir(this.tema() === 'escuro' ? 'claro' : 'escuro');
  }

  private aplicar(): void {
    const pref = this.preferencia();
    const tema: Tema = pref === 'sistema' ? (this.midia.matches ? 'escuro' : 'claro') : pref;
    document.documentElement.dataset['tema'] = tema;
    this.tema.set(tema);
  }

  private lerPreferencia(): PreferenciaTema {
    try {
      const valor = localStorage.getItem(TemaService.CHAVE);
      if (valor === 'claro' || valor === 'escuro' || valor === 'sistema') return valor;
    } catch {
      /* ignora */
    }
    return 'sistema';
  }
}
