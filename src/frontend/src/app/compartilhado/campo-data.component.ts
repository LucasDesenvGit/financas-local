import {
  Component, DestroyRef, ElementRef, HostListener, Injector, afterNextRender, booleanAttribute, computed, forwardRef, inject, input, output,
  signal, viewChild
} from '@angular/core';
import { AbstractControl, ControlValueAccessor, NG_VALIDATORS, NG_VALUE_ACCESSOR, ValidationErrors, Validator } from '@angular/forms';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';
import { NOMES_MESES } from './utilidades';

/** Intervalo padrão dos campos de data: cobre extratos antigos e metas de longo prazo (ano sempre com 4 dígitos). */
export const DATA_MINIMA = '1900-01-01';
export const DATA_MAXIMA = '2099-12-31';

const DIAS_SEMANA = ['domingo', 'segunda-feira', 'terça-feira', 'quarta-feira', 'quinta-feira', 'sexta-feira', 'sábado'];
const INICIAIS_SEMANA = ['D', 'S', 'T', 'Q', 'Q', 'S', 'S'];

/** 'AAAA-MM-DD' → 'dd/mm/aaaa' ('' se vazio ou fora do formato). */
export function isoParaTexto(iso: string | null | undefined): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso ?? '');
  return m ? `${m[3]}/${m[2]}/${m[1]}` : '';
}

export type EstadoData = 'vazia' | 'incompleta' | 'invalida' | 'ok';

/** 'dd/mm/aaaa' → ISO, com o estado: vazia, incompleta (faltam dígitos), inválida (31/02) ou ok. */
export function textoParaIso(texto: string): { estado: EstadoData; iso: string | null } {
  const digitos = texto.replace(/\D/g, '');
  if (!digitos) return { estado: 'vazia', iso: null };
  if (digitos.length < 8) return { estado: 'incompleta', iso: null };
  const dia = Number(digitos.slice(0, 2));
  const mes = Number(digitos.slice(2, 4));
  const ano = Number(digitos.slice(4, 8));
  const d = new Date(ano, mes - 1, dia);
  if (mes < 1 || mes > 12 || d.getFullYear() !== ano || d.getMonth() !== mes - 1 || d.getDate() !== dia) {
    return { estado: 'invalida', iso: null };
  }
  return { estado: 'ok', iso: `${digitos.slice(4, 8)}-${digitos.slice(2, 4)}-${digitos.slice(0, 2)}` };
}

/**
 * Máscara enquanto se digita: só dígitos (até 8), com as barras entrando sozinhas ("18102026" → "18/10/2026"). O ano
 * fica sempre com 4 dígitos. O cursor fica depois do mesmo número de dígitos.
 */
export function formatarDataDigitando(texto: string, cursor: number): { texto: string; cursor: number } {
  const antes = texto.slice(0, cursor).replace(/\D/g, '').length;
  const d = texto.replace(/\D/g, '').slice(0, 8);
  let novo = d.slice(0, 2);
  if (d.length > 2) novo += '/' + d.slice(2, 4);
  if (d.length > 4) novo += '/' + d.slice(4, 8);
  let posicao = 0;
  for (let vistos = 0; posicao < novo.length && vistos < antes; posicao++) {
    if (/\d/.test(novo[posicao])) vistos++;
  }
  return { texto: novo, cursor: posicao };
}

function iso(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function dataDe(isoTexto: string): Date {
  const [a, m, d] = isoTexto.split('-').map(Number);
  return new Date(a, m - 1, d);
}

interface Dia { iso: string; numero: number; doMes: boolean; nome: string; }

let proximoId = 0;

/**
 * Campo de data padrão (ADR 0032), no lugar de <input type="date">:
 * - digitar dd/mm/aaaa, com as barras entrando sozinhas e o ano sempre com 4 dígitos;
 * - ou escolher num calendário no visual do site (botão ao lado): ‹ › mês, « » ano, "Hoje" e "Limpar";
 *   teclado do padrão de calendário do WAI-ARIA (setas, Home/End, PageUp/PageDown, Shift+PageUp/Down para o ano,
 *   Enter escolhe, Esc fecha e devolve o foco);
 * - limites (padrão 1900–2099, ou os da tela: [maximo]="hoje"), erro em texto com aria-describedby (WCAG 3.3.1).
 * Entrega ao ngModel o mesmo 'AAAA-MM-DD' do campo nativo ('' quando vazio). (alterado) faz o papel do (change).
 * Uso: <label>Data <app-campo-data name="data" [(ngModel)]="fm.data" obrigatorio /></label>
 */
@Component({
  selector: 'app-campo-data',
  imports: [IconeComponent],
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => CampoDataComponent), multi: true },
    { provide: NG_VALIDATORS, useExisting: forwardRef(() => CampoDataComponent), multi: true }
  ],
  template: `
    <span class="campo-valor campo-data" [class.campo-valor-erro]="erroVisivel() || invalido()">
      <input type="text" inputmode="numeric" autocomplete="off" placeholder="dd/mm/aaaa"
        [attr.name]="name() || null" [value]="texto()" [disabled]="desabilitado()"
        [attr.aria-required]="obrigatorio() || null" [attr.aria-invalid]="erroVisivel() || invalido() ? 'true' : null"
        [attr.aria-describedby]="descricao()" [attr.aria-label]="rotulo() || null"
        (input)="aoDigitar($event)" (blur)="aoSair()" (keydown.alt.arrowdown)="abrir($event)">
      <button #botao type="button" class="campo-data-botao" [disabled]="desabilitado()" (click)="alternar()"
        aria-haspopup="dialog" [attr.aria-expanded]="aberto()" [attr.aria-controls]="aberto() ? idCalendario : null"
        aria-label="Escolher a data no calendário">
        <app-icone [icone]="Icones.calendario" [tamanho]="17" />
      </button>
    </span>
    @if (erroVisivel()) {
      <span class="campo-valor-mensagem" [id]="idErro" aria-hidden="true" animate.leave="sair">{{ erro() }}</span>
    }
    @if (aberto()) {
      <!-- Na camada superior (popover), com posição fixa: um modal ou uma tabela com rolagem não cortam o calendário. -->
      <div #calendario class="calendario" [id]="idCalendario" role="dialog" aria-modal="false" popover="manual"
        [style.top.px]="posicao().top" [style.bottom.px]="posicao().bottom" [style.left.px]="posicao().left"
        [attr.aria-labelledby]="idTitulo" animate.leave="sair" (keydown.escape)="fechar(true); $event.stopPropagation()">
        <div class="calendario-topo">
          <button type="button" class="botao-icone" (click)="mudarMes(-12)" aria-label="Ano anterior">«</button>
          <button type="button" class="botao-icone" (click)="mudarMes(-1)" aria-label="Mês anterior">‹</button>
          <strong [id]="idTitulo" aria-live="polite">{{ tituloMes() }}</strong>
          <button type="button" class="botao-icone" (click)="mudarMes(1)" aria-label="Próximo mês">›</button>
          <button type="button" class="botao-icone" (click)="mudarMes(12)" aria-label="Próximo ano">»</button>
        </div>
        <table class="calendario-grade" role="grid" [attr.aria-labelledby]="idTitulo" (keydown)="teclaNaGrade($event)">
          <thead>
            <tr>@for (d of iniciais; track $index) { <th scope="col" [attr.abbr]="diasSemana[$index]">{{ d }}</th> }</tr>
          </thead>
          <tbody>
            @for (semana of semanas(); track $index) {
              <tr>
                @for (dia of semana; track dia.iso) {
                  <td [attr.aria-selected]="dia.iso === valorIso()">
                    <button type="button" class="calendario-dia" [class.fora-do-mes]="!dia.doMes"
                      [class.escolhido]="dia.iso === valorIso()" [attr.aria-current]="dia.iso === hoje ? 'date' : null"
                      [attr.data-iso]="dia.iso" [attr.tabindex]="dia.iso === foco() ? 0 : -1"
                      [disabled]="foraDoIntervalo(dia.iso)" [attr.aria-label]="dia.nome" (click)="escolher(dia.iso)">
                      {{ dia.numero }}</button>
                  </td>
                }
              </tr>
            }
          </tbody>
        </table>
        <div class="calendario-rodape">
          <button type="button" class="link" (click)="limpar()">Limpar</button>
          <button type="button" class="link" [disabled]="foraDoIntervalo(hoje)" (click)="escolher(hoje)">Hoje</button>
        </div>
      </div>
    }
  `
})
export class CampoDataComponent implements ControlValueAccessor, Validator {
  private readonly injector = inject(Injector);
  private readonly elemento = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly botao = viewChild.required<ElementRef<HTMLButtonElement>>('botao');
  private readonly calendario = viewChild<ElementRef<HTMLElement>>('calendario');

  /** Data mais antiga aceita ('AAAA-MM-DD'); padrão 1900-01-01. */
  readonly minimo = input<string | null | undefined>(null);
  /** Data mais recente aceita ('AAAA-MM-DD'); padrão 2099-12-31 (ex.: [maximo]="hoje"). */
  readonly maximo = input<string | null | undefined>(null);
  readonly obrigatorio = input(false, { transform: booleanAttribute });
  readonly name = input<string>('');
  readonly rotulo = input<string>('');
  /** Id de um texto de ajuda da tela, ligado ao campo por aria-describedby (junto com a mensagem de erro). */
  readonly ajuda = input<string>('');
  /** Inválido por uma regra da tela (ex.: data limite antes do vencimento): marca aria-invalid e a borda de erro. */
  readonly invalido = input(false, { transform: booleanAttribute });
  /** Valor confirmado (ao escolher no calendário, ao sair do campo com outra data, em "Limpar"): faz o papel do (change). */
  readonly alterado = output<string>();

  readonly Icones = Icones;
  readonly iniciais = INICIAIS_SEMANA;
  readonly diasSemana = DIAS_SEMANA;
  readonly hoje = iso(new Date());
  private readonly n = ++proximoId;
  readonly idErro = `campo-data-erro-${this.n}`;
  readonly idCalendario = `campo-data-calendario-${this.n}`;
  readonly idTitulo = `campo-data-titulo-${this.n}`;

  readonly texto = signal('');
  readonly desabilitado = signal(false);
  readonly aberto = signal(false);
  /** Posição fixa do calendário (embaixo do campo; sem espaço, em cima). */
  readonly posicao = signal<{ top: number | null; bottom: number | null; left: number }>({ top: 0, bottom: null, left: 0 });
  /** Mês mostrado no calendário e o dia com o foco (tabindex 0). */
  readonly mesVisivel = signal({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  readonly foco = signal(this.hoje);
  private readonly tocado = signal(false);
  private ultimoConfirmado = '';

  readonly minimoEfetivo = computed(() => this.minimo() || DATA_MINIMA);
  readonly maximoEfetivo = computed(() => this.maximo() || DATA_MAXIMA);
  readonly valorIso = computed(() => textoParaIso(this.texto()).iso);
  readonly erro = computed(() => this.mensagemDeErro());
  readonly erroVisivel = computed(() => this.tocado() && this.erro() !== null);
  readonly descricao = computed(() => [this.ajuda(), this.erroVisivel() ? this.idErro : ''].filter(Boolean).join(' ') || null);
  readonly tituloMes = computed(() => `${NOMES_MESES[this.mesVisivel().mes - 1]} de ${this.mesVisivel().ano}`);

  /** 6 semanas de domingo a sábado, com os dias do mês anterior e do seguinte para completar. */
  readonly semanas = computed<Dia[][]>(() => {
    const { ano, mes } = this.mesVisivel();
    const primeiro = new Date(ano, mes - 1, 1);
    const inicio = new Date(ano, mes - 1, 1 - primeiro.getDay());
    const semanas: Dia[][] = [];
    for (let s = 0; s < 6; s++) {
      const semana: Dia[] = [];
      for (let d = 0; d < 7; d++) {
        const data = new Date(inicio.getFullYear(), inicio.getMonth(), inicio.getDate() + s * 7 + d);
        semana.push({
          iso: iso(data), numero: data.getDate(), doMes: data.getMonth() === mes - 1,
          nome: `${DIAS_SEMANA[data.getDay()]}, ${data.getDate()} de ${NOMES_MESES[data.getMonth()]} de ${data.getFullYear()}`
        });
      }
      semanas.push(semana);
    }
    return semanas;
  });

  private aoMudar: (v: string) => void = () => undefined;
  private aoTocar: () => void = () => undefined;

  /* ------------------------------ ControlValueAccessor / Validator ------------------------------ */

  writeValue(valor: string | null | undefined): void {
    this.texto.set(isoParaTexto(valor));
    this.ultimoConfirmado = textoParaIso(this.texto()).iso ?? '';
    this.ultimoModelo = this.ultimoConfirmado;
  }
  registerOnChange(fn: (v: string) => void): void { this.aoMudar = fn; }
  registerOnTouched(fn: () => void): void { this.aoTocar = fn; }
  setDisabledState(desabilitado: boolean): void { this.desabilitado.set(desabilitado); }

  validate(_: AbstractControl): ValidationErrors | null {
    const mensagem = this.erro();
    return mensagem ? { campoData: mensagem } : null;
  }

  /* ------------------------------ campo de texto ------------------------------ */

  aoDigitar(e: Event): void {
    const campo = e.target as HTMLInputElement;
    // Colou uma data em AAAA-MM-DD (planilha, extrato): converte para dd/mm/aaaa.
    const colada = /^\s*(\d{4})-(\d{2})-(\d{2})\s*$/.exec(campo.value);
    if (colada) campo.value = `${colada[3]}/${colada[2]}/${colada[1]}`;
    const r = formatarDataDigitando(campo.value, colada ? campo.value.length : campo.selectionStart ?? campo.value.length);
    if (campo.value !== r.texto) {
      campo.value = r.texto;
      campo.setSelectionRange?.(r.cursor, r.cursor);
    }
    this.texto.set(r.texto);
    this.empurrarModelo(this.valorIso() ?? '');
  }

  /** Só avisa o ngModel quando o valor muda de fato (digitando "1", "18", "18/1"… o modelo fica '' uma vez só). */
  private ultimoModelo = '';
  private empurrarModelo(valor: string): void {
    if (valor === this.ultimoModelo) return;
    this.ultimoModelo = valor;
    this.aoMudar(valor);
  }

  aoSair(): void {
    this.tocado.set(true);
    this.aoTocar();
    this.confirmar();
  }

  /* ------------------------------ calendário ------------------------------ */

  alternar(): void {
    if (this.aberto()) this.fechar(true); else this.abrir();
  }

  abrir(e?: Event): void {
    e?.preventDefault();
    if (this.desabilitado()) return;
    const base = this.valorIso() ?? (this.foraDoIntervalo(this.hoje) ? this.maximoEfetivo() : this.hoje);
    const d = dataDe(base);
    this.mesVisivel.set({ ano: d.getFullYear(), mes: d.getMonth() + 1 });
    this.foco.set(base);
    // Posição fixa a partir do campo; sem espaço embaixo (ex.: campo no fim da tela), abre para cima.
    const caixa = this.elemento.nativeElement.querySelector('.campo-data') ?? this.elemento.nativeElement;
    const r = caixa.getBoundingClientRect();
    const altura = 340;
    const largura = 296;
    const acima = window.innerHeight - r.bottom < altura && r.top > altura;
    this.posicao.set({
      top: acima ? null : r.bottom + 6,
      bottom: acima ? window.innerHeight - r.top + 6 : null,
      left: Math.max(8, Math.min(r.left, window.innerWidth - largura - 8))
    });
    this.aberto.set(true);
    afterNextRender(() => {
      const el = this.calendario()?.nativeElement;
      try {
        if (el && !el.matches(':popover-open')) el.showPopover?.();
      } catch {
        el?.showPopover?.();   // ambiente sem :popover-open (testes)
      }
    }, { injector: this.injector });
    this.focarDia();
  }

  /**
   * Rolar (a página ou um modal, por isso em captura no documento) ou redimensionar a janela fecha: a posição fixa
   * ficaria no lugar errado.
   */
  constructor() {
    const rolar = (e: Event) => this.aoRolar(e);
    document.addEventListener('scroll', rolar, true);
    inject(DestroyRef).onDestroy(() => document.removeEventListener('scroll', rolar, true));
  }

  @HostListener('window:resize', ['$event'])
  aoRolar(e: Event): void {
    if (!this.aberto()) return;
    const alvo = e.target as Node | null;
    if (alvo instanceof Node && this.calendario()?.nativeElement.contains(alvo)) return;
    this.fechar(false);
  }

  fechar(devolverFoco: boolean): void {
    if (!this.aberto()) return;
    this.aberto.set(false);
    if (devolverFoco) this.botao().nativeElement.focus();
  }

  escolher(isoEscolhido: string): void {
    if (this.foraDoIntervalo(isoEscolhido)) return;
    this.texto.set(isoParaTexto(isoEscolhido));
    this.empurrarModelo(isoEscolhido);
    this.tocado.set(true);
    this.aoTocar();
    this.fechar(true);
    this.confirmar();
  }

  limpar(): void {
    this.texto.set('');
    this.empurrarModelo('');
    this.tocado.set(true);
    this.fechar(true);
    this.confirmar();
  }

  mudarMes(delta: number): void {
    const { ano, mes } = this.mesVisivel();
    const d = new Date(ano, mes - 1 + delta, 1);
    this.mesVisivel.set({ ano: d.getFullYear(), mes: d.getMonth() + 1 });
    // O foco vai para o mesmo dia no mês novo (ou o último dia, se o mês for mais curto).
    const dia = Math.min(dataDe(this.foco()).getDate(), new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate());
    this.foco.set(iso(new Date(d.getFullYear(), d.getMonth(), dia)));
  }

  /** Teclado do padrão de calendário do WAI-ARIA (Date Picker Dialog). */
  teclaNaGrade(e: KeyboardEvent): void {
    const atual = dataDe(this.foco());
    let novo: Date | null = null;
    switch (e.key) {
      case 'ArrowLeft': novo = new Date(atual.getFullYear(), atual.getMonth(), atual.getDate() - 1); break;
      case 'ArrowRight': novo = new Date(atual.getFullYear(), atual.getMonth(), atual.getDate() + 1); break;
      case 'ArrowUp': novo = new Date(atual.getFullYear(), atual.getMonth(), atual.getDate() - 7); break;
      case 'ArrowDown': novo = new Date(atual.getFullYear(), atual.getMonth(), atual.getDate() + 7); break;
      case 'Home': novo = new Date(atual.getFullYear(), atual.getMonth(), atual.getDate() - atual.getDay()); break;
      case 'End': novo = new Date(atual.getFullYear(), atual.getMonth(), atual.getDate() + (6 - atual.getDay())); break;
      case 'PageUp': this.mudarMes(e.shiftKey ? -12 : -1); break;
      case 'PageDown': this.mudarMes(e.shiftKey ? 12 : 1); break;
      default: return;
    }
    e.preventDefault();
    if (novo) {
      this.foco.set(iso(novo));
      if (novo.getMonth() + 1 !== this.mesVisivel().mes || novo.getFullYear() !== this.mesVisivel().ano) {
        this.mesVisivel.set({ ano: novo.getFullYear(), mes: novo.getMonth() + 1 });
      }
    }
    this.focarDia();
  }

  foraDoIntervalo(isoData: string): boolean {
    return isoData < this.minimoEfetivo() || isoData > this.maximoEfetivo();
  }

  @HostListener('document:mousedown', ['$event'])
  aoClicarFora(e: MouseEvent): void {
    if (this.aberto() && !this.elemento.nativeElement.contains(e.target as Node)) this.fechar(false);
  }

  /** O foco saiu do campo e do calendário (Tab para fora): fecha. */
  @HostListener('focusout', ['$event'])
  aoPerderFoco(e: FocusEvent): void {
    const destino = e.relatedTarget as Node | null;
    if (this.aberto() && destino && !this.elemento.nativeElement.contains(destino)) this.fechar(false);
  }

  /* ------------------------------ interno ------------------------------ */

  private focarDia(): void {
    afterNextRender(() => {
      this.elemento.nativeElement.querySelector<HTMLButtonElement>(`.calendario-dia[data-iso="${this.foco()}"]`)?.focus();
    }, { injector: this.injector });
  }

  /** Emite (alterado) quando a data confirmada mudou (como o (change) do campo nativo). */
  private confirmar(): void {
    const valor = this.valorIso() ?? (this.texto() ? null : '');
    if (valor === null || valor === this.ultimoConfirmado) return;   // incompleta/inválida: espera corrigir
    this.ultimoConfirmado = valor;
    this.alterado.emit(valor);
  }

  private mensagemDeErro(): string | null {
    const { estado, iso: valor } = textoParaIso(this.texto());
    if (estado === 'vazia') return this.obrigatorio() ? 'Informe a data.' : null;
    if (estado === 'incompleta') return 'Data incompleta. Use dd/mm/aaaa.';
    if (estado === 'invalida') return 'Data inválida.';
    if (valor! < this.minimoEfetivo()) return `Use uma data a partir de ${isoParaTexto(this.minimoEfetivo())}.`;
    if (valor! > this.maximoEfetivo()) return `Use uma data até ${isoParaTexto(this.maximoEfetivo())}.`;
    return null;
  }
}
