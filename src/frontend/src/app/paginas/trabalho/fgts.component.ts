import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { Emprego, Fgts, MotivoSaqueFgts, TipoLancamentoFgts } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { empregoAtual } from './salario';
import { MOTIVOS, TIPOS, descreverLancamento, totalDepositado, ultimosMeses } from './fgts';

/**
 * #216 (ADR 0066): Trabalho › FGTS. O saldo estimado (os depósitos de 8%, corrigidos a 3% ao ano), conferido com o saldo
 * do app do FGTS (a diferença aparece; Nielsen 1), os saques e a distribuição de resultados, e os últimos meses. Fora do
 * patrimônio e dos relatórios.
 */
@Component({
  selector: 'app-trabalho-fgts',
  imports: [FormsModule, DatePipe, RouterLink, MoedaPipe, CampoDataComponent, CampoValorComponent, ListaSuspensaDirective],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina"><h1>FGTS</h1></header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (!emprego()) {
      <section class="painel-bloco"><p>Cadastre o emprego antes, em <a routerLink="/trabalho/salario">Trabalho › Salário</a>.</p></section>
    } @else if (erro(); as msg) {
      <section class="painel-bloco"><p class="aviso-inline">{{ msg }}</p></section>
    } @else if (fgts(); as f) {
      <p class="texto-suave aviso-estimativa">Estimativa: 8% do salário bruto de cada mês (e do 13º e das férias), corrigido a 3% ao
        ano. A TR fica de fora; confira com o app do FGTS de vez em quando. O FGTS não entra no patrimônio do Painel nem nos
        relatórios: é seu, mas só sai em casos da lei.</p>
      <div class="grade-2">
        <section class="painel-bloco" aria-labelledby="titulo-saldo">
          <h2 id="titulo-saldo">{{ emprego()!.empregador }}</h2>
          <p class="saldo">{{ f.saldo | currency }}</p>
          <p class="texto-suave">Saldo estimado em {{ f.ate | date: 'MM/yyyy' }}. Depositado no período: {{ depositado() | currency }}.</p>
          @if (ultimaConferencia(); as c) {
            <p>Na última conferência ({{ c.data | date: 'dd/MM/yyyy' }}), o app disse {{ c.informado | currency }} e o site estimava
              {{ c.estimado | currency }}: <strong>{{ c.diferenca >= 0 ? 'a mais' : 'a menos' }} {{ (c.diferenca < 0 ? -c.diferenca : c.diferenca) | currency }}</strong>
              no app. Diferença grande pode ser depósito que a empresa não fez.</p>
          }
          <h3>Lançamentos</h3>
          @if (f.lancamentos.length) {
            <ul class="lancamentos">
              @for (l of f.lancamentos; track l.id) {
                <li><span>{{ l.data | date: 'dd/MM/yyyy' }} · {{ descrever(l) }}</span>
                  <span class="valor">{{ l.valor | currency }}</span>
                  <button type="button" class="botao botao-pequeno" (click)="excluir(l.id)">Excluir<span class="so-leitor"> o lançamento de {{ l.data | date: 'dd/MM/yyyy' }}</span></button></li>
              }
            </ul>
          } @else {
            <p class="texto-suave">Nenhum ainda. Comece informando o saldo que você vê no app do FGTS.</p>
          }
        </section>

        <section class="painel-bloco" aria-labelledby="titulo-lancar">
          <h2 id="titulo-lancar">Lançar</h2>
          <form class="formulario" (ngSubmit)="lancar()" #fl="ngForm">
            <label>O quê
              <select name="tipo" [(ngModel)]="tipo">
                @for (t of tipos; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }
              </select>
            </label>
            <label>Data <app-campo-data name="data" [(ngModel)]="data" obrigatorio /></label>
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="valor" obrigatorio [minimo]="0" /></label>
            @if (tipo === Tipo.Saque) {
              <label>Motivo do saque
                <select name="motivo" [(ngModel)]="motivo" required>
                  @for (m of motivos; track m.valor) { <option [ngValue]="m.valor">{{ m.rotulo }}</option> }
                </select>
              </label>
              @if (motivo === Motivo.SaqueAniversario) {
                <p class="texto-suave">Com o saque-aniversário, se você for demitido sem justa causa, sai só a multa de 40%: o saldo fica
                  bloqueado. Voltar para o saque-rescisão leva 2 anos.</p>
              }
            }
            <div class="rodape-form"><span class="cresce"></span>
              <button class="botao botao-primario" [disabled]="fl.invalid || ocupado()">Lançar</button></div>
          </form>
        </section>
      </div>

      <section class="painel-bloco" aria-labelledby="titulo-meses-fgts">
        <h2 id="titulo-meses-fgts">Os últimos meses</h2>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead><tr><th scope="col">Mês</th><th scope="col" class="direita so-largo">Correção</th><th scope="col" class="direita">Depósito</th>
              <th scope="col" class="direita">Saques</th><th scope="col" class="direita">Saldo</th></tr></thead>
            <tbody>
              @for (m of meses(); track m.mes) {
                <tr><td>{{ m.mes | date: 'MM/yyyy' }}</td><td class="direita so-largo">{{ m.correcao | currency }}</td>
                  <td class="direita">{{ m.deposito + m.distribuicao | currency }}</td><td class="direita">{{ m.saques | currency }}</td>
                  <td class="direita">{{ m.saldo | currency }}@if (m.conferido) { <span class="etiqueta">conferido</span> }</td></tr>
              }
            </tbody>
          </table>
        </div>
      </section>
    }
  `,
  styles: [`
    .saldo { font-size: 1.8rem; font-weight: 700; margin: 4px 0; font-variant-numeric: tabular-nums; }
    .lancamentos { list-style: none; margin: 0; padding: 0; display: grid; gap: 6px; }
    .lancamentos li { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
    .lancamentos .valor { margin-left: auto; font-variant-numeric: tabular-nums; }
    h3 { font-size: 1rem; margin: 16px 0 8px; }
    .aviso-estimativa { margin-top: 0; }
    td.direita { white-space: nowrap; }
    @media (max-width: 600px) { .so-largo { display: none; } }
  `]
})
export class TrabalhoFgtsComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);

  readonly Tipo = TipoLancamentoFgts;
  readonly Motivo = MotivoSaqueFgts;
  readonly tipos = TIPOS;
  readonly motivos = MOTIVOS;
  readonly descrever = descreverLancamento;
  readonly carregando = signal(true);
  readonly ocupado = signal(false);
  readonly emprego = signal<Emprego | null>(null);
  readonly fgts = signal<Fgts | null>(null);
  readonly erro = signal<string | null>(null);
  readonly meses = computed(() => ultimosMeses(this.fgts()?.meses ?? []));
  readonly depositado = computed(() => totalDepositado(this.fgts()?.meses ?? []));
  readonly ultimaConferencia = computed(() => this.fgts()?.conferencias.at(-1) ?? null);
  tipo = TipoLancamentoFgts.Conferencia;
  data: string | null = null;
  valor: number | null = null;
  motivo: MotivoSaqueFgts | null = MotivoSaqueFgts.Rescisao;

  ngOnInit(): void {
    this.api.empregos().subscribe({
      next: (lista) => { this.emprego.set(empregoAtual(lista)); this.carregando.set(false); this.carregar(); },
      error: (e) => { this.carregando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  private carregar(): void {
    const e = this.emprego();
    if (!e) return;
    this.api.fgts(e.id).subscribe({ next: (f) => { this.fgts.set(f); this.erro.set(null); }, error: (err) => this.erro.set(mensagemDeErro(err)) });
  }

  lancar(): void {
    const e = this.emprego();
    if (!e || !this.data || this.valor === null) return;
    this.ocupado.set(true);
    this.api.lancarFgts(e.id, { data: this.data, tipo: this.tipo, valor: this.valor,
      motivo: this.tipo === TipoLancamentoFgts.Saque ? this.motivo : null }).subscribe({
      next: (f) => { this.ocupado.set(false); this.fgts.set(f); this.valor = null; this.avisos.sucesso('Lançado: o saldo foi recalculado.'); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  async excluir(id: number): Promise<void> {
    const e = this.emprego();
    if (!e) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Excluir lançamento', mensagem: 'Excluir este lançamento do FGTS?',
      confirmar: 'Excluir', perigo: true });
    if (!ok) return;
    this.api.excluirLancamentoFgts(e.id, id).subscribe({
      next: (f) => { this.fgts.set(f); this.avisos.sucesso('Lançamento excluído.'); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }
}
