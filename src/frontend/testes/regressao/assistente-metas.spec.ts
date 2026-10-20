import { Meta } from '../../src/app/core/modelos';
import { planejarMetas } from '../../src/app/paginas/metas-plano';
import { lerAjustes } from '../../src/app/paginas/metas.component';
import { abrirMetas, abrirMetasPelaUrl, meta } from '../apoio/metas';
import { METAS, plano } from '../apoio/plano-metas';

/** Viagem de R$ 22.000 com prazo em set/2027 (12 meses contando out/2026): divisão igual de R$ 1.833,33. */
const VIAGEM: Meta = meta({ id: 1, nome: 'Viagem', valorAlvo: 22000, valorAtual: 0, dataAlvo: '2027-09-01', reservaNome: null,
  saldoReserva: null, prioridade: 1, aporteMensalSugerido: 1833.33 });
const EMERGENCIA: Meta = meta({ id: 2, nome: 'Emergência', valorAlvo: 8000, valorAtual: 0, dataAlvo: null, reservaNome: null,
  saldoReserva: null, prioridade: 1 });
/** O cenário do #149: fatura alta agora (parcelas), caindo a partir de janeiro. Sobra: 500, 1.000, 1.200 e 4.500/mês. */
const FATURA_CAINDO = { faturas: [5500, 5000, 4800, 1500] };

/**
 * Regressão do assistente de metas (#90, #149). O cálculo do plano é feito na tela (metas-plano.ts) sobre o caixa da
 * API: receitas − despesas − faturas que vencem no mês.
 */
describe('Regressão: assistente de metas (cálculo)', () => {
  it('#149: com a fatura alta agora e caindo em janeiro, pede pouco este mês e compensa depois, cumprindo o prazo', () => {
    const r = planejarMetas(plano({}, FATURA_CAINDO), [VIAGEM], 90);
    const v = r.metas[0];
    // Antes: quase todo o valor de uma vez. Agora: a fatia deste mês pela folga dele (450 de 38.880 até o prazo).
    expect(v.recomendado).toBeCloseTo(254.63, 1);
    expect(v.recomendado).toBeLessThan(VIAGEM.aporteMensalSugerido!);
    expect(v.noPrazo).toBe(true);
    expect(v.dataProvavel).toBe('2027-09-01');
    // Sem picos: nenhum mês passa de 2× a divisão igual.
    expect(Math.max(...v.meses.map((m) => m.guardar))).toBeLessThanOrEqual(2 * 22000 / 12 + 0.01);
    // Degraus: nov e dez apertados, e de jan a set o mesmo valor, com a fatura menor.
    const ultimo = v.degraus[v.degraus.length - 1];
    expect(ultimo).toMatchObject({ inicio: { ano: 2027, mes: 1 }, fim: { ano: 2027, mes: 9 }, faturas: 1500 });
    expect(ultimo.valor).toBeCloseTo(2291.66, 1);
    expect(v.meses.slice(3, 9).every((m) => m.guardar === ultimo.valor)).toBe(true);
    expect(v.dependeDaFatura).toEqual({ aPartir: { ano: 2027, mes: 1 }, antes: 5000, depois: 1500 });
  });

  it('o total do plano é o que falta (os degraus não mudam a soma)', () => {
    const v = planejarMetas(plano({}, FATURA_CAINDO), [VIAGEM], 90).metas[0];
    expect(v.meses.reduce((s, m) => s + m.guardar, 0)).toBeCloseTo(22000, 1);
  });

  it('o mês atual não passa do saldo previsto nas contas no fim do mês (já paga a fatura)', () => {
    const r = planejarMetas(plano({ saldoFimDoMes: 200 }), [EMERGENCIA], 90);
    expect(r.limitadoPor).toBe('saldo');
    expect(r.totalRecomendado).toBeCloseTo(180, 2);
  });

  it('o mês atual não passa do que sobrou de fato no melhor mês recente', () => {
    const r = planejarMetas(plano({ melhorMesRecente: 600 }), [EMERGENCIA], 90);
    expect(r.limitadoPor).toBe('historico');
    expect(r.totalRecomendado).toBeCloseTo(600, 2);
  });

  it('quem nunca conseguiu guardar recebe um mínimo pequeno (R$ 50), se há sobra e saldo', () => {
    const r = planejarMetas(plano({ melhorMesRecente: -300 }), [EMERGENCIA], 90);
    expect(r.limitadoPor).toBe('minimo');
    expect(r.totalRecomendado).toBe(50);
  });

  it('o mínimo também vale quando a folga está nos meses seguintes (meta com prazo)', () => {
    const r = planejarMetas(plano({}, { faturas: [5950, 1500] }), [VIAGEM], 90);
    // Sobra deste mês de R$ 50 × 90% = R$ 45: pede o mínimo possível, não uma fração de centavos.
    expect(r.metas[0].recomendado).toBeCloseTo(45, 2);
  });

  it('mês no vermelho pelo caixa recomenda R$ 0', () => {
    const r = planejarMetas(plano({}, { faturas: [7000, 2500] }), [EMERGENCIA], 90);
    expect(r.sobraMes).toBe(-1000);
    expect(r.totalRecomendado).toBe(0);
  });

  it('meta com prazo não recebe mais de 2× a divisão igual num mês, mesmo com uma folga enorme nele', () => {
    const p = plano();
    p.meses[1] = { ...p.meses[1], sobra: 100000 };
    const carro = meta({ id: 3, nome: 'Carro', valorAlvo: 2400, valorAtual: 0, dataAlvo: '2027-09-01', aporteMensalSugerido: 200 });
    const v = planejarMetas(p, [carro], 90).metas[0];
    expect(v.meses[1].guardar).toBeLessThanOrEqual(400);
    expect(v.noPrazo).toBe(true);
  });

  it('"consigo guardar R$ 50 este mês": vale no lugar da recomendação e o resto do plano é refeito', () => {
    const sem = planejarMetas(plano({}, FATURA_CAINDO), [VIAGEM], 90).metas[0];
    const com = planejarMetas(plano({}, FATURA_CAINDO), [VIAGEM], 90, new Map([[1, 50]])).metas[0];
    expect(com.recomendado).toBe(50);
    expect(com.ajustado).toBe(true);
    expect(com.noPrazo).toBe(true);
    expect(com.meses.reduce((s, m) => s + m.guardar, 0)).toBeCloseTo(22000, 1);
    expect(com.meses[1].guardar).toBeGreaterThan(sem.meses[1].guardar);
  });

  it('o percentual da sobra que vai para as metas muda a recomendação', () => {
    expect(planejarMetas(plano(), [EMERGENCIA], 50).totalRecomendado).toBe(1750);
    expect(planejarMetas(plano(), [EMERGENCIA], 100).totalRecomendado).toBe(3500);
  });

  it('?guardar= aceita só pares meta:valor válidos', () => {
    expect([...lerAjustes('1:50,3:120.5,x:9,2:-1,4')]).toEqual([[1, 50], [3, 120.5]]);
    expect(lerAjustes(null).size).toBe(0);
  });
});

describe('Regressão: assistente de metas (tela)', () => {
  const texto = (raiz: HTMLElement, nome: string) =>
    Array.from(raiz.querySelectorAll<HTMLElement>('.cartao-meta')).find((c) => c.querySelector('strong')!.textContent === nome)!
      .textContent!.replace(/\s+/g, ' ');

  it('a sobra do mês mostra a conta: entra, fatura do cartão e outras despesas', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    expect(t.raiz.querySelector('.conta-sobra')!.textContent!.replace(/\s+/g, ' '))
      .toMatch(/Entra R\$\s10\.000,00 · fatura do cartão R\$\s2\.500,00 · outras despesas R\$\s4\.000,00/);
  });

  it('mudar o percentual recalcula na hora e grava a margem de segurança (o complemento)', async () => {
    const t = abrirMetas([EMERGENCIA], plano());
    await t.atualizar();
    expect(texto(t.raiz, 'Emergência')).toMatch(/Este mês: guarde R\$\s3\.150,00/);
    await t.escolher('percentual', '50%');
    expect(t.parametros).toEqual([{ 'Metas.MargemSeguranca': 50 }]);
    expect(texto(t.raiz, 'Emergência')).toMatch(/Este mês: guarde R\$\s1\.750,00/);
  });

  it('?guardar=1:50 abre com o valor escolhido; "Voltar à recomendação" tira da URL', async () => {
    const t = await abrirMetasPelaUrl('/metas?guardar=1:50', [VIAGEM], plano({}, FATURA_CAINDO));
    expect(texto(t.raiz, 'Viagem')).toMatch(/Este mês: guarde R\$\s50,00 \(o valor que você escolheu\)/);
    Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.includes('Voltar à recomendação'))!.click();
    await t.atualizar();
    expect(t.router.url).toBe('/metas');
    expect(texto(t.raiz, 'Viagem')).toMatch(/Este mês: guarde R\$\s254,6\d/);
  });

  it('dizer quanto consegue guardar grava ?guardar= na URL, sem empilhar histórico', async () => {
    const t = await abrirMetasPelaUrl('/metas', [VIAGEM], plano({}, FATURA_CAINDO));
    t.tela.ajustar(1, 120);
    await t.atualizar();
    expect(t.router.url).toBe('/metas?guardar=1:120');
  });

  it('degraus com o motivo e a nota de que o plano conta com a fatura caindo', async () => {
    const t = abrirMetas([VIAGEM], plano({}, FATURA_CAINDO));
    await t.atualizar();
    const v = texto(t.raiz, 'Viagem');
    const degraus = Array.from(t.raiz.querySelectorAll('.degraus li')).map((li) => li.textContent!.replace(/\s+/g, ' ').trim());
    expect(degraus[degraus.length - 1]).toMatch(/^R\$\s2\.291,6\d\/mês de jan\/27 a set\/27 \(faturas ~R\$\s1\.500\)$/);
    expect(v).toMatch(/conta com as faturas caindo para ~R\$\s1\.500,00 a partir de jan\/27 \(hoje ~R\$\s5\.000,00\)/);
  });

  it('quando o prazo não cabe, diz a data provável, quanto falta por mês e onde mexer', async () => {
    const carro = meta({ id: 3, nome: 'Carro', valorAlvo: 60000, valorAtual: 0, dataAlvo: '2027-06-01', reservaNome: null,
      saldoReserva: null, aporteMensalSugerido: 6666.67 });
    const t = abrirMetas([carro], plano());
    await t.atualizar();
    const aviso = t.raiz.querySelector('.aviso-prazo')!;
    expect(aviso.textContent!.replace(/\s+/g, ' ')).toMatch(/Nesse ritmo você chega em .* \(meta: jun\.?\/2027\)\. Para manter o prazo/);
    expect(Array.from(aviso.querySelectorAll('a')).map((a) => a.getAttribute('href'))).toEqual(['/anual', '/orcamentos']);
    expect(aviso.querySelector('button')!.textContent).toContain('Mudar o prazo ou o valor');
  });

  it('mês no vermelho diz isso e recomenda R$ 0, com o plano compensando depois', async () => {
    const t = abrirMetas(METAS, plano({}, { faturas: [7000, 2500] }));
    await t.atualizar();
    expect(t.raiz.querySelector('.assistente-metas')!.textContent!.replace(/\s+/g, ' '))
      .toContain('este mês fecha no vermelho: recomendo guardar R$ 0 agora. O plano compensa nos meses com mais folga.');
  });

  it('descer a prioridade envia a nova ordem das metas em aberto', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    t.raiz.querySelector<HTMLButtonElement>('[aria-label="Descer a prioridade de Viagem"]')!.click();
    await t.atualizar();
    expect(t.ordens).toEqual([[2, 1, 3]]);
  });
});
