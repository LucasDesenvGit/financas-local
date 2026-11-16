import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { AvisoRescisao, CalculoRescisao, ComparacaoRescisao, Emprego, TipoRescisao } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { empregoAtual } from './salario';
import { TIPOS, avisoPadrao, avisosDoTipo, textoDoFgts } from './rescisao';

/**
 * #215 (ADR 0065): Trabalho › Rescisão. Simula quanto se recebe ao sair do emprego em cada tipo de desligamento (as
 * verbas, os descontos, o FGTS e o seguro-desemprego), compara os tipos lado a lado na mesma data e registra a
 * rescisão real (fecha o emprego e guarda a conta). Estimativa, não termo de rescisão.
 */
@Component({
  selector: 'app-trabalho-rescisao',
  imports: [FormsModule, DatePipe, RouterLink, MoedaPipe, CampoDataComponent, ListaSuspensaDirective],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina"><h1>Rescisão</h1></header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (!emprego()) {
      <section class="painel-bloco"><p>Cadastre o emprego antes, em <a routerLink="/trabalho/salario">Trabalho › Salário</a>.</p></section>
    } @else {
      <p class="texto-suave aviso-estimativa">Estimativa, não termo de rescisão: confira com o da empresa e com o sindicato. Nada daqui
        entra nos relatórios: a receita é a transação do que cair na conta.</p>
      @if (registrada(); as r) {
        <section class="painel-bloco" aria-labelledby="titulo-registrada">
          <h2 id="titulo-registrada">Rescisão registrada</h2>
          <p>{{ r.nomeDoTipo }}, saída em {{ r.saida | date: 'dd/MM/yyyy' }}: <strong>{{ r.liquido | currency }}</strong> na conta e
            {{ r.fgts.sacavel | currency }} para sacar do FGTS. O emprego está fechado nessa data.</p>
          <button type="button" class="botao" [disabled]="ocupado()" (click)="desfazer()">Desfazer o registro</button>
        </section>
      }
      <div class="grade-2">
        <section class="painel-bloco" aria-labelledby="titulo-simular">
          <h2 id="titulo-simular">Simular</h2>
          <form class="formulario" (ngSubmit)="simular()" #fs="ngForm">
            <label>Tipo
              <select name="tipo" [(ngModel)]="tipo" (ngModelChange)="mudarTipo($event)">
                @for (t of tipos; track t.valor) { <option [ngValue]="t.valor">{{ t.rotulo }}</option> }
              </select>
            </label>
            <label>Aviso prévio
              <select name="aviso" [(ngModel)]="aviso" (ngModelChange)="calculo.set(null)">
                @for (a of avisos(); track a.valor) { <option [ngValue]="a.valor">{{ a.rotulo }}</option> }
              </select>
            </label>
            <label>Data de saída <app-campo-data name="saida" [(ngModel)]="saida" obrigatorio (ngModelChange)="limpar()" /></label>
            <div class="rodape-form">
              <button type="button" class="botao" [disabled]="!saida || ocupado()" (click)="comparar()">Comparar os tipos</button>
              <span class="cresce"></span>
              <button class="botao botao-primario" [disabled]="!saida || ocupado()">Simular</button>
            </div>
          </form>
          @if (comparacao(); as lista) {
            <div class="tabela-rolagem comparacao">
              <table class="tabela">
                <caption class="so-leitor">Os tipos de desligamento na mesma data</caption>
                <thead><tr><th scope="col">Tipo</th><th scope="col" class="direita">Na conta</th><th scope="col" class="direita">Saca do FGTS</th></tr></thead>
                <tbody>
                  @for (c of lista; track c.tipo) {
                    <tr><th scope="row">{{ c.nome }}@if (c.seguroDesemprego) { <span class="detalhe texto-suave">e o seguro-desemprego</span> }</th>
                      <td class="direita">{{ c.liquido | currency }}</td><td class="direita">{{ c.sacavel | currency }}</td></tr>
                  }
                </tbody>
              </table>
            </div>
            <p class="texto-suave">A dispensa e o acordo com o aviso indenizado; o pedido de demissão cumprindo o aviso.</p>
          }
        </section>

        <section class="painel-bloco" aria-labelledby="titulo-conta">
          <h2 id="titulo-conta">A conta</h2>
          @if (calculo(); as c) {
            <p class="texto-suave">{{ c.nomeDoTipo }}, saída em {{ c.saida | date: 'dd/MM/yyyy' }}. Aviso de {{ c.diasDeAviso }} dias pelo
              tempo de casa@if (c.dataProjetada !== c.saida) {; com o aviso indenizado, a data projetada (para o 13º e as férias) é
              {{ c.dataProjetada | date: 'dd/MM/yyyy' }}}.</p>
            <dl class="resumo">
              @for (l of c.verbas; track $index) {
                <dt>{{ l.descricao }}@if (l.referencia) { <span class="texto-suave"> ({{ l.referencia }})</span> }</dt><dd>{{ l.valor | currency }}</dd>
              }
              @for (l of c.descontos; track $index) {
                <dt>{{ l.descricao }}</dt><dd>− {{ l.valor | currency }}</dd>
              }
              <dt class="total">Cai na conta</dt><dd class="total">{{ c.liquido | currency }}</dd>
            </dl>
            <h3>FGTS</h3>
            <p>{{ textoFgts(c.fgts.percentualMulta, c.fgts.percentualSaque) }} Saldo estimado: {{ c.fgts.saldo | currency }}, mais o
              depósito da rescisão ({{ c.fgts.depositoRescisao | currency }})@if (c.fgts.multa) {; multa: {{ c.fgts.multa | currency }}}.
              <strong>Para sacar: {{ c.fgts.sacavel | currency }}.</strong></p>
            <p>{{ c.seguroDesemprego ? 'Você pode ter direito ao seguro-desemprego (12 meses ou mais de emprego, na 1ª vez que pede).' :
              'Este tipo de desligamento não dá direito ao seguro-desemprego.' }}</p>
            @if (!registrada()) {
              <button type="button" class="botao" [disabled]="ocupado()" (click)="registrar()">Registrar esta rescisão</button>
            }
          } @else {
            <p class="texto-suave">Escolha o tipo, o aviso e a data de saída e clique em “Simular”.</p>
          }
        </section>
      </div>
    }
  `,
  styles: [`
    .resumo { display: grid; grid-template-columns: 1fr auto; row-gap: 6px; margin: 0; }
    .resumo dt { padding-right: 16px; }
    .resumo dd { margin: 0; text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .resumo .total { font-weight: 600; border-top: 1px solid var(--linha); padding-top: 6px; }
    h3 { font-size: 1rem; margin: 16px 0 6px; }
    .comparacao { margin-top: 16px; }
    .comparacao .detalhe { display: block; font-size: 0.8rem; font-weight: 400; }
    td.direita { white-space: nowrap; }
    .aviso-estimativa { margin-top: 0; }
  `]
})
export class TrabalhoRescisaoComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisosService = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);

  readonly tipos = TIPOS;
  readonly textoFgts = textoDoFgts;
  readonly carregando = signal(true);
  readonly ocupado = signal(false);
  readonly emprego = signal<Emprego | null>(null);
  readonly calculo = signal<CalculoRescisao | null>(null);
  readonly comparacao = signal<ComparacaoRescisao[] | null>(null);
  readonly registrada = signal<CalculoRescisao | null>(null);
  readonly avisos = signal(avisosDoTipo(TipoRescisao.SemJustaCausa));
  tipo = TipoRescisao.SemJustaCausa;
  aviso = AvisoRescisao.Indenizado;
  saida: string | null = null;

  ngOnInit(): void {
    this.api.empregos().subscribe({
      next: (lista) => {
        const e = empregoAtual(lista);
        this.emprego.set(e);
        this.carregando.set(false);
        if (e) this.api.rescisaoRegistrada(e.id).subscribe({ next: (r) => this.registrada.set(r.registrada) });
      },
      error: (e) => { this.carregando.set(false); this.avisosService.erro(mensagemDeErro(e)); }
    });
  }

  mudarTipo(tipo: TipoRescisao): void {
    this.avisos.set(avisosDoTipo(Number(tipo)));
    this.aviso = avisoPadrao(Number(tipo));
    this.calculo.set(null);
  }

  limpar(): void {
    this.calculo.set(null);
    this.comparacao.set(null);
  }

  simular(): void {
    const e = this.emprego();
    if (!e || !this.saida) return;
    this.api.simularRescisao(e.id, this.tipo, this.aviso, this.saida).subscribe({
      next: (c) => this.calculo.set(c),
      error: (err) => this.avisosService.erro(mensagemDeErro(err))
    });
  }

  comparar(): void {
    const e = this.emprego();
    if (!e || !this.saida) return;
    this.api.compararRescisao(e.id, this.saida).subscribe({
      next: (l) => this.comparacao.set(l),
      error: (err) => this.avisosService.erro(mensagemDeErro(err))
    });
  }

  async registrar(): Promise<void> {
    const e = this.emprego();
    const c = this.calculo();
    if (!e || !c) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Registrar a rescisão',
      mensagem: `Registrar a saída em ${c.saida.split('-').reverse().join('/')} (${c.nomeDoTipo})? O emprego fica fechado nessa data e a conta fica guardada como está.`,
      confirmar: 'Registrar' });
    if (!ok) return;
    this.ocupado.set(true);
    this.api.registrarRescisao(e.id, c.tipo, c.aviso, c.saida).subscribe({
      next: (r) => { this.ocupado.set(false); this.registrada.set(r); this.avisosService.sucesso('Rescisão registrada: o emprego foi fechado.'); },
      error: (err) => { this.ocupado.set(false); this.avisosService.erro(mensagemDeErro(err)); }
    });
  }

  async desfazer(): Promise<void> {
    const e = this.emprego();
    if (!e) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Desfazer o registro',
      mensagem: 'Desfazer o registro da rescisão? O emprego volta a ficar aberto.', confirmar: 'Desfazer', perigo: true });
    if (!ok) return;
    this.ocupado.set(true);
    this.api.desfazerRescisao(e.id).subscribe({
      next: () => { this.ocupado.set(false); this.registrada.set(null); this.avisosService.sucesso('Registro desfeito: o emprego voltou a ficar aberto.'); },
      error: (err) => { this.ocupado.set(false); this.avisosService.erro(mensagemDeErro(err)); }
    });
  }
}
