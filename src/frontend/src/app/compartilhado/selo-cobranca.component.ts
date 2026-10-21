import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { IconeCobrancaPadrao, IconesCategoria } from './icones';
import { LogoBancoComponent } from './logo-banco.component';
import { palavras, servicoDe } from './servicos';

/**
 * #265: selo de uma cobrança (conta a pagar, recorrente, sugestão do extrato). Serviço conhecido (YouTube, TIM,
 * iFood…, compartilhado/servicos.ts): o selo da marca, como o dos bancos (ADR 0024). Senão, o ícone da categoria num
 * quadrado neutro; sem categoria conhecida, um recibo. Decorativo: o nome da cobrança vem escrito ao lado (WCAG 1.1.1).
 * Uso: <app-selo-cobranca [texto]="c.textoCobranca" [nome]="c.descricao" [categoria]="c.categoriaNome" />.
 */
@Component({
  selector: 'app-selo-cobranca',
  imports: [LogoBancoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `<app-logo-banco [visualPronto]="servico()?.visual ?? null" [iconeGenerico]="iconeCategoria()" [tamanho]="tamanho()" />`,
  styles: [`:host { display: inline-flex; flex-shrink: 0; line-height: 0; }`]
})
export class SeloCobrancaComponent {
  /** O texto que o banco usa na cobrança (o mais certeiro para reconhecer o serviço). */
  readonly texto = input<string | null | undefined>(null);
  /** O nome que você deu (ou a descrição da sugestão). */
  readonly nome = input<string | null | undefined>(null);
  readonly categoria = input<string | null | undefined>(null);
  readonly tamanho = input(24);

  readonly servico = computed(() => servicoDe(this.texto(), this.nome()));
  readonly iconeCategoria = computed(() => IconesCategoria[palavras(this.categoria()).join(' ')] ?? IconeCobrancaPadrao);
}
