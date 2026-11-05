import { of } from 'rxjs';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #173 b) (ADR 0044): a sincronização passa a classificar sozinha o troco "sem dúvida" das compras de renda variável. O
 * Painel diz o que mudou (Nielsen 1), leva aos lançamentos em Transações (detalhe por navegação, ADR 0018) e deixa
 * desfazer ou dispensar (Nielsen 3).
 */
describe('Regressão: trocos classificados na sincronização', () => {
  const trocos = { quantidade: 2, total: 5.4, ids: [7, 8], desde: '2026-09-01', ate: '2026-09-14' };
  const aviso = (raiz: HTMLElement) => raiz.querySelector<HTMLElement>('.aviso-trocos');

  it('o aviso diz quantos, quanto e por quê, e "Ver em Transações" leva só a esses lançamentos', async () => {
    const { raiz } = await abrirPainel(painel({ trocosAutomaticos: trocos }));
    const texto = aviso(raiz)!.textContent!.replace('ⓘ', '').replace(/\s+/g, ' ');
    expect(texto).toContain('2 créditos de troco das compras de FIIs e ações (R$ 5,40) foram para “Investimentos” na sincronização');
    const link = new URL(aviso(raiz)!.querySelector('a[href^="/transacoes"]')!.getAttribute('href')!, 'http://x');
    expect(Object.fromEntries(link.searchParams)).toEqual({ inicio: '2026-09-01', fim: '2026-09-14', ids: '7,8', de: 'trocos-automaticos' });
  });

  it('"Desfazer" chama a API e recarrega o Painel', async () => {
    let desfeitos = 0;
    const t = await abrirPainel(painel({ trocosAutomaticos: trocos }), '/painel', {
      desfazerTrocosAutomaticos: () => { desfeitos++; return of({ transacoesAtualizadas: 2 }); }
    });
    Array.from(aviso(t.raiz)!.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Desfazer')!.click();
    await t.atualizar();
    expect(desfeitos).toBe(1);
    expect(t.chamadas).toHaveLength(2);
  });

  it('"Ok" dispensa o aviso (os trocos ficam) e recarrega o Painel', async () => {
    let dispensados = 0;
    const t = await abrirPainel(painel({ trocosAutomaticos: trocos }), '/painel', {
      dispensarTrocosAutomaticos: () => { dispensados++; return of(undefined); }
    });
    const ok = aviso(t.raiz)!.querySelector<HTMLButtonElement>('[aria-label="Ok, os trocos ficam em Investimentos"]')!;
    ok.click();
    await t.atualizar();
    expect(dispensados).toBe(1);
    expect(t.chamadas).toHaveLength(2);
  });

  it('sem trocos classificados, não há aviso', async () => {
    const { raiz } = await abrirPainel(painel());
    expect(aviso(raiz)).toBeNull();
  });
});
