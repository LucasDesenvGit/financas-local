/**
 * Glossário único do site (UX-10): o mesmo termo tem a mesma definição em qualquer tela. Linguagem simples,
 * 1 a 2 frases, com exemplo quando ajuda (Nielsen 2: a linguagem de quem usa, não jargão; Nielsen 10: ajuda curta
 * no contexto). Para usar: <app-termo chave="dy">DY</app-termo>.
 */
export interface DefinicaoTermo {
  termo: string;
  definicao: string;
  exemplo?: string;
}

export const GLOSSARIO = {
  dy: {
    termo: 'DY (dividend yield)',
    definicao: 'Quanto um investimento pagou de proventos em relação ao preço da cota hoje, normalmente somando os últimos 12 meses.',
    exemplo: 'DY de 0,9% ao mês: a cada R$ 100 investidos pela cotação de hoje, R$ 0,90 de rendimento por mês.'
  },
  dyNaCompra: {
    termo: 'DY na compra',
    definicao: 'O DY que o fundo tinha quando você comprou: proventos dos 12 meses anteriores à compra divididos pelo preço que você pagou.'
  },
  yieldOnCost: {
    termo: 'Yield on cost',
    definicao: 'Quanto um investimento paga de proventos em relação ao preço que você pagou, e não à cotação de hoje.',
    exemplo: 'Comprou a cota por R$ 100 e ela pagou R$ 12 em 12 meses: yield on cost de 12% ao ano, mesmo que a cota hoje valha R$ 120.'
  },
  cet: {
    termo: 'CET (custo efetivo total)',
    definicao: 'O custo real de um empréstimo ou financiamento por ano: juros mais tarifas, seguros e impostos. É o número para comparar ofertas.'
  },
  selic: {
    termo: 'Selic',
    definicao: 'A taxa básica de juros do Brasil, definida pelo Banco Central. Aplicações de renda fixa simples (Tesouro Selic, CDB de liquidez diária) rendem perto dela, por isso serve de régua para outros investimentos.'
  },
  ifix: {
    termo: 'IFIX',
    definicao: 'O índice dos fundos imobiliários da bolsa: mostra como os FIIs em geral se saíram, para comparar com os seus.'
  },
  regra503020: {
    termo: 'Regra 50/30/20',
    definicao: 'Uma referência de divisão da renda: até 50% em gastos essenciais (moradia, mercado, saúde), até 30% em estilo de vida (lazer, compras) e pelo menos 20% guardados.'
  },
  taxaPoupanca: {
    termo: 'Taxa de poupança',
    definicao: 'A parte da renda que sobrou depois das despesas.',
    exemplo: 'Recebeu R$ 5.000 e gastou R$ 4.000: taxa de poupança de 20%.'
  },
  reservaEmergencia: {
    termo: 'Reserva de emergência',
    definicao: 'Dinheiro guardado com resgate rápido para imprevistos. Aqui é medida em meses: quantos meses você conseguiria pagar os seus gastos sem renda.'
  },
  comprometimento: {
    termo: 'Comprometimento com parcelas',
    definicao: 'A parte da renda mensal que já está presa a parcelas: compras parceladas no cartão, financiamentos e empréstimos.'
  },
  patrimonioSemRendaVariavel: {
    termo: 'Patrimônio sem renda variável',
    definicao: 'O patrimônio líquido sem contar ações, FIIs e ETFs, cujo valor sobe e desce todo dia. Mostra a parte mais estável do que você tem.'
  },
  dividaCartoes: {
    termo: 'Dívida total nos cartões',
    definicao: 'Tudo o que você deve nos cartões hoje, somando todos, inclusive parcelas que ainda vão vencer. Não é a fatura do mês.'
  },
  precoMedio: {
    termo: 'Preço médio',
    definicao: 'Quanto você pagou, em média, por cota, somando todas as compras.',
    exemplo: '10 cotas a R$ 100 e mais 10 a R$ 120: preço médio de R$ 110.'
  }
} satisfies Record<string, DefinicaoTermo>;

export type ChaveGlossario = keyof typeof GLOSSARIO;
