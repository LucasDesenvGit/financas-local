import { Component, ElementRef, HostListener, computed, inject, signal, viewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideIconData } from 'lucide-angular';
import { AuthService } from '../core/auth.service';
import { PreferenciaTema, TemaService } from '../core/tema.service';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

/**
 * UX-7: menu do usuário na barra superior (iniciais ▾) com as ações raras: tema, aparência e sair.
 * Padrão de menu do WAI-ARIA: Enter/Espaço/↓ abrem e focam o 1º item, ↑/↓ navegam, Esc fecha e devolve o foco
 * ao botão, Tab fecha; clique fora também fecha. Nielsen 8: ações raras saem da frente.
 */
@Component({
  selector: 'app-menu-usuario',
  imports: [RouterLink, IconeComponent],
  template: `
    <button #botao type="button" class="botao-barra menu-usuario-botao" aria-haspopup="menu" [attr.aria-expanded]="aberto()"
      aria-controls="menu-usuario" [attr.aria-label]="'Menu do usuário ' + (login() ?? '')" (click)="alternar()"
      (keydown.arrowdown)="abrir(0, $event)" (keydown.arrowup)="abrir(-1, $event)">
      <span class="avatar" aria-hidden="true">{{ iniciais() }}</span>
      <app-icone [icone]="Icones.abrirMenu" [tamanho]="14" />
    </button>

    @if (aberto()) {
      <div #menu id="menu-usuario" class="menu-flutuante" role="menu" [attr.aria-label]="'Conta ' + (login() ?? '')" animate.leave="sair"
        (keydown)="teclado($event)">
        <div class="menu-cabecalho" role="presentation">
          <span class="avatar" aria-hidden="true">{{ iniciais() }}</span>
          <div><strong>{{ login() }}</strong><small>conectado neste computador</small></div>
        </div>
        <div class="menu-grupo" role="group" aria-label="Tema">
          <span class="menu-titulo" aria-hidden="true">Tema</span>
          @for (o of temas; track o.valor) {
            <button type="button" role="menuitemradio" class="menu-item" [attr.aria-checked]="tema.preferencia() === o.valor"
              tabindex="-1" (click)="tema.definir(o.valor)">
              <app-icone [icone]="o.icone" [tamanho]="16" /> <span class="cresce">{{ o.rotulo }}</span>
              @if (tema.preferencia() === o.valor) { <app-icone [icone]="Icones.marcado" [tamanho]="16" /> }
            </button>
          }
        </div>
        <div class="menu-grupo" role="group">
          <a role="menuitem" class="menu-item" tabindex="-1" routerLink="/configuracoes" [queryParams]="{ aba: 'aparencia' }"
            (click)="fechar(false)">
            <app-icone [icone]="Icones.aparencia" [tamanho]="16" /> Cores e aparência…
          </a>
          <button type="button" role="menuitem" class="menu-item" tabindex="-1" (click)="sair()">
            <app-icone [icone]="Icones.sair" [tamanho]="16" /> Sair
          </button>
        </div>
      </div>
    }
  `,
  host: { class: 'menu-usuario' }
})
export class MenuUsuarioComponent {
  private readonly auth = inject(AuthService);
  readonly tema = inject(TemaService);
  private readonly el = inject(ElementRef<HTMLElement>);
  private readonly botao = viewChild.required<ElementRef<HTMLButtonElement>>('botao');
  private readonly menu = viewChild<ElementRef<HTMLElement>>('menu');

  readonly Icones = Icones;
  readonly aberto = signal(false);
  readonly login = computed(() => this.auth.estado()?.login ?? null);
  readonly iniciais = computed(() => (this.login() ?? '?').replace(/[^\p{L}\p{N}]/gu, '').slice(0, 2).toUpperCase() || '?');
  readonly temas: { valor: PreferenciaTema; rotulo: string; icone: LucideIconData }[] = [
    { valor: 'claro', rotulo: 'Claro', icone: Icones.claro },
    { valor: 'escuro', rotulo: 'Escuro', icone: Icones.escuro },
    { valor: 'sistema', rotulo: 'Automático (do sistema)', icone: Icones.sistema }
  ];

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

  async sair(): Promise<void> {
    this.fechar(false);
    await this.auth.sair();
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
    if (this.aberto() && !(this.el.nativeElement as HTMLElement).contains(ev.target as Node)) this.fechar(false);
  }

  private itens(): HTMLElement[] {
    const lista = this.menu()?.nativeElement.querySelectorAll<HTMLElement>('[role^="menuitem"]');
    return lista ? Array.from(lista) : [];
  }

  private focar(indice: number): void {
    const itens = this.itens();
    if (!itens.length) return;
    itens[indice < 0 ? itens.length - 1 : indice]?.focus();
  }
}
