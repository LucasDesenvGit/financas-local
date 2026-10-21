import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { ApiService } from '../core/api.service';
import { CotacaoMoeda } from '../core/modelos';
import { TermoComponent } from '../compartilhado/termo.component';
import { dataIso } from '../compartilhado/utilidades';

/** As moedas do bloco, na ordem; o iene vai por 100 (1 iene vale poucos centavos). */
export const MOEDAS_CAMBIO: { codigo: string; nome: string; unidades: number; rotulo: string }[] = [
  { codigo: 'USD', nome: 'Dólar americano', unidades: 1, rotulo: '1 dólar' },
  { codigo: 'EUR', nome: 'Euro', unidades: 1, rotulo: '1 euro' },
  { codigo: 'JPY', nome: 'Iene japonês', unidades: 100, rotulo: '100 ienes' }
];

/** Cotação mais antiga que isto (em dias) ganha o aviso de que pode estar desatualizada. */
export const DIAS_COTACAO_ANTIGA = 4;

export interface LinhaCambio {
  codigo: string; nome: string; rotulo: string;
  /** Reais por `rotulo` (1 dólar, 1 euro, 100 ienes); null sem cotação guardada. */
  valor: number | null;
  data: string | null;
}

/**
 * #247: o câmbio do dia em Investimentos (dólar, euro e iene em reais), com a PTAX de compra que o site já guarda
 * (CambioService; GET /api/cambio/cotacoes, sem consultar a internet). Bloco compacto, de apoio à carteira
 * (Shneiderman, overview first). A data de cada cotação fica sempre à vista (Nielsen 1), e a PTAX tem o ⓘ do glossário
 * (Nielsen 2). Não segue "ocultar valores": a cotação é pública, não é dado seu (por isso o número sem o pipe de moeda).
 */
@Component({
  selector: 'app-investimentos-cambio',
  imports: [DatePipe, DecimalPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="painel-bloco bloco-cambio" aria-labelledby="titulo-cambio">
      <div class="bloco-titulo">
        <h2 id="titulo-cambio">Câmbio</h2>
        <small class="texto-suave">cotação <app-termo chave="ptax">PTAX</app-termo> de compra, do Banco Central</small>
      </div>
      @if (erro()) {
        <p class="texto-suave">Não deu para ler as cotações guardadas agora.</p>
      } @else if (linhas(); as lista) {
        <ul class="lista-simples lista-cambio">
          @for (l of lista; track l.codigo) {
            <li>
              <span class="codigo-moeda" aria-hidden="true">{{ l.codigo }}</span>
              <span class="cresce">{{ l.nome }} <small class="texto-suave">· {{ l.rotulo }}</small></span>
              @if (l.valor !== null) {
                <span class="valor">R$ {{ l.valor | number: '1.2-2' }}</span>
                <small class="texto-suave data-cambio">PTAX de {{ l.data | date: 'dd/MM/yyyy' }}</small>
              } @else {
                <small class="texto-suave">sem cotação guardada ainda</small>
              }
            </li>
          }
        </ul>
        @if (antiga()) {
          <p class="nota-discreta aviso-cambio">
            A última cotação guardada é de {{ antiga() | date: 'dd/MM' }}. O Banco Central só publica em dia útil; se faz
            mais tempo, o site pode ter ficado sem internet (a atualização é automática).
          </p>
        }
      } @else {
        <p class="texto-suave">Carregando…</p>
      }
    </section>
  `,
  styles: [`
    .bloco-cambio .bloco-titulo { flex-wrap: wrap; }
    .lista-cambio li { flex-wrap: wrap; }
    .codigo-moeda {
      font-size: 0.72rem; font-weight: 700; letter-spacing: 0.04em; padding: 2px 6px; border-radius: 6px;
      background: var(--superficie-2); color: var(--texto-suave); font-variant-numeric: tabular-nums;
    }
    .data-cambio { min-width: 9.5em; text-align: right; }
    /* Celular: a data desce para a linha de baixo, e o nome da moeda fica com a largura toda. */
    @media (max-width: 600px) {
      .data-cambio { flex-basis: 100%; min-width: 0; margin-top: -6px; padding-left: 3.4em; text-align: left; }
    }
  `]
})
export class InvestimentosCambioComponent {
  private readonly api = inject(ApiService);
  private readonly cotacoes = signal<CotacaoMoeda[] | null>(null);
  readonly erro = signal(false);
  /** Hoje (pode vir de fora nos testes). */
  hoje = dataIso();

  constructor() {
    this.api.cotacoesCambio().subscribe({
      next: (r) => this.cotacoes.set(r.cotacoes),
      error: () => this.erro.set(true)
    });
  }

  readonly linhas = computed<LinhaCambio[] | null>(() => {
    const lista = this.cotacoes();
    if (!lista) return null;
    return MOEDAS_CAMBIO.map((m) => {
      const c = lista.find((x) => x.moeda === m.codigo);
      return { codigo: m.codigo, nome: m.nome, rotulo: m.rotulo, valor: c ? c.compra * m.unidades : null, data: c?.data ?? null };
    });
  });

  /** A data da cotação mais recente, quando ela tem mais de alguns dias (fim de semana e feriado não contam como antiga). */
  readonly antiga = computed<string | null>(() => {
    const datas = (this.linhas() ?? []).map((l) => l.data).filter((d): d is string => d !== null).sort();
    const ultima = datas.at(-1);
    if (!ultima) return null;
    const dias = (Date.parse(this.hoje) - Date.parse(ultima)) / 86_400_000;
    return dias > DIAS_COTACAO_ANTIGA ? ultima : null;
  });
}
