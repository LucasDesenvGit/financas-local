import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { DecimoTerceiro, Emprego } from '../../core/modelos';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { aliquota, empregoAtual } from './salario';
import { anoDaUrl } from './decimo-terceiro';

/**
 * #214 (ADR 0065): Trabalho › 13º salário. O 13º do ano (proporcional, com a média das horas extras), as duas parcelas
 * com as datas e "como calculei" (Nielsen 10), e o que a pessoa recebeu da 1ª (adiantada nas férias, por exemplo). O
 * ano fica na URL (?ano=, ADR 0018). Estimativa; não é receita.
 */
@Component({
  selector: 'app-trabalho-decimo-terceiro',
  imports: [FormsModule, DatePipe, RouterLink, MoedaPipe, CampoDataComponent, CampoValorComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina">
      <h1>13º salário</h1>
      <div class="seletor-ano" role="group" aria-label="Ano">
        <button type="button" class="botao-icone" aria-label="Ano anterior" (click)="mudarAno(-1)">‹</button>
        <strong aria-live="polite">{{ ano() }}</strong>
        <button type="button" class="botao-icone" aria-label="Próximo ano" (click)="mudarAno(1)">›</button>
      </div>
    </header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (!emprego()) {
      <section class="painel-bloco">
        <p>Cadastre o emprego antes, em <a routerLink="/trabalho/salario">Trabalho › Salário</a>.</p>
      </section>
    } @else if (erro(); as msg) {
      <section class="painel-bloco"><p class="aviso-inline">{{ msg }}</p></section>
    } @else if (decimo(); as d) {
      <p class="texto-suave aviso-estimativa">Estimativa, não holerite: confira com o da empresa. Nada daqui entra nos relatórios.</p>
      @if (d.saida) {
        <p class="aviso-inline">Você saiu em {{ d.saida | date: 'dd/MM/yyyy' }}: o 13º proporcional vem na rescisão, e não nas datas abaixo.</p>
      }
      <div class="grade-2">
        <section class="painel-bloco" aria-labelledby="titulo-parcelas">
          <h2 id="titulo-parcelas">{{ emprego()!.empregador }}</h2>
          <dl class="resumo">
            <dt>1ª parcela, {{ d.primeira.informada ? 'recebida em' : 'até' }} {{ d.primeira.data | date: 'dd/MM' }}</dt>
            <dd>{{ d.primeira.valor | currency }}</dd>
            <dt>2ª parcela, até {{ d.segunda.data | date: 'dd/MM' }}</dt><dd>{{ d.segunda.valor | currency }}</dd>
            <dt class="total">O 13º líquido</dt><dd class="total">{{ d.liquido | currency }}</dd>
          </dl>
          <details class="como-calculei">
            <summary>Como calculei</summary>
            <ul>
              <li>{{ d.avos }}/12 avos: cada mês do ano em que você trabalhou 15 dias ou mais.</li>
              <li>Salário de {{ d.salarioBase | currency }}{{ d.saida ? ' (o do mês da saída)' : ' (o de dezembro)' }} mais a média das horas
                extras, do adicional noturno e do DSR dos holerites do ano ({{ d.mediaVariaveis | currency }}).</li>
              <li>13º bruto: ({{ d.salarioBase | currency }} + {{ d.mediaVariaveis | currency }}) × {{ d.avos }}/12 = {{ d.total | currency }}.</li>
              <li>1ª parcela: {{ d.primeira.informada ? 'o que você informou' : 'metade do 13º, sem desconto' }}.</li>
              <li>2ª parcela: o 13º, menos o INSS ({{ d.inss.valor | currency }}), o IRRF ({{ d.irrf.valor | currency }}) e a 1ª parcela.
                O IR do 13º é separado do salário: base de {{ d.irrf.base | currency }}, na faixa de {{ aliquota(d.irrf.aliquota) }}@if (d.irrf.reducaoLei15270 > 0) {<span>, menos a redução da Lei 15.270/2025 ({{ d.irrf.reducaoLei15270 | currency }})</span>}.</li>
            </ul>
          </details>
        </section>

        <section class="painel-bloco" aria-labelledby="titulo-primeira">
          <h2 id="titulo-primeira">A 1ª parcela que veio</h2>
          <p class="texto-suave">Se a 1ª parcela veio antes (com as férias) ou num valor diferente, informe: a 2ª é recalculada.</p>
          <form class="formulario" (ngSubmit)="salvarPrimeira()">
            <label>Recebida em <app-campo-data name="em" [(ngModel)]="primeiraEm" /></label>
            <label>Valor recebido <app-campo-valor name="valor" [(ngModel)]="primeiraValor" [minimo]="0" /></label>
            <div class="rodape-form">
              @if (d.primeira.informada) {
                <button type="button" class="botao" [disabled]="ocupado()" (click)="limparPrimeira()">Voltar ao padrão</button>
              }
              <span class="cresce"></span>
              <button class="botao botao-primario" [disabled]="ocupado() || (!primeiraEm && primeiraValor === null)">Gravar</button>
            </div>
          </form>
        </section>
      </div>
    }
  `,
  styles: [`
    .seletor-ano { display: inline-flex; align-items: center; gap: 8px; }
    .resumo { display: grid; grid-template-columns: 1fr auto; row-gap: 6px; margin: 0; }
    .resumo dt { padding-right: 16px; }
    .resumo dd { margin: 0; text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .resumo .total { font-weight: 600; border-top: 1px solid var(--linha); padding-top: 6px; }
    .como-calculei { margin-top: 12px; }
    .como-calculei ul { margin: 6px 0 0; padding-left: 20px; }
    .aviso-estimativa { margin-top: 0; }
  `]
})
export class TrabalhoDecimoTerceiroComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly aliquota = aliquota;
  readonly carregando = signal(true);
  readonly ocupado = signal(false);
  readonly emprego = signal<Emprego | null>(null);
  readonly ano = signal(anoDaUrl(this.rota.snapshot.queryParamMap.get('ano'), new Date().getFullYear()));
  readonly decimo = signal<DecimoTerceiro | null>(null);
  readonly erro = signal<string | null>(null);
  primeiraEm: string | null = null;
  primeiraValor: number | null = null;

  ngOnInit(): void {
    this.api.empregos().subscribe({
      next: (lista) => { this.emprego.set(empregoAtual(lista)); this.carregando.set(false); this.carregar(); },
      error: (e) => { this.carregando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  mudarAno(passo: number): void {
    this.ano.update((a) => a + passo);
    this.router.navigate([], { relativeTo: this.rota, queryParams: { ano: this.ano() }, replaceUrl: true });
    this.carregar();
  }

  private mostrar(d: DecimoTerceiro): void {
    this.decimo.set(d);
    this.erro.set(null);
    this.primeiraEm = d.primeira.informada ? d.primeira.data : null;
    this.primeiraValor = d.primeira.informada ? d.primeira.valor : null;
  }

  private carregar(): void {
    const e = this.emprego();
    if (!e) return;
    this.decimo.set(null);
    this.api.decimoTerceiro(e.id, this.ano()).subscribe({
      next: (d) => this.mostrar(d),
      error: (err) => { this.decimo.set(null); this.erro.set(mensagemDeErro(err)); }
    });
  }

  salvarPrimeira(): void {
    this.gravar(this.primeiraEm, this.primeiraValor, '1ª parcela gravada: a 2ª foi recalculada.');
  }

  limparPrimeira(): void {
    this.gravar(null, null, 'A 1ª parcela voltou ao padrão: metade do 13º, até 30/11.');
  }

  private gravar(em: string | null, valor: number | null, mensagem: string): void {
    const e = this.emprego();
    if (!e) return;
    this.ocupado.set(true);
    this.api.definirPrimeiraParcela(e.id, this.ano(), em, valor).subscribe({
      next: (d) => { this.ocupado.set(false); this.mostrar(d); this.avisos.sucesso(mensagem); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }
}
