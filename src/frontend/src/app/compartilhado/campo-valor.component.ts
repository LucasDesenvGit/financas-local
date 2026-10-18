import { Component, booleanAttribute, computed, forwardRef, input, output, signal } from '@angular/core';
import { AbstractControl, ControlValueAccessor, NG_VALIDATORS, NG_VALUE_ACCESSOR, ValidationErrors, Validator } from '@angular/forms';

export const SIMBOLOS_MOEDA: Record<string, string> = { BRL: 'R$', USD: 'US$', EUR: '€', GBP: '£', JPY: '¥', CHF: 'CHF' };
const NOMES_MOEDA: Record<string, string> = { BRL: 'reais', USD: 'dólares', EUR: 'euros', GBP: 'libras', JPY: 'ienes', CHF: 'francos suíços' };

export function simboloMoeda(moeda: string | null | undefined): string {
  const m = moeda || 'BRL';
  return SIMBOLOS_MOEDA[m] ?? m;
}

/**
 * Lê um valor em dinheiro do jeito brasileiro (#132): "1234,56", "1.234,56", "R$ 1.234,56" (colado do app do banco),
 * "-50" e também "1234.56" (ponto como decimal). Devolve null para vazio e NaN para texto que não é número.
 * - Com ponto e vírgula, o último dos dois é o decimal ("1.234,56" e "1,234.56").
 * - Só vírgula: uma vírgula é o decimal; várias são milhar.
 * - Só ponto: um ponto seguido de exatamente 3 dígitos é milhar ("1.500" = 1500); com outra quantidade de dígitos
 *   é decimal ("1234.56", "0.1234"). Vários pontos são milhar ("1.234.567").
 */
export function interpretarValor(texto: string | null | undefined): number | null {
  let t = (texto ?? '').replace(/[\s ]/g, '');
  if (!t) return null;
  t = t.replace(/^(-?)(R\$|US\$|€|£|¥|CHF|\$)/i, '$1').replace(/^(R\$|US\$|€|£|¥|CHF|\$)-/i, '-');
  if (!/^-?[\d.,]+$/.test(t) || !/\d/.test(t)) return NaN;
  const ultimoPonto = t.lastIndexOf('.');
  const ultimaVirgula = t.lastIndexOf(',');
  let normal: string;
  if (ultimoPonto >= 0 && ultimaVirgula >= 0) {
    const decimal = ultimoPonto > ultimaVirgula ? '.' : ',';
    const milhar = decimal === '.' ? ',' : '.';
    if (t.indexOf(decimal) !== t.lastIndexOf(decimal)) return NaN;
    normal = t.split(milhar).join('').replace(decimal, '.');
  } else if (ultimaVirgula >= 0) {
    normal = t.indexOf(',') === ultimaVirgula ? t.replace(',', '.') : t.split(',').join('');
  } else if (ultimoPonto >= 0) {
    const pontos = t.split('.').length - 1;
    const depois = t.length - ultimoPonto - 1;
    normal = pontos > 1 || depois === 3 ? t.split('.').join('') : t;
  } else {
    normal = t;
  }
  const n = Number(normal);
  return Number.isFinite(n) ? n : NaN;
}

/** "1.234,56" (pt-BR), com 2 casas no mínimo e `casas` no máximo. */
export function formatarValor(valor: number, casas = 2): string {
  return valor.toLocaleString('pt-BR', { minimumFractionDigits: Math.min(2, casas), maximumFractionDigits: casas });
}

/** Caracteres que contam para a posição do cursor (o que a pessoa digitou; os pontos de milhar são do campo). */
const SIGNIFICATIVO = /[\d,-]/;

/**
 * Formatação enquanto se digita (#132): põe os pontos de milhar e mantém o cursor no mesmo lugar em relação aos
 * dígitos. Ponto digitado vira a vírgula decimal (`pontoDigitado`); letras são ignoradas; no máximo `casas` casas.
 * Devolve o texto novo e a posição do cursor nele.
 */
export function formatarDigitando(texto: string, cursor: number, casas = 2, negativo = false,
  pontoDigitado = false): { texto: string; cursor: number } {
  let t = texto;
  if (pontoDigitado && t[cursor - 1] === '.') t = t.slice(0, cursor - 1) + ',' + t.slice(cursor);
  // Quantos caracteres que importam estão antes do cursor (para devolvê-lo ao mesmo lugar).
  let antes = [...t.slice(0, cursor)].filter((c) => SIGNIFICATIVO.test(c)).length;
  const sinal = negativo && t.trim().startsWith('-') ? '-' : '';
  const limpo = t.replace(/[^\d,]/g, '');
  const virgula = limpo.indexOf(',');
  let inteiro = virgula >= 0 ? limpo.slice(0, virgula) : limpo;
  const decimais = virgula >= 0 ? limpo.slice(virgula + 1).replace(/,/g, '').slice(0, casas) : '';
  inteiro = inteiro.replace(/^0+(?=\d)/, '');
  if (!inteiro && virgula >= 0) {
    inteiro = '0';                       // ",5" vira "0,5": o cursor anda junto com o zero acrescentado
    if (antes > 0) antes++;
  }
  const comMilhar = inteiro.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
  const novo = sinal + comMilhar + (virgula >= 0 ? ',' + decimais : '');
  // Cursor: depois do mesmo número de caracteres que importam.
  let posicao = 0;
  for (let vistos = 0; posicao < novo.length && vistos < antes; posicao++) {
    if (SIGNIFICATIVO.test(novo[posicao])) vistos++;
  }
  return { texto: novo, cursor: Math.min(posicao, novo.length) };
}

let proximoId = 0;

/**
 * Campo de valor em dinheiro padrão (#132, ADR 0032), no lugar de <input type="number">:
 * - entende o jeito brasileiro (1.234,56) e o "R$" colado do banco; "1.500" é mil e quinhentos, não 1,5;
 * - texto com inputmode="decimal": no celular abre o teclado numérico, sem setinhas e sem a rodinha do mouse mudar
 *   o valor sem querer;
 * - formata enquanto se digita (pontos de milhar, sem tirar o cursor do lugar) e completa as casas ao sair do campo;
 * - o símbolo da moeda fica dentro do campo (decorativo); o nome acessível diz a moeda ("em reais");
 * - erro em texto, perto do campo e ligado por aria-describedby (WCAG 3.3.1).
 * Entrega um number (ou null) ao ngModel, como o campo numérico de antes: a API não muda.
 * Uso: <label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>
 */
@Component({
  selector: 'app-campo-valor',
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => CampoValorComponent), multi: true },
    { provide: NG_VALIDATORS, useExisting: forwardRef(() => CampoValorComponent), multi: true }
  ],
  template: `
    <span class="campo-valor" [class.campo-valor-erro]="erroVisivel()" [class.campo-valor-compacto]="compacto()">
      <span class="campo-valor-simbolo" aria-hidden="true">{{ simbolo() }}</span>
      <span class="so-leitor">(em {{ nomeMoeda() }})</span>
      <input type="text" inputmode="decimal" autocomplete="off" [attr.name]="name() || null" [value]="texto()"
        [disabled]="desabilitado()" [attr.aria-required]="obrigatorio() || null" [attr.aria-invalid]="erroVisivel() ? 'true' : null"
        [attr.aria-describedby]="erroVisivel() ? idErro : null" [attr.aria-label]="rotulo() || null"
        (input)="aoDigitar($event)" (blur)="aoSair()" (focus)="aoEntrar()"
        (keydown.enter)="aoConfirmarEnter($event)">
    </span>
    @if (erroVisivel()) {
      <!-- aria-hidden: fora do nome do <label> que envolve o campo; o input aponta para cá pelo aria-describedby. -->
      <span class="campo-valor-mensagem" [id]="idErro" aria-hidden="true">{{ erro() }}</span>
    }
  `
})
export class CampoValorComponent implements ControlValueAccessor, Validator {
  readonly moeda = input<string | null | undefined>('BRL');
  /** Máximo de casas decimais (cotação: 4). Mostra sempre pelo menos 2. */
  readonly casas = input(2);
  readonly minimo = input<number | null>(null);
  readonly obrigatorio = input(false, { transform: booleanAttribute });
  /** Saldo inicial pode ser negativo; o resto não. */
  readonly negativo = input(false, { transform: booleanAttribute });
  readonly compacto = input(false, { transform: booleanAttribute });
  readonly name = input<string>('');
  /** Nome acessível quando o campo não está dentro de um <label> (ex.: célula de tabela). */
  readonly rotulo = input<string>('');
  /** Valor confirmado: ao sair do campo com o valor mudado, ou com Enter (ex.: orçamento salvo na própria linha). */
  readonly confirmado = output<number | null>();
  /** Enter confirma o valor aqui, em vez de enviar o formulário em volta (orçamento salvo na própria linha). */
  readonly enterConfirma = input(false, { transform: booleanAttribute });

  readonly idErro = `campo-valor-erro-${++proximoId}`;
  readonly texto = signal('');
  readonly desabilitado = signal(false);
  private readonly tocado = signal(false);
  readonly simbolo = computed(() => simboloMoeda(this.moeda()));
  readonly nomeMoeda = computed(() => NOMES_MOEDA[this.moeda() || 'BRL'] ?? this.moeda());

  readonly erro = computed(() => this.mensagemDeErro(this.texto()));
  readonly erroVisivel = computed(() => this.tocado() && this.erro() !== null);

  private aoMudar: (v: number | null) => void = () => undefined;
  private aoTocar: () => void = () => undefined;
  private valorAoEntrar: number | null = null;

  /* ------------------------------ ControlValueAccessor ------------------------------ */

  writeValue(valor: number | null | undefined): void {
    this.texto.set(valor === null || valor === undefined || Number.isNaN(valor) ? '' : formatarValor(valor, this.casas()));
  }
  registerOnChange(fn: (v: number | null) => void): void { this.aoMudar = fn; }
  registerOnTouched(fn: () => void): void { this.aoTocar = fn; }
  setDisabledState(desabilitado: boolean): void { this.desabilitado.set(desabilitado); }

  /* ------------------------------ Validator ------------------------------ */

  validate(_: AbstractControl): ValidationErrors | null {
    const mensagem = this.erro();
    return mensagem ? { campoValor: mensagem } : null;
  }

  /* ------------------------------ eventos ------------------------------ */

  aoDigitar(e: Event): void {
    const campo = e.target as HTMLInputElement;
    const evento = e as InputEvent;
    const anterior = this.texto();
    let texto = campo.value;
    // Colar, arrastar ou trocar o texto todo de uma vez: lê do jeito completo ("R$ 1.234,56", "1234.56").
    const colado = evento.inputType === 'insertFromPaste' || evento.inputType === 'insertFromDrop'
      || (!evento.inputType && texto.length - anterior.length > 1);
    if (colado) {
      const n = interpretarValor(texto);
      if (n !== null && !Number.isNaN(n)) texto = formatarValor(n, this.casas());
      campo.value = texto;
    } else {
      // Digitando: pontos de milhar na hora, sem tirar o cursor do lugar.
      let cursor = campo.selectionStart ?? texto.length;
      // Backspace/Delete sobre um ponto de milhar apaga o dígito ao lado (senão o ponto voltaria e nada mudaria).
      if (anterior.length - texto.length === 1 && anterior[cursor] === '.') {
        if (evento.inputType === 'deleteContentBackward' && cursor > 0) {
          texto = texto.slice(0, cursor - 1) + texto.slice(cursor);
          cursor--;
        } else if (evento.inputType === 'deleteContentForward') {
          texto = texto.slice(0, cursor) + texto.slice(cursor + 1);
        }
      }
      const r = formatarDigitando(texto, cursor, this.casas(), this.negativo(), evento.data === '.');
      texto = r.texto;
      if (campo.value !== texto) {
        campo.value = texto;
        campo.setSelectionRange?.(r.cursor, r.cursor);
      }
    }
    this.texto.set(texto);
    this.aoMudar(this.valorValido(texto));
  }

  aoEntrar(): void {
    this.valorAoEntrar = this.valorValido(this.texto());
  }

  aoSair(): void {
    this.tocado.set(true);
    this.aoTocar();
    const valor = this.valorValido(this.texto());
    if (valor !== null) this.texto.set(formatarValor(valor, this.casas()));
    if (this.erro() === null && valor !== this.valorAoEntrar) {
      this.valorAoEntrar = valor;
      this.confirmado.emit(valor);
    }
  }

  aoConfirmarEnter(e: Event): void {
    if (!this.enterConfirma()) return;   // num formulário, o Enter continua enviando o formulário
    e.preventDefault();
    this.aoSair();
  }

  /* ------------------------------ regras ------------------------------ */

  /** Número arredondado às casas do campo, ou null (vazio ou inválido). */
  private valorValido(texto: string): number | null {
    const n = interpretarValor(texto);
    if (n === null || Number.isNaN(n)) return null;
    const fator = 10 ** this.casas();
    return Math.round(n * fator) / fator;
  }

  private mensagemDeErro(texto: string): string | null {
    const n = interpretarValor(texto);
    if (n === null) return this.obrigatorio() ? 'Informe o valor.' : null;
    if (Number.isNaN(n)) return 'Valor inválido. Use, por exemplo, 1.234,56.';
    if (n < 0 && !this.negativo()) return 'Informe um valor positivo.';
    const minimo = this.minimo();
    if (minimo !== null && n < minimo) {
      return minimo > 0 && minimo <= 0.01 ? 'Informe um valor maior que zero.' : `Informe pelo menos ${formatarValor(minimo, this.casas())}.`;
    }
    return null;
  }
}
