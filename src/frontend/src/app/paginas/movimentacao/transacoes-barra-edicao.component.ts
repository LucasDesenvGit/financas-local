import { Component, ChangeDetectionStrategy, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { CategoriaRotulada, EscolhaCategoria, SugestaoRegra } from './transacoes';

/**
 * Barra "Editando categorias" de Transações (#87). Componente de apresentação (#475, ADR 0059): mostra quantas trocas
 * estão pendentes, "todas do filtro" e as regras sugeridas, e avisa a tela de cada ação; não chama a API. Nada é
 * gravado até "Salvar alterações" (Nielsen 5).
 *
 * O próprio elemento é o bloco: a tela põe nele `class="painel-bloco barra-edicao expandir"` e `animate.leave`, como
 * punha na `section` de antes. Assim o `.bloco-titulo` continua filho direto do `.painel-bloco` (a linha embaixo do
 * título, no styles.css, usa `>`), e a animação de saída fica no elemento que a tela tira. É uma região com nome,
 * como a `section` (WAI-ARIA, landmark region).
 */
@Component({
  selector: 'app-transacoes-barra-edicao',
  imports: [ListaSuspensaDirective, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  host: { role: 'region', 'aria-labelledby': 'titulo-edicao' },
  template: `
    <div class="bloco-titulo">
      <h2 id="titulo-edicao">Editando categorias</h2>
      <div class="acoes">
        <span role="status">{{ quantidadePendentes() ? quantidadePendentes() + ' alteração(ões) não salva(s)' : 'Nenhuma alteração pendente' }}</span>
        <button type="button" class="botao" [disabled]="!quantidadePendentes()" (click)="descartar.emit()">Descartar</button>
        <button type="button" class="botao botao-primario" [disabled]="!quantidadePendentes() || salvando()"
          (click)="salvar.emit()">Salvar alterações ({{ quantidadePendentes() }})</button>
      </div>
    </div>
    <p class="texto-suave pequeno-texto">
      Troque a categoria direto na lista ou marque várias linhas e use a barra de seleção. Nada é gravado até
      "Salvar alterações"; Esc numa linha desfaz a troca dela. Só a categoria muda por aqui.
      @if (!soSemCategoriaAtivo()) {
        <button type="button" class="link" (click)="soSemCategoria.emit()">Ver só as sem categoria</button>
      }
    </p>
    @if (total() > 0) {
      <div class="todas-do-filtro">
        <label>Todas as {{ total() }} transações do filtro
          <select [(ngModel)]="categoriaTodas" name="categoriaTodas">
            <option [ngValue]="null">— escolher categoria —</option>
            <option ngValue="auto">Categorização automática (regras)</option>
            @for (c of categorias(); track c.id) { <option [ngValue]="'' + c.id">{{ c.rotulo }}</option> }
          </select>
        </label>
        <button type="button" class="botao" [disabled]="!categoriaTodas() || salvando()" (click)="aplicarATodas.emit()">
          Aplicar a todas as {{ total() }}</button>
      </div>
    }
    @for (r of sugestoesRegra(); track r.texto + r.categoriaId) {
      <p class="sugestao-regra" role="status">
        {{ r.quantidade }} transações “{{ r.texto }}” foram para {{ r.categoriaNome }}. Criar regra para as próximas?
        <button type="button" class="botao botao-pequeno" (click)="criarRegra.emit(r)">Criar regra</button>
        <button type="button" class="link" (click)="dispensarRegra.emit(r)">agora não</button>
      </p>
    }
  `,
  styles: `
    :host { display: block; }
    .bloco-titulo { flex-wrap: wrap; gap: 10px; }
    p { margin: 6px 0; }
    .todas-do-filtro { display: flex; flex-wrap: wrap; gap: 10px; align-items: flex-end; margin-top: 8px; }
    .todas-do-filtro label { min-width: 240px; }
    .sugestao-regra { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; padding: 8px 10px;
      border-radius: var(--raio-pequeno); background: var(--primaria-suave); }
  `
})
export class TransacoesBarraEdicaoComponent {
  readonly quantidadePendentes = input(0);
  readonly salvando = input(false);
  /** O filtro já é só "Sem categoria": o atalho "Ver só as sem categoria" some. */
  readonly soSemCategoriaAtivo = input(false);
  /** Quantas transações o filtro tem, em todas as páginas (0 = sem "todas do filtro"). */
  readonly total = input(0);
  readonly categorias = input<CategoriaRotulada[]>([]);
  readonly sugestoesRegra = input<SugestaoRegra[]>([]);
  /** A escolha de "todas do filtro" (null = nada escolhido); a tela a limpa depois de aplicar. */
  readonly categoriaTodas = model<EscolhaCategoria | null>(null);

  readonly descartar = output<void>();
  readonly salvar = output<void>();
  readonly soSemCategoria = output<void>();
  readonly aplicarATodas = output<void>();
  readonly criarRegra = output<SugestaoRegra>();
  readonly dispensarRegra = output<SugestaoRegra>();
}
