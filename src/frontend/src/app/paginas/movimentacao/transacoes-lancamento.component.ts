import { Component, ChangeDetectionStrategy, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Conta, OpcaoCategoria, TipoTransacao } from '../../core/modelos';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { CategoriaRotulada, FormTransacao } from './transacoes';

/**
 * Janela "Novo lançamento" / "Editar transação" da tela de Transações. Componente de apresentação (#475, ADR 0059):
 * recebe o lançamento e as listas, edita uma cópia e avisa em `salvar` com ela; não chama a API. Fechar sem salvar
 * não muda nada (Nielsen 3). Transação do Open Finance só ajusta o que não vem do banco (valor, data e conta ficam
 * de fora, com o porquê em texto; Nielsen 1).
 */
@Component({
  selector: 'app-transacoes-lancamento',
  imports: [ListaSuspensaDirective, FormsModule, ModalComponent, CampoDataComponent, CampoValorComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="lancamento() !== null" [titulo]="lancamento()?.id ? 'Editar transação' : 'Novo lançamento'"
      (fechar)="fechar.emit()">
      @if (dados(); as fm) {
        <form (ngSubmit)="salvar.emit(fm)" #f="ngForm" class="formulario">
          @if (fm.openFinance) {
            <p class="aviso-inline expandir" animate.leave="recolher">Transação importada do Open Finance: valor, data e conta vêm do banco.
              @if (fm.descricaoOriginal) { <br><small>Original: {{ fm.descricaoOriginal }}</small> }
            </p>
          } @else {
            <div class="grade-campos">
              <label>Tipo
                <select name="tipo" [(ngModel)]="fm.tipo">
                  <option [ngValue]="TipoTransacao.Despesa">Despesa</option>
                  <option [ngValue]="TipoTransacao.Receita">Receita</option>
                </select>
              </label>
              <label>Conta
                <select name="conta" [(ngModel)]="fm.contaId" required>
                  @for (c of contasManuais(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
                </select>
              </label>
              <label>Data <app-campo-data name="data" [(ngModel)]="fm.data" obrigatorio /></label>
              <label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>
            </div>
            @if (!contasManuais().length) {
              <p class="erro">Crie uma conta manual (ex.: "Carteira") em Contas e cartões para lançar manualmente.</p>
            }
          }
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="300"></label>
          <label>Categoria
            <select name="categoria" [(ngModel)]="fm.categoriaId">
              <option [ngValue]="null">Automática (regras)</option>
              @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ c.rotulo }}</option> }
            </select>
          </label>
          @if (sugestoes().length) {
            <div class="sugestoes-chips expandir" animate.leave="recolher">
              <span class="texto-suave">Sugestões:</span>
              @for (o of sugestoes(); track o.categoriaId) {
                <button type="button" class="chip-sugestao" [title]="o.motivo" (click)="fm.categoriaId = o.categoriaId">
                  <span class="ponto" [style.background]="o.cor"></span>{{ o.nome }}
                </button>
              }
            </div>
          }
          <label>Observação <textarea name="observacao" [(ngModel)]="fm.observacao" rows="2" maxlength="1000"></textarea></label>
          <label class="checkbox"><input type="checkbox" name="ignorar" [(ngModel)]="fm.ignorarRelatorios"> Ignorar nos relatórios e totais</label>
          @if (fm.tipo === TipoTransacao.Despesa) {
            <label class="checkbox"><input type="checkbox" name="emergencia" [(ngModel)]="fm.emergencia">
              <span class="texto-emergencia">Gasto de emergência</span>
              <small class="texto-suave">(destacado em laranja nos orçamentos e relatórios)</small></label>
          }
          @if (!fm.openFinance) {
            <label class="checkbox"><input type="checkbox" name="pendente" [(ngModel)]="fm.pendente"> Pendente (ainda não efetivada)</label>
          }
          <div class="rodape-form">
            @if (fm.id && !fm.openFinance) {
              <button type="button" class="botao botao-perigo" (click)="excluir.emit()">Excluir</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class TransacoesLancamentoComponent {
  /** O lançamento da janela (null = fechada). */
  readonly lancamento = input<FormTransacao | null>(null);
  readonly contasManuais = input<Conta[]>([]);
  readonly categorias = input<CategoriaRotulada[]>([]);
  /** Categorias sugeridas para esta transação; um clique escolhe. */
  readonly sugestoes = input<OpcaoCategoria[]>([]);
  readonly salvando = input(false);

  /** "Salvar", com a cópia editada. */
  readonly salvar = output<FormTransacao>();
  readonly excluir = output<void>();
  readonly fechar = output<void>();

  readonly TipoTransacao = TipoTransacao;
  /** A cópia que a janela edita: refeita a cada lançamento novo que chega. */
  readonly dados = linkedSignal(() => {
    const l = this.lancamento();
    return l ? { ...l } : null;
  });
}
