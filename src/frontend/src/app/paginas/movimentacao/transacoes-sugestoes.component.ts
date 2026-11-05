import { Component, ChangeDetectionStrategy, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CategoriaRotulada, LinhaSugestao, MudancaSugestao, nomeConfianca } from './transacoes';

/**
 * Janela "Sugestões de categoria" de Transações. Componente de apresentação (#475, ADR 0059): mostra as linhas que a
 * tela manda e avisa de cada mudança (`mudar`, com a chave da linha) e de cada ação; não chama a API nem mexe nas
 * linhas que recebe. A tela é quem atualiza a lista e grava. Os conflitos de regra (#254) também vêm prontos da tela,
 * com o porquê em texto embaixo de "criar regra" (WCAG 3.3.1).
 */
@Component({
  selector: 'app-transacoes-sugestoes',
  imports: [ListaSuspensaDirective, FormsModule, ModalComponent, MoedaPipe],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-modal [aberto]="aberta()" titulo="Sugestões de categoria" (fechar)="fechar.emit()">
      <div class="formulario">
        <div class="linha-sugestoes">
          <div class="segmentado" role="group" aria-label="O que revisar">
            <button type="button" [class.ativo]="somenteSemCategoria()" (click)="trocarModo.emit(true)">Sem categoria</button>
            <button type="button" [class.ativo]="!somenteSemCategoria()" (click)="trocarModo.emit(false)">Revisar automáticas</button>
          </div>
          <span class="cresce"></span>
          <button type="button" class="botao botao-suave" (click)="aplicarAltas.emit()" [disabled]="!quantidadeAlta() || salvando()">
            Aplicar as {{ quantidadeAlta() }} de confiança alta</button>
        </div>
        <p class="texto-suave">
          Transações parecidas foram agrupadas pela descrição. A sugestão vem de como você categorizou transações iguais
          antes (suas escolhas manuais pesam mais) e de palavras-chave conhecidas. Marque "criar regra" para que as próximas
          sejam categorizadas sozinhas.
        </p>
        @if (carregando()) {
          <p class="vazio">Analisando suas transações…</p>
        } @else {
          <div class="tabela-rolagem">
            <table class="tabela tabela-sugestoes">
              <thead>
                <tr><th>Transações</th><th class="direita">Qtd.</th><th class="direita">Total</th><th>Sugestão</th>
                  <th>Categoria</th><th>Regra</th><th></th></tr>
              </thead>
              <tbody>
                @for (l of linhas(); track l.chave) {
                  <tr [class.encerrada]="l.aplicada">
                    <td><div class="truncar" [title]="l.exemplo">{{ l.exemplo }}</div>
                      <small class="texto-suave">{{ l.chave }}</small></td>
                    <td class="direita">{{ l.quantidade }}</td>
                    <td class="direita nowrap" [class.negativo]="l.total < 0" [class.positivo]="l.total > 0">{{ l.total | currency }}</td>
                    <td>
                      <span [class]="'confianca confianca-' + l.confianca">{{ nomeConfianca(l.confianca) }}</span>
                      @if (l.motivo) { <br><small class="texto-suave">{{ l.motivo }}</small> }
                      @if (l.alternativas.length) {
                        <div class="sugestoes-chips">
                          @for (a of l.alternativas; track a.categoriaId) {
                            <button type="button" class="chip-sugestao" [title]="a.motivo"
                              (click)="mudar.emit({ chave: l.chave, mudanca: { escolhida: a.categoriaId } })">
                              <span class="ponto" [style.background]="a.cor"></span>{{ a.nome }}</button>
                          }
                        </div>
                      }
                    </td>
                    <td>
                      <select [ngModel]="l.escolhida" (ngModelChange)="mudar.emit({ chave: l.chave, mudanca: { escolhida: $event } })"
                        [disabled]="l.aplicada" [attr.aria-label]="'Categoria para ' + l.exemplo">
                        <option [ngValue]="null">— escolher —</option>
                        @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ c.rotulo }}</option> }
                      </select>
                    </td>
                    <td>
                      @if (l.textoRegra) {
                        <label class="checkbox"><input type="checkbox" [ngModel]="l.criarRegra"
                          (ngModelChange)="mudar.emit({ chave: l.chave, mudanca: { criarRegra: $event } })" [disabled]="l.aplicada">
                          <small>contém "{{ l.textoRegra }}"</small></label>
                        <!-- #254: o porquê de a regra vir desmarcada, em texto (WCAG 3.3.1), sem esconder a escolha. -->
                        @if (conflitos().get(l.chave); as outra) {
                          <small class="aviso-regra texto-alerta">Outra linha cria esta regra para {{ outra }}: só uma pode valer.</small>
                        } @else if (l.avisoRegra) {
                          <small class="aviso-regra texto-alerta">{{ l.avisoRegra }}</small>
                        }
                      } @else {
                        <small class="texto-suave">Sem texto que identifique só estas transações: categorize sem regra.</small>
                      }
                    </td>
                    <td>
                      @if (l.aplicada) {
                        <span class="etiqueta etiqueta-sucesso">aplicada</span>
                      } @else {
                        <button type="button" class="botao botao-primario botao-pequeno" [disabled]="!l.escolhida || salvando()"
                          (click)="aplicar.emit(l)">Aplicar</button>
                      }
                    </td>
                  </tr>
                } @empty {
                  <tr><td colspan="7" class="vazio">Nada para sugerir.</td></tr>
                }
              </tbody>
            </table>
          </div>
        }
      </div>
    </app-modal>
  `,
  styles: `
    .linha-sugestoes { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .tabela-sugestoes td { vertical-align: top; }
    .tabela-sugestoes td:first-child { max-width: 280px; }
    .tabela-sugestoes select { min-width: 180px; }
    /* #254: o margin-top negativo dos chips (feito para o formulário) puxava os chips para cima do motivo. */
    .tabela-sugestoes .sugestoes-chips { margin-top: 6px; }
    .tabela-sugestoes .aviso-regra { display: block; max-width: 240px; margin-top: 4px; font-size: 0.78rem; }
    tr.encerrada td { opacity: 0.55; }
    :host ::ng-deep .modal:has(.tabela-sugestoes) { width: min(1150px, 96vw); }
  `
})
export class TransacoesSugestoesComponent {
  readonly aberta = input(false);
  readonly carregando = input(false);
  /** "Sem categoria" (true) ou "Revisar automáticas" (false). */
  readonly somenteSemCategoria = input(true);
  readonly salvando = input(false);
  /** Quantas linhas ainda não aplicadas têm confiança alta. */
  readonly quantidadeAlta = input(0);
  readonly linhas = input<LinhaSugestao[]>([]);
  readonly categorias = input<CategoriaRotulada[]>([]);
  /** O conflito de regra de cada linha, pela chave (`conflitosPorLinha`). */
  readonly conflitos = input<ReadonlyMap<string, string | null>>(new Map());

  /** A pessoa mudou a categoria escolhida ou "criar regra" de uma linha. */
  readonly mudar = output<{ chave: string; mudanca: MudancaSugestao }>();
  readonly trocarModo = output<boolean>();
  readonly aplicar = output<LinhaSugestao>();
  readonly aplicarAltas = output<void>();
  readonly fechar = output<void>();

  readonly nomeConfianca = nomeConfianca;
}
