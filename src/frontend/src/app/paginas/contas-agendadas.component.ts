import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  Categoria, Conta, ContaAgendada, NOMES_RECORRENCIA, OrigemConta, Recorrencia, SalvarContaAgendada, TipoCategoria
} from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { dataIso } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

@Component({
  selector: 'app-contas-agendadas',
  imports: [FormsModule, MoedaPipe, DatePipe, ModalComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Contas a pagar</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova conta</button>
    </header>

    <section class="kpis">
      <article class="kpi"><span>Vencidas</span><strong class="negativo">{{ totalVencido() | currency }}</strong></article>
      <article class="kpi"><span>Próximos 30 dias</span><strong>{{ total30() | currency }}</strong></article>
    </section>

    <div class="tabela-rolagem">
      <table class="tabela">
        <thead>
          <tr><th>Vencimento</th><th>Descrição</th><th>Recorrência</th><th>Categoria</th><th class="direita">Valor</th><th></th></tr>
        </thead>
        <tbody>
          @for (c of itens(); track c.id) {
            <tr [class.ignorada]="!c.ativa">
              <td class="nowrap">
                {{ c.proximoVencimento | date: 'dd/MM/yyyy' }}
                @if (c.ativa) {
                  <br><small [class.negativo]="c.diasParaVencer < 0" class="texto-suave">
                    {{ c.diasParaVencer < 0 ? 'vencida há ' + (-c.diasParaVencer) + 'd' : c.diasParaVencer === 0 ? 'vence hoje' : 'em ' + c.diasParaVencer + ' dias' }}
                  </small>
                }
              </td>
              <td>{{ c.descricao }} @if (c.contaNome) { <small class="texto-suave">· {{ c.contaNome }}</small> }</td>
              <td>{{ nomeRecorrencia(c.recorrencia) }}</td>
              <td>{{ c.categoriaNome ?? '—' }}</td>
              <td class="direita valor">{{ c.valor | currency }}</td>
              <td class="direita nowrap">
                @if (c.ativa) { <button type="button" class="botao botao-pequeno" (click)="abrirPagamento(c)">Pagar</button> }
                <button type="button" class="link" (click)="editar(c)">editar</button>
              </td>
            </tr>
          } @empty {
            <tr><td colspan="6" class="vazio">Cadastre aluguel, condomínio, escola, assinaturas… para ser lembrado no painel.</td></tr>
          }
        </tbody>
      </table>
    </div>

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar conta' : 'Nova conta a pagar'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="150"></label>
          <div class="grade-campos">
            <label>Valor (R$) <input type="number" name="valor" [(ngModel)]="fm.valor" required min="0.01" step="0.01"></label>
            <label>Próximo vencimento <input type="date" name="venc" [(ngModel)]="fm.proximoVencimento" required></label>
            <label>Recorrência
              <select name="rec" [(ngModel)]="fm.recorrencia">
                <option [ngValue]="Recorrencia.Mensal">Mensal</option>
                <option [ngValue]="Recorrencia.Anual">Anual</option>
                <option [ngValue]="Recorrencia.Unica">Única</option>
              </select>
            </label>
            <label>Categoria
              <select name="cat" [(ngModel)]="fm.categoriaId">
                <option [ngValue]="null">—</option>
                @for (c of categoriasDespesa(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
          </div>
          <label>Conta de pagamento (opcional)
            <select name="conta" [(ngModel)]="fm.contaId">
              <option [ngValue]="null">—</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="fm.ativa"> Ativa</label>
          <div class="rodape-form">
            @if (editandoId) { <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="pagando !== null" [titulo]="'Pagar · ' + (pagando?.descricao ?? '')" (fechar)="pagando = null">
      @if (pagando; as pg) {
        <form (ngSubmit)="confirmarPagamento()" class="formulario">
          <label>Data do pagamento <input type="date" name="data" [(ngModel)]="dataPagamento" required></label>
          @if (podeLancar(pg)) {
            <label class="checkbox"><input type="checkbox" name="lancar" [(ngModel)]="lancarTransacao">
              Lançar a despesa na conta {{ pg.contaNome }}</label>
          } @else {
            <p class="texto-suave">Se o pagamento sair de uma conta do Open Finance, a transação chega pela sincronização.</p>
          }
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="pagando = null">Cancelar</button>
            <button class="botao botao-primario">Confirmar pagamento</button>
          </div>
        </form>
      }
    </app-modal>
  `
})
export class ContasAgendadasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly Recorrencia = Recorrencia;
  readonly itens = signal<ContaAgendada[]>([]);
  readonly contas = signal<Conta[]>([]);
  readonly categorias = signal<Categoria[]>([]);
  readonly categoriasDespesa = computed(() => this.categorias().filter((c) => c.tipo === TipoCategoria.Despesa && c.ativa));
  readonly totalVencido = computed(() =>
    this.itens().filter((c) => c.ativa && c.diasParaVencer < 0).reduce((s, c) => s + c.valor, 0));
  readonly total30 = computed(() =>
    this.itens().filter((c) => c.ativa && c.diasParaVencer >= 0 && c.diasParaVencer <= 30).reduce((s, c) => s + c.valor, 0));

  form: SalvarContaAgendada | null = null;
  editandoId: number | null = null;
  pagando: ContaAgendada | null = null;
  dataPagamento = dataIso();
  lancarTransacao = false;

  ngOnInit(): void {
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
    this.api.categorias().subscribe((c) => this.categorias.set(c));
  }

  nomeRecorrencia(r: Recorrencia): string {
    return NOMES_RECORRENCIA[r];
  }

  podeLancar(c: ContaAgendada): boolean {
    return this.contas().some((x) => x.id === c.contaId && x.origem === OrigemConta.Manual);
  }

  carregar(): void {
    this.api.contasAgendadas().subscribe({
      next: (i) => this.itens.set(i),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  nova(): void {
    this.editandoId = null;
    this.form = { descricao: '', valor: 0, proximoVencimento: dataIso(), recorrencia: Recorrencia.Mensal, categoriaId: null, contaId: null, ativa: true };
  }

  editar(c: ContaAgendada): void {
    this.editandoId = c.id;
    this.form = {
      descricao: c.descricao, valor: c.valor, proximoVencimento: c.proximoVencimento, recorrencia: c.recorrencia,
      categoriaId: c.categoriaId, contaId: c.contaId, ativa: c.ativa
    };
  }

  salvar(): void {
    if (!this.form) return;
    const dto = { ...this.form, valor: Number(this.form.valor) };
    const req = this.editandoId ? this.api.atualizarContaAgendada(this.editandoId, dto) : this.api.criarContaAgendada(dto);
    req.subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Conta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluir(): void {
    if (!this.editandoId || !confirm('Excluir esta conta a pagar?')) return;
    this.api.excluirContaAgendada(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirPagamento(c: ContaAgendada): void {
    this.pagando = c;
    this.dataPagamento = dataIso();
    this.lancarTransacao = this.podeLancar(c);
  }

  confirmarPagamento(): void {
    if (!this.pagando) return;
    this.api.pagarContaAgendada(this.pagando.id, this.lancarTransacao && this.podeLancar(this.pagando), this.dataPagamento)
      .subscribe({
        next: () => {
          this.pagando = null;
          this.avisos.sucesso('Pagamento registrado.');
          this.carregar();
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      });
  }
}
