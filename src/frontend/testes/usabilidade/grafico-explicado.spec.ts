import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { GraficoExplicadoComponent } from '../../src/app/compartilhado/grafico-explicado.component';

@Component({
  imports: [GraficoExplicadoComponent],
  template: `
    <app-grafico-explicado [resumo]="resumo()" comoLer="Verde: receitas; rosa: despesas.">
      <canvas></canvas>
    </app-grafico-explicado>`
})
class Hospedeiro {
  readonly resumo = signal<string | null>('Você gastou 12% menos que no mês passado.');
}

/**
 * Usabilidade dos gráficos (UX-9, #41). Padrão do CLAUDE.md: "gráfico se explica" — conclusão + como ler.
 * WCAG 1.1.1: o <canvas> tem alternativa em texto, montada com a frase-resumo e a linha "como ler".
 */
describe('Usabilidade: gráfico explicado', () => {
  function montar() {
    const fixture = TestBed.createComponent(Hospedeiro);
    fixture.detectChanges();
    return { fixture, raiz: fixture.nativeElement as HTMLElement };
  }

  it('a conclusão aparece antes do gráfico e o "como ler" fica junto dele', () => {
    const { raiz } = montar();
    const figura = raiz.querySelector('figure')!;
    const filhos = Array.from(figura.children).map((e) => e.className || e.tagName.toLowerCase());
    expect(filhos).toEqual(['grafico-resumo', 'grafico-area', 'grafico-como-ler']);
    expect(raiz.querySelector('figcaption')!.textContent).toContain('Verde: receitas');
  });

  it('o gráfico tem alternativa em texto: role="img" com resumo + como ler (WCAG 1.1.1)', () => {
    const { raiz } = montar();
    const area = raiz.querySelector('.grafico-area')!;
    expect(area.getAttribute('role')).toBe('img');
    expect(area.getAttribute('aria-label')).toBe('Você gastou 12% menos que no mês passado. Verde: receitas; rosa: despesas.');
  });

  it('o leitor de tela não lê o resumo duas vezes (texto visível fica aria-hidden)', () => {
    const { raiz } = montar();
    expect(raiz.querySelector('.grafico-resumo')!.getAttribute('aria-hidden')).toBe('true');
  });

  it('sem dados, avisa isso em vez de deixar o gráfico mudo', () => {
    const { fixture, raiz } = montar();
    fixture.componentInstance.resumo.set(null);
    fixture.detectChanges();
    expect(raiz.querySelector('.grafico-resumo')).toBeNull();
    expect(raiz.querySelector('.grafico-area')!.getAttribute('aria-label')).toBe('Gráfico sem dados. Verde: receitas; rosa: despesas.');
  });
});
