/**
 * Lógica da tela de Transações em funções puras (#475, ADR 0059): filtro ↔ URL, chips de filtro, ordenação, seleção,
 * modo edição de categorias, sugestões e formulário. Sem Angular: o componente busca os dados, guarda o estado e chama
 * estas funções; os testes do Jest as chamam direto, sem montar a tela (#204).
 */
import type { OpcaoMultipla } from '../../compartilhado/ui/seletor-multiplo.component';
import {
  AjusteTransacao, Categoria, Conta, FiltroTransacoes, NaturezaGasto, OrdemTransacoes, SalvarTransacao, StatusTransacao,
  SugestaoCategoria, TipoTransacao, Transacao
} from '../../core/modelos';
import {
  ContextoTransacoes, NOMES_MESES, NaturezaUrl, ORIGENS_IDS, OrigemIdsUrl, dataIsoValida, idsDaUrl, paraData,
  paramsTransacoes, primeiroDiaMes, ultimoDiaMes
} from '../../compartilhado/utilidades';
import { GRUPOS_LANCAMENTO, NAO_E_GASTO } from '../../compartilhado/lancamentos/tipos-lancamento';

/** O que a função precisa da URL: o `ParamMap` do Angular serve, e no teste um objeto com `get`. */
export interface ParametrosUrl { get(nome: string): string | null }

/* ------------------------------ tipos da tela ------------------------------ */

export interface FormTransacao {
  id: number | null;
  openFinance: boolean;
  contaId: number | null;
  data: string;
  descricao: string;
  valor: number | null;
  tipo: TipoTransacao;
  categoriaId: number | null;
  observacao: string;
  ignorarRelatorios: boolean;
  pendente: boolean;
  descricaoOriginal: string | null;
  emergencia: boolean;
}

export interface FormTransferencia {
  contaOrigemId: number | null;
  contaDestinoId: number | null;
  valor: number | null;
  data: string;
  descricao: string;
}

/** Linha editável do assistente de sugestões. */
export interface LinhaSugestao extends SugestaoCategoria { escolhida: number | null; criarRegra: boolean; aplicada: boolean }

export interface ChipFiltro {
  /** Categoria e natureza têm um chip por valor marcado: `categoria:12`, `categoria:sem`, `natureza:essencial`. */
  id: 'periodo' | `categoria:${string}` | `natureza:${NaturezaUrl}` | 'conta' | 'tipo' | 'lancamento' | 'emergencias' | 'generica' | 'busca'
    | 'ids';
  rotulo: string;
  cor?: string;
}

/** #87: categoria escolhida no modo edição: 'auto' (voltar para as regras) ou o id da categoria. */
export type EscolhaCategoria = 'auto' | `${number}`;

/** "Criar regra para as próximas": várias transações iguais foram para a mesma categoria. */
export interface SugestaoRegra { texto: string; categoriaId: number; categoriaNome: string; quantidade: number; }

/* ------------------------------ filtro e URL ------------------------------ */

/** #244: a direção mais útil de cada coluna ao clicar nela pela primeira vez. */
export const DIRECAO_PADRAO: Record<OrdemTransacoes, 'asc' | 'desc'> = { data: 'desc', valor: 'desc', categoria: 'asc', conta: 'asc' };

/** #244: as colunas que ordenam (Descrição não), na ordem do cabeçalho. */
export const COLUNAS_ORDENAVEIS: readonly { chave: OrdemTransacoes; rotulo: string }[] = [
  { chave: 'data', rotulo: 'Data' }, { chave: 'categoria', rotulo: 'Categoria' }, { chave: 'conta', rotulo: 'Conta' },
  { chave: 'valor', rotulo: 'Valor' }
];

/** Tamanhos de página oferecidos; o escolhido fica no navegador (`financas.tamanhoPagina`, ADR 0017). */
export const TAMANHOS_PAGINA = [25, 50, 100, 200];
export const TAMANHO_PAGINA_PADRAO = 50;

/** ?natureza= da URL (Visão anual, regra 50/30/20) e o rótulo do chip: as mesmas palavras do gráfico de lá. */
export const NATUREZAS_URL: Record<NaturezaUrl, { natureza: NaturezaGasto | null; rotulo: string }> = {
  'essencial': { natureza: NaturezaGasto.Essencial, rotulo: 'Essenciais' },
  'estilo-de-vida': { natureza: NaturezaGasto.EstiloDeVida, rotulo: 'Estilo de vida' },
  'sem': { natureza: null, rotulo: 'Sem classificação (essencial/estilo de vida)' }
};

const TIPOS_URL: Record<string, TipoTransacao> = {
  despesa: TipoTransacao.Despesa, receita: TipoTransacao.Receita, transferencia: TipoTransacao.Transferencia
};
const URL_DO_TIPO: Record<number, ContextoTransacoes['tipo']> = {
  [TipoTransacao.Despesa]: 'despesa', [TipoTransacao.Receita]: 'receita', [TipoTransacao.Transferencia]: 'transferencia'
};
const NOME_DO_TIPO: Record<number, string> = {
  [TipoTransacao.Despesa]: 'Despesas', [TipoTransacao.Receita]: 'Receitas', [TipoTransacao.Transferencia]: 'Transferências'
};

/** O filtro padrão: o mês de `hoje`, sem outros filtros, página 1. */
export function filtroInicial(hoje: Date, tamanhoPagina: number): FiltroTransacoes {
  return {
    inicio: primeiroDiaMes(hoje.getFullYear(), hoje.getMonth() + 1),
    fim: ultimoDiaMes(hoje.getFullYear(), hoje.getMonth() + 1),
    contaId: null, categoriaId: null, categoriaIds: [], semCategoria: false, somenteEmergencias: false, poucoInformativa: false,
    lancamento: null, naturezas: [], semNatureza: false, tipo: null, busca: '', ids: [], pagina: 1, tamanhoPagina
  };
}

/** O tamanho de página guardado no navegador; inválido ou ausente, o padrão. */
export function tamanhoPaginaValido(guardado: string | null): number {
  const n = Number(guardado);
  return TAMANHOS_PAGINA.includes(n) ? n : TAMANHO_PAGINA_PADRAO;
}

/** Naturezas do filtro como estão na URL, na ordem fixa de NATUREZAS_URL. */
export function naturezasNaUrl(f: FiltroTransacoes): NaturezaUrl[] {
  return (Object.keys(NATUREZAS_URL) as NaturezaUrl[]).filter((chave) => {
    const n = NATUREZAS_URL[chave].natureza;
    return n === null ? !!f.semNatureza : (f.naturezas ?? []).includes(n);
  });
}

/** As categorias marcadas ('sem' ou o id) como a API as recebe. */
export function camposDeCategorias(marcadas: string[]): Pick<FiltroTransacoes, 'semCategoria' | 'categoriaIds' | 'categoriaId'> {
  return { semCategoria: marcadas.includes('sem'), categoriaIds: marcadas.filter((v) => v !== 'sem').map(Number), categoriaId: null };
}

/** O contrário de {@link camposDeCategorias}: as categorias marcadas a partir do filtro ('sem' primeiro). */
export function categoriasDoFiltro(f: FiltroTransacoes): string[] {
  return [...(f.semCategoria ? ['sem'] : []), ...(f.categoriaIds ?? []).map(String)];
}

/** Página da lista vinda da URL (UX-5); inválida ou ausente = 1. */
export function paginaDaUrl(params: ParametrosUrl): number {
  const n = Number(params.get('pagina'));
  return Number.isInteger(n) && n >= 1 && n <= 100000 ? n : 1;
}

/** A última página que existe (1 com a lista vazia). */
export function ultimaPagina(total: number, tamanhoPagina: number): number {
  return Math.max(1, Math.ceil(total / tamanhoPagina));
}

/** O que a URL muda na tela, além do filtro. */
export interface LidoDaUrl {
  filtro: FiltroTransacoes;
  /** Categorias marcadas no filtro ('sem' ou o id), sem repetir, na ordem da URL. */
  categorias: string[];
  modoEdicao: boolean;
  origemIds: OrigemIdsUrl | null;
  transacaoDestacada: number | null;
}

/**
 * UX-1: filtro vindo de outra tela (?categoria=12|sem&inicio=yyyy-MM-dd&fim=yyyy-MM-dd&tipo=despesa|receita&conta=3).
 * Várias categorias vêm separadas por vírgula (?categoria=12,20,sem).
 * UX-5: e o que a própria tela grava na URL (&emergencias=1&busca=texto&pagina=2), para o F5 e o link salvo.
 * Cada parâmetro é validado sozinho: um inválido é ignorado e os outros continuam valendo. Categoria e conta só valem
 * se estão nas listas (`categorias` inclui as inativas).
 */
export function lerFiltroDaUrl(params: ParametrosUrl, base: FiltroTransacoes, categorias: Categoria[], contas: Conta[]): LidoDaUrl {
  const filtro: FiltroTransacoes = { ...base };
  const inicio = params.get('inicio');
  const fim = params.get('fim');
  const inicioOk = dataIsoValida(inicio);
  const fimOk = dataIsoValida(fim);
  if (inicioOk) filtro.inicio = inicio;
  if (fimOk) filtro.fim = fim;
  // Só uma das datas veio válida: a outra acompanha o mês dela, para não sobrar um período invertido (lista vazia).
  if (inicioOk !== fimOk && filtro.inicio! > filtro.fim!) {
    const data = paraData(inicioOk ? filtro.inicio! : filtro.fim!);
    if (inicioOk) filtro.fim = ultimoDiaMes(data.getFullYear(), data.getMonth() + 1);
    else filtro.inicio = primeiroDiaMes(data.getFullYear(), data.getMonth() + 1);
  }

  const tipo = params.get('tipo');
  if (tipo && Object.hasOwn(TIPOS_URL, tipo)) filtro.tipo = TIPOS_URL[tipo];

  const validas = new Set(['sem', ...categorias.map((c) => String(c.id))]);
  const marcadas = [...new Set((params.get('categoria') ?? '').split(',').map((v) => v.trim()).filter((v) => validas.has(v)))];
  Object.assign(filtro, camposDeCategorias(marcadas));

  const conta = Number(params.get('conta'));
  if (conta && contas.some((c) => c.id === conta)) filtro.contaId = conta;

  const naturezas = (params.get('natureza') ?? '').split(',').map((v) => v.trim())
    .filter((v): v is NaturezaUrl => Object.hasOwn(NATUREZAS_URL, v));
  filtro.semNatureza = naturezas.includes('sem');
  filtro.naturezas = [...new Set(naturezas.map((v) => NATUREZAS_URL[v].natureza).filter((n) => n !== null))];

  if (params.get('emergencias') === '1') filtro.somenteEmergencias = true;
  if (params.get('generica') === '1') filtro.poucoInformativa = true;
  const lancamento = params.get('lancamento');
  if (lancamento && GRUPOS_LANCAMENTO.some((g) => g.valor === lancamento)) filtro.lancamento = lancamento;
  const busca = params.get('busca')?.trim();
  if (busca) filtro.busca = busca.slice(0, 100);
  // #244: ordenação da URL, só das colunas da lista fixa; sem ela (ou inválida), o padrão (data, mais recente primeiro).
  const ordem = params.get('ordem');
  if (ordem && COLUNAS_ORDENAVEIS.some((c) => c.chave === ordem)) {
    filtro.ordem = ordem as OrdemTransacoes;
    const direcao = params.get('direcao');
    filtro.direcao = direcao === 'asc' || direcao === 'desc' ? direcao : DIRECAO_PADRAO[filtro.ordem];
  }
  let origemIds: OrigemIdsUrl | null = null;
  const ids = idsDaUrl(params.get('ids'));
  if (ids.length) {
    filtro.ids = ids;
    const de = params.get('de');
    origemIds = de && Object.hasOwn(ORIGENS_IDS, de) ? de as OrigemIdsUrl : null;
  }
  const transacao = Number(params.get('transacao'));
  return {
    filtro, categorias: marcadas, modoEdicao: params.get('editar') === '1', origemIds,
    transacaoDestacada: Number.isInteger(transacao) && transacao > 0 ? transacao : null
  };
}

/** O que vai para a URL além do filtro. */
export interface ExtrasDaUrl {
  categorias: string[];
  transacaoDestacada: number | null;
  origemIds: OrigemIdsUrl | null;
  modoEdicao: boolean;
}

/**
 * UX-5: os parâmetros da URL para o filtro `f`: só o que difere do `padrao`, para o F5 e o link salvo manterem a
 * lista e a URL do mês atual sem filtro ficar limpa (`/transacoes`).
 */
export function parametrosDoFiltro(f: FiltroTransacoes, padrao: FiltroTransacoes, extras: ExtrasDaUrl): Record<string, string> {
  const mudouPeriodo = f.inicio !== padrao.inicio || f.fim !== padrao.fim;
  const params = paramsTransacoes({
    inicio: mudouPeriodo ? f.inicio ?? undefined : undefined,
    fim: mudouPeriodo ? f.fim ?? undefined : undefined,
    tipo: f.tipo ? URL_DO_TIPO[f.tipo] : undefined,
    contaId: f.contaId ?? undefined,
    transacaoId: extras.transacaoDestacada ?? undefined,
    naturezas: naturezasNaUrl(f),
    ids: f.ids ?? undefined,
    origemIds: extras.origemIds ?? undefined
  });
  if (extras.categorias.length) params['categoria'] = extras.categorias.join(',');
  if (f.somenteEmergencias) params['emergencias'] = '1';
  if (f.poucoInformativa) params['generica'] = '1';
  if (f.lancamento) params['lancamento'] = f.lancamento;
  if (f.busca?.trim()) params['busca'] = f.busca.trim();
  if ((f.pagina ?? 1) > 1) params['pagina'] = String(f.pagina);
  if (f.ordem && !(f.ordem === 'data' && (f.direcao ?? 'desc') === 'desc')) {
    params['ordem'] = f.ordem;
    params['direcao'] = f.direcao ?? DIRECAO_PADRAO[f.ordem];
  }
  if (extras.modoEdicao) params['editar'] = '1';
  return params;
}

/**
 * #133: o filtro mudou além da página, do tamanho e da ordem? Trocar só de página mantém a seleção; trocar o filtro
 * limpa, para nenhuma ação em lote pegar uma transação que não aparece no filtro atual (Nielsen 5).
 */
export function trocouFiltro(antes: FiltroTransacoes, depois: FiltroTransacoes): boolean {
  const semPagina = (f: FiltroTransacoes) =>
    JSON.stringify({ ...f, pagina: undefined, tamanhoPagina: undefined, ordem: undefined, direcao: undefined });
  return semPagina(antes) !== semPagina(depois);
}

/* ------------------------------ chips de "Filtrando:" ------------------------------ */

/** "outubro/2026" quando é o mês cheio; senão "01/09 – 15/10" (com ano se o período cruza anos). */
export function rotuloPeriodo(inicio?: string | null, fim?: string | null): string {
  if (!inicio || !fim) return inicio ? `a partir de ${dataCurta(inicio, true)}` : fim ? `até ${dataCurta(fim, true)}` : 'Todo o período';
  const [ai, mi] = inicio.split('-').map(Number);
  if (inicio === primeiroDiaMes(ai, mi) && fim === ultimoDiaMes(ai, mi)) return `${NOMES_MESES[mi - 1]}/${ai}`;
  const cruzaAno = inicio.substring(0, 4) !== fim.substring(0, 4);
  return `${dataCurta(inicio, cruzaAno)} – ${dataCurta(fim, cruzaAno)}`;
}

function dataCurta(iso: string, comAno: boolean): string {
  const [a, m, d] = iso.split('-');
  return comAno ? `${d}/${m}/${a}` : `${d}/${m}`;
}

/** "Lazer › Cinema" para a subcategoria; o nome só para a categoria de cima (ou se o pai não está na lista). */
export function rotuloCategoria(c: Categoria, todas: Categoria[]): string {
  if (!c.categoriaPaiId) return c.nome;
  const pai = todas.find((x) => x.id === c.categoriaPaiId);
  return pai ? `${pai.nome} › ${c.nome}` : c.nome;
}

/** O rótulo da categoria pelo id; 'categoria' se ela não está na lista. */
export function rotuloCategoriaPorId(id: number, todas: Categoria[]): string {
  const c = todas.find((x) => x.id === id);
  return c ? rotuloCategoria(c, todas) : 'categoria';
}

/**
 * Opções do filtro de categorias: "Sem categoria" e cada categoria seguida das subcategorias (recuadas), por nome.
 * `oferecidas` são as que aparecem; `todas` (inclusive inativas) dão o nome do pai no rótulo.
 */
export function opcoesCategoria(oferecidas: Categoria[], todas: Categoria[]): OpcaoMultipla[] {
  const porNome = (a: Categoria, b: Categoria) => a.nome.localeCompare(b.nome, 'pt-BR');
  const ids = new Set(oferecidas.map((c) => c.id));
  const raizes = oferecidas.filter((c) => !c.categoriaPaiId || !ids.has(c.categoriaPaiId)).sort(porNome);
  const opcoes: OpcaoMultipla[] = [{ valor: 'sem', rotulo: 'Sem categoria' }];
  for (const r of raizes) {
    opcoes.push({ valor: String(r.id), rotulo: rotuloCategoria(r, todas), cor: r.cor });
    for (const f of oferecidas.filter((c) => c.categoriaPaiId === r.id).sort(porNome)) {
      opcoes.push({ valor: String(f.id), rotulo: f.nome, cor: f.cor, nivel: 1 });
    }
  }
  return opcoes;
}

/**
 * UX-2: um chip para cada filtro diferente do padrão (mês atual, sem filtros), para quem chega por um link
 * saber que a lista está filtrada. Nielsen 1 (visibilidade do estado) e 3 (controle: ✕ e "Limpar filtros").
 */
export function chipsDoFiltro(
  f: FiltroTransacoes, padrao: FiltroTransacoes, categorias: Categoria[], contas: Conta[], origemIds: OrigemIdsUrl | null
): ChipFiltro[] {
  const chips: ChipFiltro[] = [];
  if (f.inicio !== padrao.inicio || f.fim !== padrao.fim) chips.push({ id: 'periodo', rotulo: rotuloPeriodo(f.inicio, f.fim) });
  if (f.semCategoria) chips.push({ id: 'categoria:sem', rotulo: 'Sem categoria' });
  for (const id of f.categoriaIds ?? []) {
    const c = categorias.find((x) => x.id === id);
    chips.push({ id: `categoria:${id}`, rotulo: c ? rotuloCategoria(c, categorias) : 'Categoria', cor: c?.cor });
  }
  for (const n of naturezasNaUrl(f)) chips.push({ id: `natureza:${n}`, rotulo: NATUREZAS_URL[n].rotulo });
  if (f.contaId) chips.push({ id: 'conta', rotulo: contas.find((c) => c.id === f.contaId)?.nome ?? 'Conta' });
  if (f.tipo) chips.push({ id: 'tipo', rotulo: NOME_DO_TIPO[f.tipo] });
  if (f.lancamento) chips.push({ id: 'lancamento', rotulo: GRUPOS_LANCAMENTO.find((g) => g.valor === f.lancamento)?.rotulo ?? 'Lançamento' });
  if (f.somenteEmergencias) chips.push({ id: 'emergencias', rotulo: 'Só emergências' });
  if (f.poucoInformativa) chips.push({ id: 'generica', rotulo: 'Descrição pouco informativa' });
  if (f.busca?.trim()) chips.push({ id: 'busca', rotulo: `“${f.busca.trim()}”` });
  if (f.ids?.length) {
    const n = f.ids.length;
    const quantos = `${n} ${n === 1 ? 'lançamento' : 'lançamentos'}`;
    const escolhidos = n === 1 ? 'Só 1 lançamento escolhido' : `Só ${n} lançamentos escolhidos`;
    chips.push({ id: 'ids', rotulo: origemIds ? `${ORIGENS_IDS[origemIds]}: ${quantos}` : escolhidos });
  }
  return chips;
}

/**
 * O ✕ de um chip: o filtro sem aquele valor (o resto continua), e as categorias marcadas sem ele. O chip de ids tira
 * também a origem (quem chama limpa `origemIds`).
 */
export function semOChip(
  f: FiltroTransacoes, categorias: string[], id: ChipFiltro['id'], padrao: FiltroTransacoes
): { filtro: FiltroTransacoes; categorias: string[] } {
  const filtro: FiltroTransacoes = { ...f };
  let marcadas = categorias;
  if (id.startsWith('natureza:')) {
    const natureza = NATUREZAS_URL[id.substring('natureza:'.length) as NaturezaUrl].natureza;
    if (natureza === null) filtro.semNatureza = false;
    else filtro.naturezas = (filtro.naturezas ?? []).filter((n) => n !== natureza);
  } else if (id.startsWith('categoria:')) {
    const valor = id.substring('categoria:'.length);
    marcadas = categorias.filter((v) => v !== valor);
    Object.assign(filtro, camposDeCategorias(marcadas));
  }
  switch (id) {
    case 'periodo': filtro.inicio = padrao.inicio; filtro.fim = padrao.fim; break;
    case 'conta': filtro.contaId = null; break;
    case 'tipo': filtro.tipo = null; break;
    case 'emergencias': filtro.somenteEmergencias = false; break;
    case 'generica': filtro.poucoInformativa = false; break;
    case 'lancamento': filtro.lancamento = null; break;
    case 'busca': filtro.busca = ''; break;
    case 'ids': filtro.ids = []; break;
  }
  return { filtro, categorias: marcadas };
}

/* ------------------------------ #244: ordenar pelo cabeçalho ------------------------------ */

export function ordemDe(f: FiltroTransacoes): OrdemTransacoes {
  return f.ordem ?? 'data';
}

export function direcaoDe(f: FiltroTransacoes): 'asc' | 'desc' {
  return f.direcao ?? DIRECAO_PADRAO[f.ordem ?? 'data'];
}

/** Só a coluna que vale tem aria-sort (WAI-ARIA APG, Sortable Table). */
export function ariaSortDe(f: FiltroTransacoes, coluna: OrdemTransacoes): 'ascending' | 'descending' | null {
  if (ordemDe(f) !== coluna) return null;
  return direcaoDe(f) === 'asc' ? 'ascending' : 'descending';
}

/** ▲ crescente, ▼ decrescente; ↕ apagado nas outras, para mostrar que dá para ordenar (Nielsen 1). */
export function setaDe(f: FiltroTransacoes, coluna: OrdemTransacoes): string {
  if (ordemDe(f) !== coluna) return '↕';
  return direcaoDe(f) === 'asc' ? '▲' : '▼';
}

/** Clicar na coluna ordena por ela (na direção mais útil dela); clicar de novo inverte. */
export function direcaoAoClicar(f: FiltroTransacoes, coluna: OrdemTransacoes): 'asc' | 'desc' {
  if (ordemDe(f) !== coluna) return DIRECAO_PADRAO[coluna];
  return direcaoDe(f) === 'asc' ? 'desc' : 'asc';
}

/* ------------------------------ #133: seleção ------------------------------ */

/** "5 selecionadas (2 em outras páginas)": deixa claro que a ação em lote vale também para as que não aparecem. */
export function textoSelecao(total: number, naPagina: number): string {
  if (!total) return '';
  const fora = total - naPagina;
  return `${total} ${total === 1 ? 'selecionada' : 'selecionadas'}`
    + (fora ? ` (${fora} em ${fora === 1 ? 'outra página' : 'outras páginas'})` : '');
}

/**
 * #133: marca ou desmarca a linha `indice`. Com Shift, o mesmo estado vale para o intervalo entre a última caixa usada
 * (`ultimo`) e esta, como no Gmail e no Explorador de Arquivos.
 */
export function marcarIntervalo(
  selecionadas: ReadonlySet<number>, itens: readonly { id: number }[], indice: number, ultimo: number | null, comShift: boolean,
  marcar: boolean
): Set<number> {
  const de = comShift && ultimo !== null ? Math.min(ultimo, indice) : indice;
  const ate = comShift && ultimo !== null ? Math.max(ultimo, indice) : indice;
  const novo = new Set(selecionadas);
  for (const t of itens.slice(de, ate + 1)) {
    if (marcar) novo.add(t.id); else novo.delete(t.id);
  }
  return novo;
}

/** "Selecionar todas desta página": parcial ou nenhuma → marca todas as da página; todas → desmarca as da página. */
export function alternarPagina(selecionadas: ReadonlySet<number>, itens: readonly { id: number }[]): Set<number> {
  const todas = itens.length > 0 && itens.every((t) => selecionadas.has(t.id));
  const novo = new Set(selecionadas);
  for (const t of itens) { if (todas) novo.delete(t.id); else novo.add(t.id); }
  return novo;
}

/** Nome acessível da caixa de cada linha: "Selecionar: PIX ENVIADO JOAO, 12/10/2026, R$ 150,00" (respeita "ocultar valores"). */
export function nomeSelecao(t: Transacao, valorOculto: boolean): string {
  const data = t.data ? paraData(t.data.slice(0, 10)).toLocaleDateString('pt-BR') : '';
  const valor = valorOculto
    ? 'valor oculto'
    : Math.abs(t.valor ?? 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
  return 'Selecionar: ' + [t.descricao, data, valor].filter(Boolean).join(', ');
}

/* ------------------------------ o que conta nos totais ------------------------------ */

/** Não entra nas despesas nem nas receitas (transferência ou "ignorar nos relatórios"). */
export function foraDosGastos(t: Transacao): boolean {
  return t.tipo === TipoTransacao.Transferencia || t.ignorarRelatorios;
}

/**
 * Pagamento de fatura ou transferência entre contas suas que está entrando nos totais: na saída, conta o gasto em
 * dobro (as compras já estão no cartão); na entrada (o crédito do pagamento no cartão), infla as receitas.
 */
export function somandoNosTotais(t: Transacao): 'gastos' | 'receitas' | null {
  if (!NAO_E_GASTO.has(t.tipoLancamento) || t.tipo === TipoTransacao.Transferencia || t.ignorarRelatorios) return null;
  return t.valor < 0 ? 'gastos' : 'receitas';
}

/* ------------------------------ #87: modo edição ------------------------------ */

/** Valor do select da linha: a troca pendente ou a categoria atual ('' = sem categoria). */
export function valorCategoria(t: Transacao, pendentes: ReadonlyMap<number, EscolhaCategoria>): string {
  return pendentes.get(t.id) ?? (t.categoriaId === null ? '' : String(t.categoriaId));
}

/** As trocas com a escolha `valor` na linha `t`; voltar para o que já estava gravado tira a pendência. */
export function comEscolha(pendentes: ReadonlyMap<number, EscolhaCategoria>, t: Transacao, valor: string): Map<number, EscolhaCategoria> {
  const atual = t.categoriaId === null ? '' : String(t.categoriaId);
  const novo = new Map(pendentes);
  if (valor === atual || valor === '') novo.delete(t.id);
  else novo.set(t.id, valor as EscolhaCategoria);
  return novo;
}

/** As trocas como a API as recebe em "Salvar alterações". */
export function itensDoLote(pendentes: ReadonlyMap<number, EscolhaCategoria>): { id: number; categoriaId: number | null; automatica: boolean }[] {
  return [...pendentes].map(([id, v]) => ({ id, categoriaId: v === 'auto' ? null : Number(v), automatica: v === 'auto' }));
}

/**
 * Várias transações com a mesma descrição foram para a mesma categoria: sugere regra (como nas Sugestões). Até 3, só
 * com 2 ou mais transações e texto de 4 letras ou mais; usa a descrição original do banco quando há.
 */
export function regrasSugeridas(
  itens: readonly Transacao[], pendentes: ReadonlyMap<number, EscolhaCategoria>, categorias: Categoria[]
): SugestaoRegra[] {
  const grupos = new Map<string, SugestaoRegra>();
  for (const t of itens) {
    const escolha = pendentes.get(t.id);
    if (!escolha || escolha === 'auto') continue;
    const texto = (t.descricaoOriginal ?? t.descricao).trim().replace(/\s+/g, ' ').slice(0, 100);
    if (texto.length < 4) continue;
    const chave = `${texto.toLocaleUpperCase('pt-BR')}|${escolha}`;
    const g = grupos.get(chave);
    if (g) g.quantidade++;
    else grupos.set(chave, { texto, categoriaId: Number(escolha), categoriaNome: rotuloCategoriaPorId(Number(escolha), categorias), quantidade: 1 });
  }
  return [...grupos.values()].filter((g) => g.quantidade >= 2).slice(0, 3);
}

/* ------------------------------ sugestões de categoria ------------------------------ */

/**
 * As linhas do assistente, com a categoria sugerida já escolhida. "Criar regra" vem marcado só com confiança alta,
 * 3 ou mais transações, texto de 4 letras ou mais e sem aviso; #254: o mesmo texto com categorias diferentes em linhas
 * diferentes não vem marcado em nenhuma.
 */
export function linhasDeSugestao(sugestoes: readonly SugestaoCategoria[]): LinhaSugestao[] {
  const categoriasPorTexto = new Map<string, Set<number | null>>();
  for (const x of sugestoes) {
    if (!x.textoRegra) continue;
    const chave = x.textoRegra.toUpperCase();
    categoriasPorTexto.set(chave, (categoriasPorTexto.get(chave) ?? new Set()).add(x.categoriaId));
  }
  return sugestoes.map((x) => ({
    ...x, escolhida: x.categoriaId, aplicada: false,
    criarRegra: x.confianca === 'alta' && x.quantidade >= 3 && x.textoRegra.length >= 4 && !x.avisoRegra
      && (categoriasPorTexto.get(x.textoRegra.toUpperCase())?.size ?? 0) <= 1
  }));
}

export function nomeConfianca(c: string): string {
  const nomes: Record<string, string> = {
    alta: 'confiança alta', media: 'confiança média', baixa: 'palavra-chave', nenhuma: 'sem sugestão'
  };
  return nomes[c] ?? c;
}

/**
 * #254: o nome da outra categoria quando a regra desta linha ("contém X") também é pedida, com categoria diferente,
 * por outra linha marcada ou já foi criada nesta janela (`regrasCriadas`: texto em maiúsculas → categoria). Duas
 * regras iguais com categorias diferentes: só uma vale.
 */
export function conflitoRegra(
  l: LinhaSugestao, linhas: readonly LinhaSugestao[], regrasCriadas: ReadonlyMap<string, number>, categorias: Categoria[]
): string | null {
  if (!l.criarRegra || !l.textoRegra) return null;
  const nome = (id: number | null) => categorias.find((c) => c.id === id)?.nome ?? 'outra categoria';
  const texto = l.textoRegra.trim().toUpperCase();
  const criada = regrasCriadas.get(texto);
  if (criada !== undefined && criada !== l.escolhida) return nome(criada);
  const outra = linhas.find((x) => x.chave !== l.chave && !x.aplicada && x.criarRegra
    && x.textoRegra.trim().toUpperCase() === texto && x.escolhida !== l.escolhida);
  return outra ? nome(outra.escolhida) : null;
}

/* ------------------------------ formulário ------------------------------ */

/** Transação nova: despesa de hoje (`hoje` em yyyy-MM-dd) na primeira conta manual. */
export function formNovo(contaId: number | null, hoje: string): FormTransacao {
  return {
    id: null, openFinance: false, contaId, data: hoje, descricao: '', valor: null, tipo: TipoTransacao.Despesa,
    categoriaId: null, observacao: '', ignorarRelatorios: false, pendente: false, descricaoOriginal: null, emergencia: false
  };
}

/** O formulário com a transação: o valor sem sinal e o tipo pelo sinal. */
export function formDaTransacao(t: Transacao): FormTransacao {
  return {
    id: t.id, openFinance: t.openFinance, contaId: t.contaId, data: t.data, descricao: t.descricao,
    valor: Math.abs(t.valor), tipo: t.valor > 0 ? TipoTransacao.Receita : TipoTransacao.Despesa,
    categoriaId: t.categoriaId, observacao: t.observacao ?? '', ignorarRelatorios: t.ignorarRelatorios,
    pendente: t.status === StatusTransacao.Pendente, descricaoOriginal: t.descricaoOriginal, emergencia: t.emergencia
  };
}

/** Transação do Open Finance: só o que dá para ajustar (o resto vem do banco). Emergência só vale em despesa. */
export function ajusteDoForm(f: FormTransacao): AjusteTransacao {
  return {
    descricao: f.descricao, categoriaId: f.categoriaId, observacao: f.observacao || null,
    ignorarRelatorios: f.ignorarRelatorios, emergencia: f.emergencia && f.tipo === TipoTransacao.Despesa
  };
}

/** Transação manual, nova ou editada. Emergência só vale em despesa. */
export function transacaoDoForm(f: FormTransacao): SalvarTransacao {
  return {
    contaId: f.contaId ?? 0, data: f.data, descricao: f.descricao, valor: Number(f.valor), tipo: f.tipo,
    categoriaId: f.categoriaId, observacao: f.observacao || null, ignorarRelatorios: f.ignorarRelatorios,
    status: f.pendente ? StatusTransacao.Pendente : StatusTransacao.Efetivada,
    emergencia: f.emergencia && f.tipo === TipoTransacao.Despesa
  };
}
