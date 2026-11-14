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
        loadComponent: () => import('./paginas/painel/painel.component').then((m) => m.PainelComponent),
        title: 'Painel · Finanças Local'
      },
      {
        path: 'transacoes',
        loadComponent: () => import('./paginas/movimentacao/transacoes.component').then((m) => m.TransacoesComponent),
        title: 'Transações · Finanças Local'
      },
      {
        path: 'contas',
        loadComponent: () => import('./paginas/movimentacao/contas.component').then((m) => m.ContasComponent),
        title: 'Contas e cartões · Finanças Local'
      },
      {
        path: 'orcamentos',
        loadComponent: () => import('./paginas/planejamento/orcamentos.component').then((m) => m.OrcamentosComponent),
        title: 'Orçamentos · Finanças Local'
      },
      {
        path: 'metas',
        loadComponent: () => import('./paginas/planejamento/metas.component').then((m) => m.MetasComponent),
        title: 'Metas · Finanças Local'
      },
      {
        path: 'contas-a-pagar',
        loadComponent: () => import('./paginas/movimentacao/contas-agendadas.component').then((m) => m.ContasAgendadasComponent),
        title: 'Contas a pagar · Finanças Local'
      },
      {
        path: 'investimentos',
        loadComponent: () => import('./paginas/patrimonio/investimentos.component').then((m) => m.InvestimentosComponent),
        title: 'Investimentos · Finanças Local'
      },
      {
        path: 'emprestimos',
        loadComponent: () => import('./paginas/patrimonio/emprestimos.component').then((m) => m.EmprestimosComponent),
        title: 'Empréstimos e financiamentos · Finanças Local'
      },
      {
        path: 'relatorios',
        loadComponent: () => import('./paginas/analises/relatorios.component').then((m) => m.RelatoriosComponent),
        title: 'Relatórios · Finanças Local'
      },
      {
        path: 'anual',
        loadComponent: () => import('./paginas/analises/anual.component').then((m) => m.AnualComponent),
        title: 'Visão anual · Finanças Local'
      },
      {
        path: 'projecao',
        loadComponent: () => import('./paginas/analises/projecao.component').then((m) => m.ProjecaoComponent),
        title: 'Projeção · Finanças Local'
      },
      {
        path: 'categorias',
        loadComponent: () => import('./paginas/ajustes/categorias.component').then((m) => m.CategoriasComponent),
        title: 'Categorias e regras · Finanças Local'
      },
      {
        path: 'conferir-banco',
        loadComponent: () => import('./paginas/movimentacao/conferir-banco.component').then((m) => m.ConferirBancoComponent),
        title: 'Conferir com o banco · Finanças Local'
      },
      {
        path: 'configuracoes',
        loadComponent: () => import('./paginas/ajustes/configuracoes.component').then((m) => m.ConfiguracoesComponent),
        title: 'Configurações · Finanças Local'
      },
      {
        // #210 (ADR 0064): o módulo Trabalho, dentro do portão (desligado, mostra o aviso e a tela nem é criada).
        path: 'trabalho',
        loadComponent: () => import('./compartilhado/modulos/portao-modulo.component').then((m) => m.PortaoModuloComponent),
        data: { modulo: 'trabalho' },
        children: [
          { path: '', pathMatch: 'full', redirectTo: 'salario' },
          {
            path: 'salario',
            loadComponent: () => import('./paginas/trabalho/salario.component').then((m) => m.TrabalhoSalarioComponent),
            title: 'Salário · Trabalho · Finanças Local'
          },
          {
            path: 'holerites',
            loadComponent: () => import('./paginas/trabalho/holerites.component').then((m) => m.TrabalhoHoleritesComponent),
            title: 'Holerites · Trabalho · Finanças Local'
          }
        ]
      },
      {
        path: 'manual',
        loadComponent: () => import('./paginas/manual/manual.component').then((m) => m.ManualComponent),
        title: 'Manual · Finanças Local'
      },
      {
        path: 'manual/:capitulo',
        loadComponent: () => import('./paginas/manual/manual.component').then((m) => m.ManualComponent),
        title: 'Manual · Finanças Local'
      }
    ]
  },
  { path: '**', redirectTo: '' }
];
