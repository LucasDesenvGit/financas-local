import { HttpErrorResponse } from '@angular/common/http';
import { ErrorHandler, Injectable, Injector, inject } from '@angular/core';
import { ApiService } from './api.service';
import { AuthService } from './auth.service';

/**
 * Erros não tratados no navegador também vão para Configurações › Erros (#11, ADR 0022), além do console.
 * Fica de fora: erro de chamada à API (a API já registra o que deu errado nela), quem não está logado (o envio
 * daria 401) e repetições. No máximo {@link ErroGlobalHandler.MAXIMO_POR_CARGA} por carga da página; a API ainda
 * limita por hora e limpa dados sensíveis.
 */
@Injectable()
export class ErroGlobalHandler implements ErrorHandler {
  static readonly MAXIMO_POR_CARGA = 10;

  // ApiService/AuthService só na hora de enviar: o ErrorHandler é criado antes de boa parte do app.
  private readonly injector = inject(Injector);
  private readonly enviados = new Set<string>();

  handleError(erro: unknown): void {
    console.error(erro);
    try {
      this.reportar(erro);
    } catch {
      // Reportar nunca pode virar outro erro.
    }
  }

  private reportar(erro: unknown): void {
    // Promise rejeitada chega embrulhada ({ rejection }).
    const e = (erro as { rejection?: unknown } | null)?.rejection ?? erro;
    if (e instanceof HttpErrorResponse) return;
    if (!this.injector.get(AuthService).estado()?.autenticado) return;
    const mensagem = (e instanceof Error ? `${e.name}: ${e.message}` : String(e)).slice(0, 1000);
    if (this.enviados.has(mensagem) || this.enviados.size >= ErroGlobalHandler.MAXIMO_POR_CARGA) return;
    this.enviados.add(mensagem);
    const detalhe = e instanceof Error && e.stack ? e.stack.slice(0, 8000) : null;
    this.injector.get(ApiService)
      .registrarErroFrontEnd({ mensagem, detalhe, rota: location.pathname.slice(0, 280) })
      .subscribe({ error: () => undefined });
  }
}
