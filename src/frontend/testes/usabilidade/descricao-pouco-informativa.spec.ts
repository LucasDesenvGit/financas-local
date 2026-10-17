import { abrirTransacoes } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';
import { abrirPainel, painel } from '../apoio/pouco-informativa';
import { paginaVazia } from '../apoio/transacoes';

/**
 * Usabilidade do filtro "Descrição pouco informativa" (#9). Nielsen 1: o filtro ativo aparece como chip e a lista
 * explica por que cada transação está ali e como sair dela; Nielsen 2 e 10: o termo tem ⓘ do glossário; Nielsen 8:
 * no Painel é um aviso discreto (sem ⚠ nem fundo de alerta), como a issue pede; WCAG 4.1.2: a ação da linha diz de
 * qual transação é.
 */
describe('Usabilidade: descrição pouco informativa', () => {
  it('o filtro tem rótulo e o ⓘ explica o termo', async () => {
    const t = await abrirTransacoes();
    const caixa = t.elemento.querySelector<HTMLInputElement>('input[name=poucoInformativa]')!;
    expect(caixa.closest('label')!.textContent!.trim()).toBe('Descrição pouco informativa');
    const termo = t.elemento.querySelector('.filtro-generica .termo-botao')!;
    expect(termo.getAttribute('aria-label')).toBe('O que é Descrição pouco informativa?');
  });

  it('com o filtro: chip, explicação de como sair da lista e ação por linha com o nome da transação', async () => {
    const t = await abrirTransacoes('/transacoes?generica=1', (f) => ({ ...paginaVazia(f, 1), itens: [transacao({ id: 7, descricao: 'PIX ENVIADO' })] }));
    const chips = Array.from(t.elemento.querySelectorAll('.chip-filtro')).map((c) => c.textContent!.replace('✕', '').trim());
    expect(chips).toEqual(['Descrição pouco informativa']);
    expect(t.elemento.querySelector('.dica-generica')!.textContent).toContain('marca "revisada"');
    const botao = t.elemento.querySelector<HTMLButtonElement>('[aria-label="Marcar como revisada: PIX ENVIADO"]')!;
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.textContent!.trim()).toBe('✓ marcar como revisada');
  });

  it('sem o filtro, as linhas não têm a ação nem a explicação', async () => {
    const t = await abrirTransacoes('/transacoes', (f) => ({ ...paginaVazia(f, 1), itens: [transacao({ descricao: 'PIX ENVIADO' })] }));
    expect(t.elemento.querySelector('.botao-revisada')).toBeNull();
    expect(t.elemento.querySelector('.dica-generica')).toBeNull();
  });

  it('no Painel é um aviso discreto, fora da caixa de alertas, com exemplo e "Revisar"', async () => {
    const { raiz } = await abrirPainel(painel({
      alertas: ['Orçamento de Mercado estourado (115%).'], poucoInformativas: 17, poucoInformativasDesde: '2026-07-20',
      exemploPoucoInformativa: 'PIX ENVIADO'
    }));
    const nota = raiz.querySelector('.nota-discreta')!;
    expect(nota.closest('.alertas')).toBeNull();
    expect(nota.textContent).not.toContain('⚠');
    expect(nota.textContent!.replace(/\s+/g, ' ')).toContain('17 transações dos últimos 90 dias com descrição pouco informativa');
    expect(nota.textContent).toContain('(ex.: “PIX ENVIADO”)');
    expect(nota.querySelector('a')!.textContent!.trim()).toBe('Revisar');
  });

  it('sem descrições pouco informativas, o Painel não mostra nada', async () => {
    const { raiz } = await abrirPainel(painel());
    expect(raiz.querySelector('.nota-discreta')).toBeNull();
  });
});
