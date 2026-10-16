import { DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { ItemSugestaoOrcamento, NaturezaGasto, SugestaoOrcamento } from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { AvisosService } from '../compartilhado/avisos.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES } from '../compartilhado/utilidades';

/** Linha editável: a sugestão da API + o que a pessoa marcou e digitou. */
interface LinhaSugestao {
  item: ItemSugestaoOrcamento;
  marcada: boolean;
  valor: number | null;
}

/** Fatia da barra de distribuição da renda (em % da renda). */
interface Fatia { classe: string; rotulo: string; percentual: number; referencia: string }

/**
 * #39 — sugestão de orçamentos por categoria, para revisar antes de gravar (Nielsen 3: nada é gravado sem
 * "Aplicar"; Nielsen 1: total e distribuição da renda se atualizam a cada valor digitado). A regra fica na API
 * (SugestaoOrcamentoService); aqui só a revisão. Categoria que já tem orçamento no mês vem desmarcada, para não
 * sobrescrever sem querer, e o total do mês conta o orçamento atual dela.
 */
@Component({
  selector: 'app-sugestao-orcamentos',
  imports: [FormsModule, MoedaPipe, DecimalPipe],
  template: `
    @if (erro()) {
      <p class="erro">{{ erro() }}</p>
    } @else if (!dados()) {
      <p class="vazio">Analisando os últimos meses…</p>
    } @else if (dados(); as d) {
      <p class="texto-suave introducao">
        Sugestão para <strong>{{ nomeMes() }}</strong> a partir dos seus últimos {{ d.mesesBase }} meses de gastos,
        da sua renda média e do uso do limite dos cartões. Revise e ajuste os valores: nada é gravado até você aplicar.
      </p>

      <section class="resumo-sugestao" aria-label="Resumo da renda">
        <div><span>Renda média</span><strong>{{ d.rendaMedia | currency }}</strong><small>{{ d.mesesRenda }} meses</small></div>
        <div><span>Parcelas fixas</span><strong>{{ parcelas() | currency }}</strong><small>cartão, financiamentos e empréstimos</small></div>
        <div><span>Meta de poupança</span><strong>{{ d.metaPoupanca | currency }}</strong><small>{{ d.percentualPoupanca }}% da renda</small></div>
        <div class="destaque"><span>Disponível para orçamentos</span><strong>{{ d.disponivel | currency }}</strong></div>
      </section>

      @if (d.cartoes.length) {
        <p class="linha-cartoes" [class.texto-alerta]="d.nivelCartao === 'atencao'" [class.negativo]="d.nivelCartao === 'alerta'">
          💳 Cartões: {{ d.usoLimiteCartoes | number: '1.0-0' }}% do limite em uso
          ({{ resumoCartoes() }})
          @if (d.nivelCartao !== 'ok') { · acima de {{ d.nivelCartao === 'alerta' ? d.limiteAlerta : d.limiteAtencao }}% }
        </p>
      }

      @if (d.avisos.length) {
        <ul class="avisos-sugestao">
          @for (a of d.avisos; track a) { <li>{{ a }}</li> }
        </ul>
      }

      @if (d.rendaMedia > 0) {
        <figure class="distribuicao" aria-labelledby="distribuicao-texto">
          <div class="barra-distribuicao" aria-hidden="true">
            @for (f of fatias(); track f.classe) {
              <span [class]="'fatia ' + f.classe" [style.width.%]="f.percentual"></span>
            }
          </div>
          <figcaption id="distribuicao-texto">
            @for (f of fatias(); track f.classe; let ultimo = $last) {
              <span class="legenda-fatia"><span [class]="'ponto ' + f.classe"></span>{{ f.rotulo }} {{ f.percentual | number: '1.0-0' }}%
                <small class="texto-suave">({{ f.referencia }})</small></span>@if (!ultimo) { <span aria-hidden="true"> · </span> }
            }
          </figcaption>
        </figure>
      }

      <div class="tabela-rolagem">
        <table class="tabela tabela-sugestao-orcamento">
          <thead>
            <tr>
              <th><span class="so-leitor">Aplicar</span></th><th>Categoria</th><th class="direita">Sugerido</th>
              <th class="direita">Atual</th><th class="direita">Mediana 6m</th><th class="direita">Último mês</th><th>Por quê</th>
            </tr>
          </thead>
          <tbody>
            @for (l of linhas(); track l.item.categoriaId) {
              <tr [class.sem-dados]="l.item.sugerido === null">
                <td><input type="checkbox" [(ngModel)]="l.marcada" (ngModelChange)="mudou()" [disabled]="l.item.sugerido === null"
                  [attr.aria-label]="'Aplicar orçamento de ' + l.item.nome"></td>
                <td class="nowrap"><span class="ponto" [style.background]="l.item.cor"></span>{{ l.item.nome }}
                  <br><small class="texto-suave">{{ rotuloNatureza(l.item.natureza) }}</small></td>
                <td class="direita">
                  @if (l.item.sugerido === null) {
                    <span class="texto-suave">—</span>
                  } @else {
                    <input type="number" min="0" step="10" class="input-compacto campo-sugerido" [class.campo-oculto]="privacidade.oculto()"
                      [(ngModel)]="l.valor" (ngModelChange)="mudou()" [attr.aria-label]="'Valor sugerido para ' + l.item.nome">
                  }
                </td>
                <td class="direita valor">{{ l.item.atual === null ? '—' : (l.item.atual | currency) }}</td>
                <td class="direita valor">{{ l.item.mediana | currency }}</td>
                <td class="direita valor nowrap">{{ l.item.ultimoMes | currency }}
                  @if (l.item.tendencia === 'alta') { <span class="negativo" title="em alta">↑</span><span class="so-leitor">em alta</span> }
                  @if (l.item.tendencia === 'queda') { <span class="positivo" title="em queda">↓</span><span class="so-leitor">em queda</span> }</td>
                <td class="motivos">{{ l.item.motivos.join('; ') || '—' }}</td>
              </tr>
            } @empty {
              <tr><td colspan="7" class="vazio">Nenhuma categoria de despesa com gasto nos últimos meses.</td></tr>
            }
          </tbody>
        </table>
      </div>

      <div class="rodape-sugestao">
        <p role="status" [class.negativo]="totalDoMes() > d.disponivel && d.rendaMedia > 0">
          Total do mês <strong>{{ totalDoMes() | currency }}</strong>
          @if (d.rendaMedia > 0) {
            de {{ d.disponivel | currency }} disponíveis
            {{ totalDoMes() > d.disponivel ? '— passou do disponível' : '✔' }}
          }
        </p>
        <div class="acoes">
          <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
          <button type="button" class="botao botao-primario" [disabled]="!marcadas().length || salvando()" (click)="aplicar()">
            Aplicar selecionados ({{ marcadas().length }})
          </button>
        </div>
      </div>
    }
  `,
  styles: `
    .introducao { margin-top: 0; }
    .resumo-sugestao { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; margin-bottom: 12px; }
    .resumo-sugestao div { display: grid; gap: 2px; padding: 10px 12px; border-radius: var(--raio-pequeno); background: var(--superficie-2); }
    .resumo-sugestao span, .resumo-sugestao small { font-size: 0.8rem; color: var(--texto-suave); }
    .resumo-sugestao .destaque { background: var(--primaria-suave); }
    .linha-cartoes { margin: 0 0 8px; font-size: 0.9rem; }
    .avisos-sugestao { margin: 0 0 12px; padding: 10px 12px 10px 28px; border-radius: var(--raio-pequeno);
      background: var(--alerta-suave); font-size: 0.88rem; display: grid; gap: 4px; }
    .distribuicao { margin: 0 0 14px; }
    .barra-distribuicao { display: flex; height: 14px; border-radius: 7px; overflow: hidden; background: var(--superficie-2); }
    .fatia { height: 100%; }
    figcaption { margin-top: 6px; font-size: 0.85rem; }
    .legenda-fatia { white-space: nowrap; }
    .ponto { display: inline-block; width: 9px; height: 9px; border-radius: 50%; margin-right: 5px; }
    .essencial { background: #92c8f0; }
    .estilo { background: #f3b3d3; }
    .sem-natureza { background: #a9b4c6; }
    .parcelas { background: #d9b45a; }
    .sobra { background: #8fd5b4; }
    .campo-sugerido { width: 110px; text-align: right; }
    .campo-oculto:not(:focus) { filter: blur(6px); }
    .motivos { font-size: 0.82rem; color: var(--texto-suave); min-width: 220px; }
    tr.sem-dados td { color: var(--texto-suave); }
    .rodape-sugestao { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 12px; margin-top: 14px; }
    .rodape-sugestao p { margin: 0; }
  `
})
export class SugestaoOrcamentosComponent implements OnInit {
  readonly ano = input.required<number>();
  readonly mes = input.required<number>();
  /** Emitido depois de gravar, com quantos orçamentos foram aplicados. */
  readonly aplicado = output<number>();
  readonly fechar = output<void>();

  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  readonly privacidade = inject(PrivacidadeService);

  readonly dados = signal<SugestaoOrcamento | null>(null);
  readonly erro = signal<string | null>(null);
  readonly linhas = signal<LinhaSugestao[]>([]);
  readonly salvando = signal(false);
  /** Muda a cada edição: as contas abaixo dependem de campos mutáveis das linhas (ngModel). */
  private readonly versao = signal(0);

  readonly nomeMes = computed(() => `${NOMES_MESES[this.mes() - 1]}/${this.ano()}`);
  readonly parcelas = computed(() => {
    const d = this.dados();
    return d ? d.parcelasCartao + d.parcelasFinanciamentos + d.parcelasEmprestimos : 0;
  });
  readonly resumoCartoes = computed(() =>
    (this.dados()?.cartoes ?? []).map((c) => `${c.nome} ${Math.round(c.percentual)}%`).join(', '));
  readonly marcadas = computed(() => {
    this.versao();
    return this.linhas().filter((l) => l.marcada && l.item.sugerido !== null && (l.valor ?? 0) > 0);
  });

  /** Valor que a categoria vai ter no mês: o digitado, se marcada; senão, o orçamento que já existe (ou nada). */
  private valorNoMes(l: LinhaSugestao): number {
    return l.marcada && l.item.sugerido !== null ? Number(l.valor) || 0 : l.item.atual ?? 0;
  }

  readonly totalDoMes = computed(() => {
    this.versao();
    return this.linhas().reduce((s, l) => s + this.valorNoMes(l), 0);
  });

  /** Distribuição da renda com os valores atuais da tabela (regra 50/30/20 como referência). */
  readonly fatias = computed<Fatia[]>(() => {
    this.versao();
    const d = this.dados();
    if (!d || d.rendaMedia <= 0) return [];
    const soma = (n: NaturezaGasto | null) =>
      this.linhas().filter((l) => l.item.natureza === n).reduce((s, l) => s + this.valorNoMes(l), 0);
    const pct = (v: number) => Math.max(0, (v / d.rendaMedia) * 100);
    const essenciais = soma(NaturezaGasto.Essencial);
    const estilo = soma(NaturezaGasto.EstiloDeVida);
    const semNatureza = soma(null);
    const sobra = d.rendaMedia - essenciais - estilo - semNatureza - this.parcelas();
    const fatias: Fatia[] = [
      { classe: 'essencial', rotulo: 'Essenciais', percentual: pct(essenciais), referencia: 'até 50%' },
      { classe: 'estilo', rotulo: 'Estilo de vida', percentual: pct(estilo), referencia: 'até 30%' },
      { classe: 'sem-natureza', rotulo: 'Sem classificação', percentual: pct(semNatureza), referencia: 'classifique em Categorias' },
      { classe: 'parcelas', rotulo: 'Parcelas', percentual: pct(this.parcelas()), referencia: 'gasto já contratado' },
      { classe: 'sobra', rotulo: 'Sobra para guardar', percentual: pct(sobra), referencia: 'pelo menos 20%' }
    ];
    return fatias.filter((f) => f.percentual > 0 || f.classe === 'sobra');
  });

  ngOnInit(): void {
    this.api.sugestaoOrcamentos(this.ano(), this.mes()).subscribe({
      next: (d) => {
        this.dados.set(d);
        this.linhas.set(d.itens.map((item) => ({ item, marcada: item.selecionado, valor: item.sugerido })));
      },
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }

  mudou(): void {
    this.versao.update((v) => v + 1);
  }

  rotuloNatureza(n: NaturezaGasto | null): string {
    return n === NaturezaGasto.Essencial ? 'essencial' : n === NaturezaGasto.EstiloDeVida ? 'estilo de vida' : 'sem classificação';
  }

  aplicar(): void {
    const itens = this.marcadas().map((l) => ({ categoriaId: l.item.categoriaId, valor: Number(l.valor) }));
    if (!itens.length) return;
    this.salvando.set(true);
    this.api.aplicarSugestaoOrcamentos(this.ano(), this.mes(), itens).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.aplicado.emit(r.transacoesAtualizadas);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }
}
