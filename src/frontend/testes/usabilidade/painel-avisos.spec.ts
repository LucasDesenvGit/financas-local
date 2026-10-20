import { TestBed } from '@angular/core/testing';
import { Orcamento } from '../../src/app/core/modelos';
import {
  CHAVE_AVISOS_OCULTOS, CHAVE_ORCAMENTOS_OCULTOS, avisosContinuamOcultos, impressaoAviso, lerOrcamentosOcultos, nivelOrcamento,
  ocultarOrcamentos, orcamentosContinuamOcultos
} from '../../src/app/paginas/painel-avisos';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #206: avisos do Painel agrupados, que dá para ocultar e que voltam quando algo novo ou pior aparece
 * (Nielsen 1 e 3; Progressive Disclosure; APG Disclosure no botão).
 */
function orc(categoriaId: number, percentual: number, situacao: Orcamento['situacao'] = 'estourado'): Orcamento {
  return { id: categoriaId, categoriaId, categoriaNome: `Categoria ${categoriaId}`, categoriaCor: '#999', ano: 2026, mes: 10,
    valor: 100, gasto: percentual, percentual, gastoEmergencia: 0, situacao };
}

const hoje = new Date();
const periodoAtual = { ano: hoje.getFullYear(), mes: hoje.getMonth() + 1 };
const urlAtual = `/painel?ano=${periodoAtual.ano}&mes=${periodoAtual.mes}`;

describe('quando os avisos de orçamento ocultos voltam', () => {
  const periodo = '2026-10';

  it('continuam ocultos no mesmo mês, sem nada novo nem pior', () => {
    const lista = [orc(1, 130), orc(2, 105)];
    const salvo = ocultarOrcamentos({}, periodo, lista);
    expect(orcamentosContinuamOcultos(salvo, periodo, lista)).toBe(true);
    // Subir dentro da mesma faixa (130% → 145%) não reabre.
    expect(orcamentosContinuamOcultos(salvo, periodo, [orc(1, 145), orc(2, 105)])).toBe(true);
    // Um deles sair da lista também não.
    expect(orcamentosContinuamOcultos(salvo, periodo, [orc(1, 130)])).toBe(true);
  });

  it('voltam no mês seguinte', () => {
    const lista = [orc(1, 130)];
    expect(orcamentosContinuamOcultos(ocultarOrcamentos({}, periodo, lista), '2026-11', lista)).toBe(false);
  });

  it('voltam quando outra categoria estoura', () => {
    const salvo = ocultarOrcamentos({}, periodo, [orc(1, 130)]);
    expect(orcamentosContinuamOcultos(salvo, periodo, [orc(1, 130), orc(2, 101)])).toBe(false);
  });

  it('voltam quando uma já estourada sobe de faixa (120%, 150%, 200%, 300%)', () => {
    const salvo = ocultarOrcamentos({}, periodo, [orc(1, 119)]);
    expect(orcamentosContinuamOcultos(salvo, periodo, [orc(1, 120)])).toBe(false);
    const salvo150 = ocultarOrcamentos({}, periodo, [orc(1, 140)]);
    expect(orcamentosContinuamOcultos(salvo150, periodo, [orc(1, 150)])).toBe(false);
  });

  it('voltam quando o que era só emergência vira estouro de verdade, na mesma faixa', () => {
    expect(nivelOrcamento(orc(1, 110, 'estourado'))).toBeGreaterThan(nivelOrcamento(orc(1, 110, 'emergencia')));
    const salvo = ocultarOrcamentos({}, periodo, [orc(1, 110, 'emergencia')]);
    expect(orcamentosContinuamOcultos(salvo, periodo, [orc(1, 110, 'estourado')])).toBe(false);
  });

  it('lembra só os 6 meses mais recentes, e valor estragado no navegador conta como nada oculto', () => {
    let salvo = {};
    for (let m = 1; m <= 9; m++) salvo = ocultarOrcamentos(salvo, `2026-0${m}`, [orc(1, 130)]);
    expect(Object.keys(salvo)).toEqual(['2026-04', '2026-05', '2026-06', '2026-07', '2026-08', '2026-09']);

    localStorage.setItem(CHAVE_ORCAMENTOS_OCULTOS, '{"isto não é": um json');
    expect(lerOrcamentosOcultos()).toEqual({});
    localStorage.setItem(CHAVE_ORCAMENTOS_OCULTOS, JSON.stringify({ '2026-10': { 1: 'x', 2: 5 }, lixo: { 1: 3 } }));
    expect(lerOrcamentosOcultos()).toEqual({ '2026-10': { 2: 5 } });
    localStorage.clear();
  });
});

describe('os outros avisos ocultos', () => {
  it('voltam quando aparece um aviso que não estava lá ao ocultar', () => {
    const alertas = ['Conta "Aluguel" vencida desde 05/10.'];
    const vistos = alertas.map(impressaoAviso);
    expect(avisosContinuamOcultos(vistos, alertas)).toBe(true);
    expect(avisosContinuamOcultos(vistos, [...alertas, 'Falha na última sincronização de Nubank.'])).toBe(false);
    expect(avisosContinuamOcultos(vistos, ['Conta "Aluguel" vencida desde 06/10.'])).toBe(false);
    expect(avisosContinuamOcultos(vistos, [])).toBe(false);
  });
});

describe('Painel: card dos orçamentos estourados (#206)', () => {
  beforeEach(() => localStorage.clear());

  const estourados = [orc(1, 348), orc(2, 120), orc(3, 112, 'emergencia'), orc(4, 105)];

  it('agrupa num card com título, os 3 piores primeiro e "Ver todos" para o resto', async () => {
    const { raiz, atualizar } = await abrirPainel(painel({ orcamentosEstourados: estourados }), urlAtual);
    const card = raiz.querySelector('.aviso-orcamentos') as HTMLElement;
    expect(card.querySelector('h2')!.textContent).toContain('4 orçamentos estourados neste mês (1 por emergência)');
    expect([...card.querySelectorAll('.lista-estourados li a')].map((a) => a.textContent!.trim()))
      .toEqual(['Categoria 1', 'Categoria 2', 'Categoria 3']);

    const verTodos = [...card.querySelectorAll('button')].find((b) => b.textContent!.includes('Ver todos os 4'))!;
    expect(verTodos.getAttribute('aria-expanded')).toBe('false');
    verTodos.click();
    await atualizar();
    expect(card.querySelectorAll('.lista-estourados li')).toHaveLength(4);
    expect(verTodos.getAttribute('aria-expanded')).toBe('true');
  });

  it('diz quanto passou, marca o que foi por emergência e leva a Transações filtrada pela categoria e pelo mês', async () => {
    const { raiz } = await abrirPainel(painel({ orcamentosEstourados: estourados }), urlAtual);
    const itens = raiz.querySelectorAll('.lista-estourados li');
    expect(itens[0].textContent).toContain('348%');
    expect(itens[2].classList).toContain('por-emergencia');
    expect(itens[2].querySelector('.etiqueta-emergencia')!.textContent).toContain('por emergência');

    const link = itens[0].querySelector('a')!;
    const href = decodeURIComponent(link.getAttribute('href')!);
    expect(href).toContain('/transacoes?');
    expect(href).toContain('categoria=1');
    expect(href).toContain('tipo=despesa');
    expect(href).toMatch(new RegExp(`inicio=${periodoAtual.ano}-${String(periodoAtual.mes).padStart(2, '0')}-01`));
    expect(link.getAttribute('aria-label')).toMatch(/^Ver as despesas de Categoria 1 em /);
  });

  it('"Ocultar até o mês que vem" recolhe numa linha, é lembrado e "Mostrar" abre de novo', async () => {
    const { raiz, atualizar } = await abrirPainel(painel({ orcamentosEstourados: estourados }), urlAtual);
    const botao = raiz.querySelector('.aviso-orcamentos .aviso-painel-alternar') as HTMLButtonElement;
    expect(botao.textContent!.trim()).toBe('Ocultar até o mês que vem');
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(botao.getAttribute('aria-controls')).toBe('lista-orcamentos-estourados');
    expect(raiz.querySelector('#lista-orcamentos-estourados')).not.toBeNull();

    botao.click();
    await atualizar();
    expect(raiz.querySelector('.lista-estourados')).toBeNull();
    // O título continua: nada some de vez (Nielsen 1).
    expect(raiz.querySelector('.aviso-orcamentos h2')!.textContent).toContain('4 orçamentos estourados');
    expect(botao.textContent!.trim()).toBe('Mostrar');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(localStorage.getItem(CHAVE_ORCAMENTOS_OCULTOS)).toContain(`${periodoAtual.ano}-`);

    botao.click();
    await atualizar();
    expect(raiz.querySelectorAll('.lista-estourados li').length).toBeGreaterThan(0);
  });

  it('ocultos, voltam abertos ao recarregar com uma categoria nova estourada', async () => {
    const primeira = await abrirPainel(painel({ orcamentosEstourados: [orc(1, 130)] }), urlAtual);
    (primeira.raiz.querySelector('.aviso-orcamentos .aviso-painel-alternar') as HTMLButtonElement).click();
    await primeira.atualizar();
    expect(primeira.raiz.querySelector('.lista-estourados')).toBeNull();

    primeira.harness.fixture.destroy();
    TestBed.resetTestingModule();
    const segunda = await abrirPainel(painel({ orcamentosEstourados: [orc(1, 130), orc(5, 101)] }), urlAtual);
    expect(segunda.raiz.querySelectorAll('.lista-estourados li')).toHaveLength(2);
  });

  it('sem orçamento estourado, não há card; e os estourados não aparecem mais como alerta solto', async () => {
    const { raiz } = await abrirPainel(painel({ alertas: [] }), urlAtual);
    expect(raiz.querySelector('.aviso-orcamentos')).toBeNull();
    expect(raiz.querySelector('.aviso-geral')).toBeNull();
  });
});

describe('Painel: card dos outros avisos (#206)', () => {
  beforeEach(() => localStorage.clear());

  it('lista os avisos com título e "Ocultar até aparecer um novo"; oculto, fica a linha com a quantidade', async () => {
    const alertas = ['Conta "Aluguel" vencida desde 05/10.', 'Falha na última sincronização de Nubank.'];
    const { raiz, atualizar } = await abrirPainel(painel({ alertas }), urlAtual);
    const card = raiz.querySelector('.aviso-geral') as HTMLElement;
    expect(card.querySelector('h2')!.textContent).toContain('2 avisos pedem atenção');
    expect(card.querySelectorAll('li')).toHaveLength(2);

    const botao = card.querySelector('.aviso-painel-alternar') as HTMLButtonElement;
    expect(botao.textContent!.trim()).toBe('Ocultar até aparecer um novo');
    botao.click();
    await atualizar();
    expect(card.querySelectorAll('li')).toHaveLength(0);
    expect(card.querySelector('h2')!.textContent).toContain('2 avisos pedem atenção');
    // Guarda só a impressão de cada aviso, não o texto (nomes de contas ficam fora do localStorage, ADR 0017).
    const guardado = localStorage.getItem(CHAVE_AVISOS_OCULTOS)!;
    expect(JSON.parse(guardado)).toEqual(alertas.map(impressaoAviso));
    expect(guardado).not.toContain('Aluguel');
  });
});
