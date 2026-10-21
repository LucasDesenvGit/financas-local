import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { IconeCobrancaPadrao, IconesCategoria } from './icones';
import { LogoBancoComponent } from './logo-banco.component';
import { palavras, servicoDe } from './servicos';

/**
 * #265: selo de um lançamento ou de uma cobrança, para qualquer tela que lista dinheiro saindo ou entrando (Transações,
 * Contas a pagar, Painel…). Serviço conhecido (YouTube, TIM, iFood, Mercado Livre…, compartilhado/servicos.ts): o selo
 * da marca, como o dos bancos (ADR 0024). Senão, o ícone da categoria num quadrado neutro; sem categoria conhecida, um
 * recibo. Decorativo: o nome vem escrito ao lado (WCAG 1.1.1).
 *
 * O serviço é procurado nos textos nesta ordem: `texto` (o do banco: descrição original, texto da cobrança), depois
 * `estabelecimento`, depois `nome` (o que você deu). Exemplos:
 *   Transação:     <app-selo-servico [texto]="t.descricaoOriginal" [estabelecimento]="t.estabelecimento"
 *                    [nome]="t.descricao" [categoria]="t.categoriaNome" />
 *   Conta a pagar: <app-selo-servico [texto]="c.textoCobranca" [nome]="c.descricao" [categoria]="c.categoriaNome" />
 */
@Component({
  selector: 'app-selo-servico',
  imports: [LogoBancoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `<app-logo-banco [visualPronto]="servico()?.visual ?? null" [iconeGenerico]="iconeCategoria()" [tamanho]="tamanho()" />`,
  styles: [`:host { display: inline-flex; flex-shrink: 0; line-height: 0; }`]
})
export class SeloServicoComponent {
  /** O texto do banco (descrição original da transação, texto da cobrança): o mais certeiro para reconhecer o serviço. */
  readonly texto = input<string | null | undefined>(null);
  /** O estabelecimento que o Open Finance informa, quando há. */
  readonly estabelecimento = input<string | null | undefined>(null);
  /** O nome que você deu (ou a descrição). */
  readonly nome = input<string | null | undefined>(null);
  readonly categoria = input<string | null | undefined>(null);
  readonly tamanho = input(24);

  readonly servico = computed(() => servicoDe(this.texto(), this.estabelecimento(), this.nome()));
  readonly iconeCategoria = computed(() => IconesCategoria[palavras(this.categoria()).join(' ')] ?? IconeCobrancaPadrao);
}
