import { Component, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ApiService } from '../core/api.service';
import { AuthService } from '../core/auth.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { AvisosService } from '../compartilhado/avisos.component';
import { BotaoPrivacidadeComponent } from '../compartilhado/botao-privacidade.component';
import { CalculadoraComponent } from '../compartilhado/calculadora.component';
import { DicaDirective } from '../compartilhado/dica.directive';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { MenuUsuarioComponent } from '../compartilhado/menu-usuario.component';
import { LucideIconData } from 'lucide-angular';

interface ItemMenu { rota: string; rotulo: string; icone: LucideIconData; }

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, BotaoPrivacidadeComponent, CalculadoraComponent, MenuUsuarioComponent,
    IconeComponent, DicaDirective],
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
            <button type="button" class="botao-barra botao-sincronizar" (click)="sincronizar()" [disabled]="sincronizando()"
              aria-label="Sincronizar bancos" appDica="Sincronizar bancos" dicaLado="abaixo">
              <app-icone [icone]="Icones.sincronizar" [tamanho]="18" [class.girando]="sincronizando()" />
              <span class="rotulo-barra">{{ sincronizando() ? 'Sincronizando…' : 'Sincronizar' }}</span>
            </button>
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
  readonly auth = inject(AuthService);
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly menuAberto = signal(false);
  readonly sincronizando = signal(false);

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

  sincronizar(): void {
    this.sincronizando.set(true);
    this.api.sincronizar().subscribe({
      next: (r) => {
        this.sincronizando.set(false);
        if (r.sucesso) {
          this.avisos.sucesso(`Sincronizado: ${r.transacoesNovas} nova(s), ${r.transacoesAtualizadas} atualizada(s).`);
        } else {
          this.avisos.erro(r.mensagem ?? 'Falha na sincronização.');
        }
      },
      error: (e) => {
        this.sincronizando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }
}
