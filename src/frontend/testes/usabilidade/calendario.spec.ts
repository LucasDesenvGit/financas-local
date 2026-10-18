import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { CampoDataComponent } from '../../src/app/compartilhado/campo-data.component';

/**
 * Usabilidade do calendário do campo de data (padrão Date Picker Dialog do WAI-ARIA APG).
 * - WCAG 2.1.1: abre pelo botão (ou Alt+↓ no campo); setas andam um dia/uma semana, PageUp/PageDown um mês, Shift
 *   um ano, Home/End o começo/fim da semana; Enter escolhe; Esc fecha e devolve o foco ao botão.
 * - WCAG 4.1.2: dialog com título do mês, grid, dia escolhido (aria-selected) e hoje (aria-current="date"), e cada
 *   dia com o nome completo ("domingo, 18 de outubro de 2026").
 * - Nielsen 3/7: "Hoje" e "Limpar"; « » pulam um ano.
 */
describe('Usabilidade: calendário do campo de data', () => {
  @Component({
    imports: [FormsModule, CampoDataComponent],
    template: `<label>Data <app-campo-data [(ngModel)]="data" (alterado)="alterados.push($event)" /></label>`
  })
  class Hospedeiro { data = '2026-10-18'; alterados: string[] = []; }

  async function montar() {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
    const botao = () => raiz.querySelector<HTMLButtonElement>('.campo-data-botao')!;
    const calendario = () => raiz.querySelector<HTMLElement>('.calendario');
    const dia = (iso: string) => raiz.querySelector<HTMLButtonElement>(`.calendario-dia[data-iso="${iso}"]`)!;
    const tecla = async (key: string, extra: KeyboardEventInit = {}) => {
      (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...extra }));
      await atualizar();
    };
    return { fixture, raiz, atualizar, botao, calendario, dia, tecla, h: fixture.componentInstance };
  }

  async function abrir() {
    const t = await montar();
    t.botao().click();
    await t.atualizar();
    return t;
  }

  it('o botão abre um calendário (dialog) no mês da data, com o dia escolhido destacado e o foco nele', async () => {
    const t = await abrir();
    expect(t.botao().getAttribute('aria-expanded')).toBe('true');
    expect(t.calendario()!.getAttribute('role')).toBe('dialog');
    expect(t.calendario()!.querySelector('strong')!.textContent).toBe('outubro de 2026');
    expect(t.dia('2026-10-18').getAttribute('aria-label')).toBe('domingo, 18 de outubro de 2026');
    expect(t.dia('2026-10-18').closest('td')!.getAttribute('aria-selected')).toBe('true');
    expect(document.activeElement).toBe(t.dia('2026-10-18'));
    expect(t.dia('2026-10-18').getAttribute('tabindex')).toBe('0');
    expect(t.dia('2026-10-19').getAttribute('tabindex')).toBe('-1');
  });

  it('clicar num dia escolhe, fecha e confirma (alterado)', async () => {
    const t = await abrir();
    t.dia('2026-10-25').click();
    await t.atualizar();
    expect(t.h.data).toBe('2026-10-25');
    expect(t.h.alterados).toEqual(['2026-10-25']);
    expect(t.calendario()).toBeNull();
    expect(document.activeElement).toBe(t.botao());
  });

  it('teclado: → um dia, ↓ uma semana, PageDown um mês, Shift+PageUp um ano; Enter escolhe', async () => {
    const t = await abrir();
    await t.tecla('ArrowRight');
    expect(document.activeElement).toBe(t.dia('2026-10-19'));
    await t.tecla('ArrowDown');
    expect(document.activeElement).toBe(t.dia('2026-10-26'));
    await t.tecla('PageDown');
    expect(t.calendario()!.querySelector('strong')!.textContent).toBe('novembro de 2026');
    expect(document.activeElement).toBe(t.dia('2026-11-26'));
    await t.tecla('PageUp', { shiftKey: true });
    expect(t.calendario()!.querySelector('strong')!.textContent).toBe('novembro de 2025');
    (document.activeElement as HTMLButtonElement).click();
    await t.atualizar();
    expect(t.h.data).toBe('2025-11-26');
  });

  it('Esc fecha sem mudar a data e devolve o foco ao botão', async () => {
    const t = await abrir();
    await t.tecla('ArrowRight');
    await t.tecla('Escape');
    expect(t.calendario()).toBeNull();
    expect(t.h.data).toBe('2026-10-18');
    expect(document.activeElement).toBe(t.botao());
  });

  it('« e » pulam um ano; "Limpar" esvazia', async () => {
    const t = await abrir();
    t.calendario()!.querySelector<HTMLButtonElement>('[aria-label="Próximo ano"]')!.click();
    await t.atualizar();
    expect(t.calendario()!.querySelector('strong')!.textContent).toBe('outubro de 2027');
    Array.from(t.calendario()!.querySelectorAll<HTMLButtonElement>('.calendario-rodape button')).find((b) => b.textContent!.trim() === 'Limpar')!.click();
    await t.atualizar();
    expect(t.h.data).toBe('');
    expect(t.h.alterados).toEqual(['']);
  });

  it('o campo de texto tem a máscara dd/mm/aaaa e teclado numérico no celular', async () => {
    const t = await montar();
    const campo = t.raiz.querySelector<HTMLInputElement>('app-campo-data input')!;
    expect(campo.getAttribute('placeholder')).toBe('dd/mm/aaaa');
    expect(campo.getAttribute('inputmode')).toBe('numeric');
  });
});
