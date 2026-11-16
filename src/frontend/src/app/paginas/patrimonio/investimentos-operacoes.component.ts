import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { OperacaoAtivo, OrigemOperacao, PosicaoAtivo, TipoOperacao } from '../../core/modelos';
import { nomeOrigem, percentual, quantidade } from './investimentos';

/**
 * A janela "Operações · ATIVO" de Investimentos: cada compra e venda do ativo, comparada com o preço atual (#731,
 * ADR 0059). Componente de apresentação: recebe o ativo e as operações e avisa "editar" e "nova operação"; quem busca as
 * operações na API e abre o formulário é a tela. Sem ativo, a janela fica fechada.
 */
@Component({
  selector: 'app-investimentos-operacoes',
  imports: [MoedaPipe, DatePipe, DecimalPipe, ModalComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    .resumo-ativo { display: flex; gap: 24px; flex-wrap: wrap; margin-bottom: 14px; }
    .resumo-ativo div { display: grid; gap: 2px; }
    .resumo-ativo span { font-size: 0.8rem; }
  `,
  template: `
    <app-modal [aberto]="ativo() !== null" [titulo]="'Operações · ' + (ativo()?.ticker ?? '')" (fechar)="fechar.emit()">
      @if (ativo(); as a) {
        <div class="resumo-ativo">
          <div><span class="texto-suave">Preço médio</span><strong>{{ a.precoMedio | currency: a.moeda }}</strong></div>
          <div><span class="texto-suave">Cotação atual</span>
            <strong>{{ a.precoAtual !== null ? (a.precoAtual | currency: a.moeda) : '—' }}</strong></div>
          <div><span class="texto-suave">Cotas</span><strong>{{ qtd(a.quantidade) }}</strong></div>
          @if (a.ultimoProvento; as up) {
            <div><span class="texto-suave">Último provento (data-com {{ up.dataCom | date: 'dd/MM/yy' }})</span>
              <strong>{{ up.valorPorCota | currency: a.moeda : 'symbol' : '1.2-4' }} por cota</strong></div>
          }
        </div>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead>
              <tr><th>Data</th><th>Operação</th><th class="direita">Cotas</th><th class="direita">Preço</th>
                <th class="direita">Total</th><th class="direita">Hoje x preço pago</th><th class="direita">Se fosse Selic</th>
                <th>Origem</th><th></th></tr>
            </thead>
            <tbody>
              @for (o of operacoes(); track o.id) {
                <tr>
                  <td>{{ o.data | date: 'dd/MM/yyyy' }}</td>
                  <td><span class="etiqueta" [class.etiqueta-sucesso]="o.tipo === TipoOperacao.Compra"
                    [class.etiqueta-alerta]="o.tipo === TipoOperacao.Venda">{{ o.tipo === TipoOperacao.Compra ? 'compra' : 'venda' }}</span></td>
                  <td class="direita">{{ qtd(o.quantidade) }}</td>
                  <td class="direita">{{ o.precoUnitario | currency: o.moeda : 'symbol' : '1.2-4' }}
                    @if (o.moeda !== 'BRL') { <br><small class="texto-suave">câmbio {{ o.cambio | number: '1.2-4' }}</small> }</td>
                  <td class="direita">{{ o.valorTotal | currency }}</td>
                  <td class="direita">
                    @if (o.variacaoPreco !== null) {
                      <span [class.positivo]="o.variacaoPreco >= 0" [class.negativo]="o.variacaoPreco < 0">{{ pct(o.variacaoPreco) }}</span>
                      <br><small class="texto-suave">vale {{ o.valorAtual | currency }}</small>
                    } @else {
                      <span class="texto-suave">—</span>
                    }
                  </td>
                  <td class="direita texto-suave">{{ o.tipo === TipoOperacao.Compra ? (o.valorSeFosseSelic | currency) : '—' }}</td>
                  <td><span class="etiqueta etiqueta-suave">{{ nomeOrigem(o.origem) }}</span></td>
                  <td class="direita"><button type="button" class="link" (click)="editar.emit(o)">editar</button></td>
                </tr>
              } @empty {
                <tr><td colspan="9" class="vazio">Carregando…</td></tr>
              }
            </tbody>
          </table>
        </div>
        <div class="rodape-form">
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="nova.emit(a.ticker)">+ Nova operação de {{ a.ticker }}</button>
        </div>
      }
    </app-modal>
  `
})
export class InvestimentosOperacoesComponent {
  /** O ativo da janela; null fecha. */
  readonly ativo = input<PosicaoAtivo | null>(null);
  readonly operacoes = input<OperacaoAtivo[]>([]);
  /** "Ocultar valores" ligado: a quantidade de cotas vira "•••". */
  readonly oculto = input(false);
  readonly editar = output<OperacaoAtivo>();
  /** "+ Nova operação de ATIVO", com o ticker. */
  readonly nova = output<string>();
  readonly fechar = output<void>();

  readonly TipoOperacao = TipoOperacao;
  readonly pct = percentual;
  readonly nomeOrigem: (o: OrigemOperacao) => string = nomeOrigem;

  qtd(valor: number): string {
    return quantidade(valor, this.oculto());
  }
}
