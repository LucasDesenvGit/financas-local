import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PreviaHoleritePdf, TipoLinhaHolerite } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { MesAno, SeletorMesComponent } from '../../compartilhado/ui/seletor-mes.component';
import {
  LinhaPrevia, TIPOS_LINHA_DESCONTO, TIPOS_LINHA_PROVENTO, faltaParaConfirmar, precisaEscolher, provasAtuais, situacaoDaLinha,
  textoDaProva
} from './holerite-pdf';

/**
 * #228 (ADR 0068): a prévia do holerite lido do PDF (apresentação, ADR 0059). Cada linha com o tipo, o valor e o
 * trecho do PDF de onde saiu, para conferir e corrigir; a situação em texto (Nielsen 1); a prova real, que diz qual
 * total não bateu e por quanto (WCAG 3.3.1); o lido × calculado. Nada é gravado até "Confirmar" (Nielsen 3).
 */
@Component({
  selector: 'app-holerite-pdf-previa',
  imports: [FormsModule, MoedaPipe, CampoValorComponent, ListaSuspensaDirective, SeletorMesComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="painel-bloco previa-pdf" aria-labelledby="titulo-previa-pdf">
      <h2 id="titulo-previa-pdf">Prévia do holerite em PDF</h2>
      <p class="texto-suave">{{ arquivo() }}@if (restantes()) { <span> · mais {{ restantes() }} na fila</span> }
        @if (previa().empregador) { · {{ previa().empregador }} }</p>
      <p>Nada foi gravado. Confira cada linha com o PDF, corrija o que precisar e clique em “Confirmar”.</p>
      @if (previa().avisos.length) {
        <ul class="avisos-previa">
          @for (a of previa().avisos; track $index) { <li class="aviso-inline">{{ a }}</li> }
        </ul>
      }
      <div class="mes-previa">
        <span id="rotulo-mes-pdf">Mês do holerite</span>
        <app-seletor-mes aria-labelledby="rotulo-mes-pdf" [valor]="mesSeletor()" (mudou)="mesMudou.emit($event)" />
        @if (!mes()) { <span class="aviso-inline">Escolha o mês: o PDF não diz.</span> }
      </div>

      <div class="tabela-rolagem">
        <table class="tabela previa">
          <caption class="so-leitor">Linhas lidas do PDF</caption>
          <thead><tr><th scope="col">Linha do PDF</th><th scope="col">Tipo</th><th scope="col" class="direita">Valor</th>
            <th scope="col">Situação</th></tr></thead>
          <tbody>
            @for (l of linhas(); track $index; let i = $index) {
              <tr [class.ignorada]="l.ignorar" [class.escolher]="escolher(l)">
                <td>@if (l.rubrica.codigo) { <span class="texto-suave">{{ l.rubrica.codigo }}</span> }
                  {{ l.rubrica.descricao }}@if (l.rubrica.referencia) { <span class="texto-suave"> · {{ l.rubrica.referencia }}</span> }
                  <span class="trecho texto-suave">No PDF: “{{ l.rubrica.trecho }}”</span></td>
                <td>
                  <select [attr.aria-label]="'Tipo de ' + l.rubrica.descricao" [ngModel]="l.tipo" [ngModelOptions]="{ standalone: true }"
                    [disabled]="l.ignorar" (ngModelChange)="mudar(i, { tipo: +$event })">
                    <optgroup label="Proventos">@for (t of tiposProvento; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }</optgroup>
                    <optgroup label="Descontos">@for (t of tiposDesconto; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }</optgroup>
                  </select>
                </td>
                <td class="direita">
                  <app-campo-valor compacto [rotulo]="'Valor de ' + l.rubrica.descricao" [ngModel]="l.valor" [ngModelOptions]="{ standalone: true }"
                    [minimo]="0.01" obrigatorio [disabled]="l.ignorar" (ngModelChange)="mudar(i, { valor: $event })" />
                </td>
                <td>
                  <span class="situacao">{{ situacao(l) }}</span>
                  <label class="caixa"><input type="checkbox" [ngModel]="l.ignorar" [ngModelOptions]="{ standalone: true }"
                    (ngModelChange)="mudar(i, { ignorar: $event })"> Deixar de fora<span class="so-leitor">: {{ l.rubrica.descricao }}</span></label>
                </td>
              </tr>
            }
          </tbody>
        </table>
      </div>

      <h3>Prova real</h3>
      @if (provas().length) {
        <ul class="provas" aria-live="polite">
          @for (p of provas(); track p.descricao) {
            <li [class.nao-bateu]="!p.bateu"><strong>{{ p.descricao }}:</strong> {{ p.encontrado | currency }} somado,
              {{ p.impresso | currency }} no PDF: @if (!p.bateu) { {{ abs(p.diferenca) | currency }} }{{ prova(p) }}.</li>
          }
        </ul>
      } @else {
        <p class="texto-suave">O PDF não trouxe os totais: confira as linhas com ele.</p>
      }

      @if (previa().comparacoes.length) {
        <div class="tabela-rolagem">
          <table class="tabela comparacao">
            <caption>Lido × calculado pelas tabelas do mês</caption>
            <thead><tr><th scope="col"></th><th scope="col" class="direita">Lido do PDF</th><th scope="col" class="direita">Calculado</th></tr></thead>
            <tbody>
              @for (c of previa().comparacoes; track c.descricao) {
                <tr><th scope="row">{{ c.descricao }}</th><td class="direita">{{ c.lido | currency }}</td><td class="direita">{{ c.calculado | currency }}</td></tr>
              }
            </tbody>
          </table>
        </div>
        <p class="texto-suave">O calculado é a estimativa do site para o mês (o salário cadastrado, os lançamentos do mês e as tabelas). O que vale é o lido.</p>
      }
      @if (previa().salarioBase !== null || previa().baseInss !== null || previa().fgtsDoMes !== null) {
        <p class="texto-suave">Do rodapé do PDF:
          @if (previa().salarioBase !== null) { salário base {{ previa().salarioBase | currency }}. }
          @if (previa().baseInss !== null) { Base do INSS {{ previa().baseInss | currency }}. }
          @if (previa().fgtsDoMes !== null) { FGTS do mês {{ previa().fgtsDoMes | currency }}. }
        </p>
      }

      @if (falta(); as f) { <p class="aviso-inline" role="status">{{ f }}</p> }
      <div class="rodape-form">
        <button type="button" class="botao" [disabled]="ocupado()" (click)="cancelar.emit()">{{ restantes() ? 'Pular este' : 'Cancelar' }}</button>
        <span class="cresce"></span>
        <button type="button" class="botao botao-primario" [disabled]="ocupado() || !!falta()" (click)="confirmar.emit()">Confirmar e gravar</button>
      </div>
    </section>
  `,
  styles: [`
    .avisos-previa { list-style: none; padding: 0; margin: 0 0 12px; display: grid; gap: 6px; }
    .mes-previa { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px; }
    .previa td { vertical-align: top; }
    .previa .trecho { display: block; font-size: 0.8rem; overflow-wrap: anywhere; }
    .previa select { max-width: 13em; }
    .previa .situacao { display: block; }
    .previa tr.escolher .situacao { font-weight: 600; }
    .previa tr.ignorada td:first-child { text-decoration: line-through; }
    .previa .caixa { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; min-height: 24px; font-size: 0.85rem; }
    .provas { margin: 0 0 12px; padding-left: 1.2em; }
    .provas .nao-bateu { font-weight: 600; }
    .comparacao { max-width: 32em; }
    .comparacao caption { text-align: left; font-weight: 600; padding-bottom: 4px; }
    @media (max-width: 600px) {
      .previa thead { display: none; }
      .previa tr { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; padding: 8px 0; border-bottom: 1px solid var(--linha); }
      .previa td { border: 0; padding: 0; }
      .previa td:first-child, .previa td:last-child { grid-column: 1 / -1; }
      .previa select { max-width: 100%; width: 100%; }
    }
  `]
})
export class HoleritePdfPreviaComponent {
  readonly previa = input.required<PreviaHoleritePdf>();
  readonly linhas = input.required<LinhaPrevia[]>();
  /** "AAAA-MM-01", ou null enquanto a pessoa não escolheu (o PDF não disse). */
  readonly mes = input<string | null>(null);
  readonly arquivo = input('');
  readonly restantes = input(0);
  readonly ocupado = input(false);
  readonly linhasMudaram = output<LinhaPrevia[]>();
  readonly mesMudou = output<MesAno>();
  readonly confirmar = output<void>();
  readonly cancelar = output<void>();

  readonly tiposProvento = TIPOS_LINHA_PROVENTO;
  readonly tiposDesconto = TIPOS_LINHA_DESCONTO;
  readonly situacao = situacaoDaLinha;
  readonly escolher = precisaEscolher;
  readonly Tipo = TipoLinhaHolerite;

  readonly provas = computed(() => provasAtuais(this.previa(), this.linhas()));
  readonly falta = computed(() => faltaParaConfirmar(this.linhas(), this.mes()));
  readonly mesSeletor = computed<MesAno>(() => {
    const m = this.mes();
    if (!m) { const hoje = new Date(); return { ano: hoje.getFullYear(), mes: hoje.getMonth() + 1 }; }
    const [ano, mes] = m.split('-').map(Number);
    return { ano, mes };
  });

  readonly prova = textoDaProva;
  readonly abs = Math.abs;

  mudar(i: number, mudanca: Partial<LinhaPrevia>): void {
    this.linhasMudaram.emit(this.linhas().map((l, j) => (j === i ? { ...l, ...mudanca } : l)));
  }
}
