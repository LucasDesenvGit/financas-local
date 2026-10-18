import { DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Categoria, Orcamento, TipoCategoria, TipoTransacao, ValorCategoria } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { MesAno, SeletorMesComponent } from '../compartilhado/seletor-mes.component';
import { mesAnoDaUrl, paramsTransacoes, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { ModalComponent } from '../compartilhado/modal.component';
import { SugestaoOrcamentosComponent } from './sugestao-orcamentos.component';

interface LinhaOrcamento {
  categoria: Categoria;
  orcamento: Orcamento | null;
  gasto: number;
  valorEditado: number | null;
}

@Component({
  selector: 'app-orcamentos',
  imports: [FormsModule, RouterLink, MoedaPipe, DecimalPipe, SeletorMesComponent, ModalComponent, SugestaoOrcamentosComponent,
    CampoValorComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Orçamentos</h1>
      <div class="acoes">
        <app-seletor-mes [valor]="periodo()" (mudou)="mudarPeriodo($event)" />
        <button type="button" class="botao" (click)="copiar()">Copiar do mês anterior</button>
        <button type="button" class="botao botao-primario" (click)="sugestaoAberta.set(true)">✨ Sugerir orçamentos</button>
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

    <div class="texto-suave explicacao-orcamentos">
      <p>Defina um limite mensal por categoria de despesa. Subcategorias somam no orçamento da categoria pai.
        Deixe em branco para não orçar.</p>
      <p><span class="texto-emergencia">Laranja</span> = passou do limite só por causa de gastos de emergência.</p>
    </div>

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
                <!-- #132: salva ao sair do campo com o valor mudado, ou com Enter. -->
                <app-campo-valor compacto enterConfirma [(ngModel)]="l.valorEditado" (confirmado)="salvar(l)"
                  [rotulo]="'Limite de ' + l.categoria.nome" />

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

    <app-modal [aberto]="sugestaoAberta()" titulo="✨ Sugestão de orçamentos" (fechar)="sugestaoAberta.set(false)">
      @if (sugestaoAberta()) {
        <app-sugestao-orcamentos [ano]="periodo().ano" [mes]="periodo().mes"
          (aplicado)="sugestaoAplicada($event)" (fechar)="sugestaoAberta.set(false)" />
      }
    </app-modal>
  `,
  styles: `
    :host ::ng-deep .modal:has(.tabela-sugestao-orcamento) { width: min(1100px, 96vw); }
  `
})
export class OrcamentosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly periodo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  readonly linhas = signal<LinhaOrcamento[]>([]);
  /** #39: janela de sugestão de orçamentos. */
  readonly sugestaoAberta = signal(false);

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

  sugestaoAplicada(quantidade: number): void {
    this.sugestaoAberta.set(false);
    this.avisos.sucesso(`${quantidade} orçamento(s) aplicado(s).`);
    this.carregar();
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
