import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { AuthService } from '../../../src/app/core/auth.service';
import { EntrarComponent } from '../../../src/app/paginas/entrar.component';

/**
 * Regressão do item 4 da #68 (ADR 0075): o primeiro acesso só cria o usuário com o token do link que o gerenciador
 * abre (?configurar=...). O token vai no cabeçalho e, sem ele, a tela só explica como abrir.
 */
describe('Regressão: primeiro acesso com o link do gerenciador (#68)', () => {
  function montar(token?: string) {
    // O seletor de tema lê prefers-color-scheme; o jsdom não tem matchMedia.
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    TestBed.configureTestingModule({ providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()] });
    const http = TestBed.inject(HttpTestingController);
    const fixture = TestBed.createComponent(EntrarComponent);
    if (token) fixture.componentRef.setInput('configurar', token);
    fixture.detectChanges();
    http.expectOne('/api/auth/estado').flush({ configurado: false, autenticado: false, login: null });
    return { fixture, http, texto: () => (fixture.nativeElement as HTMLElement).textContent ?? '' };
  }

  it('o configurar leva o token no cabeçalho X-Configurar-Token', () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([])] });
    const http = TestBed.inject(HttpTestingController);
    TestBed.inject(AuthService).configurar('ana', 'Senha-Forte-123', '123.abc').subscribe();
    const pedido = http.expectOne('/api/auth/configurar');
    expect(pedido.request.headers.get('X-Configurar-Token')).toBe('123.abc');
    expect(pedido.request.body).toEqual({ login: 'ana', senha: 'Senha-Forte-123' });
  });

  it('com o link, mostra o formulário; sem ele, só explica como abrir pelo gerenciador', async () => {
    const com = montar('123.abc');
    await com.fixture.whenStable();
    com.fixture.detectChanges();
    expect(com.texto()).toContain('Criar e entrar');
    TestBed.resetTestingModule();

    const sem = montar();
    await sem.fixture.whenStable();
    sem.fixture.detectChanges();
    expect(sem.texto()).not.toContain('Criar e entrar');
    expect(sem.texto()).toContain('abra o site pelo gerenciador');
  });
});
