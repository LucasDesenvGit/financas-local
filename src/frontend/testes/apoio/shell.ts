import { Component, NO_ERRORS_SCHEMA } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, provideRouter } from '@angular/router';
import { IconeComponent } from '../../src/app/compartilhado/icone.component';
import { ShellComponent } from '../../src/app/layout/shell.component';

@Component({ template: '<p>tela</p>' })
class TelaFalsaComponent {}

/** matchMedia falso (o jsdom não tem): diz se a tela é de celular e deixa o teste "girar" a tela. */
function instalarMidia(celular: boolean) {
  const ouvintes: ((e: MediaQueryListEvent) => void)[] = [];
  const midia = {
    matches: celular,
    addEventListener: (_: string, f: (e: MediaQueryListEvent) => void) => ouvintes.push(f),
    removeEventListener: () => undefined
  };
  Object.defineProperty(window, 'matchMedia', { configurable: true, writable: true, value: jest.fn(() => midia) });
  return (agoraCelular: boolean) => {
    midia.matches = agoraCelular;
    ouvintes.forEach((f) => f({ matches: agoraCelular } as MediaQueryListEvent));
  };
}

/**
 * Monta a casca do site (menu lateral + barra superior) numa tela falsa. Os componentes da barra que conversam com a
 * API (sincronização, menu do usuário, calculadora…) ficam de fora: aqui só interessa a navegação.
 */
export async function abrirShell(opcoes: { celular?: boolean; recolhidoSalvo?: boolean } = {}) {
  if (opcoes.recolhidoSalvo) localStorage.setItem(ShellComponent.CHAVE_RECOLHIDO, '1');
  const girar = instalarMidia(opcoes.celular ?? false);
  TestBed.configureTestingModule({
    providers: [provideRouter([{ path: '**', component: TelaFalsaComponent }])]
  });
  TestBed.overrideComponent(ShellComponent, {
    set: { imports: [RouterOutlet, RouterLink, RouterLinkActive, IconeComponent], schemas: [NO_ERRORS_SCHEMA] }
  });
  const router = TestBed.inject(Router);
  await router.navigateByUrl('/transacoes');
  const fixture = TestBed.createComponent(ShellComponent);
  fixture.detectChanges();
  await fixture.whenStable();
  const raiz = fixture.nativeElement as HTMLElement;
  const atualizar = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  return {
    fixture, raiz, router, atualizar,
    shell: fixture.componentInstance,
    app: () => raiz.querySelector<HTMLElement>('.app')!,
    lateral: () => raiz.querySelector<HTMLElement>('aside.lateral')!,
    botao: () => raiz.querySelector<HTMLButtonElement>('.barra-superior button.botao-menu')!,
    marca: () => raiz.querySelector<HTMLAnchorElement>('aside .marca')!,
    tituloBarra: () => raiz.querySelector<HTMLAnchorElement>('.barra-superior .barra-titulo')!,
    /** Simula girar o celular ou redimensionar a janela além da quebra de 900 px. */
    async girar(celular: boolean) {
      girar(celular);
      await atualizar();
    }
  };
}
