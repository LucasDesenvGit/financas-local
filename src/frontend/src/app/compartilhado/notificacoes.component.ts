import { ChangeDetectionStrategy, Component, ElementRef, HostListener, computed, inject, signal, viewChild } from '@angular/core';
import { NavigationEnd, Router, RouterLink } from '@angular/router';
import { filter } from 'rxjs';
import { ApiService } from '../core/api.service';
import { Avisos } from '../core/modelos';
import { PainelAvisosComponent } from '../paginas/painel-avisos.component';
import {
  CHAVE_AVISOS_MINIMIZADOS, CHAVE_ORCAMENTOS_MINIMIZADOS, avisosContinuamOcultos, chavePeriodo, lerAvisosOcultos, lerMinimizado,
  lerOrcamentosOcultos, orcamentosContinuamOcultos
} from '../paginas/painel-avisos';
import { IconeComponent } from './icone.component';
import { TermoComponent } from './termo.component';
import { dataIso } from './utilidades';
import { Icones } from './icones';

/**
 * #327: as notificações na barra do topo — os orçamentos estourados e os avisos do mês (os mesmos cards que ficavam no
 * topo do Painel, #206, com "Ocultar" e "Mostrar"), num card que abre pelo sino, em qualquer tela (Nielsen 1). O Painel
 * deixa de ser empurrado para baixo. O contador soma o que não está oculto nem minimizado.
 *
 * Padrão Disclosure do WAI-ARIA APG: o botão diz o estado em aria-expanded e aponta o card em aria-controls. Esc e o
 * clique fora fecham, e o foco volta ao botão (WCAG 1.4.13: dispensável sem mover o ponteiro).
 */
@Component({
  selector: 'app-notificacoes',
  imports: [IconeComponent, PainelAvisosComponent, RouterLink, TermoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  // Esc fecha de qualquer ponto (o sino ou o card); o menu "Ocultar" de dentro fecha antes e não deixa o Esc subir.
  host: { class: 'notificacoes', '(keydown.escape)': 'fechar(true)' },
  template: `
    <button #botao type="button" class="botao-barra botao-notificacoes" [attr.aria-expanded]="aberto()"
      aria-controls="card-notificacoes" [attr.aria-label]="rotulo()" (click)="alternar()">
      <app-icone [icone]="Icones.notificacoes" [tamanho]="18" />
      @if (pendentes() > 0) { <span class="contador" aria-hidden="true">{{ pendentes() > 9 ? '9+' : pendentes() }}</span> }
    </button>
    <div id="card-notificacoes">
      @if (aberto()) {
        <section class="card-notificacoes" aria-labelledby="titulo-notificacoes">
          <h2 id="titulo-notificacoes" class="titulo-notificacoes">Notificações@if (pendentes() > 0) {<span class="texto-suave">
            · {{ pendentes() }} {{ pendentes() === 1 ? 'pendente' : 'pendentes' }}</span>}</h2>
          @if (avisos(); as a) {
            @if (a.orcamentosEstourados.length || a.alertas.length) {
              <app-painel-avisos [estourados]="a.orcamentosEstourados" [alertas]="a.alertas" [periodo]="{ ano: a.ano, mes: a.mes }"
                (mudou)="versao.set(versao() + 1)" />
            }
            <!-- #9/#327: aviso discreto, sem "Ocultar": fica no card, mas fora do contador (não teria como zerar). -->
            @if (a.poucoInformativas) {
              <p class="nota-discreta pouco-informativas">
                {{ a.poucoInformativas === 1 ? '1 transação' : a.poucoInformativas + ' transações' }} dos últimos 90 dias com
                <app-termo chave="descricaoPoucoInformativa">descrição pouco informativa</app-termo>@if (a.exemploPoucoInformativa) { (ex.: “{{ a.exemploPoucoInformativa }}”)}.
                <a routerLink="/transacoes" [queryParams]="linkPoucoInformativas(a)">Revisar</a>
              </p>
            }
            @if (!a.orcamentosEstourados.length && !a.alertas.length && !a.poucoInformativas) {
              <p class="texto-suave sem-notificacoes">Nada pede atenção agora.</p>
            }
          } @else if (falhou()) {
            <!-- Nielsen 9: diz o que houve e dá a saída, em vez de "Carregando…" para sempre. -->
            <p class="sem-notificacoes" role="status">Não foi possível carregar as notificações.
              <button type="button" class="link" (click)="carregar()">Tentar de novo</button></p>
          } @else {
            <p class="texto-suave sem-notificacoes" role="status">Carregando…</p>
          }
        </section>
      }
    </div>
  `,
  styles: [`
    :host { position: relative; display: inline-flex; }
    .botao-notificacoes { position: relative; }
    .contador {
      position: absolute; top: 2px; right: 0; min-width: 18px; height: 18px; padding: 0 5px; border-radius: 99px;
      background: var(--negativo); color: #fff; font-size: 0.7rem; font-weight: 700; line-height: 18px; text-align: center;
    }
    .card-notificacoes {
      position: absolute; top: calc(100% + 8px); right: 0; z-index: 30; width: min(440px, calc(100vw - 32px));
      padding: 14px; border-radius: var(--raio);
      background: var(--fundo); border: 1px solid var(--linha); box-shadow: var(--sombra);
    }
    .titulo-notificacoes { margin: 0 0 10px; font-size: 1rem; }
    .sem-notificacoes { margin: 4px 0; }
    .pouco-informativas { margin: 0; }
    /* No celular, o card ocupa a largura da tela, preso abaixo da barra. */
    @media (max-width: 760px) {
      :host { position: static; }
      .card-notificacoes { position: fixed; top: 56px; left: 16px; right: 16px; width: auto; max-height: calc(100dvh - 72px); overflow: auto; }
    }
  `]
})
export class NotificacoesComponent {
  readonly Icones = Icones;
  private readonly api = inject(ApiService);
  private readonly el = inject(ElementRef<HTMLElement>);
  private readonly botao = viewChild.required<ElementRef<HTMLButtonElement>>('botao');

  readonly aberto = signal(false);
  readonly avisos = signal<Avisos | null>(null);
  /** A busca falhou e ainda não há avisos para mostrar. */
  readonly falhou = signal(false);
  /** Muda quando um card é ocultado ou mostrado: o contador lê de novo o que está guardado no navegador. */
  readonly versao = signal(0);

  readonly pendentes = computed(() => {
    this.versao();
    const a = this.avisos();
    if (!a) return 0;
    const orcamentosRecolhidos = lerMinimizado(CHAVE_ORCAMENTOS_MINIMIZADOS)
      || orcamentosContinuamOcultos(lerOrcamentosOcultos(), chavePeriodo({ ano: a.ano, mes: a.mes }), a.orcamentosEstourados);
    const avisosRecolhidos = lerMinimizado(CHAVE_AVISOS_MINIMIZADOS) || avisosContinuamOcultos(lerAvisosOcultos(), a.alertas);
    return (orcamentosRecolhidos ? 0 : a.orcamentosEstourados.length) + (avisosRecolhidos ? 0 : a.alertas.length);
  });
  readonly rotulo = computed(() => {
    const n = this.pendentes();
    return n === 0 ? 'Notificações, nada pendente' : `Notificações, ${n} ${n === 1 ? 'pendente' : 'pendentes'}`;
  });

  constructor() {
    this.carregar();
    // A cada tela, o contador fica em dia (uma sincronização ou um orçamento novo mudam os avisos), e o card aberto
    // fecha: um link dentro dele leva para outra tela, que não fica coberta.
    inject(Router).events.pipe(filter((e) => e instanceof NavigationEnd)).subscribe(() => {
      this.fechar(false);
      this.carregar();
    });
  }

  carregar(): void {
    // Complementar: se falhar, o sino fica sem contador, o resto da tela segue, e o card diz que não carregou.
    this.falhou.set(false);
    this.api.avisos().subscribe({
      next: (a) => this.avisos.set(a),
      error: () => { if (!this.avisos()) this.falhou.set(true); }
    });
  }

  /** Transações dos últimos 90 dias com descrição pouco informativa, em Transações. */
  linkPoucoInformativas(a: Avisos): Record<string, string> {
    return { generica: '1', ...(a.poucoInformativasDesde ? { inicio: a.poucoInformativasDesde, fim: dataIso() } : {}) };
  }

  alternar(): void {
    if (this.aberto()) {
      this.fechar(true);
    } else {
      this.aberto.set(true);
      this.carregar();
    }
  }

  fechar(devolverFoco: boolean): void {
    if (!this.aberto()) return;
    this.aberto.set(false);
    if (devolverFoco) this.botao().nativeElement.focus();
  }

  @HostListener('document:click', ['$event'])
  cliqueFora(ev: MouseEvent): void {
    if (this.aberto() && !(this.el.nativeElement as HTMLElement).contains(ev.target as Node)) this.fechar(false);
  }
}
