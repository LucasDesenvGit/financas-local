import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { CampoDataComponent } from '../compartilhado/campo-data.component';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Conta, Meta, PlanoMeta, PlanoMetas, Reserva, SalvarMeta } from '../core/modelos';
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
        } @else {
          <!-- O valor à direita, em verde (sobra) ou vermelho (falta): a resposta do bloco se lê de longe (Nielsen 1). -->
          <h2 id="titulo-assistente" class="titulo-com-valor">
            <span>{{ nomeMes(p.mes) }}/{{ p.ano }} · <app-termo chave="sobraPrevista">sobra prevista</app-termo></span>
            <span class="valor-titulo" [class.positivo]="p.sobraPrevista >= 0" [class.negativo]="p.sobraPrevista < 0">
              {{ p.sobraPrevista | currency }}</span>
          </h2>
          @if (p.sobraNegativa) {
            <p class="negativo">A projeção deste mês fecha no vermelho: recomendo guardar R$ 0 agora.</p>
          } @else {
            <p>
              @if (p.totalPedido > 0) { Suas metas com prazo pedem {{ p.totalPedido | currency }} este mês. }
              Recomendo guardar <strong>{{ p.totalRecomendado | currency }}</strong>
              (a sobra menos {{ p.margemPercentual }}% de <app-termo chave="margemSeguranca">margem de segurança</app-termo>),
              dividido pela ordem de prioridade das metas.
            </p>
          }
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
            <p class="recomendado">Este mês: guarde <strong>{{ pm.recomendado | currency }}</strong></p>
            @if (pm.divisaoIgual) {
              <p class="texto-suave">Divisão igual seria {{ pm.divisaoIgual | currency }}/mês · faltam {{ pm.falta | currency }}</p>
            }
            @if (pm.noPrazo === false && prazoVencido(m)) {
              <p class="aviso-prazo" role="status">⚠ O prazo ({{ m.dataAlvo | date: 'MMM/yyyy' }}) já passou. Nesse ritmo você chega em
                {{ pm.dataProvavel ? (pm.dataProvavel | date: 'MMM/yyyy') : 'mais de 10 anos' }}; se quiser, edite a data da meta.</p>
            } @else if (pm.noPrazo === false) {
              <p class="aviso-prazo" role="status">⚠ Nesse ritmo você chega em {{ pm.dataProvavel ? (pm.dataProvavel | date: 'MMM/yyyy') : 'mais de 10 anos' }}
                (meta: {{ m.dataAlvo | date: 'MMM/yyyy' }}).
                @if (pm.paraManterPrazo) { Para manter o prazo, os próximos meses precisam de {{ pm.paraManterPrazo | currency }}. }</p>
            } @else if (pm.noPrazo === null) {
              <p class="texto-suave">Nesse ritmo você chega em {{ pm.dataProvavel ? (pm.dataProvavel | date: 'MMM/yyyy') : 'mais de 10 anos' }}.</p>
            }
            <button type="button" class="link botao-plano" [attr.aria-expanded]="planoAberto() === m.id"
              [attr.aria-controls]="'plano-meta-' + m.id" (click)="alternarPlano(m.id)">
              <span aria-hidden="true">{{ planoAberto() === m.id ? '▾' : '▸' }}</span> Plano mês a mês</button>
            <div [id]="'plano-meta-' + m.id">
              @if (planoAberto() === m.id) {
                <table class="tabela tabela-plano">
                  <thead><tr><th>Mês</th><th class="direita">Sobra prevista</th><th class="direita">Parcelas</th><th class="direita">Guardar</th></tr></thead>
                  <tbody>
                    @for (x of pm.plano; track x.ano * 100 + x.mes) {
                      <tr><td>{{ nomeMesCurto(x.mes) }}/{{ x.ano % 100 }}</td><td class="direita">{{ x.sobraPrevista | currency }}</td>
                        <td class="direita">{{ x.parcelas | currency }}</td><td class="direita"><strong>{{ x.recomendado | currency }}</strong></td></tr>
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
    .botao-plano { justify-self: start; padding-left: 0; }
    .tabela-plano { font-size: 0.8rem; }
    .tabela-plano th, .tabela-plano td { padding: 4px 6px; }
  `
})
export class MetasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly cores = CORES_SUGERIDAS;
  readonly metas = signal<Meta[]>([]);
  readonly contas = signal<Conta[]>([]);
  /** #90: plano do mês (quanto guardar em cada meta) e o plano mês a mês aberto. */
  readonly plano = signal<PlanoMetas | null>(null);
  readonly planoAberto = signal<number | null>(null);
  /** Metas em aberto, na ordem de prioridade (a da API). */
  readonly ativas = computed(() => this.metas().filter((m) => !m.concluida));
  /** #89: cofrinhos (reservas do extrato), para o campo da meta. */
  readonly reservas = signal<Reserva[]>([]);

  form: SalvarMeta | null = null;
  editandoId: number | null = null;
  aporteMeta: Meta | null = null;
  valorAporte: number | null = null;

  ngOnInit(): void {
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
    this.api.planoMetas().subscribe({ next: (p) => this.plano.set(p), error: () => this.plano.set(null) });
  }

  /** Recomendação da meta (null sem histórico suficiente: o cartão mostra a divisão igual). */
  planoDe(m: Meta): PlanoMeta | null {
    const p = this.plano();
    if (!p?.disponivel || m.concluida) return null;
    return p.metas.find((x) => x.metaId === m.id && x.recomendado !== null) ?? null;
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
