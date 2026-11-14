import { Injectable, inject, signal } from '@angular/core';
import { Observable, tap } from 'rxjs';
import { ApiService } from './api.service';
import { Modulo } from './modelos';

/**
 * #191 (ADR 0064): os módulos e se cada um está ligado. Lê a lista da API uma vez (o shell pede ao abrir) e a atualiza
 * quando a pessoa liga ou desliga: o menu, a rota do módulo e o Painel leem o mesmo signal, e mudam sem recarregar.
 */
@Injectable({ providedIn: 'root' })
export class ModulosService {
  private readonly api = inject(ApiService);
  /** null enquanto não chegou; [] também quando a leitura falhou (aí todo módulo conta como desligado). */
  readonly lista = signal<Modulo[] | null>(null);
  private pedido = false;

  /** Lê a lista, só na primeira vez. */
  carregar(): void {
    if (this.pedido) return;
    this.pedido = true;
    this.api.modulos().subscribe({ next: (l) => this.lista.set(l), error: () => this.lista.set([]) });
  }

  modulo(id: string): Modulo | undefined {
    return this.lista()?.find((m) => m.id === id);
  }

  ligado(id: string): boolean {
    return this.modulo(id)?.ligado ?? false;
  }

  /** Liga ou desliga e troca o módulo na lista com o que a API devolveu. */
  definir(id: string, ligado: boolean): Observable<Modulo> {
    return this.api.definirModulo(id, ligado).pipe(tap((m) => this.lista.update((l) => trocarModulo(l, m))));
  }
}

/** A lista com o módulo trocado pelo novo (pelo id); sem lista ainda, só ele. */
export function trocarModulo(lista: Modulo[] | null, novo: Modulo): Modulo[] {
  if (!lista) return [novo];
  return lista.some((m) => m.id === novo.id) ? lista.map((m) => (m.id === novo.id ? novo : m)) : [...lista, novo];
}

/** "a.com" · "a.com e b.com" · "a.com, b.com e c.com": os destinos do módulo numa frase. */
export function listarDestinos(destinos: string[]): string {
  if (destinos.length <= 1) return destinos.join('');
  return `${destinos.slice(0, -1).join(', ')} e ${destinos[destinos.length - 1]}`;
}
