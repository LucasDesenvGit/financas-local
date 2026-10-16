// Primeiro import: carrega zone.js e o compilador JIT, que o resto do Angular precisa ao ser importado.
import { setupZoneTestEnv } from 'jest-preset-angular/setup-env/zone';
import { registerLocaleData } from '@angular/common';
import { DEFAULT_CURRENCY_CODE, LOCALE_ID } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import localePt from '@angular/common/locales/pt';
import { instalarPopoverFalso } from './apoio/popover';

// O app usa zone.js (provideZoneChangeDetection em app.config.ts) e pt-BR em datas e moeda.
setupZoneTestEnv();
registerLocaleData(localePt, 'pt-BR');
instalarPopoverFalso();

// Mesmo idioma e moeda do app (app.config.ts): números e datas como a pessoa vê (1.000, 16/10/2026, R$).
beforeEach(() => {
  TestBed.configureTestingModule({
    providers: [{ provide: LOCALE_ID, useValue: 'pt-BR' }, { provide: DEFAULT_CURRENCY_CODE, useValue: 'BRL' }]
  });
});

afterEach(() => {
  localStorage.clear();
  jest.restoreAllMocks();
});
