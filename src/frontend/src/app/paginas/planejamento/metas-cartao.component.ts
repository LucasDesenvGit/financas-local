import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, ChangeDetectionStrategy, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import type { ChartConfiguration } from 'chart.js';
import { GraficoExplicadoComponent } from '../../compartilhado/graficos/grafico-explicado.component';
import { GraficoComponent } from '../../compartilhado/graficos/grafico.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { IconeComponent } from '../../compartilhado/ui/icone.component';
import { Icones, iconeDaMeta } from '../../compartilhado/ui/icones';
import { Meta } from '../../core/modelos';
import { mesesAposMostrarMais, nomeMesCurto, porMesAcimaDaSobra, prazoVencido } from './metas';
import { EvolucaoDaMeta, comoLerEvolucao } from './metas-evolucao';
import { MesRef, PlanoDaMeta } from './metas-plano';

/**
 * O cartão de uma meta na tela de Metas (#732, ADR 0059): a sanfona com o resumo à esquerda e o plano à direita.
 * Componente de apresentação: recebe a meta, o plano dela (já calculado) e o que está aberto, e avisa por `output()`;
 * não chama a API nem lê a rota. O elemento do componente é o próprio cartão (classe `cartao-meta`, `role="article"`,
 * ADR 0059, "Na prática": as regras globais de `.cartao-meta` continuam casando). Só o que é de uma meta e não
 * precisa ser lembrado fora dela fica aqui: o plano mês a mês aberto e quantos meses ele mostra. Os links para outras
 * telas vêm da tela por projeção de conteúdo (`link-anual`, `link-orcamentos`, `link-reservas`): quem navega é ela.
 */
@Component({
  selector: 'app-metas-cartao',
  imports: [FormsModule, DatePipe, DecimalPipe, MoedaPipe, CampoValorComponent, IconeComponent, GraficoComponent, GraficoExplicadoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  host: {
    class: 'cartao-meta', role: 'article',
    '[class.concluida]': 'meta().concluida', '[class.fechada]': '!aberta()'
  },
  templateUrl: './metas-cartao.component.html',
  styleUrl: './metas-cartao.component.css'
})
export class MetasCartaoComponent {
  readonly meta = input.required<Meta>();
  /** A recomendação da meta; null sem histórico suficiente (o cartão mostra a divisão igual) ou com a meta concluída. */
  readonly plano = input<PlanoDaMeta | null>(null);
  /** Quanto do mês atual pode ir para as metas, para avisar quando o valor por mês escolhido passa dele. */
  readonly limiteMes = input<number | undefined>(undefined);
  /** O mês do plano, para saber se o prazo da meta já passou. */
  readonly mesDoPlano = input<MesRef | null>(null);
  /** A sanfona da meta está aberta (lembrada neste navegador, ADR 0017). */
  readonly aberta = input(true);
  /** Posição na ordem de prioridade (1 = primeira); 0 = fora da ordem (concluída). */
  readonly posicao = input(0);
  readonly totalAtivas = input(0);
  /** A caixa "Quanto consigo guardar este mês" está aberta (uma meta por vez) e o valor que a pessoa escolheu. */
  readonly ajusteAberto = input(false);
  readonly ajuste = input<number | null>(null);
  /** O "Como está indo" desta meta está aberto, com os aportes dela (null enquanto carregam), o resumo e o gráfico. */
  readonly evolucaoAberta = input(false);
  readonly evolucao = input<EvolucaoDaMeta | null>(null);
  readonly resumoEvolucao = input('');
  readonly grafico = input<ChartConfiguration<'line'> | null>(null);

  readonly alternar = output<void>();
  readonly mover = output<-1 | 1>();
  readonly alternarAjuste = output<void>();
  /** Quanto a pessoa consegue guardar este mês nesta meta (vazio volta à recomendação). */
  readonly ajustar = output<number | null>();
  readonly limparAjuste = output<void>();
  readonly alternarEvolucao = output<void>();
  readonly aporte = output<void>();
  readonly editar = output<void>();
  readonly excluir = output<void>();

  readonly iconeDaMeta = iconeDaMeta;
  readonly setaGrupo = Icones.setaGrupo;
  readonly nomeMesCurto = nomeMesCurto;
  readonly comoLerEvolucao = comoLerEvolucao;

  /** #335: o plano mês a mês vem aberto; "Plano mês a mês" o fecha. */
  readonly planoAberto = signal(true);
  /** #335: quantos meses o plano mostra (5 no começo; "Mostrar mais" soma 5, até os 12 do plano). */
  readonly passoMeses = 5;
  readonly mesesNoPlano = signal(this.passoMeses);

  vencido(): boolean {
    return prazoVencido(this.meta().dataAlvo, this.mesDoPlano());
  }

  /** #359: a parte da sobra para as metas este mês, quando o valor por mês escolhido passa dela; senão, null. */
  acimaDaSobra(pm: PlanoDaMeta): number | null {
    return porMesAcimaDaSobra(pm.porMes, this.limiteMes());
  }

  alternarPlano(): void {
    this.planoAberto.update((v) => !v);
  }

  mostrarMaisMeses(total: number): void {
    this.mesesNoPlano.update((atual) => mesesAposMostrarMais(atual, total, this.passoMeses));
  }

  mostrarMenosMeses(): void {
    this.mesesNoPlano.set(this.passoMeses);
  }

  /** Quantos meses o "Mostrar mais" acrescenta agora. */
  quantosMais(total: number): number {
    return Math.min(this.passoMeses, total - this.mesesNoPlano());
  }
}
