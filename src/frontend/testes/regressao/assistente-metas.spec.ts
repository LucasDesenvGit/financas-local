import { abrirMetas } from '../apoio/metas';
import { METAS, plano } from '../apoio/plano-metas';

/**
 * Regressão do assistente de metas (#90). O cálculo fica na API (AssistenteMetasService); aqui, o que a tela diz em
 * cada situação e a ordem de prioridade enviada.
 */
describe('Regressão: assistente de metas', () => {
  const texto = (raiz: HTMLElement, nome: string) =>
    Array.from(raiz.querySelectorAll<HTMLElement>('.cartao-meta')).find((c) => c.querySelector('strong')!.textContent === nome)!
      .textContent!.replace(/\s+/g, ' ');

  it('mostra o recomendado do mês e a divisão igual como referência', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    expect(texto(t.raiz, 'Viagem')).toMatch(/Este mês: guarde R\$\s706,92\s*Divisão igual seria R\$\s785,71\/mês · faltam R\$\s11\.000,00/);
  });

  it('quando a sobra não cobre o prazo, diz a data provável e quanto os próximos meses precisam', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    const aviso = Array.from(t.raiz.querySelectorAll('.aviso-prazo')).map((a) => a.textContent!.replace(/\s+/g, ' '));
    expect(aviso).toHaveLength(1);
    expect(aviso[0]).toMatch(/Nesse ritmo você chega em set\.?\/2028 \(meta: jun\.?\/2027\)\. Para manter o prazo, os próximos meses precisam de R\$\s7\.323,77\./);
  });

  it('meta sem data mostra a data provável no ritmo recomendado', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    expect(texto(t.raiz, 'Emergência')).toMatch(/Nesse ritmo você chega em mai\.?\/2027\./);
  });

  it('sobra negativa no mês recomenda R$ 0 e aponta para a Projeção', async () => {
    const t = abrirMetas(METAS, plano({ sobraPrevista: -250, capacidade: 0, totalRecomendado: 0, sobraNegativa: true }));
    await t.atualizar();
    const bloco = t.raiz.querySelector('.assistente-metas')!;
    expect(bloco.textContent).toContain('A projeção deste mês fecha no vermelho: recomendo guardar R$ 0 agora.');
    expect(bloco.querySelector('a')!.getAttribute('href')).toBe('/projecao');
  });

  it('descer a prioridade envia a nova ordem das metas em aberto', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    t.raiz.querySelector<HTMLButtonElement>('[aria-label="Descer a prioridade de Viagem"]')!.click();
    await t.atualizar();
    expect(t.ordens).toEqual([[2, 1, 3]]);
  });
});
