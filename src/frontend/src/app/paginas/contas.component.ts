import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Conta, Fatura, NOMES_TIPO_CONTA, OrigemConta, SalvarConta, TipoConta } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { CORES_SUGERIDAS } from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';

@Component({
  selector: 'app-contas',
  imports: [FormsModule, MoedaPipe, DatePipe, RouterLink, ModalComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Contas e cartões</h1>
      <div class="acoes">
        <label class="checkbox"><input type="checkbox" [(ngModel)]="mostrarArquivadas" (change)="carregar()"> mostrar arquivadas</label>
        <button type="button" class="botao botao-primario" (click)="nova()">+ Conta manual</button>
      </div>
    </header>

    <p class="texto-suave">Contas do Open Finance são criadas automaticamente pela sincronização
      (<a routerLink="/configuracoes">Configurações › Open Finance</a>). Contas manuais servem para dinheiro em espécie,
      bancos sem Open Finance ou controles separados.</p>

    <!-- Contas (dinheiro disponível) -->
    <section class="secao-contas">
      <header class="secao-cabecalho">
        <div>
          <h2>Contas</h2>
          <small class="texto-suave">{{ contasBancarias().length }} conta(s)</small>
        </div>
        <div class="secao-total">
          <span class="texto-suave">Saldo total</span>
          <strong [class.negativo]="totalContas() < 0">{{ totalContas() | currency }}</strong>
        </div>
      </header>
      <div class="grade-cartoes">
        @for (c of contasBancarias(); track c.id) {
          <article class="cartao-conta" [class.arquivada]="c.arquivada" [style.--cor-conta]="c.cor">
            <header>
              <span class="conta-icone" aria-hidden="true">{{ iconeTipo(c.tipo) }}</span>
              <div class="cresce">
                <strong>{{ c.nome }}</strong>
                <small class="texto-suave">{{ nomeTipo(c.tipo) }}{{ c.nomeInstituicao ? ' · ' + c.nomeInstituicao : '' }}</small>
              </div>
              <span class="etiqueta" [class.etiqueta-suave]="c.origem !== OrigemConta.OpenFinance">
                {{ c.origem === OrigemConta.OpenFinance ? 'open finance' : 'manual' }}</span>
            </header>
            <div class="saldo" [class.negativo]="c.saldo < 0">{{ c.saldo | currency }}</div>
            <small class="texto-suave">{{ c.incluirNoSaldo ? 'saldo atual' : 'saldo atual · fora do total' }}</small>
            <footer>
              <small class="texto-suave">{{ c.numero ?? '' }}</small>
              <span class="cresce"></span>
              <button type="button" class="link" (click)="editar(c)">editar</button>
            </footer>
          </article>
        } @empty {
          <p class="vazio">Nenhuma conta ainda. Configure o Open Finance ou crie uma conta manual.</p>
        }
      </div>
    </section>

    <!-- Cartões de crédito (dívida da fatura) -->
    @if (cartoes().length) {
      <section class="secao-contas">
        <header class="secao-cabecalho">
          <div>
            <h2>Cartões de crédito</h2>
            <small class="texto-suave">{{ cartoes().length }} cartão(ões)</small>
          </div>
          <div class="secao-total">
            <span class="texto-suave">Faturas em aberto</span>
            <strong class="negativo">{{ totalFaturas() | currency }}</strong>
          </div>
        </header>
        <div class="grade-cartoes grade-cartoes-credito">
          @for (c of cartoes(); track c.id) {
            <article class="cartao-credito-bloco" [class.arquivada]="c.arquivada">
              <div class="cartao-credito" [style.--cor-conta]="c.cor">
                <div class="cartao-credito-topo">
                  <span class="cartao-credito-instituicao">{{ c.nomeInstituicao ?? 'Cartão' }}</span>
                  <span class="cartao-credito-tipo">crédito</span>
                </div>
                <span class="cartao-credito-chip" aria-hidden="true"></span>
                <div class="cartao-credito-rodape">
                  <strong>{{ c.nome }}</strong>
                  <span>•••• {{ finalCartao(c) }}</span>
                </div>
              </div>

              <div class="cartao-credito-detalhes">
                <div class="linha-valores">
                  <div>
                    <span class="texto-suave">Fatura atual</span>
                    <strong class="negativo">{{ fatura(c) | currency }}</strong>
                  </div>
                  <div class="direita">
                    <span class="texto-suave">Disponível</span>
                    <strong>{{ disponivel(c) !== null ? (disponivel(c) | currency) : '—' }}</strong>
                  </div>
                </div>
                @if (limiteEfetivo(c); as limite) {
                  <div class="trilho" [title]="usoLimite(c) + '% do limite em uso'">
                    <div class="progresso" [style.width.%]="usoLimite(c)"
                      [class.atencao]="usoLimite(c) >= 70 && usoLimite(c) < 90" [class.estourado]="usoLimite(c) >= 90"></div>
                  </div>
                  <small class="texto-suave">
                    {{ usoLimite(c) }}% de {{ limite | currency }} em uso
                    @if (limiteAprovadoMaior(c)) { · aprovado pelo banco: {{ c.limiteCredito | currency }} }
                  </small>
                }
                <small class="texto-suave">
                  @if (c.diaFechamento) { fecha dia {{ c.diaFechamento }} }
                  @if (c.diaFechamento && c.diaVencimento) { · }
                  @if (c.diaVencimento) { vence dia {{ c.diaVencimento }} }
                </small>
                <footer>
                  <span class="etiqueta" [class.etiqueta-suave]="c.origem !== OrigemConta.OpenFinance">
                    {{ c.origem === OrigemConta.OpenFinance ? 'open finance' : 'manual' }}</span>
                  <span class="cresce"></span>
                  <button type="button" class="link" (click)="verFaturas(c)">faturas</button>
                  <button type="button" class="link" (click)="editar(c)">editar</button>
                </footer>
              </div>
            </article>
          }
        </div>
      </section>
    }

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar conta' : 'Nova conta manual'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          @if (editandoOpenFinance) {
            <p class="aviso-inline">Conta do Open Finance: saldo, limite e tipo vêm do banco. Você pode alterar nome, cor e visibilidade.</p>
          }
          <label>Nome <input name="nome" [(ngModel)]="fm.nome" required maxlength="120"></label>
          @if (!editandoOpenFinance) {
            <div class="grade-campos">
              <label>Tipo
                <select name="tipo" [(ngModel)]="fm.tipo">
                  @for (t of tipos; track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
                </select>
              </label>
              <label>Saldo inicial (R$)
                <input type="number" name="saldoInicial" [(ngModel)]="fm.saldoInicial" step="0.01">
              </label>
              <label>Número / identificação <input name="numero" [(ngModel)]="fm.numero" maxlength="60"></label>
              @if (fm.tipo === TipoConta.CartaoCredito) {
                <label>Limite (R$) <input type="number" name="limite" [(ngModel)]="fm.limiteCredito" min="0" step="0.01"></label>
                <label>Dia de fechamento <input type="number" name="fech" [(ngModel)]="fm.diaFechamento" min="1" max="31"></label>
                <label>Dia de vencimento <input type="number" name="venc" [(ngModel)]="fm.diaVencimento" min="1" max="31"></label>
              }
            </div>
            @if (fm.tipo === TipoConta.CartaoCredito) {
              <p class="texto-suave">Para cartão, use saldo inicial negativo para uma fatura já em aberto.</p>
            }
          }
          <div class="cores">
            @for (cor of cores; track cor) {
              <button type="button" class="cor" [style.background]="cor" [class.selecionada]="fm.cor === cor"
                (click)="fm.cor = cor" [attr.aria-label]="'Cor ' + cor"></button>
            }
          </div>
          <label class="checkbox"><input type="checkbox" name="incluir" [(ngModel)]="fm.incluirNoSaldo"> Incluir no saldo total</label>
          <label class="checkbox"><input type="checkbox" name="arquivada" [(ngModel)]="fm.arquivada"> Arquivada</label>
          <div class="rodape-form">
            @if (editandoId && !editandoOpenFinance) {
              <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="faturasDe() !== null" [titulo]="'Faturas · ' + (faturasDe()?.nome ?? '')" (fechar)="faturasDe.set(null)">
      <table class="tabela">
        <thead><tr><th>Vencimento</th><th>Fechamento</th><th class="direita">Total</th><th class="direita">Mínimo</th><th class="direita">Pago</th></tr></thead>
        <tbody>
          @for (f of faturas(); track f.id) {
            <tr>
              <td>{{ f.dataVencimento | date: 'dd/MM/yyyy' }}</td>
              <td>{{ f.dataFechamento ? (f.dataFechamento | date: 'dd/MM/yyyy') : '—' }}</td>
              <td class="direita">{{ f.valorTotal | currency }}</td>
              <td class="direita">{{ f.valorMinimo !== null ? (f.valorMinimo | currency) : '—' }}</td>
              <td class="direita">{{ f.valorPago !== null ? (f.valorPago | currency) : '—' }}</td>
            </tr>
          } @empty {
            <tr><td colspan="5" class="vazio">Sem faturas (disponíveis apenas para cartões do Open Finance).</td></tr>
          }
        </tbody>
      </table>
    </app-modal>
  `
})
export class ContasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly TipoConta = TipoConta;
  readonly OrigemConta = OrigemConta;
  readonly cores = CORES_SUGERIDAS;
  readonly tipos = [TipoConta.ContaCorrente, TipoConta.Poupanca, TipoConta.CartaoCredito, TipoConta.Investimento,
    TipoConta.Dinheiro, TipoConta.Outro];

  readonly contas = signal<Conta[]>([]);
  readonly faturasDe = signal<Conta | null>(null);
  readonly faturas = signal<Fatura[]>([]);
  readonly contasBancarias = computed(() => this.contas().filter((c) => c.tipo !== TipoConta.CartaoCredito));
  readonly cartoes = computed(() => this.contas().filter((c) => c.tipo === TipoConta.CartaoCredito));
  readonly totalContas = computed(() =>
    this.contasBancarias().reduce((s, c) => s + (c.incluirNoSaldo && !c.arquivada ? c.saldo : 0), 0));
  readonly totalFaturas = computed(() =>
    this.cartoes().reduce((s, c) => s + (c.arquivada ? 0 : Math.max(0, -c.saldo)), 0));

  mostrarArquivadas = false;
  form: SalvarConta | null = null;
  editandoId: number | null = null;
  editandoOpenFinance = false;

  ngOnInit(): void {
    this.carregar();
  }

  nomeTipo(t: TipoConta): string {
    return NOMES_TIPO_CONTA[t];
  }

  iconeTipo(t: TipoConta): string {
    switch (t) {
      case TipoConta.Poupanca: return '◍';
      case TipoConta.Investimento: return '△';
      case TipoConta.Dinheiro: return '¤';
      case TipoConta.Outro: return '◇';
      default: return '▣';
    }
  }

  finalCartao(c: Conta): string {
    const digitos = (c.numero ?? '').replace(/\D/g, '');
    return digitos.length >= 4 ? digitos.slice(-4) : '••••';
  }

  /** Valor da fatura atual (positivo). O saldo do cartão é negativo quando há dívida. */
  fatura(c: Conta): number {
    return Math.max(0, -c.saldo);
  }

  disponivel(c: Conta): number | null {
    return c.limiteDisponivel;
  }

  /**
   * Limite realmente em uso. Muitos bancos informam pelo Open Finance o limite APROVADO (ex.: R$ 10.150),
   * mas o disponível já reflete o limite que você ajustou no app. Por isso usamos fatura + disponível.
   */
  limiteEfetivo(c: Conta): number | null {
    if (c.limiteDisponivel !== null) {
      const efetivo = this.fatura(c) + Math.max(0, c.limiteDisponivel);
      if (efetivo > 0) return c.limiteCredito ? Math.min(efetivo, c.limiteCredito) : efetivo;
    }
    return c.limiteCredito;
  }

  usoLimite(c: Conta): number {
    const limite = this.limiteEfetivo(c);
    if (!limite) return 0;
    return Math.round(Math.min(100, Math.max(0, (this.fatura(c) / limite) * 100)));
  }

  limiteAprovadoMaior(c: Conta): boolean {
    const efetivo = this.limiteEfetivo(c);
    return !!c.limiteCredito && !!efetivo && c.limiteCredito > efetivo * 1.05;
  }

  carregar(): void {
    this.api.contas(this.mostrarArquivadas).subscribe({
      next: (c) => this.contas.set(c),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  nova(): void {
    this.editandoId = null;
    this.editandoOpenFinance = false;
    this.form = {
      nome: '', tipo: TipoConta.Dinheiro, numero: null, saldoInicial: 0, limiteCredito: null, diaFechamento: null,
      diaVencimento: null, cor: CORES_SUGERIDAS[0], incluirNoSaldo: true, arquivada: false
    };
  }

  editar(c: Conta): void {
    this.editandoId = c.id;
    this.editandoOpenFinance = c.origem === OrigemConta.OpenFinance;
    this.form = {
      nome: c.nome, tipo: c.tipo, numero: c.numero, saldoInicial: c.saldoInicial, limiteCredito: c.limiteCredito,
      diaFechamento: c.diaFechamento, diaVencimento: c.diaVencimento, cor: c.cor, incluirNoSaldo: c.incluirNoSaldo,
      arquivada: c.arquivada
    };
  }

  salvar(): void {
    if (!this.form) return;
    const dto: SalvarConta = {
      ...this.form,
      saldoInicial: Number(this.form.saldoInicial) || 0,
      limiteCredito: this.form.limiteCredito ? Number(this.form.limiteCredito) : null,
      diaFechamento: this.form.diaFechamento ? Number(this.form.diaFechamento) : null,
      diaVencimento: this.form.diaVencimento ? Number(this.form.diaVencimento) : null,
      numero: this.form.numero || null
    };
    const req = this.editandoId ? this.api.atualizarConta(this.editandoId, dto) : this.api.criarConta(dto);
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
    if (!this.editandoId || !confirm('Excluir a conta e TODAS as suas transações? Esta ação não pode ser desfeita.')) return;
    this.api.excluirConta(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Conta excluída.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  verFaturas(c: Conta): void {
    this.faturas.set([]);
    this.faturasDe.set(c);
    this.api.faturas(c.id).subscribe({
      next: (f) => this.faturas.set(f),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }
}
