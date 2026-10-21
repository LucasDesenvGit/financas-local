import { DatePipe, DecimalPipe, NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { ApiService } from '../core/api.service';
import { CotacaoMoeda } from '../core/modelos';
import { TermoComponent } from '../compartilhado/termo.component';
import { dataIso } from '../compartilhado/utilidades';

export interface MoedaCambio {
  codigo: string; nome: string; unidades: number; rotulo: string;
  /** Arquivo da bandeira em public/bandeiras (flag-icons, MIT). */
  bandeira: string;
  /** Fica à vista sempre; as outras ficam em "Mais moedas". */
  principal: boolean;
}

/**
 * As 10 moedas que o Banco Central publica na PTAX (o site já guarda todas; #278), na ordem. As três principais ficam
 * à vista e as outras atrás de "Mais moedas" (Nielsen, Progressive Disclosure). O iene vai por 100 (1 iene vale
 * poucos centavos).
 */
export const MOEDAS_CAMBIO: MoedaCambio[] = [
  { codigo: 'USD', nome: 'Dólar americano', unidades: 1, rotulo: '1 dólar', bandeira: 'us', principal: true },
  { codigo: 'EUR', nome: 'Euro', unidades: 1, rotulo: '1 euro', bandeira: 'eu', principal: true },
  { codigo: 'JPY', nome: 'Iene japonês', unidades: 100, rotulo: '100 ienes', bandeira: 'jp', principal: true },
  { codigo: 'GBP', nome: 'Libra esterlina', unidades: 1, rotulo: '1 libra', bandeira: 'gb', principal: false },
  { codigo: 'CHF', nome: 'Franco suíço', unidades: 1, rotulo: '1 franco', bandeira: 'ch', principal: false },
  { codigo: 'CAD', nome: 'Dólar canadense', unidades: 1, rotulo: '1 dólar', bandeira: 'ca', principal: false },
  { codigo: 'AUD', nome: 'Dólar australiano', unidades: 1, rotulo: '1 dólar', bandeira: 'au', principal: false },
  { codigo: 'DKK', nome: 'Coroa dinamarquesa', unidades: 1, rotulo: '1 coroa', bandeira: 'dk', principal: false },
  { codigo: 'NOK', nome: 'Coroa norueguesa', unidades: 1, rotulo: '1 coroa', bandeira: 'no', principal: false },
  { codigo: 'SEK', nome: 'Coroa sueca', unidades: 1, rotulo: '1 coroa', bandeira: 'se', principal: false }
];

/** Cotação mais antiga que isto (em dias) ganha o aviso de que pode estar desatualizada. */
export const DIAS_COTACAO_ANTIGA = 4;

/** Onde fica lembrado, neste navegador, se "Mais moedas" está aberto (ADR 0017). */
export const CHAVE_MAIS_MOEDAS = 'investimentos.cambio.maisMoedas';

export interface LinhaCambio {
  codigo: string; nome: string; rotulo: string; bandeira: string; principal: boolean;
  /** Reais por `rotulo` (1 dólar, 1 euro, 100 ienes); null sem cotação guardada. */
  valor: number | null;
  data: string | null;
}

/**
 * #247: o câmbio do dia em Investimentos, com a PTAX de compra que o site já guarda (CambioService;
 * GET /api/cambio/cotacoes, sem consultar a internet). Bloco compacto, de apoio à carteira (Shneiderman, overview
 * first). A data de cada cotação fica sempre à vista (Nielsen 1), e a PTAX tem o ⓘ do glossário (Nielsen 2). Não segue
 * "ocultar valores": a cotação é pública, não é dado seu (por isso o número sem o pipe de moeda).
 *
 * #278: bandeira de cada país à direita do código, para achar a moeda de olho (Nielsen 6); é decoração, porque o nome
 * está escrito ao lado (alt vazio, WCAG 1.1.1). As 10 moedas da PTAX: três à vista e as outras em "Mais moedas", um
 * botão no padrão Disclosure do WAI-ARIA (aria-expanded/aria-controls) com a escolha lembrada (ADR 0017).
 */
@Component({
  selector: 'app-investimentos-cambio',
  imports: [DatePipe, DecimalPipe, NgTemplateOutlet, TermoComponent],
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
          @for (l of principais(); track l.codigo) {
            <ng-container *ngTemplateOutlet="linha; context: { $implicit: l }" />
          }
        </ul>
        <!-- Sempre no HTML, escondida com hidden: o aria-controls do botão aponta para ela (padrão Disclosure). -->
        <ul class="lista-simples lista-cambio lista-cambio-mais" id="cambio-mais-moedas" [hidden]="!maisMoedasAberto()">
          @for (l of outras(); track l.codigo) {
            <ng-container *ngTemplateOutlet="linha; context: { $implicit: l }" />
          }
        </ul>
        <button type="button" class="link botao-mais-moedas" [attr.aria-expanded]="maisMoedasAberto()"
          aria-controls="cambio-mais-moedas" (click)="alternarMaisMoedas()">
          {{ maisMoedasAberto() ? 'Menos moedas' : 'Mais ' + outras().length + ' moedas' }}
          @if (!maisMoedasAberto()) { <small class="texto-suave">(libra, franco suíço, dólar canadense e outras)</small> }
        </button>
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

    <ng-template #linha let-l>
      <li>
        <span class="moeda-identificacao" aria-hidden="true">
          <span class="codigo-moeda">{{ l.codigo }}</span>
          <img class="bandeira" [src]="'bandeiras/' + l.bandeira + '.svg'" alt="" width="20" height="15" />
        </span>
        <span class="cresce">{{ l.nome }} <small class="texto-suave">· {{ l.rotulo }}</small></span>
        @if (l.valor !== null) {
          <span class="valor">R$ {{ l.valor | number: '1.2-2' }}</span>
          <small class="texto-suave data-cambio">PTAX de {{ l.data | date: 'dd/MM/yyyy' }}</small>
        } @else {
          <small class="texto-suave data-cambio">sem cotação guardada ainda</small>
        }
      </li>
    </ng-template>
  `,
  styles: [`
    .bloco-cambio .bloco-titulo { flex-wrap: wrap; }
    .lista-cambio li { flex-wrap: wrap; }
    .lista-cambio-mais { border-top: 1px solid var(--linha); }
    .lista-cambio-mais[hidden] { display: none; }
    .moeda-identificacao { display: inline-flex; align-items: center; gap: 6px; flex-shrink: 0; }
    .codigo-moeda {
      font-size: 0.72rem; font-weight: 700; letter-spacing: 0.04em; padding: 2px 6px; border-radius: 6px;
      background: var(--superficie-2); color: var(--texto-suave); font-variant-numeric: tabular-nums;
      min-width: 2.6em; text-align: center;
    }
    /* Borda suave: a bandeira branca (Japão) não some no tema claro. */
    .bandeira { display: block; width: 20px; height: 15px; border-radius: 3px; object-fit: cover;
      box-shadow: 0 0 0 1px var(--linha); }
    .botao-mais-moedas { margin-top: 6px; min-height: 24px; }
    .data-cambio { min-width: 9.5em; text-align: right; }
    /* Celular: a data (ou "sem cotação guardada ainda") desce para a linha de baixo, e o nome fica com a largura toda. */
    @media (max-width: 600px) {
      .data-cambio { flex-basis: 100%; min-width: 0; margin-top: -6px; padding-left: 5.6em; text-align: left; }
    }
  `]
})
export class InvestimentosCambioComponent {
  private readonly api = inject(ApiService);
  private readonly cotacoes = signal<CotacaoMoeda[] | null>(null);
  readonly erro = signal(false);
  readonly maisMoedasAberto = signal(lerMaisMoedas());
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
      return {
        codigo: m.codigo, nome: m.nome, rotulo: m.rotulo, bandeira: m.bandeira, principal: m.principal,
        valor: c ? c.compra * m.unidades : null, data: c?.data ?? null
      };
    });
  });
  readonly principais = computed(() => (this.linhas() ?? []).filter((l) => l.principal));
  readonly outras = computed(() => (this.linhas() ?? []).filter((l) => !l.principal));

  /** A data da cotação mais recente, quando ela tem mais de alguns dias (fim de semana e feriado não contam como antiga). */
  readonly antiga = computed<string | null>(() => {
    const datas = (this.linhas() ?? []).map((l) => l.data).filter((d): d is string => d !== null).sort();
    const ultima = datas.at(-1);
    if (!ultima) return null;
    const dias = (Date.parse(this.hoje) - Date.parse(ultima)) / 86_400_000;
    return dias > DIAS_COTACAO_ANTIGA ? ultima : null;
  });

  alternarMaisMoedas(): void {
    this.maisMoedasAberto.update((v) => !v);
    try {
      localStorage.setItem(CHAVE_MAIS_MOEDAS, this.maisMoedasAberto() ? '1' : '0');
    } catch { /* navegador sem armazenamento: só não lembra */ }
  }
}

function lerMaisMoedas(): boolean {
  try {
    return localStorage.getItem(CHAVE_MAIS_MOEDAS) === '1';
  } catch {
    return false;
  }
}
