import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MesAno, SeletorMesComponent } from '../../src/app/compartilhado/seletor-mes.component';

/**
 * Usabilidade do seletor de mês (Painel, Orçamentos). ‹ › andam um mês; o nome do mês abre um quadro com o ano e os
 * 12 meses para pular direto (Nielsen 7, Flexibility and Efficiency of Use); Esc fecha e devolve o foco (Nielsen 3,
 * WCAG 2.1.1); o botão diz aria-expanded e cada mês tem nome completo (WCAG 4.1.2).
 */
describe('Usabilidade: seletor de mês', () => {
  @Component({
    imports: [SeletorMesComponent],
    template: `<app-seletor-mes [valor]="valor" (mudou)="valor = $event; mudancas.push($event)" />`
  })
  class Hospedeiro { valor: MesAno = { ano: 2026, mes: 10 }; mudancas: MesAno[] = []; }

  function montar() {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const botaoMes = () => raiz.querySelector<HTMLButtonElement>('.seletor-mes-rotulo')!;
    const painel = () => raiz.querySelector<HTMLElement>('.seletor-mes-painel');
    const clicar = (el: HTMLElement) => { el.click(); fixture.detectChanges(); };
    return { fixture, raiz, botaoMes, painel, clicar, hospedeiro: fixture.componentInstance };
  }

  it('o nome do mês é um botão que abre o quadro de mês e ano', () => {
    const t = montar();
    expect(t.botaoMes().tagName).toBe('BUTTON');
    expect(t.botaoMes().getAttribute('aria-expanded')).toBe('false');
    t.clicar(t.botaoMes());
    expect(t.botaoMes().getAttribute('aria-expanded')).toBe('true');
    expect(t.painel()!.getAttribute('role')).toBe('dialog');
    expect(t.painel()!.querySelectorAll('.seletor-mes-opcao').length).toBe(12);
    expect(t.painel()!.querySelector('[aria-pressed="true"]')!.getAttribute('aria-label')).toBe('outubro de 2026');
  });

  it('pula vários meses de uma vez: muda o ano no quadro e escolhe o mês', () => {
    const t = montar();
    t.clicar(t.botaoMes());
    t.clicar(t.painel()!.querySelector<HTMLButtonElement>('[aria-label="Ano anterior"]')!);
    t.clicar(t.painel()!.querySelector<HTMLButtonElement>('[aria-label="março de 2025"]')!);
    expect(t.hospedeiro.mudancas).toEqual([{ ano: 2025, mes: 3 }]);
    expect(t.painel()).toBeNull();
    expect(t.botaoMes().textContent).toContain('março de 2025');
  });

  it('mudar o ano no quadro não troca o mês da tela até escolher um mês', () => {
    const t = montar();
    t.clicar(t.botaoMes());
    t.clicar(t.painel()!.querySelector<HTMLButtonElement>('[aria-label="Próximo ano"]')!);
    expect(t.hospedeiro.mudancas).toEqual([]);
    expect(t.painel()!.querySelector('[aria-pressed="true"]')).toBeNull();
  });

  it('"Este mês" volta ao mês atual', () => {
    const t = montar();
    t.clicar(t.botaoMes());
    t.clicar(t.painel()!.querySelector<HTMLButtonElement>('.seletor-mes-hoje')!);
    const hoje = new Date();
    expect(t.hospedeiro.valor).toEqual({ ano: hoje.getFullYear(), mes: hoje.getMonth() + 1 });
  });

  it('Esc fecha o quadro e devolve o foco ao botão do mês', () => {
    const t = montar();
    t.clicar(t.botaoMes());
    t.painel()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    t.fixture.detectChanges();
    expect(t.painel()).toBeNull();
    expect(document.activeElement).toBe(t.botaoMes());
  });

  it('as setas ‹ › continuam andando um mês', () => {
    const t = montar();
    t.clicar(t.raiz.querySelector<HTMLButtonElement>('[aria-label="Próximo mês"]')!);
    expect(t.hospedeiro.valor).toEqual({ ano: 2026, mes: 11 });
  });
});
