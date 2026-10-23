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
import { SeloServicoComponent } from '../compartilhado/selo-servico.component';
import { IconeCategoriaComponent } from '../compartilhado/icone-categoria.component';
import { fatiasRosca } from '../compartilhado/rosca-categorias';

@Component({
  selector: 'app-painel',
  imports: [IconeCategoriaComponent, KpiFaturasAbertasComponent, MoedaPipe, DatePipe, DecimalPipe, RouterLink, GraficoComponent, GraficoExplicadoComponent, SeletorMesComponent, IconeComponent, TermoComponent,
    MaisAnalisesComponent, LogoBancoComponent, SeloServicoComponent],
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
      <!-- #327: os avisos e os orçamentos estourados ficam nas notificações da barra do topo (o sino). -->
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
                    <a routerLink="/contas" [queryParams]="linkFaturas(f.contaId)" [attr.aria-label]="'Ver faturas de ' + f.cartao">{{ f.cartao }}</a></span>
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
              comoLer="Cada fatia é uma categoria, do tamanho da parte dela nas despesas do mês; as menores que 3% ficam juntas em “Outras”. Clique numa fatia ou no nome da categoria para ver as transações.">
              <!-- #336: o total do mês no centro da rosca (decorativo: o mesmo número está no topo do Painel). -->
              <div class="rosca">
                <app-grafico [configuracao]="graficoCategorias()" [altura]="240" dicaClique="Clique para ver as transações"
                  (itemClicado)="abrirCategoria($event)" />
                <div class="rosca-centro" aria-hidden="true">
                  <strong>{{ d.despesasMes | currency: 'BRL' : 'symbol' : '1.0-0' }}</strong><span>em despesas</span>
                </div>
              </div>
            </app-grafico-explicado>
            <!-- #336: cada categoria com o ícone na cor dela, o nome com a barrinha proporcional embaixo, o valor e o %. -->
            <ul class="lista-legenda lista-categorias">
              @for (g of categoriasNaLista(); track g.nome) {
                <li>
                  <app-icone-categoria [nome]="g.nome" [cor]="g.cor" />
                  <span class="cresce nome-e-barra">
                    <a class="link-categoria" routerLink="/transacoes" [queryParams]="linkCategoria(g)"
                      [attr.aria-label]="'Ver transações de ' + g.nome">{{ g.nome }}</a>
                    <span class="trilho" aria-hidden="true"><span class="progresso" [style.background]="g.cor"
                      [style.width.%]="g.valor / d.gastosPorCategoria[0].valor * 100"></span></span>
                  </span>
                  <span class="valor">{{ g.valor | currency }}</span>
                  <span class="texto-suave pct">{{ g.valor / d.despesasMes * 100 | number: '1.0-0' }}%</span>
                </li>
              }
            </ul>
            @if (d.gastosPorCategoria.length > CATEGORIAS_NA_LISTA) {
              <button type="button" class="link mais-categorias" [attr.aria-expanded]="todasCategorias()"
                (click)="todasCategorias.set(!todasCategorias())">{{ todasCategorias() ? 'Mostrar menos'
                  : 'Mostrar todas as ' + d.gastosPorCategoria.length + ' categorias' }}</button>
            }
          } @else {
            <p class="vazio">Nenhuma despesa neste mês.</p>
          }
        </section>
        <!-- #336: o bloco ocupa a altura do vizinho ("Gastos por categoria"): as linhas crescem até um limite e o total
             fica no rodapé. Com "Mostrar todas" as categorias, ele acompanha sem as linhas ficarem esparsas. -->
        <section class="painel-bloco bloco-orcamentos">
          <div class="bloco-titulo"><h2>Orçamentos</h2><a routerLink="/orcamentos">ver todos</a></div>
          <div class="lista-orcamentos">
          @for (o of d.orcamentos; track o.id) {
            <div class="barra-orcamento">
              <div class="linha-flex">
                <span class="categoria-com-icone"><app-icone-categoria [nome]="o.categoriaNome" [cor]="o.categoriaCor" [tamanho]="24" />{{ o.categoriaNome }}</span>
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
          </div>
          @if (totalOrcamentos(); as tot) {
            <p class="total-orcamentos">
              <span>Total: <strong>{{ tot.gasto | currency }}</strong> de {{ tot.valor | currency }}</span>
              @if (tot.estourados) {
                <span class="negativo">{{ tot.estourados }} {{ tot.estourados === 1 ? 'passou' : 'passaram' }} do limite</span>
              } @else {
                <span class="positivo">tudo dentro do limite</span>
              }
            </p>
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
              <!-- #334: as últimas vão só até hoje; as futuras (parcelas já lançadas) ficam atrás do botão "Futuras". -->
              <div class="bloco-titulo">
                <h2>{{ verFuturas() ? 'Próximas transações' : 'Últimas transações' }}</h2>
                @if (d.quantidadeFuturas) {
                  <div class="segmentado segmentado-transacoes" role="group" aria-label="Quais transações mostrar">
                    <button type="button" [class.ativo]="!verFuturas()" [attr.aria-pressed]="!verFuturas()"
                      (click)="verFuturas.set(false)">Até hoje</button>
                    <button type="button" [class.ativo]="verFuturas()" [attr.aria-pressed]="verFuturas()"
                      (click)="verFuturas.set(true)">Futuras ({{ d.quantidadeFuturas }})</button>
                  </div>
                }
                <a routerLink="/transacoes" [queryParams]="linkTodas()">ver todas</a>
              </div>
              <ul class="lista-simples">
                @for (t of (verFuturas() ? (d.transacoesFuturas ?? []) : d.ultimasTransacoes); track t.id) {
                  <li>
                    <span class="data-curta">{{ t.data | date: (verFuturas() ? 'dd/MM/yy' : 'dd/MM') }}</span>
                    <app-selo-servico [texto]="t.descricaoOriginal" [estabelecimento]="t.estabelecimento" [nome]="t.descricao" [tipo]="t.tipoLancamento"
                      [categoria]="t.categoriaNome" [tamanho]="22" />
                    <span class="cresce truncar" [title]="t.descricao">{{ t.descricao }}</span>
                    <span class="valor" [class.positivo]="t.valor > 0" [class.negativo]="t.valor < 0">{{ t.valor | currency }}</span>
                  </li>
                } @empty {
                  <li class="vazio">{{ verFuturas() ? 'Nenhuma transação com data futura.' : 'Sem transações ainda.' }}</li>
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
    /* #336: a rosca com o total no centro; a lista com o ícone, o nome e a barrinha embaixo dele. */
    .rosca { position: relative; }
    .rosca-centro {
      position: absolute; inset: 0; display: grid; place-content: center; text-align: center; pointer-events: none;
    }
    .rosca-centro strong { font-family: var(--fonte-titulo); font-size: 1.25rem; color: var(--texto-forte); font-variant-numeric: tabular-nums; }
    .rosca-centro span { font-size: 0.8rem; color: var(--texto-suave); }
    .lista-categorias li { gap: 12px; }
    .nome-e-barra { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
    .nome-e-barra .link-categoria { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .nome-e-barra .trilho { display: block; height: 5px; margin: 0; }
    .nome-e-barra .progresso { display: block; }
    .lista-categorias .valor { min-width: 7.5em; text-align: right; font-variant-numeric: tabular-nums; }
    .mais-categorias { margin-top: 6px; padding-left: 6px; }
    .bloco-orcamentos { display: flex; flex-direction: column; }
    .lista-orcamentos { flex: 1; display: flex; flex-direction: column; }
    .lista-orcamentos .barra-orcamento {
      flex: 1 1 auto; max-height: 120px; display: flex; flex-direction: column; justify-content: center;
    }
    .lista-orcamentos .barra-orcamento + .barra-orcamento { border-top: 1px solid var(--linha); }
    .total-orcamentos {
      display: flex; flex-wrap: wrap; justify-content: space-between; gap: 4px 12px; margin: 12px 0 0; padding-top: 12px;
      border-top: 1px solid var(--linha); font-size: 0.9rem;
    }
    /* #334: "Até hoje" e "Futuras" no cabeçalho de "Últimas transações"; no celular, numa linha própria, na largura toda. */
    .segmentado-transacoes { flex-shrink: 0; white-space: nowrap; margin-left: auto; }
    @media (max-width: 760px) {
      .bloco-titulo:has(.segmentado-transacoes) { flex-wrap: wrap; }
      .segmentado-transacoes { order: 3; flex-basis: 100%; margin-left: 0; display: flex; }
      .segmentado-transacoes button { flex: 1; }
    }
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
  /** #334: "Últimas transações" mostra as até hoje ou as futuras (parcelas já lançadas com data depois de hoje). */
  readonly verFuturas = signal(false);

  /** "ver todas" leva a Transações no mesmo recorte: até hoje, ou de amanhã em diante. */
  linkTodas(): Record<string, string> {
    const amanha = new Date(this.hoje.getFullYear(), this.hoje.getMonth(), this.hoje.getDate() + 1);
    return this.verFuturas() ? paramsTransacoes({ inicio: dataIso(amanha) }) : paramsTransacoes({ fim: dataIso(this.hoje) });
  }

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

  linkCategoria(g: ValorCategoria): Record<string, string> {
    const { ano, mes } = this.periodo();
    return paramsTransacoes({ categoriaId: g.categoriaId, inicio: primeiroDiaMes(ano, mes), fim: ultimoDiaMes(ano, mes), tipo: 'despesa' });
  }

  /** #336: o rodapé do bloco de Orçamentos: o total gasto, o total orçado e quantos passaram do limite. */
  readonly totalOrcamentos = computed(() => {
    const orcamentos = this.dados()?.orcamentos ?? [];
    if (!orcamentos.length) return null;
    return {
      gasto: orcamentos.reduce((s, o) => s + o.gasto, 0),
      valor: orcamentos.reduce((s, o) => s + o.valor, 0),
      estourados: orcamentos.filter((o) => o.situacao === 'estourado' || o.situacao === 'emergencia').length
    };
  });

  /** #336: a lista mostra as 8 maiores; "Mostrar todas" abre o resto (Progressive Disclosure). */
  readonly CATEGORIAS_NA_LISTA = 8;
  readonly todasCategorias = signal(false);
  readonly categoriasNaLista = computed(() => {
    const itens = this.dados()?.gastosPorCategoria ?? [];
    return this.todasCategorias() ? itens : itens.slice(0, this.CATEGORIAS_NA_LISTA);
  });
  /** #336: as fatias da rosca, com as menores que 3% juntas em "Outras". */
  private readonly fatiasCategorias = computed(() => fatiasRosca(this.dados()?.gastosPorCategoria ?? []));

  /** UX-4: clique na fatia da rosca. A legenda é o caminho por teclado/toque. "Outras" abre as despesas do mês. */
  abrirCategoria(indice: number): void {
    const posicao = this.fatiasCategorias().indices[indice];
    const item = posicao === undefined || posicao < 0 ? null : (this.dados()?.gastosPorCategoria ?? [])[posicao];
    if (item) {
      void this.router.navigate(['/transacoes'], { queryParams: this.linkCategoria(item) });
    } else if (posicao === -1) {
      const { ano, mes } = this.periodo();
      void this.router.navigate(['/transacoes'], { queryParams: paramsTransacoes({ inicio: primeiroDiaMes(ano, mes), fim: ultimoDiaMes(ano, mes), tipo: 'despesa' }) });
    }
  }

  readonly graficoCategorias = computed<ChartConfiguration<'doughnut'>>(() => {
    const f = this.fatiasCategorias();
    return {
      type: 'doughnut',
      data: {
        labels: f.rotulos,
        // #336: um respiro entre as fatias e as pontas arredondadas.
        datasets: [{ data: f.valores, backgroundColor: f.cores, borderWidth: 0, spacing: 3, borderRadius: 4 }]
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

  /** #323: o histórico do cartão em Contas, no mesmo mês do Painel (no mês atual, sem ano e mês na URL). */
  linkFaturas(contaId: number): Record<string, number> {
    const { ano, mes } = this.periodo();
    const atual = ano === this.hoje.getFullYear() && mes === this.hoje.getMonth() + 1;
    return atual ? { faturas: contaId } : { faturas: contaId, ano, mes };
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
