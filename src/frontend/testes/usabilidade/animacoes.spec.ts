import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Usabilidade: regiões que abrem e fecham no meio da página animam a altura em vez de aparecer de uma vez.
 * O que o CSS faz (qual animação cada elemento ganha, quem vence, "reduzir movimento") é conferido no navegador, com
 * o estilo calculado de verdade (testes-navegador/estilos.spec.ts, #204). Aqui fica só o que está no template.
 */
describe('Usabilidade: animação das regiões que abrem e fecham', () => {
  it('a barra do modo edição de Transações usa expandir/recolher (não fecha de uma vez)', () => {
    const fonte = readFileSync(join(__dirname, '../../src/app/paginas/transacoes.component.ts'), 'utf8');
    expect(fonte).toMatch(/class="painel-bloco barra-edicao expandir"[^>]*animate\.leave="recolher"/);
  });
});
