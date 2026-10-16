import { abrirMetas } from '../apoio/metas';
import { METAS, plano } from '../apoio/plano-metas';

/**
 * Usabilidade do assistente de metas (#90). Shneiderman: o mês primeiro, a meta depois e o plano mês a mês só sob
 * demanda; Progressive Disclosure: plano fechado por padrão; WCAG 2.1.1/4.1.2: botões com nome e estado;
 * Nielsen 10: os termos da conta (sobra prevista, margem de segurança) com ⓘ do glossário.
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
    expect(termos).toEqual(['O que é Sobra prevista?', 'O que é Margem de segurança?']);
    expect(bloco.querySelector('a')!.getAttribute('href')).toBe('/projecao');
  });

  it('plano mês a mês começa fechado e abre num botão com aria-expanded', async () => {
    const t = abrirMetas(METAS, plano());
    await t.atualizar();
    const botao = cartao(t.raiz, 'Viagem').querySelector<HTMLButtonElement>('.botao-plano')!;
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(cartao(t.raiz, 'Viagem').querySelector('table')).toBeNull();
    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    const linhas = Array.from(cartao(t.raiz, 'Viagem').querySelectorAll('tbody tr'))
      .map((tr) => Array.from(tr.querySelectorAll('th, td')).map((c) => c.textContent!.trim()).join(' | '));
    expect(linhas[1]).toMatch(/^nov\/26 \| R\$\s3\.590,59 \| R\$\s100,00 \| R\$\s719,48$/);
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
