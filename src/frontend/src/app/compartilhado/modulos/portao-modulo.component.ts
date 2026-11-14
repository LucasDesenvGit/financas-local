import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink, RouterOutlet } from '@angular/router';
import { ModulosService, listarDestinos } from '../../core/modulos.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';

/**
 * #191 (ADR 0064): a rota de um módulo fica dentro deste portão (`data: { modulo: '<id>' }` na rota-mãe). Ligado, mostra
 * a tela; desligado, o aviso, com o que o módulo faz, o que ele acessa na internet e o botão para ligar ali mesmo
 * (Nielsen 3, controle e liberdade; Nielsen 1, visibilidade do estado). Como a tela nem é criada com ele desligado, ela
 * não chama a API. Ligou: a tela aparece sem recarregar a página.
 */
@Component({
  selector: 'app-portao-modulo',
  imports: [RouterOutlet, RouterLink],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    @if (modulos.lista() === null) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (modulo(); as m) {
      @if (m.ligado) {
        <router-outlet />
      } @else {
        <header class="cabecalho-pagina"><h1>{{ m.nome }}</h1></header>
        <section class="painel-bloco modulo-desligado" aria-labelledby="titulo-modulo-desligado">
          <h2 id="titulo-modulo-desligado">Este módulo está desligado</h2>
          <p>{{ m.descricao }}</p>
          @if (m.acessaInternet) {
            <p>Ligado, ele acessa a internet: {{ destinos() }}. Desligado, nada sai do computador.</p>
          } @else {
            <p class="texto-suave">Ele não acessa a internet: tudo fica neste computador.</p>
          }
          <div class="rodape-form">
            <button type="button" class="botao botao-primario" [disabled]="ligando()" (click)="ligar(m.id)">Ligar {{ m.nome }}</button>
            <a class="link" routerLink="/configuracoes" [queryParams]="{ aba: 'modulos' }">Ver todos os módulos</a>
          </div>
        </section>
      }
    } @else {
      <section class="painel-bloco">
        <h1>Módulo não encontrado</h1>
        <p>Este módulo não existe nesta versão do site.</p>
        <a class="link" routerLink="/configuracoes" [queryParams]="{ aba: 'modulos' }">Ver os módulos</a>
      </section>
    }
  `
})
export class PortaoModuloComponent {
  readonly modulos = inject(ModulosService);
  private readonly avisos = inject(AvisosService);
  private readonly id: string = inject(ActivatedRoute).snapshot.data['modulo'];
  readonly modulo = computed(() => this.modulos.modulo(this.id));
  readonly destinos = computed(() => listarDestinos(this.modulo()?.destinos ?? []));
  readonly ligando = signal(false);

  constructor() {
    this.modulos.carregar();
  }

  ligar(id: string): void {
    this.ligando.set(true);
    this.modulos.definir(id, true).subscribe({
      next: (m) => { this.ligando.set(false); this.avisos.sucesso(`${m.nome} ligado.`); },
      error: (e) => { this.ligando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }
}
