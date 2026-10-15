import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Conta, Meta, SalvarMeta } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { CORES_SUGERIDAS } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

@Component({
  selector: 'app-metas',
  imports: [FormsModule, MoedaPipe, DatePipe, DecimalPipe, ModalComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Metas</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova meta</button>
    </header>

    <section class="grade-cartoes">
      @for (m of metas(); track m.id) {
        <article class="cartao-meta" [class.concluida]="m.concluida">
          <header>
            <strong>{{ m.nome }}</strong>
            @if (m.concluida) { <span class="etiqueta etiqueta-sucesso">concluída</span> }
          </header>
          <div class="anel" [style.--pct]="m.percentual" [style.--cor]="m.cor">
            <span>{{ m.percentual | number: '1.0-0' }}%</span>
          </div>
          <p><strong>{{ m.valorAtual | currency }}</strong> <span class="texto-suave">de {{ m.valorAlvo | currency }}</span></p>
          @if (m.dataAlvo) { <p class="texto-suave">até {{ m.dataAlvo | date: 'MMM/yyyy' }}</p> }
          @if (m.aporteMensalSugerido) {
            <p class="texto-suave">Guardar <strong>{{ m.aporteMensalSugerido | currency }}</strong>/mês para chegar lá</p>
          }
          @if (m.contaNome) {
            <p class="texto-suave">{{ m.valorInicial | currency }} guardados + {{ m.saldoConta | currency }} em {{ m.contaNome }}</p>
          }
          <footer>
            @if (!m.concluida) {
              <button type="button" class="link" (click)="abrirAporte(m)">+ aporte</button>
            }
            <span class="cresce"></span>
            <button type="button" class="link" (click)="editar(m)">editar</button>
          </footer>
        </article>
      } @empty {
        <p class="vazio">Crie metas como reserva de emergência, viagem ou entrada de um imóvel.</p>
      }
    </section>

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar meta' : 'Nova meta'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fm.nome" required maxlength="120"></label>
          <div class="grade-campos">
            <label>Valor alvo (R$) <input type="number" name="alvo" [(ngModel)]="fm.valorAlvo" required min="0.01" step="0.01"></label>
            <label>Data alvo <input type="date" name="data" [(ngModel)]="fm.dataAlvo"></label>
          </div>
          <label>Acompanhar saldo de uma conta (opcional)
            <select name="conta" [(ngModel)]="fm.contaId">
              <option [ngValue]="null">Não — vou registrar os aportes</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <label>Valor inicial já guardado (R$)
            <input type="number" name="atual" [(ngModel)]="fm.valorAtual" min="0" step="0.01">
          </label>
          @if (fm.contaId !== null) {
            <p class="texto-suave">O progresso será o valor inicial somado ao saldo atual da conta escolhida.</p>
          }
          <div class="cores">
            @for (cor of cores; track cor) {
              <button type="button" class="cor" [style.background]="cor" [class.selecionada]="fm.cor === cor"
                (click)="fm.cor = cor" [attr.aria-label]="'Cor ' + cor"></button>
            }
          </div>
          <label class="checkbox"><input type="checkbox" name="concluida" [(ngModel)]="fm.concluida"> Concluída</label>
          <div class="rodape-form">
            @if (editandoId) { <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="aporteMeta !== null" [titulo]="'Aporte · ' + (aporteMeta?.nome ?? '')" (fechar)="aporteMeta = null">
      <form (ngSubmit)="salvarAporte()" class="formulario">
        <label>Valor (R$) — use negativo para retirada
          <input type="number" name="aporte" [(ngModel)]="valorAporte" required step="0.01">
        </label>
        <div class="rodape-form">
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="aporteMeta = null">Cancelar</button>
          <button class="botao botao-primario">Registrar</button>
        </div>
      </form>
    </app-modal>
  `
})
export class MetasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly cores = CORES_SUGERIDAS;
  readonly metas = signal<Meta[]>([]);
  readonly contas = signal<Conta[]>([]);

  form: SalvarMeta | null = null;
  editandoId: number | null = null;
  aporteMeta: Meta | null = null;
  valorAporte: number | null = null;

  ngOnInit(): void {
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
  }

  carregar(): void {
    this.api.metas().subscribe({
      next: (m) => this.metas.set(m),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  nova(): void {
    this.editandoId = null;
    this.form = { nome: '', valorAlvo: 0, valorAtual: 0, dataAlvo: null, contaId: null, cor: CORES_SUGERIDAS[0], concluida: false };
  }

  editar(m: Meta): void {
    this.editandoId = m.id;
    this.form = {
      nome: m.nome, valorAlvo: m.valorAlvo, valorAtual: m.valorInicial, dataAlvo: m.dataAlvo,
      contaId: m.contaId, cor: m.cor, concluida: m.concluida
    };
  }

  salvar(): void {
    if (!this.form) return;
    const dto: SalvarMeta = {
      ...this.form,
      valorAlvo: Number(this.form.valorAlvo),
      valorAtual: Number(this.form.valorAtual) || 0,
      dataAlvo: this.form.dataAlvo || null
    };
    const req = this.editandoId ? this.api.atualizarMeta(this.editandoId, dto) : this.api.criarMeta(dto);
    req.subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Meta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluir(): void {
    if (!this.editandoId || !confirm('Excluir esta meta?')) return;
    this.api.excluirMeta(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirAporte(m: Meta): void {
    this.aporteMeta = m;
    this.valorAporte = null;
  }

  salvarAporte(): void {
    if (!this.aporteMeta || !this.valorAporte) return;
    this.api.aporteMeta(this.aporteMeta.id, Number(this.valorAporte)).subscribe({
      next: () => {
        this.aporteMeta = null;
        this.avisos.sucesso('Aporte registrado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
