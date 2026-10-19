import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { CampoDataComponent } from '../compartilhado/campo-data.component';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Conta, Meta, PlanoMetas, Reserva, SalvarMeta } from '../core/modelos';
import { Degrau, PlanoDaMeta, planejarMetas } from './metas-plano';
import { TermoComponent } from '../compartilhado/termo.component';
import { NOMES_MESES } from '../compartilhado/utilidades';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { CORES_SUGERIDAS } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';

@Component({
  selector: 'app-metas',
  imports: [ListaSuspensaDirective, FormsModule, RouterLink, MoedaPipe, DatePipe, DecimalPipe, ModalComponent, TermoComponent, CampoValorComponent,
    CampoDataComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Metas</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova meta</button>
    </header>

    @if (plano(); as p) {
      <section class="painel-bloco assistente-metas" aria-labelledby="titulo-assistente">
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
          @if (c.sobraMes <= 0) {
            <p class="negativo">Pelo dinheiro que entra e sai, este mês fecha no vermelho: recomendo guardar R$ 0 agora.
              O plano compensa nos meses com mais folga.</p>
          } @else {
            <p>Recomendo guardar <strong>{{ c.totalRecomendado | currency }}</strong> este mês, dividido pela ordem de prioridade das metas.</p>
            @switch (c.limitadoPor) {
              @case ('saldo') {
                <p class="texto-suave">Limitei ao que deve sobrar nas suas contas no fim do mês, depois de pagar a fatura
                  ({{ p.saldoFimDoMes | currency }}).</p>
              }
              @case ('historico') {
                <p class="texto-suave">Limitei ao que de fato sobrou no seu melhor mês recente ({{ p.melhorMesRecente | currency }}).</p>
              }
              @case ('minimo') {
                <p class="texto-suave">É pouco, mas é um começo: o plano compensa nos meses com mais folga.</p>
              }
            }
          }
          <!-- #149 (C): quanto da sobra vai para as metas, na própria tela; muda na hora (Nielsen 3 e 6). -->
          <label class="parte-sobra">Guardar
            <select name="percentual" [ngModel]="percentual()" (ngModelChange)="mudarPercentual($event)">
              @for (v of opcoesPercentual; track v) { <option [ngValue]="v">{{ v }}%</option> }
            </select>
            da sobra para as metas <app-termo chave="parteDaSobra"></app-termo>
          </label>
          <p class="links-assistente">
            <app-termo chave="recomendacaoMetas">Como calculei</app-termo> · <a routerLink="/projecao">Ver projeção →</a>
          </p>
        }
      </section>
    }

    <section class="grade-cartoes">
      @for (m of metas(); track m.id) {
        <article class="cartao-meta" [class.concluida]="m.concluida">
          <header>
            <strong>{{ m.nome }}</strong>
            @if (m.concluida) { <span class="etiqueta etiqueta-sucesso">concluída</span> }
          </header>
          <div class="anel" [style.--pct]="m.percentual" [style.--cor]="m.cor">
            <span>{{ m.percentual | number: '1.0-0' }}%</span>
          </div>
          <p><strong>{{ m.valorAtual | currency }}</strong> <span class="texto-suave">de {{ m.valorAlvo | currency }}</span></p>
          @if (m.dataAlvo) { <p class="texto-suave">até {{ m.dataAlvo | date: 'MMM/yyyy' }}</p> }
          @if (!m.concluida && posicao(m) > 0) {
            <div class="linha-prioridade">
              <span class="etiqueta etiqueta-suave">{{ posicao(m) }}ª prioridade</span>
              <button type="button" class="botao-icone" (click)="mover(m, -1)" [disabled]="posicao(m) === 1"
                [attr.aria-label]="'Subir a prioridade de ' + m.nome">↑</button>
              <button type="button" class="botao-icone" (click)="mover(m, 1)" [disabled]="posicao(m) === ativas().length"
                [attr.aria-label]="'Descer a prioridade de ' + m.nome">↓</button>
            </div>
          }
          @if (planoDe(m); as pm) {
            <p class="recomendado">Este mês: guarde <strong>{{ pm.recomendado | currency }}</strong>@if (pm.ajustado) {<span
              class="texto-suave"> (o valor que você escolheu)</span>}</p>
            <!-- #149 (D): o usuário diz quanto consegue; o resto do plano é refeito na hora. Simulação até registrar o aporte. -->
            @if (!pm.ajustado) {
              <button type="button" class="link botao-ajuste" [attr.aria-expanded]="ajusteAberto() === m.id"
                [attr.aria-controls]="'ajuste-meta-' + m.id" (click)="alternarAjuste(m.id)">Consigo guardar outro valor</button>
            }
            <div [id]="'ajuste-meta-' + m.id">
              @if (ajusteAberto() === m.id || pm.ajustado) {
                <div class="ajuste-mes">
                  <label>Quanto consigo guardar este mês
                    <app-campo-valor [name]="'ajuste-' + m.id" [ngModel]="ajustes().get(m.id) ?? null"
                      (ngModelChange)="ajustar(m.id, $event)" />
                  </label>
                  @if (pm.ajustado) {
                    <button type="button" class="link" (click)="limparAjuste(m.id)">Voltar à recomendação</button>
                  }
                </div>
              }
            </div>
            @if (pm.degraus.length) {
              <!-- #149 (B): o plano em degraus, com o motivo (as faturas), em vez de um valor por mês (Nielsen 2 e 10). -->
              <div class="degraus">
                <span class="texto-suave">Depois:</span>
                <ul>
                  @for (d of pm.degraus.slice(0, 4); track d.inicio.ano * 100 + d.inicio.mes) {
                    <li><strong>{{ d.valor > 0 ? (d.valor | currency) + (mesmoMes(d) ? '' : '/mês') : 'nada' }}</strong>
                      {{ mesmoMes(d) ? 'em ' + rotuloMes(d.inicio) : 'de ' + rotuloMes(d.inicio) + ' a ' + rotuloMes(d.fim) }}
                      <span class="texto-suave">(faturas ~{{ d.faturas | currency: 'BRL' : 'symbol' : '1.0-0' }})</span></li>
                  }
                  @if (pm.degraus.length > 4) { <li class="texto-suave">… veja o plano mês a mês</li> }
                </ul>
              </div>
            }
            @if (m.aporteMensalSugerido) {
              <p class="texto-suave">Divisão igual seria {{ m.aporteMensalSugerido | currency }}/mês · faltam {{ pm.falta | currency }}</p>
            }
            @if (pm.noPrazo === false && prazoVencido(m)) {
              <p class="aviso-prazo" role="status">⚠ O prazo ({{ m.dataAlvo | date: 'MMM/yyyy' }}) já passou. Nesse ritmo você chega em
                {{ pm.dataProvavel ? (pm.dataProvavel | date: 'MMM/yyyy') : 'mais de 10 anos' }}; se quiser, edite a data da meta.</p>
            } @else if (pm.noPrazo === false) {
              <p class="aviso-prazo" role="status">⚠ Nesse ritmo você chega em {{ pm.dataProvavel ? (pm.dataProvavel | date: 'MMM/yyyy') : 'mais de 10 anos' }}
                (meta: {{ m.dataAlvo | date: 'MMM/yyyy' }}).
                @if (pm.paraManterPrazo) { Para manter o prazo, os próximos meses precisam de {{ pm.paraManterPrazo | currency }}. }
                <span class="saidas-prazo">
                  <button type="button" class="link" (click)="editar(m)">Mudar o prazo ou o valor</button> ·
                  <a routerLink="/anual">Ver onde cortar (50/30/20)</a> ·
                  <a routerLink="/orcamentos">Orçamentos</a>
                </span></p>
            } @else if (pm.noPrazo === null) {
              <p class="texto-suave">Nesse ritmo você chega em {{ pm.dataProvavel ? (pm.dataProvavel | date: 'MMM/yyyy') : 'mais de 10 anos' }}.</p>
            }
            @if (pm.dependeDaFatura; as dep) {
              <p class="nota-plano texto-suave">Este plano conta com as faturas caindo para ~{{ dep.depois | currency }} a partir de
                {{ nomeMesCurto(dep.aPartir.mes) }}/{{ dep.aPartir.ano % 100 }} (hoje ~{{ dep.antes | currency }}). Se entrar
                parcelamento novo, ele atrasa; o assistente refaz a conta todo mês.</p>
            }
            <button type="button" class="link botao-plano" [attr.aria-expanded]="planoAberto() === m.id"
              [attr.aria-controls]="'plano-meta-' + m.id" (click)="alternarPlano(m.id)">
              <span aria-hidden="true">{{ planoAberto() === m.id ? '▾' : '▸' }}</span> Plano mês a mês</button>
            <div [id]="'plano-meta-' + m.id">
              @if (planoAberto() === m.id) {
                <table class="tabela tabela-plano">
                  <thead><tr><th>Mês</th><th class="direita">Sobra</th><th class="direita">Faturas</th><th class="direita">Guardar</th></tr></thead>
                  <tbody>
                    @for (x of pm.meses; track x.ano * 100 + x.mes) {
                      <tr><td>{{ nomeMesCurto(x.mes) }}/{{ x.ano % 100 }}</td><td class="direita">{{ x.sobra | currency }}</td>
                        <td class="direita">{{ x.faturas | currency }}</td><td class="direita"><strong>{{ x.guardar | currency }}</strong></td></tr>
                    }
                  </tbody>
                </table>
              }
            </div>
          } @else if (m.aporteMensalSugerido) {
            <p class="texto-suave">Guardar <strong>{{ m.aporteMensalSugerido | currency }}</strong>/mês para chegar lá</p>
          }
          @if (m.reservaNome) {
            @if (m.reservaNaoEncontrada) {
              <p class="aviso-cofrinho" role="status">Cofrinho "{{ m.reservaNome }}" não encontrado no extrato de {{ m.contaNome }}.
                <button type="button" class="link" (click)="editar(m)">Editar a meta</button></p>
            } @else {
              <p class="texto-suave">{{ m.valorInicial | currency }} guardados + <strong>{{ m.saldoReserva | currency }}</strong>
                no cofrinho {{ m.reservaNome }} ({{ m.contaNome }})</p>
              @if (m.reservaConferir) {
                <p class="texto-suave">saldo estimado, <a routerLink="/investimentos" [queryParams]="{ aba: 'reservas' }">confira em Reservas</a></p>
              }
            }
          } @else if (m.contaNome) {
            <p class="texto-suave">{{ m.valorInicial | currency }} guardados + {{ m.saldoConta | currency }} em {{ m.contaNome }}</p>
          }
          <footer>
            @if (!m.concluida) {
              <button type="button" class="link" (click)="abrirAporte(m)">+ aporte</button>
            }
            <span class="cresce"></span>
            <button type="button" class="link" (click)="editar(m)">editar</button>
          </footer>
        </article>
      } @empty {
        <p class="vazio">Crie metas como reserva de emergência, viagem ou entrada de um imóvel.</p>
      }
    </section>

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar meta' : 'Nova meta'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fm.nome" required maxlength="120"></label>
          <div class="grade-campos">
            <label>Valor alvo <app-campo-valor name="alvo" [(ngModel)]="fm.valorAlvo" obrigatorio [minimo]="0.01" /></label>
            <label>Data alvo <app-campo-data name="data" [(ngModel)]="fm.dataAlvo" /></label>
          </div>
          <label>Acompanhar saldo de uma conta (opcional)
            <select name="conta" [(ngModel)]="fm.contaId" (ngModelChange)="fm.reservaNome = null">
              <option [ngValue]="null">Não — vou registrar os aportes</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <label>Valor inicial já guardado
            <app-campo-valor name="atual" [(ngModel)]="fm.valorAtual" />
          </label>
          @if (opcoesCofrinho(fm).length) {
            <label>Qual cofrinho (reserva)?
              <select name="reserva" [(ngModel)]="fm.reservaNome">
                <option [ngValue]="null">Saldo disponível da conta (sem cofrinhos)</option>
                @for (r of opcoesCofrinho(fm); track r.nome) {
                  <option [ngValue]="r.nome">{{ r.nome }}{{ r.saldo === null ? ' (não encontrado no extrato)' : ' — ' + (r.saldo | currency) }}</option>
                }
              </select>
            </label>
            <p class="texto-suave">O saldo do cofrinho é estimado pelo extrato.
              <a routerLink="/investimentos" [queryParams]="{ aba: 'reservas' }" (click)="form = null">Confira em Investimentos › Reservas</a>.</p>
          }
          @if (fm.contaId !== null) {
            <p class="texto-suave">O progresso será o valor inicial somado ao saldo {{ fm.reservaNome ? 'do cofrinho' : 'atual da conta' }} escolhido.</p>
          }
          <div class="cores">
            @for (cor of cores; track cor) {
              <button type="button" class="cor" [style.background]="cor" [class.selecionada]="fm.cor === cor"
                (click)="fm.cor = cor" [attr.aria-label]="'Cor ' + cor"></button>
            }
          </div>
          <label class="checkbox"><input type="checkbox" name="concluida" [(ngModel)]="fm.concluida"> Concluída</label>
          <div class="rodape-form">
            @if (editandoId) { <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="aporteMeta !== null" [titulo]="'Aporte · ' + (aporteMeta?.nome ?? '')" (fechar)="aporteMeta = null">
      <form (ngSubmit)="salvarAporte()" class="formulario">
        <label>Valor — use negativo para retirada
          <app-campo-valor name="aporte" [(ngModel)]="valorAporte" obrigatorio negativo />
        </label>
        <div class="rodape-form">
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="aporteMeta = null">Cancelar</button>
          <button class="botao botao-primario">Registrar</button>
        </div>
      </form>
    </app-modal>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    .aviso-cofrinho { margin: 0; padding: 6px 10px; border-radius: var(--raio-pequeno); background: var(--alerta-suave); font-size: 0.85rem; }
    .assistente-metas h2 { margin: 0 0 8px; }
    .titulo-com-valor { display: flex; justify-content: space-between; align-items: baseline; gap: 6px 16px; flex-wrap: wrap; }
    .valor-titulo { font-family: var(--fonte-titulo); font-size: 1.25rem; font-weight: 650; font-variant-numeric: tabular-nums; }
    .assistente-metas p { margin: 0 0 8px; }
    .assistente-metas .links-assistente { margin: 0; }
    .linha-prioridade { display: flex; align-items: center; gap: 6px; }
    .linha-prioridade .botao-icone { width: 28px; height: 28px; }
    .recomendado { margin: 0; }
    .aviso-prazo { margin: 0; padding: 6px 10px; border-radius: var(--raio-pequeno); background: var(--alerta-suave); font-size: 0.85rem; }
    .botao-plano, .botao-ajuste { justify-self: start; padding-left: 0; }
    .conta-sobra { font-size: 0.85rem; }
    .parte-sobra { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; margin: 0 0 8px; font-weight: 400; }
    .parte-sobra select { width: auto; }
    .ajuste-mes { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; }
    .ajuste-mes label { flex: 1 1 160px; font-size: 0.85rem; }
    .degraus { font-size: 0.85rem; }
    .degraus ul { margin: 2px 0 0; padding-left: 18px; display: grid; gap: 2px; }
    .saidas-prazo { display: block; margin-top: 4px; }
    .nota-plano { margin: 0; font-size: 0.8rem; }
    .tabela-plano { font-size: 0.8rem; }
    .tabela-plano th, .tabela-plano td { padding: 4px 6px; }
  `
})
export class MetasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly router = inject(Router);
  private readonly rota = inject(ActivatedRoute);

  readonly cores = CORES_SUGERIDAS;
  /** Guardar de 50% a 100% da sobra (a margem de segurança vai de 0% a 50%, ParametroService). */
  readonly opcoesPercentual = [50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100];
  readonly metas = signal<Meta[]>([]);
  readonly contas = signal<Conta[]>([]);
  /** #90/#149: o caixa que vem da API; o plano das metas é calculado aqui (metas-plano.ts). */
  readonly plano = signal<PlanoMetas | null>(null);
  readonly planoAberto = signal<number | null>(null);
  /** #149 (C): quanto da sobra vai para as metas (100 − margem de segurança). */
  readonly percentual = signal(90);
  /** #149 (D): quanto o usuário diz que consegue guardar este mês, por meta. Fica na URL (?guardar=id:valor). */
  readonly ajustes = signal<ReadonlyMap<number, number>>(new Map());
  readonly ajusteAberto = signal<number | null>(null);
  /** Metas em aberto, na ordem de prioridade (a da API). */
  readonly ativas = computed(() => this.metas().filter((m) => !m.concluida));
  readonly calculado = computed(() => {
    const p = this.plano();
    return p?.disponivel ? planejarMetas(p, this.ativas(), this.percentual(), this.ajustes()) : null;
  });
  /** #89: cofrinhos (reservas do extrato), para o campo da meta. */
  readonly reservas = signal<Reserva[]>([]);

  form: SalvarMeta | null = null;
  editandoId: number | null = null;
  aporteMeta: Meta | null = null;
  valorAporte: number | null = null;

  ngOnInit(): void {
    this.ajustes.set(lerAjustes(this.rota.snapshot.queryParamMap.get('guardar')));
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
    // Cofrinhos são complementares: se falhar, o formulário fica como antes (sem o campo).
    this.api.reservas().subscribe({ next: (r) => this.reservas.set(r), error: () => this.reservas.set([]) });
  }

  /**
   * Cofrinhos da conta escolhida, com saldo (Nielsen 6: escolher, não digitar). O campo só aparece quando a conta tem
   * cofrinho (divulgação progressiva). Se a meta aponta para um que sumiu do extrato, ele continua na lista (marcado),
   * para não ser trocado sem querer.
   */
  opcoesCofrinho(fm: SalvarMeta): { nome: string; saldo: number | null }[] {
    if (fm.contaId === null) return [];
    const opcoes = this.reservas().filter((r) => r.contaId === fm.contaId)
      .map((r) => ({ nome: r.nome, saldo: r.saldo as number | null }))
      .sort((a, b) => a.nome.localeCompare(b.nome, 'pt-BR'));
    const escolhido = fm.reservaNome;
    if (escolhido && !opcoes.some((o) => o.nome.toLowerCase() === escolhido.toLowerCase())) opcoes.push({ nome: escolhido, saldo: null });
    return opcoes;
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
    const novo = new Map(this.ajustes());
    if (valor === null || valor === undefined || !Number.isFinite(valor) || valor < 0) novo.delete(id);
    else novo.set(id, valor);
    this.gravarAjustes(novo);
  }

  limparAjuste(id: number): void {
    const novo = new Map(this.ajustes());
    novo.delete(id);
    this.gravarAjustes(novo);
    this.ajusteAberto.set(null);
  }

  alternarAjuste(id: number): void {
    this.ajusteAberto.update((v) => (v === id ? null : id));
  }

  /** Ajustes na URL, sem poluir o histórico (replaceUrl): voltar e recarregar mantêm a simulação. */
  private gravarAjustes(ajustes: ReadonlyMap<number, number>): void {
    this.ajustes.set(ajustes);
    const guardar = [...ajustes].map(([id, v]) => `${id}:${v}`).join(',') || null;
    this.router.navigate([], { relativeTo: this.rota, queryParams: { guardar }, queryParamsHandling: 'merge', replaceUrl: true });
  }

  /** Degrau de um mês só ("R$ 300 em set/27") em vez de um intervalo ("R$ 50/mês de nov/26 a dez/26"). */
  mesmoMes(d: Degrau): boolean {
    return d.inicio.ano === d.fim.ano && d.inicio.mes === d.fim.mes;
  }

  rotuloMes(x: { ano: number; mes: number }): string {
    return `${this.nomeMesCurto(x.mes)}/${x.ano % 100}`;
  }

  /** Prazo antes do mês do plano: não há mais "manter o prazo", só a data provável. */
  prazoVencido(m: Meta): boolean {
    const p = this.plano();
    if (!p || !m.dataAlvo) return false;
    const [ano, mes] = m.dataAlvo.split('-').map(Number);
    return ano * 12 + mes < p.ano * 12 + p.mes;
  }

  /** Posição na ordem de prioridade (1 = primeira) entre as metas em aberto. */
  posicao(m: Meta): number {
    return this.ativas().findIndex((x) => x.id === m.id) + 1;
  }

  /** Sobe (-1) ou desce (+1) a meta uma posição e grava a nova ordem. */
  mover(m: Meta, direcao: -1 | 1): void {
    const ordem = this.ativas().map((x) => x.id);
    const i = ordem.indexOf(m.id);
    const j = i + direcao;
    if (i < 0 || j < 0 || j >= ordem.length) return;
    [ordem[i], ordem[j]] = [ordem[j], ordem[i]];
    this.api.ordenarMetas(ordem).subscribe({
      next: () => this.carregar(),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  alternarPlano(id: number): void {
    this.planoAberto.update((v) => (v === id ? null : id));
  }

  nomeMes(mes: number): string {
    const n = NOMES_MESES[mes - 1];
    return n.charAt(0).toUpperCase() + n.slice(1);
  }

  nomeMesCurto(mes: number): string {
    return NOMES_MESES[mes - 1].slice(0, 3);
  }

  nova(): void {
    this.editandoId = null;
    this.form = { nome: '', valorAlvo: 0, valorAtual: 0, dataAlvo: null, contaId: null, cor: CORES_SUGERIDAS[0], concluida: false, reservaNome: null };
  }

  editar(m: Meta): void {
    this.editandoId = m.id;
    this.form = {
      nome: m.nome, valorAlvo: m.valorAlvo, valorAtual: m.valorInicial, dataAlvo: m.dataAlvo,
      contaId: m.contaId, cor: m.cor, concluida: m.concluida, reservaNome: m.reservaNome
    };
  }

  salvar(): void {
    if (!this.form) return;
    const dto: SalvarMeta = {
      ...this.form,
      valorAlvo: Number(this.form.valorAlvo),
      valorAtual: Number(this.form.valorAtual) || 0,
      dataAlvo: this.form.dataAlvo || null
    };
    const req = this.editandoId ? this.api.atualizarMeta(this.editandoId, dto) : this.api.criarMeta(dto);
    req.subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Meta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluir(): void {
    if (!this.editandoId || !confirm('Excluir esta meta?')) return;
    this.api.excluirMeta(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirAporte(m: Meta): void {
    this.aporteMeta = m;
    this.valorAporte = null;
  }

  salvarAporte(): void {
    if (!this.aporteMeta || !this.valorAporte) return;
    this.api.aporteMeta(this.aporteMeta.id, Number(this.valorAporte)).subscribe({
      next: () => {
        this.aporteMeta = null;
        this.avisos.sucesso('Aporte registrado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}

/** Lê "?guardar=1:50,3:120" (meta:valor); o que não for número válido e ≥ 0 é ignorado. */
export function lerAjustes(texto: string | null): ReadonlyMap<number, number> {
  const ajustes = new Map<number, number>();
  for (const par of (texto ?? '').split(',')) {
    const [id, valor] = par.split(':').map(Number);
    if (Number.isInteger(id) && id > 0 && Number.isFinite(valor) && valor >= 0) ajustes.set(id, valor);
  }
  return ajustes;
}
