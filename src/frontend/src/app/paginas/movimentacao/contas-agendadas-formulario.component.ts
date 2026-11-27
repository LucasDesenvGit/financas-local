import { Component, ChangeDetectionStrategy, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { Categoria, Conta, FormaCobranca, Recorrencia, Reserva, SalvarContaAgendada } from '../../core/modelos';
import {
  FORMAS_DE_COBRANCA, dataLimitePelosRestantes, limiteInvalido, nomeForma, restantesDoForm
} from './contas-agendadas';
import { OpcaoCofrinho, formAceitaCofrinho, opcoesCofrinho, textoOpcaoCofrinho } from './pago-com-cofrinho';

/**
 * Janela "Nova conta a pagar" / "Editar conta" de Contas a pagar (#777, ADR 0059). Componente de apresentação: recebe o
 * formulário, as contas, as categorias e os cofrinhos, edita uma cópia e avisa em `salvar` com ela; não chama a API
 * nem lê a rota. Fechar sem salvar não muda nada (Nielsen 3).
 */
@Component({
  selector: 'app-contas-agendadas-formulario',
  imports: [ListaSuspensaDirective, FormsModule, ModalComponent, CampoValorComponent, CampoDataComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="formulario() !== null" [titulo]="editando() ? 'Editar conta' : 'Nova conta a pagar'" (fechar)="fechar.emit()">
      @if (edicao(); as fm) {
        <form (ngSubmit)="salvar.emit(fm)" #f="ngForm" class="formulario">
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="150"></label>
          <div class="grade-campos">
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>
            <label>Próximo vencimento <app-campo-data name="venc" [(ngModel)]="fm.proximoVencimento" obrigatorio /></label>
            <label>Recorrência
              <select name="rec" [(ngModel)]="fm.recorrencia">
                <option [ngValue]="Recorrencia.Mensal">Mensal</option>
                <option [ngValue]="Recorrencia.Anual">Anual</option>
                <option [ngValue]="Recorrencia.Unica">Única</option>
              </select>
            </label>
            <label>Categoria
              <select name="cat" [(ngModel)]="fm.categoriaId">
                <option [ngValue]="null">—</option>
                @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
          </div>
          @if (fm.recorrencia !== Recorrencia.Unica) {
            <div class="grade-campos">
              <label>Até (opcional)
                <app-campo-data name="limite" [(ngModel)]="fm.dataLimite" [minimo]="fm.proximoVencimento"
                  ajuda="ajuda-limite" [invalido]="limiteInvalido(fm)" /></label>
              <label>ou quantos pagamentos faltam
                <input type="number" name="restantes" min="1" step="1" [ngModel]="restantesDoForm(fm)"
                  (ngModelChange)="fm.dataLimite = dataLimitePelosRestantes(fm, $event)" aria-describedby="ajuda-limite"></label>
            </div>
            @if (limiteInvalido(fm)) {
              <p class="negativo pequeno-texto" role="alert">A data limite não pode ser antes do próximo vencimento.</p>
            }
            <p class="texto-suave pequeno-texto" id="ajuda-limite">
              Depois do último pagamento a conta é encerrada sozinha e sai do Painel. Em branco, ela se repete sem fim.</p>
          }
          <label>Como é cobrada
            <select name="forma" [(ngModel)]="fm.formaCobranca" aria-describedby="ajuda-forma">
              @for (f of formas; track f) { <option [ngValue]="f">{{ nomeForma(f) }}</option> }
            </select>
          </label>
          <label>{{ fm.formaCobranca === FormaCobranca.CartaoCredito ? 'Cartão onde cai' : 'Conta de pagamento' }} (opcional)
            <select name="conta" [(ngModel)]="fm.contaId" (ngModelChange)="fm.reservaNome = null; outroCofrinho.set(false)">
              <option [ngValue]="null">—</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <!-- #691 (ADR 0063): de onde sai o dinheiro. Só em conta do Open Finance, fora do cartão (divulgação
               progressiva); os cofrinhos da conta numa lista, para escolher em vez de digitar (Nielsen 6). -->
          @if (aceitaCofrinho(fm)) {
            <label>Pago com o cofrinho
              <select name="pagoCom" [ngModel]="outroCofrinho() ? OUTRO : fm.reservaNome" (ngModelChange)="escolherCofrinho(fm, $event)"
                aria-describedby="ajuda-cofrinho">
                <option [ngValue]="null">Nenhum (sai do saldo da conta)</option>
                @for (r of opcoesCofrinho(fm); track r.nome) {
                  <option [ngValue]="r.nome">{{ textoOpcaoCofrinho(r) }}</option>
                }
                <option [ngValue]="OUTRO">Outro…</option>
              </select>
            </label>
            @if (outroCofrinho()) {
              <label>Nome do cofrinho
                <input name="reservaNome" [(ngModel)]="fm.reservaNome" maxlength="100" required placeholder="ex.: Emergência"
                  aria-describedby="ajuda-cofrinho"></label>
            }
            <p class="texto-suave pequeno-texto" id="ajuda-cofrinho">
              @if (!fm.reservaNome) {
                O dinheiro sai da conta, e o pagamento chega pela sincronização.
              } @else if (fm.formaCobranca === FormaCobranca.DebitoAutomatico) {
                Sai direto do cofrinho, sem passar pela conta: no vencimento, o app lança a despesa e baixa o cofrinho.
                Se a cobrança chegar pelo extrato mesmo assim, vale a do banco.
              } @else {
                Sai direto do cofrinho: o “Pagar” lança a despesa e baixa o cofrinho.
              }
            </p>
          }
          @if (fm.formaCobranca !== FormaCobranca.AMao) {
            <label>Texto da cobrança no extrato
              <input name="textoCobranca" [(ngModel)]="fm.textoCobranca" maxlength="100" placeholder="ex.: NETFLIX"
                aria-describedby="ajuda-forma"></label>
          }
          <p class="texto-suave pequeno-texto" id="ajuda-forma">
            @if (fm.formaCobranca === FormaCobranca.AMao) {
              Boleto ou Pix que você paga: use “Pagar” quando pagar.
            } @else {
              Cobrada sozinha: com o texto (a descrição do banco contém…), ela se dá como paga quando a cobrança chega
              no extrato. Sem o texto, use “Pagar”.
            }
          </p>
          <label>Observação (opcional)
            <input name="observacao" [(ngModel)]="fm.observacao" maxlength="300" placeholder="ex.: conta da família, cancelar em março"></label>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="fm.ativa"> Ativa</label>
          <div class="rodape-form">
            @if (editando()) { <button type="button" class="botao botao-perigo" (click)="excluir.emit()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || limiteInvalido(fm)">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class ContasAgendadasFormularioComponent {
  /** A conta da janela (null = fechada). */
  readonly formulario = input<SalvarContaAgendada | null>(null);
  /** É a edição de uma conta que já existe: muda o título e mostra "Excluir". */
  readonly editando = input(false);
  readonly contas = input<Conta[]>([]);
  /** As categorias de despesa que a lista oferece. */
  readonly categorias = input<Categoria[]>([]);
  /** #691: os cofrinhos das contas, para o "Pago com". */
  readonly reservas = input<Reserva[]>([]);

  /** "Salvar", com a cópia editada. */
  readonly salvar = output<SalvarContaAgendada>();
  readonly excluir = output<void>();
  readonly fechar = output<void>();

  readonly Recorrencia = Recorrencia;
  readonly FormaCobranca = FormaCobranca;
  readonly formas = FORMAS_DE_COBRANCA;
  readonly nomeForma = nomeForma;
  readonly limiteInvalido = limiteInvalido;
  readonly restantesDoForm = restantesDoForm;
  readonly dataLimitePelosRestantes = dataLimitePelosRestantes;
  readonly textoOpcaoCofrinho = textoOpcaoCofrinho;
  readonly OUTRO = '__outro__';

  /** A cópia que a janela edita: refeita a cada conta nova que chega (fechar sem salvar não muda a da tela). */
  readonly edicao = linkedSignal(() => {
    const f = this.formulario();
    return f ? { ...f } : null;
  });
  /** #691: "Pago com o cofrinho" → "Outro…": o nome é digitado. Volta a falso a cada conta nova. */
  readonly outroCofrinho = linkedSignal(() => {
    this.formulario();
    return false;
  });

  aceitaCofrinho(fm: SalvarContaAgendada): boolean {
    return formAceitaCofrinho(fm, this.contas());
  }

  opcoesCofrinho(fm: SalvarContaAgendada): OpcaoCofrinho[] {
    return opcoesCofrinho(this.reservas(), fm.contaId, this.outroCofrinho() ? null : fm.reservaNome);
  }

  escolherCofrinho(fm: SalvarContaAgendada, valor: string | null): void {
    this.outroCofrinho.set(valor === this.OUTRO);
    fm.reservaNome = this.outroCofrinho() ? '' : valor;
  }
}
