import { TestBed } from '@angular/core/testing';
import { readFileSync } from 'fs';
import { join } from 'path';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { abrirSelecao } from '../apoio/selecao';

/**
 * Usabilidade da seleção nas tabelas (#133).
 * - Nielsen 1 (Visibility of System Status): linha selecionada destacada; contagem que diz quantas estão fora da tela.
 * - WCAG 2.5.8 (Target Size): a célula inteira é o alvo, ≥ 24×24 px.
 * - WCAG 4.1.2 (Name, Role, Value): nome por linha ("Selecionar: PIX…, 11/10/2026, R$ 10,00") e o estado "misto".
 * - WCAG 1.4.11 (Non-text Contrast): a caixa tem borda própria (não é mais a do navegador), conferida no tema.
 * - A contagem é anunciada quando muda (aria-live).
 */
describe('Usabilidade: seleção nas tabelas', () => {
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  it('cada caixa tem o nome da sua linha (descrição, data e valor) e o "todas" diz que vale para a página', async () => {
    const t = await abrirSelecao();
    expect(t.caixas()[0].getAttribute('aria-label')).toMatch(/^Selecionar: PIX ENVIADO 1, 11\/10\/2026, R\$\s10,00$/);
    expect(t.todas().getAttribute('aria-label')).toBe('Selecionar todas desta página');
  });

  it('com "ocultar valores", o nome da caixa não diz o valor', async () => {
    const t = await abrirSelecao();
    TestBed.inject(PrivacidadeService).alternar();
    await t.assentar();
    expect(t.caixas()[0].getAttribute('aria-label')).toBe('Selecionar: PIX ENVIADO 1, 11/10/2026, valor oculto');
    TestBed.inject(PrivacidadeService).alternar();
  });

  it('a célula inteira é o alvo (label de pelo menos 24×24 px envolvendo a caixa)', async () => {
    const t = await abrirSelecao();
    const alvo = t.caixas()[0].closest('label')!;
    expect(alvo.classList).toContain('celula-marcar');
    expect(parseFloat(getComputedStyle(alvo).minWidth)).toBeGreaterThanOrEqual(24);
    expect(parseFloat(getComputedStyle(alvo).minHeight)).toBeGreaterThanOrEqual(24);
  });

  it('a linha selecionada fica destacada', async () => {
    const t = await abrirSelecao();
    await t.clicar(0);
    const linhas = Array.from(t.elemento.querySelectorAll('tbody tr'));
    expect(linhas[0].classList).toContain('selecionada');
    expect(linhas[1].classList).not.toContain('selecionada');
  });

  it('a contagem fica numa região aria-live que existe antes da primeira seleção', async () => {
    const t = await abrirSelecao();
    const regiao = t.elemento.querySelector('[aria-live="polite"].so-leitor')!;
    expect(regiao.textContent!.trim()).toBe('');
    await t.clicar(0);
    expect(regiao.textContent!.trim()).toBe('1 selecionada');
  });

  it('sem seleção, a barra de ações está fechada e fora do Tab (inert); com seleção, abre', async () => {
    const t = await abrirSelecao();
    expect(t.barra().classList).not.toContain('aberta');
    expect(t.barra().hasAttribute('inert')).toBe(true);
    await t.clicar(0);
    expect(t.barra().classList).toContain('aberta');
    expect(t.barra().hasAttribute('inert')).toBe(false);
  });

  it('caixa de marcar nova em todo o site: 18 px, desenhada pelo site (não a do navegador), e o interruptor fica de fora', () => {
    const caixa = document.createElement('input');
    caixa.type = 'checkbox';
    const interruptor = document.createElement('input');
    interruptor.type = 'checkbox';
    interruptor.setAttribute('role', 'switch');
    document.body.append(caixa, interruptor);
    expect(getComputedStyle(caixa).width).toBe('18px');
    expect(getComputedStyle(caixa).height).toBe('18px');
    expect(getComputedStyle(interruptor).width).not.toBe('18px');
    caixa.remove();
    interruptor.remove();
  });
});
