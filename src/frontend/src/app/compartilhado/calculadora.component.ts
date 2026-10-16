import { Component, HostListener, signal } from '@angular/core';
import { DicaDirective } from './dica.directive';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

interface ItemHistorico { expressao: string; resultado: number; }

/**
 * Calculadora da barra superior (UX-7: ícone na barra, no mesmo lugar em desktop e celular; o painel abre ancorado
 * ao botão). Aceita teclado: números, + - * / % ( ) , . Enter, Backspace e Esc; Alt+C abre/fecha de qualquer tela.
 * O cálculo é feito por um analisador próprio (sem eval), compatível com a CSP do site.
 * "100 + 10%" = 110 e "200 - 15%" = 170, como nas calculadoras de bolso.
 */
@Component({
  selector: 'app-calculadora',
  imports: [DicaDirective, IconeComponent],
  host: { class: 'calc' },
  template: `
    <button type="button" class="botao-barra calc-botao" (click)="alternar()" [attr.aria-expanded]="aberta()"
      aria-controls="calc-painel" aria-label="Calculadora (Alt+C)" appDica="Calculadora (Alt+C)" dicaLado="abaixo">
      <app-icone [icone]="aberta() ? Icones.fechar : Icones.calculadora" [tamanho]="18" />
    </button>

    @if (aberta()) {
      <section id="calc-painel" class="calc-painel" role="dialog" aria-label="Calculadora">
        <div class="calc-visor">
          <div class="calc-expressao">{{ expressao() || ' ' }}</div>
          <div class="calc-resultado" aria-live="polite">{{ visor() }}</div>
        </div>
        <div class="calc-teclas">
          <button type="button" class="limpar" (click)="limpar()">C</button>
          <button type="button" class="op" (click)="digitar('(')">(</button>
          <button type="button" class="op" (click)="digitar(')')">)</button>
          <button type="button" class="op" (click)="apagar()" aria-label="Apagar">⌫</button>

          <button type="button" (click)="digitar('7')">7</button>
          <button type="button" (click)="digitar('8')">8</button>
          <button type="button" (click)="digitar('9')">9</button>
          <button type="button" class="op" (click)="digitar('÷')">÷</button>

          <button type="button" (click)="digitar('4')">4</button>
          <button type="button" (click)="digitar('5')">5</button>
          <button type="button" (click)="digitar('6')">6</button>
          <button type="button" class="op" (click)="digitar('×')">×</button>

          <button type="button" (click)="digitar('1')">1</button>
          <button type="button" (click)="digitar('2')">2</button>
          <button type="button" (click)="digitar('3')">3</button>
          <button type="button" class="op" (click)="digitar('−')">−</button>

          <button type="button" (click)="digitar('0')">0</button>
          <button type="button" (click)="digitar(',')">,</button>
          <button type="button" class="op" (click)="digitar('%')">%</button>
          <button type="button" class="op" (click)="digitar('+')">+</button>

          <button type="button" class="igual" style="grid-column: span 4" (click)="calcular()">=</button>
        </div>
        <div class="calc-rodape">
          <span class="texto-suave">teclado liberado</span>
          <button type="button" class="link" (click)="copiar()" [disabled]="resultado() === null">copiar resultado</button>
        </div>
        @if (historico().length) {
          <ul class="calc-historico" aria-label="Últimos cálculos">
            @for (h of historico(); track $index) {
              <li (click)="reutilizar(h)" title="Usar este resultado">
                <span class="truncar">{{ h.expressao }}</span><strong>{{ formatar(h.resultado) }}</strong>
              </li>
            }
          </ul>
        }
      </section>
    }
  `
})
export class CalculadoraComponent {
  readonly Icones = Icones;
  readonly aberta = signal(false);
  readonly expressao = signal('');
  readonly resultado = signal<number | null>(null);
  readonly erro = signal(false);
  readonly historico = signal<ItemHistorico[]>([]);

  visor(): string {
    if (this.erro()) return 'Erro';
    const r = this.resultado();
    if (r !== null) return this.formatar(r);
    const previa = this.avaliar(this.expressao());
    return previa === null ? '0' : this.formatar(previa);
  }

  alternar(): void {
    this.aberta.update((v) => !v);
  }

  digitar(t: string): void {
    // Depois de um resultado: operador continua a conta, número começa outra.
    if (this.resultado() !== null) {
      const operador = ['+', '−', '×', '÷', '%'].includes(t);
      this.expressao.set(operador ? this.formatarEntrada(this.resultado()!) : '');
      this.resultado.set(null);
    }
    this.erro.set(false);
    this.expressao.update((e) => (e + t).slice(0, 120));
  }

  apagar(): void {
    this.resultado.set(null);
    this.erro.set(false);
    this.expressao.update((e) => e.slice(0, -1));
  }

  limpar(): void {
    this.expressao.set('');
    this.resultado.set(null);
    this.erro.set(false);
  }

  calcular(): void {
    const e = this.expressao();
    if (!e.trim()) return;
    const r = this.avaliar(e);
    if (r === null) {
      this.erro.set(true);
      return;
    }
    this.resultado.set(r);
    this.historico.update((h) => [{ expressao: e, resultado: r }, ...h].slice(0, 8));
  }

  reutilizar(h: ItemHistorico): void {
    this.expressao.set(this.formatarEntrada(h.resultado));
    this.resultado.set(null);
  }

  copiar(): void {
    const r = this.resultado();
    if (r === null) return;
    navigator.clipboard?.writeText(this.formatarEntrada(r)).catch(() => { /* sem permissão: ignora */ });
  }

  formatar(n: number): string {
    return n.toLocaleString('pt-BR', { maximumFractionDigits: 8 });
  }

  /** Número no formato de digitação (vírgula decimal, sem separador de milhar). */
  private formatarEntrada(n: number): string {
    return String(Math.round(n * 1e8) / 1e8).replace('.', ',');
  }

  @HostListener('document:keydown', ['$event'])
  aoTeclar(ev: KeyboardEvent): void {
    if (ev.altKey && (ev.key === 'c' || ev.key === 'C')) {
      ev.preventDefault();
      this.alternar();
      return;
    }
    if (!this.aberta()) return;
    // Não rouba o teclado de campos de formulário.
    const alvo = ev.target as HTMLElement | null;
    if (alvo && (alvo.tagName === 'INPUT' || alvo.tagName === 'TEXTAREA' || alvo.tagName === 'SELECT' || alvo.isContentEditable)) return;
    if (ev.ctrlKey || ev.metaKey) return;

    const k = ev.key;
    const mapa: Record<string, string> = { '*': '×', 'x': '×', '/': '÷', '-': '−', '.': ',', '+': '+', '%': '%', '(': '(', ')': ')', ',': ',' };
    if (/^[0-9]$/.test(k)) this.digitar(k);
    else if (mapa[k]) this.digitar(mapa[k]);
    else if (k === 'Enter' || k === '=') this.calcular();
    else if (k === 'Backspace') this.apagar();
    else if (k === 'Escape') { if (this.expressao()) this.limpar(); else this.aberta.set(false); }
    else if (k === 'Delete') this.limpar();
    else return;
    ev.preventDefault();
  }

  /* ------------------------------ analisador ------------------------------ */

  /** Avalia a expressão; retorna null se estiver incompleta ou inválida. */
  private avaliar(texto: string): number | null {
    const tokens = this.tokenizar(texto);
    if (!tokens || tokens.length === 0) return null;
    let pos = 0;
    const ver = () => tokens[pos];
    const pegar = () => tokens[pos++];

    type Valor = { v: number; pct: boolean };

    const fator = (): Valor => {
      const t = pegar();
      if (t === undefined) throw new Error('fim');
      let valor: Valor;
      if (t === '−' || t === '+') {
        const f = fator();
        valor = { v: t === '−' ? -f.v : f.v, pct: f.pct };
        return valor;
      }
      if (t === '(') {
        const v = soma();
        if (pegar() !== ')') throw new Error('parêntese');
        valor = { v, pct: false };
      } else if (typeof t === 'number') {
        valor = { v: t, pct: false };
      } else {
        throw new Error('token');
      }
      while (ver() === '%') {
        pegar();
        valor = { v: valor.v / 100, pct: true };
      }
      return valor;
    };

    const produto = (): Valor => {
      let esq = fator();
      while (ver() === '×' || ver() === '÷') {
        const op = pegar();
        const dir = fator();
        esq = { v: op === '×' ? esq.v * dir.v : esq.v / dir.v, pct: false };
      }
      return esq;
    };

    const soma = (): number => {
      let esq = produto().v;
      while (ver() === '+' || ver() === '−') {
        const op = pegar();
        const dir = produto();
        // "a + b%" soma b% de a.
        const valor = dir.pct ? esq * dir.v : dir.v;
        esq = op === '+' ? esq + valor : esq - valor;
      }
      return esq;
    };

    try {
      const r = soma();
      if (pos !== tokens.length || !Number.isFinite(r)) return null;
      return Math.round(r * 1e10) / 1e10;
    } catch {
      return null;
    }
  }

  private tokenizar(texto: string): (number | string)[] | null {
    const tokens: (number | string)[] = [];
    let i = 0;
    while (i < texto.length) {
      const c = texto[i];
      if (c === ' ') { i++; continue; }
      if (/[0-9,]/.test(c)) {
        let j = i;
        while (j < texto.length && /[0-9,]/.test(texto[j])) j++;
        const numero = Number(texto.slice(i, j).replace(',', '.'));
        if (Number.isNaN(numero) || (texto.slice(i, j).match(/,/g)?.length ?? 0) > 1) return null;
        tokens.push(numero);
        i = j;
        continue;
      }
      if ('+−×÷%()'.includes(c)) { tokens.push(c); i++; continue; }
      return null;
    }
    return tokens;
  }
}
