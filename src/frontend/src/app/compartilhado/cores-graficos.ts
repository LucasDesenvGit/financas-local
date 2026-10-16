/**
 * Cores com significado nos gráficos: a mesma cor quer dizer a mesma coisa em todas as telas (Nielsen 4; UX-9).
 * Receita é sempre verde, despesa sempre rosa e emergência sempre laranja; nenhuma outra série usa essas cores.
 */
export const CORES_GRAFICO = {
  receita: '#8fd5b4',
  receitaAnterior: '#cfe9dc',
  despesa: '#f5a3b0',
  despesaAnterior: '#f9d3da',
  emergencia: '#f6a467',
  saldo: '#a7a3f5',
  /** Comparação com a Selic (linhas tracejadas e barras de referência). */
  selic: '#f8c09a',
  financiamento: '#8fa8f0',
  /** Dourado: não pode ser o laranja de emergência. */
  emprestimo: '#d9b45a',
  /** Ganhou/perdeu da referência (ex.: resultado × Selic, cotação × preço médio). */
  ganhou: '#8fd5b4',
  perdeu: '#f5a3b0'
} as const;
