import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  Categoria, Conta, FiltroTransacoes, OpcaoCategoria, OrigemConta, PaginaTransacoes, StatusTransacao, SugestaoCategoria,
  TipoCorrespondencia, Transacao, TipoTransacao
} from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { PaginacaoComponent } from '../compartilhado/paginacao.component';
import { dataIso, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

interface FormTransacao {
  id: number | null;
  openFinance: boolean;
  contaId: number | null;
  data: string;
  descricao: string;
  valor: number | null;
  tipo: TipoTransacao;
  categoriaId: number | null;
  observacao: string;
  ignorarRelatorios: boolean;
  pendente: boolean;
  descricaoOriginal: string | null;
  emergencia: boolean;
}

/** Linha editável do assistente de sugestões. */
interface LinhaSugestao extends SugestaoCategoria { escolhida: number | null; criarRegra: boolean; aplicada: boolean }

interface FormTransferencia {
  contaOrigemId: number | null;
  contaDestinoId: number | null;
  valor: number | null;
  data: string;
  descricao: string;
}

@Component({
  selector: 'app-transacoes',
  imports: [FormsModule, MoedaPipe, DatePipe, ModalComponent, PaginacaoComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Transações</h1>
      <div class="acoes">
        <button type="button" class="botao" (click)="abrirSugestoes()">✦ Sugestões de categoria</button>
        <button type="button" class="botao" (click)="abrirTransferencia()">⇄ Transferência</button>
        <button type="button" class="botao botao-primario" (click)="novo()">+ Lançamento</button>
      </div>
    </header>

    <section class="filtros">
      <label>De <input type="date" [(ngModel)]="filtro.inicio" (change)="buscar(1)"></label>
      <label>Até <input type="date" [(ngModel)]="filtro.fim" (change)="buscar(1)"></label>
      <label>Conta
        <select [(ngModel)]="filtro.contaId" (change)="buscar(1)">
          <option [ngValue]="null">Todas</option>
          @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
        </select>
      </label>
      <label>Categoria
        <select [(ngModel)]="filtroCategoria" (change)="aplicarFiltroCategoria()">
          <option value="">Todas</option>
          <option value="sem">— Sem categoria —</option>
          @for (c of categorias(); track c.id) { <option [value]="c.id">{{ rotuloCategoria(c) }}</option> }
        </select>
      </label>
      <label>Tipo
        <select [(ngModel)]="filtro.tipo" (change)="buscar(1)">
          <option [ngValue]="null">Todos</option>
          <option [ngValue]="TipoTransacao.Despesa">Despesas</option>
          <option [ngValue]="TipoTransacao.Receita">Receitas</option>
          <option [ngValue]="TipoTransacao.Transferencia">Transferências</option>
        </select>
      </label>
      <label class="checkbox"><input type="checkbox" [(ngModel)]="filtro.somenteEmergencias" (change)="buscar(1)">
        <span class="texto-emergencia">Só emergências</span></label>
      <label class="cresce">Buscar
        <input type="search" [(ngModel)]="filtro.busca" (keyup.enter)="buscar(1)" placeholder="descrição, estabelecimento…" maxlength="100">
      </label>
    </section>

    @if (pagina(); as p) {
      <section class="resumo-filtro">
        <span>{{ p.total }} transação(ões)</span>
        <span class="positivo">Receitas {{ p.totalReceitas | currency }}</span>
        <span class="negativo">Despesas {{ p.totalDespesas | currency }}</span>
        <span>Resultado <strong>{{ p.totalReceitas - p.totalDespesas | currency }}</strong></span>
      </section>

      @if (selecionadas().size) {
        <section class="barra-lote">
          <span>{{ selecionadas().size }} selecionada(s)</span>
          <select [(ngModel)]="categoriaLote">
            <option [ngValue]="null">Categorização automática</option>
            @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
          </select>
          <button type="button" class="botao botao-primario" (click)="categorizarLote()">Aplicar categoria</button>
          <button type="button" class="botao botao-emergencia" (click)="marcarEmergenciaLote(true)">Marcar emergência</button>
          <button type="button" class="botao" (click)="marcarEmergenciaLote(false)">Desmarcar emergência</button>
          <button type="button" class="link" (click)="limparSelecao()">limpar seleção</button>
        </section>
      }

      <div class="tabela-rolagem">
        <table class="tabela">
          <thead>
            <tr>
              <th class="col-check"><input type="checkbox" [checked]="todasSelecionadas()" (change)="alternarTodas()" aria-label="Selecionar todas"></th>
              <th>Data</th>
              <th>Descrição</th>
              <th>Categoria</th>
              <th>Conta</th>
              <th class="direita">Valor</th>
            </tr>
          </thead>
          <tbody>
            @for (t of p.itens; track t.id) {
              <tr (click)="editar(t)" class="clicavel" [class.ignorada]="t.ignorarRelatorios">
                <td class="col-check" (click)="$event.stopPropagation()">
                  <input type="checkbox" [checked]="selecionadas().has(t.id)" (change)="alternar(t.id)" aria-label="Selecionar">
                </td>
                <td class="nowrap">{{ t.data | date: 'dd/MM/yy' }}</td>
                <td>
                  <div class="truncar" [title]="t.descricao">{{ t.descricao }}</div>
                  <div class="detalhe">
                    @if (t.parcelaTotal && t.parcelaTotal > 1) { <span class="etiqueta">{{ t.parcelaNumero }}/{{ t.parcelaTotal }}</span> }
                    @if (t.status === StatusTransacao.Pendente) { <span class="etiqueta etiqueta-alerta">pendente</span> }
                    @if (t.emergencia) { <span class="etiqueta etiqueta-emergencia">emergência</span> }
                    @if (t.openFinance) { <span class="etiqueta">open finance</span> }
                    @if (t.observacao) { <span class="texto-suave truncar">{{ t.observacao }}</span> }
                  </div>
                </td>
                <td>
                  @if (t.categoriaNome) {
                    <span class="chip"><span class="ponto" [style.background]="t.categoriaCor"></span>{{ t.categoriaNome }}</span>
                  } @else { <span class="texto-suave">—</span> }
                </td>
                <td class="texto-suave">{{ t.contaNome }}</td>
                <td class="direita valor nowrap" [class.positivo]="t.valor > 0" [class.negativo]="t.valor < 0">{{ t.valor | currency }}</td>
              </tr>
            } @empty {
              <tr><td colspan="6" class="vazio">Nenhuma transação encontrada para os filtros.</td></tr>
            }
          </tbody>
        </table>
      </div>

      <app-paginacao [pagina]="p.pagina" [tamanho]="p.tamanhoPagina" [total]="p.total"
        (paginaMudou)="buscar($event)" (tamanhoMudou)="mudarTamanhoPagina($event)" />
    }

    <app-modal [aberto]="form !== null" [titulo]="form?.id ? 'Editar transação' : 'Novo lançamento'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          @if (fm.openFinance) {
            <p class="aviso-inline">Transação importada do Open Finance: valor, data e conta vêm do banco.
              @if (fm.descricaoOriginal) { <br><small>Original: {{ fm.descricaoOriginal }}</small> }
            </p>
          } @else {
            <div class="grade-campos">
              <label>Tipo
                <select name="tipo" [(ngModel)]="fm.tipo">
                  <option [ngValue]="TipoTransacao.Despesa">Despesa</option>
                  <option [ngValue]="TipoTransacao.Receita">Receita</option>
                </select>
              </label>
              <label>Conta
                <select name="conta" [(ngModel)]="fm.contaId" required>
                  @for (c of contasManuais(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
                </select>
              </label>
              <label>Data <input type="date" name="data" [(ngModel)]="fm.data" required></label>
              <label>Valor (R$) <input type="number" name="valor" [(ngModel)]="fm.valor" required min="0.01" step="0.01"></label>
            </div>
            @if (!contasManuais().length) {
              <p class="erro">Crie uma conta manual (ex.: "Carteira") em Contas e cartões para lançar manualmente.</p>
            }
          }
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="300"></label>
          <label>Categoria
            <select name="categoria" [(ngModel)]="fm.categoriaId">
              <option [ngValue]="null">Automática (regras)</option>
              @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
            </select>
          </label>
          @if (sugestoesForm().length) {
            <div class="sugestoes-chips">
              <span class="texto-suave">Sugestões:</span>
              @for (o of sugestoesForm(); track o.categoriaId) {
                <button type="button" class="chip-sugestao" [title]="o.motivo" (click)="fm.categoriaId = o.categoriaId">
                  <span class="ponto" [style.background]="o.cor"></span>{{ o.nome }}
                </button>
              }
            </div>
          }
          <label>Observação <textarea name="observacao" [(ngModel)]="fm.observacao" rows="2" maxlength="1000"></textarea></label>
          <label class="checkbox"><input type="checkbox" name="ignorar" [(ngModel)]="fm.ignorarRelatorios"> Ignorar nos relatórios e totais</label>
          @if (fm.tipo === TipoTransacao.Despesa) {
            <label class="checkbox"><input type="checkbox" name="emergencia" [(ngModel)]="fm.emergencia">
              <span class="texto-emergencia">Gasto de emergência</span>
              <small class="texto-suave">(destacado em laranja nos orçamentos e relatórios)</small></label>
          }
          @if (!fm.openFinance) {
            <label class="checkbox"><input type="checkbox" name="pendente" [(ngModel)]="fm.pendente"> Pendente (ainda não efetivada)</label>
          }
          <div class="rodape-form">
            @if (fm.id && !fm.openFinance) {
              <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="sugestoesAbertas()" titulo="Sugestões de categoria" (fechar)="fecharSugestoes()">
      <div class="formulario">
        <div class="linha-sugestoes">
          <div class="segmentado" role="group" aria-label="O que revisar">
            <button type="button" [class.ativo]="somenteSemCategoria()" (click)="trocarModoSugestoes(true)">Sem categoria</button>
            <button type="button" [class.ativo]="!somenteSemCategoria()" (click)="trocarModoSugestoes(false)">Revisar automáticas</button>
          </div>
          <span class="cresce"></span>
          <button type="button" class="botao botao-suave" (click)="aplicarConfiancaAlta()" [disabled]="!quantidadeAlta() || salvando()">
            Aplicar as {{ quantidadeAlta() }} de confiança alta</button>
        </div>
        <p class="texto-suave">
          Transações parecidas foram agrupadas pela descrição. A sugestão vem de como você categorizou transações iguais
          antes (suas escolhas manuais pesam mais) e de palavras-chave conhecidas. Marque "criar regra" para que as próximas
          sejam categorizadas sozinhas.
        </p>
        @if (carregandoSugestoes()) {
          <p class="vazio">Analisando suas transações…</p>
        } @else {
          <div class="tabela-rolagem">
            <table class="tabela tabela-sugestoes">
              <thead>
                <tr><th>Transações</th><th class="direita">Qtd.</th><th class="direita">Total</th><th>Sugestão</th>
                  <th>Categoria</th><th>Regra</th><th></th></tr>
              </thead>
              <tbody>
                @for (l of linhasSugestao(); track l.chave) {
                  <tr [class.encerrada]="l.aplicada">
                    <td><div class="truncar" [title]="l.exemplo">{{ l.exemplo }}</div>
                      <small class="texto-suave">{{ l.chave }}</small></td>
                    <td class="direita">{{ l.quantidade }}</td>
                    <td class="direita nowrap" [class.negativo]="l.total < 0" [class.positivo]="l.total > 0">{{ l.total | currency }}</td>
                    <td>
                      <span [class]="'confianca confianca-' + l.confianca">{{ nomeConfianca(l.confianca) }}</span>
                      @if (l.motivo) { <br><small class="texto-suave">{{ l.motivo }}</small> }
                      @if (l.alternativas.length) {
                        <div class="sugestoes-chips">
                          @for (a of l.alternativas; track a.categoriaId) {
                            <button type="button" class="chip-sugestao" [title]="a.motivo" (click)="l.escolhida = a.categoriaId">
                              <span class="ponto" [style.background]="a.cor"></span>{{ a.nome }}</button>
                          }
                        </div>
                      }
                    </td>
                    <td>
                      <select [(ngModel)]="l.escolhida" [disabled]="l.aplicada" [attr.aria-label]="'Categoria para ' + l.exemplo">
                        <option [ngValue]="null">— escolher —</option>
                        @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
                      </select>
                    </td>
                    <td><label class="checkbox"><input type="checkbox" [(ngModel)]="l.criarRegra" [disabled]="l.aplicada">
                      <small>contém "{{ l.textoRegra }}"</small></label></td>
                    <td>
                      @if (l.aplicada) {
                        <span class="etiqueta etiqueta-sucesso">aplicada</span>
                      } @else {
                        <button type="button" class="botao botao-primario botao-pequeno" [disabled]="!l.escolhida || salvando()"
                          (click)="aplicarSugestao(l)">Aplicar</button>
                      }
                    </td>
                  </tr>
                } @empty {
                  <tr><td colspan="7" class="vazio">Nada para sugerir.</td></tr>
                }
              </tbody>
            </table>
          </div>
        }
      </div>
    </app-modal>

    <app-modal [aberto]="transferencia !== null" titulo="Transferência entre contas" (fechar)="transferencia = null">
      @if (transferencia; as tf) {
        <form (ngSubmit)="salvarTransferencia()" #ft="ngForm" class="formulario">
          <p class="texto-suave">Os lançamentos são criados apenas nas contas manuais. O lado do Open Finance chega pela sincronização.</p>
          <div class="grade-campos">
            <label>De
              <select name="origem" [(ngModel)]="tf.contaOrigemId" required>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Para
              <select name="destino" [(ngModel)]="tf.contaDestinoId" required>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Data <input type="date" name="data" [(ngModel)]="tf.data" required></label>
            <label>Valor (R$) <input type="number" name="valor" [(ngModel)]="tf.valor" required min="0.01" step="0.01"></label>
          </div>
          <label>Descrição <input name="descricao" [(ngModel)]="tf.descricao" maxlength="200" placeholder="Transferência"></label>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="transferencia = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="ft.invalid || salvando()">Transferir</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  styles: `
    .linha-sugestoes { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .tabela-sugestoes td { vertical-align: top; }
    .tabela-sugestoes td:first-child { max-width: 280px; }
    .tabela-sugestoes select { min-width: 180px; }
    tr.encerrada td { opacity: 0.55; }
    :host ::ng-deep .modal:has(.tabela-sugestoes) { width: min(1150px, 96vw); }
  `
})
export class TransacoesComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly TipoTransacao = TipoTransacao;
  readonly StatusTransacao = StatusTransacao;

  readonly contas = signal<Conta[]>([]);
  readonly categorias = signal<Categoria[]>([]);
  readonly pagina = signal<PaginaTransacoes | null>(null);
  readonly selecionadas = signal<Set<number>>(new Set());
  readonly salvando = signal(false);
  readonly sugestoesForm = signal<OpcaoCategoria[]>([]);
  readonly sugestoesAbertas = signal(false);
  readonly carregandoSugestoes = signal(false);
  readonly somenteSemCategoria = signal(true);
  readonly linhasSugestao = signal<LinhaSugestao[]>([]);
  private sugestoesAlteraram = false;

  readonly contasManuais = computed(() => this.contas().filter((c) => c.origem === OrigemConta.Manual));
  readonly totalPaginas = computed(() => {
    const p = this.pagina();
    return p ? Math.max(1, Math.ceil(p.total / p.tamanhoPagina)) : 1;
  });
  readonly todasSelecionadas = computed(() => {
    const itens = this.pagina()?.itens ?? [];
    return itens.length > 0 && itens.every((t) => this.selecionadas().has(t.id));
  });

  filtro: FiltroTransacoes = this.filtroInicial();
  filtroCategoria = '';
  categoriaLote: number | null = null;
  form: FormTransacao | null = null;
  transferencia: FormTransferencia | null = null;

  ngOnInit(): void {
    this.api.contas().subscribe((c) => this.contas.set(c));
    this.api.categorias().subscribe((c) => this.categorias.set(c.filter((x) => x.ativa)));
    this.buscar(1);
  }

  rotuloCategoria(c: Categoria): string {
    if (!c.categoriaPaiId) return c.nome;
    const pai = this.categorias().find((x) => x.id === c.categoriaPaiId);
    return pai ? `${pai.nome} › ${c.nome}` : c.nome;
  }

  aplicarFiltroCategoria(): void {
    this.filtro.semCategoria = this.filtroCategoria === 'sem';
    this.filtro.categoriaId = this.filtroCategoria && this.filtroCategoria !== 'sem' ? Number(this.filtroCategoria) : null;
    this.buscar(1);
  }

  buscar(pagina: number): void {
    this.filtro.pagina = pagina;
    this.api.transacoes(this.filtro).subscribe({
      next: (p) => {
        const trocouPagina = this.pagina() !== null && this.pagina()!.pagina !== p.pagina;
        this.pagina.set(p);
        this.limparSelecao();
        if (trocouPagina) this.rolarParaTopo();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  mudarTamanhoPagina(tamanho: number): void {
    this.filtro.tamanhoPagina = tamanho;
    try { localStorage.setItem('financas.tamanhoPagina', String(tamanho)); } catch { /* ignora */ }
    this.buscar(1);
  }

  /** Volta ao topo da lista ao trocar de página. */
  private rolarParaTopo(): void {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  alternar(id: number): void {
    this.selecionadas.update((s) => {
      const novo = new Set(s);
      if (novo.has(id)) novo.delete(id); else novo.add(id);
      return novo;
    });
  }

  alternarTodas(): void {
    const itens = this.pagina()?.itens ?? [];
    this.selecionadas.set(this.todasSelecionadas() ? new Set() : new Set(itens.map((t) => t.id)));
  }

  limparSelecao(): void {
    this.selecionadas.set(new Set());
  }

  categorizarLote(): void {
    this.api.categorizarEmLote([...this.selecionadas()], this.categoriaLote).subscribe({
      next: (r) => {
        this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) atualizada(s).`);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  marcarEmergenciaLote(emergencia: boolean): void {
    this.api.marcarEmergencia([...this.selecionadas()], emergencia).subscribe({
      next: (r) => {
        this.avisos.sucesso(emergencia
          ? `${r.transacoesAtualizadas} despesa(s) marcada(s) como emergência.`
          : `${r.transacoesAtualizadas} transação(ões) desmarcada(s).`);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ sugestões de categoria ------------------------------ */

  readonly quantidadeAlta = computed(() =>
    this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.categoriaId).length);

  abrirSugestoes(): void {
    this.sugestoesAbertas.set(true);
    this.sugestoesAlteraram = false;
    this.carregarSugestoes();
  }

  trocarModoSugestoes(semCategoria: boolean): void {
    this.somenteSemCategoria.set(semCategoria);
    this.carregarSugestoes();
  }

  private carregarSugestoes(): void {
    this.carregandoSugestoes.set(true);
    this.api.sugestoesCategoria(this.somenteSemCategoria(), 365).subscribe({
      next: (s) => {
        this.carregandoSugestoes.set(false);
        this.linhasSugestao.set(s.map((x) => ({
          ...x, escolhida: x.categoriaId, aplicada: false,
          criarRegra: x.confianca === 'alta' && x.quantidade >= 3 && x.textoRegra.length >= 4
        })));
      },
      error: (e) => {
        this.carregandoSugestoes.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  nomeConfianca(c: string): string {
    const nomes: Record<string, string> = {
      alta: 'confiança alta', media: 'confiança média', baixa: 'palavra-chave', nenhuma: 'sem sugestão'
    };
    return nomes[c] ?? c;
  }

  aplicarSugestao(l: LinhaSugestao, silencioso = false): void {
    if (!l.escolhida) return;
    const categoriaId = l.escolhida;
    this.salvando.set(true);
    this.api.categorizarEmLote(l.transacaoIds, categoriaId).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.sugestoesAlteraram = true;
        this.linhasSugestao.update((linhas) => linhas.map((x) => (x.chave === l.chave ? { ...x, aplicada: true } : x)));
        if (l.criarRegra && l.textoRegra.trim()) {
          this.api.criarRegra({
            texto: l.textoRegra.trim(), tipoCorrespondencia: TipoCorrespondencia.Contem, categoriaId, prioridade: 100, ativa: true
          }).subscribe({ error: (e) => this.avisos.erro(`Regra "${l.textoRegra}": ${mensagemDeErro(e)}`) });
        }
        if (!silencioso) this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) categorizada(s).`);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  aplicarConfiancaAlta(): void {
    const linhas = this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.escolhida);
    if (!linhas.length || !confirm(`Aplicar ${linhas.length} sugestão(ões) de confiança alta?`)) return;
    for (const l of linhas) this.aplicarSugestao(l, true);
    this.avisos.sucesso(`${linhas.length} grupo(s) categorizado(s).`);
  }

  fecharSugestoes(): void {
    this.sugestoesAbertas.set(false);
    if (this.sugestoesAlteraram) this.buscar(this.filtro.pagina ?? 1);
  }

  novo(): void {
    this.form = {
      id: null, openFinance: false, contaId: this.contasManuais()[0]?.id ?? null, data: dataIso(), descricao: '',
      valor: null, tipo: TipoTransacao.Despesa, categoriaId: null, observacao: '', ignorarRelatorios: false,
      pendente: false, descricaoOriginal: null, emergencia: false
    };
    this.sugestoesForm.set([]);
  }

  editar(t: Transacao): void {
    this.form = {
      id: t.id, openFinance: t.openFinance, contaId: t.contaId, data: t.data, descricao: t.descricao,
      valor: Math.abs(t.valor), tipo: t.valor > 0 ? TipoTransacao.Receita : TipoTransacao.Despesa,
      categoriaId: t.categoriaId, observacao: t.observacao ?? '', ignorarRelatorios: t.ignorarRelatorios,
      pendente: t.status === StatusTransacao.Pendente, descricaoOriginal: t.descricaoOriginal, emergencia: t.emergencia
    };
    this.sugestoesForm.set([]);
    this.api.sugestoesDaTransacao(t.id).subscribe({ next: (s) => this.sugestoesForm.set(s), error: () => { /* opcional */ } });
  }

  salvar(): void {
    const f = this.form;
    if (!f) return;
    this.salvando.set(true);

    const requisicao = f.openFinance && f.id
      ? this.api.ajustarTransacao(f.id, {
          descricao: f.descricao, categoriaId: f.categoriaId, observacao: f.observacao || null,
          ignorarRelatorios: f.ignorarRelatorios, emergencia: f.emergencia && f.tipo === TipoTransacao.Despesa
        })
      : (() => {
          const dto = {
            contaId: f.contaId ?? 0, data: f.data, descricao: f.descricao, valor: Number(f.valor), tipo: f.tipo,
            categoriaId: f.categoriaId, observacao: f.observacao || null, ignorarRelatorios: f.ignorarRelatorios,
            status: f.pendente ? StatusTransacao.Pendente : StatusTransacao.Efetivada,
            emergencia: f.emergencia && f.tipo === TipoTransacao.Despesa
          };
          return f.id ? this.api.atualizarTransacao(f.id, dto) : this.api.criarTransacao(dto);
        })();

    requisicao.subscribe({
      next: () => {
        this.salvando.set(false);
        this.form = null;
        this.avisos.sucesso('Transação salva.');
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  excluir(): void {
    const f = this.form;
    if (!f?.id || !confirm('Excluir esta transação?')) return;
    this.api.excluirTransacao(f.id).subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Transação excluída.');
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirTransferencia(): void {
    this.transferencia = { contaOrigemId: null, contaDestinoId: null, valor: null, data: dataIso(), descricao: '' };
  }

  salvarTransferencia(): void {
    const t = this.transferencia;
    if (!t?.contaOrigemId || !t.contaDestinoId || !t.valor) return;
    this.salvando.set(true);
    this.api.transferir({
      contaOrigemId: t.contaOrigemId, contaDestinoId: t.contaDestinoId, valor: Number(t.valor), data: t.data,
      descricao: t.descricao || null
    }).subscribe({
      next: () => {
        this.salvando.set(false);
        this.transferencia = null;
        this.avisos.sucesso('Transferência registrada.');
        this.buscar(1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  private tamanhoSalvo(): number {
    try {
      const n = Number(localStorage.getItem('financas.tamanhoPagina'));
      return [25, 50, 100, 200].includes(n) ? n : 50;
    } catch {
      return 50;
    }
  }

  private filtroInicial(): FiltroTransacoes {
    const hoje = new Date();
    return {
      inicio: primeiroDiaMes(hoje.getFullYear(), hoje.getMonth() + 1),
      fim: ultimoDiaMes(hoje.getFullYear(), hoje.getMonth() + 1),
      contaId: null, categoriaId: null, semCategoria: false, somenteEmergencias: false, tipo: null, busca: '', pagina: 1,
      tamanhoPagina: this.tamanhoSalvo()
    };
  }
}

