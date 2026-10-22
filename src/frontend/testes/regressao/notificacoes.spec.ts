import { Orcamento } from '../../src/app/core/modelos';
import { abrirNotificacoes } from '../apoio/notificacoes';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #327: os avisos e os orçamentos estourados saíram do topo do Painel e foram para o sino da barra do topo, em qualquer
 * tela. O contador soma o que não está oculto; o card abre e fecha pelo sino (Disclosure), pelo Esc e pelo clique fora.
 */
function orc(categoriaId: number, percentual: number): Orcamento {
  return { id: categoriaId, categoriaId, categoriaNome: `Categoria ${categoriaId}`, categoriaCor: '#999', ano: 2026, mes: 10,
    valor: 100, gasto: percentual, percentual, gastoEmergencia: 0, situacao: 'estourado' };
}
const ALERTA = 'Conta "Aluguel" vencida desde 05/10.';

describe('Regressão: notificações na barra do topo (#327)', () => {
  beforeEach(() => localStorage.clear());

  it('o sino diz quantas notificações estão pendentes, no nome e no contador', async () => {
    const { sino } = await abrirNotificacoes({ orcamentosEstourados: [orc(1, 130), orc(2, 110)], alertas: [ALERTA] }, false);
    expect(sino().getAttribute('aria-label')).toBe('Notificações, 3 pendentes');
    expect(sino().querySelector('.contador')!.textContent!.trim()).toBe('3');
    expect(sino().getAttribute('aria-expanded')).toBe('false');
    expect(sino().getAttribute('aria-controls')).toBe('card-notificacoes');
  });

  it('sem nada pendente: sem contador, e o card diz que nada pede atenção', async () => {
    const { raiz, sino } = await abrirNotificacoes({});
    expect(sino().getAttribute('aria-label')).toBe('Notificações, nada pendente');
    expect(sino().querySelector('.contador')).toBeNull();
    expect(raiz.querySelector('#card-notificacoes')!.textContent).toContain('Nada pede atenção agora.');
  });

  it('o sino abre o card com os avisos; Esc fecha e devolve o foco ao sino; o clique fora também fecha', async () => {
    const { raiz, sino, atualizar } = await abrirNotificacoes({ alertas: [ALERTA] }, false);
    sino().click();
    await atualizar();
    expect(sino().getAttribute('aria-expanded')).toBe('true');
    const card = raiz.querySelector('#card-notificacoes .card-notificacoes')!;
    expect(card.textContent).toContain('1 aviso pede atenção');
    card.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await atualizar();
    expect(raiz.querySelector('.card-notificacoes')).toBeNull();
    expect(document.activeElement).toBe(sino());

    sino().click();
    await atualizar();
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await atualizar();
    expect(raiz.querySelector('.card-notificacoes')).toBeNull();
  });

  it('ocultar um card tira as notificações dele do contador', async () => {
    const { raiz, sino, atualizar } = await abrirNotificacoes({ orcamentosEstourados: [orc(1, 130)], alertas: [ALERTA] });
    expect(sino().getAttribute('aria-label')).toBe('Notificações, 2 pendentes');
    (raiz.querySelector('.aviso-geral .menu-acoes-botao') as HTMLButtonElement).click();
    await atualizar();
    const minimizar = [...raiz.querySelectorAll<HTMLButtonElement>('.aviso-geral [role="menuitem"]')]
      .find((b) => b.textContent!.includes('Minimizar'))!;
    minimizar.click();
    await atualizar();
    await new Promise((r) => setTimeout(r));
    await atualizar();
    expect(sino().getAttribute('aria-label')).toBe('Notificações, 1 pendente');
    expect(sino().querySelector('.contador')!.textContent!.trim()).toBe('1');
    expect(raiz.querySelector('#titulo-notificacoes')!.textContent!.replace(/\s+/g, ' ').trim()).toBe('Notificações · 1 pendente');
  });

  it('se a busca falhar, o card diz que não carregou e oferece "Tentar de novo" (e não "Carregando…" para sempre)', async () => {
    const { raiz, buscas, atualizar } = await abrirNotificacoes({}, true, true);
    const card = raiz.querySelector('#card-notificacoes')!;
    expect(card.textContent).toContain('Não foi possível carregar as notificações.');
    expect(card.textContent).not.toContain('Carregando');
    const antes = buscas();
    [...card.querySelectorAll('button')].find((b) => b.textContent!.includes('Tentar de novo'))!.click();
    await atualizar();
    expect(buscas()).toBe(antes + 1);
  });

  it('o Painel não mostra mais os avisos no topo (ficam no sino)', async () => {
    const { raiz } = await abrirPainel(painel({ orcamentosEstourados: [orc(1, 130)], alertas: [ALERTA] }));
    expect(raiz.querySelector('app-painel-avisos')).toBeNull();
    expect(raiz.querySelector('.aviso-orcamentos')).toBeNull();
    expect(raiz.textContent).not.toContain('aviso pede atenção');
  });
});
