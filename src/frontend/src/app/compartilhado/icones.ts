/**
 * Ícones do site (Lucide, licença ISC — https://lucide.dev). Todos vêm do pacote npm e entram no build:
 * nada é baixado da internet (a CSP só permite recursos do próprio site). Para trocar um ícone, mude aqui.
 */
import {
  ArrowLeftRight, CalendarClock, CalendarRange, ChartColumn, ChartLine, ChartPie, Check, CreditCard, Eye, EyeOff,
  HandCoins, House, Landmark, LayoutDashboard, Monitor, Moon, Palette, PiggyBank, RefreshCw, Settings, Sun, Tags, Target,
  TrendingUp, Wallet
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
  investimentos: TrendingUp,
  emprestimos: HandCoins,
  relatorios: ChartColumn,
  projecao: ChartLine,
  categorias: Tags,
  configuracoes: Settings,
  // Ações e preferências
  sincronizar: RefreshCw,
  mostrar: Eye,
  ocultar: EyeOff,
  claro: Sun,
  escuro: Moon,
  sistema: Monitor,
  aparencia: Palette,
  marcado: Check,
  // Marca e indicadores
  marca: PiggyBank,
  patrimonio: Wallet,
  banco: Landmark,
  financiamento: House
} as const;
