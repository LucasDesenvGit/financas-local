import { Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter, map } from 'rxjs';
import { ajudaDaUrl } from '../paginas/manual-conteudo';
import { BotaoPrivacidadeComponent } from '../compartilhado/botao-privacidade.component';
import { CalculadoraComponent } from '../compartilhado/calculadora.component';
import { StatusSincronizacaoComponent } from '../compartilhado/status-sincronizacao.component';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { MenuUsuarioComponent } from '../compartilhado/menu-usuario.component';
import { LucideIconData } from 'lucide-angular';

export interface ItemMenu { rota: string; rotulo: string; icone: LucideIconData; }

/** Menu lateral. Toda tela daqui precisa de seção no manual (AJUDA_POR_ROTA; teste de regressão). */
export const MENU: ItemMenu[] = [
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
          <a routerLink="/manual" routerLinkActive="ativo" class="item-manual" (click)="menuAberto.set(false)">
            <app-icone class="menu-icone" [icone]="Icones.manual" />
            Manual
          </a>
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
            <!-- #113: a seção do manual que explica a tela aberta (Nielsen 10: ajuda no contexto). -->
            @if (ajuda(); as a) {
              <a class="botao-barra botao-ajuda" [routerLink]="['/manual', a.capitulo]" [fragment]="a.secao"
                aria-label="Ajuda: como usar esta tela">
                <app-icone [icone]="Icones.ajuda" [tamanho]="18" /><span class="rotulo-ajuda">Ajuda</span>
              </a>
            }
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
  `,
  styles: [`
    .botao-ajuda { text-decoration: none; }
    .item-manual { margin-top: 10px; border-top: 1px solid var(--linha); padding-top: 12px; }
    @media (max-width: 760px) { .rotulo-ajuda { display: none; } }
  `]
})
export class ShellComponent {
  private readonly router = inject(Router);
  readonly menuAberto = signal(false);

  readonly Icones = Icones;
  readonly menu = MENU;

  private readonly url = toSignal(
    this.router.events.pipe(filter((e) => e instanceof NavigationEnd), map(() => this.router.url)),
    { initialValue: this.router.url });
  /** Capítulo e seção do manual da tela aberta (null no próprio manual). */
  readonly ajuda = computed(() => ajudaDaUrl(this.url()));
}
