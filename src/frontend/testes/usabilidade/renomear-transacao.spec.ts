import { of } from 'rxjs';
import { PaginaTransacoes } from '../../src/app/core/modelos';
import { abrirRenomear } from '../apoio/renomear';
import { abrirTransacoes, paginaVazia } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';

/**
 * Usabilidade de "Renomear" (#123). Nielsen 6 (o nome do banco à vista), 5 (prévia com quantas mudam antes de
 * salvar), 3 (voltar e desfazer); WCAG 4.1.2 (campo com rótulo, opções num grupo com legenda, prévia anunciada) e
 * 2.1.1 (botão ✎ alcançável por teclado, com nome).
 */
describe('Usabilidade: renomear transação', () => {
  it('a janela mostra a descrição do banco, o campo com rótulo e as opções num grupo com legenda', async () => {
    const t = await abrirRenomear();
    expect(t.raiz.querySelector('.do-banco')!.textContent!.trim()).toBe('Descrição do banco: PIX ENVIADO 12345 JOAO');
    const rotulo = Array.from(t.raiz.querySelectorAll('label')).find((l) => l.textContent!.includes('Nova descrição'))!;
    expect(rotulo.querySelector('input[name=nome]')).not.toBeNull();
    expect(t.raiz.querySelector('fieldset legend')!.textContent).toBe('Onde usar este nome');
    expect(t.raiz.querySelectorAll('fieldset input[type=radio]')).toHaveLength(2);
  });

  it('a prévia diz quantas transações mudam, com exemplo, num role=status', async () => {
    const t = await abrirRenomear();
    const previa = t.raiz.querySelector('.previa[role=status]')!;
    expect(previa.textContent!.trim()).toBe('Vai renomear mais 3 transações (ex.: PIX ENVIADO 99887 JOAO) e as próximas.');
  });

  it('sem outras iguais, diz que o nome vale para as próximas', async () => {
    const t = await abrirRenomear(undefined, 0);
    t.tela.escopo = 'outras';
    await t.assentar();
    expect(t.raiz.querySelector('.previa')!.textContent).toContain('Nenhuma outra transação tem essa descrição agora');
  });

  it('cada linha tem o botão ✎ com nome acessível, e o nome do banco embaixo tem o prefixo "do banco:"', async () => {
    const itens = [transacao({ id: 1, descricao: 'Aluguel', descricaoOriginal: 'PIX ENVIADO 123' })];
    const t = await abrirTransacoes('/transacoes', (f): PaginaTransacoes => ({ ...paginaVazia(f, 1), itens }), {
      previaRenomear: () => of({ descricaoOriginal: 'PIX ENVIADO 123', textoSugerido: 'PIX ENVIADO', quantidade: 0, exemplos: [] })
    });
    const botao = t.elemento.querySelector<HTMLButtonElement>('tbody .botao-renomear')!;
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('aria-label')).toBe('Renomear: Aluguel');
    botao.click();
    await t.assentar();
    expect(t.tela.renomeando()?.id).toBe(1);
    expect(t.elemento.querySelector('.descricao-original')!.textContent).toContain('do banco:');
  });
});
