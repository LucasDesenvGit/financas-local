import { readFileSync } from 'fs';
import { join } from 'path';
import { CONTAS, abrirContas, conta } from '../apoio/contas';

/**
 * Usabilidade do interruptor (#130, ADR 0032).
 * - NN/g, Toggle-Switch Guidelines: interruptor é para efeito imediato; o que depende de "Salvar" fica caixa de
 *   marcar. Rótulo curto, palavra-chave primeiro.
 * - WAI-ARIA APG, Switch: role="switch" no checkbox nativo (Espaço alterna) e rótulo que não muda com o estado.
 * - WCAG 1.4.1 (Use of Color): conta arquivada diz "Arquivada" em texto, não só pelo visual apagado.
 * - WCAG 2.5.8 (Target Size): alvo ≥ 24×24 px. O jsdom não calcula layout, mas aplica a cascata do styles.css.
 */
describe('Usabilidade: interruptor', () => {
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  const interruptor = (raiz: HTMLElement) => raiz.querySelector<HTMLInputElement>('label.interruptor input')!;

  it('"Mostrar arquivadas" é um interruptor (role="switch") com o checkbox nativo e o rótulo clicável', async () => {
    const t = await abrirContas();
    const campo = interruptor(t.raiz);
    expect(campo.type).toBe('checkbox');
    expect(campo.getAttribute('role')).toBe('switch');
    expect(campo.closest('label')!.textContent!.trim()).toBe('Mostrar arquivadas');
  });

  it('o rótulo não muda quando o interruptor liga', async () => {
    const t = await abrirContas();
    const campo = interruptor(t.raiz);
    campo.click();
    await t.atualizar();
    expect(campo.checked).toBe(true);
    expect(campo.closest('label')!.textContent!.trim()).toBe('Mostrar arquivadas');
  });

  it('alvo de pelo menos 24 px de altura, com o rótulo; o trilho tem 40×24 px', async () => {
    const t = await abrirContas();
    const campo = interruptor(t.raiz);
    expect(parseFloat(getComputedStyle(campo.closest('label')!).minHeight)).toBeGreaterThanOrEqual(24);
    expect(getComputedStyle(campo).width).toBe('40px');
    expect(getComputedStyle(campo).height).toBe('24px');
  });

  it('com o interruptor ligado, conta e cartão arquivados mostram o selo "Arquivada" em texto', async () => {
    const contas = [...CONTAS, conta({ id: 9, nome: 'Conta antiga', arquivada: true }),
      conta({ id: 10, nome: 'Cartão antigo', tipo: CONTAS[1].tipo, arquivada: true, limiteCredito: 1000 })];
    const t = await abrirContas('/contas?arquivadas=1', contas);
    const selos = Array.from(t.raiz.querySelectorAll('.etiqueta-arquivada')).map((e) => e.textContent!.trim());
    expect(selos).toEqual(['Arquivada', 'Arquivada']);
    expect(t.raiz.querySelector('.cartao-conta.arquivada .etiqueta-arquivada')).not.toBeNull();
    expect(t.cartao(10).querySelector('.etiqueta-arquivada')).not.toBeNull();
  });

  it('as caixas de marcar de formulário (com "Salvar") continuam caixas de marcar', async () => {
    const t = await abrirContas();
    t.tela.nova();
    await t.atualizar();
    const doFormulario = Array.from(document.querySelectorAll<HTMLInputElement>('form input[type="checkbox"], .modal input[type="checkbox"]'));
    expect(doFormulario.length).toBeGreaterThan(0);
    expect(doFormulario.every((c) => c.getAttribute('role') !== 'switch' && !c.closest('label.interruptor'))).toBe(true);
  });
});
