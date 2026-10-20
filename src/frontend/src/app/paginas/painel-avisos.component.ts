import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { Orcamento } from '../core/modelos';
import { IconeComponent } from '../compartilhado/icone.component';
import { AcaoMenu, MenuAcoesComponent } from '../compartilhado/menu-acoes.component';
import { Icones } from '../compartilhado/icones';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES, paramsTransacoes, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import {
  CHAVE_AVISOS_MINIMIZADOS, CHAVE_AVISOS_OCULTOS, CHAVE_ORCAMENTOS_MINIMIZADOS, CHAVE_ORCAMENTOS_OCULTOS, ORCAMENTOS_VISIVEIS, avisosContinuamOcultos, chavePeriodo, guardar,
  impressaoAviso, lerAvisosOcultos, lerMinimizado, lerOrcamentosOcultos, mostrarOrcamentos, ocultarOrcamentos, orcamentosContinuamOcultos
} from './painel-avisos';

/**
 * #206: os avisos do topo do Painel em dois cards — orçamentos estourados (agrupados, os piores primeiro, cada um
 * levando a Transações filtrada) e os outros avisos. Os dois dá para ocultar (Nielsen 3) e voltam sozinhos quando
 * algo novo ou pior aparece (Nielsen 1). Ocultos, viram uma linha com a quantidade: nada some de vez, e a conta
 * vencida ou o erro de sincronização continuam à vista. O botão segue o padrão Disclosure (aria-expanded/controls).
 */
@Component({
  selector: 'app-painel-avisos',
  imports: [DecimalPipe, RouterLink, IconeComponent, MenuAcoesComponent, MoedaPipe],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    @if (estourados().length) {
      <section class="aviso-painel aviso-orcamentos" [class.recolhido]="orcamentosRecolhidos()" aria-labelledby="titulo-orcamentos-estourados">
        <header class="aviso-painel-topo">
          <app-icone [icone]="Icones.orcamentos" [tamanho]="18" />
          <!-- O span anuncia a mudança sem tirar o papel de título do h2. -->
          <h2 id="titulo-orcamentos-estourados"><span role="status">{{ tituloOrcamentos() }}</span></h2>
          @if (orcamentosRecolhidos()) {
            <button type="button" id="mostrar-orcamentos-estourados" class="link aviso-painel-alternar" aria-controls="lista-orcamentos-estourados"
              aria-expanded="false" aria-label="Mostrar os orçamentos estourados" (click)="mostrarCardOrcamentos()">Mostrar</button>
          } @else {
            <app-menu-acoes rotulo="Ocultar" rotuloAcessivel="Ocultar os avisos de orçamento" [acoes]="acoesOrcamentos()"
              (escolhida)="ocultarCardOrcamentos($event)" />
          }
        </header>
        <div id="lista-orcamentos-estourados">
          @if (!orcamentosRecolhidos()) {
            <ul class="lista-estourados">
              @for (o of orcamentosVisiveis(); track o.categoriaId) {
                <li [class.por-emergencia]="o.situacao === 'emergencia'">
                  <div class="estourado-linha">
                    <a class="link-alvo" routerLink="/transacoes" [queryParams]="linkTransacoes(o)"
                      [attr.aria-label]="'Ver as despesas de ' + o.categoriaNome + ' em ' + nomeMes()">{{ o.categoriaNome }}</a>
                    @if (o.situacao === 'emergencia') { <span class="etiqueta etiqueta-emergencia">por emergência</span> }
                    <span class="estourado-quanto" [class.negativo]="o.situacao !== 'emergencia'" [class.texto-emergencia]="o.situacao === 'emergencia'">
                      passou {{ o.gasto - o.valor | currency }} <span class="texto-suave">({{ o.percentual | number: '1.0-0' }}%)</span>
                    </span>
                  </div>
                  <!-- A parte clara é o limite; a colorida, o que passou dele. O texto ao lado já diz os números. -->
                  <div class="trilho trilho-estourado" aria-hidden="true">
                    <div class="dentro-limite" [style.width.%]="100 / o.percentual * 100"></div>
                    <div class="progresso" [class.estourado]="o.situacao !== 'emergencia'" [class.emergencia]="o.situacao === 'emergencia'"></div>
                  </div>
                  <small class="texto-suave">{{ o.gasto | currency }} gastos de {{ o.valor | currency }} de limite</small>
                </li>
              }
            </ul>
            <footer class="aviso-painel-rodape">
              @if (estourados().length > visiveis) {
                <button type="button" class="link" [attr.aria-expanded]="todos()" aria-controls="lista-orcamentos-estourados"
                  (click)="todos.set(!todos())">{{ todos() ? 'Mostrar só os ' + visiveis + ' piores' : 'Ver todos os ' + estourados().length }}</button>
              }
              <a class="link-alvo" routerLink="/orcamentos" [queryParams]="periodo()">Abrir Orçamentos</a>
            </footer>
          }
        </div>
      </section>
    }

    @if (alertas().length) {
      <section class="aviso-painel aviso-geral" [class.recolhido]="avisosRecolhidos()" aria-labelledby="titulo-avisos-painel">
        <header class="aviso-painel-topo">
          <app-icone [icone]="Icones.alerta" [tamanho]="18" />
          <h2 id="titulo-avisos-painel"><span role="status">
            {{ alertas().length === 1 ? '1 aviso pede atenção' : alertas().length + ' avisos pedem atenção' }}</span>
          </h2>
          @if (avisosRecolhidos()) {
            <button type="button" id="mostrar-avisos-painel" class="link aviso-painel-alternar" aria-controls="lista-avisos-painel"
              aria-expanded="false" aria-label="Mostrar os avisos" (click)="mostrarCardAvisos()">Mostrar</button>
          } @else {
            <app-menu-acoes rotulo="Ocultar" rotuloAcessivel="Ocultar os avisos" [acoes]="acoesAvisos"
              (escolhida)="ocultarCardAvisos($event)" />
          }
        </header>
        <div id="lista-avisos-painel">
          @if (!avisosRecolhidos()) {
            <ul class="lista-avisos">
              @for (a of alertas(); track a) { <li>{{ a }}</li> }
            </ul>
          }
        </div>
      </section>
    }
  `,
  styles: [`
    /* A faixa da esquerda é sombra interna (e não ::before com overflow: hidden), para o menu "Ocultar" não ser cortado. */
    .aviso-painel {
      position: relative; background: var(--superficie); border-radius: var(--raio);
      box-shadow: inset 5px 0 0 var(--negativo), var(--sombra); padding: 14px 18px 14px 22px; margin-bottom: 16px;
    }
    .aviso-geral { box-shadow: inset 5px 0 0 var(--alerta), var(--sombra); }
    .aviso-painel-topo { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; }
    .aviso-painel-topo app-icone { color: var(--negativo); display: inline-flex; }
    .aviso-geral .aviso-painel-topo app-icone { color: var(--alerta); }
    .aviso-painel-topo h2 { margin: 0; font-size: 1rem; flex: 1 1 220px; }
    .aviso-painel-alternar { font-size: 0.85rem; }
    .lista-estourados { list-style: none; margin: 12px 0 0; padding: 0; display: grid; gap: 12px; }
    .lista-estourados li { min-width: 0; }
    .estourado-linha { display: flex; flex-wrap: wrap; align-items: center; gap: 2px 8px; font-size: 0.92rem; }
    .estourado-linha a { font-weight: 600; }
    .estourado-quanto { margin-left: auto; font-weight: 600; white-space: nowrap; }
    .trilho-estourado { display: flex; }
    .trilho-estourado .dentro-limite { height: 100%; background: var(--superficie-3); border-right: 2px solid var(--texto-suave); flex: none; }
    .trilho-estourado .progresso { flex: 1; border-radius: 0 99px 99px 0; }
    .lista-estourados small { font-size: 0.8rem; }
    .aviso-painel-rodape { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 16px; margin-top: 10px; font-size: 0.88rem; }
    .lista-avisos { margin: 10px 0 0; padding-left: 20px; display: grid; gap: 4px; font-size: 0.92rem; }
  `]
})
export class PainelAvisosComponent {
  readonly Icones = Icones;
  readonly visiveis = ORCAMENTOS_VISIVEIS;

  readonly estourados = input<Orcamento[]>([]);
  readonly alertas = input<string[]>([]);
  readonly periodo = input.required<{ ano: number; mes: number }>();

  readonly todos = signal(false);
  /** "Minimizar": lembrado neste navegador, sem prazo; só o "Mostrar" abre de novo (a linha com o título fica). */
  private readonly orcamentosMinimizados = signal(lerMinimizado(CHAVE_ORCAMENTOS_MINIMIZADOS));
  private readonly avisosMinimizados = signal(lerMinimizado(CHAVE_AVISOS_MINIMIZADOS));
  private readonly orcamentosOcultos = signal(lerOrcamentosOcultos());
  private readonly avisosOcultos = signal(lerAvisosOcultos());

  readonly nomeMes = computed(() => `${NOMES_MESES[this.periodo().mes - 1].toLowerCase()}/${this.periodo().ano}`);
  private readonly mesAtual = computed(() => {
    const hoje = new Date();
    return this.periodo().ano === hoje.getFullYear() && this.periodo().mes === hoje.getMonth() + 1;
  });
  readonly ateQuando = computed(() => this.mesAtual() ? 'até o mês que vem' : `de ${this.nomeMes()}`);

  readonly tituloOrcamentos = computed(() => {
    const lista = this.estourados();
    const porEmergencia = lista.filter((o) => o.situacao === 'emergencia').length;
    const base = lista.length === 1 ? '1 orçamento estourado' : `${lista.length} orçamentos estourados`;
    const quando = this.mesAtual() ? 'neste mês' : `em ${this.nomeMes()}`;
    if (!porEmergencia) return `${base} ${quando}`;
    return `${base} ${quando} (${lista.length === 1 ? 'por emergência' : porEmergencia + ' por emergência'})`;
  });

  readonly orcamentosVisiveis = computed(() => this.todos() ? this.estourados() : this.estourados().slice(0, this.visiveis));
  readonly orcamentosRecolhidos = computed(() => this.orcamentosMinimizados()
    || orcamentosContinuamOcultos(this.orcamentosOcultos(), chavePeriodo(this.periodo()), this.estourados()));
  readonly avisosRecolhidos = computed(() => this.avisosMinimizados()
    || avisosContinuamOcultos(this.avisosOcultos(), this.alertas()));

  /** As duas opções do menu "Ocultar": só agora, ou lembrado até algo mudar. */
  readonly acoesOrcamentos = computed<AcaoMenu[]>(() => [
    { id: 'minimizar', rotulo: 'Minimizar', detalhe: 'Fica recolhido até você clicar em “Mostrar”.' },
    { id: 'ocultar', rotulo: `Ocultar ${this.ateQuando()}`,
      detalhe: 'Volta antes se outra categoria estourar ou uma piorar de faixa.' }
  ]);
  readonly acoesAvisos: AcaoMenu[] = [
    { id: 'minimizar', rotulo: 'Minimizar', detalhe: 'Fica recolhido até você clicar em “Mostrar”.' },
    { id: 'ocultar', rotulo: 'Ocultar até aparecer um novo', detalhe: 'Fica a linha com a quantidade.' }
  ];

  linkTransacoes(o: Orcamento): Record<string, string> {
    const { ano, mes } = this.periodo();
    return paramsTransacoes({ categoriaId: o.categoriaId, inicio: primeiroDiaMes(ano, mes), fim: ultimoDiaMes(ano, mes), tipo: 'despesa' });
  }

  ocultarCardOrcamentos(acao: string): void {
    if (acao === 'minimizar') {
      this.orcamentosMinimizados.set(true);
      guardar(CHAVE_ORCAMENTOS_MINIMIZADOS, true);
    } else {
      this.orcamentosOcultos.update((salvo) => ocultarOrcamentos(salvo, chavePeriodo(this.periodo()), this.estourados()));
      guardar(CHAVE_ORCAMENTOS_OCULTOS, this.orcamentosOcultos());
    }
    focarDepois('mostrar-orcamentos-estourados');
  }

  mostrarCardOrcamentos(): void {
    this.orcamentosMinimizados.set(false);
    guardar(CHAVE_ORCAMENTOS_MINIMIZADOS, false);
    this.orcamentosOcultos.update((salvo) => mostrarOrcamentos(salvo, chavePeriodo(this.periodo())));
    guardar(CHAVE_ORCAMENTOS_OCULTOS, this.orcamentosOcultos());
  }

  ocultarCardAvisos(acao: string): void {
    if (acao === 'minimizar') {
      this.avisosMinimizados.set(true);
      guardar(CHAVE_AVISOS_MINIMIZADOS, true);
    } else {
      this.avisosOcultos.set(this.alertas().map(impressaoAviso));
      guardar(CHAVE_AVISOS_OCULTOS, this.avisosOcultos());
    }
    focarDepois('mostrar-avisos-painel');
  }

  mostrarCardAvisos(): void {
    this.avisosMinimizados.set(false);
    guardar(CHAVE_AVISOS_MINIMIZADOS, false);
    this.avisosOcultos.set([]);
    guardar(CHAVE_AVISOS_OCULTOS, this.avisosOcultos());
  }
}

/** O menu some ao recolher o card; o foco vai para o "Mostrar" que entra no lugar, e não se perde (WCAG 2.4.3). */
function focarDepois(id: string): void {
  setTimeout(() => document.getElementById(id)?.focus());
}
