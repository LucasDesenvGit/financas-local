import { TestBed } from '@angular/core/testing';
import { MetasComponent } from '../../src/app/paginas/metas.component';
import { abrirMetas, meta } from '../apoio/metas';

/**
 * #335: cada meta é uma sanfona, como os grupos de "Assinaturas e recorrentes" (ADR 0046): o título é o botão
 * (aria-expanded/aria-controls), fechada mostra o percentual e os valores, todas abertas no começo, as fechadas lembradas
 * neste navegador (ADR 0017), "Fechar todas"/"Abrir todas" com mais de uma; e a meta sem data diz "sem data definida".
 */
const VIAGEM = meta({ id: 1, nome: 'Viagem', valorAlvo: 8000, valorAtual: 2400, percentual: 30, dataAlvo: '2027-07-01', contaId: null, reservaNome: null, saldoReserva: null });
const CARRO = meta({ id: 2, nome: 'Carro', valorAlvo: 30000, valorAtual: 0, percentual: 0, dataAlvo: null, contaId: null, reservaNome: null, saldoReserva: null });

describe('Regressão: Metas em sanfona (#335)', () => {
  beforeEach(() => localStorage.clear());

  const botao = (raiz: HTMLElement, id: number) => raiz.querySelector<HTMLButtonElement>(`#meta-${id}`)!;
  const conteudo = (raiz: HTMLElement, id: number) => raiz.querySelector<HTMLElement>(`#meta-${id}-conteudo`)!;

  it('todas abertas no começo; o título é o botão, e fechada continua mostrando o percentual e os valores', async () => {
    const { raiz, atualizar } = abrirMetas([VIAGEM, CARRO]);
    expect(botao(raiz, 1).closest('h2')).not.toBeNull();
    expect(botao(raiz, 1).getAttribute('aria-expanded')).toBe('true');
    expect(botao(raiz, 1).getAttribute('aria-controls')).toBe('meta-1-conteudo');
    expect(botao(raiz, 1).textContent!.replace(/\s+/g, ' ')).toContain('30% · R$ 2.400,00 de R$ 8.000,00');
    botao(raiz, 1).click();
    await atualizar();
    expect(botao(raiz, 1).getAttribute('aria-expanded')).toBe('false');
    // Fechada: fora do Tab e do leitor de tela (inert), sem sair do DOM.
    expect(conteudo(raiz, 1).hasAttribute('inert')).toBe(true);
    expect(conteudo(raiz, 2).hasAttribute('inert')).toBe(false);
    expect(JSON.parse(localStorage.getItem(MetasComponent.CHAVE_FECHADAS)!)).toEqual([1]);
  });

  it('a meta fechada continua fechada ao abrir a tela de novo', async () => {
    localStorage.setItem(MetasComponent.CHAVE_FECHADAS, '[2]');
    const { raiz } = abrirMetas([VIAGEM, CARRO]);
    expect(botao(raiz, 1).getAttribute('aria-expanded')).toBe('true');
    expect(botao(raiz, 2).getAttribute('aria-expanded')).toBe('false');
  });

  it('"Fechar todas" fecha todas; depois, "Abrir todas" abre de novo', async () => {
    const { raiz, atualizar } = abrirMetas([VIAGEM, CARRO]);
    const acao = () => raiz.querySelector<HTMLButtonElement>('.acoes-metas button')!;
    expect(acao().textContent!.trim()).toBe('Fechar todas');
    acao().click();
    await atualizar();
    expect([botao(raiz, 1), botao(raiz, 2)].map((b) => b.getAttribute('aria-expanded'))).toEqual(['false', 'false']);
    expect(acao().textContent!.trim()).toBe('Abrir todas');
    acao().click();
    await atualizar();
    expect([botao(raiz, 1), botao(raiz, 2)].map((b) => b.getAttribute('aria-expanded'))).toEqual(['true', 'true']);
  });

  it('com uma meta só, não há "Fechar todas"; sem data alvo, diz "sem data definida"', () => {
    const { raiz } = abrirMetas([CARRO]);
    expect(raiz.querySelector('.acoes-metas')).toBeNull();
    expect(conteudo(raiz, 2).textContent).toContain('sem data definida');
    TestBed.resetTestingModule();
    const outra = abrirMetas([VIAGEM]);
    expect(conteudo(outra.raiz, 1).textContent).not.toContain('sem data definida');
  });
});
