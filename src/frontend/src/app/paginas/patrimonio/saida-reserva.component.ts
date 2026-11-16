import { ChangeDetectionStrategy, Component, OnInit, computed, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { Categoria, Reserva, SaidaReserva, TipoCategoria } from '../../core/modelos';

/** O formulário da saída, sem a reserva (que vem de fora). */
interface FormSaida { descricao: string; valor: number | null; data: string; categoriaId: number | null }

/**
 * #691 (ADR 0063): "Registrar saída" de um cofrinho, o gasto pago direto da reserva (ex.: uma compra com o "Cofrinho
 * Mercado Livre") que não chega pelo Open Finance. Componente de apresentação (ADR 0059): recebe a reserva e as
 * categorias, e avisa a tela com o que salvar. A tela o cria a cada abertura (dentro de um @if), e o formulário começa
 * limpo, com a data de hoje.
 */
@Component({
  selector: 'app-saida-reserva',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, ModalComponent, CampoValorComponent, CampoDataComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="reserva() !== null" [titulo]="'Registrar saída · ' + (reserva()?.nome ?? '')" (fechar)="fechar.emit()">
      @if (reserva(); as r) {
        <form (ngSubmit)="enviar(r)" #fs="ngForm" class="formulario">
          <p class="texto-suave">Um gasto que saiu direto deste cofrinho, sem passar pela conta {{ r.nomeConta }}. Ele entra
            nos relatórios e nos orçamentos, e a estimativa do cofrinho baixa ({{ r.saldo | currency }} hoje).</p>
          <label>Descrição <input name="descricao" [(ngModel)]="form.descricao" required maxlength="300"
            placeholder="ex.: compra no Mercado Livre"></label>
          <div class="grade-campos">
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="form.valor" obrigatorio [minimo]="0.01" /></label>
            <label>Data <app-campo-data name="data" [(ngModel)]="form.data" obrigatorio [maximo]="hoje()" /></label>
          </div>
          <label>Categoria (opcional)
            <select name="categoria" [(ngModel)]="form.categoriaId">
              <option [ngValue]="null">—</option>
              @for (c of categoriasDespesa(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fs.invalid || salvando()">Registrar</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class SaidaReservaComponent implements OnInit {
  /** A reserva da saída; null fecha o formulário. */
  readonly reserva = input<Reserva | null>(null);
  readonly categorias = input<Categoria[]>([]);
  readonly hoje = input.required<string>();
  readonly salvando = input(false);
  readonly salvar = output<SaidaReserva>();
  readonly fechar = output<void>();

  readonly categoriasDespesa = computed(() => this.categorias().filter((c) => c.tipo === TipoCategoria.Despesa && c.ativa));
  form: FormSaida = { descricao: '', valor: null, data: '', categoriaId: null };

  ngOnInit(): void {
    this.form.data = this.hoje();
  }

  enviar(r: Reserva): void {
    if (this.form.valor === null) return;
    this.salvar.emit({ contaId: r.contaId, nome: r.nome, data: this.form.data, valor: Number(this.form.valor),
      descricao: this.form.descricao.trim(), categoriaId: this.form.categoriaId });
  }
}
