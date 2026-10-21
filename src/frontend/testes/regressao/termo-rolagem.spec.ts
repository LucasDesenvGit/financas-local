import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TermoComponent } from '../../src/app/compartilhado/termo.component';

/**
 * #247 (CI, celular): o balão do ⓘ fechava a qualquer rolagem da página, e no celular a rolagem que vem com o toque o
 * fechava logo depois de abrir. Agora o balão acompanha o ⓘ e só fecha se o ⓘ saiu da tela (como o calendário, #264).
 */
@Component({ imports: [TermoComponent], template: `<app-termo chave="ptax">PTAX</app-termo>` })
class Hospedeiro {}

const caixa = (top: number) => ({ top, bottom: top + 20, left: 30, right: 50, width: 20, height: 20, x: 30, y: top,
  toJSON: () => ({}) }) as DOMRect;

describe('Regressão: balão do ⓘ ao rolar a página', () => {
  it('rolar com o ⓘ na tela: o balão continua aberto e vai para baixo do ⓘ', () => {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const botao = raiz.querySelector<HTMLButtonElement>('.termo-botao')!;
    botao.click();
    fixture.detectChanges();
    expect(raiz.querySelector('.termo-balao')).not.toBeNull();

    jest.spyOn(botao, 'getBoundingClientRect').mockReturnValue(caixa(200));
    window.dispatchEvent(new Event('scroll'));
    fixture.detectChanges();
    const balao = raiz.querySelector<HTMLElement>('.termo-balao');
    expect(balao).not.toBeNull();
    expect(balao!.style.top).toBe('226px');   // 200 + 20 de altura + 6
  });

  it('rolar até o ⓘ sair da tela: fecha', () => {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const botao = raiz.querySelector<HTMLButtonElement>('.termo-botao')!;
    botao.click();
    fixture.detectChanges();
    jest.spyOn(botao, 'getBoundingClientRect').mockReturnValue(caixa(-300));
    window.dispatchEvent(new Event('scroll'));
    fixture.detectChanges();
    expect(raiz.querySelector('.termo-balao')).toBeNull();
  });
});
