import { Routes } from '@angular/router';
import { autenticadoGuard, anonimoGuard } from './core/auth.guard';
import { ShellComponent } from './layout/shell.component';

export const rotas: Routes = [
  {
    path: 'entrar',
    canActivate: [anonimoGuard],
    loadComponent: () => import('./paginas/entrar.component').then((m) => m.EntrarComponent),
    title: 'Entrar · Finanças Local'
  },
  {
    path: '',
    component: ShellComponent,
    canActivate: [autenticadoGuard],
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'painel' },
      {
        path: 'painel',
        loadComponent: () => import('./paginas/painel.component').then((m) => m.PainelComponent),
        title: 'Painel · Finanças Local'
      },
      {
        path: 'transacoes',
        loadComponent: () => import('./paginas/transacoes.component').then((m) => m.TransacoesComponent),
        title: 'Transações · Finanças Local'
      },
      {
        path: 'contas',
        loadComponent: () => import('./paginas/contas.component').then((m) => m.ContasComponent),
        title: 'Contas e cartões · Finanças Local'
      },
      {
        path: 'orcamentos',
        loadComponent: () => import('./paginas/orcamentos.component').then((m) => m.OrcamentosComponent),
        title: 'Orçamentos · Finanças Local'
      },
      {
        path: 'metas',
        loadComponent: () => import('./paginas/metas.component').then((m) => m.MetasComponent),
        title: 'Metas · Finanças Local'
      },
      {
        path: 'contas-a-pagar',
        loadComponent: () => import('./paginas/contas-agendadas.component').then((m) => m.ContasAgendadasComponent),
        title: 'Contas a pagar · Finanças Local'
      },
      {
        path: 'investimentos',
        loadComponent: () => import('./paginas/investimentos.component').then((m) => m.InvestimentosComponent),
        title: 'Investimentos · Finanças Local'
      },
      {
        path: 'emprestimos',
        loadComponent: () => import('./paginas/emprestimos.component').then((m) => m.EmprestimosComponent),
        title: 'Empréstimos e financiamentos · Finanças Local'
      },
      {
        path: 'relatorios',
        loadComponent: () => import('./paginas/relatorios.component').then((m) => m.RelatoriosComponent),
        title: 'Relatórios · Finanças Local'
      },
      {
        path: 'anual',
        loadComponent: () => import('./paginas/anual.component').then((m) => m.AnualComponent),
        title: 'Visão anual · Finanças Local'
      },
      {
        path: 'projecao',
        loadComponent: () => import('./paginas/projecao.component').then((m) => m.ProjecaoComponent),
        title: 'Projeção · Finanças Local'
      },
      {
        path: 'categorias',
        loadComponent: () => import('./paginas/categorias.component').then((m) => m.CategoriasComponent),
        title: 'Categorias e regras · Finanças Local'
      },
      {
        path: 'configuracoes',
        loadComponent: () => import('./paginas/configuracoes.component').then((m) => m.ConfiguracoesComponent),
        title: 'Configurações · Finanças Local'
      }
    ]
  },
  { path: '**', redirectTo: '' }
];
