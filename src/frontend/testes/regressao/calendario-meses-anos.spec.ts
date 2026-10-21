import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { CampoDataComponent, inicioDaPaginaDeAnos } from '../../src/app/compartilhado/campo-data.component';

/**
 * #264: no calendário do campo de data, o título leva à grade de meses e daí à de anos, para ir longe sem dezenas de
 * cliques nas setas (Nielsen 7). Escolher desce um nível; Esc sobe um antes de fechar; meses e anos fora dos limites
 * ficam desabilitados; teclado nas grades (WCAG 2.1.1).
 */
describe('Regressão: calendário com meses e anos pelo título (#264)', () => {
  @Component({
    imports: [FormsModule, CampoDataComponent],
    template: `<label>Data <app-campo-data [(ngModel)]="data" [minimo]="minimo" [maximo]="maximo" /></label>`
  })
  class Hospedeiro { data = '2026-10-18'; minimo: string | null = null; maximo: string | null = null; }

  async function abrir(campos: Partial<Hospedeiro> = {}) {
    const fixture = TestBed.createComponent(Hospedeiro);
    Object.assign(fixture.componentInstance, campos);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
    raiz.querySelector<HTMLButtonElement>('.campo-data-botao')!.click();
    await atualizar();
    const titulo = () => raiz.querySelector('.calendario-topo strong')!.textContent!.trim();
    const botaoTitulo = () => raiz.querySelector<HTMLButtonElement>('.calendario-titulo-botao');
    const celula = (valor: number) => raiz.querySelector<HTMLButtonElement>(`.calendario-celula[data-valor="${valor}"]`)!;
    const celulas = () => Array.from(raiz.querySelectorAll<HTMLButtonElement>('.calendario-celula'));
    const tecla = async (key: string) => {
      (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
      await atualizar();
    };
    const clicar = async (el: HTMLElement) => { el.click(); await atualizar(); };
    return { fixture, raiz, atualizar, titulo, botaoTitulo, celula, celulas, tecla, clicar, h: fixture.componentInstance };
  }

  it('páginas de 12 anos a partir de 1900', () => {
    expect(inicioDaPaginaDeAnos(2026)).toBe(2020);
    expect(inicioDaPaginaDeAnos(2031)).toBe(2020);
    expect(inicioDaPaginaDeAnos(2032)).toBe(2032);
    expect(inicioDaPaginaDeAnos(2099)).toBe(2092);
  });

  it('título → meses do ano (o escolhido destacado) → anos; o título diz o nível', async () => {
    const t = await abrir();
    expect(t.titulo()).toBe('outubro de 2026');
    expect(t.botaoTitulo()!.getAttribute('aria-describedby')).toBeTruthy();

    await t.clicar(t.botaoTitulo()!);
    expect(t.titulo()).toBe('2026');
    expect(t.celulas().map((c) => c.textContent!.trim())).toEqual(
      ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']);
    expect(t.celula(10).classList).toContain('escolhido');
    expect(t.celula(10).getAttribute('aria-label')).toBe('outubro de 2026');
    expect(document.activeElement).toBe(t.celula(10));

    await t.clicar(t.botaoTitulo()!);
    expect(t.titulo()).toBe('2020–2031');
    expect(t.botaoTitulo()).toBeNull();           // nos anos, o título não sobe mais
    expect(t.celulas()).toHaveLength(12);
    expect(t.celula(2026).classList).toContain('escolhido');
    expect(document.activeElement).toBe(t.celula(2026));
  });

  it('escolher um ano leva aos meses dele; escolher um mês leva aos dias, com o foco no mesmo dia', async () => {
    const t = await abrir();
    await t.clicar(t.botaoTitulo()!);
    await t.clicar(t.botaoTitulo()!);
    await t.clicar(t.raiz.querySelector<HTMLButtonElement>('[aria-label="Anos anteriores"]')!);
    expect(t.titulo()).toBe('2008–2019');
    await t.clicar(t.celula(2015));
    expect(t.titulo()).toBe('2015');
    await t.clicar(t.celula(3));
    expect(t.titulo()).toBe('março de 2015');
    expect(document.activeElement).toBe(t.raiz.querySelector('.calendario-dia[data-iso="2015-03-18"]'));
    // A data só muda ao escolher o dia.
    expect(t.h.data).toBe('2026-10-18');
  });

  it('meses e anos fora dos limites ficam desabilitados', async () => {
    const t = await abrir({ data: '2026-10-18', minimo: '2026-03-10', maximo: '2028-02-01' });
    await t.clicar(t.botaoTitulo()!);
    expect(t.celula(2).disabled).toBe(true);      // fevereiro de 2026, antes do mínimo
    expect(t.celula(3).disabled).toBe(false);     // março tem dias a partir do dia 10
    await t.clicar(t.botaoTitulo()!);
    expect(t.celula(2025).disabled).toBe(true);
    expect(t.celula(2026).disabled).toBe(false);
    expect(t.celula(2028).disabled).toBe(false);
    expect(t.celula(2029).disabled).toBe(true);
  });

  it('teclado nos meses: → um, ↓ três, passar de dezembro vai para o ano seguinte; Esc sobe um nível antes de fechar', async () => {
    const t = await abrir();
    await t.clicar(t.botaoTitulo()!);
    await t.tecla('ArrowRight');
    expect(document.activeElement).toBe(t.celula(11));
    await t.tecla('ArrowDown');
    expect(t.titulo()).toBe('2027');
    expect(document.activeElement).toBe(t.celula(2));
    await t.tecla('Home');
    expect(document.activeElement).toBe(t.celula(1));

    await t.clicar(t.botaoTitulo()!);             // anos
    await t.tecla('Escape');
    expect(t.titulo()).toBe('2027');              // anos → meses
    await t.tecla('Escape');
    expect(t.titulo()).toMatch(/de 2027$/);       // meses → dias
    expect(t.raiz.querySelector('.calendario')).not.toBeNull();
    await t.tecla('Escape');
    expect(t.raiz.querySelector('.calendario')).toBeNull();   // dias → fecha
  });

  it('abrir de novo volta para os dias', async () => {
    const t = await abrir();
    await t.clicar(t.botaoTitulo()!);
    t.raiz.querySelector<HTMLButtonElement>('.campo-data-botao')!.click();   // fecha
    await t.atualizar();
    t.raiz.querySelector<HTMLButtonElement>('.campo-data-botao')!.click();   // abre
    await t.atualizar();
    expect(t.titulo()).toBe('outubro de 2026');
  });
});
