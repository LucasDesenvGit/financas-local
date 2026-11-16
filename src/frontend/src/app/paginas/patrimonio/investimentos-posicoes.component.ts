import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { PosicaoAtivo } from '../../core/modelos';
import { nomeTipo, percentual, quantidade } from './investimentos';

/**
 * A aba "Renda variável" de Investimentos: a tabela das posições (#731, ADR 0059). Componente de apresentação: recebe
 * as posições já filtradas e avisa os cliques (`operacoes`, `editar`, `fonteCotacoes`, `alternarEncerradas`); não chama
 * a API. O interruptor "Mostrar encerradas" tem efeito imediato (#130), e quem guarda a escolha é a tela.
 */
@Component({
  selector: 'app-investimentos-posicoes',
  imports: [MoedaPipe, DecimalPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    .cabecalho-bloco { display: flex; align-items: center; gap: 14px; margin-bottom: 10px; flex-wrap: wrap; }
    .cabecalho-bloco h2 { margin: 0; }
    .tabela-carteira td { white-space: nowrap; }
    .tabela-carteira td:first-child { white-space: normal; min-width: 160px; }
    tr.encerrada td { opacity: 0.6; }
    .nowrap { white-space: nowrap; }
    .nowrap .link + .link { margin-left: 10px; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
  `,
  template: `
    <section class="painel-bloco">
      <div class="cabecalho-bloco">
        <h2>Posições</h2>
        <span class="cresce"></span>
        <!-- #130: efeito imediato → interruptor. -->
        <label class="interruptor"><input type="checkbox" role="switch" [checked]="mostrarEncerradas()"
          (change)="alternarEncerradas.emit()"> Mostrar encerradas</label>
        <button type="button" class="link" (click)="fonteCotacoes.emit()">fonte das cotações</button>
      </div>
      <div class="tabela-rolagem">
        <table class="tabela tabela-carteira">
          <thead>
            <tr>
              <th>Ativo</th><th class="direita">Cotas</th><th class="direita"><app-termo chave="precoMedio">Preço médio</app-termo></th>
              <th class="direita">Cotação</th><th class="direita">Valor atual</th><th class="direita">Valorização</th>
              <th class="direita">Proventos 12m</th><th class="direita">Resultado total</th><th class="direita">x Selic</th><th></th>
            </tr>
          </thead>
          <tbody>
            @for (p of posicoes(); track p.id) {
              <tr [class.encerrada]="p.quantidade === 0">
                <td>
                  <strong>{{ p.ticker }}</strong> <span class="etiqueta etiqueta-suave">{{ nomeTipo(p.tipo) }}</span>
                  @if (p.moeda !== 'BRL') { <span class="etiqueta etiqueta-suave" title="Ativo no exterior">{{ p.moeda }}</span> }
                  @if (p.nome) { <br><small class="texto-suave">{{ p.nome }}</small> }
                </td>
                <td class="direita">{{ qtd(p.quantidade) }}</td>
                <td class="direita">{{ p.precoMedio | currency: p.moeda }}</td>
                <td class="direita">
                  @if (p.precoAtual !== null) {
                    {{ p.precoAtual | currency: p.moeda }}
                    @if (p.moeda !== 'BRL') {
                      <br><small class="texto-suave">{{ p.precoAtualEmReais !== null ? (p.precoAtualEmReais | currency) : 'sem câmbio' }}</small>
                    }
                    @if (p.precoManual) {
                      <br><small class="etiqueta etiqueta-alerta">manual</small>
                    } @else if (p.variacaoDia !== null) {
                      <br><small [class.positivo]="p.variacaoDia >= 0" [class.negativo]="p.variacaoDia < 0">{{ pct(p.variacaoDia) }} hoje</small>
                    }
                  } @else {
                    <span class="texto-suave" title="Sem cotação: usando o preço médio">—</span>
                  }
                </td>
                <td class="direita valor">{{ p.valorMercado | currency }}</td>
                <td class="direita" [class.positivo]="p.valorizacao >= 0" [class.negativo]="p.valorizacao < 0">
                  {{ p.valorizacao | currency }}<br><small>{{ pct(p.valorizacaoPercentual) }}</small>
                </td>
                <td class="direita">
                  {{ p.proventos12Meses | currency }}
                  @if (p.dividendYield12Meses !== null) {
                    <br><small class="texto-suave">DY {{ p.dividendYield12Meses | number: '1.1-1' }}%</small>
                  }
                </td>
                <td class="direita" [class.positivo]="p.resultadoTotal >= 0" [class.negativo]="p.resultadoTotal < 0">
                  {{ p.resultadoTotal | currency }}<br><small>{{ pct(p.resultadoTotalPercentual) }}</small>
                </td>
                <td class="direita" [class.positivo]="p.diferencaSelic >= 0" [class.negativo]="p.diferencaSelic < 0">
                  {{ p.diferencaSelic >= 0 ? '+' : '' }}{{ p.diferencaSelic | currency }}
                  <br><small class="texto-suave">Selic {{ pct(p.resultadoSelicPercentual) }}</small>
                </td>
                <td class="direita nowrap">
                  <button type="button" class="link" (click)="operacoes.emit(p)">operações</button>
                  <button type="button" class="link" (click)="editar.emit(p)">editar</button>
                </td>
              </tr>
            } @empty {
              <tr><td colspan="10" class="vazio">
                Nenhum ativo lançado. Use <strong>+ Lançar compra/venda</strong> com o código da B3 (ex.: MXRF11) e o preço pago,
                ou escolha outra moeda para um ativo no exterior (ex.: AAPL em USD).
              </td></tr>
            }
          </tbody>
        </table>
      </div>
      <p class="texto-suave nota">
        Preço médio pelo método da Receita (taxas entram no custo). Proventos são estimados pela quantidade que você tinha na
        data-com. Ativos no exterior: preço médio e cotação na moeda do ativo; custo, valores e resultados em reais, cada
        operação pelo seu câmbio e o valor atual pelo câmbio de hoje (proventos do exterior aparecem brutos, antes do imposto
        retido lá fora). "x Selic" compara o valor atual com quanto o mesmo dinheiro (com as mesmas vendas e proventos retirados)
        teria hoje na Selic.
      </p>
    </section>
  `
})
export class InvestimentosPosicoesComponent {
  /** As posições a mostrar (a tela já tirou as encerradas, se for o caso). */
  readonly posicoes = input.required<PosicaoAtivo[]>();
  readonly mostrarEncerradas = input(false);
  /** "Ocultar valores" ligado: a quantidade de cotas vira "•••". */
  readonly oculto = input(false);
  readonly alternarEncerradas = output<void>();
  readonly fonteCotacoes = output<void>();
  readonly operacoes = output<PosicaoAtivo>();
  readonly editar = output<PosicaoAtivo>();

  readonly pct = percentual;
  readonly nomeTipo = nomeTipo;

  qtd(valor: number): string {
    return quantidade(valor, this.oculto());
  }
}
