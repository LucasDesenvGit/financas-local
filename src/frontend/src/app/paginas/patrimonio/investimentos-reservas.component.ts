import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { Reserva } from '../../core/modelos';
import { podeConferirReserva, podeRegistrarSaida } from './reserva-conferencia';

/**
 * A aba "Reservas" de Investimentos: a tabela dos cofrinhos, com a base de cada saldo (#731, ADR 0059). Componente de
 * apresentação: recebe as reservas e avisa "conferir saldo" e "registrar saída"; as janelas dessas ações são da tela.
 */
@Component({
  selector: 'app-investimentos-reservas',
  imports: [MoedaPipe, DatePipe, DecimalPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    tr.encerrada td { opacity: 0.6; }
    .nowrap { white-space: nowrap; }
    .destaque-conferir { font-weight: 600; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
  `,
  template: `
    <div class="tabela-rolagem">
      <table class="tabela">
        <thead>
          <tr><th>Reserva</th><th>Conta</th><th class="direita">Guardado</th><th class="direita">Retirado</th>
            <th class="direita">Saldo</th><th>Base do saldo</th><th></th></tr>
        </thead>
        <tbody>
          @for (r of reservas(); track r.contaId + '|' + r.nome) {
            <tr [class.encerrada]="r.saldo === 0">
              <td><strong>{{ r.nome }}</strong>
                <br><small class="texto-suave">{{ r.movimentos }} lançamento(s)
                  @if (r.primeiroMovimento) { de {{ r.primeiroMovimento | date: 'dd/MM/yy' }} a {{ r.ultimoMovimento | date: 'dd/MM/yy' }} }
                </small>
                <!-- #169: o Méli Dólar mostra a quantidade em dólar e a PTAX que deu o valor em reais (Nielsen 1). -->
                @if (r.moeda === 'USD') {
                  <br><small class="texto-suave">{{ r.quantidade | currency: 'USD' }}
                    @if (r.cotacao) { × <app-termo chave="ptax">PTAX</app-termo> {{ r.cotacao | number: '1.4-4' }} }</small>
                }</td>
              <td class="texto-suave">{{ r.nomeConta }}</td>
              <td class="direita">{{ r.guardado | currency }}</td>
              <td class="direita">{{ r.retirado | currency }}</td>
              <td class="direita valor">{{ r.saldo | currency }}</td>
              <td>
                @if (r.informadoPeloBanco) {
                  <span class="etiqueta etiqueta-sucesso">informado pelo banco</span>
                  <br><small class="texto-suave">valor que a instituição manda pelo Open Finance</small>
                } @else if (r.restanteDoBanco) {
                  <!-- #237: o banco manda só o total; esta linha é o que ele passa da soma das reservas acima. -->
                  <span class="etiqueta">restante do total do banco</span>
                  <br><small class="texto-suave">a instituição informa {{ r.totalDoBanco | currency }} reservados nesta conta;
                    aqui fica o que não está nas outras reservas. Conferir as outras ajusta este valor.</small>
                } @else if (r.dataConferencia) {
                  @if (r.conferenciaAntiga) {
                    <span class="etiqueta etiqueta-alerta">conferido há mais de 30 dias</span>
                  } @else {
                    <span class="etiqueta etiqueta-sucesso">conferido</span>
                  }
                  <br><small class="texto-suave">{{ r.saldoConferido | currency: (r.moeda ?? 'BRL') }} em {{ r.dataConferencia | date: 'dd/MM/yyyy' }}
                    + lançamentos depois</small>
                } @else {
                  <span class="etiqueta etiqueta-alerta">estimado</span>
                  <br><small class="texto-suave">só pelos lançamentos importados</small>
                  @if (r.podeTerSaldoAnterior) {
                    <br><small class="negativo">Pode haver saldo de antes do 1º lançamento importado. Confira.</small>
                  }
                }
                @if (r.foraDaListaDoBanco) {
                  <br><small class="texto-suave">não está na lista de reservas que a instituição manda: se ela já aparece
                    com outro nome, confira esta com zero.</small>
                }
              </td>
              <td class="direita nowrap">
                @if (podeConferir(r)) {
                  <button type="button" class="link" [class.destaque-conferir]="r.podeTerSaldoAnterior || r.conferenciaAntiga"
                    (click)="conferir.emit(r)">conferir saldo</button>
                }
                <!-- #691: gasto que saiu direto do cofrinho, sem passar pela conta (não chega pelo Open Finance). -->
                @if (podeRegistrarSaida(r)) {
                  <br><button type="button" class="link" (click)="registrarSaida.emit(r)"
                    [attr.aria-label]="'Registrar saída do cofrinho ' + r.nome">registrar saída</button>
                }
              </td>
            </tr>
          }
        </tbody>
      </table>
    </div>
    <p class="texto-suave nota">
      Quando a instituição informa, pelo Open Finance, o valor de cada reserva pelo nome, vale esse valor ("informado pelo
      banco"). Quando ela informa só o total reservado da conta, cada reserva continua estimada e conferível, e o que o
      total passa da soma delas aparece como "Reservado". Senão,
      ela só manda os lançamentos "Dinheiro reservado" e "Dinheiro retirado" no extrato da conta, e o saldo é estimado. Sem conferência, ele fica abaixo do real
      quando a reserva já tinha dinheiro antes do primeiro lançamento importado e quando o rendimento ainda não foi
      retirado. Confira o valor no app da instituição de vez em quando: os lançamentos seguintes passam a somar a
      partir dele. O total entra no patrimônio, no total investido e na reserva de emergência.
      O Méli Dólar do Mercado Pago também aparece aqui: a quantidade em dólar vem das compras e vendas do extrato
      (pelo "MUSD" da descrição ou pela PTAX do dia) e o saldo em reais é essa quantidade pela última PTAX.
    </p>
  `
})
export class InvestimentosReservasComponent {
  readonly reservas = input.required<Reserva[]>();
  readonly conferir = output<Reserva>();
  readonly registrarSaida = output<Reserva>();

  /** #237: "conferir saldo" em toda reserva cujo valor não veio, pelo nome, do banco (reserva-conferencia.ts). */
  podeConferir(r: Reserva): boolean {
    return podeConferirReserva(r);
  }

  /** #691: "registrar saída" em toda reserva em reais, menos a linha do restante do banco (reserva-conferencia.ts). */
  podeRegistrarSaida(r: Reserva): boolean {
    return podeRegistrarSaida(r);
  }
}
