import { TestBed } from '@angular/core/testing';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { abrirContasAPagar, contaAgendada, sugestao } from '../apoio/contas-agendadas';

/**
 * #205: a sugestão do extrato voltava depois de cadastrada com outro nome ("PIX ENVIADO LIVEPIX LTDA" cadastrada como
 * "Manwha"). A conta a pagar agora guarda a chave da descrição do banco ao cadastrar pela sugestão, e a conta a pagar
 * parecida com outro nome aparece como "parece ser …" com o botão "é a mesma", que grava a ligação.
 * (A regra que esconde a sugestão já ligada ou certamente cadastrada fica na API: SugestaoContaAgendadaService.)
 */
describe('Regressão #205: sugestão do extrato já cadastrada com outro nome', () => {
  const livepix = () => sugestao({
    chave: 'LIVEPIX', chaveExtrato: 'LIVEPIX', descricao: 'PIX ENVIADO LIVEPIX LTDA', valor: 7.99, valorMinimo: 7.99,
    valorMaximo: 7.99, diaHabitual: 16, textoCobranca: 'LIVEPIX'
  });

  it('"Cadastrar" manda a chave da descrição do banco, mesmo que a descrição seja trocada', () => {
    const t = abrirContasAPagar([livepix()]);
    t.clicar(t.botao('Cadastrar PIX ENVIADO LIVEPIX LTDA'));
    t.tela.form!.descricao = 'Manwha';
    t.tela.salvar();
    expect(t.salvos).toHaveLength(1);
    expect(t.salvos[0]).toMatchObject({ descricao: 'Manwha', chaveExtrato: 'LIVEPIX' });
  });

  it('editar uma conta a pagar não manda chave (a ligação que já existe fica na API)', () => {
    const t = abrirContasAPagar([], [contaAgendada({ id: 7, descricao: 'Manwha', valor: 7.99 })]);
    t.tela.editar(t.tela.itens()[0]);
    t.tela.salvar();
    expect(t.salvos).toHaveLength(1);
    expect(t.salvos[0].chaveExtrato ?? null).toBeNull();
  });

  it('conta a pagar parecida mostra "parece ser <nome>" e "é a mesma" grava a ligação e tira a sugestão', () => {
    const t = abrirContasAPagar([livepix(), sugestao({ pareceSerId: 9, pareceSerNome: 'Parcela Casa', chave: 'DANIELA CRISTINA',
      chaveExtrato: 'DANIELA CRISTINA', descricao: 'Transferência enviada|Daniela Cristina' })]);
    expect(t.linhas()).toHaveLength(2);
    expect(t.linhas()[0].textContent).not.toContain('parece ser');
    expect(t.linhas()[1].querySelector('.parece-ser')!.textContent).toContain('parece ser Parcela Casa');

    t.clicar(t.botao('é a mesma: Transferência enviada|Daniela Cristina é Parcela Casa'));
    expect(t.ligadas).toEqual([{ id: 9, chaveExtrato: 'DANIELA CRISTINA' }]);
    expect(t.linhas()).toHaveLength(1);
    expect(t.linhas()[0].textContent).toContain('PIX ENVIADO LIVEPIX LTDA');
  });

  it('o aviso do "é a mesma" tem "Desfazer", que tira a ligação (Nielsen 3)', () => {
    const t = abrirContasAPagar([sugestao({ pareceSerId: 9, pareceSerNome: 'Parcela Casa', chave: 'D', chaveExtrato: 'D',
      descricao: 'TRANSFERENCIA D' })]);
    const avisos = TestBed.inject(AvisosService);
    t.clicar(t.botao('é a mesma: TRANSFERENCIA D é Parcela Casa'));
    const aviso = avisos.avisos().at(-1)!;
    expect(aviso.texto).toContain('Ligada a Parcela Casa');
    expect(aviso.acao!.rotulo).toBe('Desfazer');
    avisos.executar(aviso);
    expect(t.desligadas).toEqual([9]);
  });

  it('"é a mesma" é um botão de verdade, alcançável por teclado (WCAG 2.1.1), com o nome visível no rótulo', () => {
    const t = abrirContasAPagar([livepix(), sugestao({ pareceSerId: 3, pareceSerNome: 'Manwha', chave: 'X', chaveExtrato: 'X',
      descricao: 'PIX ENVIADO OUTRO' })]);
    const botao = t.botao('é a mesma: PIX ENVIADO OUTRO é Manwha');
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('type')).toBe('button');
    expect(botao.textContent!.trim()).toBe('é a mesma');
    expect(botao.getAttribute('tabindex')).not.toBe('-1');
  });

  it('sem conta a pagar parecida, não há "é a mesma"', () => {
    const { raiz } = abrirContasAPagar([livepix()]);
    expect(raiz.querySelector('section.sugestoes .parece-ser')).toBeNull();
    expect(Array.from(raiz.querySelectorAll('section.sugestoes button')).some((b) => b.textContent!.includes('é a mesma'))).toBe(false);
  });
});
