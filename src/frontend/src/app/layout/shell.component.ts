import { Component, DestroyRef, ElementRef, Injector, afterNextRender, computed, inject, signal, viewChild } from '@angular/core';
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
    <div class="app" [class.menu-aberto]="menuAberto()" [class.menu-recolhido]="recolhidoNoComputador()">
      <!-- #131: menu escondido (recolhido no computador, gaveta fechada no celular) fica fora do Tab (inert). -->
      <aside id="menu-lateral" class="lateral" [attr.inert]="menuVisivel() ? null : ''">
        <!-- #131: a marca leva ao Painel, a tela inicial (convenção da web; Nielsen 3 e 4). -->
        <a class="marca" routerLink="/painel" aria-label="Finanças, ir para o Painel" (click)="menuAberto.set(false)">
          <span class="marca-simbolo" aria-hidden="true"><app-icone [icone]="Icones.marca" [tamanho]="22" [traco]="2" /></span>
          <div>
            <strong>Finanças</strong>
            <small>local e privado</small>
          </div>
        </a>

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
          <!-- #131: o mesmo ☰ no computador (esconde/mostra o menu) e no celular (abre a gaveta); nome fixo,
               o estado vai no aria-expanded. -->
          <div #barraInicio class="barra-inicio">
            <button type="button" class="botao-barra botao-menu" (click)="alternarMenu()"
              [attr.aria-expanded]="menuVisivel()" aria-controls="menu-lateral" aria-label="Menu de navegação">
              <app-icone [icone]="Icones.menuLateral" [tamanho]="20" />
            </button>
            <a class="barra-titulo so-celular" routerLink="/painel" aria-label="Finanças, ir para o Painel">
              <span class="barra-titulo-simbolo" aria-hidden="true"><app-icone [icone]="Icones.marca" [tamanho]="16" [traco]="2" /></span>
              Finanças
            </a>
          </div>
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
        <main #pagina class="pagina">
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
  /** Preferência de exibição (ADR 0017): menu recolhido no computador; padrão aberto. */
  static readonly CHAVE_RECOLHIDO = 'financas.menuRecolhido';
  /** Mesma quebra do styles.css (@media max-width: 900px), em que o menu vira gaveta. */
  static readonly MIDIA_CELULAR = '(max-width: 900px)';

  /** Duração do deslize do menu (a mesma do styles.css). */
  private static readonly DURACAO_MS = 220;

  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly pagina = viewChild.required<ElementRef<HTMLElement>>('pagina');
  private readonly barraInicio = viewChild.required<ElementRef<HTMLElement>>('barraInicio');
  /** Gaveta do celular: sempre começa fechada. */
  readonly menuAberto = signal(false);
  /** Escolha do computador, lembrada no navegador. */
  readonly menuRecolhido = signal(this.lerRecolhido());
  readonly celular = signal(false);

  /** No celular a preferência não vale: lá quem manda é a gaveta. */
  readonly recolhidoNoComputador = computed(() => !this.celular() && this.menuRecolhido());
  readonly menuVisivel = computed(() => (this.celular() ? this.menuAberto() : !this.menuRecolhido()));

  readonly Icones = Icones;
  readonly menu = MENU;

  private readonly url = toSignal(
    this.router.events.pipe(filter((e) => e instanceof NavigationEnd), map(() => this.router.url)),
    { initialValue: this.router.url });
  /** Capítulo e seção do manual da tela aberta (null no próprio manual). */
  readonly ajuda = computed(() => ajudaDaUrl(this.url()));

  constructor() {
    // Acompanha a largura da tela: girar o celular ou redimensionar a janela troca entre gaveta e menu fixo.
    const midia = typeof window.matchMedia === 'function' ? window.matchMedia(ShellComponent.MIDIA_CELULAR) : null;
    if (midia) {
      this.celular.set(midia.matches);
      const mudou = (e: MediaQueryListEvent) => this.celular.set(e.matches);
      midia.addEventListener('change', mudou);
      inject(DestroyRef).onDestroy(() => midia.removeEventListener('change', mudou));
    }
  }

  alternarMenu(): void {
    if (this.celular()) {
      this.menuAberto.update((v) => !v);
      return;
    }
    const voltando = this.menuRecolhido();
    // A página e o começo da barra (☰ + título); os botões da direita não mudam de lugar.
    const elementos = [this.pagina().nativeElement, this.barraInicio().nativeElement];
    const antes = voltando ? elementos.map((el) => el.getBoundingClientRect().left) : [];
    this.menuRecolhido.update((v) => !v);
    try {
      localStorage.setItem(ShellComponent.CHAVE_RECOLHIDO, this.menuRecolhido() ? '1' : '0');
    } catch {
      /* armazenamento indisponível: vale só até recarregar */
    }
    if (voltando) {
      afterNextRender(() => elementos.forEach((el, i) => this.deslizar(el, antes[i])), { injector: this.injector });
    }
  }

  /**
   * Volta do menu sem pulo (FLIP): a coluna abre de uma vez (layout refeito uma vez só) e o elemento desliza da
   * posição antiga até a nova, junto com o menu. Só transform: animar a largura refazia o layout a cada quadro.
   */
  private deslizar(el: HTMLElement, antes: number): void {
    const deslocamento = antes - el.getBoundingClientRect().left;
    if (!deslocamento || typeof el.animate !== 'function') return;
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
    el.animate([{ transform: `translateX(${deslocamento}px)` }, { transform: 'none' }],
      { duration: ShellComponent.DURACAO_MS, easing: 'ease' });
  }

  private lerRecolhido(): boolean {
    try {
      return localStorage.getItem(ShellComponent.CHAVE_RECOLHIDO) === '1';
    } catch {
      return false;
    }
  }
}
