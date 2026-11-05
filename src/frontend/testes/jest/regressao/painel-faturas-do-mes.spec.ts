import { abrirPainel, painel } from '../apoio/painel';
import { parametrosDaUrl } from '../apoio/transacoes';

/**
 * Regressão do Painel em duas seções (UX-12, #88, ajustado no #103): "Faturas do mês" muda com o mês escolhido;
 * "Dívida total nos cartões" é a posição de hoje e não muda.
 */
describe('Regressão: "Faturas do mês" segue o seletor; "Dívida total" não', () => {
  // A API devolve a mesma dívida em qualquer mês (é a de hoje) e as faturas do mês pedido.
  const porMes = (ano: number, mes: number) => painel({
    dividaCartoes: 2950,
    totalFaturasDoMes: mes === 9 ? 820 : 410,
    faturasDoMes: [{ contaId: 2, cartao: 'Cartão Roxo', valorMes: mes === 9 ? 820 : 410, parcelasMes: 100, faturaId: null,
      vencimento: null, situacao: null }]
  });
  const texto = (el: Element | null) => el!.textContent!.replace(/\s+/g, ' ').trim();

  it('trocar o mês busca o mês novo, grava na URL e muda "Faturas do mês"', async () => {
    const t = await abrirPainel(porMes, '/painel?ano=2026&mes=9');
    expect(t.chamadas).toEqual([[2026, 9]]);
    expect(texto(t.raiz.querySelector('.kpi-faturas strong'))).toMatch(/R\$\s820,00/);
    t.tela.mudarPeriodo({ ano: 2026, mes: 10 });
    await t.atualizar();
    expect(t.chamadas.at(-1)).toEqual([2026, 10]);
    expect(parametrosDaUrl(t.router)).toEqual({ ano: '2026', mes: '10' });
    expect(texto(t.raiz.querySelector('.kpi-faturas strong'))).toMatch(/R\$\s410,00/);
    expect(texto(t.raiz.querySelector('.lista-faturas li'))).toMatch(/Cartão Roxo\s*R\$\s410,00\s*R\$\s100,00 em parcelas/);
  });

  it('"Dívida total nos cartões" mostra a posição de hoje em qualquer mês', async () => {
    const t = await abrirPainel(porMes, '/painel?ano=2026&mes=9');
    const divida = () => texto(t.raiz.querySelector('section[aria-labelledby="titulo-hoje"] .kpi strong.negativo'));
    const antes = divida();
    t.tela.mudarPeriodo({ ano: 2026, mes: 10 });
    await t.atualizar();
    expect(divida()).toBe(antes);
    expect(antes).toMatch(/R\$\s2\.950,00/);
  });

  it('cada cartão em "Faturas do mês" leva às faturas dele em Contas', async () => {
    const t = await abrirPainel(porMes);
    const link = t.raiz.querySelector<HTMLAnchorElement>('.lista-faturas a')!;
    expect(link.getAttribute('aria-label')).toBe('Ver faturas de Cartão Roxo');
    expect(link.getAttribute('href')).toBe('/contas?faturas=2');
  });
});
