import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { desenharQr } from './acesso-celular';

/**
 * #677: um QR code com a legenda embaixo. Sempre preto no branco, também no tema escuro: a câmera do celular lê melhor
 * o código escuro em fundo claro, e muitos leitores não leem o invertido. O endereço continua escrito no passo a passo:
 * o QR é um atalho, e a legenda diz o que ele abre (WCAG 1.1.1, alternativa em texto).
 */
@Component({
  selector: 'app-qr-code',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <figure class="qr-code">
      <svg [attr.viewBox]="'0 0 ' + desenho().lado + ' ' + desenho().lado" role="img" [attr.aria-label]="rotulo()"
        shape-rendering="crispEdges">
        <rect [attr.width]="desenho().lado" [attr.height]="desenho().lado" fill="#ffffff" />
        <path [attr.d]="desenho().caminho" fill="#000000" />
      </svg>
      <figcaption>{{ legenda() }}</figcaption>
    </figure>
  `,
  styles: [`
    .qr-code { margin: 0; display: grid; justify-items: center; gap: 6px; width: 168px; }
    .qr-code svg { width: 168px; height: 168px; border-radius: 6px; }
    .qr-code figcaption { font-size: 0.85rem; text-align: center; }
  `]
})
export class QrCodeComponent {
  /** O endereço que o QR abre. */
  readonly texto = input.required<string>();
  /** O que o leitor de tela diz da imagem. */
  readonly rotulo = input.required<string>();
  /** O que aparece embaixo do código. */
  readonly legenda = input.required<string>();

  readonly desenho = computed(() => desenharQr(this.texto()));
}
