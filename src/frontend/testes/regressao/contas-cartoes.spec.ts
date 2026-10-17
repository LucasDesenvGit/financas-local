import { TipoConta } from '../../src/app/core/modelos';
import { abrirContas, conta, fatura } from '../apoio/contas';

/** Regressão de Contas e cartões (ADR 0024): valores do cartão, banco/conta do cartão no formulário e histórico. */
describe('Regressão: Contas e cartões', () => {
  const texto = (el: Element | null) => el!.textContent!.replace(/\s+/g, ' ').trim();

  it('"Fatura do mês" vem do que foi lançado no mês, não do saldo (que é a dívida total)', async () => {
    const t = await abrirContas();
    expect(texto(t.cartao(2).querySelector('.valor-fatura-mes strong'))).toMatch(/R\$\s820,00/);
    expect(texto(t.cartao(2).querySelector('.valor-divida strong'))).toMatch(/R\$\s2\.950,00/);
    expect(texto(t.cartao(2).querySelector('.valor-fatura-mes small'))).toMatch(/^vence 10\/10 fechada$/);
    // Cartão sem nada no mês: zero, sem inventar a partir do saldo.
    expect(texto(t.cartao(3).querySelector('.valor-fatura-mes strong'))).toMatch(/R\$\s0,00/);
  });

  it('o resumo soma faturas do mês, dívida e limite disponível dos cartões', async () => {
    const t = await abrirContas();
    const valores = Array.from(t.raiz.querySelectorAll('.kpis-contas .kpi strong')).map(texto);
    expect(valores[0]).toMatch(/R\$\s2\.500,00/);
    expect(valores[1]).toMatch(/R\$\s820,00/);
    expect(valores[2]).toMatch(/R\$\s4\.480,00/);
    expect(valores[3]).toMatch(/R\$\s2\.050,00/);
  });

  it('a conta mostra os cartões dela e o cartão mostra a conta dele', async () => {
    const t = await abrirContas();
    expect(texto(t.raiz.querySelector('.cartao-conta .cartoes-da-conta'))).toBe('Cartões: Ultravioleta');
    expect(t.cartao(2).textContent).toContain('da conta Nubank conta');
  });

  it('salvar cartão manda o banco escolhido e a conta do cartão; conta comum nunca manda conta vinculada', async () => {
    const t = await abrirContas();
    t.tela.editar(t.tela.cartoes().find((c) => c.id === 3)!);
    t.tela.form!.banco = 'itau';
    t.tela.form!.contaVinculadaId = 1;
    t.tela.salvar();
    t.tela.editar(t.tela.contasBancarias()[0]);
    t.tela.form!.contaVinculadaId = 99;
    t.tela.salvar();
    // A conta Nubank tem o banco detectado (não escolhido): continua "detectar" (null).
    expect(t.salvas.map((s) => [s.id, s.dto.banco, s.dto.contaVinculadaId])).toEqual([[3, 'itau', 1], [1, null, null]]);
  });

  it('banco detectado vem como "Detectar pelo nome (Nubank)" e não é gravado como escolha', async () => {
    const contas = [conta({ id: 1, nome: 'Conta Nubank', banco: 'nubank', bancoDefinido: false })];
    const t = await abrirContas('/contas', contas);
    t.tela.editar(contas[0]);
    await t.atualizar();
    const select = t.raiz.querySelector<HTMLSelectElement>('select[name=banco]')!;
    expect(select.options[select.selectedIndex].textContent!.trim()).toBe('Detectar pelo nome (Nubank)');
    expect(t.tela.form!.banco).toBeNull();
  });

  it('"Outro (sem banco)" escolhido continua escolhido ao editar de novo', async () => {
    const contas = [conta({ id: 1, nome: 'Carteira', tipo: TipoConta.Dinheiro, banco: null, bancoDefinido: true })];
    const t = await abrirContas('/contas', contas);
    t.tela.editar(contas[0]);
    expect(t.tela.form!.banco).toBe('outro');
  });

  it('histórico mostra 6 faturas e "Mostrar mais antigas" com o que falta; situação em texto', async () => {
    const faturas = Array.from({ length: 9 }, (_, i) => fatura({ id: i + 1, situacao: i === 0 ? 'aberta' : 'paga' }));
    const t = await abrirContas('/contas?faturas=2', undefined, { 2: faturas });
    const linhas = () => t.cartao(2).querySelectorAll('.tabela-historico tbody tr');
    expect(linhas().length).toBe(6);
    expect(texto(linhas()[0].querySelector('td:last-child'))).toBe('em aberto');
    expect(texto(linhas()[1].querySelector('td:last-child'))).toBe('paga');
    const mais = Array.from(t.cartao(2).querySelectorAll<HTMLButtonElement>('.historico-faturas button.link'))
      .find((b) => b.textContent!.includes('Mostrar mais antigas'))!;
    expect(texto(mais)).toBe('Mostrar mais antigas (3)');
    mais.click();
    await t.atualizar();
    expect(linhas().length).toBe(9);
  });

  it('cartão manual sem faturas explica de onde vem o histórico', async () => {
    const t = await abrirContas('/contas?faturas=3');
    expect(t.cartao(3).querySelector('.historico-faturas')!.textContent).toContain('O histórico vem do banco pelo Open Finance');
  });

  it('cartão acima do limite diz quanto passou, em vez de "disponível" negativo', async () => {
    const t = await abrirContas('/contas', [conta({ id: 9, nome: 'Cartão manual', tipo: TipoConta.CartaoCredito, saldo: -2600,
      limiteCredito: 2000, limiteDisponivel: -600 })]);
    const texto9 = t.cartao(9).textContent!.replace(/\s+/g, ' ');
    expect(texto9).toMatch(/acima do limite em R\$\s600,00/);
    expect(texto9).not.toContain('disponível -');
  });

  it('cada conta e cartão leva às suas transações', async () => {
    const t = await abrirContas();
    const link = t.raiz.querySelector<HTMLAnchorElement>('[aria-label="Ver transações de Ultravioleta"]')!;
    expect(link.getAttribute('href')).toBe('/transacoes?conta=2');
  });
});
