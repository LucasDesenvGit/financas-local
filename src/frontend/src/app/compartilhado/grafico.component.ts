import { AfterViewInit, Component, ElementRef, EventEmitter, OnDestroy, Output, effect, inject, input, viewChild, ChangeDetectionStrategy } from '@angular/core';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { PrivacidadeService } from '../core/privacidade.service';
import { TemaService } from '../core/tema.service';

Chart.register(...registerables);

/** Envoltório mínimo do Chart.js (renderiza em canvas, compatível com a CSP do site). */
@Component({
  selector: 'app-grafico',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `<div class="grafico" [style.height.px]="altura()"><canvas #canvas></canvas></div>`
})
export class GraficoComponent implements AfterViewInit, OnDestroy {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly configuracao = input.required<ChartConfiguration<any>>();
  readonly altura = input(260);
  /** Linha extra no tooltip quando o gráfico é clicável (ex.: "Clique para ver as transações"). */
  readonly dicaClique = input<string | null>(null);
  /**
   * Índice do item clicado (fatia, barra, ponto). Opcional: só quando há quem ouça o gráfico fica clicável (cursor de mão
   * e dica no tooltip); os demais gráficos não mudam. O clique no canvas é atalho a mais: a tela que usa precisa ter o
   * mesmo caminho em HTML (WCAG 2.1.1). Em toque, o primeiro toque mostra o tooltip e não navega (não existe hover).
   */
  @Output() readonly itemClicado = new EventEmitter<number>();
  /** Como itemClicado, com a série (dataset) do item: para barras empilhadas, em que cada pedaço da barra é uma série. */
  @Output() readonly serieClicada = new EventEmitter<{ indice: number; serie: number }>();
  /** Quais itens são clicáveis (cursor, dica e clique); padrão: todos. Ex.: só a barra "Você", não a de referência. */
  readonly itemClicavel = input<(indice: number, serie: number) => boolean>(() => true);
  /** Tipo do último ponteiro sobre o canvas ("mouse", "pen" ou "touch"). */
  private ultimoPonteiro = 'mouse';

  private readonly canvas = viewChild.required<ElementRef<HTMLCanvasElement>>('canvas');
  private grafico: Chart | null = null;
  private pronto = false;
  private readonly tema = inject(TemaService);
  private readonly privacidade = inject(PrivacidadeService);

  constructor() {
    effect(() => {
      const cfg = this.configuracao();
      this.tema.aparencia(); // redesenha ao trocar claro/escuro ou a paleta de cores
      this.privacidade.oculto(); // e ao ocultar/mostrar valores
      if (this.pronto) this.desenhar(cfg);
    });
  }

  ngAfterViewInit(): void {
    this.canvas().nativeElement.addEventListener('pointerdown', (e) => (this.ultimoPonteiro = e.pointerType));
    this.pronto = true;
    this.desenhar(this.configuracao());
  }

  ngOnDestroy(): void {
    this.grafico?.destroy();
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private desenhar(cfg: ChartConfiguration<any>): void {
    this.grafico?.destroy();
    const estilo = getComputedStyle(document.documentElement);
    Chart.defaults.color = estilo.getPropertyValue('--texto-suave').trim() || '#6b7280';
    Chart.defaults.borderColor = estilo.getPropertyValue('--linha').trim() || '#e5e7eb';
    Chart.defaults.font.family = estilo.getPropertyValue('--fonte').trim() || 'system-ui';
    Chart.defaults.plugins.legend.labels.usePointStyle = true;
    Chart.defaults.plugins.legend.labels.boxHeight = 8;
    Chart.defaults.plugins.tooltip.padding = 10;
    Chart.defaults.plugins.tooltip.cornerRadius = 10;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const opcoes: any = { responsive: true, maintainAspectRatio: false, ...(cfg.options ?? {}) };
    if (this.privacidade.oculto()) {
      // Sem valores: esconde tooltips e a escala de valores (o formato do gráfico continua visível).
      opcoes.plugins = { ...(opcoes.plugins ?? {}), tooltip: { enabled: false } };
      if (cfg.type !== 'doughnut' && cfg.type !== 'pie') {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const eixoCategorias = opcoes.indexAxis === 'y' ? 'y' : 'x';
        const eixoValores = eixoCategorias === 'y' ? 'x' : 'y';
        const escalas: Record<string, any> = { [eixoValores]: {}, ...(opcoes.scales ?? {}) };
        for (const chave of Object.keys(escalas)) {
          if (chave === eixoCategorias) continue;
          const e = escalas[chave] ?? {};
          escalas[chave] = { ...e, ticks: { ...(e.ticks ?? {}), display: false } };
        }
        opcoes.scales = escalas;
      }
    }
    if (this.itemClicado.observed || this.serieClicada.observed) this.tornarClicavel(opcoes);
    this.grafico = new Chart(this.canvas().nativeElement, { ...cfg, options: opcoes } as ChartConfiguration);
  }

  /** Religado a cada desenho: o gráfico é recriado ao trocar tema, paleta ou privacidade. */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private tornarClicavel(opcoes: any): void {
    const canvas = this.canvas().nativeElement;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const clicavel = (itens: any[]) => !!itens.length && this.itemClicavel()(itens[0].index, itens[0].datasetIndex);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    opcoes.onClick = (_: unknown, itens: any[]) => {
      if (this.ultimoPonteiro === 'touch' || !clicavel(itens)) return;
      this.itemClicado.emit(itens[0].index);
      this.serieClicada.emit({ indice: itens[0].index, serie: itens[0].datasetIndex });
    };
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    opcoes.onHover = (_: unknown, itens: any[]) => {
      canvas.style.cursor = clicavel(itens) && this.ultimoPonteiro !== 'touch' ? 'pointer' : 'default';
    };
    const dica = this.dicaClique();
    if (dica && opcoes.plugins?.tooltip?.enabled !== false) {
      const tooltip = opcoes.plugins?.tooltip ?? {};
      opcoes.plugins = {
        ...(opcoes.plugins ?? {}),
        tooltip: {
          ...tooltip,
          callbacks: {
            ...(tooltip.callbacks ?? {}),
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            footer: (itens: any[]) => (this.ultimoPonteiro === 'touch'
              || !this.itemClicavel()(itens[0]?.dataIndex, itens[0]?.datasetIndex) ? '' : dica)
          }
        }
      };
    }
  }
}
