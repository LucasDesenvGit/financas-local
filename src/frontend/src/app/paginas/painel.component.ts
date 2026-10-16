import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Painel } from '../core/modelos';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MesAno, SeletorMesComponent } from '../compartilhado/seletor-mes.component';
import { NOMES_MESES_CURTOS } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';

@Component({
  selector: 'app-painel',
  imports: [MoedaPipe, DatePipe, DecimalPipe, RouterLink, GraficoComponent, SeletorMesComponent, IconeComponent],
  template: `
    <header class="cabecalho-pagina">
      <div>
        <h1>Painel</h1>
        @if (dados()?.ultimaSincronizacao; as u) {
          <p class="texto-suave">Última sincronização: {{ u | date: 'dd/MM/yyyy HH:mm' }}</p>
        }
      </div>
      <app-seletor-mes [valor]="periodo()" (mudou)="mudarPeriodo($event)" />
    </header>

    @if (erro()) { <p class="erro">{{ erro() }}</p> }

    @if (dados(); as d) {
      @if (d.alertas.length) {
        <section class="alertas">
          @for (a of d.alertas; track a) { <p>⚠ {{ a }}</p> }
        </section>
      }

      <section class="kpis">
        <article class="kpi kpi-destaque">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.patrimonio" [tamanho]="16" /> Patrimônio líquido</span>
          <strong>{{ d.patrimonioLiquido | currency }}</strong>
          <small>contas + investimentos (cotação atual) − cartões{{ d.dividaFinanciamentos + d.dividaEmprestimos > 0 ? ' − financiamentos − empréstimos' : '' }}</small>
        </article>
        <article class="kpi">
          <span>Patrimônio sem renda variável</span>
          <strong [class.negativo]="d.patrimonioSemRendaVariavel < 0">{{ d.patrimonioSemRendaVariavel | currency }}</strong>
          <small>sem FIIs, ações e ETFs ({{ d.rendaVariavel | currency }})</small>
        </article>
        <article class="kpi">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.banco" [tamanho]="16" /> Saldo em contas</span>
          <strong [class.negativo]="d.saldoContas < 0">{{ d.saldoContas | currency }}</strong>
        </article>
        <article class="kpi">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.contas" [tamanho]="16" /> Faturas de cartão</span>
          <strong class="negativo">{{ d.dividaCartoes | currency }}</strong>
        </article>
        @if (d.dividaFinanciamentos > 0) {
          <article class="kpi kpi-credito kpi-financiamento">
            <span class="kpi-rotulo"><app-icone [icone]="Icones.financiamento" [tamanho]="16" /> Financiamentos</span>
            <strong class="negativo">{{ d.dividaFinanciamentos | currency }}</strong>
            <small>saldo devedor · <a routerLink="/emprestimos">ver contratos</a></small>
          </article>
        }
        @if (d.dividaEmprestimos > 0) {
          <article class="kpi kpi-credito kpi-emprestimo">
            <span class="kpi-rotulo"><app-icone [icone]="Icones.emprestimos" [tamanho]="16" /> Empréstimos</span>
            <strong class="negativo">{{ d.dividaEmprestimos | currency }}</strong>
            <small>saldo devedor · <a routerLink="/emprestimos">ver contratos</a></small>
          </article>
        }
        <article class="kpi">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.investimentos" [tamanho]="16" /> Investimentos</span>
          <strong>{{ d.totalInvestido | currency }}</strong>
          <small><a routerLink="/investimentos">ver carteira x Selic</a></small>
        </article>
      </section>

      <section class="kpis kpis-mes">
        <article class="kpi">
          <span>Receitas do mês</span>
          <strong class="positivo">{{ d.receitasMes | currency }}</strong>
        </article>
        <article class="kpi">
          <span>Despesas do mês</span>
          <strong class="negativo">{{ d.despesasMes | currency }}</strong>
          @if (d.despesasEmergenciaMes > 0) {
            <small class="texto-emergencia">{{ d.despesasEmergenciaMes | currency }} em emergências</small>
          }
        </article>
        <article class="kpi">
          <span>Resultado</span>
          <strong [class.positivo]="d.receitasMes - d.despesasMes >= 0" [class.negativo]="d.receitasMes - d.despesasMes < 0">
            {{ d.receitasMes - d.despesasMes | currency }}
          </strong>
          @if (d.receitasMes > 0) {
            <small>{{ (d.receitasMes - d.despesasMes) / d.receitasMes * 100 | number: '1.0-0' }}% da renda poupada</small>
          }
        </article>
      </section>

      <div class="grade-2">
        <section class="painel-bloco">
          <h2>Gastos por categoria</h2>
          @if (d.gastosPorCategoria.length) {
            <app-grafico [configuracao]="graficoCategorias()" [altura]="240" />
            <ul class="lista-legenda">
              @for (g of d.gastosPorCategoria.slice(0, 8); track g.nome) {
                <li>
                  <span class="ponto" [style.background]="g.cor"></span>
                  <span class="cresce">{{ g.nome }}</span>
                  <span class="valor">{{ g.valor | currency }}</span>
                  <span class="texto-suave pct">{{ g.valor / d.despesasMes * 100 | number: '1.0-0' }}%</span>
                </li>
              }
            </ul>
          } @else {
            <p class="vazio">Nenhuma despesa neste mês.</p>
          }
        </section>

        <section class="painel-bloco">
          <h2>Receitas × despesas (12 meses)</h2>
          <app-grafico [configuracao]="graficoEvolucao()" [altura]="300" />
        </section>
      </div>

      <div class="grade-3">
        <section class="painel-bloco">
          <div class="bloco-titulo"><h2>Orçamentos</h2><a routerLink="/orcamentos">ver todos</a></div>
          @for (o of d.orcamentos; track o.id) {
            <div class="barra-orcamento">
              <div class="linha-flex">
                <span>{{ o.categoriaNome }}</span>
                <span class="texto-suave">{{ o.gasto | currency }} de {{ o.valor | currency }}</span>
              </div>
              <div class="trilho"><div class="progresso" [class.estourado]="o.situacao === 'estourado'"
                [class.emergencia]="o.situacao === 'emergencia'"
                [class.atencao]="o.percentual >= 80 && o.percentual <= 100"
                [style.width.%]="o.percentual > 100 ? 100 : o.percentual"></div></div>
            </div>
          } @empty {
            <p class="vazio">Nenhum orçamento para este mês. <a routerLink="/orcamentos">Criar</a></p>
          }
        </section>

        <section class="painel-bloco">
          <div class="bloco-titulo"><h2>Próximos vencimentos</h2><a routerLink="/contas-a-pagar">ver todos</a></div>
          <ul class="lista-simples">
            @for (c of d.proximasContas; track c.id) {
              <li>
                <span class="cresce">{{ c.descricao }}</span>
                <span [class.negativo]="c.diasParaVencer < 0" class="texto-suave">
                  {{ c.diasParaVencer < 0 ? 'vencida' : c.diasParaVencer === 0 ? 'hoje' : 'em ' + c.diasParaVencer + 'd' }}
                </span>
                <span class="valor">{{ c.valor | currency }}</span>
              </li>
            } @empty {
              <li class="vazio">Nada vencendo nos próximos {{ d.diasProximosVencimentos }} dias.</li>
            }
          </ul>
        </section>

        <section class="painel-bloco">
          <div class="bloco-titulo"><h2>Últimas transações</h2><a routerLink="/transacoes">ver todas</a></div>
          <ul class="lista-simples">
            @for (t of d.ultimasTransacoes; track t.id) {
              <li>
                <span class="data-curta">{{ t.data | date: 'dd/MM' }}</span>
                <span class="cresce truncar" [title]="t.descricao">{{ t.descricao }}</span>
                <span class="valor" [class.positivo]="t.valor > 0" [class.negativo]="t.valor < 0">{{ t.valor | currency }}</span>
              </li>
            } @empty {
              <li class="vazio">Sem transações ainda.</li>
            }
          </ul>
        </section>
      </div>
    } @else if (!erro()) {
      <p class="texto-suave">Carregando…</p>
    }
  `,
  styles: [`
    .kpi-credito { position: relative; overflow: hidden; }
    .kpi-credito::before { content: ""; position: absolute; inset: 0 auto 0 0; width: 5px; }
    .kpi-financiamento::before { background: #8fa8f0; }
    .kpi-emprestimo::before { background: #f6a467; }
  `]
})
export class PainelComponent implements OnInit {
  readonly Icones = Icones;
  private readonly api = inject(ApiService);

  readonly periodo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  readonly dados = signal<Painel | null>(null);
  readonly erro = signal<string | null>(null);

  readonly graficoCategorias = computed<ChartConfiguration<'doughnut'>>(() => {
    const itens = (this.dados()?.gastosPorCategoria ?? []).slice(0, 10);
    return {
      type: 'doughnut',
      data: {
        labels: itens.map((i) => i.nome),
        datasets: [{ data: itens.map((i) => i.valor), backgroundColor: itens.map((i) => i.cor), borderWidth: 0 }]
      },
      options: { cutout: '72%', plugins: { legend: { display: false } } }
    };
  });

  readonly graficoEvolucao = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.dados()?.evolucao ?? [];
    return {
      type: 'bar',
      data: {
        labels: meses.map((m) => `${NOMES_MESES_CURTOS[m.mes - 1]}/${String(m.ano).slice(2)}`),
        datasets: [
          { label: 'Receitas', data: meses.map((m) => m.receitas), backgroundColor: '#8fd5b4', borderRadius: 8, borderSkipped: false },
          { label: 'Despesas', data: meses.map((m) => m.despesas), backgroundColor: '#f5a3b0', borderRadius: 8, borderSkipped: false }
        ]
      },
      options: {
        plugins: { legend: { position: 'bottom' } },
        scales: { y: { ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { notation: 'compact' }) } } }
      }
    };
  });

  ngOnInit(): void {
    this.carregar();
  }

  mudarPeriodo(p: MesAno): void {
    this.periodo.set(p);
    this.carregar();
  }

  private carregar(): void {
    const { ano, mes } = this.periodo();
    this.erro.set(null);
    this.api.painel(ano, mes).subscribe({
      next: (d) => this.dados.set(d),
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }
}
