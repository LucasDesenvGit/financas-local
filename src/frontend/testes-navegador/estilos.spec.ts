import { Locator, Page } from '@playwright/test';
import { expect, test } from './base';

/**
 * Estilo calculado de verdade (#204): o que os testes do Jest só conseguiam adivinhar lendo o styles.css (o jsdom não
 * calcula var(), grid, subgrid, a ordem da cascata nem "reduzir movimento"). Aqui o navegador aplica o CSS do build de
 * produção e o teste mede o efeito na tela: alturas, colunas, bordas, animações, tamanho de alvo.
 * Vieram de: contas-cards-alinhados, painel-cards-do-mes, tabela-rolagem-celular, texto-vazio-centralizado,
 * divisorias, lista-suspensa, animacoes e menu-lateral (parte de CSS).
 */

type Caixa = { x: number; y: number; width: number; height: number };
const caixa = async (l: Locator): Promise<Caixa> => (await l.boundingBox())!;
const estilo = (l: Locator, prop: string) => l.evaluate((el, p) => getComputedStyle(el).getPropertyValue(p), prop);
const borda = async (l: Locator, lado: 'top' | 'bottom' | 'left') =>
  ({ largura: await estilo(l, `border-${lado}-width`), tipo: await estilo(l, `border-${lado}-style`) });
const temLinha = async (l: Locator, lado: 'top' | 'bottom' | 'left') => {
  const b = await borda(l, lado);
  return b.largura === '1px' && b.tipo === 'solid';
};

/** Põe HTML dentro da área da página (main.pagina), onde valem as regras do site, e devolve o primeiro elemento. */
let injecoes = 0;
async function injetar(page: Page, html: string, onde = 'main.pagina'): Promise<Locator> {
  const id = String(++injecoes);
  await page.evaluate(([conteudo, seletor, marca]) => {
    const div = document.createElement('div');
    div.dataset['injetado'] = marca;
    div.innerHTML = conteudo;
    document.querySelector(seletor)!.appendChild(div);
  }, [html, onde, id] as const);
  return page.locator(`[data-injetado="${id}"] > *`).first();
}

async function abrir(page: Page, rota: string) {
  await page.goto(rota);
  await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
  await page.waitForLoadState('networkidle');
}

test.describe('Contas e cartões: cards do mesmo tamanho e alinhados', () => {
  test('cartões de crédito lado a lado têm a mesma altura (a grade estica todos)', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular os cartões empilham');
    await abrir(page, '/contas');
    const cartoes = page.locator('.grade-cartoes-credito .cartao-credito-bloco');
    expect(await cartoes.count()).toBeGreaterThanOrEqual(2);
    const [a, b] = [await caixa(cartoes.nth(0)), await caixa(cartoes.nth(1))];
    expect(Math.abs(a.y - b.y), 'lado a lado').toBeLessThan(2);
    expect(Math.abs(a.height - b.height), 'mesma altura').toBeLessThan(2);
  });

  test('contas lado a lado: o rodapé de cada card fica na mesma altura (subgrid)', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular as contas empilham');
    await abrir(page, '/contas');
    const cards = page.locator('.cartao-conta');
    expect(await cards.count()).toBeGreaterThanOrEqual(2);
    const [a, b] = [await caixa(cards.nth(0)), await caixa(cards.nth(1))];
    expect(Math.abs(a.y - b.y), 'lado a lado').toBeLessThan(2);
    expect(Math.abs(a.height - b.height), 'mesma altura').toBeLessThan(2);
    const [ra, rb] = [await caixa(cards.nth(0).locator('footer')), await caixa(cards.nth(1).locator('footer'))];
    expect(Math.abs(ra.y - rb.y), 'rodapés alinhados').toBeLessThan(2);
  });
});

test.describe('Painel: cards do mês', () => {
  test('Receitas, Despesas e Resultado em três colunas iguais, com a mesma altura; "Faturas do mês" na linha de baixo, inteira', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular os cards empilham');
    await abrir(page, '/painel');
    const linha = page.locator('.kpis-mes');
    const kpis = linha.locator(':scope > .kpi:not(.kpi-faturas)');
    expect(await kpis.count()).toBe(3);
    const c = await Promise.all([0, 1, 2].map((i) => caixa(kpis.nth(i))));
    for (const k of c.slice(1)) {
      expect(Math.abs(k.y - c[0].y), 'mesma linha').toBeLessThan(2);
      expect(Math.abs(k.width - c[0].width), 'mesma largura').toBeLessThan(2);
      expect(Math.abs(k.height - c[0].height), 'mesma altura').toBeLessThan(2);
    }
    const faturas = await caixa(linha.locator('.kpi-faturas'));
    const total = await caixa(linha);
    expect(faturas.y, 'linha própria').toBeGreaterThan(c[0].y + c[0].height - 1);
    expect(Math.abs(faturas.width - total.width), 'ocupa a linha toda').toBeLessThan(2);
  });

  test('o valor fica logo abaixo do rótulo no card do mês (não no meio do card)', async ({ page }) => {
    await abrir(page, '/painel');
    const kpi = page.locator('.kpis-mes > .kpi').first();
    const rotulo = await caixa(kpi.locator('span').first());
    const valor = await caixa(kpi.locator('strong').first());
    expect(valor.y - (rotulo.y + rotulo.height)).toBeLessThan(16);
  });

  test('os cartões de "Faturas do mês" dividem a largura toda, sem sobra à direita', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular os cartões empilham');
    await abrir(page, '/painel');
    const lista = page.locator('.lista-faturas');
    const itens = lista.locator(':scope > li');
    expect(await itens.count()).toBe(2);
    const [l, ultimo] = [await caixa(lista), await caixa(itens.last())];
    expect(Math.abs(l.x + l.width - (ultimo.x + ultimo.width))).toBeLessThan(3);
  });

  test('as outras linhas de cards não ficam presas em três colunas', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular é uma coluna');
    await abrir(page, '/painel');
    const colunas = (await estilo(page.locator('.kpis:not(.kpis-mes)').first(), 'grid-template-columns')).trim().split(/\s+/);
    expect(colunas.length).not.toBe(3);
  });
});

test.describe('Divisórias', () => {
  test('Painel: linha abaixo do cabeçalho, entre "Situação hoje" e "Mês de …" e abaixo do título de cada bloco', async ({ page }) => {
    await abrir(page, '/painel');
    expect(await temLinha(page.locator('.cabecalho-pagina'), 'bottom')).toBe(true);
    const secoes = page.locator('.secao-painel');
    expect(await temLinha(secoes.nth(0), 'top'), 'nada acima da primeira').toBe(false);
    expect(await temLinha(secoes.nth(1), 'top')).toBe(true);
    const titulos = page.locator('.painel-bloco > .bloco-titulo');
    expect(await titulos.count()).toBeGreaterThan(0);
    for (const t of await titulos.all()) expect(await temLinha(t, 'bottom')).toBe(true);
  });

  test('"Faturas do mês": linha abaixo do total e linha vertical entre os cartões lado a lado', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular os cartões empilham, com a linha em cima');
    await abrir(page, '/painel');
    const lista = page.locator('.lista-faturas');
    expect(await temLinha(lista, 'top')).toBe(true);
    expect(await temLinha(lista.locator('li').nth(0), 'left')).toBe(false);
    expect(await temLinha(lista.locator('li').nth(1), 'left')).toBe(true);
  });

  test('Contas e cartões: linha abaixo do cabeçalho e entre "Contas" e "Cartões de crédito"', async ({ page }) => {
    await abrir(page, '/contas');
    expect(await temLinha(page.locator('.cabecalho-pagina'), 'bottom')).toBe(true);
    const secoes = page.locator('.secao-contas');
    expect(await temLinha(secoes.nth(0), 'top')).toBe(false);
    expect(await temLinha(secoes.nth(1), 'top')).toBe(true);
  });

  test('o h2 que abre um bloco ganha a linha, o do meio não; o .cabecalho-bloco também', async ({ page }) => {
    await abrir(page, '/relatorios');
    const bloco = await injetar(page, '<section class="painel-bloco"><h2>Fluxo de caixa mensal</h2><p>conteúdo</p><h2>outro título</h2></section>');
    expect(await temLinha(bloco.locator('h2').nth(0), 'bottom')).toBe(true);
    expect(await temLinha(bloco.locator('h2').nth(1), 'bottom')).toBe(false);
    const comCabecalho = await injetar(page, '<section class="painel-bloco"><div class="cabecalho-bloco"><h2>Posições</h2></div></section>');
    expect(await temLinha(comCabecalho.locator('.cabecalho-bloco').last(), 'bottom')).toBe(true);
  });
});

test.describe('Texto e listas vazias', () => {
  test('parágrafo comum tem largura máxima de leitura (78ch)', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular a tela é mais estreita que 78ch');
    await abrir(page, '/relatorios');
    const p = await injetar(page, `<section class="painel-bloco"><p>${'palavra '.repeat(200)}</p></section>`);
    const [bloco, paragrafo] = [await caixa(p), await caixa(p.locator('p'))];
    expect(paragrafo.width).toBeLessThan(bloco.width - 40);
    const ch = await p.locator('p').evaluate((el) => {
      const medida = document.createElement('span');
      medida.textContent = '0'.repeat(78);
      medida.style.cssText = 'position:absolute;visibility:hidden;white-space:nowrap';
      el.appendChild(medida);
      const largura = medida.getBoundingClientRect().width;
      medida.remove();
      return largura;
    });
    expect(Math.abs(paragrafo.width - ch)).toBeLessThan(4);
  });

  test('Metas sem metas: o aviso ocupa a linha toda da grade e fica centralizado', async ({ page }) => {
    await abrir(page, '/metas');
    const aviso = page.locator('.grade-cartoes > .vazio').first();
    await expect(aviso).toBeVisible();
    const [grade, a] = [await caixa(page.locator('.grade-cartoes').first()), await caixa(aviso)];
    expect(Math.abs((a.x + a.width / 2) - (grade.x + grade.width / 2)), 'centralizado na grade').toBeLessThan(3);
    expect(await estilo(aviso, 'text-align')).toBe('center');
  });

  test('o aviso de vazio centraliza a caixa (margens iguais), não só o texto', async ({ page }) => {
    await abrir(page, '/emprestimos');
    const p = await injetar(page, '<section class="painel-bloco"><p class="vazio">Nenhum empréstimo encontrado.</p></section>');
    const [bloco, aviso] = [await caixa(p), await caixa(p.locator('.vazio'))];
    const esquerda = aviso.x - bloco.x;
    const direita = bloco.x + bloco.width - (aviso.x + aviso.width);
    expect(Math.abs(esquerda - direita)).toBeLessThan(3);
    expect(await estilo(p.locator('.vazio'), 'text-align')).toBe('center');
  });

  test('a tabela com rolagem segura o texto só de leitor e rola na horizontal, sem alargar a página', async ({ page }) => {
    await abrir(page, '/transacoes');
    const rolagem = page.locator('.tabela-rolagem').first();
    expect(await estilo(rolagem, 'position')).toBe('relative');
    expect(await estilo(rolagem, 'overflow-x')).toBe('auto');
    const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(sobra).toBeLessThanOrEqual(1);
  });
});

test.describe('Lista suspensa', () => {
  test('select comum: sem a aparência do sistema, com a seta do site e espaço para ela, também com foco', async ({ page }) => {
    await abrir(page, '/transacoes');
    const select = page.locator('select:not([multiple])').first();
    expect(await estilo(select, 'appearance')).toBe('none');
    expect(await estilo(select, 'background-image')).toContain('linear-gradient');
    expect(parseFloat(await estilo(select, 'padding-right'))).toBeGreaterThanOrEqual(30);
    expect(await estilo(select, 'white-space')).toBe('nowrap');
    await select.focus();
    expect(await estilo(select, 'background-image'), 'a seta continua no foco').toContain('linear-gradient');
  });

  test('select múltiplo fica de fora (sem a seta)', async ({ page }) => {
    await abrir(page, '/transacoes');
    const multiplo = await injetar(page, '<select multiple><option>a</option><option>b</option></select>');
    expect(await estilo(multiplo, 'background-image')).not.toContain('linear-gradient');
  });
});

test.describe('Animações', () => {
  const animacao = (l: Locator) => estilo(l, 'animation-name');

  test('o que aparece na página entra com o fade "surgir"; texto, tabelas e linhas não', async ({ page }) => {
    await abrir(page, '/relatorios');
    for (const classe of ['botao', 'botao-icone', 'etiqueta', 'aviso-inline', 'painel-bloco', 'kpi', 'cartao-conta',
      'indicador', 'vazio', 'chip']) {
      const el = await injetar(page, `<div class="${classe}">x</div>`);
      expect(await animacao(el), `.${classe}`).toBe('surgir');
      await page.evaluate(() => document.querySelectorAll('[data-injetado]').forEach((e) => e.remove()));
    }
    for (const tag of ['section', 'article', 'li', 'figure']) {
      const el = await injetar(page, `<${tag}>x</${tag}>`);
      expect(await animacao(el), `<${tag}>`).toBe('surgir');
      await page.evaluate(() => document.querySelectorAll('[data-injetado]').forEach((e) => e.remove()));
    }
    for (const html of ['<p>x</p>', '<span>x</span>', '<table><tr><td>x</td></tr></table>']) {
      const el = await injetar(page, html);
      expect(await animacao(el), html).toBe('none');
      await page.evaluate(() => document.querySelectorAll('[data-injetado]').forEach((e) => e.remove()));
    }
    const tabela = await injetar(page, '<table><tbody><tr><td>x</td></tr></tbody></table>');
    expect(await animacao(tabela.locator('tr'))).toBe('none');
    expect(await animacao(tabela.locator('td'))).toBe('none');
  });

  test('menus que abrem por cima também entram com o fade', async ({ page }) => {
    await abrir(page, '/relatorios');
    for (const classe of ['menu-flutuante', 'seletor-multiplo-painel']) {
      const el = await injetar(page, `<div class="${classe}">x</div>`, 'body');
      expect(await animacao(el), `.${classe}`).toBe('surgir');
      await page.evaluate(() => document.querySelectorAll('[data-injetado]').forEach((e) => e.remove()));
    }
  });

  test('região que abre e fecha anima a altura e vence o fade; a saída (.sair) também vence', async ({ page }) => {
    await abrir(page, '/relatorios');
    expect(await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('interpolate-size'))).toBe('allow-keywords');
    for (const [html, esperada] of [
      ['<section class="painel-bloco barra-edicao expandir">x</section>', 'expandir'],
      ['<section class="painel-bloco recolher">x</section>', 'recolher'],
      ['<div class="botao sair">x</div>', 'sumir']
    ]) {
      const el = await injetar(page, html);
      expect(await animacao(el), html).toBe(esperada);
      await page.evaluate(() => document.querySelectorAll('[data-injetado]').forEach((e) => e.remove()));
    }
  });

  test('a tela aberta pelo roteador entra com animação; o router-outlet em si não', async ({ page }) => {
    await abrir(page, '/painel');
    expect(await animacao(page.locator('main.pagina > :not(router-outlet)').first())).toBe('entrar-tela');
    expect(await animacao(page.locator('main.pagina > router-outlet'))).toBe('none');
  });

  test('"reduzir movimento" desliga animações e transições', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await abrir(page, '/painel');
    for (const seletor of ['main.pagina > :not(router-outlet)', '.painel-bloco', '.kpi', 'header button']) {
      const el = page.locator(seletor).first();
      expect(await animacao(el), seletor).toBe('none');
      expect(await estilo(el, 'transition-duration'), seletor).toMatch(/^0s(, 0s)*$/);
    }
  });
});

test.describe('Menu lateral e barra de cima', () => {
  test('o ☰ tem alvo de pelo menos 24×24 px', async ({ page }) => {
    await abrir(page, '/painel');
    const c = await caixa(page.getByRole('button', { name: 'Menu de navegação' }));
    expect(c.width).toBeGreaterThanOrEqual(24);
    expect(c.height).toBeGreaterThanOrEqual(24);
  });

  test('a marca ocupa a largura toda do topo do menu (área clicável inteira) e nada fica sublinhado', async ({ page, isMobile }) => {
    test.skip(isMobile, 'no celular o menu é a gaveta fechada');
    await abrir(page, '/painel');
    const marca = page.getByRole('link', { name: 'Finanças, ir para o Painel' }).first();
    expect(await estilo(marca, 'flex-grow')).toBe('1');
    expect(await estilo(marca, 'text-decoration-line')).toBe('none');
    await page.getByRole('button', { name: 'Menu de navegação' }).click();
    const titulo = page.locator('header').getByRole('link', { name: 'Finanças, ir para o Painel' });
    await expect(titulo).toBeVisible();
    expect(await estilo(titulo, 'text-decoration-line')).toBe('none');
  });
});
