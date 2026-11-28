import { MANUAL } from '../../../src/app/paginas/manual/manual-conteudo';
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
    // Pela posição do capítulo no MANUAL: capítulo novo no meio não quebra o teste.
    const categorizar = MANUAL.findIndex((c) => c.id === 'categorizar');
    expect(capitulos[categorizar].getAttribute('href')).toBe('/manual/categorizar');
  });

  it('busca com rótulo; acha a seção pelo texto, sem acento nem maiúscula fazendo diferença', async () => {
    const t = await abrirManual('/manual?busca=MELHOR dia de COMPRA');
    expect(t.raiz.querySelector('label[for="busca-manual"]')!.textContent).toContain('Buscar no manual');
    expect(t.raiz.querySelector('[role=status]')!.textContent).toBe('1 seção encontrada');
    const link = t.raiz.querySelector('.resultados-busca a')!;
    expect(link.textContent!.trim()).toBe('Contas e cartões › Fatura do mês × dívida total');
    expect(link.getAttribute('href')).toBe('/manual/contas?busca=MELHOR%20dia%20de%20COMPRA#fatura-e-divida');
  });

  it('busca sem resultado diz o que fazer', async () => {
    const t = await abrirManual('/manual?busca=xyzzyx');
    expect(t.raiz.querySelector('.vazio')!.textContent).toContain('Tente outra palavra');
  });

  it('o capítulo tem trilha, índice das seções e cada seção no formato do manual', async () => {
    const t = await abrirManual('/manual/categorizar');
    expect(t.raiz.querySelector('.trilha [aria-current="page"]')!.textContent).toBe('Categorizar suas transações');
    const numero = MANUAL.findIndex((c) => c.id === 'categorizar') + 1;
    expect(t.raiz.querySelector('h1')!.textContent!.trim()).toBe(`${numero}. Categorizar suas transações`);
    const secao = t.raiz.querySelector<HTMLElement>('#modo-edicao')!;
    expect(secao.querySelector('h2')!.textContent).toBe('Várias de uma vez: “Editar categorias”');
    expect(secao.querySelector('.para-que')!.textContent!.length).toBeGreaterThan(20);
    expect(secao.querySelectorAll('ul.topicos li').length).toBeGreaterThanOrEqual(1);
    expect(secao.querySelector('ol.passos')).toBeNull();   // só quando a ordem importa (#817)
    // "Mais detalhes": nativo (abre com Enter e Espaço, WCAG 2.1.1), fechado, com o resto do texto.
    const detalhes = secao.querySelector<HTMLDetailsElement>('details.mais-detalhes')!;
    expect(detalhes.querySelector('summary')!.textContent).toBe('Mais detalhes');
    expect(detalhes.open).toBe(false);
    expect(detalhes.querySelectorAll('li').length).toBeGreaterThanOrEqual(5);
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

  it('a seção com print mostra a imagem do tema aberto, com alt, tamanho e carga preguiçosa', async () => {
    const t = await abrirManual('/manual/relatorios');
    const img = t.raiz.querySelector<HTMLImageElement>('#painel img.print-manual')!;
    const secao = MANUAL.find((c) => c.id === 'relatorios')!.secoes.find((s) => s.id === 'painel')!;
    expect(img.getAttribute('src')).toMatch(/^\/imagens\/manual\/painel-(claro|escuro)\.webp$/);
    expect(img.getAttribute('alt')).toBe(secao.print!.alt);
    expect(Number(img.getAttribute('width'))).toBeGreaterThan(0);
    expect(Number(img.getAttribute('height'))).toBeGreaterThan(0);
    expect(img.getAttribute('loading')).toBe('lazy');
    // Logo depois da linha "para que serve" e antes dos tópicos.
    expect(img.previousElementSibling!.classList).toContain('para-que');
    expect(img.nextElementSibling!.classList).toContain('topicos');
  });

  it('a lista numerada aparece só no passo a passo de verdade, sem tópico solto antes dela', async () => {
    const t = await abrirManual('/manual/primeiros-passos');
    const passos = t.raiz.querySelector<HTMLElement>('#por-onde-comecar ol.passos')!;
    expect(passos.querySelectorAll('li').length).toBe(6);
    expect(t.raiz.querySelector('#por-onde-comecar ul.topicos')).toBeNull();
    expect(t.raiz.querySelectorAll('#conhecendo-a-tela ol.passos').length).toBe(0);
  });

  it('a busca acha o que está só em "Mais detalhes" e leva à seção', async () => {
    const t = await abrirManual('/manual?busca=condomínio');
    const link = t.raiz.querySelector('.resultados-busca a')!;
    expect(link.getAttribute('href')).toBe('/manual/transacoes?busca=condom%C3%ADnio#renomear');
    const secao = MANUAL.find((c) => c.id === 'transacoes')!.secoes.find((s) => s.id === 'renomear')!;
    expect((secao.topicos ?? []).join(' ')).not.toMatch(/condomínio/);
  });

  it('a busca acha a seção pelo texto alternativo do print', async () => {
    const t = await abrirManual('/manual?busca=Pôr do sol Lavanda');
    expect(t.raiz.querySelector('.resultados-busca a')!.getAttribute('href')).toBe('/manual/configuracoes?busca=P%C3%B4r%20do%20sol%20Lavanda#aparencia');
  });

  // #817: a busca leva à seção e, se as palavras só estão em "Mais detalhes", ele já vem aberto.
  it('a busca abre o "Mais detalhes" da seção achada (palavra só lá) e deixa os das outras fechados', async () => {
    const t = await abrirManual('/manual/transacoes?busca=condomínio');
    expect(t.raiz.querySelector<HTMLDetailsElement>('#renomear details')!.open).toBe(true);
    expect(t.raiz.querySelector<HTMLDetailsElement>('#filtros details')!.open).toBe(false);
  });

  it('sem busca, o "Mais detalhes" fica fechado', async () => {
    const semBusca = await abrirManual('/manual/transacoes');
    expect([...semBusca.raiz.querySelectorAll<HTMLDetailsElement>('details.mais-detalhes')].some((d) => d.open)).toBe(false);
  });

  it('com a palavra buscada já à vista, o "Mais detalhes" fica fechado', async () => {
    const t = await abrirManual('/manual/transacoes?busca=Renomear');
    expect(t.raiz.querySelector<HTMLDetailsElement>('#renomear details')!.open).toBe(false);
  });
});
