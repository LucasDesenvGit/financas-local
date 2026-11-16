import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { TipoOperacao } from '../../core/modelos';
import { LinhaImportacao, totalSelecionadas } from './investimentos';

/**
 * A janela "Importar compras e vendas do extrato" de Investimentos (#731, ADR 0059). Componente de apresentação: recebe
 * as linhas que a tela montou (as sugestões do extrato), deixa a pessoa marcar e corrigir ticker, tipo, cotas e preço
 * numa cópia, e avisa a tela com as linhas ao clicar em "Importar"; a conferência das linhas e a chamada da API são da
 * tela. Fechar sem importar não muda nada (Nielsen 3).
 */
@Component({
  selector: 'app-investimentos-importacao',
  imports: [ListaSuspensaDirective, FormsModule, DatePipe, MoedaPipe, ModalComponent, CampoValorComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    tr.encerrada td { opacity: 0.6; }
    .tabela-importacao td { vertical-align: top; }
    .campo-curto { width: 96px; padding: 4px 8px; }
    .maiusculas { text-transform: uppercase; }
    :host ::ng-deep .modal:has(.tabela-importacao) { width: min(1100px, 96vw); }
  `,
  template: `
    <app-modal [aberto]="aberta()" titulo="Importar compras e vendas do extrato" (fechar)="fechar.emit()">
      <div class="formulario">
        <p class="texto-suave">
          Transações com código da B3 na descrição (ex.: "COMPRA MXRF11 10 x 9,85"). Quando o extrato não traz quantidade
          ou preço, eles são calculados pelo valor da transação e pela cotação de fechamento do dia. Confira antes de importar.
          FIIs da corretora conectada no Meu Pluggy entram sozinhos na sincronização.
        </p>
        @if (carregando()) {
          <p class="vazio">Procurando nas transações e buscando as cotações do dia…</p>
        } @else {
          <div class="tabela-rolagem">
            <table class="tabela tabela-importacao">
              <thead>
                <tr><th></th><th>Data</th><th>Transação</th><th>Ativo</th><th>Operação</th>
                  <th class="direita">Valor</th><th class="direita">Cotas</th><th class="direita">Preço</th><th>Como</th></tr>
              </thead>
              <tbody>
                @for (l of copia(); track l.transacaoId) {
                  <tr [class.encerrada]="l.jaImportada" [class.selecionada]="l.selecionada && !l.jaImportada">
                    <td class="col-check"><label class="celula-marcar"><input type="checkbox" [(ngModel)]="l.selecionada"
                      [disabled]="l.jaImportada" [attr.aria-label]="'Importar ' + l.descricao"></label></td>
                    <td>{{ l.data | date: 'dd/MM/yy' }}</td>
                    <td><small>{{ l.descricao }}</small><br><small class="texto-suave">{{ l.contaNome }}</small>
                      @if (l.jaImportada) { <br><small class="etiqueta etiqueta-sucesso">já importada</small> }
                      @else if (l.possivelDuplicata) { <br><small class="etiqueta etiqueta-alerta">já existe operação parecida</small> }</td>
                    <td><input class="campo-curto maiusculas" [(ngModel)]="l.ticker" maxlength="12" [disabled]="l.jaImportada"></td>
                    <td>
                      <select [(ngModel)]="l.tipo" [disabled]="l.jaImportada">
                        <option [ngValue]="TipoOperacao.Compra">compra</option>
                        <option [ngValue]="TipoOperacao.Venda">venda</option>
                      </select>
                    </td>
                    <td class="direita">{{ l.valor | currency }}</td>
                    <td class="direita"><input type="number" class="campo-curto" [(ngModel)]="l.quantidade" min="0" step="any"
                      [disabled]="l.jaImportada"></td>
                    <td class="direita"><app-campo-valor class="campo-curto" compacto [(ngModel)]="l.precoUnitario" [casas]="4"
                      [disabled]="l.jaImportada" [rotulo]="'Preço por cota de ' + (l.ticker || 'ativo')" />
                      @if (l.fechamentoNoDia !== null) { <br><small class="texto-suave">fech. {{ l.fechamentoNoDia | currency }}</small> }</td>
                    <td><small class="texto-suave">{{ l.origem ?? 'preencha' }}</small></td>
                  </tr>
                } @empty {
                  <tr><td colspan="9" class="vazio">Nenhuma transação com código de ativo encontrada no extrato.</td></tr>
                }
              </tbody>
            </table>
          </div>
        }
        <div class="rodape-form">
          <span class="texto-suave">{{ selecionadas() }} selecionada(s)</span>
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="fechar.emit()">Fechar</button>
          <button type="button" class="botao botao-primario" [disabled]="!selecionadas() || salvando()"
            (click)="importar.emit(copia())">Importar</button>
        </div>
      </div>
    </app-modal>
  `
})
export class InvestimentosImportacaoComponent {
  readonly aberta = input(false);
  /** Procurando as sugestões no extrato. */
  readonly carregando = input(false);
  /** As linhas que a tela montou; a janela edita uma cópia, refeita a cada lista nova que chega. */
  readonly linhas = input<LinhaImportacao[]>([]);
  readonly salvando = input(false);
  /** Todas as linhas, com o que a pessoa marcou e corrigiu: a tela confere e importa as marcadas. */
  readonly importar = output<LinhaImportacao[]>();
  readonly fechar = output<void>();

  readonly copia = linkedSignal(() => this.linhas().map((l) => ({ ...l })));
  readonly TipoOperacao = TipoOperacao;

  selecionadas(): number {
    return totalSelecionadas(this.copia());
  }
}
