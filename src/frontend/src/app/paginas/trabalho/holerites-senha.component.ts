import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';

/**
 * Bloco "O PDF tem senha" de Holerites (#777, ADR 0059). Componente de apresentação: a senha só vive no campo e sai
 * por `abrir`; não é guardada nem chamada à API aqui. O campo limpa ao enviar (a senha errada volta vazia).
 */
@Component({
  selector: 'app-holerites-senha',
  imports: [FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    .senha-pdf input { max-width: 20em; }
  `,
  template: `
    <section class="painel-bloco" aria-labelledby="titulo-senha-pdf">
      <h2 id="titulo-senha-pdf">O PDF tem senha</h2>
      <form class="senha-pdf" (ngSubmit)="abrir()" #fs="ngForm">
        <label>Senha de {{ arquivo() }}
          <input type="password" name="senhaPdf" autocomplete="off" [(ngModel)]="senha" required></label>
        <p class="texto-suave">Costuma ser parte do CPF ou a data de nascimento. Ela só abre o arquivo: não é guardada.</p>
        <div class="rodape-form">
          <button type="button" class="botao" [disabled]="ocupado()" (click)="pular.emit()">{{ restantes() ? 'Pular este' : 'Cancelar' }}</button>
          <span class="cresce"></span>
          <button class="botao botao-primario" [disabled]="fs.invalid || ocupado()">Abrir</button>
        </div>
      </form>
    </section>
  `
})
export class HoleritesSenhaComponent {
  readonly arquivo = input.required<string>();
  /** Quantos PDFs esperam depois deste: com fila o botão é "Pular este", sem fila é "Cancelar". */
  readonly restantes = input(0);
  readonly ocupado = input(false);

  readonly abrirComSenha = output<string>();
  readonly pular = output<void>();

  senha = '';

  abrir(): void {
    this.abrirComSenha.emit(this.senha);
    this.senha = '';
  }
}
