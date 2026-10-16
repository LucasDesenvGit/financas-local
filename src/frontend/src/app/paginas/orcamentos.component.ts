import { DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Categoria, Orcamento, TipoCategoria, TipoTransacao, ValorCategoria } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { MesAno, SeletorMesComponent } from '../compartilhado/seletor-mes.component';
import { mesAnoDaUrl, paramsTransacoes, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

interface LinhaOrcamento {
  categoria: Categoria;
  orcamento: Orcamento | null;
  gasto: number;
  valorEditado: number | null;
}

@Component({
  selector: 'app-orcamentos',
  imports: [FormsModule, RouterLink, MoedaPipe, DecimalPipe, SeletorMesComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Orçamentos</h1>
      <div class="acoes">
        <app-seletor-mes [valor]="periodo()" (mudou)="mudarPeriodo($event)" />
        <button type="button" class="botao" (click)="copiar()">Copiar do mês anterior</button>
      </div>
    </header>

    <section class="kpis">
      <article class="kpi"><span>Orçado</span><strong>{{ totalOrcado() | currency }}</strong></article>
      <article class="kpi"><span>Gasto nas categorias orçadas</span><strong>{{ totalGastoOrcado() | currency }}</strong></article>
      @if (totalEmergencia() > 0) {
        <article class="kpi">
          <span>Emergências no mês</span>
          <strong class="texto-emergencia">{{ totalEmergencia() | currency }}</strong>
          <small>gastos marcados como emergência</small>
        </article>
      }
      <article class="kpi">
        <span>Disponível</span>
        <strong [class.negativo]="totalOrcado() - totalGastoOrcado() < 0">{{ totalOrcado() - totalGastoOrcado() | currency }}</strong>
      </article>
    </section>

    <p class="texto-suave">Defina um limite mensal por categoria de despesa. Subcategorias somam no orçamento da categoria pai.
      Deixe em branco para não orçar.
      <span class="texto-emergencia">Laranja</span> = passou do limite só por causa de gastos de emergência.</p>

    <div class="tabela-rolagem">
      <table class="tabela">
        <thead>
          <tr><th>Categoria</th><th class="direita">Gasto</th><th style="width: 160px">Limite (R$)</th><th style="width: 30%">Progresso</th></tr>
        </thead>
        <tbody>
          @for (l of linhas(); track l.categoria.id) {
            <tr>
              <td><a class="chip chip-link" routerLink="/transacoes" [queryParams]="linkCategoria(l.categoria.id)"
                [attr.aria-label]="'Ver transações de ' + l.categoria.nome"><span class="ponto" [style.background]="l.categoria.cor"></span>{{ l.categoria.nome }}</a></td>
              <td class="direita valor">{{ l.gasto | currency }}
                @if (l.orcamento && l.orcamento.gastoEmergencia > 0) {
                  <br><small class="texto-emergencia">{{ l.orcamento.gastoEmergencia | currency }} emergência</small>
                }</td>
              <td>
                <input type="number" min="0" step="10" [(ngModel)]="l.valorEditado" (change)="salvar(l)" class="input-compacto"
                  [attr.aria-label]="'Limite de ' + l.categoria.nome">
              </td>
              <td>
                @if (l.orcamento; as orc) {
                  <div class="trilho"><div class="progresso" [class.estourado]="pct(l) > 100 && orc.situacao !== 'emergencia'"
                    [class.emergencia]="orc.situacao === 'emergencia'" [class.atencao]="pct(l) >= 80 && pct(l) <= 100"
                    [style.width.%]="pct(l) > 100 ? 100 : pct(l)"></div></div>
                  <small [class.texto-suave]="orc.situacao !== 'emergencia'" [class.texto-emergencia]="orc.situacao === 'emergencia'">
                    {{ pct(l) | number: '1.0-0' }}%
                    @if (orc.situacao === 'emergencia') { · estourou por emergência }
                  </small>
                }
              </td>
            </tr>
          }
        </tbody>
      </table>
    </div>
  `
})
export class OrcamentosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly periodo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  readonly linhas = signal<LinhaOrcamento[]>([]);

  /** UX-3: despesas da categoria no mês do orçamento. */
  linkCategoria(categoriaId: number): Record<string, string> {
    const { ano, mes } = this.periodo();
    return paramsTransacoes({ categoriaId, inicio: primeiroDiaMes(ano, mes), fim: ultimoDiaMes(ano, mes), tipo: 'despesa' });
  }

  readonly totalOrcado = computed(() => this.linhas().reduce((s, l) => s + (l.orcamento?.valor ?? 0), 0));
  readonly totalEmergencia = computed(() => this.linhas().reduce((s, l) => s + (l.orcamento?.gastoEmergencia ?? 0), 0));
  readonly totalGastoOrcado = computed(() => this.linhas().reduce((s, l) => s + (l.orcamento ? l.gasto : 0), 0));

  ngOnInit(): void {
    const daUrl = mesAnoDaUrl(this.rota.snapshot.queryParamMap);
    if (daUrl) this.periodo.set(daUrl);
    this.carregar();
  }

  mudarPeriodo(p: MesAno): void {
    this.periodo.set(p);
    // Mês na URL: ao voltar de Transações, Orçamentos reabre no mesmo mês.
    void this.router.navigate([], { relativeTo: this.rota, queryParams: { ano: p.ano, mes: p.mes }, replaceUrl: true });
    this.carregar();
  }

  pct(l: LinhaOrcamento): number {
    return l.orcamento && l.orcamento.valor > 0 ? (l.gasto / l.orcamento.valor) * 100 : 0;
  }

  carregar(): void {
    const { ano, mes } = this.periodo();
    forkJoin({
      categorias: this.api.categorias(),
      orcamentos: this.api.orcamentos(ano, mes),
      gastos: this.api.relatorioCategorias(primeiroDiaMes(ano, mes), ultimoDiaMes(ano, mes), TipoTransacao.Despesa, true)
    }).subscribe({
      next: ({ categorias, orcamentos, gastos }) => this.montar(categorias, orcamentos, gastos),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  salvar(l: LinhaOrcamento): void {
    const { ano, mes } = this.periodo();
    const valor = l.valorEditado === null || (l.valorEditado as unknown) === '' ? null : Number(l.valorEditado);
    const req = valor === null || isNaN(valor)
      ? (l.orcamento ? this.api.excluirOrcamento(l.orcamento.id) : null)
      : this.api.salvarOrcamento(l.categoria.id, ano, mes, valor);
    if (!req) return;
    req.subscribe({
      next: () => this.carregar(),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  copiar(): void {
    const { ano, mes } = this.periodo();
    this.api.copiarOrcamentos(ano, mes).subscribe({
      next: (r) => {
        this.avisos.sucesso(r.transacoesAtualizadas ? `${r.transacoesAtualizadas} orçamento(s) copiado(s).` : 'Nada novo para copiar.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  private montar(categorias: Categoria[], orcamentos: Orcamento[], gastos: ValorCategoria[]): void {
    const principais = categorias.filter((c) => c.tipo === TipoCategoria.Despesa && c.ativa && !c.categoriaPaiId);
    const linhas = principais.map<LinhaOrcamento>((c) => {
      const orcamento = orcamentos.find((o) => o.categoriaId === c.id) ?? null;
      const gasto = gastos.find((g) => g.categoriaId === c.id)?.valor ?? 0;
      return { categoria: c, orcamento, gasto, valorEditado: orcamento?.valor ?? null };
    });
    linhas.sort((a, b) => Number(!!b.orcamento) - Number(!!a.orcamento) || b.gasto - a.gasto);
    this.linhas.set(linhas);
  }
}
