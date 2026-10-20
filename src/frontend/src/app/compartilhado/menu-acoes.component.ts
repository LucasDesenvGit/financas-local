import { ChangeDetectionStrategy, Component, ElementRef, HostListener, inject, input, output, signal, viewChild } from '@angular/core';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

let sequencia = 0;

/** Uma opção do menu: `detalhe` é a segunda linha, menor (o que acontece ao escolher). */
export interface AcaoMenu { id: string; rotulo: string; detalhe?: string; }

/**
 * Botão com menu de ações ("Ocultar ▾" → Minimizar / Ocultar até…), no padrão Menu Button do WAI-ARIA, o mesmo do
 * menu do usuário (UX-7): Enter/Espaço/↓ abrem e focam o 1º item, ↑/↓/Home/End navegam, Esc fecha e devolve o foco
 * ao botão, Tab fecha; clique fora também fecha. Quem usa este componente precisa de `position: relative` em volta
 * e sem `overflow: hidden`, senão o menu fica cortado.
 */
@Component({
  selector: 'app-menu-acoes',
  imports: [IconeComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <button #botao type="button" class="link menu-acoes-botao" aria-haspopup="menu" [attr.aria-expanded]="aberto()"
      [attr.aria-controls]="id" [attr.aria-label]="rotuloAcessivel() || null" (click)="alternar()"
      (keydown.arrowdown)="abrir(0, $event)" (keydown.arrowup)="abrir(-1, $event)">
      {{ rotulo() }} <app-icone [icone]="Icones.abrirMenu" [tamanho]="14" />
    </button>
    @if (aberto()) {
      <div #menu [id]="id" class="menu-flutuante menu-acoes" role="menu" [attr.aria-label]="rotuloAcessivel() || rotulo()"
        animate.leave="sair" (keydown)="teclado($event)">
        @for (a of acoes(); track a.id) {
          <button type="button" role="menuitem" class="menu-item" tabindex="-1" (click)="escolher(a.id)">
            <span><span class="menu-acoes-rotulo">{{ a.rotulo }}</span>
              @if (a.detalhe) { <small>{{ a.detalhe }}</small> }</span>
          </button>
        }
      </div>
    }
  `,
  styles: [`
    :host { position: relative; display: inline-flex; }
    .menu-acoes-botao { display: inline-flex; align-items: center; gap: 4px; }
    .menu-acoes { width: 270px; top: calc(100% + 6px); }
    .menu-acoes .menu-item { min-height: 44px; padding: 6px 10px; }
    .menu-acoes-rotulo { display: block; }
    .menu-acoes small { display: block; color: var(--texto-suave); font-size: 0.78rem; }
  `]
})
export class MenuAcoesComponent {
  readonly Icones = Icones;
  readonly rotulo = input.required<string>();
  /** Nome do botão para leitor de tela, quando o rótulo visível sozinho não diz do quê (ex.: "Ocultar os avisos de orçamento"). */
  readonly rotuloAcessivel = input('');
  readonly acoes = input.required<AcaoMenu[]>();
  readonly escolhida = output<string>();

  readonly id = `menu-acoes-${++sequencia}`;
  readonly aberto = signal(false);
  private readonly host = inject(ElementRef<HTMLElement>);
  private readonly botao = viewChild.required<ElementRef<HTMLButtonElement>>('botao');
  private readonly menu = viewChild<ElementRef<HTMLElement>>('menu');

  alternar(): void {
    if (this.aberto()) this.fechar(true);
    else this.abrir(0);
  }

  /** Abre e foca o item indicado (0 = primeiro, -1 = último). */
  abrir(indice: number, ev?: Event): void {
    ev?.preventDefault();
    this.aberto.set(true);
    setTimeout(() => this.focar(indice));
  }

  fechar(devolverFoco: boolean): void {
    if (!this.aberto()) return;
    this.aberto.set(false);
    if (devolverFoco) this.botao().nativeElement.focus();
  }

  escolher(id: string): void {
    this.fechar(false);
    this.escolhida.emit(id);
  }

  teclado(ev: KeyboardEvent): void {
    const itens = this.itens();
    const atual = itens.indexOf(document.activeElement as HTMLElement);
    switch (ev.key) {
      case 'ArrowDown': ev.preventDefault(); this.focar((atual + 1) % itens.length); break;
      case 'ArrowUp': ev.preventDefault(); this.focar((atual - 1 + itens.length) % itens.length); break;
      case 'Home': ev.preventDefault(); this.focar(0); break;
      case 'End': ev.preventDefault(); this.focar(-1); break;
      case 'Escape': ev.preventDefault(); ev.stopPropagation(); this.fechar(true); break;
      case 'Tab': this.fechar(false); break;
    }
  }

  @HostListener('document:click', ['$event'])
  cliqueFora(ev: MouseEvent): void {
    if (this.aberto() && !(this.host.nativeElement as HTMLElement).contains(ev.target as Node)) this.fechar(false);
  }

  private itens(): HTMLElement[] {
    const lista = this.menu()?.nativeElement.querySelectorAll<HTMLElement>('[role="menuitem"]');
    return lista ? Array.from(lista) : [];
  }

  private focar(indice: number): void {
    const itens = this.itens();
    if (!itens.length) return;
    itens[indice < 0 ? itens.length - 1 : indice]?.focus();
  }
}
