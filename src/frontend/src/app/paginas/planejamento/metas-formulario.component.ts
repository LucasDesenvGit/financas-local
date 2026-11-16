import { DatePipe } from '@angular/common';
import { Component, ChangeDetectionStrategy, computed, input, linkedSignal, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CORES_SUGERIDAS, NOMES_CORES } from '../../compartilhado/utilidades';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { IconeComponent } from '../../compartilhado/ui/icone.component';
import { IconesMeta, iconeDaMeta } from '../../compartilhado/ui/icones';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { Conta, Reserva } from '../../core/modelos';
import {
  FormMeta, aoMudarOAlvo, dataAlvoPeloPorMes, faltaNoForm, mensagemLimitePorMes, mesDe, opcoesCofrinho, porMesAcimaDoLimite,
  previsaoPorMes
} from './metas';

/**
 * Janela "Nova meta" / "Editar meta" da tela de Metas (#732, ADR 0059). Componente de apresentação: recebe a meta, as
 * contas, os cofrinhos e o limite do valor por mês, edita uma cópia e avisa em `salvar` com ela; não chama a API nem lê
 * a rota. Fechar sem salvar não muda nada (Nielsen 3). O link para Investimentos › Reservas vem da tela, por projeção
 * de conteúdo (`link-reservas`): quem navega (e fecha a janela) é ela.
 */
@Component({
  selector: 'app-metas-formulario',
  imports: [ListaSuspensaDirective, FormsModule, DatePipe, MoedaPipe, ModalComponent, CampoValorComponent, CampoDataComponent, IconeComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="formulario() !== null" [titulo]="formulario()?.id ? 'Editar meta' : 'Nova meta'" (fechar)="fechar.emit()">
      @if (edicao(); as fm) {
        <form (ngSubmit)="salvar.emit(fm)" #f="ngForm" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fm.dados.nome" required maxlength="120"></label>
          <div class="grade-campos">
            <label>Valor alvo <app-campo-valor name="alvo" [ngModel]="fm.dados.valorAlvo" (ngModelChange)="mudarAlvo(fm, $event); recalcularPorMes(fm)" obrigatorio [minimo]="0.01" /></label>
            <label>Data alvo <app-campo-data name="data" [(ngModel)]="fm.dados.dataAlvo" /></label>
          </div>
          <label>Acompanhar saldo de uma conta (opcional)
            <select name="conta" [(ngModel)]="fm.dados.contaId" (ngModelChange)="fm.dados.reservaNome = null">
              <option [ngValue]="null">Não — vou registrar os aportes</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <label>Valor inicial já guardado
            <app-campo-valor name="atual" [ngModel]="fm.dados.valorAtual" (ngModelChange)="fm.dados.valorAtual = $event; recalcularPorMes(fm)" />
          </label>
          <!-- #359: em vez da data, dá para dizer quanto vai guardar por mês: o site calcula e preenche a data alvo. -->
          <label>Quanto vou guardar por mês (opcional)
            <!-- #359: não deixa passar do que sobra para as metas: o erro aparece ao digitar e "Salvar" fica desabilitado
                 (WCAG 3.3.1). -->
            <app-campo-valor name="porMes" [ngModel]="fm.porMes" (ngModelChange)="fm.porMes = $event; recalcularPorMes(fm)"
              [maximo]="limitePorMes()" [mensagemMaximo]="mensagemLimite()" />
          </label>
          @if (fm.porMes && !acimaDoLimite(fm)) {
            <p class="nota-discreta nota-por-mes" role="status">
              @if (previsao(fm); as p) {
                Com {{ fm.porMes | currency }} por mês, você chega lá em {{ p.data | date: 'MMM/yyyy' }}
                ({{ p.meses === 1 ? 'este mês' : p.meses + ' meses' }}). A data alvo foi preenchida com ela.
              } @else {
                O que já está guardado chega ao valor alvo: não falta nada.
              }
            </p>
          }
          @if (cofrinhos(fm).length) {
            <label>Qual cofrinho (reserva)?
              <select name="reserva" [(ngModel)]="fm.dados.reservaNome">
                <option [ngValue]="null">Saldo disponível da conta (sem cofrinhos)</option>
                @for (r of cofrinhos(fm); track r.nome) {
                  <option [ngValue]="r.nome">{{ r.nome }}{{ r.saldo === null ? ' (não encontrado no extrato)' : ' — ' + (r.saldo | currency) }}</option>
                }
              </select>
            </label>
            <p class="texto-suave">O saldo do cofrinho é estimado pelo extrato.
              <ng-content select="[link-reservas]" />.</p>
          }
          @if (fm.dados.contaId !== null) {
            <p class="texto-suave">O progresso será o valor inicial somado ao saldo {{ fm.dados.reservaNome ? 'do cofrinho' : 'atual da conta' }} escolhido.</p>
          }
          <!-- #316: ícone da meta. Botões com o nome no aria-label e o escolhido em aria-pressed (como as cores). -->
          <fieldset class="escolha-icone">
            <legend>Ícone</legend>
            <div class="icones-meta">
              @for (i of iconesMeta; track i.chave) {
                <button type="button" class="opcao-icone" [class.selecionada]="iconeDaMeta(fm.dados.icone).chave === i.chave"
                  [attr.aria-pressed]="iconeDaMeta(fm.dados.icone).chave === i.chave" [attr.aria-label]="'Ícone ' + i.nome"
                  [style.--cor]="fm.dados.cor" (click)="fm.dados.icone = i.chave">
                  <app-icone [icone]="i.icone" [tamanho]="20" />
                </button>
              }
            </div>
          </fieldset>
          <fieldset class="escolha-cor">
            <legend>Cor</legend>
            <div class="cores">
              @for (cor of cores; track cor) {
                <button type="button" class="cor" [style.background]="cor" [class.selecionada]="fm.dados.cor === cor"
                  [attr.aria-pressed]="fm.dados.cor === cor" (click)="fm.dados.cor = cor" [attr.aria-label]="'Cor ' + (nomesCores[cor] ?? cor)"></button>
              }
            </div>
          </fieldset>
          <label class="checkbox"><input type="checkbox" name="concluida" [(ngModel)]="fm.dados.concluida"> Concluída</label>
          @if (avisoReaberta()) {
            <p class="nota-discreta" role="status">A meta voltou para o plano: o alvo novo é maior que o que já foi guardado.</p>
          } @else if (fm.dados.concluida && falta(fm) > 0) {
            <p class="nota-discreta" role="status">Faltam {{ falta(fm) | currency }} para o alvo. Marcada como concluída, a meta
              sai do plano do mês; desmarque para ela voltar.</p>
          }
          <div class="rodape-form">
            @if (fm.id) { <button type="button" class="botao botao-perigo" (click)="excluir.emit()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  styles: `
    .escolha-icone, .escolha-cor { border: 0; padding: 0; margin: 0; min-width: 0; }
    .escolha-icone legend, .escolha-cor legend { padding: 0; margin-bottom: 6px; font-size: 0.86rem; color: var(--texto-suave); }
    .icones-meta { display: flex; flex-wrap: wrap; gap: 8px; }
    .opcao-icone {
      display: inline-grid; place-items: center; width: 40px; height: 40px; padding: 0; border-radius: 10px;
      border: 1px solid var(--linha); background: var(--superficie-2); color: var(--texto-suave); cursor: pointer;
    }
    .opcao-icone:hover { border-color: var(--cor); color: var(--texto); }
    .opcao-icone.selecionada {
      color: var(--cor); border-color: var(--cor); background: color-mix(in srgb, var(--cor) 18%, transparent);
      box-shadow: 0 0 0 2px var(--cor);
    }
  `
})
export class MetasFormularioComponent {
  /** A meta da janela (null = fechada). */
  readonly formulario = input<FormMeta | null>(null);
  readonly contas = input<Conta[]>([]);
  /** Os cofrinhos do extrato, para o campo "Qual cofrinho (reserva)?". */
  readonly reservas = input<Reserva[]>([]);
  /** O maior valor por mês que dá para escolher (null sem o plano do assistente). */
  readonly limitePorMes = input<number | null>(null);

  /** "Salvar", com a cópia editada. */
  readonly salvar = output<FormMeta>();
  readonly excluir = output<void>();
  readonly fechar = output<void>();

  readonly cores = CORES_SUGERIDAS;
  readonly nomesCores = NOMES_CORES;
  readonly iconesMeta = IconesMeta;
  readonly iconeDaMeta = iconeDaMeta;

  /** A cópia que a janela edita: refeita a cada meta nova que chega (fechar sem salvar não muda a da tela). */
  readonly edicao = linkedSignal(() => {
    const f = this.formulario();
    return f ? { ...f, dados: { ...f.dados } } : null;
  });
  /** #316: o alvo novo reabriu uma meta concluída (o aviso some quando a janela abre de novo). */
  readonly avisoReaberta = linkedSignal(() => {
    this.formulario();
    return false;
  });
  readonly mensagemLimite = computed(() => mensagemLimitePorMes(this.limitePorMes()));

  cofrinhos(fm: FormMeta) {
    return opcoesCofrinho(this.reservas(), fm.dados.contaId, fm.dados.reservaNome);
  }

  falta(fm: FormMeta): number {
    return faltaNoForm(fm.dados.valorAlvo, fm.dados.valorAtual, fm.deFora);
  }

  acimaDoLimite(fm: FormMeta): boolean {
    return porMesAcimaDoLimite(this.limitePorMes(), fm.porMes);
  }

  /** #359: com o valor por mês do formulário, em quantos meses e em que mês a meta chega ao alvo. */
  previsao(fm: FormMeta) {
    return previsaoPorMes(fm, mesDe(new Date()));
  }

  /** #359: mudou o valor por mês, o alvo ou o já guardado: a data alvo acompanha (só com o valor por mês preenchido). */
  recalcularPorMes(fm: FormMeta): void {
    const data = dataAlvoPeloPorMes(fm, this.limitePorMes(), mesDe(new Date()));
    if (data) fm.dados.dataAlvo = data;
  }

  /** #316: aumentar o alvo de uma meta concluída para acima do que já foi guardado reabre a meta. */
  mudarAlvo(fm: FormMeta, valor: number): void {
    const { concluida, reaberta } = aoMudarOAlvo(fm, valor);
    fm.dados.valorAlvo = valor;
    fm.dados.concluida = concluida;
    if (reaberta) this.avisoReaberta.set(true);
  }
}
