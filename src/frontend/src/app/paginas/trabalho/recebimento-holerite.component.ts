import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, effect, inject, input, signal } from '@angular/core';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { RecebimentosHolerite } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { situacaoDoRecebimento } from './recebimento';

/**
 * #212 (ADR 0065): "Recebido na conta", dentro do holerite. Os créditos ligados (com a parte de cada um), a diferença
 * para o líquido, as sugestões com o motivo de cada uma (o site sugere, a pessoa confirma; como a "Conferir
 * investimentos", ADR 0027) e os outros créditos do período, para ligar à mão. Ligar não muda os relatórios: a receita
 * continua sendo a transação.
 */
@Component({
  selector: 'app-recebimento-holerite',
  imports: [DatePipe, MoedaPipe],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="recebimento" aria-labelledby="titulo-recebimento">
      <h3 id="titulo-recebimento">Recebido na conta</h3>
      @if (dados(); as r) {
        <p [class]="'situacao tom-' + situacao().tom">{{ situacao().texto }}</p>
        @if (r.recebimentos.length) {
          <ul class="creditos">
            @for (c of r.recebimentos; track c.id) {
              <li><span>{{ c.data | date: 'dd/MM' }} · {{ c.descricao }} <span class="texto-suave">({{ c.conta }})</span></span>
                <span class="valor">{{ c.valorLigado | currency }}@if (c.valorLigado !== c.valor) { <span class="texto-suave"> de {{ c.valor | currency }}</span> }</span>
                <button type="button" class="botao botao-pequeno" [disabled]="ocupado()" (click)="desligar(c.id)">Desligar<span class="so-leitor"> o crédito de {{ c.data | date: 'dd/MM' }}</span></button></li>
            }
          </ul>
          <p class="texto-suave">Recebido: {{ r.recebido | currency }}@if (r.liquido !== null) { de {{ r.liquido | currency }} estimados}.</p>
        }
        @if (r.sugestoes.length) {
          <h4>Parece o salário</h4>
          <ul class="creditos">
            @for (s of r.sugestoes; track s.id) {
              <li><span>{{ s.data | date: 'dd/MM' }} · {{ s.descricao }} <span class="texto-suave">({{ s.conta }})</span>
                  <span class="motivos texto-suave">{{ s.motivos.join(' · ') }}</span></span>
                <span class="valor">{{ s.livre | currency }}</span>
                <button type="button" class="botao botao-pequeno botao-primario" [disabled]="ocupado()" (click)="ligar(s.id)">Ligar<span class="so-leitor"> o crédito de {{ s.data | date: 'dd/MM' }}</span></button></li>
            }
          </ul>
        }
        @if (r.outros.length) {
          <details>
            <summary>Outros créditos do período ({{ r.outros.length }})</summary>
            <ul class="creditos">
              @for (o of r.outros; track o.id) {
                <li><span>{{ o.data | date: 'dd/MM' }} · {{ o.descricao }} <span class="texto-suave">({{ o.conta }})</span></span>
                  <span class="valor">{{ o.livre | currency }}</span>
                  <button type="button" class="botao botao-pequeno" [disabled]="ocupado()" (click)="ligar(o.id)">Ligar<span class="so-leitor"> o crédito de {{ o.data | date: 'dd/MM' }}</span></button></li>
              }
            </ul>
          </details>
        }
        <p class="texto-suave">Ligar só explica a receita: ela continua sendo a transação, uma vez só nos relatórios.</p>
      } @else {
        <p class="texto-suave" role="status">Carregando…</p>
      }
    </section>
  `,
  styles: [`
    .recebimento { border-top: 1px solid var(--linha); margin-top: 16px; padding-top: 12px; }
    .recebimento h3 { font-size: 1rem; margin: 0 0 8px; }
    .recebimento h4 { font-size: 0.9rem; margin: 12px 0 6px; }
    .creditos { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
    .creditos li { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
    .creditos .valor { margin-left: auto; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .motivos { display: block; font-size: 0.8rem; }
    .situacao { margin: 0 0 8px; }
    .tom-atencao { color: var(--atencao, inherit); font-weight: 600; }
  `]
})
export class RecebimentoHoleriteComponent {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  readonly holeriteId = input.required<number>();
  readonly dados = signal<RecebimentosHolerite | null>(null);
  readonly ocupado = signal(false);
  readonly situacao = computed(() => situacaoDoRecebimento(this.dados()!));

  constructor() {
    effect(() => {
      const id = this.holeriteId();
      this.dados.set(null);
      this.api.recebimentosHolerite(id).subscribe({ next: (r) => this.dados.set(r), error: (e) => this.avisos.erro(mensagemDeErro(e)) });
    });
  }

  ligar(transacaoId: number): void {
    this.acao(this.api.ligarRecebimento(this.holeriteId(), transacaoId, null), 'Crédito ligado ao holerite.');
  }

  desligar(id: number): void {
    this.acao(this.api.desligarRecebimento(this.holeriteId(), id), 'Crédito desligado.');
  }

  private acao(pedido: ReturnType<ApiService['recebimentosHolerite']>, mensagem: string): void {
    this.ocupado.set(true);
    pedido.subscribe({
      next: (r) => { this.ocupado.set(false); this.dados.set(r); this.avisos.sucesso(mensagem); },
      error: (e) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }
}
