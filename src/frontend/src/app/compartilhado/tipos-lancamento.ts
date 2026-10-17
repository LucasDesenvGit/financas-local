import { LucideIconData } from 'lucide-angular';
import { TipoLancamento } from '../core/modelos';
import { Icones } from './icones';

/**
 * #111 (ADR 0025): rótulo e ícone de cada tipo de lançamento, na linguagem de quem usa (Nielsen 2): "Pix enviado",
 * não "DEBIT". O ícone é decorativo; o texto sempre aparece junto (WCAG 1.1.1 e 1.4.1).
 */
export const TIPOS_LANCAMENTO: Record<TipoLancamento, { rotulo: string; icone: LucideIconData }> = {
  [TipoLancamento.Outro]: { rotulo: 'Outro', icone: Icones.outro },
  [TipoLancamento.CompraCartao]: { rotulo: 'Compra no cartão', icone: Icones.contas },
  [TipoLancamento.EstornoCartao]: { rotulo: 'Estorno no cartão', icone: Icones.estorno },
  [TipoLancamento.PagamentoFatura]: { rotulo: 'Pagamento de fatura', icone: Icones.pagamentoFatura },
  [TipoLancamento.PixEnviado]: { rotulo: 'Pix enviado', icone: Icones.pixEnviado },
  [TipoLancamento.PixRecebido]: { rotulo: 'Pix recebido', icone: Icones.pixRecebido },
  [TipoLancamento.TransferenciaEnviada]: { rotulo: 'Transferência enviada', icone: Icones.pixEnviado },
  [TipoLancamento.TransferenciaRecebida]: { rotulo: 'Transferência recebida', icone: Icones.pixRecebido },
  [TipoLancamento.EntreContas]: { rotulo: 'Entre suas contas', icone: Icones.entreContas },
  [TipoLancamento.Boleto]: { rotulo: 'Boleto ou débito automático', icone: Icones.boleto },
  [TipoLancamento.CompraDebito]: { rotulo: 'Compra no débito', icone: Icones.compraDebito },
  [TipoLancamento.Saque]: { rotulo: 'Saque', icone: Icones.dinheiro },
  [TipoLancamento.Deposito]: { rotulo: 'Depósito', icone: Icones.dinheiro },
  [TipoLancamento.TarifaJuros]: { rotulo: 'Tarifa ou juros', icone: Icones.tarifa },
  [TipoLancamento.Rendimento]: { rotulo: 'Rendimento', icone: Icones.investimentos },
  [TipoLancamento.AplicacaoResgate]: { rotulo: 'Aplicação ou resgate', icone: Icones.marca },
  [TipoLancamento.Salario]: { rotulo: 'Salário', icone: Icones.salario },
  [TipoLancamento.Credito]: { rotulo: 'Empréstimo ou crédito', icone: Icones.emprestimos },
  [TipoLancamento.Dinheiro]: { rotulo: 'Dinheiro', icone: Icones.patrimonio }
};

/** Opções do filtro "Lançamento" (o código vai na URL, ?lancamento=pix; os mesmos grupos da API). */
export const GRUPOS_LANCAMENTO: { valor: string; rotulo: string }[] = [
  { valor: 'cartao', rotulo: 'Compras no cartão' },
  { valor: 'debito', rotulo: 'Compras no débito' },
  { valor: 'pix', rotulo: 'Pix' },
  { valor: 'transferencia', rotulo: 'Transferências (TED/DOC)' },
  { valor: 'entre-contas', rotulo: 'Entre suas contas' },
  { valor: 'fatura', rotulo: 'Pagamento de fatura' },
  { valor: 'boleto', rotulo: 'Boletos e débitos automáticos' },
  { valor: 'saque-deposito', rotulo: 'Saques e depósitos' },
  { valor: 'tarifas', rotulo: 'Tarifas e juros' },
  { valor: 'investimentos', rotulo: 'Rendimentos e aplicações' },
  { valor: 'salario', rotulo: 'Salário' },
  { valor: 'credito', rotulo: 'Empréstimos e crédito' },
  { valor: 'dinheiro', rotulo: 'Dinheiro' },
  { valor: 'outros', rotulo: 'Outros' }
];

/** Tipos que mudam dinheiro de lugar sem ser gasto (a compra já entrou no cartão; o dinheiro só trocou de conta). */
export const NAO_E_GASTO = new Set<TipoLancamento>([TipoLancamento.PagamentoFatura, TipoLancamento.EntreContas]);
