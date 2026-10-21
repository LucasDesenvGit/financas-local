import { evolucaoDaMeta, resumoEvolucao } from '../../src/app/paginas/metas-evolucao';
import { PlanoDaMeta } from '../../src/app/paginas/metas-plano';
import { planejarMetas } from '../../src/app/paginas/metas-plano';
import { abrirMetas, meta } from '../apoio/metas';
import { plano as caixa } from '../apoio/plano-metas';

/**
 * #318: gráfico "Como está indo" da meta — o guardado no fim de cada mês (cada aporte muda o mês dele) e o plano até o
 * alvo. Antes não havia histórico: o aporte só somava no total, sem data.
 */
const HOJE = { ano: 2026, mes: 10 };
const VIAGEM = meta({ id: 1, valorAlvo: 3000, valorAtual: 2000, percentual: 66.7, contaId: null, reservaNome: null, saldoReserva: null });
const APORTES = [
  { data: '2026-08-05', valor: 500 },
  { data: '2026-09-05', valor: 300 },
  { data: '2026-09-20', valor: -100 },
  { data: '2026-10-02', valor: 400 }
];
const planoDe = (ateOAlvo: number[], dataProvavel: string | null): PlanoDaMeta =>
  ({ metaId: 1, falta: 1000, recomendado: ateOAlvo[0], ajustado: false, dataProvavel, noPrazo: null, paraManterPrazo: null,
    meses: [], ateOAlvo, degraus: [], dependeDaFatura: null });

describe('Regressão: gráfico "Como está indo" da meta (#318)', () => {
  beforeEach(() => localStorage.clear());

  it('o guardado de cada mês é o total de hoje menos os aportes que vieram depois (a retirada conta)', () => {
    const ev = evolucaoDaMeta(VIAGEM, APORTES, null, HOJE);
    expect(ev.rotulos).toEqual(['ago/26', 'set/26', 'out/26']);
    // out: 2000; set: 2000 − 400 = 1600; ago: 1600 − (300 − 100) = 1400.
    expect(ev.guardado).toEqual([1400, 1600, 2000]);
    expect(ev.plano).toEqual([null, null, null]);
    expect(ev.aportes[0]).toEqual({ data: '2026-10-02', valor: 400 });
  });

  it('o plano parte do total de hoje e chega ao alvo no mês da data provável, sem passar dele', () => {
    const ev = evolucaoDaMeta(VIAGEM, [], planoDe([300, 300, 300, 300], '2027-01-01'), HOJE);
    expect(ev.rotulos).toEqual(['out/26', 'nov/26', 'dez/26', 'jan/27']);
    expect(ev.guardado).toEqual([2000, null, null, null]);
    // O mês atual entra no ponto de novembro: 2000 + 300 + 300; e o último para no alvo (3000).
    expect(ev.plano).toEqual([2000, 2600, 2900, 3000]);
  });

  it('o alvo alcançado este mês: um ponto só, já no fim do mês', () => {
    const ev = evolucaoDaMeta(VIAGEM, [], planoDe([1000], '2026-10-01'), HOJE);
    expect(ev.rotulos).toEqual(['out/26']);
    expect(ev.plano).toEqual([3000]);
  });

  it('o plano do assistente devolve os meses até o alvo, e o último é o da data provável', () => {
    const m = meta({ id: 1, valorAlvo: 10000, valorAtual: 0, dataAlvo: null, prioridade: 1, contaId: null, reservaNome: null, saldoReserva: null });
    const p = planejarMetas(caixa(), [m], 90).metas[0];
    expect(p.ateOAlvo.reduce((s, v) => s + v, 0)).toBeCloseTo(10000, 1);
    const ultimo = evolucaoDaMeta(m, [], p, HOJE).rotulos.at(-1);
    const [ano, mes] = p.dataProvavel!.split('-').map(Number);
    expect(ultimo).toBe(evolucaoDaMeta(m, [], planoDe([0], null), { ano, mes }).rotulos[0]);
  });

  it('o resumo diz quanto foi guardado, os aportes e quando chega; com "ocultar valores", só percentual', () => {
    const p = planoDe([300, 300, 300, 300], '2027-01-01');
    const ev = evolucaoDaMeta(VIAGEM, APORTES, p, HOJE);
    expect(resumoEvolucao(VIAGEM, ev, p, false).replace(/\s/g, ' '))
      .toBe('Guardado: R$ 2.000 de R$ 3.000 (67%). 4 aportes registrados desde ago/26. No plano, chega ao alvo em jan/27.');
    const oculto = resumoEvolucao(VIAGEM, ev, p, true);
    expect(oculto).toContain('Guardado: 67% do alvo.');
    expect(oculto).not.toContain('R$');
  });

  it('sem aporte registrado, o resumo diz que cada "+ aporte" vira um ponto', () => {
    const ev = evolucaoDaMeta(VIAGEM, [], null, HOJE);
    expect(resumoEvolucao(VIAGEM, ev, null, false)).toContain('Ainda sem aporte registrado');
  });

  it('o botão "Como está indo" abre e fecha o gráfico (aria-expanded) e carrega os aportes da meta', async () => {
    // O gráfico usa o tema, que lê matchMedia (o jsdom não tem).
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    const { tela, raiz, atualizar } = abrirMetas([VIAGEM]);
    const botao = [...raiz.querySelectorAll<HTMLButtonElement>('.cartao-meta button')].find((b) => b.textContent!.includes('Como está indo'))!;
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(botao.getAttribute('aria-controls')).toBe('evolucao-meta-1');
    botao.click();
    await atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(raiz.querySelector('#evolucao-meta-1 app-grafico-explicado')).not.toBeNull();
    expect(raiz.querySelector('#evolucao-meta-1')!.textContent).toContain('Ainda sem aporte registrado');
    // Sem plano (aqui, sem histórico de caixa), a série "Plano" sai da legenda.
    expect(tela.graficoEvolucao()!.data.datasets.map((d) => d.label)).toEqual(['Guardado', 'Alvo']);
    botao.click();
    await atualizar();
    expect(raiz.querySelector('#evolucao-meta-1 app-grafico-explicado')).toBeNull();
  });
});
