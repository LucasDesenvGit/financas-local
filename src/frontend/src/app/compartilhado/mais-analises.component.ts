import { Component, TemplateRef, contentChild, input, linkedSignal } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';

let sequencia = 0;

/**
 * UX-11: divulgação progressiva. A tela mostra primeiro o que responde à pergunta principal dela; as análises
 * de apoio ficam aqui, a um clique (NN/g, Progressive Disclosure; Shneiderman: overview first, details on demand).
 *
 *   <app-mais-analises chave="painel" conteudo="evolução de 12 meses e últimas transações">
 *     <ng-template> ...blocos... </ng-template>
 *   </app-mais-analises>
 *
 * O conteúdo vem num <ng-template> para só ser criado ao abrir: gráfico recolhido não é carregado nem desenhado.
 * A escolha de abrir/fechar fica lembrada por tela neste navegador. O botão diz o que tem dentro, para a pessoa
 * reconhecer em vez de lembrar (Nielsen 6), e usa aria-expanded/aria-controls (WCAG 4.1.2).
 */
@Component({
  selector: 'app-mais-analises',
  imports: [NgTemplateOutlet],
  template: `
    <section class="mais-analises" [class.aberta]="aberta()">
      <h2 class="mais-analises-titulo">
        <button type="button" class="mais-analises-botao" [attr.aria-expanded]="aberta()" [attr.aria-controls]="id"
          (click)="alternar()">
          <span class="mais-analises-seta" aria-hidden="true">▸</span>
          <span>{{ titulo() }}</span>
          @if (conteudo()) { <small>{{ conteudo() }}</small> }
        </button>
      </h2>
      <div [id]="id" class="mais-analises-conteudo">
        @if (aberta() && modelo(); as m) { <ng-container [ngTemplateOutlet]="m" /> }
      </div>
    </section>
  `
})
export class MaisAnalisesComponent {
  /** Identifica a tela na preferência salva (uma por tela). */
  readonly chave = input.required<string>();
  readonly titulo = input('Mais análises');
  /** O que está recolhido, em poucas palavras. */
  readonly conteudo = input('');
  readonly modelo = contentChild(TemplateRef);
  readonly id = `mais-analises-${++sequencia}`;
  readonly aberta = linkedSignal(() => this.lerPreferencia(this.chave()));

  alternar(): void {
    this.aberta.update((v) => !v);
    try {
      localStorage.setItem(this.chaveArmazenada(this.chave()), this.aberta() ? '1' : '0');
    } catch { /* navegador sem armazenamento: só não lembra */ }
  }

  private lerPreferencia(chave: string): boolean {
    try {
      return localStorage.getItem(this.chaveArmazenada(chave)) === '1';
    } catch {
      return false;
    }
  }

  private chaveArmazenada(chave: string): string {
    return `financas.maisAnalises.${chave}`;
  }
}
