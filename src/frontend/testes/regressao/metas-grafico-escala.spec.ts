import { comoLerEvolucao, escalaDosAportes, evolucaoDaMeta, resumoEvolucao } from '../../src/app/paginas/metas-evolucao';
import { abrirMetas, meta } from '../apoio/metas';

/**
 * #359: o gráfico "Como está indo" acompanha os aportes: todos no mesmo dia, um ponto por aporte na hora e minuto; todos
 * no mesmo mês, um ponto por dia; em meses diferentes, mês a mês (#318). E a meta concluída diz isso numa caixa verde.
 */
const HOJE = { ano: 2026, mes: 10 };
const TESTE = meta({ id: 6, nome: 'Teste', valorAlvo: 1000, valorAtual: 2000, percentual: 100, concluida: true,
  contaId: null, reservaNome: null, saldoReserva: null });

describe('Regressão: escala do gráfico da meta (#359)', () => {
  beforeEach(() => localStorage.clear());

  it('a escala sai dos aportes: mesmo dia → hora; mesmo mês → dia; meses diferentes → mês', () => {
    expect(escalaDosAportes([])).toBe('mes');
    expect(escalaDosAportes([{ data: '2026-10-23', valor: 500 }, { data: '2026-10-23', valor: 500 }])).toBe('hora');
    expect(escalaDosAportes([{ data: '2026-10-02', valor: 500 }, { data: '2026-10-23', valor: 500 }])).toBe('dia');
    expect(escalaDosAportes([{ data: '2026-09-30', valor: 500 }, { data: '2026-10-01', valor: 500 }])).toBe('mes');
  });

  it('no mesmo dia: o ponto de antes e um por aporte, na hora e minuto, somando', () => {
    const ev = evolucaoDaMeta(TESTE, [{ data: '2026-10-23', valor: 500, hora: '18:58' }, { data: '2026-10-23', valor: 500, hora: '18:59' }],
      null, HOJE, '2026-10-23');
    expect(ev.escala).toBe('hora');
    expect(ev.rotulos).toEqual(['Antes', '18:58', '18:59']);
    expect(ev.guardado).toEqual([1000, 1500, 2000]);
    expect(ev.plano.every((v) => v === null)).toBe(true);
    expect(resumoEvolucao(TESTE, ev, null, false)).toContain('2 aportes registrados em 23/10/2026.');
    expect(comoLerEvolucao('hora')).toContain('na hora em que foi registrado');
  });

  it('no mesmo mês: um ponto por dia, da véspera do primeiro aporte até hoje', () => {
    const ev = evolucaoDaMeta(TESTE, [{ data: '2026-10-20', valor: 500 }, { data: '2026-10-22', valor: 500 }], null, HOJE, '2026-10-23');
    expect(ev.escala).toBe('dia');
    expect(ev.rotulos).toEqual(['19/10', '20/10', '21/10', '22/10', '23/10']);
    expect(ev.guardado).toEqual([1000, 1500, 1500, 2000, 2000]);
    expect(resumoEvolucao(TESTE, ev, null, false)).toContain('2 aportes registrados desde 20/10.');
  });

  it('no mesmo mês, num mês que já passou: até o dia do último aporte; a véspera do dia 1 é do mês anterior', () => {
    const ev = evolucaoDaMeta(TESTE, [{ data: '2026-09-01', valor: 500 }, { data: '2026-09-03', valor: 500 }], null, HOJE, '2026-10-23');
    expect(ev.rotulos).toEqual(['31/08', '01/09', '02/09', '03/09']);
  });

  it('a meta concluída mostra "Meta alcançada" numa caixa verde, e a lista de aportes mostra a hora', async () => {
    // O gráfico usa o tema, que lê matchMedia (o jsdom não tem).
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    const { raiz, atualizar, tela } = abrirMetas([TESTE]);
    await atualizar();
    const caixa = raiz.querySelector('#meta-6-conteudo .aviso-concluida')!;
    expect(caixa.textContent!.replace(/\s+/g, ' ')).toContain('Meta alcançada! Você guardou R$ 2.000,00 de R$ 1.000,00.');
    expect(caixa.querySelector('strong')!.textContent).toBe('Meta alcançada!');
    tela.aportesDaMeta.set({ metaId: 6, aportes: [{ data: '2026-10-23', valor: 500, hora: '18:58' }] });
    tela.evolucaoAberta.set(6);
    await atualizar();
    expect(raiz.querySelector('.lista-aportes li')!.textContent!.replace(/\s+/g, ' ')).toContain('23/10/2026 às 18:58');
  });
});
