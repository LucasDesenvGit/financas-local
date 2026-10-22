import { Component, DestroyRef, ElementRef, HostListener, Injector, afterNextRender, computed, inject, signal, viewChild, ChangeDetectionStrategy } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter, map } from 'rxjs';
import { ajudaDaUrl } from '../paginas/manual-conteudo';
import { BotaoPrivacidadeComponent } from '../compartilhado/botao-privacidade.component';
import { CalculadoraComponent } from '../compartilhado/calculadora.component';
import { StatusSincronizacaoComponent } from '../compartilhado/status-sincronizacao.component';
import { NotificacoesComponent } from '../compartilhado/notificacoes.component';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { MenuUsuarioComponent } from '../compartilhado/menu-usuario.component';
import { LucideIcon } from '@lucide/angular';

export interface ItemMenu { rota: string; rotulo: string; icone: LucideIcon; }

/**
 * Grupo do menu (#227, ADR 0046): telas do mesmo assunto, atrás de um cabeçalho que abre e fecha. São os mesmos
 * grupos, com os mesmos nomes e a mesma ordem, dos módulos do roteiro de primeiro uso (#193). No código é "grupo",
 * nunca "modulo": "módulo" é a funcionalidade que se liga e desliga (#190).
 */
export interface GrupoMenu { id: string; rotulo: string; icone: LucideIcon; telas: ItemMenu[]; }

/** Uma linha do menu: uma tela solta (o Painel) ou um grupo de telas. */
export type EntradaMenu = ItemMenu | GrupoMenu;

export function ehGrupo(entrada: EntradaMenu): entrada is GrupoMenu {
  return 'telas' in entrada;
}

/**
 * Menu lateral. Toda tela daqui precisa de seção no manual (AJUDA_POR_ROTA; teste de regressão). Tela nova entra no
 * grupo do assunto dela; grupo novo só quando nenhum serve e ele junta duas telas ou mais (ADR 0046). O Manual fica
 * fora, solto no rodapé do menu: é a ajuda de tudo.
 */
export const MENU: EntradaMenu[] = [
  { rota: '/painel', rotulo: 'Painel', icone: Icones.painel },
  {
    id: 'movimentacao', rotulo: 'Movimentação', icone: Icones.grupoMovimentacao, telas: [
      { rota: '/transacoes', rotulo: 'Transações', icone: Icones.transacoes },
      { rota: '/contas', rotulo: 'Contas e cartões', icone: Icones.contas },
      { rota: '/contas-a-pagar', rotulo: 'Contas a pagar', icone: Icones.contasAPagar }
    ]
  },
  {
    id: 'planejamento', rotulo: 'Planejamento', icone: Icones.grupoPlanejamento, telas: [
      { rota: '/orcamentos', rotulo: 'Orçamentos', icone: Icones.orcamentos },
      { rota: '/metas', rotulo: 'Metas', icone: Icones.metas }
    ]
  },
  {
    id: 'patrimonio', rotulo: 'Patrimônio', icone: Icones.grupoPatrimonio, telas: [
      { rota: '/investimentos', rotulo: 'Investimentos', icone: Icones.investimentos },
      { rota: '/emprestimos', rotulo: 'Empréstimos e financiamentos', icone: Icones.emprestimos }
    ]
  },
  {
    id: 'analises', rotulo: 'Análises', icone: Icones.grupoAnalises, telas: [
      { rota: '/relatorios', rotulo: 'Relatórios', icone: Icones.relatorios },
      { rota: '/anual', rotulo: 'Visão anual', icone: Icones.visaoAnual },
      { rota: '/projecao', rotulo: 'Projeção', icone: Icones.projecao }
    ]
  },
  {
    id: 'ajustes', rotulo: 'Ajustes', icone: Icones.grupoAjustes, telas: [
      { rota: '/categorias', rotulo: 'Categorias e regras', icone: Icones.categorias },
      { rota: '/configuracoes', rotulo: 'Configurações', icone: Icones.configuracoes }
    ]
  }
];

/** Todas as telas do menu, na ordem em que aparecem (as soltas e as de dentro dos grupos). */
export function telasDoMenu(menu: EntradaMenu[] = MENU): ItemMenu[] {
  return menu.flatMap((e) => (ehGrupo(e) ? e.telas : [e]));
}

/** Caminho da URL, sem filtros nem âncora ("/transacoes?categoria=sem" → "/transacoes"). */
function caminho(url: string): string {
  return url.split(/[?#]/)[0];
}

/** O grupo em que fica a tela da URL (null para tela solta, como o Painel e o Manual). */
export function grupoDaUrl(url: string, menu: EntradaMenu[] = MENU): GrupoMenu | null {
  const atual = caminho(url);
  return menu.filter(ehGrupo)
    .find((g) => g.telas.some((t) => atual === t.rota || atual.startsWith(`${t.rota}/`))) ?? null;
}

/**
 * Esconde as telas que não devem aparecer e, junto, o grupo que ficar sem nenhuma tela. Pronto para os módulos que
 * se ligam e desligam (#190): a tela de um módulo desligado some do menu sem deixar cabeçalho vazio.
 */
export function filtrarMenu(menu: EntradaMenu[], telaVisivel: (tela: ItemMenu) => boolean): EntradaMenu[] {
  return menu.flatMap((e): EntradaMenu[] => {
    if (!ehGrupo(e)) return telaVisivel(e) ? [e] : [];
    const telas = e.telas.filter(telaVisivel);
    return telas.length ? [{ ...e, telas }] : [];
  });
}

/** Nome estável da tela para o roteiro (#194): "/contas-a-pagar" → "menu-tela-contas-a-pagar". */
export function roteiroDaTela(rota: string): string {
  return `menu-tela-${rota.slice(1)}`;
}

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, BotaoPrivacidadeComponent, CalculadoraComponent, MenuUsuarioComponent,
    IconeComponent, StatusSincronizacaoComponent, NotificacoesComponent],
  template: `
    <div class="app" [class.menu-aberto]="menuAberto()" [class.menu-recolhido]="recolhidoNoComputador()">
      <!-- #131: menu escondido (recolhido no computador, gaveta fechada no celular) fica fora do Tab (inert). -->
      <aside id="menu-lateral" class="lateral" [attr.inert]="menuVisivel() ? null : ''"
        (pointerdown)="inicioArraste($event)" (pointerup)="fimArraste($event)">
        <div class="lateral-topo">
          <!-- #131: a marca leva ao Painel, a tela inicial (convenção da web; Nielsen 3 e 4). -->
          <a class="marca" routerLink="/painel" aria-label="Finanças, ir para o Painel" (click)="menuAberto.set(false)">
            <span class="marca-simbolo" aria-hidden="true"><app-icone [icone]="Icones.marca" [tamanho]="22" [traco]="2" /></span>
            <div>
              <strong>Finanças</strong>
              <small>local e privado</small>
            </div>
          </a>
          <!-- Celular: saída visível da gaveta sem escolher uma tela (Nielsen 3); Esc e tocar fora também fecham. -->
          <button #botaoFechar type="button" class="botao-barra botao-fechar-menu so-celular" aria-label="Fechar menu"
            (click)="fecharGaveta()">
            <app-icone [icone]="Icones.fechar" [tamanho]="20" />
          </button>
        </div>

        <!-- #227 (ADR 0046): grupos que abrem e fecham. O cabeçalho é um botão (WAI-ARIA APG, Disclosure Pattern);
             as telas continuam links. Vários grupos podem ficar abertos (NN/g, Accordions on Desktop). Tocar num
             grupo não fecha a gaveta do celular; tocar numa tela, sim. data-roteiro: alvos estáveis do roteiro (#194). -->
        <nav class="menu" aria-label="Menu principal" data-roteiro="menu">
          @for (entrada of entradas(); track entrada.rotulo) {
            @if (grupoDe(entrada); as grupo) {
              <div class="grupo-menu">
                <button type="button" class="grupo-cabecalho" [id]="'menu-grupo-' + grupo.id + '-botao'"
                  [attr.aria-expanded]="estaAberto(grupo.id)" [attr.aria-controls]="'menu-grupo-' + grupo.id"
                  [class.contem-atual]="!estaAberto(grupo.id) && grupoAtual() === grupo.id"
                  [attr.data-roteiro]="'menu-grupo-' + grupo.id" (click)="alternarGrupo(grupo.id)">
                  <app-icone class="menu-icone" [icone]="grupo.icone" />
                  <span class="grupo-nome">{{ grupo.rotulo }}</span>
                  <!-- Grupo fechado com a tela aberta dentro: a pessoa vê (e ouve) onde está (Nielsen 1). -->
                  @if (!estaAberto(grupo.id) && grupoAtual() === grupo.id) {
                    <span class="grupo-marca-atual" aria-hidden="true"></span>
                    <span class="so-leitor">, tela atual está aqui</span>
                  }
                  <app-icone class="grupo-seta" [icone]="Icones.setaGrupo" [tamanho]="16" />
                </button>
                <!-- Fechado: fora do Tab e do leitor de tela (inert + visibility no CSS), sem sair do DOM, para animar. -->
                <div class="grupo-telas" role="group" [id]="'menu-grupo-' + grupo.id"
                  [attr.aria-labelledby]="'menu-grupo-' + grupo.id + '-botao'"
                  [class.fechado]="!estaAberto(grupo.id)" [attr.inert]="estaAberto(grupo.id) ? null : ''">
                  <div class="grupo-telas-dentro">
                    @for (item of grupo.telas; track item.rota) {
                      <a [routerLink]="item.rota" routerLinkActive="ativo" [attr.data-roteiro]="roteiroDaTela(item.rota)"
                        (click)="menuAberto.set(false)">
                        <app-icone class="menu-icone" [icone]="item.icone" />
                        {{ item.rotulo }}
                      </a>
                    }
                  </div>
                </div>
              </div>
            } @else if (telaDe(entrada); as item) {
              <a [routerLink]="item.rota" routerLinkActive="ativo" [attr.data-roteiro]="roteiroDaTela(item.rota)"
                (click)="menuAberto.set(false)">
                <app-icone class="menu-icone" [icone]="item.icone" />
                {{ item.rotulo }}
              </a>
            }
          }
          <!-- Manual solto no rodapé: o espaço empurra para baixo quando sobra altura e some quando não sobra. -->
          <div class="menu-espaco" aria-hidden="true"></div>
          <a routerLink="/manual" routerLinkActive="ativo" class="item-manual" data-roteiro="menu-tela-manual"
            (click)="menuAberto.set(false)">
            <app-icone class="menu-icone" [icone]="Icones.manual" />
            Manual
          </a>
        </nav>

      </aside>
      <!-- Fundo escuro da gaveta: tocar fora fecha. Para teclado, há o ✕ e o Esc. -->
      <div class="fundo-gaveta" aria-hidden="true" (click)="fecharGaveta()"></div>

      <!-- Com a gaveta aberta, o conteúdo atrás fica fora do Tab: o foco não se perde embaixo do escurecimento. -->
      <div class="conteudo" [attr.inert]="celular() && menuAberto() ? '' : null">
        <!-- UX-7: ações globais numa barra só, fixa no topo e na mesma posição no desktop e no celular. -->
        <header class="barra-superior">
          <!-- #131: o mesmo ☰ no computador (esconde/mostra o menu) e no celular (abre a gaveta); nome fixo,
               o estado vai no aria-expanded. -->
          <div #barraInicio class="barra-inicio">
            <button #botaoMenu type="button" class="botao-barra botao-menu" (click)="alternarMenu()"
              [attr.aria-expanded]="menuVisivel()" aria-controls="menu-lateral" aria-label="Menu de navegação">
              <app-icone [icone]="Icones.menuLateral" [tamanho]="20" />
            </button>
            <a class="barra-titulo so-celular" routerLink="/painel" aria-label="Finanças, ir para o Painel">
              <span class="barra-titulo-simbolo" aria-hidden="true"><app-icone [icone]="Icones.marca" [tamanho]="16" [traco]="2" /></span>
              <span class="barra-titulo-nome">Finanças</span>
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
            <!-- #327: avisos e orçamentos estourados, em qualquer tela (saíram do topo do Painel). -->
            <app-notificacoes />
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
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [`
    .botao-ajuda { text-decoration: none; }
    .menu-espaco { flex: 1 0 10px; }
    .item-manual { border-top: 1px solid var(--linha); padding-top: 12px; }
    @media (max-width: 760px) { .rotulo-ajuda { display: none; } }
  `]
})
export class ShellComponent {
  /** Preferência de exibição (ADR 0017): menu recolhido no computador; padrão aberto. */
  static readonly CHAVE_RECOLHIDO = 'financas.menuRecolhido';
  /** Preferência de exibição (ADR 0017, #227): grupos do menu fechados; padrão todos abertos. */
  static readonly CHAVE_GRUPOS = 'financas.menuGrupos';
  /** Mesma quebra do styles.css (@media max-width: 900px), em que o menu vira gaveta. */
  static readonly MIDIA_CELULAR = '(max-width: 900px)';

  /** Duração do deslize do menu (a mesma do styles.css). */
  private static readonly DURACAO_MS = 220;

  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly pagina = viewChild.required<ElementRef<HTMLElement>>('pagina');
  private readonly barraInicio = viewChild.required<ElementRef<HTMLElement>>('barraInicio');
  private readonly botaoMenu = viewChild.required<ElementRef<HTMLButtonElement>>('botaoMenu');
  private readonly botaoFechar = viewChild.required<ElementRef<HTMLButtonElement>>('botaoFechar');
  /** Distância (px) que um arraste para a esquerda precisa para fechar a gaveta. */
  private static readonly ARRASTE_MINIMO = 60;
  private arraste: { x: number; y: number } | null = null;
  /** Gaveta do celular: sempre começa fechada. */
  readonly menuAberto = signal(false);
  /** Escolha do computador, lembrada no navegador. */
  readonly menuRecolhido = signal(this.lerRecolhido());
  readonly celular = signal(false);

  /** No celular a preferência não vale: lá quem manda é a gaveta. */
  readonly recolhidoNoComputador = computed(() => !this.celular() && this.menuRecolhido());
  readonly menuVisivel = computed(() => (this.celular() ? this.menuAberto() : !this.menuRecolhido()));

  readonly Icones = Icones;
  readonly roteiroDaTela = roteiroDaTela;

  /**
   * Quais telas aparecem no menu. Hoje todas; os módulos que se ligam e desligam (#190) trocam esta função, e a
   * tela de módulo desligado some do grupo (e o grupo, se ficar vazio).
   */
  readonly telaVisivel = signal<(tela: ItemMenu) => boolean>(() => true);
  readonly entradas = computed(() => filtrarMenu(MENU, this.telaVisivel()));

  /**
   * Grupos fechados, lembrados no navegador (ADR 0017). Guarda os fechados, não os abertos: o padrão é tudo aberto,
   * e um grupo novo (Trabalho, #209) aparece aberto para quem já tinha fechado outros.
   */
  readonly gruposFechados = signal<ReadonlySet<string>>(this.lerGruposFechados());

  private readonly url = toSignal(
    this.router.events.pipe(filter((e) => e instanceof NavigationEnd), map(() => this.router.url)),
    { initialValue: this.router.url });
  /** Capítulo e seção do manual da tela aberta (null no próprio manual). */
  readonly ajuda = computed(() => ajudaDaUrl(this.url()));
  /** Grupo da tela aberta (null no Painel e no Manual). */
  readonly grupoAtual = computed(() => grupoDaUrl(this.url())?.id ?? null);
  /** Última tela em que o grupo foi aberto sozinho: trocar só o filtro (mesma tela) não reabre o que a pessoa fechou. */
  private ultimaTela: string | null = null;

  constructor() {
    // O grupo da tela aberta abre sozinho ao entrar nela: pelo menu, por link de outra tela, pelo voltar ou pela URL.
    this.abrirGrupoDaTelaAtual(this.router.url);
    this.router.events.pipe(filter((e) => e instanceof NavigationEnd), takeUntilDestroyed())
      .subscribe(() => this.abrirGrupoDaTelaAtual(this.router.url));

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
      // Gaveta aberta: o foco vai para o ✕ (o conteúdo atrás fica inert).
      if (this.menuAberto()) afterNextRender(() => this.botaoFechar().nativeElement.focus(), { injector: this.injector });
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

  /** Fecha a gaveta do celular (✕, tocar fora, Esc, arrastar) e devolve o foco ao ☰, que abriu a gaveta. */
  fecharGaveta(): void {
    if (!this.menuAberto()) return;
    this.menuAberto.set(false);
    afterNextRender(() => this.botaoMenu().nativeElement.focus(), { injector: this.injector });
  }

  @HostListener('document:keydown.escape')
  aoApertarEsc(): void {
    if (this.celular()) this.fecharGaveta();
  }

  /** Arrastar a gaveta para a esquerda fecha, como nos apps de celular. */
  inicioArraste(e: PointerEvent): void {
    this.arraste = this.celular() && this.menuAberto() ? { x: e.clientX, y: e.clientY } : null;
  }

  fimArraste(e: PointerEvent): void {
    if (!this.arraste) return;
    const dx = e.clientX - this.arraste.x;
    const dy = e.clientY - this.arraste.y;
    this.arraste = null;
    if (dx < -ShellComponent.ARRASTE_MINIMO && Math.abs(dx) > Math.abs(dy) * 1.5) this.fecharGaveta();
  }

  grupoDe(entrada: EntradaMenu): GrupoMenu | null {
    return ehGrupo(entrada) ? entrada : null;
  }

  telaDe(entrada: EntradaMenu): ItemMenu | null {
    return ehGrupo(entrada) ? null : entrada;
  }

  estaAberto(id: string): boolean {
    return !this.gruposFechados().has(id);
  }

  /** Clique, Enter ou Espaço no cabeçalho do grupo. Não fecha a gaveta do celular: a pessoa ainda vai escolher a tela. */
  alternarGrupo(id: string): void {
    this.mudarGrupo(id, !this.estaAberto(id));
  }

  /**
   * Mostra a tela no menu: abre o grupo dela e, no celular, a gaveta. Para o roteiro (#194) destacar uma tela do
   * menu que estaria escondida.
   */
  mostrarTelaNoMenu(rota: string): void {
    const grupo = grupoDaUrl(rota);
    if (grupo) this.mudarGrupo(grupo.id, true);
    if (this.celular()) this.menuAberto.set(true);
  }

  private abrirGrupoDaTelaAtual(url: string): void {
    const tela = url.split(/[?#]/)[0];
    if (tela === this.ultimaTela) return;
    this.ultimaTela = tela;
    const grupo = grupoDaUrl(url);
    if (grupo && !this.estaAberto(grupo.id)) this.mudarGrupo(grupo.id, true);
  }

  private mudarGrupo(id: string, aberto: boolean): void {
    if (this.estaAberto(id) === aberto) return;
    const fechados = new Set(this.gruposFechados());
    if (aberto) fechados.delete(id);
    else fechados.add(id);
    this.gruposFechados.set(fechados);
    try {
      localStorage.setItem(ShellComponent.CHAVE_GRUPOS, JSON.stringify([...fechados]));
    } catch {
      /* armazenamento indisponível: vale só até recarregar */
    }
  }

  /** Lista de ids de grupos fechados (JSON). Qualquer coisa diferente disso vale o padrão: tudo aberto. */
  private lerGruposFechados(): ReadonlySet<string> {
    try {
      const salvo: unknown = JSON.parse(localStorage.getItem(ShellComponent.CHAVE_GRUPOS) ?? '[]');
      return new Set(Array.isArray(salvo) ? salvo.filter((id): id is string => typeof id === 'string') : []);
    } catch {
      return new Set();
    }
  }

  private lerRecolhido(): boolean {
    try {
      return localStorage.getItem(ShellComponent.CHAVE_RECOLHIDO) === '1';
    } catch {
      return false;
    }
  }
}
