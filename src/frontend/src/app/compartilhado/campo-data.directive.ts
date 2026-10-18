import { Directive, computed, forwardRef, input } from '@angular/core';
import { AbstractControl, NG_VALIDATORS, ValidationErrors, Validator } from '@angular/forms';

/** Intervalo padrão dos campos de data do site: cobre extratos antigos e metas de longo prazo. */
export const DATA_MINIMA = '1900-01-01';
export const DATA_MAXIMA = '2099-12-31';

/**
 * Todo <input type="date"> do site ganha limites (padrão 1900–2099; a tela pode passar os seus, ex.: [max]="hoje").
 * Sem `max`, o Chrome/Edge aceitam ano com até 6 dígitos ("13/02/275760"); com um `max` de 4 dígitos, o próprio
 * navegador limita a digitação do ano a 4 (Nielsen 5, Error Prevention). Data fora do intervalo deixa o formulário
 * inválido (o "Salvar" fica desabilitado como em qualquer campo inválido) e o campo com aria-invalid.
 * Uso: basta importar a diretiva no componente; o seletor pega todo input de data.
 */
@Directive({
  selector: 'input[type=date]',
  providers: [{ provide: NG_VALIDATORS, useExisting: forwardRef(() => CampoDataDirective), multi: true }],
  host: {
    '[attr.min]': 'minimo()',
    '[attr.max]': 'maximo()'
  }
})
export class CampoDataDirective implements Validator {
  /** Limites da tela, quando houver (ex.: [max]="hoje"); senão, o padrão. */
  readonly min = input<string | null | undefined>(undefined);
  readonly max = input<string | null | undefined>(undefined);
  readonly minimo = computed(() => this.min() || DATA_MINIMA);
  readonly maximo = computed(() => this.max() || DATA_MAXIMA);

  validate(controle: AbstractControl): ValidationErrors | null {
    const valor = controle.value as string | null;
    if (!valor) return null;
    // AAAA-MM-DD compara certo como texto; ano com mais de 4 dígitos nem chega a casar com o formato.
    if (!/^\d{4}-\d{2}-\d{2}$/.test(valor)) return { dataInvalida: true };
    if (valor < this.minimo()) return { dataAntesDoMinimo: this.minimo() };
    if (valor > this.maximo()) return { dataDepoisDoMaximo: this.maximo() };
    return null;
  }
}
