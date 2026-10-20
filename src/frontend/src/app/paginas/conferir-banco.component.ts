import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { CartaoConferencia, ConferenciaBanco, PistaConferencia } from '../core/modelos';
import { CampoValorComponent } from '../compartilhado/campo-valor.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { Comparacao, comparar } from './conferir-banco';

interface LinhaConferencia {
  chave: string;
  nome: string;
  detalhe: string | null;
  site: number;
  pistas: PistaConferencia[];
}

interface GrupoConferencia {
  id: string;
  titulo: string;
  coluna: string;
  valorSite: string;
  linhas: LinhaConferencia[];
}

const NOMES_SITUACAO: Record<string, string> = { paga: 'paga', fechada: 'fechada', aberta: 'em aberto' };

/**
 * #172: "Conferir com o banco". O usuário digita o que o app de cada banco mostra; a tela compara com o valor do site e
 * mostra as pistas do que pode explicar a diferença, a de mesmo valor primeiro. É a verificação feita à mão em
 * 19/10/2026, agora dentro do produto.
 * - Shneiderman: a resposta ("bate" ou quanto a mais) primeiro; as pistas, o detalhe, só quando não bate.
 * - Nielsen 1 e 9: a diferença diz o que fazer, com o link "ver" para a tela que mostra o detalhe.
 * - Os valores das pistas passam pelo pipe de moeda, que respeita "ocultar valores". Nada é gravado.
 */
@Component({
  selector: 'app-conferir-banco',
  imports: [FormsModule, RouterLink, CampoValorComponent, MoedaPipe],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina">
      <h1>Conferir com o banco</h1>
    </header>
    <p class="texto-suave introducao">
      Abra o app de cada banco e digite o valor que aparece lá. O site compara com o valor que ele usa e mostra o que pode
      explicar a diferença, com a pista mais provável primeiro. Nada é gravado: é só para conferir.
    </p>

    @if (erro()) { <p class="erro">{{ erro() }}</p> }
    @if (dados()) {
      <p class="resumo" aria-live="polite">{{ resumo() }}</p>
      @for (g of grupos(); track g.id) {
        <section class="painel-bloco" [attr.aria-labelledby]="'titulo-' + g.id">
          <div class="bloco-titulo"><h2 [id]="'titulo-' + g.id">{{ g.titulo }}</h2></div>
          <div class="tabela-rolagem">
            <table class="tabela">
              <thead>
                <tr><th>{{ g.coluna }}</th><th class="direita">{{ g.valorSite }}</th><th>No app do banco</th><th>Resultado</th></tr>
              </thead>
              <tbody>
                @for (l of g.linhas; track l.chave) {
                  <tr [id]="l.chave">
                    <td><strong>{{ l.nome }}</strong>
                      @if (l.detalhe) { <br><small class="texto-suave">{{ l.detalhe }}</small> }</td>
                    <td class="direita valor nowrap">{{ l.site | currency }}</td>
                    <td class="campo-app">
                      <app-campo-valor compacto negativo [name]="'app-' + l.chave" [rotulo]="'Valor no app: ' + l.nome"
                        [ngModel]="digitados()[l.chave] ?? null" (ngModelChange)="digitar(l.chave, $event)" />
                    </td>
                    <td class="resultado">
                      @if (resultado(l); as r) {
                        @if (r.bate) {
                          <span class="etiqueta etiqueta-sucesso">bate</span>
                        } @else {
                          <span class="etiqueta etiqueta-alerta">{{ r.diferenca > 0 ? 'o app mostra' : 'o site mostra' }}
                            {{ absoluto(r.diferenca) | currency }} a mais</span>
                          <ul class="pistas">
                            @for (p of r.pistas; track p.texto) {
                              <li [class.explica]="p.explica">
                                @if (p.explica) { <strong>Explica a diferença: </strong> }
                                {{ p.texto }}@if (p.valor != null) {: <span class="nowrap">{{ p.valor | currency }}</span> }
                                @if (p.rota) { <a [routerLink]="p.rota" [queryParams]="p.parametros ?? {}">ver</a> }
                              </li>
                            }
                          </ul>
                        }
                      } @else {
                        <small class="texto-suave">digite o valor do app</small>
                      }
                    </td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
        </section>
      } @empty {
        <p class="vazio">Nenhuma conta, cartão ou investimento para conferir.</p>
      }
    } @else if (!erro()) {
      <p class="vazio">Carregando…</p>
    }
  `,
  styles: `
    .introducao { max-width: 72ch; }
    .resumo { font-weight: 600; margin: 12px 0; }
    .campo-app { min-width: 150px; }
    .resultado { min-width: 260px; }
    .pistas { margin: 6px 0 0; padding-left: 18px; font-size: 0.88rem; }
    .pistas li { margin: 2px 0; }
    .pistas li.explica { font-weight: 600; }
    .pistas a { margin-left: 4px; }
  `
})
export class ConferirBancoComponent implements OnInit {
  private readonly api = inject(ApiService);

  readonly dados = signal<ConferenciaBanco | null>(null);
  readonly erro = signal<string | null>(null);
  /** Valor digitado (do app do banco) de cada linha. Só na tela. */
  readonly digitados = signal<Record<string, number | null>>({});

  readonly grupos = computed<GrupoConferencia[]>(() => {
    const d = this.dados();
    if (!d) return [];
    const grupos: GrupoConferencia[] = [
      {
        id: 'contas', titulo: 'Contas', coluna: 'Conta', valorSite: 'Saldo no site',
        linhas: d.contas.map((c) => ({ chave: `conta-${c.id}`, nome: c.nome, detalhe: c.openFinance ? 'Open Finance' : 'manual',
          site: c.saldo, pistas: c.pistas }))
      },
      {
        id: 'cartoes', titulo: 'Faturas do mês', coluna: 'Cartão', valorSite: 'Fatura no site',
        linhas: d.cartoes.map((c) => ({ chave: `cartao-${c.id}`, nome: c.nome, detalhe: detalheFatura(c), site: c.faturaMes,
          pistas: c.pistas }))
      },
      {
        id: 'investimentos', titulo: 'Investimentos', coluna: 'Investimento', valorSite: 'Valor no site',
        linhas: d.investimentos.map((i) => ({ chave: i.chave, nome: i.nome, detalhe: i.origem, site: i.saldo, pistas: i.pistas }))
      }
    ];
    return grupos.filter((g) => g.linhas.length > 0);
  });

  /** "N de M conferidos: X batem, Y com diferença." */
  readonly resumo = computed(() => {
    const linhas = this.grupos().flatMap((g) => g.linhas);
    const resultados = linhas.map((l) => this.resultado(l)).filter((r): r is Comparacao => r !== null);
    if (resultados.length === 0) return `${linhas.length} item(ns) para conferir.`;
    const batem = resultados.filter((r) => r.bate).length;
    return `${resultados.length} de ${linhas.length} conferido(s): ${batem} bate(m), ${resultados.length - batem} com diferença.`;
  });

  ngOnInit(): void {
    this.api.conferenciaBanco().subscribe({
      next: (d) => this.dados.set(d),
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }

  digitar(chave: string, valor: number | null): void {
    this.digitados.update((v) => ({ ...v, [chave]: valor }));
  }

  resultado(l: LinhaConferencia): Comparacao | null {
    return comparar(l.site, this.digitados()[l.chave], l.pistas);
  }

  absoluto(valor: number): number {
    return Math.abs(valor);
  }
}

/** "em aberto · vence 10/11". */
function detalheFatura(c: CartaoConferencia): string | null {
  const partes: string[] = [];
  if (c.situacao) partes.push(NOMES_SITUACAO[c.situacao] ?? c.situacao);
  if (c.vencimento) partes.push(`vence ${c.vencimento.slice(8, 10)}/${c.vencimento.slice(5, 7)}`);
  return partes.length ? partes.join(' · ') : null;
}
