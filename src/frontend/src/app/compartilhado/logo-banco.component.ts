import { Component, computed, input, ChangeDetectionStrategy } from '@angular/core';
import { visualBanco } from './bancos';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';

/**
 * Selo do banco de uma conta ou cartão (ADR 0024): quadrado com a cor da marca e o desenho dela (ou uma sigla).
 * Banco não identificado: ícone neutro de banco (ou de cartão). Decorativo por padrão — o nome do banco vem escrito
 * ao lado (WCAG 1.1.1: sem repetir para o leitor de tela); com `rotulo`, vira imagem com nome, para quando aparece
 * sozinho. Uso: <app-logo-banco [codigo]="conta.banco" [tamanho]="40" />.
 */
@Component({
  selector: 'app-logo-banco',
  imports: [IconeComponent],
  template: `
    @if (visual(); as v) {
      <svg class="logo-banco" [attr.width]="tamanho()" [attr.height]="tamanho()" viewBox="0 0 40 40" focusable="false"
        [attr.role]="rotulo() ? 'img' : null" [attr.aria-label]="rotulo() || null" [attr.aria-hidden]="rotulo() ? null : 'true'">
        <rect width="40" height="40" rx="10" [attr.fill]="v.fundo" />
        @if (v.caminho) {
          <path [attr.d]="v.caminho" [attr.fill]="v.cor" transform="translate(8 8)" />
        } @else {
          <text x="20" y="21" text-anchor="middle" dominant-baseline="middle" font-weight="750"
            [attr.font-size]="tamanhoSigla(v.sigla ?? '')" [attr.fill]="v.cor">{{ v.sigla }}</text>
        }
      </svg>
    } @else {
      <span class="logo-banco logo-generico" [style.width.px]="tamanho()" [style.height.px]="tamanho()"
        [attr.role]="rotulo() ? 'img' : null" [attr.aria-label]="rotulo() || null" [attr.aria-hidden]="rotulo() ? null : 'true'">
        <app-icone [icone]="cartao() ? Icones.contas : Icones.banco" [tamanho]="tamanho() * 0.5" />
      </span>
    }
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [`
    :host { display: inline-flex; flex-shrink: 0; line-height: 0; }
    .logo-banco { display: block; border-radius: 22%; }
    .logo-generico {
      display: grid; place-items: center; border-radius: 22%;
      background: var(--superficie-2); color: var(--texto-suave);
    }
    text { font-family: inherit; letter-spacing: -0.02em; }
  `]
})
export class LogoBancoComponent {
  readonly Icones = Icones;
  /** Código do banco (o de `Conta.banco`); null = não identificado. */
  readonly codigo = input<string | null | undefined>(null);
  readonly tamanho = input(36);
  /** Nome para leitor de tela quando o selo aparece sozinho (sem o nome do banco escrito ao lado). */
  readonly rotulo = input<string | null>(null);
  /** Banco não identificado num cartão: ícone de cartão em vez de banco. */
  readonly cartao = input(false);

  readonly visual = computed(() => visualBanco(this.codigo()));

  /** Sigla maior quando é curta: "C6" grande, "sicredi" menor, sempre dentro do quadrado. */
  tamanhoSigla(sigla: string): number {
    const n = sigla.length;
    return n <= 2 ? 17 : n === 3 ? 14 : n === 4 ? 12.5 : n === 5 ? 10.5 : n === 6 ? 9 : 8;
  }
}
