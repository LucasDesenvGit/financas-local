import { Component, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { BotaoPrivacidadeComponent } from '../compartilhado/botao-privacidade.component';
import { CalculadoraComponent } from '../compartilhado/calculadora.component';
import { StatusSincronizacaoComponent } from '../compartilhado/status-sincronizacao.component';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { MenuUsuarioComponent } from '../compartilhado/menu-usuario.component';
import { LucideIconData } from 'lucide-angular';

interface ItemMenu { rota: string; rotulo: string; icone: LucideIconData; }

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, BotaoPrivacidadeComponent, CalculadoraComponent, MenuUsuarioComponent,
    IconeComponent, StatusSincronizacaoComponent],
  template: `
    <div class="app" [class.menu-aberto]="menuAberto()">
      <aside class="lateral">
        <div class="marca">
          <span class="marca-simbolo" aria-hidden="true"><app-icone [icone]="Icones.marca" [tamanho]="22" [traco]="2" /></span>
          <div>
            <strong>Finanças</strong>
            <small>local e privado</small>
          </div>
        </div>

        <nav class="menu" aria-label="Menu principal">
          @for (item of menu; track item.rota) {
            <a [routerLink]="item.rota" routerLinkActive="ativo" (click)="menuAberto.set(false)">
              <app-icone class="menu-icone" [icone]="item.icone" />
              {{ item.rotulo }}
            </a>
          }
        </nav>

      </aside>

      <div class="conteudo">
        <!-- UX-7: ações globais numa barra só, fixa no topo e na mesma posição no desktop e no celular. -->
        <header class="barra-superior">
          <button type="button" class="botao-barra so-celular" (click)="menuAberto.set(!menuAberto())"
            [attr.aria-expanded]="menuAberto()" aria-label="Abrir menu de navegação">
            <app-icone [icone]="Icones.menuLateral" [tamanho]="20" />
          </button>
          <strong class="barra-titulo so-celular">Finanças</strong>
          <div class="barra-acoes">
            <app-status-sincronizacao />
            <app-botao-privacidade />
            <app-calculadora />
            <app-menu-usuario />
          </div>
        </header>
        <main class="pagina">
          <router-outlet />
        </main>
      </div>
    </div>
  `
})
export class ShellComponent {
  readonly menuAberto = signal(false);

  readonly Icones = Icones;

  readonly menu: ItemMenu[] = [
    { rota: '/painel', rotulo: 'Painel', icone: Icones.painel },
    { rota: '/anual', rotulo: 'Visão anual', icone: Icones.visaoAnual },
    { rota: '/transacoes', rotulo: 'Transações', icone: Icones.transacoes },
    { rota: '/contas', rotulo: 'Contas e cartões', icone: Icones.contas },
    { rota: '/orcamentos', rotulo: 'Orçamentos', icone: Icones.orcamentos },
    { rota: '/metas', rotulo: 'Metas', icone: Icones.metas },
    { rota: '/contas-a-pagar', rotulo: 'Contas a pagar', icone: Icones.contasAPagar },
    { rota: '/investimentos', rotulo: 'Investimentos', icone: Icones.investimentos },
    { rota: '/emprestimos', rotulo: 'Empréstimos e financiamentos', icone: Icones.emprestimos },
    { rota: '/relatorios', rotulo: 'Relatórios', icone: Icones.relatorios },
    { rota: '/projecao', rotulo: 'Projeção', icone: Icones.projecao },
    { rota: '/categorias', rotulo: 'Categorias e regras', icone: Icones.categorias },
    { rota: '/configuracoes', rotulo: 'Configurações', icone: Icones.configuracoes }
  ];
}
