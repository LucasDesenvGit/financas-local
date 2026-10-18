import { NivelErro } from '../../src/app/core/modelos';
import { abrirErros, erro } from '../apoio/erros';

/**
 * Usabilidade de Configurações › Erros (#11). Shneiderman: lista primeiro, filtros, pilha só no detalhe
 * (details-on-demand); Nielsen 1: quantas vezes e desde quando; Nielsen 10: o texto diz o que é gravado e o que é
 * removido; WCAG 1.4.1: nível em texto, não só na cor; WCAG 4.1.2: botões com nome e estado.
 */
describe('Usabilidade: erros do sistema', () => {
  it('explica o que é registrado e que dados sensíveis são removidos', async () => {
    const t = await abrirErros();
    const texto = t.raiz.querySelector('.painel-bloco > p')!.textContent!;
    expect(texto).toContain('tokens, senhas, CPF e valores em reais são trocados por [removido]');
    expect(texto).toContain('apagados sozinhos');
  });

  it('filtros com rótulo e situação em botões com aria-pressed', async () => {
    const t = await abrirErros();
    const rotulos = Array.from(t.raiz.querySelectorAll('.filtros-erros label')).map((l) => l.firstChild!.textContent!.trim());
    expect(rotulos).toEqual(['Origem', 'Nível', 'De', 'Até']);
    const situacao = Array.from(t.raiz.querySelectorAll('[role=group][aria-label="Situação"] button'))
      .map((b) => `${b.textContent!.trim()}:${b.getAttribute('aria-pressed')}`);
    expect(situacao).toEqual(['Em aberto:true', 'Resolvidos:false', 'Todos:false']);
  });

  it('linha mostra quantas vezes e desde quando, e o nível em texto', async () => {
    const t = await abrirErros('/configuracoes?aba=erros', [erro(), erro({ id: 2, nivel: NivelErro.Aviso, ocorrencias: 1 })]);
    const linhas = Array.from(t.raiz.querySelectorAll('tbody tr')).map((tr) => tr.textContent!.replace(/\s+/g, ' '));
    expect(linhas[0]).toMatch(/3 vezes desde 16\/10 \d\d:\d\d/);
    expect(linhas[0]).toContain('Erro');
    expect(linhas[1]).toContain('Aviso');
    expect(linhas[1]).not.toContain('vezes desde');
  });

  it('pilha só ao abrir o detalhe, num botão com aria-expanded e aria-controls', async () => {
    const t = await abrirErros();
    expect(t.detalhes).toEqual([]);
    const botao = t.raiz.querySelector<HTMLButtonElement>('tbody button.link')!;
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    botao.click();
    await t.assentar();
    expect(t.detalhes).toEqual([1]);
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    const painel = document.getElementById(botao.getAttribute('aria-controls')!)!;
    expect(painel.querySelector('pre')!.textContent).toContain('System.FormatException');
    botao.click();
    await t.assentar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(t.raiz.querySelector('pre')).toBeNull();
  });

  it('lista vazia diz por quê (filtro ou nada em aberto)', async () => {
    const vazia = await abrirErros('/configuracoes?aba=erros', []);
    expect(vazia.raiz.querySelector('td.vazio')!.textContent).toBe('Nenhum erro em aberto.');
  });

  it('filtrado e vazio sugere o filtro, com "Limpar filtros"', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&origem=API', []);
    expect(t.raiz.querySelector('td.vazio')!.textContent).toBe('Nenhum erro neste filtro.');
    expect(Array.from(t.raiz.querySelectorAll('button')).some((b) => b.textContent!.trim() === 'Limpar filtros')).toBe(true);
  });
});
