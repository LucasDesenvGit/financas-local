import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { ApiService } from '../core/api.service';
import { CotacaoCripto } from '../core/modelos';
import { TermoComponent } from '../compartilhado/termo.component';

/**
 * As criptomoedas do bloco, na ordem (as mesmas de CriptoService.Moedas na API). O ícone é um arquivo do próprio site
 * em public/cripto (simple-icons, CC0: o desenho da marca em branco sobre a cor dela).
 */
export const MOEDAS_CRIPTO: { codigo: string; nome: string; icone: string }[] = [
  { codigo: 'BTC', nome: 'Bitcoin', icone: 'btc' },
  { codigo: 'ETH', nome: 'Ethereum', icone: 'eth' },
  { codigo: 'SOL', nome: 'Solana', icone: 'sol' },
  { codigo: 'USDT', nome: 'Tether (dólar digital)', icone: 'usdt' }
];

/** Cotação mais antiga que isto (em horas) ganha o aviso: a API atualiza a cada 30 minutos, 24 h por dia. */
export const HORAS_COTACAO_CRIPTO_ANTIGA = 2;

export interface LinhaCripto {
  codigo: string; nome: string; icone: string;
  /** Reais por 1 unidade; null sem cotação guardada. */
  preco: number | null;
  dataHora: string | null;
  variacao24h: number | null;
}

/**
 * #279 (ADR 0048): a cotação de criptomoedas em reais em Investimentos, ao lado do Câmbio. O último negócio no Mercado
 * Bitcoin, que a API guarda a cada 30 minutos (GET /api/cripto/cotacoes, sem consultar a internet na hora).
 * - Data e hora sempre à vista (Nielsen 1): cripto muda o tempo todo, inclusive no fim de semana.
 * - Variação em 24 h com sinal e cor, e o sinal também em texto (a cor não é o único jeito de ler; WCAG 1.4.1).
 * - "Criptomoeda" com ⓘ do glossário (Nielsen 2); a fonte dita no bloco, e o aviso de conferir na corretora.
 * - Não segue "ocultar valores": a cotação é pública, não é dado seu (como o câmbio).
 */
@Component({
  selector: 'app-investimentos-cripto',
  imports: [DatePipe, DecimalPipe, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="painel-bloco bloco-cripto" aria-labelledby="titulo-cripto">
      <div class="bloco-titulo">
        <h2 id="titulo-cripto">Criptomoedas</h2>
        <small class="texto-suave">cotação de <app-termo chave="criptomoeda">criptomoedas</app-termo> em reais, último negócio no Mercado Bitcoin</small>
      </div>
      @if (erro()) {
        <p class="texto-suave">Não deu para ler as cotações guardadas agora.</p>
      } @else if (linhas(); as lista) {
        <ul class="lista-simples lista-cripto">
          @for (l of lista; track l.codigo) {
            <li>
              <!-- O ícone é decoração: o nome está escrito ao lado (alt vazio, WCAG 1.1.1). -->
              <span class="moeda-identificacao" aria-hidden="true">
                <span class="codigo-cripto">{{ l.codigo }}</span>
                <img class="icone-cripto" [src]="'cripto/' + l.icone + '.svg'" alt="" width="20" height="20" />
              </span>
              <span class="cresce">{{ l.nome }}</span>
              @if (l.preco !== null) {
                <span class="valor">R$ {{ l.preco | number: '1.2-2' }}</span>
                @if (l.variacao24h !== null) {
                  <small class="variacao-cripto" [class.positivo]="l.variacao24h > 0" [class.negativo]="l.variacao24h < 0">
                    {{ l.variacao24h > 0 ? '+' : (l.variacao24h < 0 ? '−' : '') }}{{ abs(l.variacao24h) | number: '1.2-2' }}% em 24 h
                  </small>
                } @else {
                  <small class="texto-suave variacao-cripto">variação em 24 h ainda sem dado</small>
                }
                <small class="texto-suave data-cripto">{{ l.dataHora | date: 'dd/MM HH:mm' }}</small>
              } @else {
                <small class="texto-suave data-cripto sem-cotacao">sem cotação guardada ainda</small>
              }
            </li>
          }
        </ul>
        @if (antiga(); as quando) {
          <p class="nota-discreta aviso-cripto">
            A última cotação guardada é de {{ quando | date: 'dd/MM HH:mm' }}. A atualização é automática, a cada
            30 minutos, com o site ligado e com internet.
          </p>
        }
        <p class="nota-discreta nota-cripto">Preço de referência de uma corretora; cripto varia muito. Confira na sua corretora antes de comprar ou vender.</p>
      } @else {
        <p class="texto-suave">Carregando…</p>
      }
    </section>
  `,
  styles: [`
    .bloco-cripto .bloco-titulo { flex-wrap: wrap; }
    .lista-cripto li { flex-wrap: wrap; }
    .codigo-cripto {
      font-size: 0.72rem; font-weight: 700; letter-spacing: 0.04em; padding: 2px 6px; border-radius: 6px;
      background: var(--superficie-2); color: var(--texto-suave); font-variant-numeric: tabular-nums;
      min-width: 4.2em; text-align: center; box-sizing: border-box;
    }
    .moeda-identificacao { display: inline-flex; align-items: center; gap: 6px; flex-shrink: 0; }
    .icone-cripto { display: block; width: 20px; height: 20px; border-radius: 5px; }
    .variacao-cripto { min-width: 8.5em; text-align: right; font-variant-numeric: tabular-nums; }
    .data-cripto { min-width: 6.5em; text-align: right; }
    .nota-cripto { margin-top: 10px; }
    /* Celular: duas linhas em grade. Em cima o código, o nome e o preço; embaixo, a variação e a data e hora. */
    @media (max-width: 600px) {
      .lista-cripto li { display: grid; grid-template-columns: auto 1fr auto; column-gap: 10px; row-gap: 2px; align-items: center; }
      .moeda-identificacao { grid-row: 1 / span 2; align-self: start; margin-top: 2px; }
      .lista-cripto .cresce { grid-column: 2; min-width: 0; }
      .lista-cripto .valor { grid-column: 3; text-align: right; }
      .variacao-cripto { grid-column: 2; min-width: 0; text-align: left; }
      .data-cripto { grid-column: 3; min-width: 0; text-align: right; }
      .data-cripto.sem-cotacao { grid-column: 2 / span 2; text-align: left; }
    }
  `]
})
export class InvestimentosCriptoComponent {
  private readonly api = inject(ApiService);
  private readonly cotacoes = signal<CotacaoCripto[] | null>(null);
  readonly erro = signal(false);
  /** Agora (pode vir de fora nos testes). */
  agora = Date.now();
  readonly abs = Math.abs;

  constructor() {
    this.api.cotacoesCripto().subscribe({
      next: (r) => this.cotacoes.set(r.cotacoes),
      error: () => this.erro.set(true)
    });
  }

  readonly linhas = computed<LinhaCripto[] | null>(() => {
    const lista = this.cotacoes();
    if (!lista) return null;
    return MOEDAS_CRIPTO.map((m) => {
      const c = lista.find((x) => x.moeda === m.codigo);
      return { codigo: m.codigo, nome: m.nome, icone: m.icone, preco: c?.preco ?? null, dataHora: c?.dataHora ?? null, variacao24h: c?.variacao24h ?? null };
    });
  });

  /** A data e hora da cotação mais recente, quando ela tem mais de 2 horas (a API atualiza a cada 30 minutos). */
  readonly antiga = computed<string | null>(() => {
    const datas = (this.linhas() ?? []).map((l) => l.dataHora).filter((d): d is string => d !== null).sort();
    const ultima = datas.at(-1);
    if (!ultima) return null;
    return this.agora - Date.parse(ultima) > HORAS_COTACAO_CRIPTO_ANTIGA * 3_600_000 ? ultima : null;
  });
}
