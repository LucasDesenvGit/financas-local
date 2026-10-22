import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Conta, Painel, TrocosAutomaticos, ValorCategoria } from '../core/modelos';
import { FaturaAbertaDoCartao, faturasAbertasDe, lerCartoesNoPainel } from '../compartilhado/faturas-no-painel';
import { KpiFaturasAbertasComponent } from '../compartilhado/kpi-faturas-abertas.component';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { MesAno, SeletorMesComponent } from '../compartilhado/seletor-mes.component';
import { NOMES_MESES, NOMES_MESES_CURTOS, colunasEquilibradas, dataIso, mesAnoDaUrl, paramsTransacoes, pct, primeiroDiaMes, ultimoDiaMes } from '../compartilhado/utilidades';
import { CORES_GRAFICO } from '../compartilhado/cores-graficos';
import { GraficoExplicadoComponent } from '../compartilhado/grafico-explicado.component';
import { TermoComponent } from '../compartilhado/termo.component';
import { LogoBancoComponent } from '../compartilhado/logo-banco.component';
import { MaisAnalisesComponent } from '../compartilhado/mais-analises.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';
import { PainelAvisosComponent } from './painel-avisos.component';
import { SeloServicoComponent } from '../compartilhado/selo-servico.component';

@Component({
  selector: 'app-painel',
  imports: [KpiFaturasAbertasComponent, MoedaPipe, DatePipe, DecimalPipe, RouterLink, GraficoComponent, GraficoExplicadoComponent, SeletorMesComponent, IconeComponent, TermoComponent,
    MaisAnalisesComponent, LogoBancoComponent, PainelAvisosComponent, SeloServicoComponent],
  template: `
    <header class="cabecalho-pagina">
      <div>
        <h1>Painel</h1>
        @if (dados()?.ultimaSincronizacao; as u) {
          <p class="texto-suave">Última sincronização: {{ u | date: 'dd/MM/yyyy HH:mm' }}</p>
        }
      </div>
    </header>

    @if (erro()) { <p class="erro">{{ erro() }}</p> }

    @if (dados(); as d) {
      <!-- #206: orçamentos estourados agrupados e os outros avisos, os dois com "Ocultar". -->
      <app-painel-avisos [estourados]="d.orcamentosEstourados ?? []" [alertas]="d.alertas" [periodo]="periodo()" />
      @if (d.poucoInformativas > 0) {
        <p class="nota-discreta">
          {{ d.poucoInformativas === 1 ? '1 transação' : d.poucoInformativas + ' transações' }} dos últimos 90 dias com
          <app-termo chave="descricaoPoucoInformativa">descrição pouco informativa</app-termo>@if (d.exemploPoucoInformativa) { (ex.: “{{ d.exemploPoucoInformativa }}”)}.
          <a routerLink="/transacoes" [queryParams]="linkPoucoInformativas(d)">Revisar</a>
        </p>
      }
      <!-- #170: a separação troco × provento é com prévia (ADR 0027); aqui só o aviso, com o caminho até ela. -->
      @if (d.creditosCorretoraComoReceita > 0) {
        <p class="nota-discreta">
          {{ d.creditosCorretoraComoReceita === 1 ? '1 crédito' : d.creditosCorretoraComoReceita + ' créditos' }} de
          <app-termo chave="trocoDeCompra">troco</app-termo> ou venda de FIIs e ações
          ({{ d.totalCreditosCorretoraComoReceita | currency }}) contando como rendimento.
          <a routerLink="/investimentos" [queryParams]="{ conferir: 1 }">Conferir investimentos</a>
        </p>
      }
      <!-- #173 b) (ADR 0044): o que a sincronização mudou sozinha, com o caminho para ver e para desfazer (Nielsen 1 e 3). -->
      @if (d.trocosAutomaticos; as t) {
        <p class="nota-discreta aviso-trocos" role="status">
          {{ t.quantidade === 1 ? '1 crédito' : t.quantidade + ' créditos' }} de
          <app-termo chave="trocoDeCompra">troco</app-termo> das compras de FIIs e ações ({{ t.total | currency }})
          foram para “Investimentos” na sincronização: a conta do dia fechou só com a taxa da B3.
          <a class="link-alvo" routerLink="/transacoes" [queryParams]="paramsTrocos(t)">Ver em Transações</a>
          <button type="button" class="link" [disabled]="mexendoTrocos()" (click)="desfazerTrocos()">Desfazer</button>
          <button type="button" class="link" [disabled]="mexendoTrocos()" (click)="dispensarTrocos()"
            aria-label="Ok, os trocos ficam em Investimentos">Ok</button>
        </p>
      }

      <section class="secao-painel" aria-labelledby="titulo-hoje">
        <header class="secao-titulo">
          <h2 id="titulo-hoje">Situação hoje</h2>
          <p class="texto-suave">posição em {{ hoje | date: 'dd/MM/yyyy' }} · não muda com o mês escolhido</p>
        </header>
      <!-- #322: colunas pelo número de cards, para nenhum ficar sozinho embaixo. -->
      <section class="kpis kpis-equilibrados" [style.--colunas]="colunasHoje(d)">
        <article class="kpi kpi-destaque">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.patrimonio" [tamanho]="16" /> Patrimônio líquido</span>
          <strong>{{ d.patrimonioLiquido | currency }}</strong>
          <small>contas + investimentos (cotação atual) − cartões{{ d.dividaFinanciamentos + d.dividaEmprestimos > 0 ? ' − financiamentos − empréstimos' : '' }}</small>
        </article>
        <article class="kpi">
          <span><app-termo chave="patrimonioSemRendaVariavel">Patrimônio sem renda variável</app-termo></span>
          <strong [class.negativo]="d.patrimonioSemRendaVariavel < 0">{{ d.patrimonioSemRendaVariavel | currency }}</strong>
          <small>sem FIIs, ações e ETFs ({{ d.rendaVariavel | currency }})</small>
        </article>
        <article class="kpi">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.banco" [tamanho]="16" /> Saldo em contas</span>
          <strong [class.negativo]="d.saldoContas < 0">{{ d.saldoContas | currency }}</strong>
        </article>
        <article class="kpi">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.contas" [tamanho]="16" />
            <app-termo chave="dividaCartoes">Dívida total nos cartões</app-termo></span>
          <strong class="negativo">{{ d.dividaCartoes | currency }}</strong>
          <small>todos os cartões, com parcelas futuras</small>
        </article>
        <!-- #322: o total das faturas em aberto dos cartões escolhidos em Contas e cartões (posição de hoje). -->
        @if (faturasAbertas().length) { <app-kpi-faturas-abertas [itens]="faturasAbertas()" /> }
        @if (d.dividaFinanciamentos > 0) {
          <article class="kpi kpi-credito kpi-financiamento">
            <span class="kpi-rotulo"><app-icone [icone]="Icones.financiamento" [tamanho]="16" /> Financiamentos</span>
            <strong class="negativo">{{ d.dividaFinanciamentos | currency }}</strong>
            <small>saldo devedor · <a routerLink="/emprestimos">ver contratos</a></small>
          </article>
        }
        @if (d.dividaEmprestimos > 0) {
          <article class="kpi kpi-credito kpi-emprestimo">
            <span class="kpi-rotulo"><app-icone [icone]="Icones.emprestimos" [tamanho]="16" /> Empréstimos</span>
            <strong class="negativo">{{ d.dividaEmprestimos | currency }}</strong>
            <small>saldo devedor · <a routerLink="/emprestimos">ver contratos</a></small>
          </article>
        }
        <article class="kpi">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.investimentos" [tamanho]="16" /> Investimentos</span>
          <strong>{{ d.totalInvestido | currency }}</strong>
          <small><a routerLink="/investimentos">ver carteira x Selic</a></small>
        </article>
      </section>

        <section class="painel-bloco">
          <div class="bloco-titulo"><h2>Próximos vencimentos</h2><a routerLink="/contas-a-pagar">ver todos</a></div>
          <ul class="lista-simples">
            @for (c of d.proximasContas; track c.id) {
              <li>
                <app-selo-servico [texto]="c.textoCobranca" [nome]="c.descricao" [categoria]="c.categoriaNome" [tamanho]="22" />
                <span class="cresce">{{ c.descricao }}</span>
                <span [class.negativo]="c.diasParaVencer < 0" class="texto-suave">
                  {{ c.diasParaVencer < 0 ? 'vencida' : c.diasParaVencer === 0 ? 'hoje' : 'em ' + c.diasParaVencer + 'd' }}
                </span>
                <span class="valor">{{ c.valor | currency }}</span>
              </li>
            } @empty {
              <li class="vazio">Nada vencendo nos próximos {{ d.diasProximosVencimentos }} dias.</li>
            }
          </ul>
        </section>
      </section>

      <section class="secao-painel" aria-labelledby="titulo-mes">
        <header class="secao-titulo">
          <h2 id="titulo-mes">Mês de {{ nomeMes() }}</h2>
          <app-seletor-mes [valor]="periodo()" (mudou)="mudarPeriodo($event)" />
        </header>
      <section class="kpis kpis-mes">
        <article class="kpi">
          <span>Receitas do mês</span>
          <strong class="positivo">{{ d.receitasMes | currency }}</strong>
        </article>
        <article class="kpi">
          <span>Despesas do mês</span>
          <strong class="negativo">{{ d.despesasMes | currency }}</strong>
          @if (d.despesasEmergenciaMes > 0) {
            <small class="texto-emergencia">{{ d.despesasEmergenciaMes | currency }} em emergências</small>
          }
        </article>
        <article class="kpi">
          <span>Resultado</span>
          <strong [class.positivo]="d.receitasMes - d.despesasMes >= 0" [class.negativo]="d.receitasMes - d.despesasMes < 0">
            {{ d.receitasMes - d.despesasMes | currency }}
          </strong>
          @if (d.receitasMes > 0) {
            <small>{{ (d.receitasMes - d.despesasMes) / d.receitasMes * 100 | number: '1.0-0' }}% da renda poupada</small>
          }
        </article>
        <article class="kpi kpi-faturas">
          <span class="kpi-rotulo"><app-icone [icone]="Icones.contas" [tamanho]="16" /> Faturas do mês</span>
          <strong [class.negativo]="d.totalFaturasDoMes > 0">{{ d.totalFaturasDoMes | currency }}</strong>
          <small>a fatura de cada cartão que fecha no mês</small>
          @if (d.faturasDoMes?.length) {
            <ul class="lista-faturas">
              @for (f of d.faturasDoMes; track f.contaId) {
                <li>
                  <span class="cartao-fatura"><app-logo-banco [codigo]="f.banco" [tamanho]="20" [cartao]="true" />
                    <a routerLink="/contas" [queryParams]="{ faturas: f.contaId }" [attr.aria-label]="'Ver faturas de ' + f.cartao">{{ f.cartao }}</a></span>
                  <span class="valor">{{ f.valorMes | currency }}</span>
                  @if (f.parcelasMes > 0) { <span class="texto-suave detalhe-fatura">{{ f.parcelasMes | currency }} em parcelas</span> }
                  @if (f.vencimento && f.situacao) {
                    <span class="texto-suave detalhe-fatura">@if (f.fechamento) {fecha {{ f.fechamento | date: 'dd/MM' }}, vence} @else {fatura vence}
                      {{ f.vencimento | date: 'dd/MM' }}
                      <span class="etiqueta" [class.etiqueta-sucesso]="f.situacao === 'paga'" [class.etiqueta-alerta]="f.situacao === 'fechada'">{{ rotuloSituacao(f.situacao) }}</span></span>
                  }
                </li>
              }
            </ul>
          } @else {
            <small>Nada lançado nos cartões neste mês.</small>
          }
        </article>
      </section>

      <div class="grade-2">
        <section class="painel-bloco">
          <div class="bloco-titulo"><h2>Gastos por categoria</h2></div>
          @if (d.gastosPorCategoria.length) {
            <app-grafico-explicado [resumo]="resumoCategorias()"
              comoLer="Cada fatia é uma categoria, do tamanho da parte dela nas despesas do mês. Clique numa fatia ou no nome da categoria para ver as transações.">
              <app-grafico [configuracao]="graficoCategorias()" [altura]="240" dicaClique="Clique para ver as transações"
                (itemClicado)="abrirCategoria($event)" />
            </app-grafico-explicado>
            <ul class="lista-legenda">
              @for (g of d.gastosPorCategoria.slice(0, 8); track g.nome) {
                <li>
                  <span class="ponto" [style.background]="g.cor"></span>
                  <a class="cresce link-categoria" routerLink="/transacoes" [queryParams]="linkCategoria(g)"
                    [attr.aria-label]="'Ver transações de ' + g.nome">{{ g.nome }}</a>
                  <span class="valor">{{ g.valor | currency }}</span>
                  <span class="texto-suave pct">{{ g.valor / d.despesasMes * 100 | number: '1.0-0' }}%</span>
                </li>
              }
            </ul>
          } @else {
            <p class="vazio">Nenhuma despesa neste mês.</p>
          }
        </section>
        <section class="painel-bloco">
          <div class="bloco-titulo"><h2>Orçamentos</h2><a routerLink="/orcamentos">ver todos</a></div>
          @for (o of d.orcamentos; track o.id) {
            <div class="barra-orcamento">
              <div class="linha-flex">
                <span>{{ o.categoriaNome }}</span>
                <span class="texto-suave">{{ o.gasto | currency }} de {{ o.valor | currency }}</span>
              </div>
              <div class="trilho"><div class="progresso" [class.estourado]="o.situacao === 'estourado'"
                [class.emergencia]="o.situacao === 'emergencia'"
                [class.atencao]="o.percentual >= 80 && o.percentual <= 100"
                [style.width.%]="o.percentual > 100 ? 100 : o.percentual"></div></div>
            </div>
          } @empty {
            <p class="vazio">Nenhum orçamento para este mês. <a routerLink="/orcamentos">Criar</a></p>
          }
        </section>
      </div>

      </section>

      <app-mais-analises chave="painel" conteudo="receitas × despesas em 12 meses e últimas transações">
        <ng-template>
          <div class="grade-2">
            <section class="painel-bloco">
              <div class="bloco-titulo"><h2>Receitas × despesas (12 meses)</h2></div>
              <app-grafico-explicado [resumo]="resumoEvolucao()"
                comoLer="Verde: receitas; rosa: despesas. Quando a barra rosa passa da verde, o mês fechou no vermelho.">
                <app-grafico [configuracao]="graficoEvolucao()" [altura]="300" />
              </app-grafico-explicado>
            </section>
            <section class="painel-bloco">
              <div class="bloco-titulo"><h2>Últimas transações</h2><a routerLink="/transacoes">ver todas</a></div>
              <ul class="lista-simples">
                @for (t of d.ultimasTransacoes; track t.id) {
                  <li>
                    <span class="data-curta">{{ t.data | date: 'dd/MM' }}</span>
                    <app-selo-servico [texto]="t.descricaoOriginal" [estabelecimento]="t.estabelecimento" [nome]="t.descricao"
                      [categoria]="t.categoriaNome" [tamanho]="22" />
                    <span class="cresce truncar" [title]="t.descricao">{{ t.descricao }}</span>
                    <span class="valor" [class.positivo]="t.valor > 0" [class.negativo]="t.valor < 0">{{ t.valor | currency }}</span>
                  </li>
                } @empty {
                  <li class="vazio">Sem transações ainda.</li>
                }
              </ul>
            </section>
          </div>
        </ng-template>
      </app-mais-analises>
    } @else if (!erro()) {
      <p class="texto-suave">Carregando…</p>
    }
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [`
    .kpi-credito { position: relative; overflow: hidden; }
    .kpi-credito::before { content: ""; position: absolute; inset: 0 auto 0 0; width: 5px; }
    .kpi-financiamento::before { background: #8fa8f0; }
    .kpi-emprestimo::before { background: #d9b45a; }
    .secao-painel { margin-bottom: 8px; }
    /* As divisórias do Painel ficam no styles.css (seção "Painel"), onde os testes conseguem conferi-las. */
    .secao-titulo { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 6px 16px; margin: 6px 0 12px; }
    .secao-titulo h2 { margin: 0; font-size: 1.15rem; }
    /* #246: como o seletor de mês em "Mês de …", o texto vai para a direita (sem crescer); quebrando, fica abaixo do título. */
    .secao-titulo p { margin: 0; text-align: right; }
    .lista-faturas { list-style: none; margin: 8px 0 2px; padding: 8px 0 0; display: grid; gap: 8px; font-size: 0.84rem; }
    .lista-faturas li { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; }
    .lista-faturas .valor { margin-left: auto; }
    .lista-faturas .cartao-fatura { display: inline-flex; align-items: center; gap: 6px; }
    .lista-faturas .cartao-fatura a { display: inline-flex; align-items: center; min-height: 24px; }
    .lista-faturas .detalhe-fatura { flex-basis: 100%; font-size: 0.8rem; }
  `]
})
export class PainelComponent implements OnInit {
  readonly Icones = Icones;
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  /** #173 b): um clique em "Desfazer" ou "Ok" por vez. */
  readonly mexendoTrocos = signal(false);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly periodo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  /** UX-12: "Situação hoje" é sempre a posição de hoje; "Mês de …" segue o seletor. */
  readonly hoje = new Date();
  readonly nomeMes = computed(() => `${NOMES_MESES[this.periodo().mes - 1]}/${this.periodo().ano}`);
  /** #322: cartões escolhidos em Contas e cartões (neste navegador) e as contas, carregadas só se houver algum. */
  private readonly cartoesNoPainel = lerCartoesNoPainel();
  private readonly contas = signal<Conta[]>([]);
  /** #322: "Situação hoje" tem 5 cards fixos, mais financiamentos, empréstimos e a fatura em aberto, quando há. */
  colunasHoje(d: Painel): number {
    return colunasEquilibradas(5 + (d.dividaFinanciamentos > 0 ? 1 : 0) + (d.dividaEmprestimos > 0 ? 1 : 0)
      + (this.faturasAbertas().length ? 1 : 0));
  }

  readonly faturasAbertas = computed((): FaturaAbertaDoCartao[] => faturasAbertasDe(this.contas(), new Set(this.cartoesNoPainel)));

  rotuloSituacao(situacao: 'paga' | 'fechada' | 'aberta'): string {
    return situacao === 'paga' ? 'paga' : situacao === 'fechada' ? 'fechada' : 'em aberto';
  }
  readonly dados = signal<Painel | null>(null);
  readonly erro = signal<string | null>(null);

  /** UX-3: despesas da categoria no mês selecionado no Painel ("Sem categoria" vai como categoria=sem). */
  /** #9: Transações filtrada pelas descrições pouco informativas do mesmo período do aviso (até hoje). */
  linkPoucoInformativas(d: Painel): Record<string, string> {
    return { generica: '1', ...(d.poucoInformativasDesde ? { inicio: d.poucoInformativasDesde, fim: dataIso() } : {}) };
  }

  linkCategoria(g: ValorCategoria): Record<string, string> {
    const { ano, mes } = this.periodo();
    return paramsTransacoes({ categoriaId: g.categoriaId, inicio: primeiroDiaMes(ano, mes), fim: ultimoDiaMes(ano, mes), tipo: 'despesa' });
  }

  /** UX-4: clique na fatia da rosca (mesmos itens do gráfico, na mesma ordem). A legenda é o caminho por teclado/toque. */
  abrirCategoria(indice: number): void {
    const item = (this.dados()?.gastosPorCategoria ?? []).slice(0, 10)[indice];
    if (item) void this.router.navigate(['/transacoes'], { queryParams: this.linkCategoria(item) });
  }

  readonly graficoCategorias = computed<ChartConfiguration<'doughnut'>>(() => {
    const itens = (this.dados()?.gastosPorCategoria ?? []).slice(0, 10);
    return {
      type: 'doughnut',
      data: {
        labels: itens.map((i) => i.nome),
        datasets: [{ data: itens.map((i) => i.valor), backgroundColor: itens.map((i) => i.cor), borderWidth: 0 }]
      },
      options: { cutout: '72%', plugins: { legend: { display: false } } }
    };
  });

  readonly graficoEvolucao = computed<ChartConfiguration<'bar'>>(() => {
    const meses = this.dados()?.evolucao ?? [];
    return {
      type: 'bar',
      data: {
        labels: meses.map((m) => `${NOMES_MESES_CURTOS[m.mes - 1]}/${String(m.ano).slice(2)}`),
        datasets: [
          { label: 'Receitas', data: meses.map((m) => m.receitas), backgroundColor: CORES_GRAFICO.receita, borderRadius: 8, borderSkipped: false },
          { label: 'Despesas', data: meses.map((m) => m.despesas), backgroundColor: CORES_GRAFICO.despesa, borderRadius: 8, borderSkipped: false }
        ]
      },
      options: {
        plugins: { legend: { position: 'bottom' } },
        scales: { y: { ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { notation: 'compact' }) } } }
      }
    };
  });

  /** UX-9: maior categoria do mês e o peso dela (percentuais: vale com "ocultar valores"). */
  readonly resumoCategorias = computed(() => {
    const d = this.dados();
    const itens = d?.gastosPorCategoria ?? [];
    if (!d || !itens.length || d.despesasMes <= 0) return null;
    const [primeira, segunda] = itens;
    const texto = `${primeira.nome} é o maior gasto do mês: ${pct(primeira.valor / d.despesasMes * 100)} das despesas`;
    return segunda ? `${texto}, seguido de ${segunda.nome} (${pct(segunda.valor / d.despesasMes * 100)}).` : `${texto}.`;
  });

  /** UX-9: em quantos meses (com movimento) as despesas ficaram abaixo das receitas. */
  readonly resumoEvolucao = computed(() => {
    const meses = (this.dados()?.evolucao ?? []).filter((m) => m.receitas > 0 || m.despesas > 0);
    if (!meses.length) return null;
    const positivos = meses.filter((m) => m.receitas >= m.despesas).length;
    if (positivos === meses.length) return `Você gastou menos do que recebeu em todos os ${meses.length} meses.`;
    if (positivos === 0) return `As despesas passaram as receitas em todos os ${meses.length} meses.`;
    return `Você gastou menos do que recebeu em ${positivos} dos ${meses.length} meses.`;
  });

  /** #173 b): /transacoes com as datas e só os trocos classificados na sincronização. */
  paramsTrocos(t: TrocosAutomaticos): Record<string, string> {
    return paramsTransacoes({ inicio: t.desde, fim: t.ate, ids: t.ids, origemIds: 'trocos-automaticos' });
  }

  desfazerTrocos(): void {
    this.mexendoTrocos.set(true);
    this.api.desfazerTrocosAutomaticos().subscribe({
      next: (r) => {
        this.mexendoTrocos.set(false);
        this.avisos.sucesso(`Desfeito: ${r.transacoesAtualizadas} crédito(s) voltaram para a categoria de antes, e a sincronização não os muda de novo.`);
        this.carregar();
      },
      error: (e) => { this.mexendoTrocos.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  dispensarTrocos(): void {
    this.mexendoTrocos.set(true);
    this.api.dispensarTrocosAutomaticos().subscribe({
      next: () => { this.mexendoTrocos.set(false); this.carregar(); },
      error: (e) => { this.mexendoTrocos.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  ngOnInit(): void {
    const daUrl = mesAnoDaUrl(this.rota.snapshot.queryParamMap);
    if (daUrl) this.periodo.set(daUrl);
    this.carregar();
    // #322: as contas trazem a fatura de hoje de cada cartão. Complementar: se falhar, o Painel fica sem esses cards.
    if (this.cartoesNoPainel.length) {
      this.api.contas().subscribe({ next: (c) => this.contas.set(c), error: () => this.contas.set([]) });
    }
  }

  mudarPeriodo(p: MesAno): void {
    this.periodo.set(p);
    // Mês na URL (replaceUrl, sem poluir o histórico): ao voltar de Transações, o Painel reabre no mesmo mês.
    void this.router.navigate([], { relativeTo: this.rota, queryParams: { ano: p.ano, mes: p.mes }, replaceUrl: true });
    this.carregar();
  }

  private carregar(): void {
    const { ano, mes } = this.periodo();
    this.erro.set(null);
    this.api.painel(ano, mes).subscribe({
      next: (d) => this.dados.set(d),
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }
}
