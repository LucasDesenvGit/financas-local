import { abrirSugestao, sugestao } from '../apoio/sugestao-orcamentos';

/**
 * Regressão da sugestão de orçamentos (#39): o que é gravado, o total do mês e a distribuição da renda.
 * A regra de cálculo fica na API (SugestaoOrcamentoService); aqui, a revisão na tela.
 */
describe('Regressão: sugestão de orçamentos', () => {
  it('categoria que já tem orçamento vem desmarcada e o total do mês conta o valor atual dela', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(t.caixa('Mercado').checked).toBe(false);
    // Moradia 1.210 + Lazer 160 (marcadas) + Mercado 900 (atual, fica como está) = 2.270
    expect(t.total()).toMatch(/Total do mês R\$\s2\.270,00 de R\$\s6\.400,00 disponíveis ✔/);
  });

  it('editar um valor atualiza o total e a distribuição da renda na hora', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    await t.digitar('Moradia', '3000');
    expect(t.total()).toMatch(/R\$\s4\.060,00/);
    const legenda = t.raiz.querySelector('figcaption')!.textContent!.replace(/\s+/g, ' ');
    // Essenciais: 3.000 + 900 de 8.000 = 49%; estilo de vida: 160 = 2%; sobra: 49%
    expect(legenda).toContain('Essenciais 49%');
    expect(legenda).toContain('Estilo de vida 2%');
    expect(legenda).toContain('Sobra para guardar 49%');
  });

  it('passar do disponível é avisado no rodapé', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    await t.digitar('Moradia', '6000');
    expect(t.total()).toContain('passou do disponível');
  });

  it('aplicar envia só as marcadas, com o valor editado, e não sobrescreve quem ficou desmarcado', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    await t.digitar('Lazer', '200');
    t.botaoAplicar().click();
    expect(t.aplicados).toEqual([{ ano: 2026, mes: 11, itens: [{ categoriaId: 1, valor: 1210 }, { categoriaId: 3, valor: 200 }] }]);
    expect(t.emitidos).toEqual([2]);
  });

  it('marcar uma categoria que já tem orçamento passa a gravar o sugerido nela', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    await t.marcar('Mercado');
    t.botaoAplicar().click();
    expect(t.aplicados[0].itens).toContainEqual({ categoriaId: 2, valor: 1090 });
  });

  it('valor zerado ou apagado não é gravado', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    await t.digitar('Lazer', '');
    t.botaoAplicar().click();
    expect(t.aplicados[0].itens.map((i) => i.categoriaId)).toEqual([1]);
  });

  it('sem renda, não mostra a barra de distribuição nem compara com o disponível', async () => {
    const t = abrirSugestao(sugestao({ rendaMedia: 0, disponivel: 0, metaPoupanca: 0,
      avisos: ['Sem receitas nos últimos meses: a sugestão usa só o seu histórico de gastos, sem teto de renda.'] }));
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(t.raiz.querySelector('.distribuicao')).toBeNull();
    expect(t.total()).not.toContain('disponíveis');
    expect(t.raiz.querySelector('.avisos-sugestao')!.textContent).toContain('Sem receitas');
  });
});
