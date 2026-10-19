/**
 * Ícones do site (Lucide, licença ISC — https://lucide.dev). Todos vêm do pacote npm e entram no build:
 * nada é baixado da internet (a CSP só permite recursos do próprio site). Para trocar um ícone, mude aqui.
 */
import {
  LucideArrowDownLeft, LucideArrowLeftRight, LucideArrowUpRight, LucideBanknote, LucideBookOpen, LucideBriefcase, LucideCalculator,
  LucideCalendar, LucideCalendarClock, LucideCalendarRange, LucideChartColumn, LucideChartLine, LucideChartPie, LucideCheck,
  LucideChevronDown, LucideCircleEllipsis, LucideCircleQuestionMark, LucideCreditCard, LucideEye, LucideEyeOff, LucideHandCoins,
  LucideHouse, LucideLandmark, LucideLayoutDashboard, LucideLogOut, LucideMenu, LucideMonitor, LucideMoon, LucidePalette, LucidePercent,
  LucidePiggyBank, LucideReceipt, LucideReceiptText, LucideRefreshCw, LucideRepeat2, LucideSettings, LucideShoppingBag, LucideSun,
  LucideTags, LucideTarget, LucideTrendingUp, LucideTriangleAlert, LucideUndo2, LucideWallet, LucideX
} from '@lucide/angular';

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
