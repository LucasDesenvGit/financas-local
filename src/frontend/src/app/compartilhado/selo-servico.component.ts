import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { IconeCobrancaPadrao, IconesCategoria } from './icones';
import { LogoBancoComponent } from './logo-banco.component';
import { SERVICOS, palavras, servicoDe } from './servicos';
import { bancoNoTexto, visualBanco } from './bancos';
import { TIPOS_LANCAMENTO } from './tipos-lancamento';
import { TipoLancamento } from '../core/modelos';

/**
 * #265: selo de um lançamento ou de uma cobrança, para qualquer tela que lista dinheiro saindo ou entrando (Transações,
 * Contas a pagar, Painel…). Serviço conhecido (YouTube, TIM, iFood, Mercado Livre…, compartilhado/servicos.ts): o selo
 * da marca, como o dos bancos (ADR 0024). Senão, o ícone da categoria num quadrado neutro; sem categoria conhecida, um
 * recibo. Decorativo: o nome vem escrito ao lado (WCAG 1.1.1).
 *
 * #325: com o `tipo` do lançamento (as etiquetas "Pix recebido", "Pagamento de fatura"…), sem serviço pelo nome:
 * Pix → o selo do Pix; pagamento de fatura → o logo do banco citado no texto. Sem marca nem categoria conhecida, o
 * ícone do tipo (o mesmo da etiqueta) no lugar do recibo. Ordem: serviço, marca do tipo, categoria, ícone do tipo,
 * recibo (julgamento próprio).
 *
 * O serviço é procurado nos textos nesta ordem: `texto` (o do banco: descrição original, texto da cobrança), depois
 * `estabelecimento`, depois `nome` (o que você deu). Exemplos:
 *   Transação:     <app-selo-servico [texto]="t.descricaoOriginal" [estabelecimento]="t.estabelecimento"
 *                    [nome]="t.descricao" [categoria]="t.categoriaNome" />
 *   Conta a pagar: <app-selo-servico [texto]="c.textoCobranca" [nome]="c.descricao" [categoria]="c.categoriaNome" />
 */
const PIX = SERVICOS.find((s) => s.chave === 'pix')!.visual;

@Component({
  selector: 'app-selo-servico',
  imports: [LogoBancoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `<app-logo-banco [visualPronto]="visual()" [iconeGenerico]="iconeCategoria()" [tamanho]="tamanho()" />`,
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
  /** #325: o tipo do lançamento (as etiquetas da transação). */
  readonly tipo = input<TipoLancamento | null | undefined>(null);

  readonly servico = computed(() => servicoDe(this.texto(), this.estabelecimento(), this.nome()));
  readonly visual = computed(() => {
    const s = this.servico();
    if (s) return s.visual;
    const tipo = this.tipo();
    if (tipo === TipoLancamento.PixEnviado || tipo === TipoLancamento.PixRecebido) return PIX;
    if (tipo === TipoLancamento.PagamentoFatura) return visualBanco(bancoNoTexto(this.texto(), this.estabelecimento(), this.nome()));
    return null;
  });
  readonly iconeCategoria = computed(() => {
    const daCategoria = IconesCategoria[palavras(this.categoria()).join(' ')];
    if (daCategoria) return daCategoria;
    const tipo = this.tipo();
    return tipo !== null && tipo !== undefined && tipo !== TipoLancamento.Outro ? TIPOS_LANCAMENTO[tipo].icone : IconeCobrancaPadrao;
  });
}
