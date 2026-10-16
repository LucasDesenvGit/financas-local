import {
  dataIsoValida, mesAnoDaUrl, paraData, paramsTransacoes, pct, reaisOuNada, tempoRelativo, ultimoDiaMes
} from '../../src/app/compartilhado/utilidades';

/**
 * Regressão das funções de apoio usadas em várias telas: links para Transações (UX-1/UX-3/UX-4/UX-6),
 * validação do que vem na URL, "há X min" da barra superior (UX-8) e frases-resumo dos gráficos (UX-9).
 */
describe('Regressão: utilidades', () => {
  describe('paramsTransacoes (links para /transacoes)', () => {
    it('leva categoria + período + tipo + conta', () => {
      expect(paramsTransacoes({ categoriaId: 12, inicio: '2026-10-01', fim: '2026-10-31', tipo: 'despesa', contaId: 3 }))
        .toEqual({ categoria: '12', inicio: '2026-10-01', fim: '2026-10-31', tipo: 'despesa', conta: '3' });
    });

    it('"Sem categoria" vira categoria=sem (null), e categoria omitida não vai na URL', () => {
      expect(paramsTransacoes({ categoriaId: null })).toEqual({ categoria: 'sem' });
      expect(paramsTransacoes({ inicio: '2026-10-01' })).toEqual({ inicio: '2026-10-01' });
    });

    it('não leva conta 0 nem campos vazios', () => {
      expect(paramsTransacoes({ contaId: 0, inicio: '', tipo: undefined })).toEqual({});
    });
  });

  describe('datas vindas da URL', () => {
    it.each(['2026-02-28', '2028-02-29', '2026-12-31'])('aceita %s', (d) => expect(dataIsoValida(d)).toBe(true));

    it.each(['2026-02-30', '2026-13-01', '2026-1-1', '2026-02-28T00:00', '', null, undefined, 'abc'])(
      'rejeita %p', (d) => expect(dataIsoValida(d)).toBe(false));

    it('paraData não desloca o dia pelo fuso (bug clássico do new Date("yyyy-MM-dd"))', () => {
      const d = paraData('2026-03-01');
      expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 2, 1]);
    });

    it('último dia do mês considera ano bissexto', () => {
      expect(ultimoDiaMes(2028, 2)).toBe('2028-02-29');
      expect(ultimoDiaMes(2026, 2)).toBe('2026-02-28');
      expect(ultimoDiaMes(2026, 12)).toBe('2026-12-31');
    });

    it('mesAnoDaUrl aceita só ano 2000–2100 e mês 1–12', () => {
      const url = (q: Record<string, string>) => ({ get: (n: string) => q[n] ?? null });
      expect(mesAnoDaUrl(url({ ano: '2026', mes: '9' }))).toEqual({ ano: 2026, mes: 9 });
      expect(mesAnoDaUrl(url({ ano: '1999', mes: '9' }))).toBeNull();
      expect(mesAnoDaUrl(url({ ano: '2026', mes: '13' }))).toBeNull();
      expect(mesAnoDaUrl(url({ ano: '2026', mes: '1.5' }))).toBeNull();
      expect(mesAnoDaUrl(url({ ano: '2026' }))).toBeNull();
    });
  });

  describe('tempoRelativo (status da sincronização, UX-8)', () => {
    const agora = new Date(2026, 9, 16, 15, 0, 0);
    const antes = (ms: number) => new Date(agora.getTime() - ms);

    it.each([
      [antes(30_000), 'agora há pouco'],
      [antes(5 * 60_000), 'há 5 min'],
      [antes(2 * 3_600_000), 'há 2 h'],
      [new Date(2026, 9, 16, 0, 30), 'há 14 h'],
      [new Date(2026, 9, 15, 23, 0), 'ontem'],
      [new Date(2026, 9, 13, 9, 0), 'há 3 dias'],
      [new Date(2026, 9, 6, 9, 0), 'em 06/10']
    ])('%s → "%s"', (quando, esperado) => expect(tempoRelativo(quando, agora)).toBe(esperado));

    it('logo depois da meia-noite, algo de 40 min atrás é "ontem" (dia do calendário, não 24 h)', () => {
      expect(tempoRelativo(new Date(2026, 9, 15, 23, 50), new Date(2026, 9, 16, 0, 30))).toBe('ontem');
    });
  });

  describe('frases-resumo dos gráficos (UX-9)', () => {
    it('pct: inteiro a partir de 10%, uma casa abaixo disso, sem sinal', () => {
      expect(pct(12.3)).toBe('12%');
      expect(pct(0.4)).toBe('0,4%');
      expect(pct(-7.5)).toBe('7,5%');
    });

    it('reaisOuNada: com "Ocultar valores" ligado não devolve nenhum valor', () => {
      expect(reaisOuNada(1234.5, true)).toBeNull();
      expect(reaisOuNada(1234.5, false)).toMatch(/^R\$\s1\.235$/);
    });
  });
});
