import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { Locator, Page } from '@playwright/test';
import { DATA_DOS_PRINTS } from './api-falsa';
import { expect, test } from './base';

/**
 * Prints do Manual (#752, ADR 0071). Só roda pelo `npm run prints:manual` (PRINTS_MANUAL=1): para cada linha da tabela
 * abaixo, abre a rota com a API falsa e a data fixa, corta o trecho e guarda `<id>-claro.webp` e `<id>-escuro.webp`
 * em public/imagens/manual/, mais a largura e a altura de cada id em src/app/paginas/manual/prints-manual.ts.
 * O trecho tem a largura da coluna do Manual (viewport estreito de computador: o menu continua na tela), em escala 1,
 * e o WebP sai do próprio Chromium (canvas.toBlob). Print novo: uma linha aqui e `print: { id, alt }` na seção.
 */
interface Print {
  id: string;
  rota: string;
  /** Cliques e esperas antes do print (abrir uma janela, escolher uma aba). */
  preparo?: (p: Page) => Promise<void>;
  /** Começa no topo deste elemento, na largura da página (sem ele, no topo da página, abaixo da barra de cima). */
  de?: (p: Page) => Locator;
  /** A janela inteira do navegador, da esquerda (menu e barra de cima incluídos). */
  inteira?: boolean;
  /** O print é só esta janela aberta (recorte justo no elemento). */
  janela?: boolean;
  /** Altura máxima do corte, em px (padrão 480). */
  altura?: number;
  /** Largura da janela do navegador, em px (padrão 1012: o menu mais a coluna de 760). */
  viewport?: number;
  /** Largura máxima do corte, em px (padrão 760); tabelas largas pedem `viewport` maior junto. */
  largura?: number;
}

const abas = (p: Page) => p.getByRole('tablist').first();
const janela = async (p: Page, botao: string | RegExp) => { await p.getByRole('button', { name: botao }).click(); };

const PRINTS: Print[] = [
  // O menu, a barra de cima e o começo da página, inteiros: um pouco mais largo que a coluna do Manual (a escala é de CSS).
  { id: 'conhecendo-a-tela', rota: '/painel', viewport: 901, largura: 901, altura: 480, inteira: true },
  { id: 'painel', rota: '/painel' },
  { id: 'visao-anual', rota: '/anual' },
  { id: 'relatorios-tela', rota: '/relatorios' },
  { id: 'projecao', rota: '/projecao' },
  { id: 'tipos-de-conta', rota: '/contas' },
  { id: 'conferir-banco', rota: '/conferir-banco' },
  { id: 'filtros', rota: '/transacoes', altura: 520 },
  { id: 'etiquetas', rota: '/transacoes', viewport: 1100, largura: 848, de: (p) => p.getByRole('table').first() },
  { id: 'modo-edicao', rota: '/transacoes?categoria=sem&editar=1', viewport: 1100, largura: 848, de: (p) => p.getByRole('table').first() },
  { id: 'lancamento-manual', rota: '/transacoes', janela: true, altura: 900,
    preparo: (p) => janela(p, '+ Lançamento') },
  { id: 'criar-categoria', rota: '/categorias', janela: true, altura: 900, preparo: (p) => janela(p, '+ Categoria') },
  { id: 'criar-regra', rota: '/categorias', de: abas, preparo: (p) => p.getByRole('tab', { name: 'Regras automáticas' }).click() },
  { id: 'definir-limites', rota: '/orcamentos', altura: 600 },
  { id: 'criar-meta', rota: '/metas', janela: true, altura: 900, preparo: (p) => janela(p, '+ Nova meta') },
  { id: 'cadastrar-conta', rota: '/contas-a-pagar', janela: true, altura: 900, preparo: (p) => janela(p, '+ Nova conta') },
  { id: 'assinaturas-recorrentes', rota: '/contas-a-pagar?aba=recorrentes', de: abas },
  { id: 'carteira', rota: '/investimentos' },
  { id: 'mes-a-mes', rota: '/investimentos?aba=fii', de: (p) => p.getByRole('heading', { name: 'Mês a mês' }) },
  { id: 'reservas', rota: '/investimentos?aba=reservas', viewport: 1100, largura: 848, de: abas },
  { id: 'credenciais', rota: '/configuracoes' },
  { id: 'modulos', rota: '/configuracoes?aba=modulos' },
  { id: 'aparencia', rota: '/configuracoes?aba=aparencia' },
  { id: 'seguranca', rota: '/configuracoes?aba=seguranca' },
  { id: 'acesso-pelo-celular', rota: '/configuracoes?aba=seguranca', de: (p) => p.getByRole('heading', { name: 'Acesso pelo celular' }) },
  { id: 'erros-do-sistema', rota: '/configuracoes?aba=erros' }
];

const ativo = !!process.env['PRINTS_MANUAL'];
const LARGURA = 760;
// Janela bem alta: tudo cabe sem rolar, e o print não usa `fullPage` (que redimensiona a janela e apaga os gráficos).
const ALTURA_DA_JANELA = 2400;
const pastaImagens = join(__dirname, '..', '..', 'public', 'imagens', 'manual');
const arquivoMedidas = join(__dirname, '..', '..', 'src', 'app', 'paginas', 'manual', 'prints-manual.ts');
const medidas: Record<string, { largura: number; altura: number }> = {};

test.skip(!ativo, 'só com `npm run prints:manual`');
test.skip(({ isMobile }) => isMobile, 'só o computador, em escala 1');
test.describe.configure({ mode: 'serial' });
// 1012 = 252 (menu) + 760 (a coluna do Manual): o trecho sai inteiro, sem cortar a tela no meio.
test.use({ viewport: { width: 1012, height: ALTURA_DA_JANELA }, deviceScaleFactor: 1 });

/** PNG do trecho. Página: da posição de `de` (ou do topo) na largura da área de conteúdo, com 16 px de respiro. */
async function recortar(p: Page, print: Print): Promise<Buffer> {
  const altura = print.altura ?? 480;
  const caixa = await p.evaluate(async ({ janela }) => {
    const ret = (r: DOMRect) => ({ x: r.x + scrollX, y: r.y + scrollY, width: r.width, height: r.height });
    if (janela) return ret(document.querySelector('.modal')!.getBoundingClientRect());
    const pagina = document.querySelector('main')!;
    const c = getComputedStyle(pagina);
    const r = ret(pagina.getBoundingClientRect());
    const esq = parseFloat(c.paddingLeft) - 16, dir = parseFloat(c.paddingRight) - 16;
    const cima = parseFloat(c.paddingTop) - 16;
    return { x: r.x + esq, y: r.y + cima, width: r.width - esq - dir, height: r.height - cima };
  }, { janela: !!print.janela });
  let y = caixa.y;
  let x = caixa.x;
  let largura = caixa.width;
  if (print.de) {
    const ponto = await print.de(p).evaluate((e) => { const r = e.getBoundingClientRect(); return { x: r.x + scrollX, y: r.y + scrollY }; });
    y = ponto.y - 8;
  }
  if (print.inteira) { x = 0; y = 0; largura = print.viewport!; }
  const clip = { x: Math.max(0, Math.round(x)), y: Math.max(0, Math.round(y)), width: Math.round(Math.min(print.largura ?? LARGURA, largura)),
    height: Math.round(Math.min(altura, print.inteira ? altura : print.de ? caixa.y + caixa.height - y : caixa.height)) };
  return p.screenshot({ clip, animations: 'disabled', caret: 'hide' });
}

/** Converte o PNG em WebP no próprio Chromium (ADR 0071: sem ferramenta nova), numa aba sem a CSP do site. */
async function paraWebp(p: Page, png: Buffer): Promise<Buffer> {
  const aba = await p.context().newPage();
  try {
    const base64 = await aba.evaluate(async (origem) => {
      const img = new Image();
      img.src = `data:image/png;base64,${origem}`;
      await img.decode();
      const tela = document.createElement('canvas');
      tela.width = img.width;
      tela.height = img.height;
      tela.getContext('2d')!.drawImage(img, 0, 0);
      const blob = await new Promise<Blob | null>((ok) => tela.toBlob(ok, 'image/webp', 0.8));
      if (!blob) throw new Error('o navegador não gerou WebP');
      const bytes = new Uint8Array(await blob.arrayBuffer());
      let texto = '';
      for (let i = 0; i < bytes.length; i += 0x8000) texto += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
      return btoa(texto);
    }, png.toString('base64'));
    return Buffer.from(base64, 'base64');
  } finally {
    await aba.close();
  }
}

for (const print of PRINTS) {
  test(`print do Manual: ${print.id}`, async ({ page }) => {
    mkdirSync(pastaImagens, { recursive: true });
    // O "hoje" do site é sempre o dia fixo, mas o relógio anda: o clock do Playwright congela o Date.now e os
    // gráficos (Chart.js anima pelo Date.now) ou os timers, e o print saía com o gráfico vazio.
    await page.addInitScript((fixo) => {
      const Original = Date;
      const desvio = fixo - Original.now();
      class DataFixa extends Original {
        constructor(...args: unknown[]) { super(...(args.length ? (args as [number]) : [Original.now() + desvio])); }
        static override now() { return Original.now() + desvio; }
      }
      window.Date = DataFixa as DateConstructor;
    }, DATA_DOS_PRINTS.getTime());
    for (const tema of ['claro', 'escuro'] as const) {
      await page.emulateMedia({ colorScheme: tema === 'escuro' ? 'dark' : 'light', reducedMotion: 'reduce' });
      await page.setViewportSize({ width: print.viewport ?? 1012, height: ALTURA_DA_JANELA });
      await page.goto(print.rota);
      await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
      await page.waitForLoadState('networkidle');
      await print.preparo?.(page);
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(1500); // os gráficos (Chart.js) terminam de desenhar
      const png = await recortar(page, print);
      const largura = png.readUInt32BE(16);
      const altura = png.readUInt32BE(20);
      if (medidas[print.id]) expect({ largura, altura }, `${print.id}: os dois temas têm o mesmo tamanho`).toEqual(medidas[print.id]);
      medidas[print.id] = { largura, altura };
      writeFileSync(join(pastaImagens, `${print.id}-${tema}.webp`), await paraWebp(page, png));
    }
  });
}

// Um arquivo só com as medidas (o <img> precisa de width e height, para a página não pular ao rolar). Com filtro (-g),
// os ids que não rodaram ficam como estavam.
test.afterAll(() => {
  if (!ativo || Object.keys(medidas).length === 0) return;
  const todas: Record<string, { largura: number; altura: number }> = {};
  try {
    for (const m of readFileSync(arquivoMedidas, 'utf8').matchAll(/'([^']+)': \{ largura: (\d+), altura: (\d+) \}/g)) {
      todas[m[1]] = { largura: Number(m[2]), altura: Number(m[3]) };
    }
  } catch { /* primeira vez */ }
  Object.assign(todas, medidas);
  const linhas = Object.keys(todas).sort().map((id) => `  '${id}': { largura: ${todas[id].largura}, altura: ${todas[id].altura} }`);
  writeFileSync(arquivoMedidas, `// Gerado por \`npm run prints:manual\` (testes/navegador/prints-manual.spec.ts): não edite à mão.\n` +
    `// Largura e altura de cada print do Manual, em px (o <img> as usa para a página não pular).\n` +
    `export const MEDIDAS_PRINTS: Record<string, { largura: number; altura: number }> = {\n${linhas.join(',\n')}\n};\n`);
});
