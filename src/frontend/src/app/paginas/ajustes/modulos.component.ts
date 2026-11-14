import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { ModulosService, listarDestinos } from '../../core/modulos.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { Modulo } from '../../core/modelos';

/**
 * #191 (ADR 0064): Configurações › Módulos. Um interruptor por módulo, com efeito imediato (ADR 0032; Kendrick, NN/g,
 * Toggle-Switch Guidelines), o que ele faz e, se acessa a internet, para onde (Nielsen 1). Desligado, o módulo some do
 * menu, não roda nada em segundo plano e não acessa a internet; os dados dele ficam guardados.
 */
@Component({
  selector: 'app-modulos',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="painel-bloco" aria-labelledby="titulo-modulos">
      <h2 id="titulo-modulos">Módulos</h2>
      <p class="texto-suave">Partes do site que vão além do básico das finanças. Desligado, o módulo some do menu, não roda nada
        em segundo plano e não acessa a internet. Os dados dele ficam guardados para quando você ligar de novo.</p>
      @if (modulos.lista(); as lista) {
        @if (lista.length) {
          <ul class="lista-modulos">
            @for (m of lista; track m.id) {
              <li class="modulo-item">
                <label class="interruptor">
                  <input type="checkbox" role="switch" [checked]="m.ligado" [disabled]="mudando() === m.id"
                    [attr.aria-describedby]="'modulo-' + m.id + '-descricao'" (change)="alternar(m, $event)">
                  <strong>{{ m.nome }}</strong>
                </label>
                <p class="texto-suave" [id]="'modulo-' + m.id + '-descricao'">{{ m.descricao }}
                  @if (m.acessaInternet) { Acessa a internet: {{ destinos(m) }}. } @else { Não acessa a internet. }
                </p>
              </li>
            }
          </ul>
        } @else {
          <p>Nenhum módulo nesta versão do site.</p>
        }
      } @else {
        <p class="texto-suave" role="status">Carregando…</p>
      }
    </section>
  `,
  styles: [`
    .lista-modulos { list-style: none; margin: 0; padding: 0; display: grid; gap: 1rem; }
    .modulo-item p { margin: .25rem 0 0; }
  `]
})
export class ModulosComponent {
  readonly modulos = inject(ModulosService);
  private readonly avisos = inject(AvisosService);
  /** O id do módulo que está sendo gravado: o interruptor dele fica parado até a API responder. */
  readonly mudando = signal<string | null>(null);

  constructor() {
    this.modulos.carregar();
  }

  destinos(m: Modulo): string {
    return listarDestinos(m.destinos);
  }

  alternar(m: Modulo, evento: Event): void {
    const caixa = evento.target as HTMLInputElement;
    const ligado = caixa.checked;
    this.mudando.set(m.id);
    this.modulos.definir(m.id, ligado).subscribe({
      next: (novo) => {
        this.mudando.set(null);
        this.avisos.sucesso(novo.ligado ? `${novo.nome} ligado.` : `${novo.nome} desligado. Os dados dele ficam guardados.`);
      },
      error: (e) => {
        this.mudando.set(null);
        caixa.checked = !ligado; // volta ao que está gravado
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }
}
