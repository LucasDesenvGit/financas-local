import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { HistoricoInvestimentos, MesInvestimentosOf } from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES_CURTOS, reaisOuNada } from '../compartilhado/utilidades';
import { TermoComponent } from '../compartilhado/termo.component';

/**
 * #173 e) (ADR 0045): rendimento de cada mês dos investimentos do Open Finance (caixinha, CDB, fundos), pelo histórico de
 * saldo que o site guarda a cada sincronização. Como o "Mês a mês" dos FIIs: a frase com o último mês antes da tabela
 * (Shneiderman: overview first), o mês mais recente primeiro e o detalhe por investimento sob demanda.
 *
 * #331: o mês mais recente já abre com o detalhe; cada investimento numa linha, do que mais rendeu ao que menos, com
 * uma barrinha proporcional (como "Gasto por categoria" em Relatórios) no lugar da linha corrida; "sem o valor aplicado"
 * vira etiqueta; e a explicação longa vai para um ⓘ do glossário (Nielsen 8; Progressive Disclosure).
 */
@Component({
  selector: 'app-rendimento-investimentos-of',
  imports: [DatePipe, MoedaPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="painel-bloco rendimento-of" aria-labelledby="titulo-rendimento-of">
      <h2 id="titulo-rendimento-of">Rendimento por mês</h2>
      @if (erro(); as e) {
        <p class="vazio" role="alert">{{ e }}</p>
      } @else if (dados(); as d) {
        @if (!d.meses.length) {
          <p class="texto-suave">O Open Finance manda só o saldo de hoje. A partir da próxima sincronização, o site guarda o
            saldo de cada investimento todo dia, e o rendimento de cada mês aparece aqui.</p>
        } @else {
          <p class="resumo-rendimento-of" role="status">{{ resumo() }}</p>
          <div class="tabela-rolagem">
            <table class="tabela tabela-rendimento-of">
              <caption class="so-leitor">Rendimento e saldo dos investimentos do Open Finance em cada mês, do mais recente ao mais antigo</caption>
              <thead>
                <tr><th scope="col">Mês</th><th scope="col" class="direita">Rendimento</th><th scope="col" class="direita">Saldo no fim</th></tr>
              </thead>
              <tbody>
                @for (m of mesesRecentes(); track m.ano * 100 + m.mes; let primeiro = $first) {
                  <tr>
                    <th scope="row">{{ rotuloMes(m) }}
                      @if (m.desde) { <br><small class="texto-suave">desde {{ m.desde | date: 'dd/MM' }}</small> }
                      @else if (primeiro && mesAtual(m)) { <br><small class="texto-suave">até hoje</small> }</th>
                    <td class="direita" [class.positivo]="(m.rendimento ?? 0) > 0" [class.negativo]="(m.rendimento ?? 0) < 0">
                      {{ m.rendimento !== null ? (m.rendimento | currency) : '—' }}
                      @if (m.porInvestimento.length > 1) {
                        <br><button type="button" class="link link-mes" [attr.aria-expanded]="aberto(m)"
                          [attr.aria-controls]="'rend-of-' + m.ano + '-' + m.mes" (click)="alternar(m)"
                          [attr.aria-label]="'Rendimento por investimento em ' + rotuloMes(m)">por investimento</button>
                      }</td>
                    <td class="direita">{{ m.saldo | currency }}</td>
                  </tr>
                  @if (aberto(m)) {
                    <tr class="linha-por-investimento" [id]="'rend-of-' + m.ano + '-' + m.mes">
                      <td colspan="3">
                        <ul class="lista-por-investimento" [attr.aria-label]="'Rendimento por investimento em ' + rotuloMes(m)">
                          @for (i of ordenados(m); track i.nome) {
                            <li>
                              <span class="nome-investimento">{{ i.nome }}<span class="so-leitor">: </span></span>
                              @if (i.rendimento !== null) {
                                <strong class="valor-investimento" [class.positivo]="i.rendimento > 0" [class.negativo]="i.rendimento < 0">{{ i.rendimento | currency }}</strong>
                                <!-- A barra é decoração: o valor está escrito ao lado. -->
                                <span class="trilho-investimento" aria-hidden="true"><span [style.width.%]="largura(m, i.rendimento)"
                                  [class.negativa]="i.rendimento < 0"></span></span>
                              } @else {
                                <span class="etiqueta etiqueta-suave">sem o valor aplicado</span>
                              }
                            </li>
                          }
                        </ul>
                      </td>
                    </tr>
                  }
                }
              </tbody>
            </table>
          </div>
          <p class="texto-suave nota">
            O histórico começou em {{ d.desde | date: 'dd/MM/yyyy' }}. Valores brutos, antes do IR.
            <app-termo chave="rendimentoOpenFinance">Como o rendimento é calculado</app-termo>
          </p>
        }
      }
    </section>
  `,
  styles: `
    .rendimento-of h2 { margin: 0 0 6px; }
    .resumo-rendimento-of { margin: 0 0 10px; }
    /* #331: uma linha por investimento: o nome e o valor em cima, a barra proporcional embaixo. */
    .lista-por-investimento { list-style: none; margin: 0; padding: 6px 0 8px; display: grid; gap: 10px; font-size: 0.88rem; }
    .lista-por-investimento li { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 4px 12px; }
    .nome-investimento { min-width: 0; overflow-wrap: anywhere; }
    .valor-investimento { font-variant-numeric: tabular-nums; }
    .trilho-investimento { grid-column: 1 / -1; height: 6px; border-radius: 99px; background: var(--superficie-3); overflow: hidden; }
    .trilho-investimento > span { display: block; height: 100%; border-radius: 99px; background: var(--positivo); }
    .trilho-investimento > span.negativa { background: var(--negativo); }
    .link-mes { display: inline-flex; align-items: center; min-height: 24px; font-size: 0.85rem; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
  `
})
export class RendimentoInvestimentosOfComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly dados = signal<HistoricoInvestimentos | null>(null);
  readonly erro = signal<string | null>(null);
  readonly abertos = signal<ReadonlySet<string>>(new Set());

  /** Na tabela, o mês mais recente primeiro (como no app). */
  readonly mesesRecentes = computed(() => [...(this.dados()?.meses ?? [])].reverse());

  /** A conclusão do último mês, sem R$ quando "ocultar valores" está ligado (UX-9). */
  readonly resumo = computed(() => {
    const m = this.dados()?.meses.at(-1);
    if (!m) return '';
    const quando = `Em ${this.rotuloMes(m)}${m.desde ? ` (desde ${m.desde.slice(8, 10)}/${m.desde.slice(5, 7)})` : this.mesAtual(m) ? ' (até hoje)' : ''}`;
    if (m.rendimento === null) return `${quando}, os investimentos não informam o valor aplicado: só o saldo.`;
    const valor = reaisOuNada(Math.abs(m.rendimento), this.privacidade.oculto());
    if (!valor) return `${quando}, os investimentos ${m.rendimento >= 0 ? 'renderam' : 'perderam'}.`;
    return `${quando}, os investimentos ${m.rendimento >= 0 ? 'renderam' : 'perderam'} ${valor}.`;
  });

  ngOnInit(): void {
    this.api.historicoInvestimentos().subscribe({
      next: (d) => {
        this.dados.set(d);
        // #331: o mês mais recente já abre com o detalhe por investimento.
        const ultimo = d.meses.at(-1);
        if (ultimo && ultimo.porInvestimento.length > 1) this.abertos.set(new Set([`${ultimo.ano}-${ultimo.mes}`]));
      },
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }

  /** #331: do que mais rendeu ao que menos; os sem o valor aplicado no fim. */
  ordenados(m: MesInvestimentosOf): MesInvestimentosOf['porInvestimento'] {
    return [...m.porInvestimento].sort((a, b) => (b.rendimento ?? -Infinity) - (a.rendimento ?? -Infinity));
  }

  /** Largura da barra: o rendimento em relação ao maior do mês (em módulo), com um mínimo para aparecer. */
  largura(m: MesInvestimentosOf, rendimento: number): number {
    const maior = Math.max(...m.porInvestimento.map((i) => Math.abs(i.rendimento ?? 0)));
    return maior > 0 ? Math.max(2, Math.abs(rendimento) / maior * 100) : 0;
  }

  rotuloMes(m: { ano: number; mes: number }): string {
    return `${NOMES_MESES_CURTOS[m.mes - 1]}/${String(m.ano).slice(2)}`;
  }

  mesAtual(m: { ano: number; mes: number }): boolean {
    const hoje = new Date();
    return m.ano === hoje.getFullYear() && m.mes === hoje.getMonth() + 1;
  }

  aberto(m: MesInvestimentosOf): boolean {
    return this.abertos().has(`${m.ano}-${m.mes}`);
  }

  alternar(m: MesInvestimentosOf): void {
    const chave = `${m.ano}-${m.mes}`;
    this.abertos.update((s) => {
      const novo = new Set(s);
      if (!novo.delete(chave)) novo.add(chave);
      return novo;
    });
  }
}
