import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { PrivacidadeService } from '../../core/privacidade.service';
import { Conta, Meta, MetaAporte, PlanoMetas, Reserva } from '../../core/modelos';
import {
  FormMeta, ajustesParaUrl, alternarNoConjunto, comAjuste, dtoDaMeta, fechadasAoAlternarTodas, formularioDaMeta,
  formularioVazio, graficoDaEvolucao, hojeDaEvolucao, lerAjustes, lerFechadas, limiteDoPorMes, ordemAposMover,
  posicaoNaOrdem, semAjuste
} from './metas';
import { MetasAporteComponent } from './metas-aporte.component';
import { MetasAssistenteComponent } from './metas-assistente.component';
import { MetasCartaoComponent } from './metas-cartao.component';
import { resumoEvolucao, evolucaoDaMeta } from './metas-evolucao';
import { MetasFormularioComponent } from './metas-formulario.component';
import { PlanoDaMeta, planejarMetas } from './metas-plano';

/**
 * Metas (#732, ADR 0059): a tela guarda o estado (as metas, o plano, o que está aberto, a simulação na URL) e fala com
 * a API; o assistente, o cartão de cada meta e as janelas de meta e de aporte são componentes de apresentação
 * (`metas-*.component.ts`), e a regra mora em funções puras (`metas.ts`, `metas-plano.ts`, `metas-evolucao.ts`).
 * Os links para outras telas ficam aqui e entram nos componentes por projeção de conteúdo: quem navega é a tela.
 */
@Component({
  selector: 'app-metas',
  imports: [RouterLink, MetasAssistenteComponent, MetasCartaoComponent, MetasFormularioComponent, MetasAporteComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Metas</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova meta</button>
    </header>

    @if (plano(); as p) {
      <app-metas-assistente [plano]="p" [calculado]="calculado()" [totalAtivas]="ativas().length" [percentual]="percentual()"
        (mudarPercentual)="mudarPercentual($event)">
        <a link-projecao routerLink="/projecao">Ver projeção →</a>
      </app-metas-assistente>
    }

    <!-- #335: cada meta é uma sanfona (o cartão), como os grupos de "Assinaturas e recorrentes" (ADR 0046): todas abertas
         no começo, as fechadas lembradas neste navegador (ADR 0017). -->
    @if (metas().length > 1) {
      <div class="acoes-metas">
        <button type="button" class="link" (click)="alternarTodasMetas()">{{ algumaMetaAberta() ? 'Fechar todas' : 'Abrir todas' }}</button>
      </div>
    }
    <section class="grade-cartoes lista-metas">
      @for (m of metas(); track m.id) {
        <app-metas-cartao [meta]="m" [plano]="planoDe(m)" [limiteMes]="calculado()?.limiteMes" [mesDoPlano]="mesDoPlano()"
          [aberta]="metaAberta(m.id)" [posicao]="posicaoNaOrdem(ativas(), m.id)" [totalAtivas]="ativas().length"
          [ajusteAberto]="ajusteAberto() === m.id" [ajuste]="ajustes().get(m.id) ?? null"
          [evolucaoAberta]="evolucaoAberta() === m.id" [evolucao]="evolucao()" [resumoEvolucao]="resumoDaEvolucao() ?? ''"
          [grafico]="graficoEvolucao()"
          (alternar)="alternarMeta(m.id)" (mover)="mover(m, $event)" (alternarAjuste)="alternarAjuste(m.id)"
          (ajustar)="ajustar(m.id, $event)" (limparAjuste)="limparAjuste(m.id)" (alternarEvolucao)="alternarEvolucao(m.id)"
          (aporte)="abrirAporte(m)" (editar)="editar(m)" (excluir)="excluirMeta(m)">
          <a link-anual routerLink="/anual">Ver onde cortar (50/30/20)</a>
          <a link-orcamentos routerLink="/orcamentos">Orçamentos</a>
          <a link-reservas routerLink="/investimentos" [queryParams]="{ aba: 'reservas' }">confira em Reservas</a>
        </app-metas-cartao>
      } @empty {
        <p class="vazio">Crie metas como reserva de emergência, viagem ou entrada de um imóvel.</p>
      }
    </section>

    <app-metas-formulario [formulario]="formulario()" [contas]="contas()" [reservas]="reservas()" [limitePorMes]="limitePorMes()"
      (salvar)="salvar($event)" (excluir)="excluir()" (fechar)="formulario.set(null)">
      <a link-reservas routerLink="/investimentos" [queryParams]="{ aba: 'reservas' }" (click)="formulario.set(null)">Confira em Investimentos › Reservas</a>
    </app-metas-formulario>

    <app-metas-aporte [meta]="aporteMeta()" (registrar)="registrarAporte($event)" (fechar)="aporteMeta.set(null)" />
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    /* #335: cada meta é uma sanfona na largura toda (a lista é uma coluna). */
    .lista-metas { grid-template-columns: minmax(0, 1fr); }
    .acoes-metas { display: flex; justify-content: flex-end; margin: 4px 0 -4px; }
  `
})
export class MetasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly avisos = inject(AvisosService);
  private readonly router = inject(Router);
  private readonly rota = inject(ActivatedRoute);
  private readonly privacidade = inject(PrivacidadeService);

  readonly posicaoNaOrdem = posicaoNaOrdem;
  /** #335: metas fechadas na sanfona, lembradas neste navegador (ADR 0017); todas abertas no começo. */
  static readonly CHAVE_FECHADAS = 'financas.metasFechadas';
  private readonly metasFechadas = signal<ReadonlySet<number>>(MetasComponent.lerFechadas());
  readonly algumaMetaAberta = computed(() => this.metas().some((m) => !this.metasFechadas().has(m.id)));

  readonly metas = signal<Meta[]>([]);
  readonly contas = signal<Conta[]>([]);
  /** #89: cofrinhos (reservas do extrato), para o campo da meta. */
  readonly reservas = signal<Reserva[]>([]);
  /** #90/#149: o caixa que vem da API; o plano das metas é calculado aqui (metas-plano.ts). */
  readonly plano = signal<PlanoMetas | null>(null);
  /** #149 (C): quanto da sobra vai para as metas (100 − margem de segurança). */
  readonly percentual = signal(90);
  /** #149 (D): quanto o usuário diz que consegue guardar este mês, por meta. Fica na URL (?guardar=id:valor). */
  readonly ajustes = signal<ReadonlyMap<number, number>>(new Map());
  readonly ajusteAberto = signal<number | null>(null);
  /** #318: o gráfico "Como está indo" aberto (uma meta por vez) e os aportes dela. */
  readonly evolucaoAberta = signal<number | null>(null);
  readonly aportesDaMeta = signal<{ metaId: number; aportes: MetaAporte[] } | null>(null);
  /** As janelas: a meta em edição (null = fechada) e a que recebe um aporte. */
  readonly formulario = signal<FormMeta | null>(null);
  readonly aporteMeta = signal<Meta | null>(null);

  /** Metas em aberto, na ordem de prioridade (a da API). */
  readonly ativas = computed(() => this.metas().filter((m) => !m.concluida));
  readonly calculado = computed(() => {
    const p = this.plano();
    return p?.disponivel ? planejarMetas(p, this.ativas(), this.percentual(), this.ajustes()) : null;
  });
  readonly mesDoPlano = computed(() => {
    const p = this.plano();
    return p ? { ano: p.ano, mes: p.mes } : null;
  });
  /** #359: o maior valor por mês que a janela da meta aceita. */
  readonly limitePorMes = computed(() =>
    limiteDoPorMes(this.plano(), this.calculado(), this.percentual(), this.ativas(), this.formulario()?.id ?? null));

  /** #318: o guardado mês a mês e o plano até o alvo da meta aberta (null enquanto os aportes carregam). */
  readonly evolucao = computed(() => {
    const id = this.evolucaoAberta();
    const dados = this.aportesDaMeta();
    const meta = this.metas().find((m) => m.id === id);
    if (!meta || !dados || dados.metaId !== id) return null;
    const { hoje, hojeDia } = hojeDaEvolucao(this.mesDoPlano(), new Date());
    return evolucaoDaMeta(meta, dados.aportes, this.planoDe(meta), hoje, hojeDia);
  });
  readonly resumoDaEvolucao = computed(() => {
    const ev = this.evolucao();
    const meta = this.metas().find((m) => m.id === this.evolucaoAberta());
    return ev && meta ? resumoEvolucao(meta, ev, this.planoDe(meta), this.privacidade.oculto()) : null;
  });
  readonly graficoEvolucao = computed(() => {
    const ev = this.evolucao();
    const meta = this.metas().find((m) => m.id === this.evolucaoAberta());
    return ev && meta ? graficoDaEvolucao(meta.cor, ev) : null;
  });

  ngOnInit(): void {
    this.ajustes.set(lerAjustes(this.rota.snapshot.queryParamMap.get('guardar')));
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
    // Cofrinhos são complementares: se falhar, o formulário fica como antes (sem o campo).
    this.api.reservas().subscribe({ next: (r) => this.reservas.set(r), error: () => this.reservas.set([]) });
  }

  carregar(): void {
    this.api.metas().subscribe({
      next: (m) => this.metas.set(m),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
    // O assistente é complementar: se falhar, os cartões mostram a divisão igual, como antes.
    this.api.planoMetas().subscribe({
      next: (p) => {
        this.percentual.set(100 - p.margemPercentual);
        this.plano.set(p);
      },
      error: () => this.plano.set(null)
    });
  }

  /** Recomendação da meta (null sem histórico suficiente: o cartão mostra a divisão igual). */
  planoDe(m: Meta): PlanoDaMeta | null {
    if (m.concluida) return null;
    return this.calculado()?.metas.find((x) => x.metaId === m.id) ?? null;
  }

  /* ---- sanfonas (lembradas neste navegador, ADR 0017) */

  metaAberta(id: number): boolean {
    return !this.metasFechadas().has(id);
  }

  alternarMeta(id: number): void {
    this.metasFechadas.update((s) => alternarNoConjunto(s, id));
    this.guardarFechadas();
  }

  /** "Fechar todas" com alguma aberta; "Abrir todas" com todas fechadas (Nielsen 3). */
  alternarTodasMetas(): void {
    this.metasFechadas.set(fechadasAoAlternarTodas(this.metas().map((m) => m.id), this.algumaMetaAberta()));
    this.guardarFechadas();
  }

  private guardarFechadas(): void {
    try { localStorage.setItem(MetasComponent.CHAVE_FECHADAS, JSON.stringify([...this.metasFechadas()])); } catch { /* só não lembra */ }
  }

  private static lerFechadas(): ReadonlySet<number> {
    try {
      return lerFechadas(localStorage.getItem(MetasComponent.CHAVE_FECHADAS));
    } catch {
      return new Set();
    }
  }

  /* ---- o assistente e a simulação do mês */

  /** #149 (C): recalcula na hora e grava como a margem de segurança (a mesma escolha, ao contrário). */
  mudarPercentual(valor: number): void {
    const anterior = this.percentual();
    this.percentual.set(valor);
    this.api.salvarParametros({ 'Metas.MargemSeguranca': 100 - valor }).subscribe({
      error: (e) => {
        this.percentual.set(anterior);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** #149 (D): o valor que o usuário consegue guardar este mês na meta; vazio volta à recomendação. */
  ajustar(id: number, valor: number | null): void {
    this.gravarAjustes(comAjuste(this.ajustes(), id, valor));
  }

  limparAjuste(id: number): void {
    this.gravarAjustes(semAjuste(this.ajustes(), id));
    this.ajusteAberto.set(null);
  }

  alternarAjuste(id: number): void {
    this.ajusteAberto.update((v) => (v === id ? null : id));
  }

  /** Ajustes na URL, sem poluir o histórico (replaceUrl): voltar e recarregar mantêm a simulação. */
  private gravarAjustes(ajustes: ReadonlyMap<number, number>): void {
    this.ajustes.set(ajustes);
    const guardar = ajustesParaUrl(ajustes);
    this.router.navigate([], { relativeTo: this.rota, queryParams: { guardar }, queryParamsHandling: 'merge', replaceUrl: true });
  }

  /* ---- prioridade e "Como está indo" */

  /** Sobe (-1) ou desce (+1) a meta uma posição e grava a nova ordem. */
  mover(m: Meta, direcao: -1 | 1): void {
    const ordem = ordemAposMover(this.ativas().map((x) => x.id), m.id, direcao);
    if (!ordem) return;
    this.api.ordenarMetas(ordem).subscribe({
      next: () => this.carregar(),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  alternarEvolucao(id: number): void {
    const abrir = this.evolucaoAberta() !== id;
    this.evolucaoAberta.set(abrir ? id : null);
    if (abrir) this.carregarAportes(id);
  }

  private carregarAportes(id: number): void {
    this.aportesDaMeta.set(null);
    this.api.aportesMeta(id).subscribe({
      next: (aportes) => this.aportesDaMeta.set({ metaId: id, aportes }),
      // Sem os aportes, o gráfico parte do total de hoje (como antes do histórico).
      error: () => this.aportesDaMeta.set({ metaId: id, aportes: [] })
    });
  }

  /* ---- criar, editar e excluir */

  nova(): void {
    this.formulario.set(formularioVazio());
  }

  editar(m: Meta): void {
    this.formulario.set(formularioDaMeta(m));
  }

  salvar(fm: FormMeta): void {
    const dto = dtoDaMeta(fm);
    const req = fm.id ? this.api.atualizarMeta(fm.id, dto) : this.api.criarMeta(dto);
    req.subscribe({
      next: () => {
        this.formulario.set(null);
        this.avisos.sucesso('Meta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** O "Excluir" da janela de edição. */
  excluir(): void {
    const id = this.formulario()?.id;
    if (!id) return;
    const meta = this.metas().find((m) => m.id === id);
    if (meta) void this.excluirMeta(meta);
  }

  /** #631: o Excluir do card e o do formulário de edição pedem a mesma confirmação. */
  async excluirMeta(m: Meta): Promise<void> {
    if (!(await this.confirmacao.confirmar({ titulo: 'Excluir meta', mensagem: `Excluir a meta “${m.nome}”?`, confirmar: 'Excluir', perigo: true }))) return;
    this.api.excluirMeta(m.id).subscribe({
      next: () => {
        this.formulario.set(null);
        this.avisos.sucesso('Meta excluída.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ---- aporte */

  abrirAporte(m: Meta): void {
    this.aporteMeta.set(m);
  }

  registrarAporte(valor: number): void {
    const meta = this.aporteMeta();
    if (!meta) return;
    this.api.aporteMeta(meta.id, valor).subscribe({
      next: () => {
        this.aporteMeta.set(null);
        this.avisos.sucesso('Aporte registrado.');
        this.carregar();
        // #318: o gráfico aberto ganha o ponto do aporte.
        if (this.evolucaoAberta() === meta.id) this.carregarAportes(meta.id);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
