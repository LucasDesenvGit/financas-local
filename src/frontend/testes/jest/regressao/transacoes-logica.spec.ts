import {
  Categoria, Conta, NaturezaGasto, StatusTransacao, SugestaoCategoria, TipoLancamento, TipoTransacao, Transacao
} from '../../../src/app/core/modelos';
import {
  EscolhaCategoria, LinhaSugestao, chipsDoFiltro, conflitoRegra, filtroInicial, formDaTransacao, itensDoLote, lerFiltroDaUrl,
  linhasDeSugestao, parametrosDoFiltro, regrasSugeridas, rotuloPeriodo, semOChip, somandoNosTotais, tamanhoPaginaValido,
  textoSelecao, transacaoDoForm
} from '../../../src/app/paginas/movimentacao/transacoes';

/**
 * #475 (ADR 0059): a lógica da tela de Transações em funções puras, testada sem montar o componente. Aqui fica só o que
 * ainda não tinha teste; a leitura da URL, a ordenação, a seleção e o modo edição pela tela continuam nos testes que
 * a montam (transacoes-*.spec.ts, selecao-tabelas.spec.ts) e as sugestões em conflito no navegador.
 */
const CATEGORIAS = [
  { id: 12, nome: 'Mercado', categoriaPaiId: null, ativa: true },
  { id: 20, nome: 'Lazer', categoriaPaiId: null, ativa: true },
  { id: 21, nome: 'Cinema', categoriaPaiId: 20, ativa: true }
] as Categoria[];
const CONTAS = [{ id: 3, nome: 'Nubank' }] as Conta[];

function transacao(campos: Partial<Transacao> = {}): Transacao {
  return {
    id: 1, contaId: 3, contaNome: 'Nubank', openFinance: false, data: '2026-10-10', descricao: 'PADARIA PAO QUENTE',
    descricaoOriginal: null, estabelecimento: null, valor: -25, tipo: TipoTransacao.Despesa, status: StatusTransacao.Efetivada,
    categoriaId: 12, categoriaNome: 'Mercado', categoriaCor: '#f2d98b', parcelaNumero: null, parcelaTotal: null,
    ignorarRelatorios: false, observacao: null, emergencia: false, tipoLancamento: TipoLancamento.CompraDebito, ...campos
  } as Transacao;
}

const url = (texto: string) => new URLSearchParams(texto);
const OUTUBRO = filtroInicial(new Date(2026, 9, 15), 50);
const SEM_EXTRAS = { categorias: [], transacaoDestacada: null, origemIds: null, modoEdicao: false };

describe('Regressão: lógica da tela de Transações em funções puras (#475)', () => {
  describe('filtro ↔ URL', () => {
    it('o que vai para a URL volta igual ao abrir o link (F5 e link salvo, UX-5)', () => {
      const f = {
        ...OUTUBRO, inicio: '2026-01-01', fim: '2026-03-31', tipo: TipoTransacao.Despesa, contaId: 3, somenteEmergencias: true,
        poucoInformativa: true, busca: 'uber', naturezas: [NaturezaGasto.Essencial], semNatureza: true, ordem: 'valor' as const,
        direcao: 'asc' as const, pagina: 3, ids: [7, 8]
      };
      const extras = { categorias: ['21', 'sem'], transacaoDestacada: 9, origemIds: 'fii-aportes' as const, modoEdicao: true };
      const params = parametrosDoFiltro(f, OUTUBRO, extras);
      const lido = lerFiltroDaUrl(url(new URLSearchParams(params).toString()), OUTUBRO, CATEGORIAS, CONTAS);
      expect(lido.filtro).toEqual({ ...f, pagina: 1, semCategoria: true, categoriaIds: [21], categoriaId: null });
      expect(lido).toMatchObject(extras);
    });

    it('o filtro padrão não põe nada na URL', () => {
      expect(parametrosDoFiltro(OUTUBRO, OUTUBRO, SEM_EXTRAS)).toEqual({});
    });

    it('?tipo= com nome de propriedade do objeto é ignorado', () => {
      expect(lerFiltroDaUrl(url('tipo=constructor'), OUTUBRO, CATEGORIAS, CONTAS).filtro.tipo).toBeNull();
    });
  });

  describe('chips de "Filtrando:" (UX-2)', () => {
    it.each([
      ['2026-10-01', '2026-10-31', 'outubro/2026'],
      ['2026-09-01', '2026-10-15', '01/09 – 15/10'],
      ['2025-12-20', '2026-01-10', '20/12/2025 – 10/01/2026'],
      ['2026-09-01', null, 'a partir de 01/09/2026'],
      [null, '2026-09-30', 'até 30/09/2026'],
      [null, null, 'Todo o período']
    ])('período %s a %s: "%s"', (inicio, fim, rotulo) => {
      expect(rotuloPeriodo(inicio, fim)).toBe(rotulo);
    });

    it('o ✕ de uma natureza tira só ela; o de uma categoria tira só ela, também do filtro da API', () => {
      const f = { ...OUTUBRO, naturezas: [NaturezaGasto.Essencial, NaturezaGasto.EstiloDeVida], semNatureza: true,
        semCategoria: true, categoriaIds: [12, 21] };
      const semEssencial = semOChip(f, ['sem', '12', '21'], 'natureza:essencial', OUTUBRO);
      expect(semEssencial.filtro).toMatchObject({ naturezas: [NaturezaGasto.EstiloDeVida], semNatureza: true });
      const semCinema = semOChip(f, ['sem', '12', '21'], 'categoria:21', OUTUBRO);
      expect(semCinema.categorias).toEqual(['sem', '12']);
      expect(semCinema.filtro).toMatchObject({ semCategoria: true, categoriaIds: [12] });
      expect(f.categoriaIds).toEqual([12, 21]);
    });

    it('categoria e conta que sumiram das listas ainda aparecem no chip, com um nome genérico', () => {
      const f = { ...OUTUBRO, categoriaIds: [99], contaId: 77 };
      expect(chipsDoFiltro(f, OUTUBRO, CATEGORIAS, CONTAS, null).map((c) => c.rotulo)).toEqual(['Categoria', 'Conta']);
    });
  });

  it('"N selecionadas" diz quantas estão em outras páginas (#133)', () => {
    expect(textoSelecao(0, 0)).toBe('');
    expect(textoSelecao(1, 1)).toBe('1 selecionada');
    expect(textoSelecao(5, 4)).toBe('5 selecionadas (1 em outra página)');
    expect(textoSelecao(5, 2)).toBe('5 selecionadas (3 em outras páginas)');
  });

  it('tamanho de página guardado: só os oferecidos valem; o resto, 50', () => {
    expect(tamanhoPaginaValido('100')).toBe(100);
    expect(['30', 'abc', null].map(tamanhoPaginaValido)).toEqual([50, 50, 50]);
  });

  it('pagamento de fatura que entra nos totais: na saída conta como gasto, na entrada como receita', () => {
    const fatura = (valor: number) => transacao({ tipoLancamento: TipoLancamento.PagamentoFatura, valor });
    expect(somandoNosTotais(fatura(-500))).toBe('gastos');
    expect(somandoNosTotais(fatura(500))).toBe('receitas');
    expect(somandoNosTotais({ ...fatura(-500), ignorarRelatorios: true })).toBeNull();
    expect(somandoNosTotais(transacao())).toBeNull();
  });

  describe('modo edição: regra sugerida ao salvar (#87)', () => {
    const pendentes = (pares: [number, EscolhaCategoria][]) => new Map(pares);

    it('2 ou mais iguais para a mesma categoria; "auto", texto curto e uma só não sugerem', () => {
      const itens = [
        transacao({ id: 1, descricao: 'Padaria', descricaoOriginal: 'PADARIA  PAO QUENTE' }),
        transacao({ id: 2, descricao: 'PADARIA PAO QUENTE' }),
        transacao({ id: 3, descricao: 'PIX' }), transacao({ id: 4, descricao: 'PIX' }),
        transacao({ id: 5, descricao: 'UBER TRIP' }), transacao({ id: 6, descricao: 'UBER TRIP' }),
        transacao({ id: 7, descricao: 'CINEMARK' })
      ];
      const escolhas = pendentes([[1, '21'], [2, '21'], [3, '20'], [4, '20'], [5, 'auto'], [6, 'auto'], [7, '20']]);
      expect(regrasSugeridas(itens, escolhas, CATEGORIAS)).toEqual([
        { texto: 'PADARIA PAO QUENTE', categoriaId: 21, categoriaNome: 'Lazer › Cinema', quantidade: 2 }
      ]);
      expect(itensDoLote(escolhas).slice(4, 5)).toEqual([{ id: 5, categoriaId: null, automatica: true }]);
    });

    it('no máximo 3 sugestões', () => {
      const itens = [1, 2, 3, 4, 5, 6, 7, 8].map((id) => transacao({ id, descricao: `LOJA ${Math.ceil(id / 2)}` }));
      const escolhas = pendentes(itens.map((t) => [t.id, '12']));
      expect(regrasSugeridas(itens, escolhas, CATEGORIAS)).toHaveLength(3);
    });
  });

  describe('sugestões de categoria (#254)', () => {
    const sugestao = (campos: Partial<SugestaoCategoria>): SugestaoCategoria => ({
      chave: 'a', exemplo: '', quantidade: 5, total: 0, transacaoIds: [], categoriaId: 12, categoriaNome: null, categoriaCor: null,
      confianca: 'alta', motivo: null, textoRegra: 'MERCADO LIVRE', alternativas: [], ...campos
    });

    it('"criar regra" vem marcado só com confiança alta, 3 ou mais, texto de 4 letras, sem aviso e sem conflito', () => {
      const linhas = linhasDeSugestao([
        sugestao({ chave: 'ok', textoRegra: 'IFOOD' }),
        sugestao({ chave: 'poucas', textoRegra: 'UBER', quantidade: 2 }),
        sugestao({ chave: 'media', textoRegra: 'POSTO', confianca: 'media' }),
        sugestao({ chave: 'curto', textoRegra: 'BAR' }),
        sugestao({ chave: 'aviso', textoRegra: 'AMAZON', avisoRegra: 'já existe' }),
        sugestao({ chave: 'conflito-1', textoRegra: 'Mercado Livre', categoriaId: 12 }),
        sugestao({ chave: 'conflito-2', textoRegra: 'MERCADO LIVRE', categoriaId: 20 })
      ]);
      expect(linhas.filter((l) => l.criarRegra).map((l) => l.chave)).toEqual(['ok']);
      expect(linhas[0]).toMatchObject({ escolhida: 12, aplicada: false });
    });

    it('regra já criada nesta janela com outra categoria: conflito, com o nome dela', () => {
      const linha = { ...sugestao({ textoRegra: ' ifood ' }), escolhida: 12, criarRegra: true, aplicada: false } as LinhaSugestao;
      expect(conflitoRegra(linha, [linha], new Map([['IFOOD', 20]]), CATEGORIAS)).toBe('Lazer');
      expect(conflitoRegra(linha, [linha], new Map([['IFOOD', 12]]), CATEGORIAS)).toBeNull();
      expect(conflitoRegra({ ...linha, criarRegra: false }, [linha], new Map([['IFOOD', 20]]), CATEGORIAS)).toBeNull();
    });
  });

  describe('formulário', () => {
    it('editar: valor sem sinal, tipo pelo sinal, pendente pelo status', () => {
      const f = formDaTransacao(transacao({ valor: 80, status: StatusTransacao.Pendente, observacao: null }));
      expect(f).toMatchObject({ valor: 80, tipo: TipoTransacao.Receita, pendente: true, observacao: '' });
      expect(formDaTransacao(transacao({ valor: -80 })).tipo).toBe(TipoTransacao.Despesa);
    });

    it('emergência só vai para a API em despesa; observação vazia vai como null', () => {
      const receita = { ...formDaTransacao(transacao({ valor: 80 })), emergencia: true };
      expect(transacaoDoForm(receita)).toMatchObject({ emergencia: false, observacao: null, status: StatusTransacao.Efetivada });
      expect(transacaoDoForm({ ...receita, tipo: TipoTransacao.Despesa }).emergencia).toBe(true);
    });
  });
});
