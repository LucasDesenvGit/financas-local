import { of } from 'rxjs';
import { TipoConta } from '../../src/app/core/modelos';
import { abrirContas, conta, CONTAS } from '../apoio/contas';

/**
 * #171: a pergunta "você reduziu o limite no app?" aparece no cartão (em texto, não só em cor: WCAG 1.4.1) e na edição
 * (Nielsen 6), com botões de verdade (teclado, WCAG 2.1.1) e o texto da pergunta ligado ao grupo (leitor de tela).
 */
describe('Usabilidade: limite do cartão reduzido no app', () => {
  const gold = conta({ id: 4, nome: 'Gold', tipo: TipoConta.CartaoCredito, saldo: -511.72, limiteCredito: 10150,
    limiteDisponivel: 488.28, banco: 'nubank' });

  async function abrir(cartao = gold) {
    const respostas: [number, boolean | null][] = [];
    const t = await abrirContas('/contas', [...CONTAS, cartao], {}, {
      responderLimiteAjustado: (id: number, ajustado: boolean | null) => { respostas.push([id, ajustado]); return of(undefined); }
    });
    return { ...t, respostas };
  }

  it('o cartão diz em texto que o limite usado é maior que a dívida, com um botão para responder', async () => {
    const t = await abrir();
    const aviso = t.cartao(4).querySelector('.aviso-limite')!;
    expect(aviso.textContent).toContain('Você reduziu o limite no app?');
    const responder = aviso.querySelector<HTMLButtonElement>('button[aria-label="Responder sobre o limite de Gold"]')!;
    expect(responder.tagName).toBe('BUTTON');
    responder.click();
    await t.atualizar();
    expect(t.raiz.ownerDocument.querySelector('#pergunta-limite')).not.toBeNull();
  });

  it('na edição, a pergunta é um grupo rotulado com "Sim" e "Não"; "Sim" guarda a resposta', async () => {
    const t = await abrir();
    t.tela.editar(gold);
    await t.atualizar();
    const grupo = t.raiz.ownerDocument.querySelector<HTMLElement>('.sugestao-limite')!;
    expect(grupo.getAttribute('role')).toBe('group');
    expect(grupo.getAttribute('aria-labelledby')).toBe('pergunta-limite');
    // O ⓘ do "limite usado" explica o termo; as respostas são os dois botões das ações.
    const botoes = Array.from(grupo.querySelectorAll('.acoes-limite button')).map((b) => b.textContent!.trim());
    expect(botoes).toEqual(['Sim, ajustei o limite', 'Não']);
    grupo.querySelector<HTMLButtonElement>('.acoes-limite button')!.click();
    await t.atualizar();
    expect(t.respostas).toEqual([[4, true]]);
    expect(t.raiz.ownerDocument.querySelector('.sugestao-limite')).toBeNull();
    expect(t.raiz.ownerDocument.body.textContent).toContain('Você marcou que reduziu o limite no app');
  });

  it('com "não", o limite é o aprovado e a diferença conta como em uso', async () => {
    const t = await abrir(conta({ ...gold, limiteAjustadoNoApp: false, limiteDiferencaAvaliada: 9150 }));
    expect(t.tela.limiteEfetivo(t.tela.contas().find((c) => c.id === 4)!)).toBe(10150);
    expect(t.cartao(4).textContent).toContain('comprometidos além da dívida');
    expect(t.cartao(4).textContent).toContain('95% de');
  });
});
