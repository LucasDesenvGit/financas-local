/**
 * Ícones do site (Lucide, licença ISC — https://lucide.dev). Todos vêm do pacote npm e entram no build:
 * nada é baixado da internet (a CSP só permite recursos do próprio site). Para trocar um ícone, mude aqui.
 */
import {
  LucideArrowDownLeft, LucideArrowLeftRight, LucideArrowUpRight, LucideBanknote, LucideBookOpen, LucideBriefcase, LucideCalculator,
  LucideCalendar, LucideCalendarClock, LucideCalendarRange, LucideChartColumn, LucideChartLine, LucideChartNoAxesCombined,
  LucideChartPie, LucideCheck, LucideChevronDown, LucideChevronRight, LucideCircleEllipsis, LucideCircleQuestionMark,
  LucideCar, LucideClapperboard, LucideClipboardList, LucideCoins, LucideCreditCard, LucideEye, LucideEyeOff,
  LucideGraduationCap, LucideHandCoins, LucideHandHeart, LucideHeartPulse, LucideHouse, LucideLandmark, LucidePawPrint,
  LucidePlane, LucideShieldCheck, LucideShoppingCart, LucideSparkles, LucideTv, LucideUtensils, LucideLayoutDashboard, LucideLogOut, LucideMenu, LucideMonitor, LucideMoon, LucidePalette, LucidePercent,
  LucidePiggyBank, LucideReceipt, LucideReceiptText, LucideRefreshCw, LucideRepeat2, LucideSettings, LucideShoppingBag,
  LucideSlidersHorizontal, LucideSun, LucideTags, LucideTarget, LucideTrendingUp, LucideTriangleAlert, LucideUndo2,
  LucideWallet, LucideX,
  LucideLuggage, LucideGift, LucideBaby, LucideGem, LucideLaptop, LucideSmartphone, LucideBike
} from '@lucide/angular';
import type { LucideIcon } from '@lucide/angular';

export const Icones = {
  // Menu
  painel: LucideLayoutDashboard,
  visaoAnual: LucideCalendarRange,
  transacoes: LucideArrowLeftRight,
  contas: LucideCreditCard,
  orcamentos: LucideChartPie,
  metas: LucideTarget,
  contasAPagar: LucideCalendarClock,
  calendario: LucideCalendar,
  investimentos: LucideTrendingUp,
  emprestimos: LucideHandCoins,
  relatorios: LucideChartColumn,
  projecao: LucideChartLine,
  categorias: LucideTags,
  configuracoes: LucideSettings,
  manual: LucideBookOpen,
  // Grupos do menu (#227) e a seta que mostra se o grupo está aberto
  grupoMovimentacao: LucideWallet,
  grupoPlanejamento: LucideClipboardList,
  grupoPatrimonio: LucideCoins,
  grupoAnalises: LucideChartNoAxesCombined,
  grupoAjustes: LucideSlidersHorizontal,
  setaGrupo: LucideChevronRight,
  // Ações e preferências
  ajuda: LucideCircleQuestionMark,
  sincronizar: LucideRefreshCw,
  mostrar: LucideEye,
  ocultar: LucideEyeOff,
  claro: LucideSun,
  escuro: LucideMoon,
  sistema: LucideMonitor,
  aparencia: LucidePalette,
  marcado: LucideCheck,
  calculadora: LucideCalculator,
  fechar: LucideX,
  abrirMenu: LucideChevronDown,
  menuLateral: LucideMenu,
  sair: LucideLogOut,
  alerta: LucideTriangleAlert,
  // Marca e indicadores
  marca: LucidePiggyBank,
  patrimonio: LucideWallet,
  banco: LucideLandmark,
  financiamento: LucideHouse,
  // Tipo de lançamento (#111)
  pixEnviado: LucideArrowUpRight,
  pixRecebido: LucideArrowDownLeft,
  entreContas: LucideRepeat2,
  pagamentoFatura: LucideReceiptText,
  boleto: LucideReceipt,
  compraDebito: LucideShoppingBag,
  dinheiro: LucideBanknote,
  tarifa: LucidePercent,
  estorno: LucideUndo2,
  salario: LucideBriefcase,
  outro: LucideCircleEllipsis
} as const;

/**
 * #265: ícone de cada categoria padrão (banco/02-dados-iniciais.sql), pelo nome sem acento e em maiúsculas. Usado
 * pelo <app-selo-servico> quando o lançamento não é de um serviço conhecido (compartilhado/servicos.ts). Categoria
 * criada por você, ou sem categoria: o recibo.
 */
export const IconesCategoria: Record<string, LucideIcon> = {
  ALIMENTACAO: LucideUtensils,
  MERCADO: LucideShoppingCart,
  MORADIA: LucideHouse,
  TRANSPORTE: LucideCar,
  SAUDE: LucideHeartPulse,
  EDUCACAO: LucideGraduationCap,
  LAZER: LucideClapperboard,
  COMPRAS: LucideShoppingBag,
  'SERVICOS E ASSINATURAS': LucideTv,
  VIAGENS: LucidePlane,
  'IMPOSTOS E TAXAS': LucideLandmark,
  'TARIFAS BANCARIAS': LucidePercent,
  SEGUROS: LucideShieldCheck,
  DOACOES: LucideHandHeart,
  PETS: LucidePawPrint,
  'CUIDADOS PESSOAIS': LucideSparkles,
  'JUROS E EMPRESTIMOS': LucideHandCoins,
  'OUTROS GASTOS': LucideCircleEllipsis,
  // Receitas e transferências (para o selo também servir em Transações)
  SALARIO: LucideBriefcase,
  RENDIMENTOS: LucideTrendingUp,
  'OUTRAS RECEITAS': LucideCoins,
  TRANSFERENCIAS: LucideArrowLeftRight,
  'PAGAMENTO DE FATURA': LucideReceiptText,
  INVESTIMENTOS: LucidePiggyBank
};
export const IconeCobrancaPadrao: LucideIcon = LucideReceipt;

/**
 * #316: os ícones que a pessoa escolhe para cada meta (Meta.Icone guarda a chave). Ajudam a achar a meta de olho
 * (Nielsen 6, Recognition Rather than Recall). Lucide, como os das categorias (ADR 0016); o nome vai no aria-label.
 * Chave nova: só letras minúsculas e hífen (a API confere) e nunca trocar a de uma que já existe.
 */
export const IconesMeta: readonly { chave: string; nome: string; icone: LucideIcon }[] = [
  { chave: 'cofrinho', nome: 'Cofrinho', icone: LucidePiggyBank },
  { chave: 'reserva', nome: 'Reserva de emergência', icone: LucideShieldCheck },
  { chave: 'casa', nome: 'Casa', icone: LucideHouse },
  { chave: 'carro', nome: 'Carro', icone: LucideCar },
  { chave: 'viagem', nome: 'Viagem', icone: LucidePlane },
  { chave: 'mala', nome: 'Férias', icone: LucideLuggage },
  { chave: 'estudo', nome: 'Estudo', icone: LucideGraduationCap },
  { chave: 'saude', nome: 'Saúde', icone: LucideHeartPulse },
  { chave: 'presente', nome: 'Presente', icone: LucideGift },
  { chave: 'bebe', nome: 'Bebê', icone: LucideBaby },
  { chave: 'casamento', nome: 'Casamento', icone: LucideGem },
  { chave: 'notebook', nome: 'Notebook', icone: LucideLaptop },
  { chave: 'celular', nome: 'Celular', icone: LucideSmartphone },
  { chave: 'bicicleta', nome: 'Bicicleta', icone: LucideBike },
  { chave: 'pet', nome: 'Pet', icone: LucidePawPrint },
  { chave: 'sonho', nome: 'Sonho', icone: LucideSparkles }
];

/** O ícone da meta pela chave gravada; sem chave (ou uma que não existe mais), o cofrinho. */
export function iconeDaMeta(chave: string | null | undefined): { chave: string; nome: string; icone: LucideIcon } {
  return IconesMeta.find((i) => i.chave === chave) ?? IconesMeta[0];
}

