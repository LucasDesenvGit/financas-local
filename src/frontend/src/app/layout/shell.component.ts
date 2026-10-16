import { Component, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ApiService } from '../core/api.service';
import { AuthService } from '../core/auth.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { AvisosService } from '../compartilhado/avisos.component';
import { SeletorTemaComponent } from '../compartilhado/seletor-tema.component';
import { BotaoPrivacidadeComponent } from '../compartilhado/botao-privacidade.component';
import { CalculadoraComponent } from '../compartilhado/calculadora.component';

interface ItemMenu { rota: string; rotulo: string; icone: string; }

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, SeletorTemaComponent, BotaoPrivacidadeComponent, CalculadoraComponent],
  template: `
    <div class="app" [class.menu-aberto]="menuAberto()">
      <aside class="lateral">
        <div class="marca">
          <span class="marca-simbolo" aria-hidden="true">F</span>
          <div>
            <strong>Finanças</strong>
            <small>local e privado</small>
          </div>
        </div>

        <nav class="menu" aria-label="Menu principal">
          @for (item of menu; track item.rota) {
            <a [routerLink]="item.rota" routerLinkActive="ativo" (click)="menuAberto.set(false)">
              <span class="menu-icone" aria-hidden="true">{{ item.icone }}</span>
              {{ item.rotulo }}
            </a>
          }
        </nav>

        <div class="lateral-rodape">
          <button type="button" class="botao botao-suave largura-total" (click)="sincronizar()" [disabled]="sincronizando()">
            <span aria-hidden="true" [class.girando]="sincronizando()">⟳</span>
            {{ sincronizando() ? 'Sincronizando…' : 'Sincronizar bancos' }}
          </button>
          <app-botao-privacidade />
          <app-seletor-tema />
          <div class="usuario">
            <span>{{ auth.estado()?.login }}</span>
            <button type="button" class="link" (click)="auth.sair()">Sair</button>
          </div>
        </div>
      </aside>

      <div class="conteudo">
        <header class="barra-mobile">
          <button type="button" class="botao-icone" (click)="menuAberto.set(!menuAberto())" aria-label="Abrir menu">☰</button>
          <strong class="cresce">Finanças</strong>
          <app-botao-privacidade />
          <app-seletor-tema />
        </header>
        <main class="pagina">
          <router-outlet />
        </main>
      </div>
    </div>
    <app-calculadora />
  `
})
export class ShellComponent {
  readonly auth = inject(AuthService);
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly menuAberto = signal(false);
  readonly sincronizando = signal(false);

  readonly menu: ItemMenu[] = [
    { rota: '/painel', rotulo: 'Painel', icone: '◧' },
    { rota: '/anual', rotulo: 'Visão anual', icone: '◫' },
    { rota: '/transacoes', rotulo: 'Transações', icone: '≡' },
    { rota: '/contas', rotulo: 'Contas e cartões', icone: '▭' },
    { rota: '/orcamentos', rotulo: 'Orçamentos', icone: '◔' },
    { rota: '/metas', rotulo: 'Metas', icone: '◎' },
    { rota: '/contas-a-pagar', rotulo: 'Contas a pagar', icone: '◷' },
    { rota: '/investimentos', rotulo: 'Investimentos', icone: '△' },
    { rota: '/emprestimos', rotulo: 'Empréstimos e financiamentos', icone: '⊟' },
    { rota: '/relatorios', rotulo: 'Relatórios', icone: '▤' },
    { rota: '/projecao', rotulo: 'Projeção', icone: '↗' },
    { rota: '/categorias', rotulo: 'Categorias e regras', icone: '◈' },
    { rota: '/configuracoes', rotulo: 'Configurações', icone: '⚙' }
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
