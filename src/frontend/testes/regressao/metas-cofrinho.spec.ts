import { abrirMetas, meta } from '../apoio/metas';

/**
 * Regressão do cofrinho na meta (#89): de onde vem o progresso (cofrinho, nunca o saldo da conta junto), o aviso de
 * saldo estimado e o de cofrinho que sumiu do extrato. A conta do progresso é feita na API (MetasController).
 */
describe('Regressão: cofrinho na meta', () => {
  const textoDoCartao = (raiz: HTMLElement, nome: string) =>
    Array.from(raiz.querySelectorAll('.cartao-meta')).find((c) => c.querySelector('strong')!.textContent === nome)!
      .textContent!.replace(/\s+/g, ' ');

  it('meta com cofrinho mostra o valor inicial + o cofrinho, sem o saldo da conta', async () => {
    const t = abrirMetas([meta({})]);
    await t.atualizar();
    const texto = textoDoCartao(t.raiz, 'Viagem');
    expect(texto).toMatch(/R\$\s500,00 guardados \+ R\$\s3\.250,00 no cofrinho Viagem \(Mercado Pago\)/);
    expect(texto).not.toContain('em Mercado Pago');
  });

  it('cofrinho com saldo estimado avisa e leva para Reservas', async () => {
    const t = abrirMetas([meta({ nome: 'Emergência', reservaNome: 'Emergência', saldoReserva: 12400, reservaConferir: true })]);
    await t.atualizar();
    const link = Array.from(t.raiz.querySelectorAll('.cartao-meta a')).find((a) => a.textContent!.includes('confira'))!;
    expect(link.getAttribute('href')).toBe('/investimentos?aba=reservas');
  });

  it('cofrinho que sumiu do extrato vira aviso com caminho para corrigir (não um zero silencioso)', async () => {
    const t = abrirMetas([meta({ saldoReserva: null, reservaNaoEncontrada: true, valorAtual: 500 })]);
    await t.atualizar();
    const aviso = t.raiz.querySelector('.aviso-cofrinho')!;
    expect(aviso.textContent).toContain('Cofrinho "Viagem" não encontrado no extrato de Mercado Pago.');
    (Array.from(aviso.querySelectorAll('button')).find((b) => b.textContent!.includes('Editar a meta'))!).click();
    await t.atualizar();
    // Ao editar, o cofrinho sumido continua escolhido (marcado), para não ser trocado sem querer.
    expect(t.tela.form!.reservaNome).toBe('Viagem');
  });

  it('meta sem cofrinho segue como antes: valor inicial + saldo da conta', async () => {
    const t = abrirMetas([meta({ nome: 'Carro', reservaNome: null, saldoReserva: null, saldoConta: 900, valorAtual: 1400 })]);
    await t.atualizar();
    expect(textoDoCartao(t.raiz, 'Carro')).toMatch(/R\$\s500,00 guardados \+ R\$\s900,00 em Mercado Pago/);
  });
});
