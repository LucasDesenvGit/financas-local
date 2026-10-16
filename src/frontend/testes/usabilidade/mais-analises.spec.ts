import { Component, OnDestroy } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MaisAnalisesComponent } from '../../src/app/compartilhado/mais-analises.component';

/** Conta quantas vezes o conteúdo recolhido foi criado/destruído (fica no lugar de um gráfico pesado). */
const contagem = { criados: 0, destruidos: 0 };

@Component({ selector: 'app-grafico-pesado', template: '<canvas></canvas>' })
class GraficoPesado implements OnDestroy {
  constructor() { contagem.criados++; }
  ngOnDestroy(): void { contagem.destruidos++; }
}

@Component({
  imports: [MaisAnalisesComponent, GraficoPesado],
  template: `
    <app-mais-analises chave="painel" conteudo="receitas × despesas em 12 meses e últimas transações">
      <ng-template><app-grafico-pesado /></ng-template>
    </app-mais-analises>`
})
class Hospedeiro {}

function montar() {
  const fixture = TestBed.createComponent(Hospedeiro);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const botao = raiz.querySelector<HTMLButtonElement>('.mais-analises-botao')!;
  const alternar = () => { botao.click(); fixture.detectChanges(); };
  return { fixture, raiz, botao, alternar };
}

/**
 * Usabilidade da divulgação progressiva (UX-11, #43). NN/g Progressive Disclosure: o essencial primeiro, o resto atrás
 * de um controle secundário; Shneiderman: details on demand; Nielsen 6: o botão diz o que está recolhido;
 * WCAG 4.1.2: estado aberto/fechado exposto.
 */
describe('Usabilidade: "Mais análises"', () => {
  beforeEach(() => {
    contagem.criados = 0;
    contagem.destruidos = 0;
  });

  it('é um botão dentro de um título, com aria-expanded e aria-controls', () => {
    const { raiz, botao } = montar();
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.closest('h2')).not.toBeNull();       // navegação por títulos do leitor de tela encontra a seção
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(raiz.querySelector(`#${botao.getAttribute('aria-controls')}`)).not.toBeNull();
  });

  it('o botão diz o que está recolhido (reconhecer em vez de lembrar)', () => {
    const { botao } = montar();
    expect(botao.textContent).toContain('Mais análises');
    expect(botao.textContent).toContain('receitas × despesas em 12 meses e últimas transações');
  });

  it('começa fechado e o conteúdo recolhido nem é criado (gráfico fechado não é desenhado)', () => {
    const { raiz } = montar();
    expect(contagem.criados).toBe(0);
    expect(raiz.querySelector('canvas')).toBeNull();
  });

  it('abre e fecha com um clique; fechar destrói o conteúdo', () => {
    const { raiz, botao, alternar } = montar();
    alternar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(contagem.criados).toBe(1);
    expect(raiz.querySelector('canvas')).not.toBeNull();
    alternar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(contagem.destruidos).toBe(1);
  });

  it('lembra a escolha por tela: reabrir a tela mantém aberto', () => {
    montar().alternar();
    const { botao } = montar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(localStorage.getItem('financas.maisAnalises.painel')).toBe('1');
    expect(localStorage.getItem('financas.maisAnalises.anual')).toBeNull();
  });

  it('sem armazenamento no navegador (modo privado/bloqueado): continua funcionando, só não lembra', () => {
    jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('bloqueado'); });
    jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('bloqueado'); });
    const { botao, alternar } = montar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(() => alternar()).not.toThrow();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
  });
});
