import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { catchError, throwError } from 'rxjs';
import { AuthService } from './auth.service';

/** Trata sessão expirada de forma centralizada. */
export const erroInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  return next(req).pipe(
    catchError((erro: unknown) => {
      if (erro instanceof HttpErrorResponse && erro.status === 401 && !req.url.includes('/api/auth/')) {
        auth.sessaoExpirada();
      }
      return throwError(() => erro);
    })
  );
};

/** Extrai uma mensagem legível de um erro da API (MensagemDto, ProblemDetails de validação ou genérico). */
export function mensagemDeErro(erro: unknown): string {
  if (erro instanceof HttpErrorResponse) {
    const corpo = erro.error as { mensagem?: string; title?: string; errors?: Record<string, string[]> } | null;
    if (corpo?.mensagem) return corpo.mensagem;
    if (corpo?.errors) {
      const primeiros = Object.values(corpo.errors).flat();
      if (primeiros.length) return primeiros.join(' ');
    }
    if (erro.status === 0) return 'Não foi possível falar com a API. Verifique se o site está rodando no IIS.';
    if (erro.status === 400 && corpo?.title) return 'Dados inválidos. Revise os campos e tente novamente.';
    if (erro.status === 429) return 'Muitas tentativas em pouco tempo. Aguarde um minuto.';
    if (erro.status === 403) return 'Acesso negado.';
    return `Erro ${erro.status}.`;
  }
  return 'Erro inesperado.';
}
