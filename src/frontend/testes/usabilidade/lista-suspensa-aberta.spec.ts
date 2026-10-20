import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { readdirSync, readFileSync } from 'fs';
import { join } from 'path';
import { ListaSuspensaDirective } from '../../src/app/compartilhado/lista-suspensa.directive';

/**
 * Usabilidade da lista suspensa aberta no visual do site (ListaSuspensaDirective). O <select> nativo continua dono do
 * valor (ngModel/[ngValue]/(change)); a lista do site só substitui a do sistema no computador.
 * - Nielsen 4 (Consistency and Standards): a lista aberta igual ao resto do site, e o teclado das listas do sistema
 *   (Espaço/Enter/Alt+↓ abrem; ↑/↓/Home/End; Enter escolhe; Esc fecha).
 * - WCAG 4.1.2: listbox com option/aria-selected; o select diz aria-expanded e a opção ativa (aria-activedescendant).
 */
describe('Usabilidade: lista suspensa aberta', () => {
  @Component({
    imports: [FormsModule, ListaSuspensaDirective],
    template: `<label>Tipo
      <select [(ngModel)]="tipo" (change)="mudancas = mudancas + 1">
        <option [ngValue]="1">Conta corrente</option>
        <option [ngValue]="2">Poupança</option>
        <option [ngValue]="3" disabled>Investimento</option>
        <option [ngValue]="4">Dinheiro / carteira</option>
      </select></label>`
  })
  class Hospedeiro { tipo = 1; mudancas = 0; }

  async function montar() {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const select = (fixture.nativeElement as HTMLElement).querySelector('select')!;
    const painel = () => document.querySelector<HTMLElement>('.lista-suspensa-painel');
    const opcoes = () => Array.from(painel()?.querySelectorAll<HTMLElement>('[role="option"]') ?? []);
    const tecla = async (key: string, extra: KeyboardEventInit = {}) => {
      select.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...extra }));
      fixture.detectChanges();
      await fixture.whenStable();
    };
    return { fixture, select, painel, opcoes, tecla, hospedeiro: fixture.componentInstance };
  }

  afterEach(() => document.querySelectorAll('.lista-suspensa-painel').forEach((p) => p.remove()));

  it('clicar no campo abre a lista do site (e não a do sistema), com a opção escolhida destacada', async () => {
    const t = await montar();
    const clique = new MouseEvent('mousedown', { button: 0, bubbles: true, cancelable: true });
    t.select.dispatchEvent(clique);
    expect(clique.defaultPrevented).toBe(true);
    expect(t.painel()!.getAttribute('role')).toBe('listbox');
    expect(t.painel()!.getAttribute('aria-label')).toBe('Tipo');
    expect(t.opcoes().map((o) => o.textContent)).toEqual(['Conta corrente', 'Poupança', 'Investimento', 'Dinheiro / carteira']);
    expect(t.opcoes()[0].getAttribute('aria-selected')).toBe('true');
    expect(t.opcoes()[2].getAttribute('aria-disabled')).toBe('true');
    expect(t.select.getAttribute('aria-expanded')).toBe('true');
    expect(t.select.getAttribute('aria-controls')).toBe(t.painel()!.id);
  });

  it('clicar numa opção troca o valor do ngModel ([ngValue]) e dispara o (change) da tela', async () => {
    const t = await montar();
    t.select.dispatchEvent(new MouseEvent('mousedown', { button: 0, bubbles: true, cancelable: true }));
    t.opcoes()[3].click();
    t.fixture.detectChanges();
    await t.fixture.whenStable();
    expect(t.hospedeiro.tipo).toBe(4);
    expect(t.hospedeiro.mudancas).toBe(1);
    expect(t.painel()).toBeNull();
    expect(t.select.hasAttribute('aria-expanded')).toBe(false);
  });

  it('teclado: Espaço abre; ↓ pula a opção desabilitada; Enter escolhe', async () => {
    const t = await montar();
    await t.tecla(' ');
    expect(t.painel()).not.toBeNull();
    await t.tecla('ArrowDown');                       // Poupança
    await t.tecla('ArrowDown');                       // pula "Investimento" (desabilitada)
    expect(t.select.getAttribute('aria-activedescendant')).toBe(t.opcoes()[3].id);
    await t.tecla('Enter');
    expect(t.hospedeiro.tipo).toBe(4);
    expect(t.painel()).toBeNull();
  });

  it('Esc fecha sem trocar o valor e não deixa o Esc chegar ao modal em volta', async () => {
    const t = await montar();
    await t.tecla('Enter');
    await t.tecla('ArrowDown');
    const fora = jest.fn();
    document.addEventListener('keydown', fora);
    await t.tecla('Escape');
    document.removeEventListener('keydown', fora);
    expect(t.painel()).toBeNull();
    expect(t.hospedeiro.tipo).toBe(1);
    expect(fora).not.toHaveBeenCalled();
  });

  it('letra leva à próxima opção que começa com ela', async () => {
    const t = await montar();
    await t.tecla('Enter');
    await t.tecla('d');
    expect(t.select.getAttribute('aria-activedescendant')).toBe(t.opcoes()[3].id);
  });

  it('com a lista fechada, ↑/↓ continuam trocando o valor como no select do sistema (a diretiva não intercepta)', async () => {
    const t = await montar();
    const seta = new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true });
    t.select.dispatchEvent(seta);
    expect(seta.defaultPrevented).toBe(false);
    expect(t.painel()).toBeNull();
  });

  it('no toque (celular), fica a lista do sistema', async () => {
    const original = window.matchMedia;
    window.matchMedia = ((q: string) => ({ matches: q.includes('coarse') })) as unknown as typeof window.matchMedia;
    const t = await montar();
    const clique = new MouseEvent('mousedown', { button: 0, bubbles: true, cancelable: true });
    t.select.dispatchEvent(clique);
    window.matchMedia = original;
    expect(clique.defaultPrevented).toBe(false);
    expect(t.painel()).toBeNull();
  });

  it('toda tela com <select> importa a diretiva (senão a lista aberta volta a ser a do sistema)', () => {
    const app = join(__dirname, '../../src/app');
    const pastas = ['paginas', 'compartilhado'].map((p) => join(app, p));
    const sem = pastas.flatMap((p) => readdirSync(p).filter((f) => f.endsWith('.ts')).map((f) => join(p, f)))
      .filter((f) => !f.endsWith('lista-suspensa.directive.ts'))
      .filter((f) => {
        const fonte = readFileSync(f, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
        return fonte.includes('<select') && !fonte.includes('ListaSuspensaDirective');
      });
    expect(sem).toEqual([]);
  });
});
