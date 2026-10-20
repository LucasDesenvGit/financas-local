import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../../src/app/compartilhado/campo-valor.component';

/**
 * Usabilidade do campo de valor (#132, ADR 0032).
 * - Nielsen 2 (Match Between the System and the Real World): "1.234,56", com "R$" no campo.
 * - Nielsen 5 (Error Prevention): campo de texto, sem setinhas e sem a rodinha do mouse mudando o valor.
 * - WCAG 1.1.1 / 3.3.2: o "R$" visual é decorativo; o rótulo continua visível e o nome acessível diz a moeda.
 * - WCAG 3.3.1 (Error Identification): o erro aparece em texto, perto do campo, ligado por aria-describedby.
 */
describe('Usabilidade: campo de valor', () => {
  @Component({
    imports: [FormsModule, CampoValorComponent],
    template: `<label>Valor <app-campo-valor name="v" [(ngModel)]="v" obrigatorio [minimo]="0.01" /></label>
      <label>Cotação <app-campo-valor name="c" [(ngModel)]="c" moeda="USD" [casas]="4" /></label>
      <app-campo-valor name="o" [(ngModel)]="o" compacto enterConfirma rotulo="Limite de Mercado" (confirmado)="confirmados.push($event)" />`
  })
  class Hospedeiro { v: number | null = null; c: number | null = 12.3456; o: number | null = 100; confirmados: (number | null)[] = []; }

  async function montar() {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const [valor, cotacao, orcamento] = Array.from(raiz.querySelectorAll<HTMLInputElement>('app-campo-valor input'));
    const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
    return { fixture, raiz, valor, cotacao, orcamento, atualizar, hospedeiro: fixture.componentInstance };
  }

  it('é texto com inputmode="decimal" (teclado numérico no celular, sem setinhas nem rodinha)', async () => {
    const t = await montar();
    expect(t.valor.type).toBe('text');
    expect(t.valor.getAttribute('inputmode')).toBe('decimal');
  });

  it('o símbolo da moeda é decorativo e o rótulo diz a moeda para o leitor de tela', async () => {
    const t = await montar();
    const [real, dolar] = Array.from(t.raiz.querySelectorAll('app-campo-valor'));
    expect(real.querySelector('.campo-valor-simbolo')!.textContent).toBe('R$');
    expect(real.querySelector('.campo-valor-simbolo')!.getAttribute('aria-hidden')).toBe('true');
    expect(t.valor.closest('label')!.textContent).toMatch(/Valor\s*R\$\s*\(em reais\)/);
    expect(dolar.querySelector('.campo-valor-simbolo')!.textContent).toBe('US$');
    expect(dolar.textContent).toContain('(em dólares)');
    expect(t.cotacao.value).toBe('12,3456');
  });

  it('o erro só aparece depois de sair do campo, em texto, ligado por aria-describedby e com aria-invalid', async () => {
    const t = await montar();
    expect(t.raiz.querySelector('.campo-valor-mensagem')).toBeNull();
    expect(t.valor.getAttribute('aria-required')).toBe('true');
    t.valor.dispatchEvent(new Event('blur'));
    await t.atualizar();
    const mensagem = t.raiz.querySelector('.campo-valor-mensagem')!;
    expect(mensagem.textContent).toBe('Informe o valor.');
    expect(t.valor.getAttribute('aria-invalid')).toBe('true');
    expect(t.valor.getAttribute('aria-describedby')).toBe(mensagem.id);
    // Fora do nome do <label> (o nome continua "Valor"), mas lido pelo aria-describedby.
    expect(mensagem.getAttribute('aria-hidden')).toBe('true');
  });

  it('campo fora de <label> (célula de tabela) tem nome acessível pelo rotulo', async () => {
    const t = await montar();
    expect(t.orcamento.getAttribute('aria-label')).toBe('Limite de Mercado');
  });

  it('Enter confirma o valor da linha (orçamento) sem enviar formulário, e sair sem mudar não confirma', async () => {
    const t = await montar();
    t.orcamento.dispatchEvent(new Event('focus'));
    t.orcamento.dispatchEvent(new Event('blur'));
    await t.atualizar();
    expect(t.hospedeiro.confirmados).toEqual([]);
    t.orcamento.dispatchEvent(new Event('focus'));
    t.orcamento.value = '250';
    t.orcamento.dispatchEvent(new Event('input'));
    const enter = new KeyboardEvent('keydown', { key: 'Enter', cancelable: true });
    t.orcamento.dispatchEvent(enter);
    await t.atualizar();
    expect(enter.defaultPrevented).toBe(true);
    expect(t.hospedeiro.confirmados).toEqual([250]);
    expect(t.orcamento.value).toBe('250,00');
  });
});
