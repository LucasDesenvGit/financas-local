import { DatePipe } from '@angular/common';
import { Component, computed, inject, input, signal, ChangeDetectionStrategy } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, ParamMap, Router, RouterLink } from '@angular/router';
import { Conta, ContaAgendada, FormaCobranca, NOMES_FORMA_COBRANCA, Recorrencia, TipoConta } from '../core/modelos';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { LogoBancoComponent } from '../compartilhado/logo-banco.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';
import { dataIso } from '../compartilhado/utilidades';
import {
  FILTRO_RECORRENTES_PADRAO, FiltroRecorrentes, GrupoRecorrentes, agruparRecorrentes, chaveGrupo, filtrarRecorrentes,
  guardarGruposFechados, lerGruposFechados, rotuloQuantidade, valorPorMes
} from './recorrentes';

interface ChipFiltro { id: keyof FiltroRecorrentes; rotulo: string }

/**
 * #158: "Assinaturas e recorrentes", aba de Contas a pagar. O de-para de cada cobrança que se repete com a conta ou o
 * cartão em que ela cai.
 * - Shneiderman: o total por mês e por ano primeiro; depois o de-para por conta ou cartão; filtro e busca; o detalhe
 *   (as cobranças) por navegação, em Transações filtrada.
 * - Nielsen 6: onde cada cobrança cai fica escrito na tela, e a busca "onde cai…?" responde sem lembrar.
 * - Filtro ativo sempre visível (chips com ✕ e "Limpar filtros") e na URL, com replaceUrl (ADR 0018).
 * - #242: cada conta ou cartão é uma sanfona (APG Disclosure; NN/g, Accordions on Desktop): o título é um botão com
 *   aria-expanded; fechado, ainda mostra o selo, o nome, a quantidade e o total por mês (overview first). Vários
 *   abertos ao mesmo tempo, todos abertos no começo, como o menu lateral (ADR 0046); os fechados ficam lembrados neste
 *   navegador (ADR 0017). O selo do banco é decorativo: o nome vem escrito ao lado (ADR 0024).
 */
@Component({
  selector: 'app-assinaturas-recorrentes',
  imports: [ListaSuspensaDirective, FormsModule, RouterLink, MoedaPipe, DatePipe, LogoBancoComponent, IconeComponent],
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

    @if (grupos().length > 1) {
      <div class="acoes-grupos">
        <button type="button" class="link" (click)="alternarTodos()">{{ algumAberto() ? 'Fechar todos' : 'Abrir todos' }}</button>
      </div>
    }

    @for (g of grupos(); track g.contaId) {
      <section class="painel-bloco grupo-recorrentes" [class.fechado]="!aberto(g)" [attr.aria-labelledby]="'grupo-' + chave(g)">
        <h2 class="grupo-recorrentes-titulo">
          <button type="button" class="grupo-recorrentes-botao" [id]="'grupo-' + chave(g)" [attr.aria-expanded]="aberto(g)"
            [attr.aria-controls]="'grupo-' + chave(g) + '-conteudo'" (click)="alternar(g)">
            <app-logo-banco [codigo]="bancoDe(g.contaId)" [cartao]="ehCartao(g.contaId)" [tamanho]="26" />
            <span class="grupo-recorrentes-nome">{{ g.nome }}</span>
            <span class="texto-suave grupo-recorrentes-resumo">{{ rotulo(g) }} · {{ g.porMes | currency }}/mês</span>
            <app-icone class="grupo-seta" [icone]="Icones.setaGrupo" [tamanho]="16" />
          </button>
        </h2>
        <!-- Fechado: fora do Tab e do leitor de tela (inert + visibility), sem sair do DOM, para animar a altura. -->
        <div class="grupo-recorrentes-conteudo" [id]="'grupo-' + chave(g) + '-conteudo'" [class.fechado]="!aberto(g)"
          [attr.inert]="aberto(g) ? null : ''">
        <div class="grupo-recorrentes-dentro">
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
        </div>
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
    .acoes-grupos { display: flex; justify-content: flex-end; margin: 4px 0 -4px; }
    .grupo-recorrentes-titulo { font-size: 1rem; }
    .grupo-recorrentes-botao {
      display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; width: 100%; min-height: 44px; padding: 4px 6px;
      margin: -4px -6px; box-sizing: content-box; border: 0; border-radius: var(--raio-pequeno); background: none;
      font: inherit; font-weight: 600; color: var(--texto-forte); text-align: left; cursor: pointer;
      transition: background-color var(--transicao);
    }
    .grupo-recorrentes-botao:hover { background: var(--superficie-2); }
    .grupo-recorrentes-nome { flex: 1 1 160px; min-width: 0; }
    .grupo-recorrentes-resumo { font-weight: 400; font-size: 0.88rem; }
    .grupo-recorrentes-botao .grupo-seta { color: var(--texto-suave); transition: transform 0.2s ease; }
    .grupo-recorrentes-botao[aria-expanded="true"] .grupo-seta { transform: rotate(90deg); }
    /* Fechado, o título não tem conteúdo embaixo: sem a linha e a folga do título de bloco (styles.css, "Divisórias"). */
    .grupo-recorrentes.fechado > .grupo-recorrentes-titulo { border-bottom: 0; padding-bottom: 0; margin-bottom: 0; }
    .grupo-recorrentes-conteudo {
      display: grid; grid-template-rows: 1fr; transition: grid-template-rows 0.2s ease, visibility 0s linear 0s;
    }
    .grupo-recorrentes-conteudo.fechado {
      grid-template-rows: 0fr; visibility: hidden; transition: grid-template-rows 0.2s ease, visibility 0s linear 0.2s;
    }
    .grupo-recorrentes-dentro { min-height: 0; overflow: hidden; }
  `
})
export class AssinaturasRecorrentesComponent {
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly itens = input<ContaAgendada[]>([]);
  readonly contas = input<Conta[]>([]);

  readonly Recorrencia = Recorrencia;
  readonly Icones = Icones;
  /** #242: grupos fechados (por conta ou cartão), lembrados neste navegador. */
  private readonly fechados = signal(lerGruposFechados());
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

  chave(g: GrupoRecorrentes): string {
    return chaveGrupo(g.contaId);
  }

  aberto(g: GrupoRecorrentes): boolean {
    return !this.fechados().has(chaveGrupo(g.contaId));
  }

  readonly algumAberto = computed(() => this.grupos().some((g) => !this.fechados().has(chaveGrupo(g.contaId))));

  alternar(g: GrupoRecorrentes): void {
    const chave = chaveGrupo(g.contaId);
    this.fechados.update((atual) => {
      const novo = new Set(atual);
      if (novo.has(chave)) novo.delete(chave); else novo.add(chave);
      return novo;
    });
    guardarGruposFechados(this.fechados());
  }

  /** "Fechar todos" com algum aberto; "Abrir todos" com todos fechados (Nielsen 3). */
  alternarTodos(): void {
    const fechar = this.algumAberto();
    this.fechados.update((atual) => {
      const novo = new Set(atual);
      for (const g of this.grupos()) {
        if (fechar) novo.add(chaveGrupo(g.contaId)); else novo.delete(chaveGrupo(g.contaId));
      }
      return novo;
    });
    guardarGruposFechados(this.fechados());
  }

  /** ADR 0024: o banco que vale para a conta (null = selo neutro). */
  bancoDe(contaId: number | null): string | null {
    return this.contas().find((c) => c.id === contaId)?.banco ?? null;
  }

  ehCartao(contaId: number | null): boolean {
    return this.contas().find((c) => c.id === contaId)?.tipo === TipoConta.CartaoCredito;
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
