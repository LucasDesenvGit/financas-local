import { DatePipe } from '@angular/common';
import { Component, computed, inject, input, signal, ChangeDetectionStrategy } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, ParamMap, Router, RouterLink } from '@angular/router';
import { Conta, ContaAgendada, FormaCobranca, NOMES_FORMA_COBRANCA, Recorrencia } from '../core/modelos';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';
import { dataIso } from '../compartilhado/utilidades';
import {
  FILTRO_RECORRENTES_PADRAO, FiltroRecorrentes, GrupoRecorrentes, agruparRecorrentes, filtrarRecorrentes, rotuloQuantidade,
  valorPorMes
} from './recorrentes';

interface ChipFiltro { id: keyof FiltroRecorrentes; rotulo: string }

/**
 * #158: "Assinaturas e recorrentes", aba de Contas a pagar. O de-para de cada cobrança que se repete com a conta ou o
 * cartão em que ela cai.
 * - Shneiderman: o total por mês e por ano primeiro; depois o de-para por conta ou cartão; filtro e busca; o detalhe
 *   (as cobranças) por navegação, em Transações filtrada.
 * - Nielsen 6: onde cada cobrança cai fica escrito na tela, e a busca "onde cai…?" responde sem lembrar.
 * - Filtro ativo sempre visível (chips com ✕ e "Limpar filtros") e na URL, com replaceUrl (ADR 0018).
 */
@Component({
  selector: 'app-assinaturas-recorrentes',
  imports: [ListaSuspensaDirective, FormsModule, RouterLink, MoedaPipe, DatePipe],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="kpis" aria-label="Resumo das assinaturas e recorrentes">
      <article class="kpi kpi-destaque">
        <span>Por mês</span>
        <strong>{{ totalMes() | currency }}</strong>
        <small>{{ quantidadeAtivas() }} recorrente(s) ativa(s); as anuais entram divididas por 12</small>
      </article>
      <article class="kpi">
        <span>Por ano</span>
        <strong>{{ totalMes() * 12 | currency }}</strong>
        <small>o mesmo, em 12 meses</small>
      </article>
      <article class="kpi">
        <span>Onde caem</span>
        <strong>{{ gruposComValor() }}</strong>
        <small>conta(s) e cartão(ões)</small>
      </article>
    </section>

    <div class="filtros-recorrentes">
      <label>Onde cai…?
        <input type="search" name="busca" [ngModel]="filtro().busca" (ngModelChange)="mudar({ busca: $event })"
          placeholder="ex.: netflix" autocomplete="off"></label>
      <label>Conta ou cartão
        <select name="conta" [ngModel]="filtro().contaId" (ngModelChange)="mudar({ contaId: $event })">
          <option [ngValue]="null">Todos</option>
          @for (c of contasComRecorrentes(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
        </select>
      </label>
      <label>Como é cobrada
        <select name="forma" [ngModel]="filtro().forma" (ngModelChange)="mudar({ forma: $event })">
          <option [ngValue]="null">Todas</option>
          @for (f of formas; track f) { <option [ngValue]="f">{{ nomeForma(f) }}</option> }
        </select>
      </label>
      <label class="interruptor"><input type="checkbox" role="switch" name="todas" [ngModel]="filtro().todas"
        (ngModelChange)="mudar({ todas: $event })"> Mostrar as encerradas</label>
    </div>

    @if (chips().length) {
      <section class="faixa-filtros" aria-label="Filtros ativos">
        <strong>Filtrando:</strong>
        @for (c of chips(); track c.id) {
          <span class="chip chip-filtro">{{ c.rotulo }}
            <button type="button" class="chip-remover" (click)="removerFiltro(c.id)"
              [attr.aria-label]="'Remover filtro: ' + c.rotulo" [title]="'Remover filtro: ' + c.rotulo">✕</button>
          </span>
        }
        <button type="button" class="link" (click)="limparFiltros()">Limpar filtros</button>
      </section>
    }

    @for (g of grupos(); track g.contaId) {
      <section class="painel-bloco grupo-recorrentes" [attr.aria-labelledby]="'grupo-' + (g.contaId ?? 'sem')">
        <div class="bloco-titulo">
          <h2 [id]="'grupo-' + (g.contaId ?? 'sem')">{{ g.nome }}</h2>
          <span class="texto-suave">{{ rotulo(g) }} · {{ g.porMes | currency }}/mês</span>
        </div>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead>
              <tr><th>Cobrança</th><th>Como é cobrada</th><th>Dia</th><th>Última cobrança</th><th class="direita">Valor</th>
                <th><span class="so-leitor">Ações</span></th></tr>
            </thead>
            <tbody>
              @for (c of g.itens; track c.id) {
                <tr [class.ignorada]="!c.ativa">
                  <td><strong>{{ c.descricao }}</strong>
                    @if (!c.ativa) { <span class="etiqueta etiqueta-suave">encerrada</span> }
                    @if (c.observacao) { <br><small class="texto-suave">{{ c.observacao }}</small> }</td>
                  <td>{{ nomeForma(c.formaCobranca) }}</td>
                  <td class="nowrap">{{ diaDaCobranca(c) }}</td>
                  <td class="nowrap">{{ c.ultimoPagamentoEm ? (c.ultimoPagamentoEm | date: 'dd/MM/yyyy') : '—' }}</td>
                  <td class="direita valor nowrap">{{ c.valor | currency }}
                    @if (c.recorrencia === Recorrencia.Anual) {
                      <br><small class="texto-suave">por ano ({{ porMes(c) | currency }}/mês)</small>
                    }</td>
                  <td class="direita nowrap">
                    <a class="link" routerLink="/transacoes" [queryParams]="linkCobrancas(c)"
                      [attr.aria-label]="'Ver as cobranças de ' + c.descricao">ver as cobranças</a>
                  </td>
                </tr>
              }
            </tbody>
          </table>
        </div>
      </section>
    } @empty {
      <p class="vazio">
        @if (chips().length) { Nada com esses filtros. }
        @else { Nenhuma cobrança recorrente cadastrada. Cadastre pelas sugestões do extrato, na aba Contas a pagar. }
      </p>
    }
  `,
  styles: `
    .filtros-recorrentes { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; margin: 12px 0; }
    .filtros-recorrentes label { display: flex; flex-direction: column; gap: 4px; min-width: 180px; }
    .filtros-recorrentes label.interruptor { flex-direction: row; align-items: center; min-width: 0; }
    .grupo-recorrentes { margin-top: 12px; }
  `
})
export class AssinaturasRecorrentesComponent {
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly itens = input<ContaAgendada[]>([]);
  readonly contas = input<Conta[]>([]);

  readonly Recorrencia = Recorrencia;
  readonly formas = [FormaCobranca.AMao, FormaCobranca.DebitoAutomatico, FormaCobranca.CartaoCredito];
  readonly filtro = signal<FiltroRecorrentes>(this.filtroDaUrl(this.rota.snapshot.queryParamMap));

  constructor() {
    // O voltar do navegador e os links (ex.: a linha-resumo de Contas e cartões) mudam só os query params.
    this.rota.queryParamMap.pipe(takeUntilDestroyed()).subscribe((p) => this.filtro.set(this.filtroDaUrl(p)));
  }

  readonly visiveis = computed(() => filtrarRecorrentes(this.itens(), this.filtro()));
  readonly grupos = computed<GrupoRecorrentes[]>(() => agruparRecorrentes(this.visiveis()));
  readonly totalMes = computed(() => this.visiveis().filter((c) => c.ativa).reduce((s, c) => s + valorPorMes(c), 0));
  readonly quantidadeAtivas = computed(() => this.visiveis().filter((c) => c.ativa).length);
  readonly gruposComValor = computed(() => this.grupos().filter((g) => g.porMes > 0).length);
  /** Só as contas e cartões que têm alguma recorrente, para o filtro não oferecer o que não acha nada. */
  readonly contasComRecorrentes = computed(() => {
    const ids = new Set(this.itens().filter((c) => c.recorrencia !== Recorrencia.Unica).map((c) => c.contaId));
    return this.contas().filter((c) => ids.has(c.id));
  });

  readonly chips = computed<ChipFiltro[]>(() => {
    const f = this.filtro();
    const chips: ChipFiltro[] = [];
    if (f.busca.trim()) chips.push({ id: 'busca', rotulo: `“${f.busca.trim()}”` });
    if (f.contaId !== null) {
      const nome = this.contas().find((c) => c.id === f.contaId)?.nome
        ?? this.itens().find((c) => c.contaId === f.contaId)?.contaNome ?? 'conta';
      chips.push({ id: 'contaId', rotulo: nome });
    }
    if (f.forma !== null) chips.push({ id: 'forma', rotulo: this.nomeForma(f.forma) });
    if (f.todas) chips.push({ id: 'todas', rotulo: 'com as encerradas' });
    return chips;
  });

  nomeForma(f: FormaCobranca): string {
    return NOMES_FORMA_COBRANCA[f] ?? NOMES_FORMA_COBRANCA[FormaCobranca.AMao];
  }

  rotulo(g: GrupoRecorrentes): string {
    return rotuloQuantidade(g.itens);
  }

  porMes(c: ContaAgendada): number {
    return valorPorMes(c);
  }

  /** "dia 12" na mensal; "12/11" na anual. */
  diaDaCobranca(c: ContaAgendada): string {
    const [, mes, dia] = c.proximoVencimento.split('-');
    return c.recorrencia === Recorrencia.Anual ? `${dia}/${mes}` : `dia ${Number(dia)}`;
  }

  /** Transações do último ano com o texto da cobrança (ou o nome), na conta ou cartão dela. */
  linkCobrancas(c: ContaAgendada): Record<string, string> {
    const hoje = new Date();
    const p: Record<string, string> = {
      busca: c.textoCobranca ?? c.descricao,
      tipo: 'despesa',
      inicio: dataIso(new Date(hoje.getFullYear() - 1, hoje.getMonth(), 1)),
      fim: dataIso(hoje)
    };
    if (c.contaId !== null) p['conta'] = String(c.contaId);
    return p;
  }

  mudar(parcial: Partial<FiltroRecorrentes>): void {
    this.filtro.update((f) => ({ ...f, ...parcial }));
    this.gravarNaUrl();
  }

  removerFiltro(id: keyof FiltroRecorrentes): void {
    this.mudar({ [id]: FILTRO_RECORRENTES_PADRAO[id] } as Partial<FiltroRecorrentes>);
  }

  limparFiltros(): void {
    this.filtro.set({ ...FILTRO_RECORRENTES_PADRAO });
    this.gravarNaUrl();
  }

  private gravarNaUrl(): void {
    const f = this.filtro();
    void this.router.navigate([], {
      relativeTo: this.rota, replaceUrl: true,
      queryParams: {
        aba: 'recorrentes',
        busca: f.busca.trim() || null,
        conta: f.contaId ?? null,
        forma: f.forma ?? null,
        todas: f.todas ? '1' : null
      }
    });
  }

  private filtroDaUrl(p: ParamMap): FiltroRecorrentes {
    const conta = Number(p.get('conta'));
    const forma = Number(p.get('forma'));
    return {
      busca: p.get('busca') ?? '',
      contaId: Number.isInteger(conta) && conta > 0 ? conta : null,
      forma: [FormaCobranca.AMao, FormaCobranca.DebitoAutomatico, FormaCobranca.CartaoCredito].includes(forma) ? forma : null,
      todas: p.get('todas') === '1'
    };
  }
}
