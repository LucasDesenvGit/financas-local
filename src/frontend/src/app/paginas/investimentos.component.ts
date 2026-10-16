import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ChartConfiguration } from 'chart.js';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  AtualizarAtivo, Carteira, NOMES_ORIGEM_OPERACAO, NOMES_TIPO_ATIVO, OperacaoAtivo, OrigemOperacao, PosicaoAtivo,
  SalvarOperacao, SugestaoExtrato, TipoAtivo, TipoOperacao
} from '../core/modelos';
import { PrivacidadeService } from '../core/privacidade.service';
import { AvisosService } from '../compartilhado/avisos.component';
import { GraficoComponent } from '../compartilhado/grafico.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { dataIso } from '../compartilhado/utilidades';
import { InvestimentosFiisComponent } from './investimentos-fiis.component';

const NOMES_TIPO_PLUGGY: Record<string, string> = {
  FIXED_INCOME: 'Renda fixa',
  MUTUAL_FUND: 'Fundos',
  SECURITY: 'Previdência',
  EQUITY: 'Ações',
  ETF: 'ETFs',
  COE: 'COE',
  OTHER: 'Outros'
};
const CORES = ['#a7a3f5', '#92c8f0', '#8fd5b4', '#f8c09a', '#f3b3d3', '#8fd8d8', '#f2d98b', '#a9b4c6'];
const COR_CARTEIRA = '#a7a3f5';
const COR_SELIC = '#f8c09a';
const COR_MERCADO = '#8fd5b4';
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const compacto = (v: string | number) => Number(v).toLocaleString('pt-BR', { notation: 'compact' });

type Aba = 'resumo' | 'fii' | 'rv' | 'of';

/** Linha editável da importação pelo extrato. */
interface LinhaImportacao extends SugestaoExtrato { selecionada: boolean; taxas: number }

@Component({
  selector: 'app-investimentos',
  imports: [FormsModule, MoedaPipe, DatePipe, DecimalPipe, GraficoComponent, ModalComponent, InvestimentosFiisComponent],
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
        <button type="button" class="botao" (click)="abrirImportacao()">⇣ Importar do extrato</button>
        <button type="button" class="botao" (click)="atualizarCotacoes()" [disabled]="atualizando()">
          {{ atualizando() ? 'Atualizando…' : '↻ Atualizar cotações' }}
        </button>
        <button type="button" class="botao botao-primario" (click)="novaOperacao()">+ Lançar compra/venda</button>
      </div>
    </header>

    @if (dados(); as d) {
      <section class="kpis">
        <article class="kpi kpi-destaque">
          <span>Total investido</span>
          <strong>{{ d.totalGeral | currency }}</strong>
          <small>renda variável + Open Finance + contas de investimento</small>
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
          <span>Selic</span>
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
      </div>

      @switch (aba()) {
        @case ('resumo') {
          <div class="grade-2">
            <section class="painel-bloco">
              <h2>Seu dinheiro na renda variável x na Selic</h2>
              <p class="texto-suave legenda-grafico">Compras menos vendas, e quanto esse mesmo dinheiro teria hoje na Selic.
                O ponto verde é o valor de mercado atual (proventos não entram neste gráfico).</p>
              @if (d.evolucao.length) {
                <app-grafico [configuracao]="graficoEvolucao()" [altura]="270" />
              } @else { <p class="vazio">Lance suas compras para ver a comparação.</p> }
            </section>
            <section class="painel-bloco">
              <h2>Distribuição</h2>
              @if (d.distribuicao.length) {
                <app-grafico [configuracao]="graficoDistribuicao()" [altura]="200" />
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
          </div>

          @if (abertas().length) {
            <section class="painel-bloco">
              <h2>Rentabilidade por ativo x Selic no mesmo período</h2>
              <p class="texto-suave legenda-grafico">Inclui valorização, vendas e proventos. A barra da Selic usa as mesmas datas e
                valores das suas compras.</p>
              <app-grafico [configuracao]="graficoAtivos()" [altura]="alturaGraficoAtivos()" />
            </section>
          }
        }

        @case ('fii') {
          <app-investimentos-fiis [versao]="versaoCarteira()" (alterado)="carregar()" />
        }

        @case ('rv') {
          <section class="painel-bloco">
            <div class="cabecalho-bloco">
              <h2>Posições</h2>
              <span class="cresce"></span>
              <label class="checkbox"><input type="checkbox" [checked]="mostrarEncerradas()"
                (change)="mostrarEncerradas.set(!mostrarEncerradas())"> mostrar encerradas</label>
              <button type="button" class="link" (click)="abrirConfiguracao()">fonte das cotações</button>
            </div>
            <div class="tabela-rolagem">
              <table class="tabela tabela-carteira">
                <thead>
                  <tr>
                    <th>Ativo</th><th class="direita">Cotas</th><th class="direita">Preço médio</th>
                    <th class="direita">Cotação</th><th class="direita">Valor atual</th><th class="direita">Valorização</th>
                    <th class="direita">Proventos 12m</th><th class="direita">Resultado total</th><th class="direita">x Selic</th><th></th>
                  </tr>
                </thead>
                <tbody>
                  @for (p of posicoesVisiveis(); track p.id) {
                    <tr [class.encerrada]="p.quantidade === 0">
                      <td>
                        <strong>{{ p.ticker }}</strong> <span class="etiqueta etiqueta-suave">{{ nomeTipo(p.tipo) }}</span>
                        @if (p.nome) { <br><small class="texto-suave">{{ p.nome }}</small> }
                      </td>
                      <td class="direita">{{ qtd(p.quantidade) }}</td>
                      <td class="direita">{{ p.precoMedio | currency }}</td>
                      <td class="direita">
                        @if (p.precoAtual !== null) {
                          {{ p.precoAtual | currency }}
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
                      Nenhum ativo lançado. Use <strong>+ Lançar compra/venda</strong> com o código do ativo (ex.: AOK ou MXRF11) e o preço pago.
                    </td></tr>
                  }
                </tbody>
              </table>
            </div>
            <p class="texto-suave nota">
              Preço médio pelo método da Receita (taxas entram no custo). Proventos são estimados pela quantidade que você tinha na
              data-com. "x Selic" compara o valor atual com quanto o mesmo dinheiro (com as mesmas vendas e proventos retirados)
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
                  <th class="direita">Rentabilidade</th><th class="direita">Selic no período</th><th class="direita">% da Selic</th>
                  <th>Vencimento</th></tr>
              </thead>
              <tbody>
                @for (i of d.openFinance; track i.id) {
                  <tr [class.encerrada]="i.jaNaCarteira">
                    <td>{{ i.nome }} <span class="etiqueta etiqueta-suave">{{ nomeTipoPluggy(i.tipo, i.subtipo, i.codigo) }}</span>
                      @if (i.codigo) { <small class="texto-suave">{{ i.codigo }}</small> }
                      @if (i.taxaAnual) { <br><small class="texto-suave">{{ i.taxaAnual | number: '1.2-2' }}% a.a.</small> }
                      @if (i.jaNaCarteira) { <br><small class="etiqueta etiqueta-alerta">já lançado na carteira — não somado de novo</small> }
                    </td>
                    <td class="texto-suave">{{ i.nomeInstituicao }}</td>
                    <td class="direita">{{ i.valorAplicado !== null ? (i.valorAplicado | currency) : '—' }}
                      @if (i.dataAplicacao) { <br><small class="texto-suave">{{ i.dataAplicacao | date: 'dd/MM/yy' }}</small> }</td>
                    <td class="direita valor">{{ i.saldo | currency }}</td>
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
            <button type="button" [class.ativo]="fo.tipo === TipoOperacao.Compra" (click)="fo.tipo = TipoOperacao.Compra">Compra</button>
            <button type="button" [class.ativo]="fo.tipo === TipoOperacao.Venda" (click)="fo.tipo = TipoOperacao.Venda">Venda</button>
          </div>
          <div class="grade-campos">
            @if (editandoOpId === null) {
              <label>Código na B3
                <input name="ticker" [(ngModel)]="fo.ticker" required minlength="3" maxlength="12" placeholder="AOK" autocomplete="off"
                  class="maiusculas">
              </label>
              <label>Tipo do ativo
                <select name="tipoAtivo" [(ngModel)]="fo.tipoAtivo">
                  <option [ngValue]="null">Detectar pelo código</option>
                  @for (t of tiposAtivo; track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
                </select>
              </label>
            } @else {
              <p><span class="texto-suave">Ativo</span><br><strong>{{ fo.ticker }}</strong></p>
            }
          </div>
          <div class="grade-campos">
            <label>Data <input type="date" name="data" [(ngModel)]="fo.data" required [max]="hoje"></label>
            <label>Quantidade <input type="number" name="qtd" [(ngModel)]="fo.quantidade" required min="0.000001" step="any"></label>
            <label>Preço por cota (R$) <input type="number" name="preco" [(ngModel)]="fo.precoUnitario" required min="0" step="any"></label>
            <label>Taxas/corretagem (R$) <input type="number" name="taxas" [(ngModel)]="fo.taxas" min="0" step="0.01"></label>
          </div>
          <label>Observação <input name="obs" [(ngModel)]="fo.observacao" maxlength="300"></label>
          <p class="texto-suave">Total da operação: <strong>{{ totalOperacao(fo) | currency }}</strong></p>
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
          <div><span class="texto-suave">Preço médio</span><strong>{{ a.precoMedio | currency }}</strong></div>
          <div><span class="texto-suave">Cotação atual</span>
            <strong>{{ a.precoAtual !== null ? (a.precoAtual | currency) : '—' }}</strong></div>
          <div><span class="texto-suave">Cotas</span><strong>{{ qtd(a.quantidade) }}</strong></div>
          @if (a.ultimoProvento; as up) {
            <div><span class="texto-suave">Último provento (data-com {{ up.dataCom | date: 'dd/MM/yy' }})</span>
              <strong>{{ up.valorPorCota | currency: 'BRL' : 'symbol' : '1.2-4' }} por cota</strong></div>
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
                  <td class="direita">{{ o.precoUnitario | currency: 'BRL' : 'symbol' : '1.2-4' }}</td>
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
              @for (t of tiposAtivo; track t) { <option [ngValue]="t">{{ nomeTipo(t) }}</option> }
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
            <label>Cotação atual (R$) <input type="number" name="preco" [(ngModel)]="fa.precoAtual" min="0.0001" step="any" required></label>
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

    <!-- Importar do extrato -->
    <app-modal [aberto]="importacaoAberta()" titulo="Importar compras e vendas do extrato" (fechar)="importacaoAberta.set(false)">
      <div class="formulario">
        <p class="texto-suave">
          Transações com código de ativo na descrição (ex.: "COMPRA MXRF11 10 x 9,85"). Quando o extrato não traz quantidade
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
                  <tr [class.encerrada]="l.jaImportada">
                    <td><input type="checkbox" [(ngModel)]="l.selecionada" [disabled]="l.jaImportada"
                      [attr.aria-label]="'Importar ' + l.descricao"></td>
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
                    <td class="direita"><input type="number" class="campo-curto" [(ngModel)]="l.precoUnitario" min="0" step="any"
                      [disabled]="l.jaImportada">
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
  styles: `
    .barra-abas { margin-bottom: 16px; }
    .legenda-grafico { margin: -6px 0 10px; font-size: 0.84rem; }
    .cabecalho-bloco { display: flex; align-items: center; gap: 14px; margin-bottom: 10px; flex-wrap: wrap; }
    .cabecalho-bloco h2 { margin: 0; }
    .tabela-carteira td { white-space: nowrap; }
    .tabela-carteira td:first-child { white-space: normal; min-width: 160px; }
    tr.encerrada td { opacity: 0.6; }
    .nowrap { white-space: nowrap; }
    .nowrap .link + .link { margin-left: 10px; }
    .nota { font-size: 0.82rem; margin: 12px 2px 0; }
    .resumo-ativo { display: flex; gap: 24px; flex-wrap: wrap; margin-bottom: 14px; }
    .resumo-ativo div { display: grid; gap: 2px; }
    .resumo-ativo span { font-size: 0.8rem; }
    .maiusculas { text-transform: uppercase; }
    .barra-importacao { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
    .tabela-importacao td { vertical-align: top; }
    .campo-curto { width: 96px; padding: 4px 8px; }
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
  readonly carregandoSugestoes = signal(false);
  readonly linhasImportacao = signal<LinhaImportacao[]>([]);
  readonly quantidadeFiis = computed(() => this.abertas().filter((p) => p.tipo === TipoAtivo.Fii).length);

  formOp: SalvarOperacao | null = null;
  editandoOpId: number | null = null;
  formAtivo: AtualizarAtivo | null = null;
  ativoEditado: PosicaoAtivo | null = null;
  tokenBrapi = '';
  private atualizouAutomatico = false;
  private importouAutomatico = false;
  readonly importandoOpenFinance = signal(false);

  readonly abertas = computed(() => (this.dados()?.posicoes ?? []).filter((p) => p.quantidade > 0));
  readonly posicoesVisiveis = computed(() => (this.mostrarEncerradas() ? this.dados()?.posicoes ?? [] : this.abertas()));
  readonly alturaGraficoAtivos = computed(() => Math.max(220, this.abertas().length * 36));

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
              (p.resultadoTotalPercentual ?? 0) >= (p.resultadoSelicPercentual ?? 0) ? '#8fd5b4' : '#f5a3b0'),
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
    this.editandoOpId = null;
    this.formOp = {
      ticker, tipoAtivo: null, tipo: TipoOperacao.Compra, data: this.hoje, quantidade: null, precoUnitario: null,
      taxas: 0, observacao: null
    };
  }

  editarOperacao(o: OperacaoAtivo): void {
    this.editandoOpId = o.id;
    this.formOp = {
      ticker: o.ticker, tipoAtivo: null, tipo: o.tipo, data: o.data, quantidade: o.quantidade,
      precoUnitario: o.precoUnitario, taxas: o.taxas, observacao: o.observacao
    };
  }

  totalOperacao(f: SalvarOperacao): number {
    const bruto = (f.quantidade ?? 0) * (f.precoUnitario ?? 0);
    return f.tipo === TipoOperacao.Compra ? bruto + (f.taxas ?? 0) : bruto - (f.taxas ?? 0);
  }

  salvarOperacao(): void {
    const f = this.formOp;
    if (!f) return;
    const dto: SalvarOperacao = {
      ...f, ticker: f.ticker.trim().toUpperCase(), taxas: f.taxas ?? 0, observacao: f.observacao?.trim() || null
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

  private recarregarDepoisDeAlterar(): void {
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
