import { abrirContasAPagar, sugestao } from '../apoio/contas-agendadas';

/**
 * Usabilidade das sugestões de contas a pagar (#63). Nielsen 3 (controle: nada é gravado sozinho e "ignorar" se
 * desfaz), Nielsen 6 (formulário já preenchido), WCAG 4.1.2 e 1.3.1 (ações com nome, coluna de ações com cabeçalho).
 */
describe('Usabilidade: sugestões de contas a pagar', () => {
  it('a seção tem título ligado a ela e explica de onde vem a sugestão', () => {
    const { raiz } = abrirContasAPagar([sugestao()]);
    const secao = raiz.querySelector('section.sugestoes')!;
    expect(raiz.querySelector(`#${secao.getAttribute('aria-labelledby')}`)!.textContent).toBe('Sugestões do extrato');
    expect(secao.textContent).toContain('o vencimento é estimado');
  });

  it('toda coluna tem cabeçalho com nome, inclusive a de ações (lida pelo leitor de tela)', () => {
    const { raiz } = abrirContasAPagar([sugestao()]);
    const cabecalhos = Array.from(raiz.querySelectorAll('section.sugestoes thead th')).map((th) => th.textContent!.trim());
    expect(cabecalhos).toEqual(['Próximo (estimado)', 'Descrição', 'Categoria', 'Valor', 'Ações']);
  });

  it('cada ação diz a que sugestão se refere', () => {
    const { botao } = abrirContasAPagar([sugestao()]);
    expect(botao('Cadastrar BOLETO FIAP').tagName).toBe('BUTTON');
    expect(botao('Ver pagamentos de BOLETO FIAP').tagName).toBe('A');
    expect(botao('Ignorar sugestão BOLETO FIAP').tagName).toBe('BUTTON');
  });

  it('"Cadastrar" só abre o formulário preenchido: nada é gravado sem confirmar', () => {
    const { tela, botao, clicar } = abrirContasAPagar([sugestao()]);
    clicar(botao('Cadastrar BOLETO FIAP'));
    expect(tela.form).not.toBeNull();
    expect(tela.editandoId).toBeNull();
  });

  it('valor que varia aparece como aproximado, com a faixa', () => {
    const { linhas } = abrirContasAPagar([sugestao({ valorVariavel: true, valor: 180, valorMinimo: 140, valorMaximo: 230 })]);
    const valor = linhas()[0].querySelector('td.valor')!.textContent!.replace(/\s+/g, ' ');
    expect(valor).toMatch(/≈ R\$\s180,00/);
    expect(valor).toMatch(/varia de R\$\s140,00 a R\$\s230,00/);
  });

  it('"ignorar" se desfaz: as ignoradas podem voltar para a lista', () => {
    const { raiz, linhas, botao, clicar } = abrirContasAPagar([sugestao()]);
    clicar(botao('Ignorar sugestão BOLETO FIAP'));
    expect(linhas()).toHaveLength(0);
    expect(raiz.querySelector('section.sugestoes .vazio')!.textContent).toContain('Nenhuma sugestão nova');
    const voltar = Array.from(raiz.querySelectorAll('button')).find((b) => b.textContent!.includes('Mostrar de novo'))!;
    clicar(voltar);
    expect(linhas()).toHaveLength(1);
  });
});
