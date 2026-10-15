import { CurrencyPipe } from '@angular/common';
import { DEFAULT_CURRENCY_CODE, LOCALE_ID, Pipe, PipeTransform, inject } from '@angular/core';
import { PrivacidadeService } from '../core/privacidade.service';

/**
 * Substitui o pipe "currency" do Angular: formata em reais e respeita a opção "Ocultar valores".
 * Impuro para reagir na hora em que o usuário alterna a opção.
 */
@Pipe({ name: 'currency', pure: false })
export class MoedaPipe implements PipeTransform {
  private readonly privacidade = inject(PrivacidadeService);
  private readonly base = new CurrencyPipe(inject(LOCALE_ID), inject(DEFAULT_CURRENCY_CODE));

  transform(valor: number | string | null | undefined, moeda?: string, exibicao?: string, digitos?: string): string | null {
    if (valor === null || valor === undefined || valor === '') return null;
    if (this.privacidade.oculto()) return 'R$ •••••';
    return this.base.transform(valor, moeda, exibicao, digitos);
  }
}
