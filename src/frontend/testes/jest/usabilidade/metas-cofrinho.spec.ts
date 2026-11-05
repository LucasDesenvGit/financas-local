import { abrirMetas } from '../apoio/metas';

/**
 * Usabilidade do cofrinho na meta (#89). Progressive Disclosure: o campo só aparece para conta com cofrinhos;
 * Nielsen 6: escolher numa lista com nome e saldo, sem digitar; Nielsen 2: "cofrinho (reserva)".
 */
describe('Usabilidade: cofrinho na meta', () => {
  async function abrirFormulario() {
    const t = abrirMetas();
    await t.atualizar();
    t.tela.nova();
    await t.atualizar();
    return t;
  }

  it('sem conta, ou com conta sem cofrinhos, o campo do cofrinho não aparece', async () => {
    const t = await abrirFormulario();
    expect(t.raiz.querySelector('select[name=reserva]')).toBeNull();
    await t.escolher('conta', 'Cartão Roxo');
    expect(t.raiz.querySelector('select[name=reserva]')).toBeNull();
  });

  it('conta com cofrinhos mostra "Qual cofrinho (reserva)?" com nome e saldo, e a opção de seguir pela conta', async () => {
    const t = await abrirFormulario();
    await t.escolher('conta', 'Mercado Pago');
    const sel = t.raiz.querySelector<HTMLSelectElement>('select[name=reserva]')!;
    expect(sel.closest('label')!.textContent).toContain('Qual cofrinho (reserva)?');
    expect(Array.from(sel.options).map((o) => o.text.replace(/\s+/g, ' ').trim())).toEqual([
      'Saldo disponível da conta (sem cofrinhos)', 'Emergência — R$ 12.400,00', 'Viagem — R$ 3.250,00'
    ]);
  });

  it('trocar a conta limpa o cofrinho escolhido', async () => {
    const t = await abrirFormulario();
    await t.escolher('conta', 'Mercado Pago');
    await t.escolher('reserva', 'Viagem');
    expect(t.tela.form!.reservaNome).toBe('Viagem');
    await t.escolher('conta', 'Cartão Roxo');
    expect(t.tela.form!.reservaNome).toBeNull();
  });

  it('explica que o saldo do cofrinho é estimado e leva para Investimentos › Reservas', async () => {
    const t = await abrirFormulario();
    await t.escolher('conta', 'Mercado Pago');
    const link = Array.from(t.raiz.querySelectorAll('a')).find((a) => a.textContent!.includes('Investimentos › Reservas'))!;
    expect(link.getAttribute('href')).toBe('/investimentos?aba=reservas');
  });
});
