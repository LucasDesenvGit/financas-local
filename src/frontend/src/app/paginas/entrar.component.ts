import { Component, OnInit, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { AuthService } from '../core/auth.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { SeletorTemaComponent } from '../compartilhado/seletor-tema.component';

@Component({
  selector: 'app-entrar',
  imports: [FormsModule, SeletorTemaComponent],
  template: `
    <div class="tela-entrada">
      <div class="tela-entrada-tema"><app-seletor-tema /></div>
      <div class="cartao-entrada">
        <div class="marca marca-grande">
          <span class="marca-simbolo" aria-hidden="true">F</span>
          <div>
            <strong>Finanças Local</strong>
            <small>seus dados ficam no seu computador</small>
          </div>
        </div>

        @if (carregando()) {
          <p class="texto-suave">Carregando…</p>
        } @else if (!configurado()) {
          <h1>Primeiro acesso</h1>
          <p class="texto-suave">Crie o usuário que vai proteger o sistema. A senha precisa ter pelo menos 8 caracteres.</p>
          <form (ngSubmit)="configurar()" #f="ngForm">
            <label>Usuário
              <input name="login" [(ngModel)]="login" required minlength="3" maxlength="100" autocomplete="username">
            </label>
            <label>Senha
              <input name="senha" type="password" [(ngModel)]="senha" required minlength="8" autocomplete="new-password">
            </label>
            <label>Confirmar senha
              <input name="confirmacao" type="password" [(ngModel)]="confirmacao" required autocomplete="new-password">
            </label>
            @if (erro()) { <p class="erro">{{ erro() }}</p> }
            <button class="botao botao-primario largura-total" [disabled]="enviando() || f.invalid">Criar e entrar</button>
          </form>
        } @else {
          <h1>Entrar</h1>
          @if (expirada()) { <p class="aviso-inline">Sua sessão expirou. Entre novamente.</p> }
          <form (ngSubmit)="entrar()" #f="ngForm">
            <label>Usuário
              <input name="login" [(ngModel)]="login" required autocomplete="username">
            </label>
            <label>Senha
              <input name="senha" type="password" [(ngModel)]="senha" required autocomplete="current-password">
            </label>
            @if (erro()) { <p class="erro">{{ erro() }}</p> }
            <button class="botao botao-primario largura-total" [disabled]="enviando() || f.invalid">Entrar</button>
          </form>
        }
      </div>
    </div>
  `
})
export class EntrarComponent implements OnInit {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  /** Query string ?expirada=1 (vinculada pelo router). */
  readonly expirada = input<string | undefined>();

  readonly carregando = signal(true);
  readonly configurado = signal(true);
  readonly enviando = signal(false);
  readonly erro = signal<string | null>(null);

  login = '';
  senha = '';
  confirmacao = '';

  async ngOnInit(): Promise<void> {
    try {
      const estado = await this.auth.carregarEstado();
      this.configurado.set(estado.configurado);
    } catch (e) {
      this.erro.set(mensagemDeErro(e));
    } finally {
      this.carregando.set(false);
    }
  }

  entrar(): void {
    this.enviar(() => this.auth.entrar(this.login.trim(), this.senha));
  }

  configurar(): void {
    if (this.senha !== this.confirmacao) {
      this.erro.set('As senhas não conferem.');
      return;
    }
    this.enviar(() => this.auth.configurar(this.login.trim(), this.senha));
  }

  private enviar(acao: () => ReturnType<AuthService['entrar']>): void {
    this.erro.set(null);
    this.enviando.set(true);
    acao().subscribe({
      next: () => {
        this.senha = '';
        this.confirmacao = '';
        void this.router.navigateByUrl('/painel');
      },
      error: (e) => {
        this.enviando.set(false);
        this.senha = '';
        this.erro.set(mensagemDeErro(e));
        // Renova o token CSRF caso a sessão anterior tenha expirado.
        void this.auth.carregarEstado().catch(() => undefined);
      }
    });
  }
}
