import { Component, ChangeDetectionStrategy, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { PlanoMetas } from '../../core/modelos';
import { nomeMes } from './metas';
import { PlanoCalculado } from './metas-plano';

/**
 * O bloco "Quanto guardar este mês" da tela de Metas (#732, ADR 0059). Componente de apresentação: recebe o caixa, o plano
 * já calculado e quanto da sobra vai para as metas, e avisa em `mudarPercentual`; não chama a API nem lê a rota. O elemento
 * do componente é o próprio bloco (classes `painel-bloco` e `assistente-metas` e `role="region"` com nome, ADR 0059,
 * "Na prática"). O link para a projeção vem da tela, por projeção de conteúdo (`link-projecao`): quem navega é ela.
 */
@Component({
  selector: 'app-metas-assistente',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  host: { class: 'painel-bloco assistente-metas', role: 'region', 'aria-labelledby': 'titulo-assistente' },
  template: `
    @let p = plano();
    @if (!p.disponivel) {
      <h2 id="titulo-assistente">Quanto guardar este mês</h2>
      <p>Preciso de pelo menos 3 meses completos de extrato para olhar seus gastos. Por enquanto, cada meta mostra a divisão igual até a data.</p>
    } @else if (calculado(); as c) {
      <!-- O valor à direita, em verde (sobra) ou vermelho (falta): a resposta do bloco se lê de longe (Nielsen 1). -->
      <h2 id="titulo-assistente" class="titulo-com-valor">
        <span>{{ nomeMes(p.mes) }}/{{ p.ano }} · <app-termo chave="sobraDoCaixa">sobra do mês</app-termo></span>
        <span class="valor-titulo" [class.positivo]="c.sobraMes >= 0" [class.negativo]="c.sobraMes < 0">
          {{ c.sobraMes | currency }}</span>
      </h2>
      @if (p.meses[0]; as m0) {
        <!-- #149: a conta aberta, para a sobra não parecer um número mágico (Nielsen 1 e 10). -->
        <p class="texto-suave conta-sobra">Entra {{ m0.receitas | currency }} · fatura do cartão {{ m0.faturas | currency }}
          · outras despesas {{ m0.despesas | currency }}</p>
      }
      <!-- #359: a recomendação é o aviso principal do bloco: numa caixa só, numa linha (quebra só quando não cabe), com o
           essencial em negrito (Nielsen 1 e 8). -->
      @if (c.sobraMes <= 0) {
        <p class="aviso-recomendacao aviso-vermelho">Pelo dinheiro que entra e sai, este mês <strong>fecha no vermelho</strong>:
          recomendo guardar <strong>R$ 0 agora</strong>. O plano compensa nos meses com mais folga.</p>
      } @else if (totalAtivas() === 0) {
        <!-- #316: sem meta em andamento não há para onde mandar a sobra: dizer isso, e não "guardar R$ 0,00". -->
        <p class="aviso-recomendacao"><strong>Nenhuma meta em andamento:</strong> crie uma meta, ou reabra uma concluída (em
          “Editar”, desmarque “Concluída”), para eu dividir a sobra.</p>
      } @else {
        <p class="aviso-recomendacao">Recomendo guardar <strong class="valor-recomendado">{{ c.totalRecomendado | currency }}</strong>
          este mês, dividido pela <strong>ordem de prioridade</strong> das metas.@switch (c.limitadoPor) {
            @case ('saldo') {
              <span class="limite-recomendacao">Limitei ao que deve sobrar nas contas no fim do mês, depois da fatura
                (<strong>{{ p.saldoFimDoMes | currency }}</strong>).</span>
            }
            @case ('historico') {
              <span class="limite-recomendacao">Limitei ao que de fato sobrou no seu melhor mês recente
                (<strong>{{ p.melhorMesRecente | currency }}</strong>).</span>
            }
            @case ('minimo') {
              <span class="limite-recomendacao">É pouco, mas é um começo: o plano compensa nos meses com mais folga.</span>
            }
          }</p>
      }
      <!-- #149 (C): quanto da sobra vai para as metas, na própria tela; muda na hora (Nielsen 3 e 6). -->
      <label class="parte-sobra">Guardar
        <select name="percentual" [ngModel]="percentual()" (ngModelChange)="mudarPercentual.emit($event)">
          @for (v of opcoesPercentual; track v) { <option [ngValue]="v">{{ v }}%</option> }
        </select>
        da sobra para as metas <app-termo chave="parteDaSobra"></app-termo>
      </label>
      <p class="links-assistente">
        <app-termo chave="recomendacaoMetas">Como calculei</app-termo> · <ng-content select="[link-projecao]" />
      </p>
    }
  `,
  styles: `
    :host { display: block; }
    :host(.assistente-metas) h2 { margin: 0 0 8px; }
    .titulo-com-valor { display: flex; justify-content: space-between; align-items: baseline; gap: 6px 16px; flex-wrap: wrap; }
    .valor-titulo { font-family: var(--fonte-titulo); font-size: 1.25rem; font-weight: 650; font-variant-numeric: tabular-nums; }
    :host(.assistente-metas) p { margin: 0 0 8px; }
    :host(.assistente-metas) .links-assistente { margin: 0; }
    /* #359: o aviso da recomendação numa caixa de destaque; o texto corre numa linha e só quebra quando não cabe. */
    :host(.assistente-metas) .aviso-recomendacao {
      margin: 4px 0 12px; padding: 10px 14px; border-radius: 10px; line-height: 1.5;
      background: var(--primaria-suave); border-left: 4px solid var(--primaria); color: var(--texto);
    }
    .aviso-recomendacao strong { color: var(--texto-forte); }
    .aviso-recomendacao .valor-recomendado { font-variant-numeric: tabular-nums; }
    .aviso-recomendacao .limite-recomendacao { color: var(--texto-suave); }
    .aviso-recomendacao .limite-recomendacao::before { content: ' · '; }
    :host(.assistente-metas) .aviso-vermelho { background: var(--negativo-suave); border-left-color: var(--negativo); }
    .conta-sobra { font-size: 0.85rem; }
    .parte-sobra { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; margin: 0 0 8px; font-weight: 400; }
    .parte-sobra select { width: auto; }
  `
})
export class MetasAssistenteComponent {
  /** O caixa que a API calculou (o plano de cada meta vem pronto em `calculado`). */
  readonly plano = input.required<PlanoMetas>();
  /** O plano das metas já dividido; null enquanto o caixa não tem histórico suficiente. */
  readonly calculado = input<PlanoCalculado | null>(null);
  /** Quantas metas em andamento recebem a sobra. */
  readonly totalAtivas = input(0);
  /** Quanto da sobra vai para as metas (100 − margem de segurança). */
  readonly percentual = input(90);

  /** Escolheram guardar este percentual da sobra. */
  readonly mudarPercentual = output<number>();

  /** Guardar de 50% a 100% da sobra (a margem de segurança vai de 0% a 50%, ParametroService). */
  readonly opcoesPercentual = [50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100];
  readonly nomeMes = nomeMes;
}
