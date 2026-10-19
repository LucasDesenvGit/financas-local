import { BANCOS } from '../../src/app/compartilhado/bancos';

/** Quantos números cada comando de path do SVG pede (Z não pede nenhum). */
const ARIDADE: Record<string, number> = { m: 2, l: 2, t: 2, h: 1, v: 1, c: 6, s: 4, q: 4, a: 7, z: 0 };

/**
 * Lê o atributo `d` como o navegador: comandos e números, com as abreviações do Simple Icons (sem espaço antes do
 * sinal, ".5.5" = 0.5 0.5 e, no arco, as bandeiras 0/1 grudadas). Devolve o erro, ou null se o path está completo.
 */
function erroNoPath(d: string): string | null {
  let i = 0;
  const pular = () => { while (i < d.length && /[\s,]/.test(d[i])) i++; };
  const numero = (): boolean => {
    const m = /^[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?/.exec(d.slice(i));
    if (!m) return false;
    i += m[0].length;
    return true;
  };
  const bandeira = (): boolean => {
    if (d[i] !== '0' && d[i] !== '1') return false;
    i++;
    return true;
  };

  pular();
  if (!/[Mm]/.test(d[i] ?? '')) return 'não começa com M';
  while (i < d.length) {
    pular();
    if (i >= d.length) break;
    const comando = d[i];
    const aridade = ARIDADE[comando.toLowerCase()];
    if (aridade === undefined) return `comando inválido "${comando}" na posição ${i}`;
    i++;
    if (aridade === 0) continue;
    // Um comando pode repetir os argumentos (ex.: "l1 2 3 4" = duas linhas); pelo menos uma vez.
    let grupos = 0;
    for (;;) {
      pular();
      if (i >= d.length || /[A-Za-z]/.test(d[i])) break;
      for (let k = 0; k < aridade; k++) {
        pular();
        const arcoBandeira = comando.toLowerCase() === 'a' && (k === 3 || k === 4);
        if (!(arcoBandeira ? bandeira() : numero()))
          return `"${comando}" incompleto na posição ${i} (esperava ${aridade} números, faltou o ${k + 1}º)`;
      }
      grupos++;
    }
    if (grupos === 0) return `"${comando}" sem números na posição ${i}`;
  }
  return null;
}

/** Regressão do #147: o logo do Mercado Pago veio cortado em 3000 caracteres e o navegador não o desenhava. */
describe('Regressão: logos dos bancos (bancos.ts)', () => {
  const comDesenho = Object.entries(BANCOS).filter(([, b]) => b.caminho);

  it('há bancos com desenho do Simple Icons para conferir', () => {
    expect(comDesenho.length).toBeGreaterThan(0);
  });

  it.each(comDesenho)('o caminho de "%s" é um path de SVG completo', (_codigo, banco) => {
    expect(erroNoPath(banco.caminho!)).toBeNull();
  });

  it('o conferidor pega um path cortado no meio de uma curva (o caso do #147)', () => {
    expect(erroNoPath('M1 2c-.287-.217-1.425-.93-4.46')).toMatch(/incompleto/);
    expect(erroNoPath('M11.115 16.479a.93.927 0 0 1-.939-.886z')).toBeNull();
  });

  it('o Mercado Pago tem o desenho inteiro e fecha o contorno', () => {
    const d = BANCOS['mercadopago'].caminho!;
    expect(d.length).toBeGreaterThan(3000);
    expect(d.trimEnd().endsWith('z')).toBe(true);
  });
});
