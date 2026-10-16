import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TermoComponent } from '../../src/app/compartilhado/termo.component';

@Component({
  imports: [TermoComponent],
  template: `<app-termo chave="precoMedio">Preço médio</app-termo>`
})
class Hospedeiro {}

/** Caixa que o "navegador" daria a cada parte do termo (o jsdom não calcula layout). */
function simularLayout(botao: { left: number; right: number }, larguraBalao = 300): void {
  jest.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    if (this.classList.contains('termo-botao')) {
      return { ...botao, top: 76, bottom: 100, width: 24, height: 24, x: botao.left, y: 76 } as DOMRect;
    }
    if (this.classList.contains('termo-balao')) {
      const left = this.style.left ? parseFloat(this.style.left) : window.innerWidth - parseFloat(this.style.right) - larguraBalao;
      return { left, right: left + larguraBalao, top: 106, bottom: 206, width: larguraBalao, height: 100, x: left, y: 106 } as DOMRect;
    }
    return { left: 0, right: 0, top: 0, bottom: 0, width: 0, height: 0, x: 0, y: 0 } as DOMRect;
  });
}

function abrirNaTela(largura: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: largura });
  const fixture = TestBed.createComponent(Hospedeiro);
  fixture.detectChanges();
  const termo = fixture.debugElement.query(By.directive(TermoComponent)).componentInstance as TermoComponent;
  (fixture.nativeElement as HTMLElement).querySelector<HTMLButtonElement>('.termo-botao')!.click();
  fixture.detectChanges();
  fixture.detectChanges();
  return termo;
}

/**
 * Regressão da UX-10 (#42): no celular, o balão do glossário abria cortado pela borda esquerda (de -52 a 248 px numa
 * tela de 390 px), porque era alinhado pela direita do ⓘ sem caber. Agora ele encosta na margem de 8 px.
 */
describe('Regressão: balão do glossário dentro da tela', () => {
  it('ⓘ na metade direita, balão maior que o espaço à esquerda: encosta na margem esquerda', () => {
    simularLayout({ left: 224, right: 248 });
    const termo = abrirNaTela(390);
    expect(termo.posicao()).toMatchObject({ left: 8, right: null });
  });

  it('ⓘ na metade esquerda, balão passaria da borda direita: encosta na margem direita', () => {
    simularLayout({ left: 150, right: 174 });
    const termo = abrirNaTela(390);
    expect(termo.posicao()).toMatchObject({ left: null, right: 8 });
  });

  it('com espaço sobrando, fica alinhado ao ⓘ e logo abaixo dele', () => {
    simularLayout({ left: 300, right: 324 });
    const termo = abrirNaTela(1360);
    expect(termo.posicao()).toEqual({ top: 106, left: 300, right: null });
  });
});
