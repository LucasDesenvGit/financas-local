import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { BotaoPrivacidadeComponent } from '../../src/app/compartilhado/botao-privacidade.component';
import { IconeComponent } from '../../src/app/compartilhado/icone.component';
import { Icones } from '../../src/app/compartilhado/icones';

/**
 * Regressão: o ícone de "ocultar valores" parecia invertido. Com os valores ocultos, o botão ficava destacado
 * (aria-pressed) com o olho ABERTO, que se lê como "valores à mostra". Agora o ícone mostra o estado, como nos
 * apps de banco (olho riscado = ocultos), e a dica diz a ação do clique.
 */
describe('Regressão: ícone de ocultar valores mostra o estado', () => {
  afterEach(() => document.documentElement.classList.remove('valores-ocultos'));

  function montar() {
    const fixture = TestBed.createComponent(BotaoPrivacidadeComponent);
    fixture.detectChanges();
    return {
      fixture,
      botao: () => (fixture.nativeElement as HTMLElement).querySelector('button')!,
      icone: () => fixture.debugElement.query(By.directive(IconeComponent)).componentInstance.icone()
    };
  }

  it('valores à mostra: olho aberto e botão não pressionado', () => {
    const t = montar();
    expect(t.icone()).toBe(Icones.mostrar);
    expect(t.botao().getAttribute('aria-pressed')).toBe('false');
  });

  it('valores ocultos: olho riscado com o botão pressionado (os dois sinais dizem o mesmo)', () => {
    const t = montar();
    t.botao().click();
    t.fixture.detectChanges();
    expect(t.icone()).toBe(Icones.ocultar);
    expect(t.botao().getAttribute('aria-pressed')).toBe('true');
    expect(t.botao().getAttribute('aria-label')).toBe('Ocultar valores');   // nome fixo; o estado vai no aria-pressed
  });
});
