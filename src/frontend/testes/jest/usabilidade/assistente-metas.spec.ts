import { abrirMetas } from '../apoio/metas';
import { METAS, plano } from '../apoio/plano-metas';

/**
 * Usabilidade do assistente de metas (#90, #149). Shneiderman: o mês primeiro, a meta depois e o plano mês a mês só sob
 * demanda; Progressive Disclosure: plano e "consigo guardar" fechados por padrão; WCAG 2.1.1/4.1.2: controles com nome
 * e estado; Nielsen 10: os termos da conta com ⓘ do glossário; Nielsen 3 e 6: o percentual na própria tela.
 */
describe('Usabilidade: assistente de metas', () => {
  const cartao = (raiz: HTMLElement, nome: string) =>
    Array.from(raiz.querySelectorAll<HTMLElement>('.cartao-meta')).find((c) => c.querySelector('strong')!.textContent === nome)!;

  it('o bloco do mês vem antes das metas e explica os termos no contexto', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    const bloco = t.raiz.querySelector('.assistente-metas')!;
    expect(bloco.compareDocumentPosition(t.raiz.querySelector('.grade-cartoes')!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const termos = Array.from(bloco.querySelectorAll('.termo-botao')).map((b) => b.getAttribute('aria-label'));
    expect(termos).toEqual(['O que é Sobra do mês?', 'O que é Parte da sobra para as metas?', 'O que é Recomendação de quanto guardar?']);
    const comoCalculei = bloco.querySelector('.links-assistente app-termo')!;
    expect(comoCalculei.textContent).toContain('Como calculei');
    expect(comoCalculei.querySelector('button')!.tagName).toBe('BUTTON');
    expect(bloco.querySelector('.links-assistente a')!.getAttribute('href')).toBe('/projecao');
  });

  it('a sobra do mês fica à direita do título, verde quando sobra e vermelha quando falta', async () => {
    const sobra = abrirMetas(METAS, plano());
    await sobra.atualizar();
    const valor = sobra.raiz.querySelector('#titulo-assistente .valor-titulo')!;
    expect(valor.parentElement!.classList).toContain('titulo-com-valor');
    expect(valor.classList).toContain('positivo');
  });

  it('com o mês no vermelho pelo caixa, o valor fica vermelho', async () => {
    const falta = abrirMetas(METAS, plano({}, { faturas: [7000, 2500] }));
    await falta.atualizar();
    const valor = falta.raiz.querySelector('#titulo-assistente .valor-titulo')!;
    expect(valor.classList).toContain('negativo');
    expect(valor.classList).not.toContain('positivo');
  });

  it('o percentual da sobra é um select com rótulo, na própria tela, com o valor atual (100 − margem)', async () => {
    const t = abrirMetas(METAS, plano({ margemPercentual: 15 }));
    await t.atualizar();
    const sel = t.raiz.querySelector<HTMLSelectElement>('select[name=percentual]')!;
    expect(sel.closest('label')!.textContent!.replace(/\s+/g, ' ')).toMatch(/Guardar .* da sobra para as metas/);
    expect(sel.options[sel.selectedIndex].text).toBe('85%');
  });

  it('"Consigo guardar outro valor" começa fechado e abre num botão com aria-expanded e aria-controls', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    const botao = cartao(t.raiz, 'Viagem').querySelector<HTMLButtonElement>('.botao-ajuste')!;
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(cartao(t.raiz, 'Viagem').querySelector('app-campo-valor')).toBeNull();
    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    const regiao = document.getElementById(botao.getAttribute('aria-controls')!)!;
    expect(regiao.querySelector('label')!.textContent).toContain('Quanto consigo guardar este mês');
    expect(regiao.querySelector('app-campo-valor')).not.toBeNull();
  });

  it('#335: plano mês a mês começa aberto, com 5 meses e "Mostrar mais"; o botão fecha (aria-expanded)', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    const botao = cartao(t.raiz, 'Viagem').querySelector<HTMLButtonElement>('.botao-plano')!;
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    const linhasDoPlano = () => cartao(t.raiz, 'Viagem').querySelectorAll('.tabela-plano tbody tr').length;
    const maisMeses = () => [...cartao(t.raiz, 'Viagem').querySelectorAll<HTMLButtonElement>('.mais-meses button')];
    expect(linhasDoPlano()).toBe(5);
    expect(maisMeses().map((b) => b.textContent!.replace(/\s+/g, ' ').trim())).toEqual(['Mostrar mais 5 meses']);
    maisMeses()[0].click();
    await t.atualizar();
    expect(linhasDoPlano()).toBe(10);
    expect(maisMeses().map((b) => b.textContent!.replace(/\s+/g, ' ').trim())).toEqual(['Mostrar mais 2 meses', 'Mostrar menos']);
    maisMeses()[0].click();
    await t.atualizar();
    expect(linhasDoPlano()).toBe(12);
    expect(maisMeses().map((b) => b.textContent!.trim())).toEqual(['Mostrar menos']);
    maisMeses()[0].click();
    await t.atualizar();
    expect(linhasDoPlano()).toBe(5);
    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(cartao(t.raiz, 'Viagem').querySelector('table')).toBeNull();
    botao.click();
    await t.atualizar();
    const cabecalho = Array.from(cartao(t.raiz, 'Viagem').querySelectorAll('thead th')).map((th) => th.textContent!.trim());
    expect(cabecalho).toEqual(['Mês', 'Sobra', 'Faturas', 'Guardar']);
    const linhas = Array.from(cartao(t.raiz, 'Viagem').querySelectorAll('tbody tr'))
      .map((tr) => Array.from(tr.querySelectorAll('th, td')).map((c) => c.textContent!.trim()).join(' | '));
    expect(linhas[1]).toMatch(/^nov\/26 \| R\$\s3\.500,00 \| R\$\s2\.500,00 \| R\$\s[\d.,]+$/);
    expect(document.getElementById(botao.getAttribute('aria-controls')!)).not.toBeNull();
  });

  it('sem 3 meses de extrato, explica por quê e cada meta mostra só a divisão igual', async () => {
    const t = abrirMetas(METAS);
    await t.atualizar();
    expect(t.raiz.querySelector('.assistente-metas')!.textContent).toContain('Preciso de pelo menos 3 meses completos de extrato');
    const viagem = cartao(t.raiz, 'Viagem').textContent!.replace(/\s+/g, ' ');
    expect(viagem).toMatch(/Guardar R\$\s785,71\/mês para chegar lá/);
    expect(viagem).not.toContain('Este mês: guarde');
    expect(t.raiz.querySelector('.botao-plano')).toBeNull();
    expect(t.raiz.querySelector('select[name=percentual]')).toBeNull();
  });

  it('botões de prioridade têm nome e ficam desabilitados nas pontas', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    const botao = (nome: string) => t.raiz.querySelector<HTMLButtonElement>(`[aria-label="${nome}"]`)!;
    expect(botao('Subir a prioridade de Viagem').disabled).toBe(true);
    expect(botao('Descer a prioridade de Viagem').disabled).toBe(false);
    expect(botao('Descer a prioridade de Carro').disabled).toBe(true);
    expect(cartao(t.raiz, 'Emergência').querySelector('.linha-prioridade')!.textContent).toContain('2ª prioridade');
  });
});
