/**
 * Ícones do site (Lucide, licença ISC — https://lucide.dev). Todos vêm do pacote npm e entram no build:
 * nada é baixado da internet (a CSP só permite recursos do próprio site). Para trocar um ícone, mude aqui.
 */
import {
  ArrowDownLeft, ArrowLeftRight, ArrowUpRight, Banknote, BookOpen, Briefcase, Calculator, Calendar, CalendarClock, CalendarRange, ChartColumn,
  ChartLine, ChartPie, Check, ChevronDown, CircleEllipsis, CircleQuestionMark, CreditCard, Eye, EyeOff, HandCoins, House, Landmark,
  LayoutDashboard, LogOut, Menu, Monitor, Moon, Palette, Percent, PiggyBank, Receipt, ReceiptText, RefreshCw, Repeat2,
  Settings, ShoppingBag, Sun, Tags, Target, TrendingUp, TriangleAlert, Undo2, Wallet, X
} from 'lucide-angular';

export const Icones = {
  // Menu
  painel: LayoutDashboard,
  visaoAnual: CalendarRange,
  transacoes: ArrowLeftRight,
  contas: CreditCard,
  orcamentos: ChartPie,
  metas: Target,
  contasAPagar: CalendarClock,
  calendario: Calendar,
  investimentos: TrendingUp,
  emprestimos: HandCoins,
  relatorios: ChartColumn,
  projecao: ChartLine,
  categorias: Tags,
  configuracoes: Settings,
  manual: BookOpen,
  // Ações e preferências
  ajuda: CircleQuestionMark,
  sincronizar: RefreshCw,
  mostrar: Eye,
  ocultar: EyeOff,
  claro: Sun,
  escuro: Moon,
  sistema: Monitor,
  aparencia: Palette,
  marcado: Check,
  calculadora: Calculator,
  fechar: X,
  abrirMenu: ChevronDown,
  menuLateral: Menu,
  sair: LogOut,
  alerta: TriangleAlert,
  // Marca e indicadores
  marca: PiggyBank,
  patrimonio: Wallet,
  banco: Landmark,
  financiamento: House,
  // Tipo de lançamento (#111)
  pixEnviado: ArrowUpRight,
  pixRecebido: ArrowDownLeft,
  entreContas: Repeat2,
  pagamentoFatura: ReceiptText,
  boleto: Receipt,
  compraDebito: ShoppingBag,
  dinheiro: Banknote,
  tarifa: Percent,
  estorno: Undo2,
  salario: Briefcase,
  outro: CircleEllipsis
} as const;
