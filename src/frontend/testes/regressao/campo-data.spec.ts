import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { readdirSync, readFileSync } from 'fs';
import { join } from 'path';
import { CampoDataDirective, DATA_MAXIMA, DATA_MINIMA } from '../../src/app/compartilhado/campo-data.directive';

/**
 * Regressão: o campo de data aceitava ano com 6 dígitos ("13/02/275760"), porque o <input type="date"> sem `max`
 * deixa o Chrome/Edge irem até o ano 275760. A diretiva põe limites (1900–2099, ou os da tela) — com um `max` de 4
 * dígitos o próprio navegador limita o ano a 4 — e data fora do intervalo deixa o formulário inválido.
 */
describe('Regressão: campo de data com ano de 4 dígitos', () => {
  @Component({
    imports: [FormsModule, CampoDataDirective],
    template: `<form #f="ngForm">
      <input type="date" name="livre" [(ngModel)]="livre">
      <input type="date" name="ate" [(ngModel)]="ate" [max]="hoje">
    </form>`
  })
  class Hospedeiro { livre = ''; ate = ''; hoje = '2026-10-18'; }

  async function montar() {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const [livre, ate] = Array.from(raiz.querySelectorAll('input'));
    const definir = async (campo: 'livre' | 'ate', valor: string) => {
      fixture.componentInstance[campo] = valor;
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { fixture, raiz, livre, ate, definir, form: () => raiz.querySelector('form')! };
  }

  it('sem limite na tela, o campo ganha 1900-01-01 a 2099-12-31 (o navegador limita o ano a 4 dígitos)', async () => {
    const t = await montar();
    expect(t.livre.getAttribute('min')).toBe(DATA_MINIMA);
    expect(t.livre.getAttribute('max')).toBe(DATA_MAXIMA);
  });

  it('o limite da tela vale no lugar do padrão (ex.: [max]="hoje")', async () => {
    const t = await montar();
    expect(t.ate.getAttribute('max')).toBe('2026-10-18');
    expect(t.ate.getAttribute('min')).toBe(DATA_MINIMA);
  });

  it('ano com 6 dígitos ou data fora do intervalo deixam o formulário inválido; data normal, válido', async () => {
    const t = await montar();
    await t.definir('livre', '275760-02-13');
    expect(t.form().classList).toContain('ng-invalid');
    await t.definir('livre', '1899-12-31');
    expect(t.form().classList).toContain('ng-invalid');
    await t.definir('livre', '2026-10-18');
    expect(t.form().classList).toContain('ng-valid');
    await t.definir('ate', '2026-10-19');           // depois de "hoje"
    expect(t.form().classList).toContain('ng-invalid');
  });

  it('toda tela com <input type="date"> importa a diretiva (senão o campo fica sem limite)', () => {
    const pasta = join(__dirname, '../../src/app/paginas');
    const semDiretiva = readdirSync(pasta).filter((f) => f.endsWith('.ts')).filter((f) => {
      const fonte = readFileSync(join(pasta, f), 'utf8');
      return fonte.includes('type="date"') && !fonte.includes('CampoDataDirective');
    });
    expect(semDiretiva).toEqual([]);
  });
});
