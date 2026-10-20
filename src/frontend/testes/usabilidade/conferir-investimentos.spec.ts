import { TestBed } from '@angular/core/testing';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { abrirConferencia, conferencia, taxa, troco } from '../apoio/conferencia';

/**
 * Usabilidade de "Conferir investimentos" (ADR 0027). Shneiderman (o total antes do detalhe); Nielsen 1 (o que foi
 * encontrado, em uma frase), 2 (palavras do dia a dia), 3 e 5 (nada muda sem confirmar; correção sugerida já marcada,
 * mas desmarcável); Progressive Disclosure (lista longa recolhida); WCAG 2.1.1 e 4.1.2 (controles nativos com nome).
 */
describe('Usabilidade: Conferir investimentos', () => {
  it('a frase do topo diz o que foi encontrado; cada grupo diz o que acontece', async () => {
    const t = await abrirConferencia();
    expect(t.raiz.querySelector('.resumo-conferencia')!.textContent!.trim()).toBe('Encontrado: 1 ativo com o tipo errado, '
      + '1 ativo com cotas diferentes da corretora, 1 troco ou venda contando como rendimento, 1 rendimento contando como investimento.');
    const titulos = Array.from(t.raiz.querySelectorAll('h3')).map((h) => h.textContent!.replace(/ⓘ/g, '').replace(/\s+/g, ' ').trim());
    expect(titulos).toEqual(['FII cadastrado como outro tipo (1)', 'Cotas diferentes da corretora (1)',
      'Troco ou venda contando como rendimento (1)', 'Rendimento contando como investimento (1)']);
    const cotas = t.raiz.querySelector('.linha-cotas p')!.textContent!.replace(/\s+/g, ' ');
    expect(cotas).toContain('GARE11: a corretora informa 224 cotas (posição de 17/10); a carteira tem 152. Faltam 72.');
  });

  it('nada para corrigir: diz que está tudo certo e não deixa clicar em corrigir', async () => {
    const t = await abrirConferencia(conferencia({ tipos: [], cotas: [], trocos: [], rendimentos: [], comprasSemOperacao: [] }));
    expect(t.raiz.querySelector('.resumo-conferencia')!.textContent).toContain('Tudo certo');
    expect(t.botao('Corrigir selecionados').disabled).toBe(true);
  });

  it('#173 a): a taxa da B3 mostra a conta (saiu − troco − cotas = taxa), com ⓘ e o nome das compras', async () => {
    const t = await abrirConferencia(conferencia({ tipos: [], cotas: [], trocos: [], rendimentos: [], taxas: [taxa()] }));
    expect(t.raiz.querySelector('.resumo-conferencia')!.textContent!.trim()).toBe('Encontrado: 1 dia de compras sem a taxa da B3.');
    expect(t.raiz.querySelector('#conf-taxas')!.textContent!.replace(/ⓘ/g, '').replace(/\s+/g, ' ').trim())
      .toBe('Taxa da B3 que falta nas compras (1)');
    expect(t.raiz.querySelector('#conf-taxas .termo-botao')!.getAttribute('aria-label')).toBe('O que é Taxa da B3?');
    const celulas = Array.from(t.raiz.querySelectorAll('section[aria-labelledby="conf-taxas"] tbody td'))
      .map((td) => td.textContent!.replace(/\s/g, ' ').trim());
    expect(celulas.slice(1)).toEqual(['14/09/26', 'MXRF11 (50), XPLG11 (5)', 'R$ 1.049,23', '− R$ 20,90', '− R$ 1.028,10', 'R$ 0,23']);
    expect(t.raiz.querySelector('section[aria-labelledby="conf-taxas"] input[type=checkbox]')!.getAttribute('aria-label'))
      .toBe('Lançar a taxa da B3 das compras de 14/09/2026');
  });

  it('o troco tem ⓘ do glossário e cada linha diz de onde veio', async () => {
    const t = await abrirConferencia();
    expect(t.raiz.querySelector('#conf-trocos .termo-botao')!.getAttribute('aria-label')).toBe('O que é Troco da compra?');
    const origem = t.raiz.querySelector('#tabela-trocos tbody td:nth-child(4)')!.textContent!.replace(/\s/g, ' ');
    expect(origem).toBe('troco da compra de R$ 592,75 em 15/06/2026');
  });

  it('lista longa começa recolhida, com botão que diz quantos são e aria-expanded', async () => {
    const muitos = Array.from({ length: 20 }, (_, i) => troco({ transacaoId: i + 1 }));
    const t = await abrirConferencia(conferencia({ trocos: muitos }));
    expect(t.raiz.querySelectorAll('#tabela-trocos tbody tr')).toHaveLength(8);
    const ver = t.botao('Ver todos os 20');
    expect(ver.getAttribute('aria-expanded')).toBe('false');
    expect(ver.getAttribute('aria-controls')).toBe('tabela-trocos');
    ver.click();
    t.fixture.detectChanges();
    expect(t.raiz.querySelectorAll('#tabela-trocos tbody tr')).toHaveLength(20);
    expect(t.botao('Mostrar menos').getAttribute('aria-expanded')).toBe('true');
  });

  it('caixas de marcar e campos têm nome; a compra mostra o fechamento do dia para comparar', async () => {
    const t = await abrirConferencia();
    const caixa = t.raiz.querySelector('#tabela-trocos input[type=checkbox]')!;
    expect(caixa.getAttribute('aria-label')).toBe('Corrigir o crédito de 15/06/2026');
    // Só o texto do próprio rótulo (sem as opções do seletor).
    const rotulos = Array.from(t.raiz.querySelectorAll('.compra-que-faltou label')).map((l) => Array.from(l.childNodes)
      .filter((n) => n.nodeType === Node.TEXT_NODE).map((n) => n.textContent).join('').trim());
    expect(rotulos).toEqual(['Lançar a compra', 'Débito do extrato', 'Cotas', 'Preço por cota']);
    expect(t.raiz.querySelector('.compra-que-faltou small')!.textContent).toMatch(/fechamento do dia: R\$\s8,19/);
  });

  it('carteira com cotas a mais não sugere compra: leva às operações', async () => {
    const t = await abrirConferencia(conferencia({ cotas: [{ ...conferencia().cotas[0], naCorretora: 10, naCarteira: 12 }] }));
    expect(t.raiz.querySelector('.compra-que-faltou')).toBeNull();
    t.botao('Ver as operações de GARE11').click();
    expect(t.eventos.verOperacoes).toEqual(['GARE11']);
  });

  it('"ocultar valores" tira o valor do débito e da compra (o resto da linha continua)', async () => {
    const t = await abrirConferencia();
    TestBed.inject(PrivacidadeService).oculto.set(true);
    expect(t.tela.rotuloDebito(conferencia().comprasSemOperacao[0])).toBe('15/06/2026 · Nubank');
    expect(t.tela.origem(troco())).toBe('troco da compra em 15/06/2026');
  });
});
