import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FaturaAbertaDoCartao } from './faturas-no-painel';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';
import { LogoBancoComponent } from './logo-banco.component';
import { MoedaPipe } from './moeda.pipe';

let sequencia = 0;

/**
 * #322: o card "Fatura em aberto" — o total da fatura que ainda recebe compras nos cartões escolhidos ("Fatura no
 * Painel", em Contas e cartões). A fatura de cada cartão fica atrás de "ver detalhes" (Progressive Disclosure; botão
 * com aria-expanded e aria-controls, padrão Disclosure do WAI-ARIA APG), no formato do card "Faturas do mês"
 * (Nielsen 4). O mesmo card fica no Painel ("Situação hoje") e no resumo de Contas e cartões. O host some do layout
 * (display: contents) para o card ser um item da grade de KPIs.
 */
@Component({
  selector: 'app-kpi-faturas-abertas',
  imports: [DatePipe, RouterLink, MoedaPipe, IconeComponent, LogoBancoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <article class="kpi kpi-fatura-aberta">
      <span class="kpi-rotulo"><app-icone [icone]="Icones.contas" [tamanho]="16" /> Fatura em aberto</span>
      <strong [class.negativo]="total() > 0">{{ total() | currency }}</strong>
      <small>{{ itens().length === 1 ? 'do cartão escolhido' : 'total dos ' + itens().length + ' cartões escolhidos' }} ·
        <button type="button" class="link botao-detalhes" [attr.aria-expanded]="aberto()" [attr.aria-controls]="idLista"
          (click)="aberto.set(!aberto())">{{ aberto() ? 'esconder detalhes' : 'ver detalhes' }}</button></small>
      <div [id]="idLista">
        @if (aberto()) {
          <ul class="lista-faturas-abertas">
            @for (i of itens(); track i.conta.id) {
              <li>
                <span class="cartao-fatura"><app-logo-banco [codigo]="i.conta.banco" [tamanho]="20" [cartao]="true" />
                  @if (links()) {
                    <a routerLink="/contas" [queryParams]="{ faturas: i.conta.id }" [attr.aria-label]="'Ver as faturas de ' + i.conta.nome">{{ i.conta.nome }}</a>
                  } @else {
                    {{ i.conta.nome }}
                  }</span>
                @if (i.fatura; as f) {
                  <span class="valor">{{ f.valorMes | currency }}</span>
                  @if (f.fechamento || f.vencimento) {
                    <span class="texto-suave detalhe-fatura">@if (f.fechamento) {fecha {{ f.fechamento | date: 'dd/MM' }}}@if (f.fechamento && f.vencimento) {, }@if (f.vencimento) {vence {{ f.vencimento | date: 'dd/MM' }}}</span>
                  }
                } @else {
                  <span class="valor">—</span>
                  <span class="texto-suave detalhe-fatura">a do mês já fechou e a próxima ainda não veio do banco</span>
                }
              </li>
            }
          </ul>
        }
      </div>
    </article>
  `,
  styles: [`
    :host { display: contents; }
    /* A coluna do card fica na largura dele (sem isso, a grade do .kpi cresce com a lista aberta). Classe própria na lista:
       a .lista-faturas do Painel é a de "Faturas do mês", em colunas de 260 px (styles.css). */
    .kpi-fatura-aberta { grid-template-columns: minmax(0, 1fr); align-content: start; }
    .botao-detalhes { padding: 0; min-height: 24px; font-size: inherit; }
    .lista-faturas-abertas { list-style: none; margin: 8px 0 2px; padding: 8px 0 0; display: grid; gap: 8px; font-size: 0.84rem;
      border-top: 1px solid var(--linha); }
    .lista-faturas-abertas li { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; }
    .lista-faturas-abertas li + li { border-top: 1px solid var(--linha); padding-top: 8px; }
    .lista-faturas-abertas .valor { margin-left: auto; font-variant-numeric: tabular-nums; }
    .lista-faturas-abertas .cartao-fatura { display: inline-flex; align-items: center; gap: 6px; }
    .lista-faturas-abertas .cartao-fatura a { display: inline-flex; align-items: center; min-height: 24px; }
    .lista-faturas-abertas .detalhe-fatura { flex-basis: 100%; font-size: 0.8rem; }
  `]
})
export class KpiFaturasAbertasComponent {
  readonly Icones = Icones;
  readonly itens = input.required<FaturaAbertaDoCartao[]>();
  /** No Painel, o nome do cartão abre o histórico de faturas em Contas; em Contas, o cartão já está na tela. */
  readonly links = input(true);
  readonly aberto = signal(false);
  readonly idLista = `faturas-abertas-${++sequencia}`;
  readonly total = computed(() => this.itens().reduce((s, i) => s + (i.fatura?.valorMes ?? 0), 0));
}
