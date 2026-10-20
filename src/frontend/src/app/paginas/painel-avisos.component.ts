import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { Orcamento } from '../core/modelos';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { NOMES_MESES, paramsTransacoes, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import {
  CHAVE_AVISOS_OCULTOS, CHAVE_ORCAMENTOS_OCULTOS, ORCAMENTOS_VISIVEIS, avisosContinuamOcultos, chavePeriodo, guardar,
  impressaoAviso, lerAvisosOcultos, lerOrcamentosOcultos, mostrarOrcamentos, ocultarOrcamentos, orcamentosContinuamOcultos
} from './painel-avisos';

/**
 * #206: os avisos do topo do Painel em dois cards — orçamentos estourados (agrupados, os piores primeiro, cada um
 * levando a Transações filtrada) e os outros avisos. Os dois dá para ocultar (Nielsen 3) e voltam sozinhos quando
 * algo novo ou pior aparece (Nielsen 1). Ocultos, viram uma linha com a quantidade: nada some de vez, e a conta
 * vencida ou o erro de sincronização continuam à vista. O botão segue o padrão Disclosure (aria-expanded/controls).
 */
@Component({
  selector: 'app-painel-avisos',
  imports: [DecimalPipe, RouterLink, IconeComponent, MoedaPipe],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    @if (estourados().length) {
      <section class="aviso-painel aviso-orcamentos" [class.recolhido]="orcamentosRecolhidos()" aria-labelledby="titulo-orcamentos-estourados">
        <header class="aviso-painel-topo">
          <app-icone [icone]="Icones.orcamentos" [tamanho]="18" />
          <!-- O span anuncia a mudança sem tirar o papel de título do h2. -->
          <h2 id="titulo-orcamentos-estourados"><span role="status">{{ tituloOrcamentos() }}</span></h2>
          <button type="button" class="link aviso-painel-alternar" aria-controls="lista-orcamentos-estourados"
            [attr.aria-expanded]="!orcamentosRecolhidos()" (click)="alternarOrcamentos()"
            [attr.aria-label]="orcamentosRecolhidos() ? 'Mostrar os orçamentos estourados' : 'Ocultar os avisos de orçamento ' + ateQuando()">
            {{ orcamentosRecolhidos() ? 'Mostrar' : 'Ocultar ' + ateQuando() }}
          </button>
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
          <button type="button" class="link aviso-painel-alternar" aria-controls="lista-avisos-painel"
            [attr.aria-expanded]="!avisosRecolhidos()" (click)="alternarAvisos()"
            [attr.aria-label]="avisosRecolhidos() ? 'Mostrar os avisos' : 'Ocultar os avisos até aparecer um novo'">
            {{ avisosRecolhidos() ? 'Mostrar' : 'Ocultar até aparecer um novo' }}
          </button>
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
    .aviso-painel {
      position: relative; overflow: hidden; background: var(--superficie); border-radius: var(--raio);
      box-shadow: var(--sombra); padding: 14px 18px 14px 22px; margin-bottom: 16px;
    }
    .aviso-painel::before { content: ""; position: absolute; inset: 0 auto 0 0; width: 5px; background: var(--negativo); }
    .aviso-geral::before { background: var(--alerta); }
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
  readonly orcamentosRecolhidos = computed(() =>
    orcamentosContinuamOcultos(this.orcamentosOcultos(), chavePeriodo(this.periodo()), this.estourados()));
  readonly avisosRecolhidos = computed(() => avisosContinuamOcultos(this.avisosOcultos(), this.alertas()));

  linkTransacoes(o: Orcamento): Record<string, string> {
    const { ano, mes } = this.periodo();
    return paramsTransacoes({ categoriaId: o.categoriaId, inicio: primeiroDiaMes(ano, mes), fim: ultimoDiaMes(ano, mes), tipo: 'despesa' });
  }

  alternarOrcamentos(): void {
    const periodo = chavePeriodo(this.periodo());
    this.orcamentosOcultos.update((salvo) => this.orcamentosRecolhidos()
      ? mostrarOrcamentos(salvo, periodo)
      : ocultarOrcamentos(salvo, periodo, this.estourados()));
    guardar(CHAVE_ORCAMENTOS_OCULTOS, this.orcamentosOcultos());
  }

  alternarAvisos(): void {
    this.avisosOcultos.set(this.avisosRecolhidos() ? [] : this.alertas().map(impressaoAviso));
    guardar(CHAVE_AVISOS_OCULTOS, this.avisosOcultos());
  }
}
