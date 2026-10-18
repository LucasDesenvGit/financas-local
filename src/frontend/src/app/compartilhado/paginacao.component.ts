import { DecimalPipe } from '@angular/common';
import { Component, computed, input, output } from '@angular/core';
import { ListaSuspensaDirective } from './lista-suspensa.directive';

/** Item da barra: número de página ou reticências. */
type ItemPagina = number | '…';

/**
 * Paginação completa: primeira/última, anterior/próxima, janela de números com reticências,
 * campo "ir para" e seletor de itens por página.
 */
@Component({
  selector: 'app-paginacao',
  imports: [ListaSuspensaDirective, DecimalPipe],
  template: `
    @if (total() > 0) {
      <nav class="paginacao" aria-label="Paginação">
        <span class="paginacao-info texto-suave">
          {{ inicio() | number }}–{{ fim() | number }} de {{ total() | number }}
        </span>

        @if (totalPaginas() > 1) {
          <div class="paginacao-botoes">
            <button type="button" class="pagina-botao" [disabled]="pagina() <= 1" (click)="ir(1)" title="Primeira página" aria-label="Primeira página">«</button>
            <button type="button" class="pagina-botao" [disabled]="pagina() <= 1" (click)="ir(pagina() - 1)" title="Página anterior" aria-label="Página anterior">‹</button>
            @for (item of itens(); track $index) {
              @if (item === '…') {
                <span class="pagina-reticencias" aria-hidden="true">…</span>
              } @else {
                <button type="button" class="pagina-botao" [class.atual]="item === pagina()"
                  [attr.aria-current]="item === pagina() ? 'page' : null" (click)="ir(item)">{{ item }}</button>
              }
            }
            <button type="button" class="pagina-botao" [disabled]="pagina() >= totalPaginas()" (click)="ir(pagina() + 1)" title="Próxima página" aria-label="Próxima página">›</button>
            <button type="button" class="pagina-botao" [disabled]="pagina() >= totalPaginas()" (click)="ir(totalPaginas())" title="Última página" aria-label="Última página">»</button>
          </div>
        }

        <div class="paginacao-opcoes">
          @if (totalPaginas() > 7) {
            <label class="paginacao-ir">Ir para
              <input #campo type="number" min="1" [max]="totalPaginas()" [value]="pagina()"
                (keyup.enter)="irPara(campo.value)" (change)="irPara(campo.value)" aria-label="Número da página">
            </label>
          }
          <label class="paginacao-tamanho">Por página
            <select [value]="tamanho()" (change)="mudarTamanho($any($event.target).value)">
              @for (t of tamanhos; track t) { <option [value]="t" [selected]="t === tamanho()">{{ t }}</option> }
            </select>
          </label>
        </div>
      </nav>
    }
  `
})
export class PaginacaoComponent {
  readonly pagina = input.required<number>();
  readonly tamanho = input.required<number>();
  readonly total = input.required<number>();
  readonly paginaMudou = output<number>();
  readonly tamanhoMudou = output<number>();

  readonly tamanhos = [25, 50, 100, 200];

  readonly totalPaginas = computed(() => Math.max(1, Math.ceil(this.total() / this.tamanho())));
  readonly inicio = computed(() => (this.total() === 0 ? 0 : (this.pagina() - 1) * this.tamanho() + 1));
  readonly fim = computed(() => Math.min(this.total(), this.pagina() * this.tamanho()));

  /** Sempre mostra a primeira, a última e duas páginas de cada lado da atual. */
  readonly itens = computed<ItemPagina[]>(() => {
    const total = this.totalPaginas();
    const atual = this.pagina();
    if (total <= 9) return Array.from({ length: total }, (_, i) => i + 1);

    const itens: ItemPagina[] = [1];
    let inicio = Math.max(2, atual - 2);
    let fim = Math.min(total - 1, atual + 2);
    if (atual <= 4) { inicio = 2; fim = 6; }
    if (atual >= total - 3) { inicio = total - 5; fim = total - 1; }
    if (inicio > 2) itens.push('…');
    for (let p = inicio; p <= fim; p++) itens.push(p);
    if (fim < total - 1) itens.push('…');
    itens.push(total);
    return itens;
  });

  ir(pagina: number): void {
    const destino = Math.min(Math.max(1, Math.trunc(pagina)), this.totalPaginas());
    if (destino !== this.pagina()) this.paginaMudou.emit(destino);
  }

  irPara(valor: string): void {
    const n = Number(valor);
    if (Number.isFinite(n) && n >= 1) this.ir(n);
  }

  mudarTamanho(valor: string): void {
    const n = Number(valor);
    if (this.tamanhos.includes(n) && n !== this.tamanho()) this.tamanhoMudou.emit(n);
  }
}
