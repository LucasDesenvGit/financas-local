import { Component, ElementRef, effect, inject, input, output, signal, untracked, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Subject, debounceTime, switchMap } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { PreviaRenomear, RenomearTransacao, ResultadoRenomear, TipoCorrespondencia, Transacao } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';

type Escopo = 'so-esta' | 'outras';

/**
 * "Renomear transação" (#123, ADR 0030). O nome do banco fica sempre à vista (Nielsen 6); "também nas outras" mostra
 * antes quantas transações vão mudar (Nielsen 5) e vira uma regra que vale para as próximas; o aviso tem "Desfazer" e
 * a janela tem "Voltar à descrição do banco" (Nielsen 3). Campos com rótulo, opções num grupo com legenda, prévia
 * anunciada (WCAG 4.1.2); Enter salva e Esc fecha (WCAG 2.1.1).
 */
@Component({
  selector: 'app-renomear-transacao',
  imports: [FormsModule, ModalComponent],
  template: `
    <app-modal [aberto]="!!transacao()" titulo="Renomear transação" (fechar)="fechar.emit()">
      @if (transacao(); as t) {
        <form class="formulario renomear" (ngSubmit)="salvar()" #f="ngForm">
          @if (t.descricaoOriginal) {
            <p class="do-banco">Descrição do banco: <strong>{{ t.descricaoOriginal }}</strong></p>
          }
          <label>Nova descrição
            <input #campoNome name="nome" [(ngModel)]="nome" required maxlength="300" autocomplete="off"
              placeholder="ex.: Aluguel do apartamento">
          </label>

          @if (t.descricaoOriginal) {
            <fieldset class="onde-usar">
              <legend>Onde usar este nome</legend>
              <label class="opcao"><input type="radio" name="escopo" value="so-esta" [(ngModel)]="escopo"> Só nesta transação</label>
              <label class="opcao"><input type="radio" name="escopo" value="outras" [(ngModel)]="escopo">
                Também nas outras com esta descrição do banco e nas próximas que chegarem</label>
              @if (escopo === 'outras') {
                <div class="regra">
                  <label>Quando a descrição do banco
                    <select name="tipo" [(ngModel)]="tipo" (ngModelChange)="mudarTipo()">
                      <option [ngValue]="TipoCorrespondencia.Igual">for igual a</option>
                      <option [ngValue]="TipoCorrespondencia.Contem">contiver</option>
                      <option [ngValue]="TipoCorrespondencia.ComecaCom">começar com</option>
                    </select>
                  </label>
                  <label>Texto
                    <input name="texto" [(ngModel)]="texto" (ngModelChange)="pedirPrevia()" required minlength="2" maxlength="200">
                  </label>
                </div>
                <p class="previa" role="status">{{ textoPrevia() }}</p>
              }
            </fieldset>
          }
          @if (t.regraDescricaoId) {
            <p class="texto-suave">Hoje este nome vem de uma regra de descrição (Categorias › Regras automáticas).</p>
          }

          <div class="rodape-form">
            @if (t.descricaoOriginal && t.descricao !== t.descricaoOriginal) {
              <button type="button" class="botao" (click)="voltarOriginal()" [disabled]="salvando()">Voltar à descrição do banco</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="fechar.emit()">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  styles: `
    .do-banco { margin: 0 0 4px; overflow-wrap: anywhere; }
    .onde-usar { border: 1px solid var(--borda); border-radius: 10px; padding: 8px 12px 10px; margin: 0; display: grid; gap: 6px; }
    .onde-usar legend { font-weight: 600; padding: 0 4px; }
    .opcao { display: flex; gap: 8px; align-items: baseline; font-weight: 400; }
    .opcao input { min-width: 18px; min-height: 18px; }
    .regra { display: grid; grid-template-columns: minmax(0, 200px) minmax(0, 1fr); gap: 10px; margin-top: 4px; }
    @media (max-width: 560px) { .regra { grid-template-columns: 1fr; } }
    .previa { margin: 2px 0 0; color: var(--texto-suave); }
  `
})
export class RenomearTransacaoComponent {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly transacao = input<Transacao | null>(null);
  readonly fechar = output<void>();
  /** Algo mudou (renomear ou desfazer): a lista recarrega. */
  readonly renomeada = output<void>();

  readonly TipoCorrespondencia = TipoCorrespondencia;
  readonly salvando = signal(false);
  readonly previa = signal<PreviaRenomear | null>(null);
  readonly carregandoPrevia = signal(false);
  private readonly campoNome = viewChild<ElementRef<HTMLInputElement>>('campoNome');
  private readonly pedidosPrevia = new Subject<void>();

  nome = '';
  escopo: Escopo = 'so-esta';
  tipo = TipoCorrespondencia.Igual;
  texto = '';

  constructor() {
    effect(() => {
      const t = this.transacao();
      untracked(() => this.abrir(t));
    });
    this.pedidosPrevia.pipe(
      debounceTime(300),
      switchMap(() => {
        this.carregandoPrevia.set(true);
        return this.api.previaRenomear(this.transacao()!.id, this.texto.trim() || null, this.tipo);
      }),
      takeUntilDestroyed()
    ).subscribe({
      next: (p) => { this.previa.set(p); this.carregandoPrevia.set(false); },
      error: () => this.carregandoPrevia.set(false)
    });
  }

  private abrir(t: Transacao | null): void {
    this.previa.set(null);
    if (!t) return;
    this.nome = t.descricao;
    this.escopo = 'so-esta';
    this.tipo = TipoCorrespondencia.Igual;
    this.texto = t.descricaoOriginal ?? '';
    // O nome atual vem selecionado: é só digitar por cima.
    setTimeout(() => { const campo = this.campoNome()?.nativeElement; campo?.focus(); campo?.select(); });
    if (!t.descricaoOriginal) return;
    this.carregandoPrevia.set(true);
    this.api.previaRenomear(t.id, t.descricaoOriginal, TipoCorrespondencia.Igual).subscribe({
      next: (p) => {
        this.previa.set(p);
        this.carregandoPrevia.set(false);
        // Há outras iguais: a opção "também nas outras" já vem marcada (dá para trocar).
        if (p.quantidade > 0 && this.transacao()?.id === t.id) this.escopo = 'outras';
      },
      error: () => this.carregandoPrevia.set(false)
    });
  }

  /** "Igual a" usa a descrição do banco inteira; "contém"/"começa com" sugerem o texto sem os números. */
  mudarTipo(): void {
    const t = this.transacao();
    if (!t?.descricaoOriginal) return;
    this.texto = this.tipo === TipoCorrespondencia.Igual ? t.descricaoOriginal : (this.previa()?.textoSugerido || t.descricaoOriginal);
    this.pedirPrevia();
  }

  pedirPrevia(): void {
    if (this.transacao()?.descricaoOriginal) this.pedidosPrevia.next();
  }

  textoPrevia(): string {
    const p = this.previa();
    if (this.carregandoPrevia() || !p) return 'Contando as transações…';
    if (p.quantidade === 0) return 'Nenhuma outra transação tem essa descrição agora: o nome vale para as próximas.';
    const exemplos = p.exemplos.length ? ` (ex.: ${p.exemplos.join('; ')})` : '';
    return p.quantidade === 1
      ? `Vai renomear mais 1 transação${exemplos} e as próximas.`
      : `Vai renomear mais ${p.quantidade} transações${exemplos} e as próximas.`;
  }

  salvar(): void {
    const t = this.transacao();
    if (!t || !this.nome.trim()) return;
    const outras = this.escopo === 'outras' && !!t.descricaoOriginal;
    this.enviar(t, {
      descricao: this.nome.trim(), voltarOriginal: false, aplicarNasOutras: outras,
      tipoCorrespondencia: this.tipo, texto: outras ? this.texto.trim() : null
    });
  }

  voltarOriginal(): void {
    const t = this.transacao();
    if (!t) return;
    this.enviar(t, { descricao: null, voltarOriginal: true, aplicarNasOutras: false, tipoCorrespondencia: TipoCorrespondencia.Igual, texto: null });
  }

  private enviar(t: Transacao, dto: RenomearTransacao): void {
    this.salvando.set(true);
    this.api.renomearTransacao(t.id, dto).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.avisar(dto, r);
        this.renomeada.emit();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** O que mudou, com "Desfazer" (a API guarda o nome de antes por alguns minutos). */
  private avisar(dto: RenomearTransacao, r: ResultadoRenomear): void {
    if (!r.desfazer) {
      this.avisos.info('Nada mudou: a transação já estava com esse nome.');
      return;
    }
    const outras = r.alteradas - 1;
    const texto = dto.voltarOriginal ? 'A transação voltou à descrição do banco.'
      : dto.aplicarNasOutras
        ? (outras > 0 ? `Renomeada, junto com mais ${outras} transação(ões). As próximas já chegam com esse nome.`
          : 'Renomeada. As próximas com essa descrição do banco já chegam com esse nome.')
        : 'Transação renomeada.';
    const chave = r.desfazer;
    this.avisos.sucesso(texto, {
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
