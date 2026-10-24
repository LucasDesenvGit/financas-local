import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  Categoria, CategoriaPluggy, NOMES_CORRESPONDENCIA, NOMES_TIPO_CATEGORIA, Regra, SalvarCategoria, SalvarRegra,
  RegraDescricao, SalvarRegraDescricao, PreviaRegraDescricao,
  TipoCategoria, TipoCorrespondencia, NaturezaGasto
} from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { IconeCategoriaComponent } from '../compartilhado/icone-categoria.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { CORES_SUGERIDAS, paramsTransacoes, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { DicaDirective } from '../compartilhado/dica.directive';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

type Aba = 'categorias' | 'regras' | 'openfinance';

@Component({
  selector: 'app-categorias',
  imports: [IconeCategoriaComponent, ListaSuspensaDirective, FormsModule, RouterLink, ModalComponent, IconeComponent, DicaDirective, CampoValorComponent,
    MoedaPipe],
  template: `
    <header class="cabecalho-pagina">
      <h1>Categorias e regras</h1>
      <div class="segmentado" role="tablist">
        <button type="button" role="tab" [class.ativo]="aba() === 'categorias'" (click)="aba.set('categorias')">Categorias</button>
        <button type="button" role="tab" [class.ativo]="aba() === 'regras'" (click)="aba.set('regras')">Regras automáticas</button>
        <button type="button" role="tab" [class.ativo]="aba() === 'openfinance'" (click)="abrirOpenFinance()">Categorias do banco</button>
      </div>
    </header>

    @switch (aba()) {
      @case ('categorias') {
        <div class="linha-flex"><span class="cresce"></span>
          <button type="button" class="botao botao-primario" (click)="novaCategoria()">+ Categoria</button></div>
        <div class="grade-3">
          @for (grupo of grupos(); track grupo.tipo) {
            <section class="painel-bloco">
              <h2>{{ grupo.nome }}</h2>
              <ul class="lista-simples">
                @for (c of grupo.itens; track c.id) {
                  <li class="clicavel" (click)="editarCategoria(c)" [class.ignorada]="!c.ativa">
                    <app-icone-categoria [nome]="c.nome" [cor]="c.cor" />
                    <!-- WCAG 2.1.1: o clique na linha é atalho de mouse; pelo teclado, a edição abre por este botão. -->
                    <button type="button" class="cresce botao-texto" [class.subcategoria]="c.categoriaPaiId"
                      (click)="$event.stopPropagation(); editarCategoria(c)" [attr.aria-label]="'Editar a categoria ' + c.nome">{{ c.nome }}</button>
                    @if (c.natureza === NaturezaGasto.Essencial) { <span class="etiqueta etiqueta-suave">essencial</span> }
                    @if (c.natureza === NaturezaGasto.EstiloDeVida) { <span class="etiqueta etiqueta-suave">estilo de vida</span> }
                    @if (!c.ativa) { <small class="texto-suave">inativa</small> }
                    <a class="botao-icone-link" routerLink="/transacoes" [queryParams]="linkTransacoes(c.id)"
                      (click)="$event.stopPropagation()" (keydown.enter)="$event.stopPropagation()"
                      [appDica]="'Ver transações (12 meses)'" [attr.aria-label]="'Ver transações de ' + c.nome + ' nos últimos 12 meses'">
                      <app-icone [icone]="Icones.transacoes" [tamanho]="16" />
                    </a>
                  </li>
                }
              </ul>
            </section>
          }
        </div>
      }
      @case ('regras') {
        <p class="texto-suave">Regras rodam em cada sincronização e em lançamentos manuais sem categoria. A primeira regra que
          corresponder (menor prioridade) define a categoria. Categorias escolhidas à mão nunca são sobrescritas.</p>
        <div class="linha-flex">
          <button type="button" class="botao" (click)="reaplicar()">Reaplicar regras em todas as transações</button>
          <span class="cresce"></span>
          <button type="button" class="botao botao-primario" (click)="novaRegra()">+ Regra</button>
        </div>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead><tr><th>Prioridade</th><th>Quando a descrição</th><th>Texto</th><th>Categoria</th><th></th></tr></thead>
            <tbody>
              @for (r of regras(); track r.id) {
                <tr class="clicavel" (click)="editarRegra(r)" [class.ignorada]="!r.ativa">
                  <td>{{ r.prioridade }}</td>
                  <td>{{ nomeCorrespondencia(r.tipoCorrespondencia) }}</td>
                  <td><code>{{ r.texto }}</code></td>
                  <td>{{ r.categoriaNome }}</td>
                  <td class="texto-suave">
                    <button type="button" class="link" (click)="$event.stopPropagation(); editarRegra(r)"
                      [attr.aria-label]="'Editar a regra ' + r.texto + ' → ' + r.categoriaNome">editar</button>
                    {{ r.ativa ? '' : ' · inativa' }}
                  </td>
                </tr>
              } @empty { <tr><td colspan="5" class="vazio">Ex.: descrição contém "UBER" → Transporte.</td></tr> }
            </tbody>
          </table>
        </div>

        <div class="linha-flex">
          <h2 class="titulo-regras-descricao" id="regras-descricao">Regras de descrição</h2>
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="novaRegraDescricao()">+ Regra de descrição</button>
        </div>
        <p class="texto-suave">Dão um nome mais claro às transações do banco ("PIX ENVIADO 12345" → "Aluguel"). Valem nas
          transações que já existem e nas próximas; um nome escolhido à mão não muda. Crie aqui ou pelo ✎ "Renomear" em Transações.
          Com a mesma descrição do banco e valores diferentes (as duas contas do condomínio), use o valor para separar.</p>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead><tr><th>Quando a descrição do banco</th><th>Texto</th><th>Valor</th><th>Mostrar</th><th class="direita">Transações</th><th></th></tr></thead>
            <tbody>
              @for (r of regrasDescricao(); track r.id) {
                <tr class="clicavel" (click)="editarRegraDescricao(r)" [class.ignorada]="!r.ativa">
                  <td>{{ nomeCorrespondencia(r.tipoCorrespondencia) }}</td>
                  <td><code>{{ r.texto }}</code></td>
                  <td class="faixa-valor">
                    @if (r.valorMinimo != null && r.valorMaximo != null) { de {{ r.valorMinimo | currency }} a {{ r.valorMaximo | currency }} }
                    @else if (r.valorMinimo != null) { a partir de {{ r.valorMinimo | currency }} }
                    @else if (r.valorMaximo != null) { até {{ r.valorMaximo | currency }} }
                    @else { <span class="texto-suave">qualquer</span> }
                  </td>
                  <td>{{ r.novaDescricao }}</td>
                  <td class="direita">{{ r.transacoes }}</td>
                  <td class="texto-suave">
                    <button type="button" class="link" (click)="$event.stopPropagation(); editarRegraDescricao(r)"
                      [attr.aria-label]="'Editar a regra que mostra ' + r.novaDescricao">editar</button>
                    {{ r.ativa ? '' : ' · desligada' }}
                  </td>
                </tr>
              } @empty { <tr><td colspan="6" class="vazio">Nenhuma ainda. Em Transações, use ✎ "Renomear" e escolha "Também nas outras".</td></tr> }
            </tbody>
          </table>
        </div>
      }
      @case ('openfinance') {
        <p class="texto-suave">O Open Finance sugere uma categoria para cada transação. Escolha a categoria local equivalente;
          subcategorias sem mapeamento herdam o da categoria pai.</p>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead><tr><th>Categoria do banco</th><th>Grupo</th><th class="direita">Transações</th><th style="width: 260px">Categoria local</th></tr></thead>
            <tbody>
              @for (p of categoriasPluggy(); track p.pluggyCategoriaId) {
                <tr>
                  <td>{{ p.descricao }}</td>
                  <td class="texto-suave">{{ p.paiDescricao ?? '—' }}</td>
                  <td class="direita">{{ p.quantidadeTransacoes }}</td>
                  <td>
                    <select [ngModel]="p.categoriaId" (ngModelChange)="mapear(p, $event)" class="input-compacto">
                      <option [ngValue]="null">— herdar / sem categoria —</option>
                      @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
                    </select>
                  </td>
                </tr>
              } @empty { <tr><td colspan="4" class="vazio">A lista aparece depois da primeira sincronização com o Open Finance.</td></tr> }
            </tbody>
          </table>
        </div>
      }
    }

    <app-modal [aberto]="formCategoria !== null" [titulo]="categoriaId ? 'Editar categoria' : 'Nova categoria'" (fechar)="formCategoria = null">
      @if (formCategoria; as fc) {
        <form (ngSubmit)="salvarCategoria()" #f="ngForm" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fc.nome" required maxlength="80"></label>
          <div class="grade-campos">
            <label>Tipo
              <select name="tipo" [(ngModel)]="fc.tipo">
                <option [ngValue]="TipoCategoria.Despesa">Despesa</option>
                <option [ngValue]="TipoCategoria.Receita">Receita</option>
                <option [ngValue]="TipoCategoria.Transferencia">Transferência (não entra nos relatórios)</option>
              </select>
            </label>
            <label>Subcategoria de
              <select name="pai" [(ngModel)]="fc.categoriaPaiId">
                <option [ngValue]="null">— nenhuma —</option>
                @for (c of possiveisPais(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            @if (fc.tipo === TipoCategoria.Despesa) {
              <label>Natureza do gasto
                <select name="natureza" [(ngModel)]="fc.natureza">
                  <option [ngValue]="NaturezaGasto.Essencial">Essencial (moradia, mercado, saúde…)</option>
                  <option [ngValue]="NaturezaGasto.EstiloDeVida">Estilo de vida (lazer, compras…)</option>
                  <option [ngValue]="null">Não classificar</option>
                </select>
              </label>
            }
          </div>
          <div class="cores">
            @for (cor of cores; track cor) {
              <button type="button" class="cor" [style.background]="cor" [class.selecionada]="fc.cor === cor"
                (click)="fc.cor = cor" [attr.aria-label]="'Cor ' + cor"></button>
            }
          </div>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="fc.ativa"> Ativa</label>
          @if (categoriaId) {
            <a class="link-alvo" routerLink="/transacoes" [queryParams]="linkTransacoes(categoriaId)" (click)="formCategoria = null">
              Ver transações desta categoria (últimos 12 meses)</a>
          }
          <div class="rodape-form">
            @if (categoriaId) { <button type="button" class="botao botao-perigo" (click)="excluirCategoria()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formCategoria = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="formRegraDescricao !== null" [titulo]="regraDescricaoId ? 'Editar regra de descrição' : 'Nova regra de descrição'"
      (fechar)="formRegraDescricao = null">
      @if (formRegraDescricao; as frd) {
        <form (ngSubmit)="salvarRegraDescricao()" #frdf="ngForm" class="formulario">
          <div class="grade-campos">
            <label>Quando a descrição do banco
              <select name="tipo" [(ngModel)]="frd.tipoCorrespondencia" (ngModelChange)="pedirPreviaRegraDescricao()">
                <option [ngValue]="TipoCorrespondencia.Igual">for igual a</option>
                <option [ngValue]="TipoCorrespondencia.Contem">contiver</option>
                <option [ngValue]="TipoCorrespondencia.ComecaCom">começar com</option>
              </select>
            </label>
            <label>Texto <input name="texto" [(ngModel)]="frd.texto" (ngModelChange)="pedirPreviaRegraDescricao()" required minlength="2" maxlength="200"></label>
          </div>
          <fieldset class="faixa-regra" aria-describedby="dica-faixa-regra">
            <legend>E o valor (opcional)</legend>
            <div class="grade-campos">
              <label>De <app-campo-valor name="valorMinimo" [(ngModel)]="frd.valorMinimo" [minimo]="0.01"
                (ngModelChange)="pedirPreviaRegraDescricao()" /></label>
              <label>Até <app-campo-valor name="valorMaximo" [(ngModel)]="frd.valorMaximo" [minimo]="0.01"
                (ngModelChange)="pedirPreviaRegraDescricao()" /></label>
            </div>
            @if (faixaInvertida(frd)) {
              <p class="erro pequeno" role="alert">O valor "De" é maior que o "Até": troque os dois.</p>
            }
            <p class="texto-suave" id="dica-faixa-regra">Para separar cobranças com a mesma descrição do banco e valores
              diferentes, como duas contas do condomínio: uma regra para cada faixa, cada uma com o seu nome. Vale o valor sem
              o sinal; deixe em branco para qualquer valor. Em Contas a pagar, cada nome vira uma conta.</p>
          </fieldset>
          @if (!regraDescricaoId) {
            <p class="texto-suave" role="status">{{ textoPreviaRegraDescricao() }}</p>
          }
          <label>Mostrar como <input name="nova" [(ngModel)]="frd.novaDescricao" required maxlength="300"></label>
          <p class="texto-suave">Maiúsculas e acentos são ignorados. Ao salvar, as transações de nome automático são renomeadas de novo.</p>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="frd.ativa"> Ativa</label>
          <div class="rodape-form">
            @if (regraDescricaoId) { <button type="button" class="botao botao-perigo" (click)="excluirRegraDescricao()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formRegraDescricao = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="frdf.invalid || faixaInvertida(frd)">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="formRegra !== null" [titulo]="regraId ? 'Editar regra' : 'Nova regra'" (fechar)="formRegra = null">
      @if (formRegra; as frg) {
        <form (ngSubmit)="salvarRegra()" #fr="ngForm" class="formulario">
          <div class="grade-campos">
            <label>Quando a descrição
              <select name="tipo" [(ngModel)]="frg.tipoCorrespondencia">
                <option [ngValue]="TipoCorrespondencia.Contem">contém</option>
                <option [ngValue]="TipoCorrespondencia.ComecaCom">começa com</option>
                <option [ngValue]="TipoCorrespondencia.Igual">é igual a</option>
              </select>
            </label>
            <label>Texto <input name="texto" [(ngModel)]="frg.texto" required minlength="2" maxlength="200"></label>
            <label>Categoria
              <select name="cat" [(ngModel)]="frg.categoriaId" required>
                @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Prioridade <input type="number" name="prio" [(ngModel)]="frg.prioridade" min="0" max="10000"></label>
          </div>
          <p class="texto-suave">Maiúsculas e acentos são ignorados.</p>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="frg.ativa"> Ativa</label>
          <div class="rodape-form">
            @if (regraId) { <button type="button" class="botao botao-perigo" (click)="excluirRegra()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formRegra = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fr.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    .titulo-regras-descricao { font-size: 1.05rem; margin: 26px 0 4px; }
    .faixa-regra { border: 1px solid var(--linha); border-radius: 10px; padding: 8px 12px 4px; margin: 0; }
    .faixa-regra legend { font-weight: 600; padding: 0 4px; }
    .faixa-regra .texto-suave { margin: 6px 0; }
    .faixa-valor { white-space: nowrap; }
  `
})
export class CategoriasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly TipoCategoria = TipoCategoria;
  readonly TipoCorrespondencia = TipoCorrespondencia;
  readonly NaturezaGasto = NaturezaGasto;
  readonly Icones = Icones;

  /**
   * UX-6: transações da categoria nos últimos 12 meses (esta tela não tem período; o mês atual deixaria vazia uma
   * categoria pouco usada, como IPVA). Categoria pai traz as subcategorias (o back-end já faz isso).
   */
  linkTransacoes(categoriaId: number): Record<string, string> {
    const hoje = new Date();
    const inicio = new Date(hoje.getFullYear(), hoje.getMonth() - 11, 1);
    return paramsTransacoes({
      categoriaId,
      inicio: primeiroDiaMes(inicio.getFullYear(), inicio.getMonth() + 1),
      fim: ultimoDiaMes(hoje.getFullYear(), hoje.getMonth() + 1)
    });
  }
  readonly cores = CORES_SUGERIDAS;

  readonly aba = signal<Aba>('categorias');
  readonly categorias = signal<Categoria[]>([]);
  readonly regras = signal<Regra[]>([]);
  readonly categoriasPluggy = signal<CategoriaPluggy[]>([]);

  /** Categorias agrupadas por tipo, com subcategorias logo abaixo da categoria pai. */
  readonly grupos = computed(() =>
    [TipoCategoria.Despesa, TipoCategoria.Receita, TipoCategoria.Transferencia].map((tipo) => {
      const doTipo = this.categorias().filter((c) => c.tipo === tipo);
      const ordenadas: Categoria[] = [];
      for (const pai of doTipo.filter((c) => !c.categoriaPaiId)) {
        ordenadas.push(pai, ...doTipo.filter((c) => c.categoriaPaiId === pai.id));
      }
      return { tipo, nome: NOMES_TIPO_CATEGORIA[tipo], itens: ordenadas };
    }));

  formCategoria: SalvarCategoria | null = null;
  categoriaId: number | null = null;
  formRegra: SalvarRegra | null = null;
  regraId: number | null = null;

  ngOnInit(): void {
    this.carregarCategorias();
    this.carregarRegras();
  }

  possiveisPais(): Categoria[] {
    const tipo = this.formCategoria?.tipo;
    return this.categorias().filter((c) => !c.categoriaPaiId && c.tipo === tipo && c.id !== this.categoriaId);
  }

  nomeCorrespondencia(t: TipoCorrespondencia): string {
    return NOMES_CORRESPONDENCIA[t];
  }

  carregarCategorias(): void {
    this.api.categorias().subscribe({ next: (c) => this.categorias.set(c), error: (e) => this.avisos.erro(mensagemDeErro(e)) });
  }

  carregarRegras(): void {
    this.api.regras().subscribe({ next: (r) => this.regras.set(r), error: (e) => this.avisos.erro(mensagemDeErro(e)) });
    this.carregarRegrasDescricao();
  }

  abrirOpenFinance(): void {
    this.aba.set('openfinance');
    this.api.categoriasPluggy().subscribe({
      next: (c) => this.categoriasPluggy.set(c),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  novaCategoria(): void {
    this.categoriaId = null;
    this.formCategoria = { nome: '', tipo: TipoCategoria.Despesa, cor: CORES_SUGERIDAS[0], icone: null, categoriaPaiId: null, ativa: true, natureza: NaturezaGasto.EstiloDeVida };
  }

  editarCategoria(c: Categoria): void {
    this.categoriaId = c.id;
    this.formCategoria = { nome: c.nome, tipo: c.tipo, cor: c.cor, icone: c.icone, categoriaPaiId: c.categoriaPaiId, ativa: c.ativa, natureza: c.natureza };
  }

  salvarCategoria(): void {
    if (!this.formCategoria) return;
    const req = this.categoriaId
      ? this.api.atualizarCategoria(this.categoriaId, this.formCategoria)
      : this.api.criarCategoria(this.formCategoria);
    req.subscribe({
      next: () => {
        this.formCategoria = null;
        this.avisos.sucesso('Categoria salva.');
        this.carregarCategorias();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluirCategoria(): void {
    if (!this.categoriaId || !confirm('Excluir a categoria? As transações dela ficarão sem categoria e os orçamentos/regras dela serão removidos.')) return;
    this.api.excluirCategoria(this.categoriaId).subscribe({
      next: () => {
        this.formCategoria = null;
        this.carregarCategorias();
        this.carregarRegras();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  novaRegra(): void {
    this.regraId = null;
    this.formRegra = {
      texto: '', tipoCorrespondencia: TipoCorrespondencia.Contem,
      categoriaId: this.categorias()[0]?.id ?? 0, prioridade: 100, ativa: true
    };
  }

  editarRegra(r: Regra): void {
    this.regraId = r.id;
    this.formRegra = {
      texto: r.texto, tipoCorrespondencia: r.tipoCorrespondencia, categoriaId: r.categoriaId,
      prioridade: r.prioridade, ativa: r.ativa
    };
  }

  salvarRegra(): void {
    if (!this.formRegra) return;
    const dto: SalvarRegra = { ...this.formRegra, prioridade: Number(this.formRegra.prioridade) || 0 };
    const req = this.regraId ? this.api.atualizarRegra(this.regraId, dto) : this.api.criarRegra(dto);
    req.subscribe({
      next: () => {
        this.formRegra = null;
        this.avisos.sucesso('Regra salva. Use "Reaplicar regras" para atualizar transações antigas.');
        this.carregarRegras();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluirRegra(): void {
    if (!this.regraId || !confirm('Excluir esta regra?')) return;
    this.api.excluirRegra(this.regraId).subscribe({
      next: () => {
        this.formRegra = null;
        this.carregarRegras();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ #123: regras de descrição ------------------------------ */

  readonly regrasDescricao = signal<RegraDescricao[]>([]);
  formRegraDescricao: SalvarRegraDescricao | null = null;
  regraDescricaoId: number | null = null;

  carregarRegrasDescricao(): void {
    this.api.regrasDescricao().subscribe({
      next: (r) => this.regrasDescricao.set(r),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** Prévia da regra nova: quantas transações de nome automático ela renomeia (Nielsen 5). */
  readonly previaRegraDescricao = signal<PreviaRegraDescricao | null>(null);
  private esperaPrevia: ReturnType<typeof setTimeout> | undefined;

  novaRegraDescricao(): void {
    this.regraDescricaoId = null;
    this.previaRegraDescricao.set(null);
    this.formRegraDescricao = {
      texto: '', tipoCorrespondencia: TipoCorrespondencia.Contem, novaDescricao: '', ativa: true, valorMinimo: null, valorMaximo: null
    };
  }

  /** #241: as duas pontas preenchidas e a de baixo acima da de cima (WCAG 3.3.1: o erro diz o campo e o que fazer). */
  faixaInvertida(f: SalvarRegraDescricao): boolean {
    return f.valorMinimo != null && f.valorMaximo != null && f.valorMinimo > f.valorMaximo;
  }

  pedirPreviaRegraDescricao(): void {
    clearTimeout(this.esperaPrevia);
    const f = this.formRegraDescricao;
    if (this.regraDescricaoId || !f || f.texto.trim().length < 2 || this.faixaInvertida(f)) {
      this.previaRegraDescricao.set(null);
      return;
    }
    this.esperaPrevia = setTimeout(() => this.api.previaRegraDescricao(f.texto.trim(), f.tipoCorrespondencia,
      f.valorMinimo ?? null, f.valorMaximo ?? null).subscribe({
      next: (p) => this.previaRegraDescricao.set(p),
      error: () => this.previaRegraDescricao.set(null)
    }), 300);
  }

  textoPreviaRegraDescricao(): string {
    const p = this.previaRegraDescricao();
    if (!p && this.formRegraDescricao && this.faixaInvertida(this.formRegraDescricao)) {
      return 'Acerte o valor para ver quantas transações a regra renomeia.';
    }
    if (!p) return 'Escreva o texto para ver quantas transações a regra renomeia.';
    if (p.quantidade === 0) {
      const f = this.formRegraDescricao;
      return f && (f.valorMinimo != null || f.valorMaximo != null)
        ? 'Nenhuma transação tem essa descrição do banco com esse valor agora: a regra vale para as próximas.'
        : 'Nenhuma transação tem essa descrição do banco agora: a regra vale para as próximas.';
    }
    const exemplos = p.exemplos.length ? ` (ex.: ${p.exemplos.join('; ')})` : '';
    return `Vai renomear ${p.quantidade} transação(ões)${exemplos} e as próximas.`;
  }

  editarRegraDescricao(r: RegraDescricao): void {
    this.regraDescricaoId = r.id;
    this.formRegraDescricao = {
      texto: r.texto, tipoCorrespondencia: r.tipoCorrespondencia, novaDescricao: r.novaDescricao, ativa: r.ativa,
      valorMinimo: r.valorMinimo ?? null, valorMaximo: r.valorMaximo ?? null
    };
  }

  salvarRegraDescricao(): void {
    const id = this.regraDescricaoId;
    const f = this.formRegraDescricao;
    if (!f || this.faixaInvertida(f)) return;
    const dto: SalvarRegraDescricao = {
      ...f, texto: f.texto.trim(), novaDescricao: f.novaDescricao.trim(), valorMinimo: f.valorMinimo ?? null,
      valorMaximo: f.valorMaximo ?? null
    };
    (id ? this.api.salvarRegraDescricao(id, dto) : this.api.criarRegraDescricao(dto)).subscribe({
      next: (r) => {
        this.formRegraDescricao = null;
        this.avisos.sucesso(`Regra salva: ${r.transacoes} transação(ões) com o nome "${r.novaDescricao}".`);
        this.carregarRegrasDescricao();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluirRegraDescricao(): void {
    const id = this.regraDescricaoId;
    if (!id || !confirm('Excluir esta regra? As transações com o nome dado por ela voltam à descrição do banco.')) return;
    this.api.excluirRegraDescricao(id).subscribe({
      next: () => {
        this.formRegraDescricao = null;
        this.avisos.sucesso('Regra excluída: as transações voltaram à descrição do banco.');
        this.carregarRegrasDescricao();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  reaplicar(): void {
    this.api.reaplicarRegras().subscribe({
      next: (r) => this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) recategorizada(s).`),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  mapear(p: CategoriaPluggy, categoriaId: number | null): void {
    this.api.mapearCategoriaPluggy(p.pluggyCategoriaId, categoriaId, true).subscribe({
      next: (r) => {
        p.categoriaId = categoriaId;
        this.avisos.sucesso(`Mapeamento salvo. ${r.transacoesAtualizadas} transação(ões) atualizada(s).`);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
