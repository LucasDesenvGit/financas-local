import { AfterViewInit, Component, ElementRef, OnDestroy, effect, inject, input, viewChild } from '@angular/core';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { PrivacidadeService } from '../core/privacidade.service';
import { TemaService } from '../core/tema.service';

Chart.register(...registerables);

/** Envoltório mínimo do Chart.js (renderiza em canvas, compatível com a CSP do site). */
@Component({
  selector: 'app-grafico',
  template: `<div class="grafico" [style.height.px]="altura()"><canvas #canvas></canvas></div>`
})
export class GraficoComponent implements AfterViewInit, OnDestroy {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly configuracao = input.required<ChartConfiguration<any>>();
  readonly altura = input(260);

  private readonly canvas = viewChild.required<ElementRef<HTMLCanvasElement>>('canvas');
  private grafico: Chart | null = null;
  private pronto = false;
  private readonly tema = inject(TemaService);
  private readonly privacidade = inject(PrivacidadeService);

  constructor() {
    effect(() => {
      const cfg = this.configuracao();
      this.tema.tema(); // redesenha ao trocar claro/escuro
      this.privacidade.oculto(); // e ao ocultar/mostrar valores
      if (this.pronto) this.desenhar(cfg);
    });
  }

  ngAfterViewInit(): void {
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
    this.grafico = new Chart(this.canvas().nativeElement, { ...cfg, options: opcoes } as ChartConfiguration);
  }
}
