/**
 * Lógica da tela de Investimentos em funções puras (#731, ADR 0059): aba na URL, posições e totais, frases-resumo e
 * gráficos, formulários (operação, ativo, conferência), importação pelo extrato e as mensagens dos avisos. Sem Angular:
 * o componente busca os dados, guarda o estado e chama estas funções; os testes do Jest as chamam direto, sem montar a
 * tela (#204).
 */
import type { ChartConfiguration } from 'chart.js';
import { CORES_GRAFICO } from '../../compartilhado/graficos/cores-graficos';
import { pct as pctCurto, reaisOuNada } from '../../compartilhado/utilidades';
import {
  AtualizarAtivo, Carteira, EvolucaoCarteira, ImportarExtratoItem, NOMES_ORIGEM_OPERACAO, NOMES_TIPO_ATIVO, OperacaoAtivo,
  OrigemOperacao, PosicaoAtivo, Reserva, ResultadoCotacoes, ResultadoImportacaoCarteira, SalvarOperacao, SugestaoExtrato,
  TipoAtivo, TipoOperacao
} from '../../core/modelos';

/* ------------------------------ tipos da tela ------------------------------ */

export type AbaInvestimentos = 'resumo' | 'fii' | 'rv' | 'of' | 'reservas';

const ABAS: AbaInvestimentos[] = ['resumo', 'fii', 'rv', 'of', 'reservas'];

/** O que a função precisa da URL: o `ParamMap` do Angular serve, e no teste um objeto com `get`. */
export interface ParametrosUrl { get(nome: string): string | null }

/** Formulário de conferência do saldo real de uma reserva. */
/** #169: `moeda` só conta no Méli Dólar (o saldo pode ser informado em reais ou em dólares). */
export interface FormConferencia { reserva: Reserva; saldo: number | null; data: string; moeda: 'BRL' | 'USD'; registrarDiferenca: boolean }

/** Linha editável da importação pelo extrato. */
export interface LinhaImportacao extends SugestaoExtrato { selecionada: boolean; taxas: number }

/** A PTAX que a tela sugere para o câmbio de uma operação em moeda estrangeira. */
export interface PtaxSugerida { data: string; valor: number; tipo: 'compra' | 'venda' }

/** Uma mensagem para o aviso da tela: erro ou sucesso. */
export interface MensagemAviso { tipo: 'erro' | 'sucesso'; texto: string }

/* ------------------------------ URL ------------------------------ */

/** #89: `/investimentos?aba=reservas` abre direto numa aba (link "confira" do cofrinho em Metas); sem aba válida, o Resumo. */
export function abaDaUrl(url: ParametrosUrl): AbaInvestimentos {
  const aba = url.get('aba');
  return ABAS.find((a) => a === aba) ?? 'resumo';
}

/** #170: o aviso de trocos do Painel abre direto em "Conferir investimentos" (`?conferir=1`). */
export function abreConferirNaUrl(url: ParametrosUrl): boolean {
  return url.get('conferir') === '1';
}

/* ------------------------------ nomes e formatação ------------------------------ */

export const NOMES_TIPO_PLUGGY: Record<string, string> = {
  FIXED_INCOME: 'Renda fixa',
  MUTUAL_FUND: 'Fundos',
  SECURITY: 'Previdência',
  EQUITY: 'Ações',
  ETF: 'ETFs',
  COE: 'COE',
  OTHER: 'Outros'
};

export const SEGMENTOS = [
  'Logística', 'Shoppings', 'Lajes corporativas', 'Papel (recebíveis)', 'Híbrido', 'Fundo de fundos',
  'Renda urbana', 'Agronegócio', 'Residencial', 'Hotéis', 'Hospitais/Educação', 'Desenvolvimento', 'Outros'
];

export const TIPOS_ATIVO = [TipoAtivo.Fii, TipoAtivo.Acao, TipoAtivo.Etf, TipoAtivo.Bdr, TipoAtivo.Outro];

/** FIIs e BDRs são da B3: ativo no exterior só pode ser ação, ETF ou outro. */
export function tiposPara(moeda: string | null | undefined): TipoAtivo[] {
  return !moeda || moeda === 'BRL' ? TIPOS_ATIVO : TIPOS_ATIVO.filter((t) => t !== TipoAtivo.Fii && t !== TipoAtivo.Bdr);
}

/** "GARE11, XPLG11 e XPML11". */
export function listaTickers(tickers: string[]): string {
  return tickers.length > 1 ? `${tickers.slice(0, -1).join(', ')} e ${tickers[tickers.length - 1]}` : tickers[0] ?? '';
}

/** "+1,50%" (com sinal só no positivo, se pedido); "—" sem valor. */
export function percentual(valor: number | null | undefined, sinal = true): string {
  if (valor === null || valor === undefined) return '—';
  const texto = valor.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return `${sinal && valor > 0 ? '+' : ''}${texto}%`;
}

/** A quantidade de cotas, ou "•••" com "Ocultar valores" ligado. */
export function quantidade(valor: number, oculto: boolean): string {
  if (oculto) return '•••';
  return valor.toLocaleString('pt-BR', { maximumFractionDigits: 6 });
}

export function nomeTipo(t: TipoAtivo): string {
  return NOMES_TIPO_ATIVO[t] ?? 'Outro';
}

export function nomeOrigem(o: OrigemOperacao): string {
  return NOMES_ORIGEM_OPERACAO[o] ?? '';
}

export function nomeTipoPluggy(t: string, subtipo: string | null = null, codigo: string | null = null): string {
  if (t?.toUpperCase() === 'EQUITY' && (subtipo?.toUpperCase() === 'REAL_ESTATE_FUND' || /^[A-Z]{4}1[123]$/.test(codigo ?? ''))) {
    return 'FII';
  }
  return NOMES_TIPO_PLUGGY[t] ?? t;
}

/* ------------------------------ posições e totais ------------------------------ */

export function posicoesAbertas(posicoes: PosicaoAtivo[] | undefined): PosicaoAtivo[] {
  return (posicoes ?? []).filter((p) => p.quantidade > 0);
}

export function posicoesParaMostrar(posicoes: PosicaoAtivo[] | undefined, mostrarEncerradas: boolean): PosicaoAtivo[] {
  return mostrarEncerradas ? posicoes ?? [] : posicoesAbertas(posicoes);
}

export function quantidadeDeFiis(posicoes: PosicaoAtivo[] | undefined): number {
  return posicoesAbertas(posicoes).filter((p) => p.tipo === TipoAtivo.Fii).length;
}

/** Reservas que pedem conferência: pode haver saldo de antes do 1º lançamento, ou a última conferência é antiga. */
export function contarReservasAConferir(reservas: Reserva[] | undefined): number {
  return (reservas ?? []).filter((r) => r.podeTerSaldoAnterior || r.conferenciaAntiga).length;
}

/** Uma barra por ativo aberto, 36 px cada, com o mínimo de 220. */
export function alturaGraficoAtivos(abertas: number): number {
  return Math.max(220, abertas * 36);
}

/* ------------------------------ UX-9: frases-resumo (sem R$ quando "ocultar valores" está ligado) ------------------------------ */

/** Valor de mercado hoje × o mesmo dinheiro na Selic (último mês com valor de mercado). */
export function resumoEvolucao(evolucao: EvolucaoCarteira[] | undefined, oculto: boolean): string | null {
  const ev = (evolucao ?? []).filter((m) => m.valorMercado !== null);
  const ultimo = ev[ev.length - 1];
  if (!ultimo || ultimo.aplicadoNaSelic <= 0) return null;
  const dif = ((ultimo.valorMercado! - ultimo.aplicadoNaSelic) / ultimo.aplicadoNaSelic) * 100;
  if (Math.abs(dif) < 0.5) return 'Sua renda variável vale hoje praticamente o mesmo que esse dinheiro teria na Selic.';
  const valores = oculto ? '' : ` (${reaisOuNada(ultimo.valorMercado!, false)} contra ${reaisOuNada(ultimo.aplicadoNaSelic, false)})`;
  return `Sua renda variável vale hoje ${pctCurto(dif)} ${dif > 0 ? 'mais' : 'menos'} do que esse dinheiro teria na Selic${valores}.`;
}

export function resumoDistribuicao(d: Carteira | null): string | null {
  const dist = [...(d?.distribuicao ?? [])].sort((a, b) => b.valor - a.valor);
  if (!d || !dist.length || d.totalGeral <= 0) return null;
  return `${dist[0].nome} concentra ${pctCurto(dist[0].valor / d.totalGeral * 100)} dos seus investimentos.`;
}

export function resumoAtivos(abertas: PosicaoAtivo[]): string | null {
  const ativos = abertas.filter((p) => p.resultadoTotalPercentual !== null && p.resultadoSelicPercentual !== null);
  if (!ativos.length) return null;
  const ganharam = ativos.filter((p) => p.resultadoTotalPercentual! >= p.resultadoSelicPercentual!).length;
  if (ganharam === ativos.length) return ativos.length === 1 ? 'Seu ativo rendeu mais que a Selic no mesmo período.'
    : `Todos os ${ativos.length} ativos renderam mais que a Selic no mesmo período.`;
  if (ganharam === 0) return 'Nenhum ativo rendeu mais que a Selic no mesmo período.';
  return `${ganharam} de ${ativos.length} ativos renderam mais que a Selic no mesmo período.`;
}

/* ------------------------------ gráficos ------------------------------ */

// Paleta dos tipos de investimento: sem o verde/rosa de ganhou/perdeu nem o laranja da Selic (UX-9).
const CORES = ['#a7a3f5', '#92c8f0', '#c3b1e1', '#8fd8d8', '#f3b3d3', '#f2d98b', '#a9b4c6', '#b8c7f0'];
const COR_CARTEIRA = '#a7a3f5';
const COR_SELIC = CORES_GRAFICO.selic;
const COR_MERCADO = '#8fd5b4';
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
/** Símbolo curto para rótulos de campo (o pipe de moeda já formata os valores). */
const compacto = (v: string | number) => Number(v).toLocaleString('pt-BR', { notation: 'compact' });

export function corDoTipo(i: number): string {
  return CORES[i % CORES.length];
}

export function configEvolucao(evolucao: EvolucaoCarteira[] | undefined): ChartConfiguration<'line'> {
  const ev = evolucao ?? [];
  return {
    type: 'line',
    data: {
      labels: ev.map((m) => `${MESES[m.mes - 1]}/${String(m.ano).slice(2)}`),
      datasets: [
        {
          label: 'Aplicado (compras − vendas)', data: ev.map((m) => m.aplicado), borderColor: COR_CARTEIRA,
          backgroundColor: COR_CARTEIRA + '33', fill: true, tension: 0.3, borderWidth: 2.5, pointRadius: 0
        },
        {
          label: 'Mesmo dinheiro na Selic', data: ev.map((m) => m.aplicadoNaSelic), borderColor: COR_SELIC,
          borderDash: [6, 5], tension: 0.3, borderWidth: 2, pointRadius: 0
        },
        {
          label: 'Valor de mercado hoje', data: ev.map((m) => m.valorMercado), borderColor: COR_MERCADO,
          backgroundColor: COR_MERCADO, pointRadius: 6, pointHoverRadius: 8, showLine: false
        }
      ]
    },
    options: {
      interaction: { mode: 'index', intersect: false },
      plugins: { legend: { position: 'bottom' } },
      scales: { y: { ticks: { callback: compacto } } }
    }
  };
}

export function configDistribuicao(distribuicao: { nome: string; valor: number }[] | undefined): ChartConfiguration<'doughnut'> {
  const dist = distribuicao ?? [];
  return {
    type: 'doughnut',
    data: {
      labels: dist.map((g) => g.nome),
      datasets: [{ data: dist.map((g) => g.valor), backgroundColor: dist.map((_, i) => corDoTipo(i)), borderWidth: 0 }]
    },
    options: { cutout: '72%', plugins: { legend: { display: false } } }
  };
}

export function configAtivos(ativos: PosicaoAtivo[]): ChartConfiguration<'bar'> {
  return {
    type: 'bar',
    data: {
      labels: ativos.map((p) => p.ticker),
      datasets: [
        {
          label: 'Seu resultado (%)', data: ativos.map((p) => p.resultadoTotalPercentual ?? 0),
          backgroundColor: ativos.map((p) =>
            (p.resultadoTotalPercentual ?? 0) >= (p.resultadoSelicPercentual ?? 0) ? CORES_GRAFICO.ganhou : CORES_GRAFICO.perdeu),
          borderRadius: 6, borderSkipped: false
        },
        {
          label: 'Selic no mesmo período (%)', data: ativos.map((p) => p.resultadoSelicPercentual ?? 0),
          backgroundColor: COR_SELIC, borderRadius: 6, borderSkipped: false
        }
      ]
    },
    options: {
      indexAxis: 'y',
      plugins: { legend: { position: 'bottom' } },
      scales: { x: { ticks: { callback: (v) => `${v}%` } } }
    }
  };
}

/* ------------------------------ avisos das chamadas ------------------------------ */

/** Depois de atualizar as cotações: o erro por ativo sem cotação; o sucesso só quando não foi a atualização automática. */
export function mensagemCotacoes(r: ResultadoCotacoes, silencioso: boolean): MensagemAviso | null {
  if (r.falhas.length) {
    return { tipo: 'erro', texto: `Sem cotação para ${r.falhas.join(', ')}. Tente de novo mais tarde ou informe a cotação manualmente.` };
  }
  if (silencioso) return null;
  const partes = [`${r.atualizados} cotação(ões) atualizada(s)`];
  if (r.proventosNovos) partes.push(`${r.proventosNovos} provento(s) novo(s)`);
  return { tipo: 'sucesso', texto: partes.join(' · ') };
}

/** Depois de registrar as compras do Open Finance: erro se falhou algo; sucesso, calado quando foi automático e sem nada novo. */
export function mensagemImportacaoOpenFinance(r: ResultadoImportacaoCarteira, automatico: boolean): MensagemAviso | null {
  if (r.mensagem) return { tipo: 'erro', texto: r.mensagem };
  const partes = [`${r.operacoesImportadas} compra(s)/venda(s) registrada(s)`];
  if (r.ativosCriados) partes.push(`${r.ativosCriados} ativo(s) novo(s)`);
  if (r.semHistorico.length) {
    partes.push(`sem histórico na corretora: ${r.semHistorico.join(', ')} (registrado o saldo atual; confira data e preço em Operações)`);
  }
  if (r.comLancamentoManual.length) partes.push(`ignorados por já terem lançamentos seus: ${r.comLancamentoManual.join(', ')}`);
  if (r.erros.length) partes.push(`falha em ${r.erros.join(', ')}`);
  if (r.erros.length) return { tipo: 'erro', texto: partes.join(' · ') };
  if (!automatico || r.operacoesImportadas) return { tipo: 'sucesso', texto: partes.join(' · ') };
  return null;
}

/* ------------------------------ operação (compra ou venda) ------------------------------ */

/** O formulário de uma operação nova; com o ticker de um ativo já lançado, a moeda vem dele. */
export function formularioNovaOperacao(ticker: string, moedaDoAtivo: string | undefined, hoje: string): SalvarOperacao {
  return {
    ticker, tipoAtivo: null, tipo: TipoOperacao.Compra, data: hoje, quantidade: null, precoUnitario: null,
    taxas: 0, observacao: null, moeda: moedaDoAtivo ?? 'BRL', cambio: null
  };
}

/** O formulário de uma operação já lançada; o câmbio de uma operação em reais não se mostra. */
export function formularioDaOperacao(o: OperacaoAtivo): SalvarOperacao {
  return {
    ticker: o.ticker, tipoAtivo: null, tipo: o.tipo, data: o.data, quantidade: o.quantidade,
    precoUnitario: o.precoUnitario, taxas: o.taxas, observacao: o.observacao, moeda: o.moeda,
    cambio: o.moeda === 'BRL' ? null : o.cambio
  };
}

/** Total na moeda do ativo: a compra soma as taxas, a venda desconta. */
export function totalOperacao(f: SalvarOperacao): number {
  const bruto = (f.quantidade ?? 0) * (f.precoUnitario ?? 0);
  return f.tipo === TipoOperacao.Compra ? bruto + (f.taxas ?? 0) : bruto - (f.taxas ?? 0);
}

/** O que vai para a API: ticker em maiúsculas, taxa e observação limpas, e câmbio só fora do real. */
export function operacaoParaEnviar(f: SalvarOperacao): SalvarOperacao {
  return {
    ...f, ticker: f.ticker.trim().toUpperCase(), taxas: f.taxas ?? 0, observacao: f.observacao?.trim() || null,
    cambio: f.moeda === 'BRL' ? null : f.cambio
  };
}

/** A PTAX de venda na compra e a de compra na venda (a ponta que a corretora costuma cobrar). */
export function ptaxDaOperacao(c: { data: string; compra: number; venda: number }, tipo: TipoOperacao): PtaxSugerida {
  const compra = tipo === TipoOperacao.Compra;
  return { data: c.data, valor: compra ? c.venda : c.compra, tipo: compra ? 'venda' : 'compra' };
}

/**
 * "Compra que faltou no Open Finance": as cotas que a corretora informa e a carteira não tem, e a observação. A tela
 * aplica em cima do formulário que já abriu (o mesmo objeto, que a busca da PTAX confere ao chegar).
 */
export function dadosDaCompraQueFaltou(quantidadeQueFaltou: number): Pick<SalvarOperacao, 'quantidade' | 'observacao'> {
  return { quantidade: quantidadeQueFaltou, observacao: 'Compra que faltou no Open Finance' };
}

/* ------------------------------ ativo ------------------------------ */

export function formularioDoAtivo(p: PosicaoAtivo): AtualizarAtivo {
  return { nome: p.nome, tipo: p.tipo, segmento: p.segmento, precoManual: p.precoManual, precoAtual: p.precoAtual };
}

export function ativoParaEnviar(f: AtualizarAtivo): AtualizarAtivo {
  return { ...f, nome: f.nome?.trim() || null };
}

/* ------------------------------ reserva ------------------------------ */

/** #169: o Méli Dólar começa em reais (o valor do app), com a estimativa atual. */
export function formularioConferencia(r: Reserva, hoje: string): FormConferencia {
  return r.moeda === 'USD'
    ? { reserva: r, saldo: r.saldo, data: hoje, moeda: 'BRL', registrarDiferenca: false }
    : { reserva: r, saldo: r.saldoConferido ?? r.saldo, data: hoje, moeda: 'BRL', registrarDiferenca: false };
}

/**
 * O saldo informado, em reais, para comparar com o esperado. Em dólar sem cotação conhecida, null: não dá para comparar.
 */
export function saldoInformadoEmReais(r: Reserva, moedaInformada: 'BRL' | 'USD', saldo: number): number | null {
  const emDolar = r.moeda === 'USD';
  return emDolar && moedaInformada === 'USD' ? (r.cotacao ? saldo * r.cotacao : null) : saldo;
}

const emReais = (v: number) => v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });

/** O texto de "Conferir o saldo" quando o valor informado está muito longe do esperado (#168). */
export function perguntaDeConferenciaForaDoEsperado(r: Reserva, saldo: number, esperado: number): string {
  return `Você informou ${emReais(saldo)}, mas o esperado para "${r.nome}" é perto de ${emReais(esperado)}. Confere com o app da instituição?`;
}

/* ------------------------------ importação pelo extrato ------------------------------ */

/** As sugestões do extrato viram linhas editáveis; só vem marcada a que não foi importada, não é duplicata e já tem cotas e preço. */
export function linhasDaImportacao(sugestoes: SugestaoExtrato[]): LinhaImportacao[] {
  return sugestoes.map((x) => ({
    ...x, taxas: 0,
    selecionada: !x.jaImportada && !x.possivelDuplicata && x.quantidade !== null && x.precoUnitario !== null
  }));
}

export function totalSelecionadas(linhas: LinhaImportacao[]): number {
  return linhas.filter((l) => l.selecionada).length;
}

/** As linhas que vão ser importadas: marcadas e ainda não importadas. */
export function linhasParaImportar(linhas: LinhaImportacao[]): LinhaImportacao[] {
  return linhas.filter((l) => l.selecionada && !l.jaImportada);
}

/** O aviso das linhas marcadas sem código, cotas ou preço; null quando estão todas completas. */
export function avisoDeLinhasIncompletas(selecionadas: LinhaImportacao[]): string | null {
  const invalidas = selecionadas.filter((l) => !l.ticker.trim() || !l.quantidade || l.quantidade <= 0 || l.precoUnitario === null);
  return invalidas.length ? `Preencha cotas e preço de ${invalidas.map((l) => l.ticker || 'sem código').join(', ')}.` : null;
}

/** As linhas completas no formato da API (as incompletas já foram barradas por `avisoDeLinhasIncompletas`). */
export function itensDaImportacao(selecionadas: LinhaImportacao[]): ImportarExtratoItem[] {
  return selecionadas.map((l) => ({
    transacaoId: l.transacaoId, ticker: l.ticker.trim().toUpperCase(), tipo: l.tipo,
    quantidade: l.quantidade!, precoUnitario: l.precoUnitario!, taxas: l.taxas ?? 0
  }));
}
