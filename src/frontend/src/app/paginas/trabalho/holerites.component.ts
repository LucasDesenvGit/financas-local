import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { Emprego, Holerite, LancamentoHolerite, ResumoHolerite, SituacaoHolerite, TipoLancamentoHolerite } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { MesAno, SeletorMesComponent } from '../../compartilhado/ui/seletor-mes.component';
import {
  TIPOS_DESCONTO, TIPOS_PROVENTO, ehPorHora, ehProvento, lancamentoNovo, paraSalvar, rotuloTipo, textoSituacao, trocarTipo
} from './holerites';
import { empregoAtual, mesDaApi, nomeDoMes } from './salario';
import { RecebimentoHoleriteComponent } from './recebimento-holerite.component';

/**
 * #211 (ADR 0065): Trabalho › Holerites. A lista dos meses (bruto, descontos, líquido) e o holerite escolhido: os
 * lançamentos do mês (horas extras, adicional noturno, bônus, descontos), o cálculo linha a linha e "Fechar o mês", que
 * congela os números (Nielsen 3: "Reabrir" desfaz, e fica nos acessos recentes). O holerite aberto fica na URL
 * (?holerite=), para o voltar do navegador funcionar (ADR 0018). Estimativa, não holerite; não é receita.
 */
@Component({
  selector: 'app-trabalho-holerites',
  imports: [FormsModule, DatePipe, RouterLink, MoedaPipe, CampoValorComponent, ListaSuspensaDirective, SeletorMesComponent,
    RecebimentoHoleriteComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina"><h1>Holerites</h1></header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (!emprego()) {
      <section class="painel-bloco">
        <p>Cadastre o emprego antes, em <a routerLink="/trabalho/salario">Trabalho › Salário</a>: o holerite parte do salário dele.</p>
      </section>
    } @else {
      <p class="texto-suave aviso-estimativa">Estimativa, não holerite: confira com o da empresa. Nada daqui entra nos relatórios.</p>
      <div class="grade-lateral">
        <section class="painel-bloco" aria-labelledby="titulo-meses">
          <h2 id="titulo-meses">{{ emprego()!.empregador }}</h2>
          <div class="novo-holerite">
            <app-seletor-mes [valor]="mesNovo()" (mudou)="mesNovo.set($event)" />
            <button type="button" class="botao botao-primario" [disabled]="ocupado()" (click)="criar()">Abrir o holerite do mês</button>
          </div>
          @if (lista().length) {
            <ul class="lista-meses">
              @for (r of lista(); track r.id) {
                <li>
                  <button type="button" class="item-mes" [class.ativo]="r.id === aberto()?.id" [attr.aria-current]="r.id === aberto()?.id ? 'true' : null"
                    (click)="abrir(r.id)">
                    <span class="mes-nome">{{ nome(r.mes) }}</span>
                    <span class="etiqueta" [class.etiqueta-suave]="r.situacao === Situacao.Aberto">{{ situacao(r.situacao) }}</span>
                    <span class="mes-liquido">{{ r.liquido === null ? 'sem cálculo' : (r.liquido | currency) }}</span>
                  </button>
                </li>
              }
            </ul>
          } @else {
            <p class="vazio">Nenhum holerite ainda. Escolha o mês e clique em “Abrir o holerite do mês”.</p>
          }
        </section>

        @if (aberto(); as h) {
          <section class="painel-bloco" aria-labelledby="titulo-holerite">
            <div class="cabecalho-bloco">
              <h2 id="titulo-holerite">{{ nome(h.mes) }} <span class="etiqueta" [class.etiqueta-suave]="!fechado()">{{ situacao(h.situacao) }}</span></h2>
            </div>
            @if (h.aviso) { <p class="aviso-inline">{{ h.aviso }}</p> }

            @if (!fechado()) {
              <form (ngSubmit)="salvar()" #fh="ngForm" class="formulario">
                <div class="dias-dsr">
                  <label>Dias úteis <input name="uteis" type="number" inputmode="numeric" min="1" max="31" [(ngModel)]="diasUteis" required></label>
                  <label>Domingos e feriados <input name="descanso" type="number" inputmode="numeric" min="0" max="31" [(ngModel)]="diasDescanso" required></label>
                  <p class="texto-suave">Para o DSR das horas extras. Acerte os feriados do mês.</p>
                </div>
                <div class="lancamentos">
                  @for (l of lancamentos(); track $index; let i = $index) {
                    <fieldset class="lancamento">
                      <legend class="so-leitor">Lançamento {{ i + 1 }}</legend>
                      <label>Tipo
                        <select [name]="'tipo' + i" [ngModel]="l.tipo" (ngModelChange)="mudarTipo(i, $event)">
                          <optgroup label="Proventos">@for (t of tiposProvento; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }</optgroup>
                          <optgroup label="Descontos">@for (t of tiposDesconto; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }</optgroup>
                        </select>
                      </label>
                      <label>Descrição (opcional) <input [name]="'descricao' + i" [(ngModel)]="l.descricao" maxlength="100"></label>
                      @if (porHora(l.tipo)) {
                        <label>Horas <input [name]="'horas' + i" type="number" inputmode="decimal" min="0.01" max="300" step="0.01" [(ngModel)]="l.quantidade" required></label>
                        <label>Percentual (%) <input [name]="'pct' + i" type="number" inputmode="decimal" min="0.01" max="300" step="0.01" [(ngModel)]="l.percentual" required></label>
                      } @else {
                        <label>{{ l.tipo === Tipo.ValeTransporte ? 'Custo do mês (opcional)' : 'Valor' }}
                          <app-campo-valor [name]="'valor' + i" [(ngModel)]="l.valor" [obrigatorio]="l.tipo !== Tipo.ValeTransporte" [minimo]="0.01" /></label>
                      }
                      <div class="marcas">
                        @if (l.tipo === Tipo.OutroProvento) {
                          <label class="caixa"><input type="checkbox" [name]="'inss' + i" [(ngModel)]="l.incideInss"> Tem INSS</label>
                          <label class="caixa"><input type="checkbox" [name]="'irrf' + i" [(ngModel)]="l.incideIrrf"> Tem IRRF</label>
                        }
                        @if (l.tipo === Tipo.OutroDesconto) {
                          <label class="caixa"><input type="checkbox" [name]="'deduz' + i" [(ngModel)]="l.deduzIrrf"> Deduz do IR</label>
                        }
                        <label class="caixa"><input type="checkbox" [name]="'recorrente' + i" [(ngModel)]="l.recorrente"> Repetir nos próximos meses</label>
                        <button type="button" class="botao botao-pequeno" (click)="remover(i)">Remover<span class="so-leitor"> o lançamento {{ i + 1 }}</span></button>
                      </div>
                    </fieldset>
                  }
                </div>
                <div class="rodape-form">
                  <button type="button" class="botao" (click)="acrescentar()">+ Lançamento</button>
                  <span class="cresce"></span>
                  <button class="botao botao-primario" [disabled]="fh.invalid || ocupado()">Salvar e calcular</button>
                </div>
              </form>
            } @else if (h.lancamentos.length) {
              <p class="texto-suave">Fechado em {{ h.fechadoEm | date: 'dd/MM/yyyy' }}. Para mudar, reabra.</p>
            }

            @if (h.calculo; as c) {
              <div class="tabela-rolagem">
                <table class="tabela holerite">
                  <caption class="so-leitor">Holerite de {{ nome(h.mes) }}</caption>
                  <thead><tr><th scope="col">Descrição</th><th scope="col" class="direita">Proventos</th>
                    <th scope="col" class="direita">Descontos</th></tr></thead>
                  <tbody>
                    @for (p of c.proventos; track $index) {
                      <tr><td>{{ p.descricao }}@if (p.referencia) { <span class="detalhe texto-suave">{{ p.referencia }}</span> }</td>
                        <td class="direita">{{ p.valor | currency }}</td><td></td></tr>
                    }
                    @for (d of c.descontos; track $index) {
                      <tr><td>{{ d.descricao }}@if (d.referencia) { <span class="detalhe texto-suave">{{ d.referencia }}</span> }</td>
                        <td></td><td class="direita">{{ d.valor | currency }}</td></tr>
                    }
                  </tbody>
                  <tfoot>
                    <tr><th scope="row">Totais</th><td class="direita">{{ c.totalProventos | currency }}</td>
                      <td class="direita">{{ c.totalDescontos | currency }}</td></tr>
                    <tr class="liquido"><th scope="row" colspan="2">Líquido estimado</th><td class="direita">{{ c.liquido | currency }}</td></tr>
                  </tfoot>
                </table>
              </div>
              <p class="texto-suave">Salário de {{ c.salarioContratual | currency }} ({{ c.diasTrabalhados }} de 30 dias), hora a {{ c.valorHora | currency }}.
                @if (!c.semEncargos) { Base do INSS: {{ c.baseInss | currency }}. FGTS do mês (a empresa deposita, não sai do salário): {{ c.fgtsDoMes | currency }}. }
              </p>
            }
            <div class="rodape-form">
              @if (fechado()) {
                <button type="button" class="botao" [disabled]="ocupado()" (click)="reabrir()">Reabrir</button>
              } @else {
                <button type="button" class="botao botao-perigo" [disabled]="ocupado()" (click)="excluir()">Excluir</button>
                <span class="cresce"></span>
                <button type="button" class="botao" [disabled]="ocupado() || !h.calculo || mudou()" (click)="fechar()">Fechar o mês</button>
              }
            </div>
            @if (!fechado() && mudou()) { <p class="texto-suave">Salve antes de fechar.</p> }
            <app-recebimento-holerite [holeriteId]="h.id" />
          </section>
        }
      </div>
    }
  `,
  styles: [`
    .grade-lateral { display: grid; grid-template-columns: minmax(240px, 1fr) minmax(0, 2.4fr); gap: 16px; align-items: start; }
    @media (max-width: 900px) { .grade-lateral { grid-template-columns: minmax(0, 1fr); } }
    .novo-holerite { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px; }
    .lista-meses { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; }
    .item-mes { width: 100%; display: grid; grid-template-columns: 1fr auto; gap: 2px 8px; text-align: left; padding: 8px 10px;
      border: 1px solid var(--linha); border-radius: 10px; background: var(--superficie); color: inherit; cursor: pointer; min-height: 44px; }
    .item-mes.ativo { border-color: var(--primaria); background: var(--primaria-suave); }
    .mes-liquido { grid-column: 1 / -1; font-variant-numeric: tabular-nums; color: var(--texto-suave); }
    .dias-dsr { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; }
    .dias-dsr input { width: 7em; }
    .lancamentos { display: grid; gap: 10px; }
    .lancamento { display: flex; flex-wrap: wrap; gap: 10px; align-items: flex-end; border: 1px solid var(--linha); border-radius: 10px; padding: 10px; margin: 0; }
    .lancamento input[type=number] { width: 7em; }
    .marcas { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
    .marcas .caixa { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; min-height: 24px; }
    .holerite tfoot th, .holerite tfoot td { font-weight: 600; }
    .holerite .detalhe { display: block; font-size: 0.8rem; }
    .holerite td.direita { white-space: nowrap; }
    @media (max-width: 600px) { .holerite th, .holerite td { padding: 8px 6px; } }
    .holerite .liquido td, .holerite .liquido th { border-top: 1px solid var(--linha); }
    .aviso-estimativa { margin-top: 0; }
  `]
})
export class TrabalhoHoleritesComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly Tipo = TipoLancamentoHolerite;
  readonly Situacao = SituacaoHolerite;
  readonly tiposProvento = TIPOS_PROVENTO;
  readonly tiposDesconto = TIPOS_DESCONTO;
  readonly porHora = ehPorHora;
  readonly nome = nomeDoMes;
  readonly situacao = textoSituacao;

  readonly carregando = signal(true);
  readonly ocupado = signal(false);
  readonly emprego = signal<Emprego | null>(null);
  readonly lista = signal<ResumoHolerite[]>([]);
  readonly aberto = signal<Holerite | null>(null);
  readonly fechado = computed(() => this.aberto()?.situacao === SituacaoHolerite.Fechado);
  readonly lancamentos = signal<LancamentoHolerite[]>([]);
  readonly mesNovo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  diasUteis = 0;
  diasDescanso = 0;
  /** O texto do que está salvo, para saber se a pessoa mexeu (e pedir para salvar antes de fechar). */
  private salvo = '';

  ngOnInit(): void {
    this.api.empregos().subscribe({
      next: (lista) => {
        this.emprego.set(empregoAtual(lista));
        this.carregando.set(false);
        this.recarregarLista();
        const id = Number(this.rota.snapshot.queryParamMap.get('holerite'));
        if (id) this.abrir(id);
      },
      error: (e) => { this.carregando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  /** Mexeu nos lançamentos ou nos dias desde o que está salvo. */
  mudou(): boolean {
    return this.estado() !== this.salvo;
  }

  private estado(): string {
    return JSON.stringify([this.diasUteis, this.diasDescanso, paraSalvar(this.lancamentos())]);
  }

  private recarregarLista(): void {
    const e = this.emprego();
    if (!e) return;
    this.api.holerites(e.id).subscribe({ next: (l) => this.lista.set(l), error: (err) => this.avisos.erro(mensagemDeErro(err)) });
  }

  private mostrar(h: Holerite): void {
    this.aberto.set(h);
    this.diasUteis = h.diasUteis;
    this.diasDescanso = h.diasDescanso;
    this.lancamentos.set(h.lancamentos.map((l) => ({ ...l })));
    this.salvo = this.estado();
    this.router.navigate([], { relativeTo: this.rota, queryParams: { holerite: h.id }, replaceUrl: true });
  }

  abrir(id: number): void {
    this.api.holerite(id).subscribe({ next: (h) => this.mostrar(h), error: (e) => this.avisos.erro(mensagemDeErro(e)) });
  }

  criar(): void {
    const e = this.emprego();
    if (!e) return;
    this.ocupado.set(true);
    this.api.criarHolerite(e.id, mesDaApi(this.mesNovo())).subscribe({
      next: (h) => { this.ocupado.set(false); this.mostrar(h); this.recarregarLista(); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  acrescentar(): void {
    this.lancamentos.update((l) => [...l, lancamentoNovo(TipoLancamentoHolerite.HoraExtra)]);
  }

  remover(i: number): void {
    this.lancamentos.update((l) => l.filter((_, j) => j !== i));
  }

  mudarTipo(i: number, tipo: TipoLancamentoHolerite): void {
    this.lancamentos.update((l) => l.map((x, j) => (j === i ? trocarTipo(x, Number(tipo)) : x)));
  }

  salvar(): void {
    const h = this.aberto();
    if (!h) return;
    this.ocupado.set(true);
    this.api.salvarHolerite(h.id, this.diasUteis, this.diasDescanso, paraSalvar(this.lancamentos())).subscribe({
      next: (novo) => { this.ocupado.set(false); this.mostrar(novo); this.recarregarLista(); this.avisos.sucesso('Holerite calculado.'); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  fechar(): void {
    const h = this.aberto();
    if (!h) return;
    this.acao(this.api.fecharHolerite(h.id), `Holerite de ${nomeDoMes(h.mes)} fechado: os números ficam como estão.`);
  }

  reabrir(): void {
    const h = this.aberto();
    if (!h) return;
    this.acao(this.api.reabrirHolerite(h.id), 'Holerite reaberto: ele volta a ser calculado com o salário e as tabelas de agora.');
  }

  private acao(pedido: ReturnType<ApiService['fecharHolerite']>, mensagem: string): void {
    this.ocupado.set(true);
    pedido.subscribe({
      next: (novo) => { this.ocupado.set(false); this.mostrar(novo); this.recarregarLista(); this.avisos.sucesso(mensagem); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  async excluir(): Promise<void> {
    const h = this.aberto();
    if (!h) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Excluir holerite',
      mensagem: `Excluir o holerite de ${nomeDoMes(h.mes)} e os lançamentos dele?`, confirmar: 'Excluir', perigo: true });
    if (!ok) return;
    this.api.excluirHolerite(h.id).subscribe({
      next: () => {
        this.aberto.set(null);
        this.router.navigate([], { relativeTo: this.rota, queryParams: { holerite: null }, replaceUrl: true });
        this.recarregarLista();
        this.avisos.sucesso('Holerite excluído.');
      },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  ehProvento = ehProvento;
  rotulo = rotuloTipo;
}
