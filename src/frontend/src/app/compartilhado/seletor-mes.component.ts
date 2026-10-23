import { Component, ElementRef, HostListener, computed, inject, input, output, signal, viewChild, ChangeDetectionStrategy } from '@angular/core';
import { NOMES_MESES, NOMES_MESES_CURTOS } from './utilidades';
import { ANOS_POR_PAGINA, inicioDaPaginaDeAnos } from './campo-data.component';

export interface MesAno { ano: number; mes: number; }

/**
 * Seletor de mês: ‹ › andam um mês; o nome do mês abre um quadro com o ano (‹ 2026 ›) e os 12 meses, para pular
 * direto para qualquer mês e ano, e "Este mês" para voltar. Esc ou clique fora fecham e devolvem o foco ao botão
 * (Nielsen 3, User Control and Freedom; Nielsen 7, Flexibility and Efficiency of Use: dois cliques em vez de vários).
 *
 * #342: o ano do quadro é um botão que abre uma página de 12 anos, como o título do calendário dos campos de data
 * (#264; Nielsen 4, Consistency and Standards). Escolher um ano volta aos meses dele; Esc nos anos volta aos meses.
 */
@Component({
  selector: 'app-seletor-mes',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="seletor-mes" role="group" aria-label="Selecionar mês">
      <button type="button" class="botao-icone" (click)="mover(-1)" aria-label="Mês anterior">‹</button>
      <button #botaoMes type="button" class="seletor-mes-rotulo" (click)="alternar()" [attr.aria-expanded]="aberto()"
        aria-haspopup="dialog" [attr.aria-controls]="id" [attr.aria-label]="rotulo() + ': escolher outro mês'">
        {{ rotulo() }}<span class="seletor-mes-seta" aria-hidden="true">▾</span>
      </button>
      <button type="button" class="botao-icone" (click)="mover(1)" aria-label="Próximo mês">›</button>

      @if (aberto()) {
        <div class="seletor-mes-painel" [id]="id" role="dialog" aria-label="Escolher mês e ano" animate.leave="sair"
          (keydown.escape)="esc($event)">
          <div class="seletor-mes-ano">
            <button type="button" class="botao-icone" (click)="navegar(-1)"
              [attr.aria-label]="visao() === 'meses' ? 'Ano anterior' : 'Anos anteriores'">‹</button>
            <!-- #342: nos meses, o ano é o botão que sobe para os anos (como o título do calendário, #264). -->
            <strong aria-live="polite">@if (visao() === 'meses') {<button #botaoAno type="button" class="calendario-titulo-botao"
              [attr.aria-describedby]="id + '-dica'" (click)="verAnos()">{{ anoNoPainel() }}</button>} @else {<span>{{ inicioAnos() }}–{{ inicioAnos() + anosPorPagina - 1 }}</span>}</strong>
            <button type="button" class="botao-icone" (click)="navegar(1)"
              [attr.aria-label]="visao() === 'meses' ? 'Próximo ano' : 'Próximos anos'">›</button>
          </div>
          <span class="so-leitor" [id]="id + '-dica'">escolher outro ano</span>
          <div class="seletor-mes-grade">
            @if (visao() === 'meses') {
              @for (nome of meses; track $index) {
                <button type="button" class="seletor-mes-opcao" [class.atual]="ehHoje($index + 1)"
                  [attr.aria-pressed]="ehEscolhido($index + 1)" [attr.aria-label]="nomesLongos[$index] + ' de ' + anoNoPainel()"
                  (click)="escolher($index + 1)">{{ nome }}</button>
              }
            } @else {
              @for (ano of anos(); track ano) {
                <button type="button" class="seletor-mes-opcao seletor-mes-opcao-ano" [class.atual]="ano === anoAtual"
                  [attr.aria-pressed]="ano === anoNoPainel()" (click)="escolherAno(ano)">{{ ano }}</button>
              }
            }
          </div>
          <button type="button" class="botao botao-pequeno seletor-mes-hoje" (click)="esteMes()">Este mês</button>
        </div>
      }
    </div>
  `
})
export class SeletorMesComponent {
  private static proximoId = 0;
  private readonly elemento = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly botaoMes = viewChild.required<ElementRef<HTMLButtonElement>>('botaoMes');
  private readonly botaoAno = viewChild<ElementRef<HTMLButtonElement>>('botaoAno');

  readonly valor = input.required<MesAno>();
  readonly mudou = output<MesAno>();

  readonly id = `seletor-mes-${++SeletorMesComponent.proximoId}`;
  readonly meses = NOMES_MESES_CURTOS;
  readonly nomesLongos = NOMES_MESES;
  readonly aberto = signal(false);
  /** Ano mostrado no quadro (navega sem trocar o mês da tela até escolher um mês). */
  readonly anoNoPainel = signal(new Date().getFullYear());
  /** #342: o quadro mostra os meses de um ano ou uma página de 12 anos. */
  readonly visao = signal<'meses' | 'anos'>('meses');
  readonly inicioAnos = signal(inicioDaPaginaDeAnos(new Date().getFullYear()));
  readonly anosPorPagina = ANOS_POR_PAGINA;
  readonly anoAtual = new Date().getFullYear();
  readonly anos = computed(() => Array.from({ length: ANOS_POR_PAGINA }, (_, i) => this.inicioAnos() + i));

  readonly rotulo = computed(() => `${NOMES_MESES[this.valor().mes - 1]} de ${this.valor().ano}`);

  mover(delta: number): void {
    const d = new Date(this.valor().ano, this.valor().mes - 1 + delta, 1);
    this.mudou.emit({ ano: d.getFullYear(), mes: d.getMonth() + 1 });
  }

  alternar(): void {
    if (this.aberto()) { this.fechar(false); return; }
    this.anoNoPainel.set(this.valor().ano);
    this.visao.set('meses');
    this.aberto.set(true);
  }

  /** ‹ › andam um ano nos meses e uma página (12 anos) nos anos. */
  navegar(direcao: number): void {
    if (this.visao() === 'meses') this.anoNoPainel.update((a) => a + direcao);
    else this.inicioAnos.update((a) => a + ANOS_POR_PAGINA * direcao);
  }

  verAnos(): void {
    this.inicioAnos.set(inicioDaPaginaDeAnos(this.anoNoPainel()));
    this.visao.set('anos');
    this.focar('.seletor-mes-opcao[aria-pressed="true"], .seletor-mes-opcao');
  }

  escolherAno(ano: number): void {
    this.anoNoPainel.set(ano);
    this.visao.set('meses');
    setTimeout(() => this.botaoAno()?.nativeElement.focus());
  }

  /** Esc nos anos volta aos meses; nos meses, fecha (e devolve o foco ao botão do mês). */
  esc(e: Event): void {
    if (this.visao() === 'anos') {
      e.stopPropagation();
      this.escolherAno(this.anoNoPainel());
    } else {
      this.fechar(true);
    }
  }

  private focar(seletor: string): void {
    setTimeout(() => this.elemento.nativeElement.querySelector<HTMLElement>(seletor)?.focus());
  }

  escolher(mes: number): void {
    const escolhido = { ano: this.anoNoPainel(), mes };
    this.fechar(true);
    if (escolhido.ano !== this.valor().ano || escolhido.mes !== this.valor().mes) this.mudou.emit(escolhido);
  }

  esteMes(): void {
    const hoje = new Date();
    this.anoNoPainel.set(hoje.getFullYear());
    this.escolher(hoje.getMonth() + 1);
  }

  fechar(devolverFoco: boolean): void {
    this.aberto.set(false);
    if (devolverFoco) this.botaoMes().nativeElement.focus();
  }

  ehEscolhido(mes: number): boolean {
    return this.valor().ano === this.anoNoPainel() && this.valor().mes === mes;
  }

  ehHoje(mes: number): boolean {
    const hoje = new Date();
    return hoje.getFullYear() === this.anoNoPainel() && hoje.getMonth() + 1 === mes;
  }

  @HostListener('document:mousedown', ['$event'])
  aoClicarFora(e: MouseEvent): void {
    if (this.aberto() && !this.elemento.nativeElement.contains(e.target as Node)) this.fechar(false);
  }
}
