import { DatePipe } from '@angular/common';
import { Component, effect, inject, input, output, signal, untracked } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  CompraSemOperacao, ConferenciaInvestimentos, CotasDiferentes, CreditoDaCorretora, NOMES_TIPO_ATIVO,
  RendimentoComoInvestimento, ResultadoCorrecaoInvestimentos, TipoOperacao, TipoParaCorrigir
} from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { TermoComponent } from '../compartilhado/termo.component';

/** Linha marcável da conferência. */
interface Marcavel<T> { item: T; marcada: boolean; }

/** Cotas que faltam: a compra a lançar, ligada a um débito do extrato. */
interface LinhaCotas extends Marcavel<CotasDiferentes> { debitoId: number | null; quantidade: number | null; preco: number | null; }

/** Listas longas começam recolhidas (Progressive Disclosure): o resumo do grupo já diz o essencial. */
const LINHAS_VISIVEIS = 8;

/**
 * "Conferir investimentos" (ADR 0027). Mostra, por grupo, o que parece errado e a correção sugerida, já marcada;
 * nada muda até "Corrigir selecionados", e o aviso tem "Desfazer" (Nielsen 3 e 5). Cada grupo diz em palavras do
 * dia a dia o que aconteceu e para onde vai (Nielsen 2), com o total antes do detalhe (Shneiderman).
 */
@Component({
  selector: 'app-conferir-investimentos',
  imports: [FormsModule, DatePipe, MoedaPipe, ModalComponent, TermoComponent],
  template: `
    <app-modal [aberto]="aberto()" titulo="Conferir investimentos" (fechar)="fechar.emit()">
      <div class="formulario conferencia">
        <p class="texto-suave">Compara a carteira com a posição da corretora e com o extrato. Nada muda até você clicar em
          “Corrigir selecionados”, e depois dá para desfazer.</p>

        @if (carregando()) {
          <p class="vazio" role="status">Conferindo a carteira, a posição da corretora e o extrato…</p>
        } @else if (erro(); as e) {
          <p class="vazio" role="alert">{{ e }} <button type="button" class="link" (click)="carregar()">Tentar de novo</button></p>
        } @else if (dados()) {
          <p class="resumo-conferencia" role="status">{{ resumo() }}</p>

          @if (tipos().length) {
            <section aria-labelledby="conf-tipos">
              <h3 id="conf-tipos">FII cadastrado como outro tipo ({{ tipos().length }})</h3>
              <p class="texto-suave">Fica fora do total e da aba de FIIs. Passa a ser FII.</p>
              <ul class="lista-conferencia">
                @for (l of tipos(); track l.item.ativoId) {
                  <li>
                    <label><input type="checkbox" [(ngModel)]="l.marcada">
                      <strong>{{ l.item.ticker }}</strong>: está como {{ nomeTipo(l.item) }}, mas {{ l.item.motivo }}.</label>
                  </li>
                }
              </ul>
            </section>
          }

          @if (cotas().length) {
            <section aria-labelledby="conf-cotas">
              <h3 id="conf-cotas">Cotas diferentes da corretora ({{ cotas().length }})</h3>
              <p class="texto-suave">A corretora pode deixar uma compra fora do histórico que manda pelo Open Finance. A compra
                lançada aqui fica ligada ao débito do extrato.</p>
              <ul class="lista-conferencia">
                @for (l of cotas(); track l.item.ativoId) {
                  <li class="linha-cotas">
                    <p><strong>{{ l.item.ticker }}</strong>: a corretora informa {{ qtd(l.item.naCorretora) }} cotas
                      (posição de {{ l.item.posicaoEm | date: 'dd/MM' }}); a carteira tem {{ qtd(l.item.naCarteira) }}.
                      @if (faltam(l.item) > 0) { Faltam {{ qtd(faltam(l.item)) }}. }
                      @else { A carteira tem {{ qtd(-faltam(l.item)) }} a mais. }</p>
                    @if (faltam(l.item) <= 0) {
                      <button type="button" class="botao botao-pequeno" (click)="verOperacoes.emit(l.item.ticker)">
                        Ver as operações de {{ l.item.ticker }}</button>
                    } @else if (debitosPara(l.item).length) {
                      <div class="compra-que-faltou">
                        <label class="marcar"><input type="checkbox" [(ngModel)]="l.marcada"> Lançar a compra</label>
                        <label>Débito do extrato
                          <select [(ngModel)]="l.debitoId" (ngModelChange)="recalcular(l)" [disabled]="!l.marcada">
                            <option [ngValue]="null">escolha…</option>
                            @for (d of debitosPara(l.item); track d.transacaoId) {
                              <option [ngValue]="d.transacaoId">{{ rotuloDebito(d) }}</option>
                            }
                          </select>
                        </label>
                        <label>Cotas
                          <input type="number" class="campo-curto" min="0" step="any" [(ngModel)]="l.quantidade"
                            (ngModelChange)="recalcular(l)" [disabled]="!l.marcada">
                        </label>
                        <label>Preço por cota
                          <input type="number" class="campo-curto" min="0" step="any" [(ngModel)]="l.preco" [disabled]="!l.marcada">
                        </label>
                        @if (l.debitoId !== null && l.debitoId === l.item.debitoSugeridoId && l.item.fechamentoNoDia !== null) {
                          <small class="texto-suave">fechamento do dia: {{ l.item.fechamentoNoDia | currency }}</small>
                        }
                      </div>
                    } @else {
                      <p class="texto-suave">Nenhum débito de compra sem operação no extrato explica a diferença.
                        <button type="button" class="link" (click)="lancarAMao.emit({ ticker: l.item.ticker, quantidade: faltam(l.item) })">
                          Lançar a compra à mão</button></p>
                    }
                  </li>
                }
              </ul>
            </section>
          }

          @if (trocos().length) {
            <section aria-labelledby="conf-trocos">
              <h3 id="conf-trocos"><app-termo chave="trocoDeCompra">Troco</app-termo> ou venda contando como rendimento
                ({{ trocos().length }})</h3>
              <p class="texto-suave">{{ total(trocos()) | currency }} em créditos da corretora que são troco de uma compra ou o
                valor de uma venda. Vão para “{{ dados()!.categoriaDoTroco?.nome ?? 'Investimentos' }}”, junto com a compra.</p>
              @if (!dados()!.categoriaDoTroco) {
                <p class="aviso-campo" role="alert">Crie uma categoria do tipo transferência (ex.: Investimentos) para o troco.</p>
              }
              <div class="barra-grupo">
                <button type="button" class="link" (click)="marcarTodos(trocos(), true)">Marcar todos</button>
                <button type="button" class="link" (click)="marcarTodos(trocos(), false)">Desmarcar todos</button>
                @if (trocos().length > linhasVisiveis) {
                  <button type="button" class="link" [attr.aria-expanded]="trocosAbertos()" aria-controls="tabela-trocos"
                    (click)="trocosAbertos.set(!trocosAbertos())">
                    {{ trocosAbertos() ? 'Mostrar menos' : 'Ver todos os ' + trocos().length }}</button>
                }
              </div>
              <div class="tabela-rolagem">
                <table class="tabela" id="tabela-trocos">
                  <thead><tr><th><span class="so-leitor">Corrigir</span></th><th>Data</th><th class="direita">Valor</th>
                    <th>De onde veio</th><th>Conta</th><th>Categoria hoje</th></tr></thead>
                  <tbody>
                    @for (l of visiveis(trocos(), trocosAbertos()); track l.item.transacaoId) {
                      <tr>
                        <td><input type="checkbox" [(ngModel)]="l.marcada" [disabled]="!dados()!.categoriaDoTroco"
                          [attr.aria-label]="'Corrigir o crédito de ' + (l.item.data | date: 'dd/MM/yyyy')"></td>
                        <td>{{ l.item.data | date: 'dd/MM/yy' }}</td>
                        <td class="direita">{{ l.item.valor | currency }}</td>
                        <td>{{ origem(l.item) }}</td>
                        <td>{{ l.item.conta }}</td>
                        <td>{{ l.item.categoria ?? 'sem categoria' }}</td>
                      </tr>
                    }
                  </tbody>
                </table>
              </div>
            </section>
          }

          @if (rendimentos().length) {
            <section aria-labelledby="conf-rendimentos">
              <h3 id="conf-rendimentos">Rendimento contando como investimento ({{ rendimentos().length }})</h3>
              <p class="texto-suave">{{ total(rendimentos()) | currency }} em créditos da corretora com a mesma descrição do troco,
                mas que não são troco nem venda: são proventos. Voltam para a categoria das regras
                (“{{ rendimentos()[0].item.categoriaAutomatica }}”) e contam como receita.</p>
              <div class="barra-grupo">
                <button type="button" class="link" (click)="marcarTodos(rendimentos(), true)">Marcar todos</button>
                <button type="button" class="link" (click)="marcarTodos(rendimentos(), false)">Desmarcar todos</button>
                @if (rendimentos().length > linhasVisiveis) {
                  <button type="button" class="link" [attr.aria-expanded]="rendimentosAbertos()" aria-controls="tabela-rendimentos"
                    (click)="rendimentosAbertos.set(!rendimentosAbertos())">
                    {{ rendimentosAbertos() ? 'Mostrar menos' : 'Ver todos os ' + rendimentos().length }}</button>
                }
              </div>
              <div class="tabela-rolagem">
                <table class="tabela" id="tabela-rendimentos">
                  <thead><tr><th><span class="so-leitor">Corrigir</span></th><th>Data</th><th class="direita">Valor</th>
                    <th>Descrição</th><th>Conta</th><th>Categoria hoje</th></tr></thead>
                  <tbody>
                    @for (l of visiveis(rendimentos(), rendimentosAbertos()); track l.item.transacaoId) {
                      <tr>
                        <td><input type="checkbox" [(ngModel)]="l.marcada"
                          [attr.aria-label]="'Corrigir o rendimento de ' + (l.item.data | date: 'dd/MM/yyyy')"></td>
                        <td>{{ l.item.data | date: 'dd/MM/yy' }}</td>
                        <td class="direita">{{ l.item.valor | currency }}</td>
                        <td>{{ l.item.descricao }}</td>
                        <td>{{ l.item.conta }}</td>
                        <td>{{ l.item.categoria ?? 'sem categoria' }}</td>
                      </tr>
                    }
                  </tbody>
                </table>
              </div>
            </section>
          }
        }

        <div class="rodape-form">
          <span class="texto-suave">{{ selecionadas() }} correção(ões) marcada(s)</span>
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="fechar.emit()">Fechar</button>
          <button type="button" class="botao botao-primario" [disabled]="!selecionadas() || salvando()" (click)="corrigir()">
            {{ salvando() ? 'Corrigindo…' : 'Corrigir selecionados' }}</button>
        </div>
      </div>
    </app-modal>
  `,
  styles: `
    .conferencia section { margin-top: 18px; }
    .conferencia h3 { margin: 0 0 4px; font-size: 1rem; }
    .conferencia h3 + p { margin: 0 0 8px; }
    .resumo-conferencia { font-weight: 600; margin: 8px 0 0; }
    .lista-conferencia { list-style: none; padding: 0; margin: 0; display: grid; gap: 10px; }
    .lista-conferencia label { display: inline-flex; gap: 8px; align-items: baseline; }
    .linha-cotas p { margin: 0 0 6px; }
    .compra-que-faltou { display: flex; flex-wrap: wrap; gap: 10px 14px; align-items: end; }
    .compra-que-faltou label:not(.marcar) { display: grid; gap: 4px; font-size: 0.85rem; }
    .compra-que-faltou .marcar { align-self: center; min-height: 24px; }
    .campo-curto { width: 110px; padding: 4px 8px; }
    .barra-grupo { display: flex; gap: 16px; flex-wrap: wrap; margin: 0 0 6px; }
    .barra-grupo .link, .lista-conferencia input[type=checkbox], .tabela input[type=checkbox] { min-height: 24px; min-width: 24px; }
    .tabela td { vertical-align: middle; }
    .aviso-campo { color: var(--negativo); margin: 0 0 8px; }
    :host ::ng-deep .modal:has(.conferencia) { width: min(1000px, 96vw); }
  `
})
export class ConferirInvestimentosComponent {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly aberto = input(false);
  readonly fechar = output<void>();
  /** Algo mudou (correção ou desfazer): a tela recarrega a carteira. */
  readonly corrigido = output<void>();
  readonly lancarAMao = output<{ ticker: string; quantidade: number }>();
  readonly verOperacoes = output<string>();

  readonly linhasVisiveis = LINHAS_VISIVEIS;
  readonly carregando = signal(false);
  readonly salvando = signal(false);
  readonly erro = signal<string | null>(null);
  readonly dados = signal<ConferenciaInvestimentos | null>(null);
  readonly tipos = signal<Marcavel<TipoParaCorrigir>[]>([]);
  readonly cotas = signal<LinhaCotas[]>([]);
  readonly trocos = signal<Marcavel<CreditoDaCorretora>[]>([]);
  readonly rendimentos = signal<Marcavel<RendimentoComoInvestimento>[]>([]);
  readonly trocosAbertos = signal(false);
  readonly rendimentosAbertos = signal(false);

  constructor() {
    effect(() => {
      if (this.aberto()) untracked(() => this.carregar());
    });
  }

  carregar(): void {
    this.carregando.set(true);
    this.erro.set(null);
    this.api.conferenciaInvestimentos().subscribe({
      next: (c) => {
        this.dados.set(c);
        this.tipos.set(c.tipos.map((item) => ({ item, marcada: true })));
        this.cotas.set(c.cotas.map((item) => {
          const faltam = item.naCorretora - item.naCarteira;
          return { item, marcada: faltam > 0 && item.debitoSugeridoId !== null, debitoId: item.debitoSugeridoId,
            quantidade: faltam > 0 ? faltam : null, preco: item.precoSugerido };
        }));
        this.trocos.set(c.trocos.map((item) => ({ item, marcada: c.categoriaDoTroco !== null })));
        this.rendimentos.set(c.rendimentos.map((item) => ({ item, marcada: true })));
        this.trocosAbertos.set(c.trocos.length <= LINHAS_VISIVEIS);
        this.rendimentosAbertos.set(c.rendimentos.length <= LINHAS_VISIVEIS);
        this.carregando.set(false);
      },
      error: (e) => {
        this.carregando.set(false);
        this.erro.set(mensagemDeErro(e));
      }
    });
  }

  /** O total antes do detalhe: quantos grupos e itens há para corrigir. */
  resumo(): string {
    const partes: string[] = [];
    const n = (q: number, um: string, varios: string) => `${q} ${q === 1 ? um : varios}`;
    if (this.tipos().length) partes.push(n(this.tipos().length, 'ativo com o tipo errado', 'ativos com o tipo errado'));
    if (this.cotas().length) partes.push(n(this.cotas().length, 'ativo com cotas diferentes da corretora', 'ativos com cotas diferentes da corretora'));
    if (this.trocos().length) partes.push(n(this.trocos().length, 'troco ou venda contando como rendimento', 'trocos ou vendas contando como rendimento'));
    if (this.rendimentos().length) partes.push(n(this.rendimentos().length, 'rendimento contando como investimento', 'rendimentos contando como investimento'));
    if (!partes.length) return 'Tudo certo: tipos, cotas e créditos da corretora batem com a carteira.';
    return `Encontrado: ${partes.join(', ')}.`;
  }

  faltam(c: CotasDiferentes): number {
    return c.naCorretora - c.naCarteira;
  }

  /** Débitos de compra sem operação até a data da posição da corretora. */
  debitosPara(c: CotasDiferentes): CompraSemOperacao[] {
    const ate = c.posicaoEm.slice(0, 10);
    return (this.dados()?.comprasSemOperacao ?? []).filter((d) => d.data <= ate);
  }

  rotuloDebito(d: CompraSemOperacao): string {
    const data = new Date(d.data + 'T12:00:00').toLocaleDateString('pt-BR');
    if (this.privacidade.oculto()) return `${data} · ${d.conta}`;
    const reais = (v: number) => v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
    return `${data} · ${reais(-d.valor)} · ${d.conta}${d.troco > 0 ? ` (troco ${reais(d.troco)})` : ''}`;
  }

  /** Preço = (débito − troco) ÷ cotas, ao trocar o débito ou as cotas. */
  recalcular(l: LinhaCotas): void {
    const d = (this.dados()?.comprasSemOperacao ?? []).find((x) => x.transacaoId === l.debitoId);
    if (!d || !l.quantidade || l.quantidade <= 0) return;
    l.preco = Math.round(((-d.valor - d.troco) / l.quantidade) * 10000) / 10000;
  }

  origem(c: CreditoDaCorretora): string {
    const data = (iso: string) => new Date(iso + 'T12:00:00').toLocaleDateString('pt-BR');
    if (c.vendaTicker && c.vendaData) return `venda de ${this.qtd(c.vendaQuantidade ?? 0)} ${c.vendaTicker} em ${data(c.vendaData)}`;
    if (c.compraData && c.compraValor !== null) {
      const valor = this.privacidade.oculto() ? '' : ` de ${(-c.compraValor).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })}`;
      return `troco da compra${valor} em ${data(c.compraData)}`;
    }
    return 'crédito da corretora';
  }

  nomeTipo(t: TipoParaCorrigir): string {
    return (NOMES_TIPO_ATIVO[t.tipoAtual] ?? 'outro').toLowerCase();
  }

  qtd(valor: number): string {
    if (this.privacidade.oculto()) return '•••';
    return valor.toLocaleString('pt-BR', { maximumFractionDigits: 6 });
  }

  total(linhas: Marcavel<{ valor: number }>[]): number {
    return linhas.reduce((s, l) => s + l.item.valor, 0);
  }

  visiveis<T>(linhas: Marcavel<T>[], abertas: boolean): Marcavel<T>[] {
    return abertas ? linhas : linhas.slice(0, LINHAS_VISIVEIS);
  }

  marcarTodos(linhas: Marcavel<unknown>[], marcada: boolean): void {
    if (linhas === this.trocos() && !this.dados()?.categoriaDoTroco) return;
    for (const l of linhas) l.marcada = marcada;
  }

  selecionadas(): number {
    return [...this.tipos(), ...this.cotas(), ...this.trocos(), ...this.rendimentos()].filter((l) => l.marcada).length;
  }

  corrigir(): void {
    const compras = this.cotas().filter((l) => l.marcada);
    const incompletas = compras.filter((l) => l.debitoId === null || !l.quantidade || l.quantidade <= 0 || !l.preco || l.preco <= 0);
    if (incompletas.length) {
      this.avisos.erro(`Escolha o débito e informe cotas e preço de ${incompletas.map((l) => l.item.ticker).join(', ')}.`);
      return;
    }
    const debitos = compras.map((l) => l.debitoId);
    if (new Set(debitos).size !== debitos.length) {
      this.avisos.erro('O mesmo débito foi escolhido para duas compras.');
      return;
    }
    this.salvando.set(true);
    this.api.corrigirInvestimentos({
      ativosParaFii: this.tipos().filter((l) => l.marcada).map((l) => l.item.ativoId),
      compras: compras.map((l) => ({ transacaoId: l.debitoId!, ticker: l.item.ticker, tipo: TipoOperacao.Compra,
        quantidade: l.quantidade!, precoUnitario: l.preco!, taxas: 0 })),
      trocos: this.trocos().filter((l) => l.marcada).map((l) => l.item.transacaoId),
      rendimentos: this.rendimentos().filter((l) => l.marcada).map((l) => l.item.transacaoId)
    }).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.avisar(r);
        this.corrigido.emit();
        this.fechar.emit();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** Aviso do que mudou, com "Desfazer" (a API guarda o estado anterior por alguns minutos). */
  private avisar(r: ResultadoCorrecaoInvestimentos): void {
    const partes: string[] = [];
    if (r.tipos) partes.push(`${r.tipos} ativo(s) agora FII`);
    if (r.compras) partes.push(`${r.compras} compra(s) lançada(s)`);
    if (r.trocos) partes.push(`${r.trocos} troco(s)/venda(s) em ${this.dados()?.categoriaDoTroco?.nome ?? 'Investimentos'}`);
    if (r.rendimentos) partes.push(`${r.rendimentos} rendimento(s) de volta à receita`);
    if (!r.desfazer || !partes.length) {
      this.avisos.info('Nada mudou: já estava tudo como sugerido.');
      return;
    }
    const chave = r.desfazer;
    this.avisos.sucesso(`Corrigido: ${partes.join(', ')}.`, {
      rotulo: 'Desfazer',
      executar: () => this.api.desfazerCorrecaoInvestimentos(chave).subscribe({
        next: (d) => {
          this.avisos.sucesso(`Desfeito: ${d.transacoesAtualizadas} mudança(s) voltaram como estavam.`);
          this.corrigido.emit();
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      })
    });
  }
}
