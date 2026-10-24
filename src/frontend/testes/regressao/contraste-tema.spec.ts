import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Regressão da varredura de ponta a ponta (20/10/2026): no tema claro, o texto suave, o positivo, o negativo, o alerta e a
 * emergência ficavam abaixo de 4,5:1 sobre os fundos (WCAG 2.2 — 1.4.3 Contrast (Minimum), AA). O jsdom não calcula
 * cor na tela; a conta é feita direto nas variáveis do styles.css, nos dois temas e em todas as paletas.
 */
const css = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');

/** Variáveis de cor (#rrggbb) de cada bloco cujo seletor atende `seletor`. */
function variaveis(seletor: RegExp): Record<string, string> {
  const resultado: Record<string, string> = {};
  for (const bloco of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    if (!seletor.test(bloco[1])) continue;
    for (const v of bloco[2].matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})\b/g)) resultado[v[1]] = v[2].toLowerCase();
  }
  return resultado;
}

function luminancia(hex: string): number {
  const canal = (i: number) => {
    const v = parseInt(hex.slice(i, i + 2), 16) / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * canal(1) + 0.7152 * canal(3) + 0.0722 * canal(5);
}

function contraste(a: string, b: string): number {
  const [x, y] = [luminancia(a), luminancia(b)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}

const TEXTOS = ['texto', 'texto-forte', 'texto-suave', 'positivo', 'negativo', 'alerta', 'emergencia', 'primaria-forte'];
const FUNDOS = ['fundo', 'superficie', 'superficie-2', 'superficie-3', 'primaria-suave', 'positivo-suave', 'negativo-suave',
  'alerta-suave', 'emergencia-suave', 'info-suave'];
const PALETAS = ['oceano', 'floresta', 'grafite', 'por-do-sol'];

/** As cores de cada tema (claro, escuro) em cada paleta, para conferir as contas em todas as combinações. */
function casosDosTemas(): [string, Record<string, string>][] {
  const casos: [string, Record<string, string>][] = [];
  for (const tema of ['claro', 'escuro']) {
    const base = variaveis(new RegExp(`^\\s*(:root,\\s*)?:root\\[data-tema="${tema}"\\]\\s*$`));
    casos.push([`${tema} (lavanda)`, base]);
    for (const p of PALETAS)
      casos.push([`${tema} (${p})`, { ...base, ...variaveis(new RegExp(`:root\\[data-paleta="${p}"\\]\\[data-tema="${tema}"\\]`)) }]);
  }
  return casos;
}

describe('Regressão: contraste das cores de texto do tema (WCAG 1.4.3)', () => {
  const casos = casosDosTemas();

  it.each(casos)('%s: todo texto tem pelo menos 4,5:1 sobre todo fundo', (_, cores) => {
    const abaixo: string[] = [];
    for (const t of TEXTOS)
      for (const f of FUNDOS) {
        expect(cores[t] && cores[f]).toBeTruthy();
        const razao = contraste(cores[t], cores[f]);
        if (razao < 4.5) abaixo.push(`--${t} sobre --${f}: ${razao.toFixed(2)}`);
      }
    expect(abaixo).toEqual([]);
  });
});

/**
 * #227: a seta do grupo do menu (--texto-suave, ou --primaria-forte quando a tela atual está no grupo fechado) e o
 * contorno de foco (--primaria) precisam de 3:1 com o fundo do menu e com o fundo do cabeçalho sob o mouse
 * (WCAG 2.2 — 1.4.11 Non-text Contrast, AA), nos dois temas e em todas as paletas.
 */
describe('Regressão: contraste da seta e do foco do menu em grupos (WCAG 1.4.11)', () => {
  const casos = casosDosTemas();

  it.each(casos)('%s: seta e foco com pelo menos 3:1 sobre o menu', (_, cores) => {
    const abaixo: string[] = [];
    for (const t of ['texto-suave', 'primaria-forte', 'primaria'])
      for (const f of ['superficie', 'superficie-2']) {
        const razao = contraste(cores[t], cores[f]);
        if (razao < 3) abaixo.push(`--${t} sobre --${f}: ${razao.toFixed(2)}`);
      }
    expect(abaixo).toEqual([]);
  });

  it('a seta usa --texto-suave e o foco do cabeçalho é o contorno padrão em --primaria', () => {
    expect(css).toMatch(/\.grupo-seta\s*\{[^}]*color:\s*var\(--texto-suave\)/);
    expect(css).toMatch(/:focus-visible\s*\{[^}]*outline:\s*2px solid var\(--primaria\)/);
    expect(css).not.toMatch(/\.grupo-cabecalho[^{]*:focus[^{]*\{[^}]*outline:\s*(none|0)/);
  });
});

describe('Regressão: ícone da categoria sobre o fundo suave dela (#336, WCAG 1.4.11)', () => {
  // As mesmas misturas do <app-icone-categoria>: o desenho é a cor meio a meio com o texto forte no tema claro e a cor
  // pura no escuro, e o fundo é 22% da cor sobre a superfície. Cores: as sugeridas e as das categorias padrão (as que a pessoa mais vê).
  const componente = readFileSync(join(__dirname, '../../src/app/compartilhado/icone-categoria.component.ts'), 'utf8');
  const sql = readFileSync(join(__dirname, '../../../../banco/02-dados-iniciais.sql'), 'utf8');
  const utilidades = readFileSync(join(__dirname, '../../src/app/compartilhado/utilidades.ts'), 'utf8');
  const categorias = sql.slice(sql.indexOf('INSERT OR IGNORE INTO Categoria'), sql.indexOf(';', sql.indexOf('INSERT OR IGNORE INTO Categoria')));
  const sugeridas = utilidades.slice(utilidades.indexOf('CORES_SUGERIDAS'), utilidades.indexOf('];', utilidades.indexOf('CORES_SUGERIDAS')));
  const cores = [...new Set([...categorias.matchAll(/'(#[0-9a-fA-F]{6})'/g), ...sugeridas.matchAll(/'(#[0-9a-fA-F]{6})'/g)].map((m) => m[1].toLowerCase()))];

  const misturar = (a: string, b: string, parteA: number) => '#' + [1, 3, 5].map((i) =>
    Math.round(parseInt(a.slice(i, i + 2), 16) * parteA + parseInt(b.slice(i, i + 2), 16) * (1 - parteA)).toString(16).padStart(2, '0')).join('');

  it('o componente usa as misturas conferidas aqui', () => {
    expect(componente).toContain('background: color-mix(in srgb, var(--cor) 22%, var(--superficie))');
    expect(componente).toContain('color: color-mix(in srgb, var(--cor) 50%, var(--texto-forte))');
    expect(componente).toContain(':host-context([data-tema="escuro"]) .icone-categoria { color: var(--cor); }');
    expect(cores.length).toBeGreaterThan(20);
  });

  it.each(casosDosTemas())('%s: o ícone tem pelo menos 3:1 sobre o fundo, em toda cor de categoria', (nome, tema) => {
    const abaixo = cores.filter((cor) => {
      const desenho = nome.startsWith('escuro') ? cor : misturar(cor, tema['texto-forte'], 0.5);
      const fundo = misturar(cor, tema['superficie'], 0.22);
      return contraste(desenho, fundo) < 3;
    });
    expect(abaixo).toEqual([]);
  });
});
