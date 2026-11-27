import { DatePipe } from '@angular/common';
import { Component, ChangeDetectionStrategy, input, linkedSignal, output, untracked } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { Conta, ContaAgendada, Reserva } from '../../core/modelos';
import {
  AberturaPagamento, OpcaoCofrinho, PagamentoParaApi, corpoDoPagamento, lancarNoPagar, opcoesCofrinho, podeLancar, textoOpcaoCofrinho
} from './pago-com-cofrinho';

/**
 * Janela "Pagar" de Contas a pagar (#777, ADR 0059). Componente de apresentação: recebe a conta aberta, as contas e os
 * cofrinhos, guarda a data e as escolhas e avisa em `confirmar` com o que vai para a API; não chama a API nem lê a
 * rota. Fechar sem confirmar não muda nada.
 */
@Component({
  selector: 'app-contas-agendadas-pagamento',
  imports: [ListaSuspensaDirective, FormsModule, DatePipe, ModalComponent, CampoDataComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="abertura() !== null" [titulo]="'Pagar · ' + (abertura()?.conta?.descricao ?? '')" (fechar)="fechar.emit()">
      @if (abertura(); as ab) {
        @let pg = ab.conta;
        <form (ngSubmit)="confirmarPagamento(pg)" #fp="ngForm" class="formulario">
          <label>Data do pagamento <app-campo-data name="data" [(ngModel)]="data" obrigatorio /></label>
          @if (pg.pagamentosRestantes === 1) {
            <p class="texto-suave" role="status">É o último pagamento (até {{ pg.dataLimite | date: 'dd/MM/yyyy' }}): a conta será encerrada.</p>
          }
          @if (cofrinho() !== null) {
            <!-- #691: "foi paga com um cofrinho?": escolher em vez de digitar (Nielsen 6), e guardar para as próximas. -->
            @if (opcoes(pg).length) {
              <label>Qual cofrinho
                <select name="cofrinhoPagamento" [(ngModel)]="cofrinho" required>
                  @for (r of opcoes(pg); track r.nome) { <option [ngValue]="r.nome">{{ textoOpcaoCofrinho(r) }}</option> }
                </select>
              </label>
            } @else {
              <label>Nome do cofrinho
                <input name="cofrinhoPagamento" [(ngModel)]="cofrinho" required maxlength="100" placeholder="ex.: Emergência"></label>
            }
            <label class="checkbox"><input type="checkbox" name="guardarCofrinho" [(ngModel)]="guardarCofrinho">
              Usar este cofrinho nas próximas (o app lança no vencimento)</label>
            <p class="texto-suave">A despesa é lançada na conta {{ pg.contaNome }}, saindo do cofrinho: o saldo da conta não muda.</p>
          } @else if (podeLancar(pg) && pg.reservaNome) {
            <label class="checkbox"><input type="checkbox" name="lancar" [(ngModel)]="lancar">
              Lançar a despesa saindo do cofrinho {{ pg.reservaNome }} ({{ pg.contaNome }})</label>
          } @else if (podeLancar(pg)) {
            <label class="checkbox"><input type="checkbox" name="lancar" [(ngModel)]="lancar">
              Lançar a despesa na conta {{ pg.contaNome }}</label>
          } @else {
            <p class="texto-suave">Se o pagamento sair de uma conta do Open Finance, a transação chega pela sincronização.</p>
          }
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fp.invalid">Confirmar pagamento</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class ContasAgendadasPagamentoComponent {
  /** A conta aberta, com a data e o cofrinho de partida (null = janela fechada). */
  readonly abertura = input<AberturaPagamento | null>(null);
  readonly contas = input<Conta[]>([]);
  /** #691: os cofrinhos das contas. */
  readonly reservas = input<Reserva[]>([]);

  /** "Confirmar pagamento", com o que vai para a API. */
  readonly confirmar = output<PagamentoParaApi>();
  readonly fechar = output<void>();

  readonly podeLancar = (c: ContaAgendada) => podeLancar(c, this.contas());
  readonly textoOpcaoCofrinho = textoOpcaoCofrinho;

  /** As escolhas da janela: refeitas a cada "Pagar" que abre (a data, a caixa de lançar e o cofrinho vêm da abertura). */
  readonly data = linkedSignal(() => this.abertura()?.data ?? '');
  readonly lancar = linkedSignal(() => {
    const ab = this.abertura();
    return ab ? untracked(() => lancarNoPagar(ab, this.contas())) : false;
  });
  /** #691: o cofrinho escolhido no "Pagar" aberto pelo aviso (null = o "Pagar" de sempre). */
  readonly cofrinho = linkedSignal<string | null>(() => this.abertura()?.cofrinho ?? null);
  readonly guardarCofrinho = linkedSignal(() => {
    this.abertura();
    return true;
  });

  opcoes(c: ContaAgendada): OpcaoCofrinho[] {
    return opcoesCofrinho(this.reservas(), c.contaId, null);
  }

  confirmarPagamento(conta: ContaAgendada): void {
    this.confirmar.emit(corpoDoPagamento(conta, this.contas(), {
      data: this.data(), lancar: this.lancar(), cofrinho: this.cofrinho(), guardarCofrinho: this.guardarCofrinho()
    }));
  }
}
