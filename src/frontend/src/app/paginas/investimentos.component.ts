import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent, SIMBOLOS_MOEDA } from '../compartilhado/campo-valor.component';
import { CampoDataComponent } from '../compartilhado/campo-data.component';
import { ChartConfiguration } from 'chart.js';
import { ActivatedRoute } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  AtualizarAtivo, Carteira, InvestimentoComparado, MoedaCambio, NOMES_ORIGEM_OPERACAO, NOMES_TIPO_ATIVO, OperacaoAtivo, OrigemOperacao, PosicaoAtivo,
  Reserva, SalvarOperacao, SugestaoExtrato, TipoAtivo, TipoOperacao
} from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { dataIso, pct, reaisOuNada } from '../compartilhado/utilidades';
import { CORES_GRAFICO } from '../compartilhado/cores-graficos';
import { GraficoExplicadoComponent } from '../compartilhado/grafico-explicado.component';
import { TermoComponent } from '../compartilhado/termo.component';
import { MaisAnalisesComponent } from '../compartilhado/mais-analises.component';
import { InvestimentosFiisComponent } from './investimentos-fiis.component';
import { InvestimentosCambioComponent } from './investimentos-cambio.component';
import { InvestimentosCriptoComponent } from './investimentos-cripto.component';
import { ConferirInvestimentosComponent } from './conferir-investimentos.component';
import { RendimentoInvestimentosOfComponent } from './rendimento-investimentos-of.component';
import { conferenciaForaDoEsperado, podeConferirReserva, valorEsperadoDaReserva } from './reserva-conferencia';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';

const NOMES_TIPO_PLUGGY: Record<string, string> = {
  FIXED_INCOME: 'Renda fixa',
  MUTUAL_FUND: 'Fundos',
  SECURITY: 'Previdência',
  EQUITY: 'Ações',
  ETF: 'ETFs',
  COE: 'COE',
  OTHER: 'Outros'
};
// Paleta dos tipos de investimento: sem o verde/rosa de ganhou/perdeu nem o laranja da Selic (UX-9).
const CORES = ['#a7a3f5', '#92c8f0', '#c3b1e1', '#8fd8d8', '#f3b3d3', '#f2d98b', '#a9b4c6', '#b8c7f0'];
const COR_CARTEIRA = '#a7a3f5';
const COR_SELIC = CORES_GRAFICO.selic;
const COR_MERCADO = '#8fd5b4';
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
/** Símbolo curto para rótulos de campo (o pipe de moeda já formata os valores). */
const compacto = (v: string | number) => Number(v).toLocaleString('pt-BR', { notation: 'compact' });

type Aba = 'resumo' | 'fii' | 'rv' | 'of' | 'reservas';

/** Formulário de conferência do saldo real de uma reserva. */
/** #169: `moeda` só conta no Méli Dólar (o saldo pode ser informado em reais ou em dólares). */
interface FormConferencia { reserva: Reserva; saldo: number | null; data: string; moeda: 'BRL' | 'USD' }

/** Linha editável da importação pelo extrato. */
interface LinhaImportacao extends SugestaoExtrato { selecionada: boolean; taxas: number }

@Component({
  selector: 'app-investimentos',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, DatePipe, DecimalPipe, GraficoComponent, GraficoExplicadoComponent, ModalComponent, TermoComponent, MaisAnalisesComponent,
    InvestimentosFiisComponent, ConferirInvestimentosComponent, RendimentoInvestimentosOfComponent, CampoValorComponent, CampoDataComponent, InvestimentosCambioComponent, InvestimentosCriptoComponent],
  template: `
    <header class="cabecalho-pagina">
      <div>
        <h1>Investimentos</h1>
        <span class="texto-suave">
          Renda variável lançada por você + Open Finance, comparados com a Selic
          @if (dados()?.cotacoesAtualizadasEm; as em) { · cotações de {{ em | date: 'dd/MM HH:mm' }} }
        </span>
      </div>
      <div class="acoes">
        <button type="button" class="botao" (click)="conferenciaAberta.set(true)">Conferir investimentos</button>
        <button type="button" class="botao" (click)="abrirImportacao()">⇣ Importar do extrato</button>
        <button type="button" class="botao" (click)="atualizarCotacoes()" [disabled]="atualizando()">
          {{ atualizando() ? 'Atualizando…' : '↻ Atualizar cotações' }}
        </button>
        <button type="button" class="botao botao-primario" (click)="novaOperacao()">+ Lançar compra/venda</button>
      </div>
    </header>

    @if (dados(); as d) {
      @if (d.cotasDiferentes.length > 0) {
        <section class="alertas" role="status">
          <p>A corretora informa outra quantidade de cotas de {{ listaTickers(d.cotasDiferentes) }}.
            <button type="button" class="link" (click)="conferenciaAberta.set(true)">Conferir investimentos</button></p>
        </section>
      }
      @if (d.moedasSemCotacao.length > 0) {
        <section class="alertas">
          <p>Investimentos em {{ d.moedasSemCotacao.join(', ') }} ficaram fora dos totais: não há cotação de câmbio
            guardada para essa moeda.</p>
        </section>
      }
      <section class="kpis">
        <article class="kpi kpi-destaque">
          <span>Total investido</span>
          <strong>{{ d.totalGeral | currency }}</strong>
          <small>renda variável + Open Finance + contas de investimento + reservas</small>
        </article>
        <article class="kpi">
          <span>Renda variável (cotação atual)</span>
          <strong>{{ d.rendaVariavel.valorMercado | currency }}</strong>
          <small>custo {{ d.rendaVariavel.custo | currency }} ·
            <span [class.positivo]="d.rendaVariavel.valorizacao >= 0" [class.negativo]="d.rendaVariavel.valorizacao < 0">
              {{ pct(d.rendaVariavel.valorizacaoPercentual) }}</span></small>
        </article>
        <article class="kpi">
          <span>Carteira x Selic</span>
          <strong [class.positivo]="d.rendaVariavel.diferencaSelic >= 0" [class.negativo]="d.rendaVariavel.diferencaSelic < 0">
            {{ d.rendaVariavel.diferencaSelic >= 0 ? '+' : '' }}{{ d.rendaVariavel.diferencaSelic | currency }}</strong>
          <small>carteira {{ pct(d.rendaVariavel.resultadoTotalPercentual) }} · Selic {{ pct(d.rendaVariavel.resultadoSelicPercentual) }}
            no mesmo período</small>
        </article>
        <article class="kpi">
          <span>Proventos em 12 meses</span>
          <strong class="positivo">{{ d.rendaVariavel.proventos12Meses | currency }}</strong>
          <small>{{ d.rendaVariavel.proventosRecebidos | currency }} desde a 1ª compra</small>
        </article>
        <article class="kpi">
          <span><app-termo chave="selic">Selic</app-termo></span>
          <strong>{{ d.selic.metaAnual !== null ? (d.selic.metaAnual | number: '1.2-2') + '% a.a.' : '—' }}</strong>
          <small>12 meses: {{ pct(d.selic.acumulado12Meses, false) }} · no ano: {{ pct(d.selic.acumuladoAno, false) }}</small>
        </article>
      </section>

      <div class="segmentado barra-abas" role="tablist">
        <button type="button" role="tab" [class.ativo]="aba() === 'resumo'" (click)="aba.set('resumo')">Resumo</button>
        <button type="button" role="tab" [class.ativo]="aba() === 'fii'" (click)="aba.set('fii')">
          FIIs ({{ quantidadeFiis() }})</button>
        <button type="button" role="tab" [class.ativo]="aba() === 'rv'" (click)="aba.set('rv')">
          Renda variável ({{ abertas().length }})</button>
        <button type="button" role="tab" [class.ativo]="aba() === 'of'" (click)="aba.set('of')">
          Open Finance ({{ d.openFinance.length }})</button>
        @if (d.reservas.length) {
          <button type="button" role="tab" [class.ativo]="aba() === 'reservas'" (click)="aba.set('reservas')">
            Reservas ({{ d.reservas.length }})
            @if (reservasAConferir() > 0) { <span class="etiqueta etiqueta-alerta" title="Reservas para conferir">!</span> }
          </button>
        }
      </div>

      @switch (aba()) {
        @case ('resumo') {
          <section class="painel-bloco">
            <h2>Seu dinheiro na renda variável x na Selic</h2>
            @if (d.evolucao.length) {
              <app-grafico-explicado [resumo]="resumoEvolucao()"
                comoLer="Área lilás: o que você aplicou (compras menos vendas). Tracejado: quanto esse mesmo dinheiro teria hoje na Selic. Ponto verde: valor de mercado atual (proventos não entram neste gráfico).">
                <app-grafico [configuracao]="graficoEvolucao()" [altura]="270" />
              </app-grafico-explicado>
            } @else { <p class="vazio">Lance suas compras para ver a comparação.</p> }
          </section>

          <!-- #247: dólar, euro e iene em reais (PTAX), compacto, de apoio à carteira. -->
          <app-investimentos-cambio />

          <!-- #279 (ADR 0048): criptomoedas em reais (Mercado Bitcoin), logo abaixo do câmbio. -->
          <app-investimentos-cripto />

          <app-mais-analises chave="investimentos" conteudo="distribuição por tipo e rentabilidade de cada ativo x Selic">
            <ng-template>
              <div class="grade-2">
                <section class="painel-bloco">
                  <h2>Distribuição</h2>
                  @if (d.distribuicao.length) {
                    <app-grafico-explicado [resumo]="resumoDistribuicao()"
                      comoLer="Cada fatia é um tipo de investimento, do tamanho do valor dele hoje. A lista abaixo traz os valores.">
                      <app-grafico [configuracao]="graficoDistribuicao()" [altura]="200" />
                    </app-grafico-explicado>
                    <ul class="lista-legenda">
                      @for (g of d.distribuicao; track g.nome; let i = $index) {
                        <li>
                          <span class="ponto" [style.background]="cor(i)"></span>
                          <span class="cresce">{{ g.nome }}</span>
                          <span class="valor">{{ g.valor | currency }}</span>
                          <span class="texto-suave pct">{{ d.totalGeral > 0 ? (g.valor / d.totalGeral * 100 | number: '1.0-0') : 0 }}%</span>
                        </li>
                      }
                    </ul>
                  } @else { <p class="vazio">Nenhum investimento ainda.</p> }
                </section>
                @if (abertas().length) {
                  <section class="painel-bloco">
                    <h2>Rentabilidade por ativo x Selic no mesmo período</h2>
                    <app-grafico-explicado [resumo]="resumoAtivos()"
                      comoLer="Para cada ativo: resultado total (valorização, vendas e proventos) em verde quando ganhou da Selic e em rosa quando perdeu; em laranja-claro, a Selic com as mesmas datas e valores das suas compras.">
                      <app-grafico [configuracao]="graficoAtivos()" [altura]="alturaGraficoAtivos()" />
                    </app-grafico-explicado>
                  </section>
                }
              </div>
            </ng-template>
          </app-mais-analises>
        }

        @case ('fii') {
          <app-investimentos-fiis [versao]="versaoCarteira()" (alterado)="carregar()" />
        }

        @case ('rv') {
          <section class="painel-bloco">
            <div class="cabecalho-bloco">
              <h2>Posições</h2>
              <span class="cresce"></span>
              <!-- #130: efeito imediato → interruptor. -->
              <label class="interruptor"><input type="checkbox" role="switch" [checked]="mostrarEncerradas()"
                (change)="mostrarEncerradas.set(!mostrarEncerradas())"> Mostrar encerradas</label>
              <button type="button" class="link" (click)="abrirConfiguracao()">fonte das cotações</button>
            </div>
            <div class="tabela-rolagem">
              <table class="tabela tabela-carteira">
                <thead>
                  <tr>
                    <th>Ativo</th><th class="direita">Cotas</th><th class="direita"><app-termo chave="precoMedio">Preço médio</app-termo></th>
                    <th class="direita">Cotação</th><th class="direita">Valor atual</th><th class="direita">Valorização</th>
                    <th class="direita">Proventos 12m</th><th class="direita">Resultado total</th><th class="direita">x Selic</th><th></th>
                  </tr>
                </thead>
                <tbody>
                  @for (p of posicoesVisiveis(); track p.id) {
                    <tr [class.encerrada]="p.quantidade === 0">
                      <td>
                        <strong>{{ p.ticker }}</strong> <span class="etiqueta etiqueta-suave">{{ nomeTipo(p.tipo) }}</span>
                        @if (p.moeda !== 'BRL') { <span class="etiqueta etiqueta-suave" title="Ativo no exterior">{{ p.moeda }}</span> }
                        @if (p.nome) { <br><small class="texto-suave">{{ p.nome }}</small> }
                      </td>
                      <td class="direita">{{ qtd(p.quantidade) }}</td>
                      <td class="direita">{{ p.precoMedio | currency: p.moeda }}</td>
                      <td class="direita">
                        @if (p.precoAtual !== null) {
                          {{ p.precoAtual | currency: p.moeda }}
                          @if (p.moeda !== 'BRL') {
                            <br><small class="texto-suave">{{ p.precoAtualEmReais !== null ? (p.precoAtualEmReais | currency) : 'sem câmbio' }}</small>
                          }
                          @if (p.precoManual) {
                            <br><small class="etiqueta etiqueta-alerta">manual</small>
                          } @else if (p.variacaoDia !== null) {
                            <br><small [class.positivo]="p.variacaoDia >= 0" [class.negativo]="p.variacaoDia < 0">{{ pct(p.variacaoDia) }} hoje</small>
                          }
                        } @else {
                          <span class="texto-suave" title="Sem cotação: usando o preço médio">—</span>
                        }
                      </td>
                      <td class="direita valor">{{ p.valorMercado | currency }}</td>
                      <td class="direita" [class.positivo]="p.valorizacao >= 0" [class.negativo]="p.valorizacao < 0">
                        {{ p.valorizacao | currency }}<br><small>{{ pct(p.valorizacaoPercentual) }}</small>
                      </td>
                      <td class="direita">
                        {{ p.proventos12Meses | currency }}
                        @if (p.dividendYield12Meses !== null) {
                          <br><small class="texto-suave">DY {{ p.dividendYield12Meses | number: '1.1-1' }}%</small>
                        }
                      </td>
                      <td class="direita" [class.positivo]="p.resultadoTotal >= 0" [class.negativo]="p.resultadoTotal < 0">
                        {{ p.resultadoTotal | currency }}<br><small>{{ pct(p.resultadoTotalPercentual) }}</small>
                      </td>
                      <td class="direita" [class.positivo]="p.diferencaSelic >= 0" [class.negativo]="p.diferencaSelic < 0">
                        {{ p.diferencaSelic >= 0 ? '+' : '' }}{{ p.diferencaSelic | currency }}
                        <br><small class="texto-suave">Selic {{ pct(p.resultadoSelicPercentual) }}</small>
                      </td>
                      <td class="direita nowrap">
                        <button type="button" class="link" (click)="abrirOperacoes(p)">operações</button>
                        <button type="button" class="link" (click)="editarAtivo(p)">editar</button>
                      </td>
                    </tr>
                  } @empty {
                    <tr><td colspan="10" class="vazio">
                      Nenhum ativo lançado. Use <strong>+ Lançar compra/venda</strong> com o código da B3 (ex.: MXRF11) e o preço pago,
                      ou escolha outra moeda para um ativo no exterior (ex.: AAPL em USD).
                    </td></tr>
                  }
                </tbody>
              </table>
            </div>
            <p class="texto-suave nota">
              Preço médio pelo método da Receita (taxas entram no custo). Proventos são estimados pela quantidade que você tinha na
              data-com. Ativos no exterior: preço médio e cotação na moeda do ativo; custo, valores e resultados em reais, cada
              operação pelo seu câmbio e o valor atual pelo câmbio de hoje (proventos do exterior aparecem brutos, antes do imposto
              retido lá fora). "x Selic" compara o valor atual com quanto o mesmo dinheiro (com as mesmas vendas e proventos retirados)
              teria hoje na Selic.
            </p>
          </section>
        }

        @case ('of') {
          <div class="barra-importacao">
            <span class="texto-suave">
              @if (d.openFinanceNaoRegistrados > 0) {
                {{ d.openFinanceNaoRegistrados }} FII(s)/ação(ões) ainda não registrados como compras na carteira.
              } @else {
                FIIs, ações e ETFs do Open Finance já estão registrados como compras na carteira.
              }
            </span>
            <span class="cresce"></span>
            <button type="button" class="botao botao-primario" (click)="importarOpenFinance()" [disabled]="importandoOpenFinance()">
              {{ importandoOpenFinance() ? 'Registrando…' : 'Registrar compras na carteira' }}</button>
          </div>
          <div class="tabela-rolagem">
            <table class="tabela">
              <thead>
                <tr><th>Investimento</th><th>Instituição</th><th class="direita">Aplicado</th><th class="direita">Saldo</th>
                  <th class="direita">Rentabilidade</th><th class="direita"><app-termo chave="selic">Selic</app-termo> no período</th><th class="direita">% da Selic</th>
                  <th>Vencimento</th></tr>
              </thead>
              <tbody>
                @for (i of d.openFinance; track i.id) {
                  <tr [class.encerrada]="i.jaNaCarteira">
                    <!-- #170: o nome que se reconhece (Nielsen 2 e 6), com o do banco embaixo para achar no app. -->
                    <td>{{ i.nomeExibicao }} <span class="etiqueta etiqueta-suave">{{ nomeTipoPluggy(i.tipo, i.subtipo, i.codigo) }}</span>
                      @if (i.codigo) { <small class="texto-suave">{{ i.codigo }}</small> }
                      <button type="button" class="link" (click)="abrirRenomear(i)"
                        [attr.aria-label]="'Renomear ' + i.nomeExibicao">renomear</button>
                      @if (i.nomeExibicao !== i.nome) { <br><small class="texto-suave">no banco: {{ i.nome }}</small> }
                      @if (i.taxaAnual) { <br><small class="texto-suave">{{ i.taxaAnual | number: '1.2-2' }}% a.a.</small> }
                      @if (i.jaNaCarteira) { <br><small class="etiqueta etiqueta-alerta">já lançado na carteira — não somado de novo</small> }
                    </td>
                    <td class="texto-suave">{{ i.nomeInstituicao }}</td>
                    <td class="direita">{{ i.valorAplicado !== null ? (i.valorAplicado | currency: i.moeda) : '—' }}
                      @if (i.dataAplicacao) { <br><small class="texto-suave">{{ i.dataAplicacao | date: 'dd/MM/yy' }}</small> }</td>
                    <td class="direita valor">{{ i.saldo | currency: i.moeda }}
                      @if (i.moeda !== 'BRL') {
                        <br><small class="texto-suave">{{ i.saldoEmReais !== null ? (i.saldoEmReais | currency) : 'sem cotação' }}</small>
                      }</td>
                    <td class="direita" [class.positivo]="(i.rentabilidade ?? 0) > 0" [class.negativo]="(i.rentabilidade ?? 0) < 0">
                      {{ pct(i.rentabilidade) }}</td>
                    <td class="direita texto-suave">{{ pct(i.selicNoPeriodo, false) }}</td>
                    <td class="direita">
                      @if (i.percentualDaSelic !== null) {
                        <span class="etiqueta" [class.etiqueta-sucesso]="i.percentualDaSelic >= 100"
                          [class.etiqueta-alerta]="i.percentualDaSelic < 100">{{ i.percentualDaSelic | number: '1.0-0' }}%</span>
                      } @else {
                        <span class="texto-suave">—</span>
                      }
                    </td>
                    <td>{{ i.dataVencimento ? (i.dataVencimento | date: 'dd/MM/yyyy') : '—' }}</td>
                  </tr>
                } @empty {
                  <tr><td colspan="8" class="vazio">Nenhum investimento importado pelo Open Finance.</td></tr>
                }
              </tbody>
            </table>
          </div>
          <p class="texto-suave nota">
            A comparação usa a data e o valor aplicado informados pela instituição. Em produtos com vários aportes (fundos,
            previdência) a instituição pode informar só o total, e o resultado fica aproximado. Valores brutos, antes do IR.
          </p>
          <app-rendimento-investimentos-of />
        }

        @case ('reservas') {
          <div class="tabela-rolagem">
            <table class="tabela">
              <thead>
                <tr><th>Reserva</th><th>Conta</th><th class="direita">Guardado</th><th class="direita">Retirado</th>
                  <th class="direita">Saldo</th><th>Base do saldo</th><th></th></tr>
              </thead>
              <tbody>
                @for (r of d.reservas; track r.contaId + '|' + r.nome) {
                  <tr [class.encerrada]="r.saldo === 0">
                    <td><strong>{{ r.nome }}</strong>
                      <br><small class="texto-suave">{{ r.movimentos }} lançamento(s)
                        @if (r.primeiroMovimento) { de {{ r.primeiroMovimento | date: 'dd/MM/yy' }} a {{ r.ultimoMovimento | date: 'dd/MM/yy' }} }
                      </small>
                      <!-- #169: o Méli Dólar mostra a quantidade em dólar e a PTAX que deu o valor em reais (Nielsen 1). -->
                      @if (r.moeda === 'USD') {
                        <br><small class="texto-suave">{{ r.quantidade | currency: 'USD' }}
                          @if (r.cotacao) { × <app-termo chave="ptax">PTAX</app-termo> {{ r.cotacao | number: '1.4-4' }} }</small>
                      }</td>
                    <td class="texto-suave">{{ r.nomeConta }}</td>
                    <td class="direita">{{ r.guardado | currency }}</td>
                    <td class="direita">{{ r.retirado | currency }}</td>
                    <td class="direita valor">{{ r.saldo | currency }}</td>
                    <td>
                      @if (r.informadoPeloBanco) {
                        <span class="etiqueta etiqueta-sucesso">informado pelo banco</span>
                        <br><small class="texto-suave">valor que a instituição manda pelo Open Finance</small>
                      } @else if (r.restanteDoBanco) {
                        <!-- #237: o banco manda só o total; esta linha é o que ele passa da soma das reservas acima. -->
                        <span class="etiqueta">restante do total do banco</span>
                        <br><small class="texto-suave">a instituição informa {{ r.totalDoBanco | currency }} reservados nesta conta;
                          aqui fica o que não está nas outras reservas. Conferir as outras ajusta este valor.</small>
                      } @else if (r.dataConferencia) {
                        @if (r.conferenciaAntiga) {
                          <span class="etiqueta etiqueta-alerta">conferido há mais de 30 dias</span>
                        } @else {
                          <span class="etiqueta etiqueta-sucesso">conferido</span>
                        }
                        <br><small class="texto-suave">{{ r.saldoConferido | currency: (r.moeda ?? 'BRL') }} em {{ r.dataConferencia | date: 'dd/MM/yyyy' }}
                          + lançamentos depois</small>
                      } @else {
                        <span class="etiqueta etiqueta-alerta">estimado</span>
                        <br><small class="texto-suave">só pelos lançamentos importados</small>
                        @if (r.podeTerSaldoAnterior) {
                          <br><small class="negativo">Pode haver saldo de antes do 1º lançamento importado. Confira.</small>
                        }
                      }
                      @if (r.foraDaListaDoBanco) {
                        <br><small class="texto-suave">não está na lista de reservas que a instituição manda: se ela já aparece
                          com outro nome, confira esta com zero.</small>
                      }
                    </td>
                    <td class="direita nowrap">
                      @if (podeConferir(r)) {
                        <button type="button" class="link" [class.destaque-conferir]="r.podeTerSaldoAnterior || r.conferenciaAntiga"
                          (click)="abrirConferencia(r)">conferir saldo</button>
                      }
                    </td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
          <p class="texto-suave nota">
            Quando a instituição informa, pelo Open Finance, o valor de cada reserva pelo nome, vale esse valor ("informado pelo
            banco"). Quando ela informa só o total reservado da conta, cada reserva continua estimada e conferível, e o que o
            total passa da soma delas aparece como "Reservado". Senão,
            ela só manda os lançamentos "Dinheiro reservado" e "Dinheiro retirado" no extrato da conta, e o saldo é estimado. Sem conferência, ele fica abaixo do real
            quando a reserva já tinha dinheiro antes do primeiro lançamento importado e quando o rendimento ainda não foi
            retirado. Confira o valor no app da instituição de vez em quando: os lançamentos seguintes passam a somar a
            partir dele. O total entra no patrimônio, no total investido e na reserva de emergência.
            O Méli Dólar do Mercado Pago também aparece aqui: a quantidade em dólar vem das compras e vendas do extrato
            (pelo "MUSD" da descrição ou pela PTAX do dia) e o saldo em reais é essa quantidade pela última PTAX.
          </p>
        }
      }
    } @else {
      <p class="vazio">Carregando…</p>
    }

    <!-- Lançar / editar operação -->
    <app-modal [aberto]="formOp !== null" [titulo]="editandoOpId ? 'Editar operação' : 'Lançar compra ou venda'" (fechar)="formOp = null">
      @if (formOp; as fo) {
        <form (ngSubmit)="salvarOperacao()" #f="ngForm" class="formulario">
          <div class="segmentado" role="group" aria-label="Tipo de operação">
            <button type="button" [class.ativo]="fo.tipo === TipoOperacao.Compra" (click)="fo.tipo = TipoOperacao.Compra; aoMudarDataOuTipo()">Compra</button>
            <button type="button" [class.ativo]="fo.tipo === TipoOperacao.Venda" (click)="fo.tipo = TipoOperacao.Venda; aoMudarDataOuTipo()">Venda</button>
          </div>
          <div class="grade-campos">
            @if (editandoOpId === null) {
              <label>Moeda
                <select name="moeda" [(ngModel)]="fo.moeda" (ngModelChange)="aoMudarMoeda()" [disabled]="moedaTravada">
                  <option value="BRL">BRL — Real (B3)</option>
                  @for (m of moedas(); track m.codigo) { <option [value]="m.codigo">{{ m.codigo }} — {{ m.nome }} (exterior)</option> }
                </select>
                @if (moedaTravada) { <small class="texto-suave">A moeda de um ativo já lançado não muda.</small> }
              </label>
              <label>{{ fo.moeda === 'BRL' ? 'Código na B3' : 'Código (como no Yahoo Finance)' }}
                <input name="ticker" [(ngModel)]="fo.ticker" required maxlength="12" autocomplete="off" class="maiusculas"
                  [placeholder]="fo.moeda === 'BRL' ? 'MXRF11' : 'AAPL'">
              </label>
              <label>Tipo do ativo
                <select name="tipoAtivo" [(ngModel)]="fo.tipoAtivo">
                  <option [ngValue]="null">{{ fo.moeda === 'BRL' ? 'Detectar pelo código' : 'Ações (padrão)' }}</option>
                  @for (t of tiposPara(fo.moeda); track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
                </select>
              </label>
            } @else {
              <p><span class="texto-suave">Ativo</span><br><strong>{{ fo.ticker }}</strong>
                @if (fo.moeda !== 'BRL') { <span class="etiqueta etiqueta-suave">{{ fo.moeda }}</span> }</p>
            }
          </div>
          <div class="grade-campos">
            <label>Data <app-campo-data name="data" [(ngModel)]="fo.data" (ngModelChange)="aoMudarDataOuTipo()" obrigatorio [maximo]="hoje" /></label>
            <label>Quantidade <input type="number" name="qtd" [(ngModel)]="fo.quantidade" required min="0.000001" step="any"></label>
            <label>Preço por cota <app-campo-valor name="preco" [(ngModel)]="fo.precoUnitario" [moeda]="fo.moeda" [casas]="4" obrigatorio /></label>
            <label>Taxas/corretagem <app-campo-valor name="taxas" [(ngModel)]="fo.taxas" [moeda]="fo.moeda" /></label>
          </div>
          @if (fo.moeda !== 'BRL') {
            <div class="grade-campos">
              <label>Câmbio (R$ por 1 {{ fo.moeda }})
                <input type="number" name="cambio" [(ngModel)]="fo.cambio" (ngModelChange)="cambioEditado = true"
                  required min="0.000001" step="any">
              </label>
              <p class="texto-suave nota-cambio">
                @if (buscandoCambio()) {
                  Buscando a PTAX do dia…
                } @else if (ptaxSugerida; as s) {
                  PTAX de {{ s.tipo }} de {{ s.data | date: 'dd/MM/yyyy' }}: {{ s.valor | number: '1.4-4' }}.
                  @if (cambioEditado && fo.cambio !== s.valor) {
                    <button type="button" class="link" (click)="usarPtax()">usar a PTAX</button>
                  } @else {
                    Troque pelo câmbio que a corretora cobrou, se for diferente.
                  }
                } @else {
                  Sem PTAX para essa data: informe o câmbio que a corretora cobrou.
                }
              </p>
            </div>
          }
          <label>Observação <input name="obs" [(ngModel)]="fo.observacao" maxlength="300"></label>
          <p class="texto-suave">Total da operação: <strong>{{ totalOperacao(fo) | currency: fo.moeda }}</strong>
            @if (fo.moeda !== 'BRL' && fo.cambio) { ≈ <strong>{{ totalOperacao(fo) * fo.cambio | currency }}</strong> }</p>
          <div class="rodape-form">
            @if (editandoOpId) { <button type="button" class="botao botao-perigo" (click)="excluirOperacao()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formOp = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <!-- Operações de um ativo, cada compra comparada com o preço atual -->
    <app-modal [aberto]="ativoAberto() !== null" [titulo]="'Operações · ' + (ativoAberto()?.ticker ?? '')" (fechar)="ativoAberto.set(null)">
      @if (ativoAberto(); as a) {
        <div class="resumo-ativo">
          <div><span class="texto-suave">Preço médio</span><strong>{{ a.precoMedio | currency: a.moeda }}</strong></div>
          <div><span class="texto-suave">Cotação atual</span>
            <strong>{{ a.precoAtual !== null ? (a.precoAtual | currency: a.moeda) : '—' }}</strong></div>
          <div><span class="texto-suave">Cotas</span><strong>{{ qtd(a.quantidade) }}</strong></div>
          @if (a.ultimoProvento; as up) {
            <div><span class="texto-suave">Último provento (data-com {{ up.dataCom | date: 'dd/MM/yy' }})</span>
              <strong>{{ up.valorPorCota | currency: a.moeda : 'symbol' : '1.2-4' }} por cota</strong></div>
          }
        </div>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead>
              <tr><th>Data</th><th>Operação</th><th class="direita">Cotas</th><th class="direita">Preço</th>
                <th class="direita">Total</th><th class="direita">Hoje x preço pago</th><th class="direita">Se fosse Selic</th>
                <th>Origem</th><th></th></tr>
            </thead>
            <tbody>
              @for (o of operacoes(); track o.id) {
                <tr>
                  <td>{{ o.data | date: 'dd/MM/yyyy' }}</td>
                  <td><span class="etiqueta" [class.etiqueta-sucesso]="o.tipo === TipoOperacao.Compra"
                    [class.etiqueta-alerta]="o.tipo === TipoOperacao.Venda">{{ o.tipo === TipoOperacao.Compra ? 'compra' : 'venda' }}</span></td>
                  <td class="direita">{{ qtd(o.quantidade) }}</td>
                  <td class="direita">{{ o.precoUnitario | currency: o.moeda : 'symbol' : '1.2-4' }}
                    @if (o.moeda !== 'BRL') { <br><small class="texto-suave">câmbio {{ o.cambio | number: '1.2-4' }}</small> }</td>
                  <td class="direita">{{ o.valorTotal | currency }}</td>
                  <td class="direita">
                    @if (o.variacaoPreco !== null) {
                      <span [class.positivo]="o.variacaoPreco >= 0" [class.negativo]="o.variacaoPreco < 0">{{ pct(o.variacaoPreco) }}</span>
                      <br><small class="texto-suave">vale {{ o.valorAtual | currency }}</small>
                    } @else {
                      <span class="texto-suave">—</span>
                    }
                  </td>
                  <td class="direita texto-suave">{{ o.tipo === TipoOperacao.Compra ? (o.valorSeFosseSelic | currency) : '—' }}</td>
                  <td><span class="etiqueta etiqueta-suave">{{ nomeOrigem(o.origem) }}</span></td>
                  <td class="direita"><button type="button" class="link" (click)="editarOperacao(o)">editar</button></td>
                </tr>
              } @empty {
                <tr><td colspan="9" class="vazio">Carregando…</td></tr>
              }
            </tbody>
          </table>
        </div>
        <div class="rodape-form">
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="novaOperacao(a.ticker)">+ Nova operação de {{ a.ticker }}</button>
        </div>
      }
    </app-modal>

    <!-- Editar ativo -->
    <app-modal [aberto]="formAtivo !== null" [titulo]="'Ativo · ' + (ativoEditado?.ticker ?? '')" (fechar)="formAtivo = null">
      @if (formAtivo; as fa) {
        <form (ngSubmit)="salvarAtivo()" #fat="ngForm" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fa.nome" maxlength="150"></label>
          <label>Tipo
            <select name="tipo" [(ngModel)]="fa.tipo">
              @for (t of tiposPara(ativoEditado?.moeda); track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
            </select>
          </label>
          @if (fa.tipo === TipoAtivo.Fii) {
            <label>Segmento
              <select name="segmento" [(ngModel)]="fa.segmento">
                <option [ngValue]="null">Não informado</option>
                @for (sg of segmentos; track sg) { <option [ngValue]="sg">{{ sg }}</option> }
              </select>
            </label>
          }
          <label class="checkbox"><input type="checkbox" name="manual" [(ngModel)]="fa.precoManual"> Informar a cotação manualmente</label>
          @if (fa.precoManual) {
            <label>Cotação atual <app-campo-valor name="preco" [(ngModel)]="fa.precoAtual" [moeda]="ativoEditado?.moeda" [casas]="4"
              obrigatorio [minimo]="0.0001" /></label>
            <p class="texto-suave">Útil quando a fonte automática não encontra o ativo. Desmarque para voltar à cotação automática.</p>
          }
          <div class="rodape-form">
            <button type="button" class="botao botao-perigo" (click)="excluirAtivo()">Excluir ativo</button>
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formAtivo = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fat.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-conferir-investimentos [aberto]="conferenciaAberta()" (fechar)="conferenciaAberta.set(false)"
      (corrigido)="recarregarDepoisDeAlterar()" (lancarAMao)="lancarCompraQueFaltou($event)"
      (verOperacoes)="verOperacoesDe($event)" />

    <!-- Importar do extrato -->
    <app-modal [aberto]="importacaoAberta()" titulo="Importar compras e vendas do extrato" (fechar)="importacaoAberta.set(false)">
      <div class="formulario">
        <p class="texto-suave">
          Transações com código da B3 na descrição (ex.: "COMPRA MXRF11 10 x 9,85"). Quando o extrato não traz quantidade
          ou preço, eles são calculados pelo valor da transação e pela cotação de fechamento do dia. Confira antes de importar.
          FIIs da corretora conectada no Meu Pluggy entram sozinhos na sincronização.
        </p>
        @if (carregandoSugestoes()) {
          <p class="vazio">Procurando nas transações e buscando as cotações do dia…</p>
        } @else {
          <div class="tabela-rolagem">
            <table class="tabela tabela-importacao">
              <thead>
                <tr><th></th><th>Data</th><th>Transação</th><th>Ativo</th><th>Operação</th>
                  <th class="direita">Valor</th><th class="direita">Cotas</th><th class="direita">Preço</th><th>Como</th></tr>
              </thead>
              <tbody>
                @for (l of linhasImportacao(); track l.transacaoId) {
                  <tr [class.encerrada]="l.jaImportada" [class.selecionada]="l.selecionada && !l.jaImportada">
                    <td class="col-check"><label class="celula-marcar"><input type="checkbox" [(ngModel)]="l.selecionada"
                      [disabled]="l.jaImportada" [attr.aria-label]="'Importar ' + l.descricao"></label></td>
                    <td>{{ l.data | date: 'dd/MM/yy' }}</td>
                    <td><small>{{ l.descricao }}</small><br><small class="texto-suave">{{ l.contaNome }}</small>
                      @if (l.jaImportada) { <br><small class="etiqueta etiqueta-sucesso">já importada</small> }
                      @else if (l.possivelDuplicata) { <br><small class="etiqueta etiqueta-alerta">já existe operação parecida</small> }</td>
                    <td><input class="campo-curto maiusculas" [(ngModel)]="l.ticker" maxlength="12" [disabled]="l.jaImportada"></td>
                    <td>
                      <select [(ngModel)]="l.tipo" [disabled]="l.jaImportada">
                        <option [ngValue]="TipoOperacao.Compra">compra</option>
                        <option [ngValue]="TipoOperacao.Venda">venda</option>
                      </select>
                    </td>
                    <td class="direita">{{ l.valor | currency }}</td>
                    <td class="direita"><input type="number" class="campo-curto" [(ngModel)]="l.quantidade" min="0" step="any"
                      [disabled]="l.jaImportada"></td>
                    <td class="direita"><app-campo-valor class="campo-curto" compacto [(ngModel)]="l.precoUnitario" [casas]="4"
                      [disabled]="l.jaImportada" [rotulo]="'Preço por cota de ' + (l.ticker || 'ativo')" />
                      @if (l.fechamentoNoDia !== null) { <br><small class="texto-suave">fech. {{ l.fechamentoNoDia | currency }}</small> }</td>
                    <td><small class="texto-suave">{{ l.origem ?? 'preencha' }}</small></td>
                  </tr>
                } @empty {
                  <tr><td colspan="9" class="vazio">Nenhuma transação com código de ativo encontrada no extrato.</td></tr>
                }
              </tbody>
            </table>
          </div>
        }
        <div class="rodape-form">
          <span class="texto-suave">{{ selecionadasImportacao() }} selecionada(s)</span>
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="importacaoAberta.set(false)">Fechar</button>
          <button type="button" class="botao botao-primario" [disabled]="!selecionadasImportacao() || salvando()"
            (click)="importarSelecionadas()">Importar</button>
        </div>
      </div>
    </app-modal>

    <!-- #170: nome do investimento do Open Finance (a sincronização não mexe nele) -->
    <app-modal [aberto]="formNome !== null" [titulo]="'Renomear · ' + (formNome?.investimento?.nomeExibicao ?? '')"
      (fechar)="formNome = null">
      @if (formNome; as fn) {
        <form (ngSubmit)="salvarNome()" class="formulario">
          <label>Nome <input name="nome" [(ngModel)]="fn.nome" maxlength="120" placeholder="Ex.: Caixinha viagem"></label>
          <p class="texto-suave">No banco: {{ fn.investimento.nome }}. A sincronização não muda o nome que você escolher;
            vazio, volta ao nome automático.</p>
          <div class="rodape-form">
            @if (fn.investimento.nomePersonalizado) {
              <button type="button" class="botao" (click)="fn.nome = ''; salvarNome()">Voltar ao nome automático</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formNome = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <!-- Conferir o saldo real de uma reserva -->
    <app-modal [aberto]="formConferencia !== null" [titulo]="'Conferir saldo · ' + (formConferencia?.reserva?.nome ?? '')"
      (fechar)="formConferencia = null">
      @if (formConferencia; as fc) {
        <form (ngSubmit)="salvarConferencia()" #fcf="ngForm" class="formulario">
          <p class="texto-suave">
            @if (fc.reserva.moeda === 'USD') {
              Abra o Méli Dólar no app do Mercado Pago e informe o saldo que aparece lá, em reais ou em dólares. Em reais,
              ele é convertido pela PTAX do dia da conferência.
            } @else {
              Abra a reserva no app da instituição e informe o saldo que aparece lá.
            }
            Os lançamentos até essa data já estão nesse valor; os seguintes são somados a ele.</p>
          <div class="grade-campos">
            @if (fc.reserva.moeda === 'USD') {
              <label>Informado em
                <select name="moeda" [(ngModel)]="fc.moeda">
                  <option value="BRL">reais (R$)</option>
                  <option value="USD">dólares (US$)</option>
                </select>
              </label>
            }
            <label>Saldo na instituição
              <app-campo-valor name="saldo" [(ngModel)]="fc.saldo" [moeda]="fc.moeda" obrigatorio /></label>
            <label>Conferido em <app-campo-data name="data" [(ngModel)]="fc.data" obrigatorio [maximo]="hoje" /></label>
          </div>
          <p class="texto-suave">Estimativa atual: <strong>{{ fc.reserva.saldo | currency }}</strong>
            @if (fc.reserva.moeda === 'USD' && fc.reserva.quantidade !== null) { ({{ fc.reserva.quantidade | currency: 'USD' }}) }</p>
          <div class="rodape-form">
            @if (fc.reserva.dataConferencia) {
              <button type="button" class="botao botao-perigo" (click)="removerConferencia()">Voltar a estimar</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="formConferencia = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fcf.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <!-- Fonte das cotações -->
    <app-modal [aberto]="configAberta()" titulo="Fonte das cotações" (fechar)="configAberta.set(false)">
      <form class="formulario" (ngSubmit)="salvarToken(false)">
        <p>As cotações e os proventos vêm do <strong>Yahoo Finance</strong> (gratuito, sem cadastro, não oficial). A Selic vem da
          API pública do <strong>Banco Central</strong>. Só o código do ativo é enviado; quantidades e valores ficam no seu computador.</p>
        <p class="texto-suave">Opcional: um token gratuito da <strong>brapi.dev</strong> é usado como alternativa quando o Yahoo não
          responder. Ele fica criptografado no banco local.</p>
        <label>Token da brapi.dev
          <input type="password" name="token" [(ngModel)]="tokenBrapi" autocomplete="off"
            [placeholder]="tokenConfigurado() ? '•••••• (configurado)' : 'não configurado'">
        </label>
        <div class="rodape-form">
          @if (tokenConfigurado()) {
            <button type="button" class="botao botao-perigo" (click)="salvarToken(true)">Remover token</button>
          }
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="configAberta.set(false)">Fechar</button>
          <button class="botao botao-primario" [disabled]="!tokenBrapi.trim()">Salvar</button>
        </div>
      </form>
    </app-modal>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    .barra-abas { margin-bottom: 16px; }
    .legenda-grafico { margin: -6px 0 10px; font-size: 0.84rem; }
    .cabecalho-bloco { display: flex; align-items: center; gap: 14px; margin-bottom: 10px; flex-wrap: wrap; }
    .cabecalho-bloco h2 { margin: 0; }
    .tabela-carteira td { white-space: nowrap; }
    .tabela-carteira td:first-child { white-space: normal; min-width: 160px; }
    tr.encerrada td { opacity: 0.6; }
    .nowrap { white-space: nowrap; }
    .destaque-conferir { font-weight: 600; }
    .nowrap .link + .link { margin-left: 10px; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
    .resumo-ativo { display: flex; gap: 24px; flex-wrap: wrap; margin-bottom: 14px; }
    .resumo-ativo div { display: grid; gap: 2px; }
    .resumo-ativo span { font-size: 0.8rem; }
    .maiusculas { text-transform: uppercase; }
    .barra-importacao { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
    .tabela-importacao td { vertical-align: top; }
    .campo-curto { width: 96px; padding: 4px 8px; }
    .nota-cambio { align-self: end; margin: 0 0 6px; font-size: 0.85rem; }
    :host ::ng-deep .modal:has(.tabela-importacao) { width: min(1100px, 96vw); }
  `
})
export class InvestimentosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly TipoOperacao = TipoOperacao;
  readonly TipoAtivo = TipoAtivo;
  readonly segmentos = [
    'Logística', 'Shoppings', 'Lajes corporativas', 'Papel (recebíveis)', 'Híbrido', 'Fundo de fundos',
    'Renda urbana', 'Agronegócio', 'Residencial', 'Hotéis', 'Hospitais/Educação', 'Desenvolvimento', 'Outros'
  ];
  readonly tiposAtivo = [TipoAtivo.Fii, TipoAtivo.Acao, TipoAtivo.Etf, TipoAtivo.Bdr, TipoAtivo.Outro];
  readonly hoje = dataIso();

  readonly dados = signal<Carteira | null>(null);
  private readonly rota = inject(ActivatedRoute);
  readonly aba = signal<Aba>('resumo');
  readonly mostrarEncerradas = signal(false);
  readonly atualizando = signal(false);
  readonly salvando = signal(false);
  readonly ativoAberto = signal<PosicaoAtivo | null>(null);
  readonly operacoes = signal<OperacaoAtivo[]>([]);
  readonly configAberta = signal(false);
  readonly tokenConfigurado = signal(false);
  readonly versaoCarteira = signal(0);
  readonly importacaoAberta = signal(false);
  readonly conferenciaAberta = signal(false);
  readonly carregandoSugestoes = signal(false);
  readonly linhasImportacao = signal<LinhaImportacao[]>([]);
  readonly reservasAConferir = computed(() =>
    (this.dados()?.reservas ?? []).filter((r) => r.podeTerSaldoAnterior || r.conferenciaAntiga).length);
  readonly quantidadeFiis = computed(() => this.abertas().filter((p) => p.tipo === TipoAtivo.Fii).length);

  formOp: SalvarOperacao | null = null;
  editandoOpId: number | null = null;
  readonly moedas = signal<MoedaCambio[]>([]);
  readonly buscandoCambio = signal(false);
  /** Ativo já lançado: a moeda vem dele e não pode ser trocada. */
  moedaTravada = false;
  /** O usuário digitou o câmbio: mudar data/tipo não sobrescreve. */
  cambioEditado = false;
  ptaxSugerida: { data: string; valor: number; tipo: 'compra' | 'venda' } | null = null;
  formAtivo: AtualizarAtivo | null = null;
  formConferencia: FormConferencia | null = null;
  formNome: { investimento: InvestimentoComparado; nome: string } | null = null;
  ativoEditado: PosicaoAtivo | null = null;
  tokenBrapi = '';
  private atualizouAutomatico = false;
  private importouAutomatico = false;
  readonly importandoOpenFinance = signal(false);

  readonly abertas = computed(() => (this.dados()?.posicoes ?? []).filter((p) => p.quantidade > 0));
  readonly posicoesVisiveis = computed(() => (this.mostrarEncerradas() ? this.dados()?.posicoes ?? [] : this.abertas()));
  readonly alturaGraficoAtivos = computed(() => Math.max(220, this.abertas().length * 36));

  /* ---------------- UX-9: frases-resumo (sem R$ quando "ocultar valores" está ligado) ---------------- */

  /** Valor de mercado hoje × o mesmo dinheiro na Selic (último mês com valor de mercado). */
  readonly resumoEvolucao = computed(() => {
    const ev = (this.dados()?.evolucao ?? []).filter((m) => m.valorMercado !== null);
    const ultimo = ev[ev.length - 1];
    if (!ultimo || ultimo.aplicadoNaSelic <= 0) return null;
    const dif = ((ultimo.valorMercado! - ultimo.aplicadoNaSelic) / ultimo.aplicadoNaSelic) * 100;
    if (Math.abs(dif) < 0.5) return 'Sua renda variável vale hoje praticamente o mesmo que esse dinheiro teria na Selic.';
    const oculto = this.privacidade.oculto();
    const valores = oculto ? '' : ` (${reaisOuNada(ultimo.valorMercado!, false)} contra ${reaisOuNada(ultimo.aplicadoNaSelic, false)})`;
    return `Sua renda variável vale hoje ${pct(dif)} ${dif > 0 ? 'mais' : 'menos'} do que esse dinheiro teria na Selic${valores}.`;
  });

  readonly resumoDistribuicao = computed(() => {
    const d = this.dados();
    const dist = [...(d?.distribuicao ?? [])].sort((a, b) => b.valor - a.valor);
    if (!d || !dist.length || d.totalGeral <= 0) return null;
    return `${dist[0].nome} concentra ${pct(dist[0].valor / d.totalGeral * 100)} dos seus investimentos.`;
  });

  readonly resumoAtivos = computed(() => {
    const ativos = this.abertas().filter((p) => p.resultadoTotalPercentual !== null && p.resultadoSelicPercentual !== null);
    if (!ativos.length) return null;
    const ganharam = ativos.filter((p) => p.resultadoTotalPercentual! >= p.resultadoSelicPercentual!).length;
    if (ganharam === ativos.length) return ativos.length === 1 ? 'Seu ativo rendeu mais que a Selic no mesmo período.'
      : `Todos os ${ativos.length} ativos renderam mais que a Selic no mesmo período.`;
    if (ganharam === 0) return 'Nenhum ativo rendeu mais que a Selic no mesmo período.';
    return `${ganharam} de ${ativos.length} ativos renderam mais que a Selic no mesmo período.`;
  });

  readonly graficoEvolucao = computed<ChartConfiguration<'line'>>(() => {
    const ev = this.dados()?.evolucao ?? [];
    return {
      type: 'line',
      data: {
        labels: ev.map((m) => `${MESES[m.mes - 1]}/${String(m.ano).slice(2)}`),
        datasets: [
          {
            label: 'Aplicado (compras − vendas)', data: ev.map((m) => m.aplicado), borderColor: COR_CARTEIRA,
            backgroundColor: COR_CARTEIRA + '33', fill: true, tension: 0.3, borderWidth: 2.5, pointRadius: 0
          },
          {
            label: 'Mesmo dinheiro na Selic', data: ev.map((m) => m.aplicadoNaSelic), borderColor: COR_SELIC,
            borderDash: [6, 5], tension: 0.3, borderWidth: 2, pointRadius: 0
          },
          {
            label: 'Valor de mercado hoje', data: ev.map((m) => m.valorMercado), borderColor: COR_MERCADO,
            backgroundColor: COR_MERCADO, pointRadius: 6, pointHoverRadius: 8, showLine: false
          }
        ]
      },
      options: {
        interaction: { mode: 'index', intersect: false },
        plugins: { legend: { position: 'bottom' } },
        scales: { y: { ticks: { callback: compacto } } }
      }
    };
  });

  readonly graficoDistribuicao = computed<ChartConfiguration<'doughnut'>>(() => {
    const dist = this.dados()?.distribuicao ?? [];
    return {
      type: 'doughnut',
      data: {
        labels: dist.map((g) => g.nome),
        datasets: [{ data: dist.map((g) => g.valor), backgroundColor: dist.map((_, i) => this.cor(i)), borderWidth: 0 }]
      },
      options: { cutout: '72%', plugins: { legend: { display: false } } }
    };
  });

  readonly graficoAtivos = computed<ChartConfiguration<'bar'>>(() => {
    const ativos = this.abertas();
    return {
      type: 'bar',
      data: {
        labels: ativos.map((p) => p.ticker),
        datasets: [
          {
            label: 'Seu resultado (%)', data: ativos.map((p) => p.resultadoTotalPercentual ?? 0),
            backgroundColor: ativos.map((p) =>
              (p.resultadoTotalPercentual ?? 0) >= (p.resultadoSelicPercentual ?? 0) ? CORES_GRAFICO.ganhou : CORES_GRAFICO.perdeu),
            borderRadius: 6, borderSkipped: false
          },
          {
            label: 'Selic no mesmo período (%)', data: ativos.map((p) => p.resultadoSelicPercentual ?? 0),
            backgroundColor: COR_SELIC, borderRadius: 6, borderSkipped: false
          }
        ]
      },
      options: {
        indexAxis: 'y',
        plugins: { legend: { position: 'bottom' } },
        scales: { x: { ticks: { callback: (v) => `${v}%` } } }
      }
    };
  });

  ngOnInit(): void {
    // #89: /investimentos?aba=reservas abre direto numa aba (link "confira" do cofrinho em Metas).
    const aba = this.rota.snapshot.queryParamMap.get('aba');
    if (aba === 'resumo' || aba === 'fii' || aba === 'rv' || aba === 'of' || aba === 'reservas') this.aba.set(aba);
    // #170: o aviso de trocos do Painel abre direto em "Conferir investimentos".
    if (this.rota.snapshot.queryParamMap.get('conferir') === '1') this.conferenciaAberta.set(true);
    this.carregar();
  }

  carregar(): void {
    this.api.carteira().subscribe({
      next: (c) => {
        this.dados.set(c);
        this.versaoCarteira.update((v) => v + 1);
        // Cotações antigas: atualiza uma vez ao abrir a página.
        // Investimentos do Open Finance ainda fora da carteira: registra as compras automaticamente (uma vez).
        if (c.openFinanceNaoRegistrados > 0 && !this.importouAutomatico) {
          this.importouAutomatico = true;
          this.importarOpenFinance(true);
          return;
        }
        if (c.cotacoesDesatualizadas && !this.atualizouAutomatico) {
          this.atualizouAutomatico = true;
          this.atualizarCotacoes(true);
        }
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  atualizarCotacoes(silencioso = false): void {
    if (this.atualizando()) return;
    this.atualizando.set(true);
    this.api.atualizarCotacoes().subscribe({
      next: (r) => {
        this.atualizando.set(false);
        if (r.falhas.length) {
          this.avisos.erro(`Sem cotação para ${r.falhas.join(', ')}. Tente de novo mais tarde ou informe a cotação manualmente.`);
        } else if (!silencioso) {
          const partes = [`${r.atualizados} cotação(ões) atualizada(s)`];
          if (r.proventosNovos) partes.push(`${r.proventosNovos} provento(s) novo(s)`);
          this.avisos.sucesso(partes.join(' · '));
        }
        this.carregar();
      },
      error: (e) => {
        this.atualizando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ operações ------------------------------ */

  novaOperacao(ticker = ''): void {
    const existente = ticker ? this.dados()?.posicoes.find((p) => p.ticker === ticker) : undefined;
    this.editandoOpId = null;
    this.moedaTravada = !!existente;
    this.cambioEditado = false;
    this.ptaxSugerida = null;
    this.formOp = {
      ticker, tipoAtivo: null, tipo: TipoOperacao.Compra, data: this.hoje, quantidade: null, precoUnitario: null,
      taxas: 0, observacao: null, moeda: existente?.moeda ?? 'BRL', cambio: null
    };
    this.carregarMoedas();
    this.sugerirCambio();
  }

  editarOperacao(o: OperacaoAtivo): void {
    this.editandoOpId = o.id;
    this.moedaTravada = true;
    // Mantém o câmbio gravado; a PTAX aparece só como referência.
    this.cambioEditado = true;
    this.ptaxSugerida = null;
    this.formOp = {
      ticker: o.ticker, tipoAtivo: null, tipo: o.tipo, data: o.data, quantidade: o.quantidade,
      precoUnitario: o.precoUnitario, taxas: o.taxas, observacao: o.observacao, moeda: o.moeda,
      cambio: o.moeda === 'BRL' ? null : o.cambio
    };
    this.sugerirCambio();
  }

  aoMudarMoeda(): void {
    if (!this.formOp) return;
    this.formOp.tipoAtivo = null;
    this.formOp.cambio = null;
    this.cambioEditado = false;
    this.sugerirCambio();
  }

  aoMudarDataOuTipo(): void {
    this.sugerirCambio();
  }

  usarPtax(): void {
    if (!this.formOp || !this.ptaxSugerida) return;
    this.formOp.cambio = this.ptaxSugerida.valor;
    this.cambioEditado = false;
  }

  /** Busca a PTAX da data (venda na compra, compra na venda) e preenche o câmbio, se o usuário não o digitou. */
  private sugerirCambio(): void {
    const f = this.formOp;
    this.ptaxSugerida = null;
    if (!f || f.moeda === 'BRL' || !f.data) return;
    const { moeda, data, tipo } = f;
    this.buscandoCambio.set(true);
    this.api.cotacaoCambio(moeda, data).subscribe({
      next: (c) => {
        this.buscandoCambio.set(false);
        // Ignora respostas atrasadas de uma escolha anterior.
        if (this.formOp !== f || f.moeda !== moeda || f.data !== data || f.tipo !== tipo) return;
        const compra = tipo === TipoOperacao.Compra;
        this.ptaxSugerida = { data: c.data, valor: compra ? c.venda : c.compra, tipo: compra ? 'venda' : 'compra' };
        if (!this.cambioEditado) f.cambio = this.ptaxSugerida.valor;
      },
      error: () => this.buscandoCambio.set(false)
    });
  }

  private carregarMoedas(): void {
    if (this.moedas().length > 0) return;
    this.api.moedasCambio().subscribe({ next: (m) => this.moedas.set(m), error: () => this.moedas.set([]) });
  }

  /** Total na moeda do ativo. */
  totalOperacao(f: SalvarOperacao): number {
    const bruto = (f.quantidade ?? 0) * (f.precoUnitario ?? 0);
    return f.tipo === TipoOperacao.Compra ? bruto + (f.taxas ?? 0) : bruto - (f.taxas ?? 0);
  }

  simbolo(moeda: string | null | undefined): string {
    const m = moeda ?? 'BRL';
    return SIMBOLOS_MOEDA[m] ?? m;
  }

  /** FIIs e BDRs são da B3: ativo no exterior só pode ser ação, ETF ou outro. */
  tiposPara(moeda: string | null | undefined): TipoAtivo[] {
    return !moeda || moeda === 'BRL'
      ? this.tiposAtivo
      : this.tiposAtivo.filter((t) => t !== TipoAtivo.Fii && t !== TipoAtivo.Bdr);
  }

  salvarOperacao(): void {
    const f = this.formOp;
    if (!f) return;
    const dto: SalvarOperacao = {
      ...f, ticker: f.ticker.trim().toUpperCase(), taxas: f.taxas ?? 0, observacao: f.observacao?.trim() || null,
      cambio: f.moeda === 'BRL' ? null : f.cambio
    };
    this.salvando.set(true);
    const id = this.editandoOpId;
    const concluir = {
      next: () => {
        this.salvando.set(false);
        this.formOp = null;
        this.avisos.sucesso('Operação salva.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e: unknown) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    };
    if (id) this.api.atualizarOperacao(id, dto).subscribe(concluir);
    else this.api.criarOperacao(dto).subscribe(concluir);
  }

  excluirOperacao(): void {
    const id = this.editandoOpId;
    if (!id || !confirm('Excluir esta operação?')) return;
    this.api.excluirOperacao(id).subscribe({
      next: () => {
        this.formOp = null;
        this.avisos.sucesso('Operação excluída.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirOperacoes(p: PosicaoAtivo): void {
    this.operacoes.set([]);
    this.ativoAberto.set(p);
    this.api.operacoesAtivo(p.id).subscribe({
      next: (o) => this.operacoes.set(o),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ conferir investimentos ------------------------------ */

  /** "GARE11, XPLG11 e XPML11". */
  listaTickers(tickers: string[]): string {
    return tickers.length > 1 ? `${tickers.slice(0, -1).join(', ')} e ${tickers[tickers.length - 1]}` : tickers[0] ?? '';
  }

  /** Sem débito no extrato que explique a diferença: abre "Lançar compra" com o ativo e as cotas que faltam. */
  lancarCompraQueFaltou(e: { ticker: string; quantidade: number }): void {
    this.conferenciaAberta.set(false);
    this.novaOperacao(e.ticker);
    if (this.formOp) {
      this.formOp.quantidade = e.quantidade;
      this.formOp.observacao = 'Compra que faltou no Open Finance';
    }
  }

  verOperacoesDe(ticker: string): void {
    const posicao = this.dados()?.posicoes.find((p) => p.ticker === ticker);
    if (!posicao) return;
    this.conferenciaAberta.set(false);
    this.abrirOperacoes(posicao);
  }

  recarregarDepoisDeAlterar(): void {
    this.api.carteira().subscribe({
      next: (c) => {
        this.dados.set(c);
        this.versaoCarteira.update((v) => v + 1);
        const aberto = this.ativoAberto();
        if (aberto) {
          const atualizado = c.posicoes.find((p) => p.id === aberto.id);
          if (atualizado) this.abrirOperacoes(atualizado);
          else this.ativoAberto.set(null);
        }
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ ativo ------------------------------ */

  editarAtivo(p: PosicaoAtivo): void {
    this.ativoEditado = p;
    this.formAtivo = {
      nome: p.nome, tipo: p.tipo, segmento: p.segmento, precoManual: p.precoManual, precoAtual: p.precoAtual
    };
  }

  salvarAtivo(): void {
    const p = this.ativoEditado;
    const f = this.formAtivo;
    if (!p || !f) return;
    this.api.atualizarAtivo(p.id, { ...f, nome: f.nome?.trim() || null }).subscribe({
      next: () => {
        this.formAtivo = null;
        this.avisos.sucesso('Ativo atualizado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  excluirAtivo(): void {
    const p = this.ativoEditado;
    if (!p || !confirm(`Excluir ${p.ticker} com todas as operações?`)) return;
    this.api.excluirAtivo(p.id).subscribe({
      next: () => {
        this.formAtivo = null;
        this.avisos.sucesso('Ativo excluído.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ nome (#170) ------------------------------ */

  abrirRenomear(i: InvestimentoComparado): void {
    this.formNome = { investimento: i, nome: i.nomePersonalizado ?? '' };
  }

  salvarNome(): void {
    const f = this.formNome;
    if (!f) return;
    const nome = f.nome.trim() || null;
    this.salvando.set(true);
    this.api.renomearInvestimento(f.investimento.id, nome).subscribe({
      next: () => {
        this.salvando.set(false);
        this.formNome = null;
        this.avisos.sucesso(nome ? 'Investimento renomeado.' : 'O investimento voltou ao nome automático.');
        this.carregar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ reservas ------------------------------ */

  /** #237: "conferir saldo" em toda reserva cujo valor não veio, pelo nome, do banco (reserva-conferencia.ts). */
  podeConferir(r: Reserva): boolean {
    return podeConferirReserva(r);
  }

  abrirConferencia(r: Reserva): void {
    // #169: o Méli Dólar começa em reais (o valor do app), com a estimativa atual.
    this.formConferencia = r.moeda === 'USD'
      ? { reserva: r, saldo: r.saldo, data: this.hoje, moeda: 'BRL' }
      : { reserva: r, saldo: r.saldoConferido ?? r.saldo, data: this.hoje, moeda: 'BRL' };
  }

  salvarConferencia(): void {
    const f = this.formConferencia;
    if (!f || f.saldo === null) return;
    const emDolar = f.reserva.moeda === 'USD';
    // A comparação com o esperado é em reais; em dólar sem cotação conhecida, não dá para comparar.
    const emReais = emDolar && f.moeda === 'USD' ? (f.reserva.cotacao ? f.saldo * f.reserva.cotacao : null) : f.saldo;
    if (emReais !== null && !this.conferenciaPlausivel(f.reserva, emReais)) return;
    this.salvando.set(true);
    this.api.conferirReserva(f.reserva.contaId, f.reserva.nome, f.saldo, f.data, emDolar ? f.moeda : undefined).subscribe({
      next: () => {
        this.salvando.set(false);
        this.formConferencia = null;
        this.avisos.sucesso('Saldo da reserva conferido.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** #168: valor muito diferente do esperado pede confirmação antes de salvar (reserva-conferencia.ts). */
  conferenciaPlausivel(r: Reserva, saldo: number): boolean {
    if (!conferenciaForaDoEsperado(r, saldo)) return true;
    const moeda = (v: number) => v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
    return confirm(`Você informou ${moeda(saldo)}, mas o esperado para "${r.nome}" é perto de ${moeda(valorEsperadoDaReserva(r))}. `
      + 'Confere com o app da instituição? (OK salva assim mesmo.)');
  }

  removerConferencia(): void {
    const f = this.formConferencia;
    if (!f || !confirm(`Apagar a conferência de ${f.reserva.nome}? O saldo volta a ser estimado só pelos lançamentos.`)) return;
    this.api.removerConferenciaReserva(f.reserva.contaId, f.reserva.nome).subscribe({
      next: () => {
        this.formConferencia = null;
        this.avisos.sucesso('Conferência apagada.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ importação do Open Finance ------------------------------ */

  importarOpenFinance(automatico = false): void {
    if (this.importandoOpenFinance()) return;
    this.importandoOpenFinance.set(true);
    this.importouAutomatico = true;
    this.atualizouAutomatico = true; // a importação já atualiza as cotações
    this.api.importarOpenFinance().subscribe({
      next: (r) => {
        this.importandoOpenFinance.set(false);
        if (r.mensagem) {
          this.avisos.erro(r.mensagem);
        } else {
          const partes = [`${r.operacoesImportadas} compra(s)/venda(s) registrada(s)`];
          if (r.ativosCriados) partes.push(`${r.ativosCriados} ativo(s) novo(s)`);
          if (r.semHistorico.length) {
            partes.push(`sem histórico na corretora: ${r.semHistorico.join(', ')} (registrado o saldo atual; confira data e preço em Operações)`);
          }
          if (r.comLancamentoManual.length) partes.push(`ignorados por já terem lançamentos seus: ${r.comLancamentoManual.join(', ')}`);
          if (r.erros.length) partes.push(`falha em ${r.erros.join(', ')}`);
          if (r.erros.length) this.avisos.erro(partes.join(' · '));
          else if (!automatico || r.operacoesImportadas) this.avisos.sucesso(partes.join(' · '));
        }
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.importandoOpenFinance.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ importação pelo extrato ------------------------------ */

  selecionadasImportacao(): number {
    return this.linhasImportacao().filter((l) => l.selecionada).length;
  }

  abrirImportacao(): void {
    this.importacaoAberta.set(true);
    this.carregandoSugestoes.set(true);
    this.linhasImportacao.set([]);
    this.api.sugestoesExtrato().subscribe({
      next: (s) => {
        this.carregandoSugestoes.set(false);
        this.linhasImportacao.set(s.map((x) => ({
          ...x, taxas: 0,
          selecionada: !x.jaImportada && !x.possivelDuplicata && x.quantidade !== null && x.precoUnitario !== null
        })));
      },
      error: (e) => {
        this.carregandoSugestoes.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  importarSelecionadas(): void {
    const selecionadas = this.linhasImportacao().filter((l) => l.selecionada && !l.jaImportada);
    const invalidas = selecionadas.filter((l) => !l.ticker.trim() || !l.quantidade || l.quantidade <= 0 || l.precoUnitario === null);
    if (invalidas.length) {
      this.avisos.erro(`Preencha cotas e preço de ${invalidas.map((l) => l.ticker || 'sem código').join(', ')}.`);
      return;
    }
    this.salvando.set(true);
    this.api.importarExtrato(selecionadas.map((l) => ({
      transacaoId: l.transacaoId, ticker: l.ticker.trim().toUpperCase(), tipo: l.tipo,
      quantidade: l.quantidade!, precoUnitario: l.precoUnitario!, taxas: l.taxas ?? 0
    }))).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.importacaoAberta.set(false);
        this.avisos.sucesso(`${r.transacoesAtualizadas} operação(ões) importada(s).`);
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ configuração ------------------------------ */

  abrirConfiguracao(): void {
    this.tokenBrapi = '';
    this.configAberta.set(true);
    this.api.configuracaoCotacao().subscribe((c) => this.tokenConfigurado.set(c.tokenBrapiConfigurado));
  }

  salvarToken(remover: boolean): void {
    if (!remover && !this.tokenBrapi.trim()) return;
    this.api.salvarConfiguracaoCotacao(remover ? null : this.tokenBrapi.trim(), remover).subscribe({
      next: () => {
        this.tokenBrapi = '';
        this.tokenConfigurado.set(!remover);
        this.avisos.sucesso(remover ? 'Token removido.' : 'Token salvo.');
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ formatação ------------------------------ */

  pct(valor: number | null, sinal = true): string {
    if (valor === null || valor === undefined) return '—';
    const texto = valor.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    return `${sinal && valor > 0 ? '+' : ''}${texto}%`;
  }

  qtd(valor: number): string {
    if (this.privacidade.oculto()) return '•••';
    return valor.toLocaleString('pt-BR', { maximumFractionDigits: 6 });
  }

  nomeTipo(t: TipoAtivo): string {
    return NOMES_TIPO_ATIVO[t] ?? 'Outro';
  }

  nomeOrigem(o: OrigemOperacao): string {
    return NOMES_ORIGEM_OPERACAO[o] ?? '';
  }

  nomeTipoPluggy(t: string, subtipo: string | null = null, codigo: string | null = null): string {
    if (t?.toUpperCase() === 'EQUITY' && (subtipo?.toUpperCase() === 'REAL_ESTATE_FUND' || /^[A-Z]{4}1[123]$/.test(codigo ?? ''))) {
      return 'FII';
    }
    return NOMES_TIPO_PLUGGY[t] ?? t;
  }

  cor(i: number): string {
    return CORES[i % CORES.length];
  }
}
