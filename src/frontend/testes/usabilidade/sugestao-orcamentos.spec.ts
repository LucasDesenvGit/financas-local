import { TestBed } from '@angular/core/testing';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { abrirSugestao, sugestao } from '../apoio/sugestao-orcamentos';

/**
 * Usabilidade da sugestão de orçamentos (#39). Nielsen 3 (nada é gravado sem "Aplicar"; categoria com orçamento
 * não é sobrescrita sem marcar), Nielsen 1 (total e distribuição atualizados), Nielsen 6 (o motivo de cada valor ao
 * lado dele), WCAG 4.1.2 e 1.3.1 (caixas e campos com nome; coluna de seleção com cabeçalho), WCAG 1.1.1 (a barra
 * de distribuição tem o mesmo conteúdo em texto).
 */
describe('Usabilidade: sugestão de orçamentos', () => {
  afterEach(() => document.documentElement.classList.remove('valores-ocultos'));

  it('cada caixa e cada campo dizem a que categoria se referem', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(t.caixa('Moradia').type).toBe('checkbox');
    expect(t.campo('Moradia')!.type).toBe('number');
    const cabecalhos = Array.from(t.raiz.querySelectorAll('thead th')).map((th) => th.textContent!.trim());
    expect(cabecalhos).toEqual(['Aplicar', 'Categoria', 'Sugerido', 'Atual', 'Mediana 6m', 'Último mês', 'Por quê']);
  });

  it('o motivo de cada valor aparece na linha', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    const linha = t.caixa('Moradia').closest('tr')!;
    expect(linha.querySelector('.motivos')!.textContent).toBe('mediana de 6 meses; +5% de folga (essencial)');
  });

  it('categoria sem dados suficientes não pode ser aplicada e explica por quê', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(t.caixa('Viagens').disabled).toBe(true);
    expect(t.campo('Viagens')).toBeNull();
    expect(t.caixa('Viagens').closest('tr')!.textContent).toContain('sem dados suficientes');
  });

  it('o botão diz quantas serão aplicadas e fica desabilitado sem nenhuma marcada', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(t.botaoAplicar().textContent!.trim()).toBe('Aplicar selecionados (2)');
    await t.marcar('Moradia');
    await t.marcar('Lazer');
    expect(t.botaoAplicar().disabled).toBe(true);
  });

  it('a distribuição da renda está também em texto (não só nas cores da barra)', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    const figura = t.raiz.querySelector('figure.distribuicao')!;
    expect(figura.getAttribute('aria-labelledby')).toBe('distribuicao-texto');
    expect(figura.querySelector('.barra-distribuicao')!.getAttribute('aria-hidden')).toBe('true');
    expect(t.raiz.querySelector('#distribuicao-texto')!.textContent).toMatch(/Essenciais \d+%.*\(até 50%\)/s);
  });

  it('o total do mês é anunciado (role="status") ao mudar', async () => {
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(t.raiz.querySelector('.rodape-sugestao p')!.getAttribute('role')).toBe('status');
  });

  it('cartão acima do limite de atenção fica destacado com o percentual', async () => {
    const t = abrirSugestao(sugestao({ usoLimiteCartoes: 77, nivelCartao: 'alerta',
      cartoes: [{ contaId: 2, nome: 'Cartão Roxo', limite: 2000, faturaMedia: 1540, percentual: 77 }] }));
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    const linha = t.raiz.querySelector('.linha-cartoes')!;
    expect(linha.classList).toContain('negativo');
    expect(linha.textContent!.replace(/\s+/g, ' ')).toContain('77% do limite em uso (Cartão Roxo 77%) · acima de 70%');
  });

  it('com "Ocultar valores", nenhum valor em reais aparece e os campos ficam borrados até receber foco', async () => {
    localStorage.setItem('financas.ocultarValores', '1');
    const t = abrirSugestao();
    await t.fixture.whenStable();
    t.fixture.detectChanges();
    expect(TestBed.inject(PrivacidadeService).oculto()).toBe(true);
    expect(t.raiz.textContent).not.toMatch(/R\$\s?\d/);
    expect(t.campo('Moradia')!.classList).toContain('campo-oculto');
  });
});
