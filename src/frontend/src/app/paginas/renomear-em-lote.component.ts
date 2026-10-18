import { Component, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { ResultadoRenomearEmLote, Transacao } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';

type Escopo = 'so-estas' | 'regras';

/** Descrições do banco listadas na janela; o resto vira "e mais N". */
const ORIGINAIS_VISIVEIS = 5;

/**
 * Renomear várias transações de uma vez (#123), pela barra de seleção. "Nas próximas" cria uma regra "é igual a" por
 * descrição do banco das selecionadas, como o renomear de uma; a janela mostra quais são antes de salvar (Nielsen 5) e o
 * aviso tem "Desfazer" (Nielsen 3).
 */
@Component({
  selector: 'app-renomear-em-lote',
  imports: [FormsModule, ModalComponent],
  template: `
    <app-modal [aberto]="!!transacoes()" [titulo]="'Renomear ' + quantidade() + ' transações'" (fechar)="fechar.emit()">
      @if (transacoes()) {
        <form class="formulario renomear-lote" (ngSubmit)="salvar()" #f="ngForm">
          @if (originais().length) {
            <div class="originais">
              <span>Descrições do banco das selecionadas:</span>
              <ul>
                @for (o of originais().slice(0, visiveis); track o) { <li>{{ o }}</li> }
              </ul>
              @if (originais().length > visiveis) { <small class="texto-suave">e mais {{ originais().length - visiveis }}</small> }
            </div>
          }
          <label>Novo nome
            <input name="nome" [(ngModel)]="nome" required maxlength="300" autocomplete="off" placeholder="ex.: Pagamento da fatura do Nubank">
          </label>
          <fieldset class="onde-usar">
            <legend>Onde usar este nome</legend>
            <label class="opcao"><input type="radio" name="escopo" value="so-estas" [(ngModel)]="escopo">
              Só nas {{ quantidade() }} selecionadas</label>
            <label class="opcao" [class.desligada]="!originais().length">
              <input type="radio" name="escopo" value="regras" [(ngModel)]="escopo" [disabled]="!originais().length">
              Nas selecionadas e nas próximas com as mesmas descrições do banco
              ({{ originais().length === 1 ? 'cria 1 regra' : 'cria ' + originais().length + ' regras' }} “é igual a”)</label>
          </fieldset>
          <p class="texto-suave">Para juntar descrições que mudam a cada mês (com número), crie uma regra “contém” em
            Categorias › Regras automáticas.</p>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  styles: `
    .originais ul { margin: 4px 0 0; padding-left: 20px; display: grid; gap: 2px; overflow-wrap: anywhere; }
    .onde-usar { border: 1px solid var(--borda); border-radius: 10px; padding: 8px 12px 10px; margin: 0; display: grid; gap: 6px; }
    .onde-usar legend { font-weight: 600; padding: 0 4px; }
    .opcao { display: flex; gap: 8px; align-items: baseline; font-weight: 400; }
    .opcao input { min-width: 18px; min-height: 18px; }
    .opcao.desligada { color: var(--texto-suave); }
  `
})
export class RenomearEmLoteComponent {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly transacoes = input<Transacao[] | null>(null);
  readonly fechar = output<void>();
  readonly renomeada = output<void>();

  readonly visiveis = ORIGINAIS_VISIVEIS;
  readonly salvando = signal(false);
  readonly quantidade = computed(() => this.transacoes()?.length ?? 0);
  /** Descrições do banco diferentes (sem diferença de maiúsculas), das que vieram do banco. */
  readonly originais = computed(() => {
    const vistos = new Map<string, string>();
    for (const t of this.transacoes() ?? []) {
      if (t.descricaoOriginal && !vistos.has(t.descricaoOriginal.toUpperCase())) vistos.set(t.descricaoOriginal.toUpperCase(), t.descricaoOriginal);
    }
    return Array.from(vistos.values());
  });

  nome = '';
  escopo: Escopo = 'so-estas';

  constructor() {
    effect(() => {
      const lista = this.transacoes();
      untracked(() => {
        if (!lista) return;
        this.nome = '';
        // Vindo do banco, a regra é o padrão: o pedido é que as próximas iguais também cheguem com o nome.
        this.escopo = this.originais().length ? 'regras' : 'so-estas';
      });
    });
  }

  salvar(): void {
    const lista = this.transacoes();
    if (!lista?.length || !this.nome.trim()) return;
    this.salvando.set(true);
    this.api.renomearEmLote({ ids: lista.map((t) => t.id), descricao: this.nome.trim(), criarRegras: this.escopo === 'regras' }).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.avisar(r);
        this.renomeada.emit();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  private avisar(r: ResultadoRenomearEmLote): void {
    if (!r.desfazer) {
      this.avisos.info('Nada mudou: as transações já estavam com esse nome.');
      return;
    }
    const partes = [`${r.alteradas} transação(ões) renomeada(s)`];
    if (r.regrasCriadas) partes.push(`${r.regrasCriadas} regra(s) criada(s)`);
    if (r.regrasAlteradas) partes.push(`${r.regrasAlteradas} regra(s) com o nome novo`);
    const chave = r.desfazer;
    this.avisos.sucesso(`${partes.join(', ')}.`, {
      rotulo: 'Desfazer',
      executar: () => this.api.desfazerRenomear(chave).subscribe({
        next: () => {
          this.avisos.sucesso('Desfeito: os nomes voltaram como estavam.');
          this.renomeada.emit();
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      })
    });
  }
}
