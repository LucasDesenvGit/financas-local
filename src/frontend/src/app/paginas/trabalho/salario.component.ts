import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { Conta, Emprego, EstimativaSalario, RegimeEmprego } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { MesAno, SeletorMesComponent } from '../../compartilhado/ui/seletor-mes.component';
import {
  FormularioEmprego, REGIMES, aliquota, empregoAtual, explicarDeducao, formatarCnpj, formularioDe, formularioVazio,
  historicoDeSalarios, mesDaApi, nomeDoMes, paraSalvar, rotuloRegime, variacao
} from './salario';

/**
 * #210 (ADR 0065): Trabalho › Salário. O emprego atual, a estimativa do mês (bruto, INSS, IRRF e líquido, com "como
 * calculei" linha a linha; Nielsen 10) e o histórico de salários com o "Aumento a partir de…". É estimativa, não
 * holerite, e a tela diz isso sempre. Fica dentro do portão do módulo: com o Trabalho desligado, nem é criada.
 */
@Component({
  selector: 'app-trabalho-salario',
  imports: [FormsModule, DatePipe, MoedaPipe, CampoDataComponent, CampoValorComponent, ListaSuspensaDirective, SeletorMesComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina">
      <h1>Salário</h1>
      @if (atual() && !formulario()) {
        <div class="acoes">
          <button type="button" class="botao" (click)="editar(atual()!)">Editar emprego</button>
          <button type="button" class="botao" (click)="novo()">+ Outro emprego</button>
        </div>
      }
    </header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (formulario(); as f) {
      <section class="painel-bloco" aria-labelledby="titulo-form-emprego">
        <h2 id="titulo-form-emprego">{{ editandoId() ? 'Editar emprego' : 'Cadastrar emprego' }}</h2>
        <form class="formulario" (ngSubmit)="salvar()" #fe="ngForm">
          <div class="grade-2 campos-emprego">
            <label>Empregador <input name="empregador" [(ngModel)]="f.empregador" required maxlength="150" autocomplete="organization"></label>
            <label>CNPJ (opcional) <input name="cnpj" [(ngModel)]="f.cnpj" inputmode="numeric" maxlength="18" placeholder="00.000.000/0000-00"></label>
            <label>Admissão <app-campo-data name="admissao" [(ngModel)]="f.admissao" obrigatorio /></label>
            <label>Saída (se já saiu) <app-campo-data name="saida" [(ngModel)]="f.saida" /></label>
            <label>Regime
              <select name="regime" [(ngModel)]="f.regime">
                @for (r of regimes; track r.valor) { <option [ngValue]="r.valor">{{ r.rotulo }}</option> }
              </select>
            </label>
            <label>Jornada mensal (horas) <input name="jornada" type="number" inputmode="numeric" min="1" max="300" [(ngModel)]="f.jornadaMensalHoras" required></label>
            <label>Dependentes para o IR <input name="dependentes" type="number" inputmode="numeric" min="0" max="20" [(ngModel)]="f.dependentes"></label>
            <label>Conta onde o salário cai (opcional)
              <select name="conta" [(ngModel)]="f.contaId">
                <option [ngValue]="null">Não informar</option>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            @if (!editandoId()) {
              <label>Salário bruto <app-campo-valor name="salario" [(ngModel)]="f.salarioBruto" obrigatorio [minimo]="0.01" /></label>
            }
          </div>
          @if (f.regime !== RegimeEmprego.Clt) {
            <p class="aviso-inline">Estágio e PJ ficam sem o cálculo do INSS e do IRRF: o site guarda o valor bruto.</p>
          }
          <div class="rodape-form">
            @if (editandoId()) {
              <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir emprego</button>
            }
            <span class="cresce"></span>
            @if (empregos().length) { <button type="button" class="botao" (click)="cancelar()">Cancelar</button> }
            <button class="botao botao-primario" [disabled]="fe.invalid || salvando()">Salvar</button>
          </div>
        </form>
      </section>
    } @else if (atual(); as e) {
      <p class="texto-suave aviso-estimativa">Estimativa, não holerite: o cálculo segue as regras da CLT e as tabelas oficiais
        do INSS e do IRRF, mas convenção coletiva, benefícios da empresa e arredondamentos mudam o valor. O que vale é o seu
        holerite e o extrato. Nada daqui entra nos relatórios: a receita continua sendo a transação do salário.</p>

      <div class="grade-2">
        <section class="painel-bloco" aria-labelledby="titulo-emprego">
          <h2 id="titulo-emprego">{{ e.empregador }}</h2>
          <dl class="dados-emprego">
            @if (e.cnpj) { <dt>CNPJ</dt><dd>{{ cnpj(e.cnpj) }}</dd> }
            <dt>Admissão</dt><dd>{{ e.admissao | date: 'dd/MM/yyyy' }}</dd>
            @if (e.saida) { <dt>Saída</dt><dd>{{ e.saida | date: 'dd/MM/yyyy' }}</dd> }
            <dt>Regime</dt><dd>{{ regime(e.regime) }}</dd>
            <dt>Jornada</dt><dd>{{ e.jornadaMensalHoras }} h por mês</dd>
            <dt>Dependentes para o IR</dt><dd>{{ e.dependentes }}</dd>
            <dt>Conta do salário</dt><dd>{{ e.contaNome ?? 'não informada' }}</dd>
          </dl>
        </section>

        <section class="painel-bloco" aria-labelledby="titulo-estimativa">
          <div class="cabecalho-bloco">
            <h2 id="titulo-estimativa">O mês</h2>
            <app-seletor-mes [valor]="periodo()" (mudou)="mudarPeriodo($event)" />
          </div>
          @if (erroEstimativa(); as msg) {
            <p class="aviso-inline">{{ msg }}</p>
          } @else if (estimativa(); as est) {
            <dl class="resumo-mes">
              <dt>Salário bruto</dt><dd>{{ est.salarioBruto | currency }}</dd>
              @if (est.semEncargos) {
                <dt>Encargos</dt><dd>sem cálculo ({{ regime(e.regime) }})</dd>
              } @else {
                <dt>INSS</dt><dd>− {{ est.inss!.valor | currency }}</dd>
                <dt>IRRF</dt><dd>− {{ est.irrf!.valor | currency }}</dd>
              }
              <dt class="total">Líquido estimado</dt><dd class="total">{{ est.liquido | currency }}</dd>
            </dl>
            <p class="texto-suave">Salário de {{ nomeDoMes(est.vigenciaDe) }} em diante.</p>
            @if (!est.semEncargos) {
              <details class="como-calculei">
                <summary>Como calculei</summary>
                <h3>INSS</h3>
                <ul>
                  @for (p of est.inss!.parcelas; track $index) {
                    <li>{{ aliquota(p.aliquota) }} de {{ p.de | currency }} até {{ p.ate | currency }}: {{ p.valor | currency }}</li>
                  }
                </ul>
                @if (est.inss!.noTeto) { <p>O salário passa do teto: o INSS é o máximo da tabela.</p> }
                <p class="texto-suave">Tabela: {{ est.inss!.fonte }}.</p>
                <h3>IRRF</h3>
                <ul>
                  <li>{{ explicarDeducao(est.irrf!) }} INSS e dependentes: {{ est.irrf!.deducoesLegais | currency }};
                    simplificado: {{ est.irrf!.simplificado | currency }}.</li>
                  <li>Base: {{ est.irrf!.base | currency }}, na faixa de {{ aliquota(est.irrf!.aliquota) }}, menos
                    {{ est.irrf!.parcelaADeduzir | currency }}: {{ est.irrf!.impostoDaTabela | currency }}.</li>
                  @if (est.irrf!.reducaoLei15270 > 0) {
                    <li>Redução da Lei 15.270/2025 (rendimento até R$ 7.350,00): − {{ est.irrf!.reducaoLei15270 | currency }}.</li>
                  }
                  <li>Retido: {{ est.irrf!.valor | currency }}.</li>
                </ul>
                <p class="texto-suave">Tabela: {{ est.irrf!.fonte }}.</p>
              </details>
            }
          } @else {
            <p class="texto-suave" role="status">Calculando…</p>
          }
        </section>
      </div>

      <section class="painel-bloco" aria-labelledby="titulo-historico">
        <h2 id="titulo-historico">Histórico de salários</h2>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead><tr><th scope="col">A partir de</th><th scope="col" class="direita">Salário bruto</th>
              <th scope="col" class="direita">Mudança</th><th scope="col"><span class="so-leitor">Ações</span></th></tr></thead>
            <tbody>
              @for (l of historico(); track l.salario.id) {
                <tr>
                  <td class="mes-vigencia">{{ nomeDoMes(l.salario.inicio) }}</td>
                  <td class="direita">{{ l.salario.salarioBruto | currency }}</td>
                  <td class="direita">{{ l.variacao === null ? 'primeiro' : variacao(l.variacao) }}</td>
                  <td class="direita">
                    @if (historico().length > 1) {
                      <button type="button" class="botao botao-pequeno" (click)="excluirSalario(e, l.salario.id, l.salario.inicio)">
                        Apagar<span class="so-leitor"> o salário de {{ nomeDoMes(l.salario.inicio) }}</span></button>
                    }
                  </td>
                </tr>
              }
            </tbody>
          </table>
        </div>
        <form class="form-aumento" (ngSubmit)="aumento(e)" #fa="ngForm">
          <label>Aumento a partir de (o mês) <app-campo-data name="inicio" [(ngModel)]="aumentoInicio" obrigatorio /></label>
          <label>Salário bruto novo <app-campo-valor name="valor" [(ngModel)]="aumentoValor" obrigatorio [minimo]="0.01" /></label>
          <button class="botao botao-primario" [disabled]="fa.invalid || salvando()">Gravar aumento</button>
        </form>
        <p class="texto-suave">Os meses antes do aumento continuam com o salário de antes. Gravar no mesmo mês troca o valor.</p>
      </section>

      @if (outros().length) {
        <section class="painel-bloco" aria-labelledby="titulo-outros">
          <h2 id="titulo-outros">Outros empregos</h2>
          <ul class="lista-simples">
            @for (o of outros(); track o.id) {
              <li><button type="button" class="link" (click)="escolher(o.id)">{{ o.empregador }}</button>
                ({{ o.admissao | date: 'MM/yyyy' }}@if (o.saida) { a {{ o.saida | date: 'MM/yyyy' }} })</li>
            }
          </ul>
        </section>
      }
    }
  `,
  styles: [`
    .dados-emprego, .resumo-mes { display: grid; grid-template-columns: auto 1fr; row-gap: 6px; margin: 0; }
    .dados-emprego dt, .resumo-mes dt { padding-right: 16px; }
    .dados-emprego dd, .resumo-mes dd { margin: 0; }
    .mes-vigencia { white-space: nowrap; }
    .resumo-mes dd { text-align: right; font-variant-numeric: tabular-nums; }
    .resumo-mes .total { font-weight: 600; border-top: 1px solid var(--linha); padding-top: 6px; }
    .como-calculei { margin-top: 12px; }
    .como-calculei h3 { font-size: 0.95rem; margin: 12px 0 4px; }
    .como-calculei ul { margin: 0; padding-left: 20px; }
    .form-aumento { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; margin-top: 16px; }
    .aviso-estimativa { margin-top: 0; }
    .lista-simples { margin: 0; padding-left: 20px; }
  `]
})
export class TrabalhoSalarioComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);

  readonly RegimeEmprego = RegimeEmprego;
  readonly regimes = REGIMES;
  readonly aliquota = aliquota;
  readonly variacao = variacao;
  readonly nomeDoMes = nomeDoMes;
  readonly explicarDeducao = explicarDeducao;
  readonly regime = rotuloRegime;
  readonly cnpj = formatarCnpj;

  readonly carregando = signal(true);
  readonly salvando = signal(false);
  readonly empregos = signal<Emprego[]>([]);
  readonly contas = signal<Conta[]>([]);
  /** O emprego que a tela mostra (o atual, ou o escolhido em "Outros empregos"). */
  readonly escolhidoId = signal<number | null>(null);
  readonly atual = computed(() => {
    const lista = this.empregos();
    return lista.find((e) => e.id === this.escolhidoId()) ?? empregoAtual(lista);
  });
  readonly outros = computed(() => this.empregos().filter((e) => e.id !== this.atual()?.id));
  readonly historico = computed(() => historicoDeSalarios(this.atual()?.salarios ?? []));
  readonly formulario = signal<FormularioEmprego | null>(null);
  readonly editandoId = signal<number | null>(null);
  readonly periodo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  readonly estimativa = signal<EstimativaSalario | null>(null);
  readonly erroEstimativa = signal<string | null>(null);
  aumentoInicio: string | null = null;
  aumentoValor: number | null = null;

  ngOnInit(): void {
    this.api.contas().subscribe({ next: (c) => this.contas.set(c), error: () => this.contas.set([]) });
    this.recarregar();
  }

  private recarregar(): void {
    this.api.empregos().subscribe({
      next: (lista) => {
        this.empregos.set(lista);
        this.carregando.set(false);
        if (!lista.length) this.novo();
        else this.estimar();
      },
      error: (e) => { this.carregando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  private estimar(): void {
    const e = this.atual();
    if (!e) return;
    this.estimativa.set(null);
    this.erroEstimativa.set(null);
    this.api.estimativaSalario(e.id, mesDaApi(this.periodo())).subscribe({
      next: (est) => this.estimativa.set(est),
      error: (err) => this.erroEstimativa.set(mensagemDeErro(err))
    });
  }

  mudarPeriodo(p: MesAno): void {
    this.periodo.set(p);
    this.estimar();
  }

  escolher(id: number): void {
    this.escolhidoId.set(id);
    this.estimar();
  }

  novo(): void {
    this.editandoId.set(null);
    this.formulario.set(formularioVazio());
  }

  editar(e: Emprego): void {
    this.editandoId.set(e.id);
    this.formulario.set(formularioDe(e));
  }

  cancelar(): void {
    this.formulario.set(null);
    this.editandoId.set(null);
  }

  salvar(): void {
    const f = this.formulario();
    if (!f) return;
    const id = this.editandoId();
    this.salvando.set(true);
    const pedido = id ? this.api.atualizarEmprego(id, paraSalvar(f, false)) : this.api.criarEmprego(paraSalvar(f, true));
    pedido.subscribe({
      next: (e) => {
        this.salvando.set(false);
        this.avisos.sucesso(id ? 'Emprego atualizado.' : 'Emprego cadastrado.');
        this.cancelar();
        this.escolhidoId.set(e.id);
        this.recarregar();
      },
      error: (err) => { this.salvando.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  async excluir(): Promise<void> {
    const id = this.editandoId();
    const e = this.empregos().find((x) => x.id === id);
    if (!e) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Excluir emprego',
      mensagem: `Excluir “${e.empregador}” e o histórico de salários dele? As transações não mudam.`, confirmar: 'Excluir', perigo: true });
    if (!ok) return;
    this.api.excluirEmprego(e.id).subscribe({
      next: () => { this.avisos.sucesso('Emprego excluído.'); this.cancelar(); this.escolhidoId.set(null); this.recarregar(); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  aumento(e: Emprego): void {
    if (!this.aumentoInicio || this.aumentoValor === null) return;
    this.salvando.set(true);
    this.api.definirSalario(e.id, this.aumentoInicio, this.aumentoValor).subscribe({
      next: (novo) => {
        this.salvando.set(false);
        this.avisos.sucesso(`Salário gravado a partir de ${nomeDoMes(novo.salarios[0].inicio)}.`);
        this.aumentoInicio = null;
        this.aumentoValor = null;
        this.trocar(novo);
      },
      error: (err) => { this.salvando.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  async excluirSalario(e: Emprego, salarioId: number, inicio: string): Promise<void> {
    const ok = await this.confirmacao.confirmar({ titulo: 'Apagar salário',
      mensagem: `Apagar o salário de ${nomeDoMes(inicio)}? Os meses dele passam a usar o salário de antes.`, confirmar: 'Apagar', perigo: true });
    if (!ok) return;
    this.api.excluirSalario(e.id, salarioId).subscribe({
      next: (novo) => { this.avisos.sucesso('Salário apagado.'); this.trocar(novo); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  /** Troca o emprego na lista com o que a API devolveu e refaz a estimativa do mês. */
  private trocar(novo: Emprego): void {
    this.empregos.update((l) => l.map((x) => (x.id === novo.id ? novo : x)));
    this.estimar();
  }
}
