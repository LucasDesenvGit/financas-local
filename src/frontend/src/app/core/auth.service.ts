import { HttpClient } from '@angular/common/http';
import { Injectable, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, firstValueFrom, tap } from 'rxjs';
import { EstadoAutenticacao } from './modelos';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);
  private readonly router = inject(Router);

  readonly estado = signal<EstadoAutenticacao | null>(null);

  /** Consulta o estado da sessão (também renova o token CSRF). */
  carregarEstado(): Promise<EstadoAutenticacao> {
    return firstValueFrom(
      this.http.get<EstadoAutenticacao>('/api/auth/estado').pipe(tap((e) => this.estado.set(e)))
    );
  }

  entrar(login: string, senha: string): Observable<EstadoAutenticacao> {
    return this.http.post<EstadoAutenticacao>('/api/auth/entrar', { login, senha }).pipe(tap((e) => this.estado.set(e)));
  }

  configurar(login: string, senha: string): Observable<EstadoAutenticacao> {
    return this.http.post<EstadoAutenticacao>('/api/auth/configurar', { login, senha }).pipe(tap((e) => this.estado.set(e)));
  }

  async sair(): Promise<void> {
    try {
      await firstValueFrom(this.http.post<void>('/api/auth/sair', {}));
    } finally {
      this.estado.set({ configurado: true, autenticado: false, login: null });
      await this.router.navigateByUrl('/entrar');
    }
  }

  /** Chamado quando a API responde 401 (sessão expirada). */
  sessaoExpirada(): void {
    const atual = this.estado();
    this.estado.set({ configurado: atual?.configurado ?? true, autenticado: false, login: null });
    if (!this.router.url.startsWith('/entrar')) {
      void this.router.navigate(['/entrar'], { queryParams: { expirada: 1 } });
    }
  }
}
