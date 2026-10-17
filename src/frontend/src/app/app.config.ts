import { registerLocaleData } from '@angular/common';
import localePt from '@angular/common/locales/pt';
import { provideHttpClient, withInterceptors, withXsrfConfiguration } from '@angular/common/http';
import { ApplicationConfig, DEFAULT_CURRENCY_CODE, ErrorHandler, LOCALE_ID, provideZoneChangeDetection } from '@angular/core';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { rotas } from './app.routes';
import { erroInterceptor } from './core/erro.interceptor';
import { ErroGlobalHandler } from './core/erro-global.handler';

registerLocaleData(localePt, 'pt-BR');

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(rotas, withComponentInputBinding()),
    provideHttpClient(
      withInterceptors([erroInterceptor]),
      // O backend emite o cookie __Host-XSRF-TOKEN; o Angular devolve no cabeçalho X-XSRF-TOKEN.
      withXsrfConfiguration({ cookieName: '__Host-XSRF-TOKEN', headerName: 'X-XSRF-TOKEN' })
    ),
    // Erros não tratados também vão para Configurações › Erros (#11).
    { provide: ErrorHandler, useClass: ErroGlobalHandler },
    { provide: LOCALE_ID, useValue: 'pt-BR' },
    { provide: DEFAULT_CURRENCY_CODE, useValue: 'BRL' }
  ]
};
