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
  tipoLancamento: {
    termo: 'Tipo de lançamento',
    definicao: 'Como o dinheiro entrou ou saiu: compra no cartão, Pix, transferência, pagamento de fatura, boleto… Vem do banco pelo Open Finance; quando ele não informa, é reconhecido pela descrição.',
    exemplo: 'Pagamento de fatura e transferência entre suas contas não são gasto novo: as compras já entraram no cartão, e o dinheiro só mudou de conta.'
  },
  dividaCartao: {
    termo: 'Dívida total do cartão',
    definicao: 'Tudo o que você deve neste cartão hoje, inclusive as parcelas que ainda vão vencer. Não é a fatura do mês.'
  },
  faturaDoMes: {
    termo: 'Fatura do mês',
    definicao: 'A fatura do cartão que fecha neste mês: compras desde o fechamento anterior e a parcela do mês das parceladas, menos estornos. Depois de fechada, vale o total informado pelo banco.'
  },
  melhorDiaCompra: {
    termo: 'Melhor dia de compra',
    definicao: 'O dia seguinte ao fechamento da fatura. O que você compra a partir dele só entra na fatura do mês seguinte, e você ganha quase 40 dias para pagar.'
  },
  dividaCartoes: {
    termo: 'Dívida total nos cartões',
    definicao: 'Tudo o que você deve nos cartões hoje, somando todos, inclusive parcelas que ainda vão vencer. Não é a fatura do mês.'
  },
  descricaoPoucoInformativa: {
    termo: 'Descrição pouco informativa',
    definicao: 'Descrição que não diz o que foi, como "PIX ENVIADO", "PAGAMENTO" ou só o nome do banco. As regras não conseguem categorizar essas transações sozinhas.',
    exemplo: 'Categorize, dê um nome na transação ou marque como revisada para ela sair da lista.'
  },
  sobraPrevista: {
    termo: 'Sobra prevista',
    definicao: 'Receitas médias menos despesas médias à vista e parcelas do mês: a mesma conta da tela de Projeção. No mês atual, o que já entrou e saiu mais o que falta pela média.'
  },
  sobraDoCaixa: {
    termo: 'Sobra do mês',
    definicao: 'O dinheiro que entra menos o que sai das suas contas no mês: receitas − fatura do cartão que vence no mês − outras despesas (Pix, débito, boletos). A compra no cartão conta quando a fatura vence, não no dia da compra.',
    exemplo: 'No mês atual, é o que já entrou e saiu mais a média do que falta. Por isso pode ser diferente da Projeção, que conta a compra no cartão no dia em que foi feita.'
  },
  parteDaSobra: {
    termo: 'Parte da sobra para as metas',
    definicao: 'Quanto da sobra do mês o assistente pode recomendar guardar; o resto fica livre para imprevistos. É a mesma escolha da "Margem de segurança" em Configurações › Parametrização, vista ao contrário (guardar 90% = margem de 10%).',
    exemplo: 'Sobra de R$ 1.350 guardando 90%: recomenda até R$ 1.215.'
  },
  recomendacaoMetas: {
    termo: 'Recomendação de quanto guardar',
    definicao: 'A sobra do mês vezes a parte que vai para as metas. Cada meta com prazo pede mais nos meses com folga e menos nos apertados (até 2× a divisão igual); o resto vai pela ordem de prioridade.',
    exemplo: 'Neste mês, nunca mais do que deve sobrar nas contas no fim do mês (já paga a fatura) nem do que sobrou no seu melhor mês recente; havendo sobra e saldo, pelo menos R$ 50. Dinheiro que vai para cofrinho ou investimento precisa estar como transferência: lançado como despesa, ele diminui a sobra.'
  },
  margemSeguranca: {
    termo: 'Margem de segurança',
    definicao: 'A parte da sobra do mês que o assistente de metas deixa livre para imprevistos antes de recomendar quanto guardar. Também dá para mudar na própria tela de Metas ("guardar X% da sobra").',
    exemplo: 'Sobra de R$ 1.350 com margem de 10%: recomenda guardar até R$ 1.215.'
  },
  precoMedio: {
    termo: 'Preço médio',
    definicao: 'Quanto você pagou, em média, por cota, somando todas as compras.',
    exemplo: '10 cotas a R$ 100 e mais 10 a R$ 120: preço médio de R$ 110.'
  },
  ptax: {
    termo: 'PTAX',
    definicao: 'A cotação oficial do dólar (e de outras moedas) que o Banco Central divulga todo dia útil. O site usa a de compra para passar para reais o que você tem em moeda estrangeira, como o Méli Dólar.',
    exemplo: 'US$ 10 com a PTAX a 5,05: R$ 50,50.'
  },
  trocoDeCompra: {
    termo: 'Troco da compra',
    definicao: 'Na compra de ações e FIIs, a corretora pode debitar um pouco a mais e devolver a sobra no mesmo dia ou nos seguintes. Esse crédito não é rendimento: é parte do dinheiro da compra que voltou.',
    exemplo: 'Débito de R$ 592,75 e crédito de R$ 3,60 no mesmo dia: a compra custou R$ 589,15.'
  }
} satisfies Record<string, DefinicaoTermo>;

export type ChaveGlossario = keyof typeof GLOSSARIO;
