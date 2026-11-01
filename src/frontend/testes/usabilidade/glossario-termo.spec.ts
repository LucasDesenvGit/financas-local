import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { GLOSSARIO } from '../../src/app/compartilhado/glossario';
import { TermoComponent } from '../../src/app/compartilhado/termo.component';

@Component({
  imports: [TermoComponent],
  template: `
    <div class="cabecalho" (click)="cliquesNoCabecalho = cliquesNoCabecalho + 1">
      <app-termo chave="precoMedio">Preço médio</app-termo>
    </div>
    <p class="fora">fora</p>`
})
class Hospedeiro {
  cliquesNoCabecalho = 0;
}

function montar() {
  const fixture = TestBed.createComponent(Hospedeiro);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const botao = raiz.querySelector<HTMLButtonElement>('.termo-botao')!;
  const balao = () => raiz.querySelector<HTMLElement>('.termo-balao');
  const clicar = (el: HTMLElement) => { el.click(); fixture.detectChanges(); };
  return { fixture, raiz, botao, balao, clicar };
}

/**
 * Usabilidade do glossário (UX-10, #42). Padrão de docs/UI-UX.md: termo técnico com definição no contexto, que abre por
 * clique/teclado e vem de um glossário único — nunca só em title=. Referências: Nielsen 2 (linguagem de quem usa) e
 * 10 (ajuda no contexto); WCAG 1.4.13 (conteúdo em foco: dispensável e persistente) e 4.1.2 (nome, papel, estado).
 */
describe('Usabilidade: termo do glossário (ⓘ)', () => {
  it('o ⓘ é um botão de verdade, com nome acessível e estado (WCAG 4.1.2 e 2.1.1)', () => {
    const { botao } = montar();
    expect(botao.tagName).toBe('BUTTON');            // Enter/Espaço funcionam sem código extra
    expect(botao.type).toBe('button');               // não envia formulário
    expect(botao.getAttribute('aria-label')).toBe('O que é Preço médio?');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
  });

  it('não depende de title= (não aparece em toque nem no teclado)', () => {
    const { raiz } = montar();
    expect(raiz.querySelector('[title]')).toBeNull();
  });

  it('abre por clique com a definição e o exemplo, ligados ao botão por aria-controls/aria-describedby', () => {
    const { botao, balao, clicar } = montar();
    clicar(botao);
    const b = balao()!;
    expect(b).not.toBeNull();
    expect(b.getAttribute('role')).toBe('note');
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(botao.getAttribute('aria-controls')).toBe(b.id);
    expect(botao.getAttribute('aria-describedby')).toBe(b.id);
    expect(b.textContent).toContain(GLOSSARIO.precoMedio.definicao);
    expect(b.textContent).toContain(GLOSSARIO.precoMedio.exemplo);
  });

  it('Esc fecha sem tirar o foco do ⓘ (WCAG 1.4.13: dispensável)', () => {
    const { fixture, botao, balao, clicar } = montar();
    botao.focus();
    clicar(botao);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    fixture.detectChanges();
    expect(balao()).toBeNull();
    expect(document.activeElement).toBe(botao);
    expect(botao.getAttribute('aria-expanded')).toBe('false');
  });

  it('clique fora fecha; clique dentro do balão não fecha (persistente enquanto é lido)', () => {
    const { raiz, botao, balao, clicar } = montar();
    clicar(botao);
    clicar(balao()!);
    expect(balao()).not.toBeNull();
    clicar(raiz.querySelector<HTMLElement>('.fora')!);
    expect(balao()).toBeNull();
  });

  it('clicar no ⓘ dentro de um cabeçalho não aciona o clique do cabeçalho (ex.: ordenar tabela)', () => {
    const { fixture, botao, clicar } = montar();
    clicar(botao);
    clicar(botao);
    expect(fixture.componentInstance.cliquesNoCabecalho).toBe(0);
  });

  describe('conteúdo do glossário (Nielsen 2: linguagem simples)', () => {
    const entradas = Object.entries(GLOSSARIO);

    it.each(entradas)('%s: definição curta, de no máximo 2 frases', (_chave, d) => {
      const frases = d.definicao.split(/(?<=[.!?])\s+/).filter(Boolean);
      expect(frases.length).toBeLessThanOrEqual(2);
      expect(d.definicao.length).toBeLessThanOrEqual(240);
      expect(d.definicao).toMatch(/[.!?]$/);
    });

    it('cada termo aparece uma vez só (o mesmo termo tem a mesma definição em qualquer tela)', () => {
      const nomes = entradas.map(([, d]) => d.termo.toLowerCase());
      expect(new Set(nomes).size).toBe(nomes.length);
    });
  });
});
