import { MANUAL } from '../../src/app/paginas/manual-conteudo';
import { abrirManual } from '../apoio/manual';

/**
 * Usabilidade do Manual (#113). Nielsen 10 (ajuda fácil de buscar, focada na tarefa, com passos concretos);
 * Shneiderman e Progressive Disclosure (índice → capítulo → passo); Nielsen 6 ("Abrir a tela" leva lá);
 * WCAG 2.1.1 e 4.1.2 (tudo em links e campos com nome; títulos em hierarquia).
 */
describe('Usabilidade: Manual', () => {
  it('o índice mostra os capítulos em ordem, cada um com resumo e link', async () => {
    const t = await abrirManual();
    expect(t.raiz.querySelector('h1')!.textContent!.trim()).toBe('Manual');
    const capitulos = Array.from(t.raiz.querySelectorAll('.grade-capitulos h2 a'));
    expect(capitulos.map((a) => a.textContent!.trim())).toEqual(MANUAL.map((c, i) => `${i + 1}. ${c.titulo}`));
    expect(capitulos[4].getAttribute('href')).toBe('/manual/categorizar');
  });

  it('busca com rótulo; acha a seção pelo texto, sem acento nem maiúscula fazendo diferença', async () => {
    const t = await abrirManual('/manual?busca=MELHOR dia de COMPRA');
    expect(t.raiz.querySelector('label[for="busca-manual"]')!.textContent).toContain('Buscar no manual');
    expect(t.raiz.querySelector('[role=status]')!.textContent).toBe('1 seção encontrada');
    const link = t.raiz.querySelector('.resultados-busca a')!;
    expect(link.textContent!.trim()).toBe('Contas e cartões › Fatura do mês × dívida total');
    expect(link.getAttribute('href')).toBe('/manual/contas#fatura-e-divida');
  });

  it('busca sem resultado diz o que fazer', async () => {
    const t = await abrirManual('/manual?busca=xyzzyx');
    expect(t.raiz.querySelector('.vazio')!.textContent).toContain('Tente outra palavra');
  });

  it('o capítulo tem trilha, índice das seções e cada seção no formato do manual', async () => {
    const t = await abrirManual('/manual/categorizar');
    expect(t.raiz.querySelector('.trilha [aria-current="page"]')!.textContent).toBe('Categorizar suas transações');
    expect(t.raiz.querySelector('h1')!.textContent!.trim()).toBe('5. Categorizar suas transações');
    const secao = t.raiz.querySelector<HTMLElement>('#modo-edicao')!;
    expect(secao.querySelector('h2')!.textContent).toBe('Várias de uma vez: “Editar categorias”');
    expect(secao.querySelector('.para-que')!.textContent!.length).toBeGreaterThan(20);
    expect(Array.from(secao.querySelectorAll('h3')).map((h) => h.textContent)).toEqual(['Passo a passo', 'Erros comuns']);
    expect(secao.querySelectorAll('ol.passos li').length).toBeGreaterThanOrEqual(5);
    const abrir = secao.querySelector<HTMLAnchorElement>('.abrir-tela a')!;
    expect(abrir.textContent!.trim()).toBe('Editar as sem categoria →');
    expect(abrir.getAttribute('href')).toBe('/transacoes?categoria=sem&editar=1');
    expect(t.raiz.querySelector('.indice-secoes a[href="/manual/categorizar#modo-edicao"]')).not.toBeNull();
  });

  it('termos da seção abrem o ⓘ do glossário', async () => {
    const t = await abrirManual('/manual/contas');
    const termos = Array.from(t.raiz.querySelectorAll('#fatura-e-divida .termo-botao')).map((b) => b.getAttribute('aria-label'));
    expect(termos).toEqual(['O que é Fatura do mês?', 'O que é Dívida total do cartão?', 'O que é Melhor dia de compra?']);
  });

  it('dá para ir ao capítulo anterior, ao próximo e voltar ao índice', async () => {
    const t = await abrirManual('/manual/contas');
    const links = Array.from(t.raiz.querySelectorAll('.navegacao-capitulos a')).map((a) => [a.textContent!.trim(), a.getAttribute('href')]);
    expect(links).toEqual([
      ['‹ Conectar seus bancos (Open Finance)', '/manual/conectar-bancos'],
      ['Índice do manual', '/manual'],
      ['Transações: encontrar e entender ›', '/manual/transacoes']
    ]);
  });
});
