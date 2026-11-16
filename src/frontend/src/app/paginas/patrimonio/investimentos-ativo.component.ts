import { ChangeDetectionStrategy, Component, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { AtualizarAtivo, PosicaoAtivo, TipoAtivo } from '../../core/modelos';
import { SEGMENTOS, formularioDoAtivo, nomeTipo, tiposPara } from './investimentos';

/**
 * A janela "Ativo · TICKER" de Investimentos: nome, tipo, segmento e cotação manual (#731, ADR 0059). Componente de
 * apresentação, como a janela de lançamento de Transações: edita uma cópia do formulário (refeita a cada ativo novo
 * que chega) e avisa a tela com ela; fechar sem salvar não muda nada (Nielsen 3). Quem fala com a API e confirma a
 * exclusão é a tela. Sem ativo, a janela fica fechada.
 */
@Component({
  selector: 'app-investimentos-ativo',
  imports: [ListaSuspensaDirective, FormsModule, ModalComponent, CampoValorComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="form() !== null" [titulo]="'Ativo · ' + (ativo()?.ticker ?? '')" (fechar)="fechar.emit()">
      @if (form(); as fa) {
        <form (ngSubmit)="salvar.emit(fa)" #fat="ngForm" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fa.nome" maxlength="150"></label>
          <label>Tipo
            <select name="tipo" [(ngModel)]="fa.tipo">
              @for (t of tiposPara(ativo()?.moeda); track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
            </select>
          </label>
          @if (fa.tipo === TipoAtivo.Fii) {
            <label>Segmento
              <select name="segmento" [(ngModel)]="fa.segmento">
                <option [ngValue]="null">Não informado</option>
                @for (sg of segmentos; track sg) { <option [ngValue]="sg">{{ sg }}</option> }
              </select>
            </label>
          }
          <label class="checkbox"><input type="checkbox" name="manual" [(ngModel)]="fa.precoManual"> Informar a cotação manualmente</label>
          @if (fa.precoManual) {
            <label>Cotação atual <app-campo-valor name="preco" [(ngModel)]="fa.precoAtual" [moeda]="ativo()?.moeda" [casas]="4"
              obrigatorio [minimo]="0.0001" /></label>
            <p class="texto-suave">Útil quando a fonte automática não encontra o ativo. Desmarque para voltar à cotação automática.</p>
          }
          <div class="rodape-form">
            <button type="button" class="botao botao-perigo" (click)="excluir.emit()">Excluir ativo</button>
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fat.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class InvestimentosAtivoComponent {
  /** O ativo da janela; null fecha. */
  readonly ativo = input<PosicaoAtivo | null>(null);
  readonly salvar = output<AtualizarAtivo>();
  readonly excluir = output<void>();
  readonly fechar = output<void>();

  /** A cópia que a janela edita: refeita a cada ativo novo que chega. */
  readonly form = linkedSignal<AtualizarAtivo | null>(() => {
    const a = this.ativo();
    return a ? formularioDoAtivo(a) : null;
  });

  readonly TipoAtivo = TipoAtivo;
  readonly segmentos = SEGMENTOS;
  readonly tiposPara = tiposPara;
  readonly nomeTipo = nomeTipo;
}
