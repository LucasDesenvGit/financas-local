import { dataPeloValorMensal, planejarMetas } from '../../src/app/paginas/metas-plano';
import { abrirMetas, meta } from '../apoio/metas';
import { plano } from '../apoio/plano-metas';

/**
 * #359: dá para criar uma meta sem data alvo (a data vai vazia); e, no formulário, "Quanto vou guardar por mês" calcula
 * quando a meta chega ao alvo e preenche a data alvo, com a frase dizendo o mês e quantos meses.
 */
const hoje = new Date();
const mesDaqui = (n: number) => {
  const d = new Date(hoje.getFullYear(), hoje.getMonth() + n, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`;
};

describe('Regressão: data alvo pelo valor por mês (#359)', () => {
  beforeEach(() => localStorage.clear());

  it('a conta: quantos meses, contando o atual, e o mês em que chega', () => {
    expect(dataPeloValorMensal(3000, 1000, { ano: 2026, mes: 10 })).toEqual({ meses: 3, data: '2026-12-01' });
    expect(dataPeloValorMensal(999.99, 1000, { ano: 2026, mes: 10 })).toEqual({ meses: 1, data: '2026-10-01' });
    expect(dataPeloValorMensal(3000.01, 1000, { ano: 2026, mes: 10 })).toEqual({ meses: 4, data: '2027-01-01' });
    expect(dataPeloValorMensal(5000, 1000, { ano: 2026, mes: 11 })).toEqual({ meses: 5, data: '2027-03-01' });
    // Centavos sem erro de arredondamento: 0,30 ÷ 0,10 = 3 meses, e não 4.
    expect(dataPeloValorMensal(0.3, 0.1, { ano: 2026, mes: 1 })!.meses).toBe(3);
    expect(dataPeloValorMensal(3000, 0, { ano: 2026, mes: 10 })).toBeNull();
    expect(dataPeloValorMensal(0, 500, { ano: 2026, mes: 10 })).toBeNull();
  });

  it('dá para criar uma meta sem data alvo: a data vai vazia para a API', () => {
    const { tela, criadas } = abrirMetas([]);
    tela.nova();
    tela.form!.nome = 'Reserva';
    tela.form!.valorAlvo = 30000;
    tela.salvar();
    expect(criadas).toHaveLength(1);
    expect(criadas[0].dataAlvo).toBeNull();
  });

  it('o valor por mês preenche a data alvo, a frase diz quando, e mudar o alvo recalcula', async () => {
    const { tela, raiz, atualizar, criadas } = abrirMetas([]);
    tela.nova();
    await atualizar();
    const fm = tela.form!;
    fm.nome = 'Viagem';
    tela.mudarAlvo(fm, 6000);
    fm.valorAtual = 1000;
    tela.porMesNoForm = 1000;
    tela.recalcularPorMes(fm);
    await atualizar();
    // Faltam 5.000: 5 meses, contando o atual.
    expect(fm.dataAlvo).toBe(mesDaqui(4));
    const nota = raiz.querySelector('.nota-por-mes')!.textContent!.replace(/\s+/g, ' ');
    expect(nota).toContain('Com R$ 1.000,00 por mês, você chega lá em');
    expect(nota).toContain('(5 meses)');
    // O rótulo do campo é visível (WCAG 3.3.2).
    expect([...raiz.querySelectorAll('label')].some((l) => l.textContent!.includes('Quanto vou guardar por mês'))).toBe(true);
    tela.mudarAlvo(fm, 2000);
    tela.recalcularPorMes(fm);
    await atualizar();
    // Faltam 1.000: este mês basta.
    expect(fm.dataAlvo).toBe(mesDaqui(0));
    expect(raiz.querySelector('.nota-por-mes')!.textContent).toContain('este mês');
    tela.salvar();
    expect(criadas[0].dataAlvo).toBe(mesDaqui(0));
  });

  it('com o alvo já alcançado, a frase diz que não falta nada, e a data não muda', async () => {
    const { tela, raiz, atualizar } = abrirMetas([]);
    tela.nova();
    const fm = tela.form!;
    fm.valorAlvo = 1000;
    fm.valorAtual = 1000;
    fm.dataAlvo = '2030-01-01';
    tela.porMesNoForm = 500;
    tela.recalcularPorMes(fm);
    await atualizar();
    expect(fm.dataAlvo).toBe('2030-01-01');
    expect(raiz.querySelector('.nota-por-mes')!.textContent).toContain('não falta nada');
  });

  it('o valor por mês fica gravado na meta: editar mostra o valor, e salvar manda de novo', () => {
    const reserva = meta({ id: 7, nome: 'Reserva', valorAlvo: 1000, valorAtual: 0, dataAlvo: '2027-07-01', guardarPorMes: 100 });
    const { tela, atualizadas } = abrirMetas([reserva]);
    tela.editar(reserva);
    expect(tela.porMesNoForm).toBe(100);
    tela.salvar();
    expect(atualizadas[0]).toMatchObject({ id: 7, dto: { guardarPorMes: 100 } });
    // Apagar o valor volta a deixar o plano decidir.
    tela.editar(reserva);
    tela.porMesNoForm = null;
    tela.salvar();
    expect(atualizadas[1].dto.guardarPorMes).toBeNull();
  });

  it('o plano usa o valor por mês escolhido, e a tabela para no mês em que a meta é alcançada', () => {
    const reserva = meta({ id: 7, nome: 'Reserva', valorAlvo: 1000, valorAtual: 0, dataAlvo: '2027-07-01', guardarPorMes: 100,
      reservaNome: null, saldoReserva: null });
    const v = planejarMetas(plano(), [reserva], 90).metas[0];
    expect(v.porMes).toBe(100);
    expect(v.porMesCortado).toBeNull();
    expect(v.recomendado).toBe(100);
    // 10 meses de R$ 100 (out/26 a jul/27), sem meses de R$ 0 depois.
    expect(v.meses.map((m) => m.guardar)).toEqual(new Array(10).fill(100));
    expect(v.dataProvavel).toBe('2027-07-01');
    expect(v.noPrazo).toBe(true);
  });

  it('o valor por mês não passa da parte da sobra que vai para as metas; a outra meta fica com o resto', () => {
    // Sobra de 500, 1.000, 1.200 e depois 4.500/mês; 90% para as metas: 450, 900, 1.080, 4.050.
    const casa = meta({ id: 1, nome: 'Casa', valorAlvo: 5000, valorAtual: 0, dataAlvo: null, guardarPorMes: 800, prioridade: 1,
      reservaNome: null, saldoReserva: null });
    const carro = meta({ id: 2, nome: 'Carro', valorAlvo: 20000, valorAtual: 0, dataAlvo: null, prioridade: 2,
      reservaNome: null, saldoReserva: null });
    const r = planejarMetas(plano({}, { faturas: [5500, 5000, 4800, 1500] }), [casa, carro], 90);
    const [c, o] = r.metas;
    expect(c.meses.slice(0, 3).map((m) => m.guardar)).toEqual([450, 800, 800]);
    expect(c.porMesCortado).toEqual({ mes: { ano: 2026, mes: 10 }, valor: 450 });
    expect(o.meses.slice(0, 3).map((m) => m.guardar)).toEqual([0, 100, 280]);
    const capacidade = [450, 900, 1080, 4050];
    capacidade.forEach((cap, k) => expect(c.meses[k].guardar + o.meses[k].guardar).toBeLessThanOrEqual(cap + 0.01));
  });

  it('o cartão da meta diz o valor por mês escolhido e quando a sobra não dá', async () => {
    const casa = meta({ id: 1, nome: 'Casa', valorAlvo: 5000, valorAtual: 0, dataAlvo: null, guardarPorMes: 800,
      reservaNome: null, saldoReserva: null });
    const { raiz, atualizar } = abrirMetas([casa], plano({}, { faturas: [5500, 1500] }));
    await atualizar();
    const nota = raiz.querySelector('.nota-por-mes-plano')!.textContent!.replace(/\s+/g, ' ');
    expect(nota).toContain('Você escolheu guardar R$ 800,00 por mês nesta meta.');
    expect(nota).toContain('Em out/26, a parte da sobra para as metas só dá R$ 450,00');
  });

  it('o aviso da recomendação é uma caixa só, com o limite na mesma linha e o essencial em negrito', async () => {
    const reserva = meta({ id: 7, nome: 'Reserva', valorAlvo: 30000, valorAtual: 0, dataAlvo: null, reservaNome: null, saldoReserva: null });
    const { raiz, atualizar } = abrirMetas([reserva], plano({ saldoFimDoMes: 200 }));
    await atualizar();
    const avisos = raiz.querySelectorAll('.aviso-recomendacao');
    expect(avisos).toHaveLength(1);
    expect(avisos[0].querySelector('.limite-recomendacao')!.textContent).toContain('Limitei ao que deve sobrar');
    expect([...avisos[0].querySelectorAll('strong')].map((s) => s.textContent!.replace(/\s/g, ' ').trim())).toEqual(['R$ 180,00', 'ordem de prioridade', 'R$ 200,00']);
  });
});
