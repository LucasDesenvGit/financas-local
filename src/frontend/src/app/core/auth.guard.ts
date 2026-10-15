import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from './auth.service';

export const autenticadoGuard: CanActivateFn = async () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  try {
    const estado = await auth.carregarEstado();
    return estado.autenticado ? true : router.parseUrl('/entrar');
  } catch {
    return router.parseUrl('/entrar');
  }
};

export const anonimoGuard: CanActivateFn = async () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  try {
    const estado = await auth.carregarEstado();
    return estado.autenticado ? router.parseUrl('/painel') : true;
  } catch {
    return true;
  }
};
