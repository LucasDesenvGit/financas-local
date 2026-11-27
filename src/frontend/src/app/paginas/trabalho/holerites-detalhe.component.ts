import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { Holerite, LancamentoHolerite, OrigemHolerite, SituacaoHolerite, TipoLancamentoHolerite } from '../../core/modelos';
import { TIPOS_DESCONTO, TIPOS_PROVENTO, ehPorHora, textoSituacao } from './holerites';
import { RecebimentoHoleriteComponent } from './recebimento-holerite.component';
import { nomeDoMes } from './salario';

/**
 * O holerite aberto de Holerites (#777, ADR 0059): dias, lançamentos, o cálculo linha a linha e as ações. Componente de
 * apresentação: a tela passa o holerite e os lançamentos (a lista é a mesma, o campo edita o item) e recebe o que a
 * pessoa pediu; não chama a API. O `section` segue como filho direto da grade da página (`display: contents`).
 */
@Component({
  selector: 'app-holerites-detalhe',
  imports: [FormsModule, DatePipe, MoedaPipe, CampoValorComponent, ListaSuspensaDirective, RecebimentoHoleriteComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    :host { display: contents; }
    .dias-dsr { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; }
    .dias-dsr input { width: 7em; }
    .lancamentos { display: grid; gap: 10px; }
    .lancamento { display: flex; flex-wrap: wrap; gap: 10px; align-items: flex-end; border: 1px solid var(--linha); border-radius: 10px; padding: 10px; margin: 0; }
    .lancamento input[type=number] { width: 7em; }
    .marcas { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
    .marcas .caixa { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; min-height: 24px; }
    .holerite tfoot th, .holerite tfoot td { font-weight: 600; }
    .holerite .detalhe { display: block; font-size: 0.8rem; }
    .holerite td.direita { white-space: nowrap; }
    @media (max-width: 600px) { .holerite th, .holerite td { padding: 8px 6px; } }
    .holerite .liquido td, .holerite .liquido th { border-top: 1px solid var(--linha); }
  `,
  template: `
    @let h = holerite();
    <section class="painel-bloco" aria-labelledby="titulo-holerite">
      <div class="cabecalho-bloco">
        <h2 id="titulo-holerite">{{ nomeDoMes(h.mes) }} <span class="etiqueta" [class.etiqueta-suave]="!fechado()">{{ textoSituacao(h.situacao) }}</span>
          @if (lido()) { <span class="etiqueta etiqueta-sucesso">Lido do PDF</span> }</h2>
      </div>
      @if (h.aviso) { <p class="aviso-inline">{{ h.aviso }}</p> }

      @if (!fechado()) {
        <form (ngSubmit)="salvar.emit()" #fh="ngForm" class="formulario">
          <div class="dias-dsr">
            <label>Dias úteis <input name="uteis" type="number" inputmode="numeric" min="1" max="31" [(ngModel)]="diasUteis" required></label>
            <label>Domingos e feriados <input name="descanso" type="number" inputmode="numeric" min="0" max="31" [(ngModel)]="diasDescanso" required></label>
            <p class="texto-suave">Para o DSR das horas extras. Acerte os feriados do mês.</p>
          </div>
          <div class="lancamentos">
            @for (l of lancamentos(); track $index; let i = $index) {
              <fieldset class="lancamento">
                <legend class="so-leitor">Lançamento {{ i + 1 }}</legend>
                <label>Tipo
                  <select [name]="'tipo' + i" [ngModel]="l.tipo" (ngModelChange)="mudarTipo.emit({ indice: i, tipo: $event })">
                    <optgroup label="Proventos">@for (t of tiposProvento; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }</optgroup>
                    <optgroup label="Descontos">@for (t of tiposDesconto; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }</optgroup>
                  </select>
                </label>
                <label>Descrição (opcional) <input [name]="'descricao' + i" [(ngModel)]="l.descricao" maxlength="100"></label>
                @if (ehPorHora(l.tipo)) {
                  <label>Horas <input [name]="'horas' + i" type="number" inputmode="decimal" min="0.01" max="300" step="0.01" [(ngModel)]="l.quantidade" required></label>
                  <label>Percentual (%) <input [name]="'pct' + i" type="number" inputmode="decimal" min="0.01" max="300" step="0.01" [(ngModel)]="l.percentual" required></label>
                } @else {
                  <label>{{ l.tipo === Tipo.ValeTransporte ? 'Custo do mês (opcional)' : 'Valor' }}
                    <app-campo-valor [name]="'valor' + i" [(ngModel)]="l.valor" [obrigatorio]="l.tipo !== Tipo.ValeTransporte" [minimo]="0.01" /></label>
                }
                <div class="marcas">
                  @if (l.tipo === Tipo.OutroProvento) {
                    <label class="caixa"><input type="checkbox" [name]="'inss' + i" [(ngModel)]="l.incideInss"> Tem INSS</label>
                    <label class="caixa"><input type="checkbox" [name]="'irrf' + i" [(ngModel)]="l.incideIrrf"> Tem IRRF</label>
                  }
                  @if (l.tipo === Tipo.OutroDesconto) {
                    <label class="caixa"><input type="checkbox" [name]="'deduz' + i" [(ngModel)]="l.deduzIrrf"> Deduz do IR</label>
                  }
                  <label class="caixa"><input type="checkbox" [name]="'recorrente' + i" [(ngModel)]="l.recorrente"> Repetir nos próximos meses</label>
                  <button type="button" class="botao botao-pequeno" (click)="remover.emit(i)">Remover<span class="so-leitor"> o lançamento {{ i + 1 }}</span></button>
                </div>
              </fieldset>
            }
          </div>
          <div class="rodape-form">
            <button type="button" class="botao" (click)="acrescentar.emit()">+ Lançamento</button>
            <span class="cresce"></span>
            <button class="botao botao-primario" [disabled]="fh.invalid || ocupado()">Salvar e calcular</button>
          </div>
        </form>
      } @else if (lido()) {
        <p class="texto-suave">Lido do PDF em {{ h.fechadoEm | date: 'dd/MM/yyyy' }}: vale o que a empresa pagou. Reabrir volta para a estimativa
          do site@if (h.lancamentos.length) { , com os lançamentos que você tinha posto }.</p>
      } @else if (h.lancamentos.length) {
        <p class="texto-suave">Fechado em {{ h.fechadoEm | date: 'dd/MM/yyyy' }}. Para mudar, reabra.</p>
      }

      @if (h.calculo; as c) {
        <div class="tabela-rolagem">
          <table class="tabela holerite">
            <caption class="so-leitor">Holerite de {{ nomeDoMes(h.mes) }}</caption>
            <thead><tr><th scope="col">Descrição</th><th scope="col" class="direita">Proventos</th>
              <th scope="col" class="direita">Descontos</th></tr></thead>
            <tbody>
              @for (p of c.proventos; track $index) {
                <tr><td>{{ p.descricao }}@if (p.referencia) { <span class="detalhe texto-suave">{{ p.referencia }}</span> }</td>
                  <td class="direita">{{ p.valor | currency }}</td><td></td></tr>
              }
              @for (d of c.descontos; track $index) {
                <tr><td>{{ d.descricao }}@if (d.referencia) { <span class="detalhe texto-suave">{{ d.referencia }}</span> }</td>
                  <td></td><td class="direita">{{ d.valor | currency }}</td></tr>
              }
            </tbody>
            <tfoot>
              <tr><th scope="row">Totais</th><td class="direita">{{ c.totalProventos | currency }}</td>
                <td class="direita">{{ c.totalDescontos | currency }}</td></tr>
              <tr class="liquido"><th scope="row" colspan="2">{{ lido() ? 'Líquido' : 'Líquido estimado' }}</th><td class="direita">{{ c.liquido | currency }}</td></tr>
            </tfoot>
          </table>
        </div>
        <p class="texto-suave">Salário de {{ c.salarioContratual | currency }} ({{ c.diasTrabalhados }} de 30 dias), hora a {{ c.valorHora | currency }}.
          @if (!c.semEncargos) { Base do INSS: {{ c.baseInss | currency }}. FGTS do mês (a empresa deposita, não sai do salário): {{ c.fgtsDoMes | currency }}. }
        </p>
      }
      <div class="rodape-form">
        @if (fechado()) {
          <button type="button" class="botao" [disabled]="ocupado()" (click)="reabrir.emit()">Reabrir</button>
        } @else {
          <button type="button" class="botao botao-perigo" [disabled]="ocupado()" (click)="excluir.emit()">Excluir</button>
          <span class="cresce"></span>
          <button type="button" class="botao" [disabled]="ocupado() || !h.calculo || mudou()" (click)="fechar.emit()">Fechar o mês</button>
        }
      </div>
      @if (!fechado() && mudou()) { <p class="texto-suave">Salve antes de fechar.</p> }
      <app-recebimento-holerite [holeriteId]="h.id" [versao]="h.situacao + ':' + (h.calculo?.liquido ?? '')" />
    </section>
  `
})
export class HoleritesDetalheComponent {
  readonly holerite = input.required<Holerite>();
  readonly lancamentos = input.required<LancamentoHolerite[]>();
  /** Mexeu desde o que está salvo (a tela decide): "Fechar o mês" espera o "Salvar". */
  readonly mudou = input(false);
  readonly ocupado = input(false);
  readonly diasUteis = model(0);
  readonly diasDescanso = model(0);

  readonly salvar = output<void>();
  readonly fechar = output<void>();
  readonly reabrir = output<void>();
  readonly excluir = output<void>();
  readonly acrescentar = output<void>();
  readonly remover = output<number>();
  readonly mudarTipo = output<{ indice: number; tipo: TipoLancamentoHolerite }>();

  readonly Tipo = TipoLancamentoHolerite;
  readonly tiposProvento = TIPOS_PROVENTO;
  readonly tiposDesconto = TIPOS_DESCONTO;
  readonly ehPorHora = ehPorHora;
  readonly nomeDoMes = nomeDoMes;
  readonly textoSituacao = textoSituacao;
  readonly fechado = computed(() => this.holerite().situacao === SituacaoHolerite.Fechado);
  readonly lido = computed(() => this.holerite().origem === OrigemHolerite.LidoDoPdf);
}
