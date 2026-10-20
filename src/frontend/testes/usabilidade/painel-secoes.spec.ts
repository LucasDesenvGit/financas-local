import { abrirPainel, painel } from '../apoio/painel';

/**
 * Usabilidade do Painel em duas seções (UX-12, #88). Shneiderman: a posição de hoje primeiro, o mês escolhido depois;
 * Nielsen 4: cada valor fica na seção da pergunta que responde (o que não depende do mês não aparece no mês, e vice-
 * versa); Nielsen 2 e 10: "Dívida total nos cartões" com ⓘ do glossário, para não ser confundida com a fatura do mês.
 */
describe('Usabilidade: Painel em "Situação hoje" e "Mês de …"', () => {
  const secao = (raiz: HTMLElement, id: string) => raiz.querySelector<HTMLElement>(`section[aria-labelledby="${id}"]`)!;

  it('duas seções, cada uma com o seu <h2>', async () => {
    const { raiz } = await abrirPainel(painel(), '/painel?ano=2026&mes=9');
    expect(secao(raiz, 'titulo-hoje').querySelector('h2')!.textContent!.trim()).toBe('Situação hoje');
    expect(secao(raiz, 'titulo-mes').querySelector('h2')!.textContent!.trim()).toBe('Mês de setembro/2026');
  });

  it('o seletor de mês fica só na seção do mês', async () => {
    const { raiz } = await abrirPainel(painel());
    expect(secao(raiz, 'titulo-mes').querySelector('app-seletor-mes')).not.toBeNull();
    expect(secao(raiz, 'titulo-hoje').querySelector('app-seletor-mes')).toBeNull();
  });

  it('"Dívida total nos cartões" fica em "Situação hoje", com ⓘ; "Faturas do mês" fica no mês', async () => {
    const { raiz } = await abrirPainel(painel({ dividaCartoes: 2950, totalFaturasDoMes: 820 }));
    const hoje = secao(raiz, 'titulo-hoje');
    const mes = secao(raiz, 'titulo-mes');
    expect(hoje.textContent).toContain('Dívida total nos cartões');
    expect(hoje.querySelector('[aria-label="O que é Dívida total nos cartões?"]')!.tagName).toBe('BUTTON');
    expect(mes.textContent).not.toContain('Dívida total');
    expect(mes.textContent).toContain('Faturas do mês');
    expect(hoje.textContent).not.toContain('Faturas do mês');
    expect(raiz.textContent).not.toContain('Faturas de cartão');
  });

  it('"Faturas do mês" explica o que soma e diz quando não há nada no mês', async () => {
    const { raiz } = await abrirPainel(painel());
    const card = raiz.querySelector('.kpi-faturas')!;
    expect(card.textContent).toContain('a fatura de cada cartão que fecha no mês');
    expect(card.textContent).toContain('Nada lançado nos cartões neste mês.');
  });
});
