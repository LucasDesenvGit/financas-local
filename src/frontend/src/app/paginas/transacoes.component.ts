import { DatePipe } from '@angular/common';
import { Component, HostListener, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import {
  Categoria, Conta, FiltroTransacoes, OpcaoCategoria, OrigemConta, PaginaTransacoes, ResultadoCategoriasEmLote, StatusTransacao,
  SugestaoCategoria, TipoConta, TipoCorrespondencia, TipoLancamento, Transacao, TipoTransacao
} from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { ModalComponent } from '../compartilhado/modal.component';
import { PaginacaoComponent } from '../compartilhado/paginacao.component';
import { OpcaoMultipla, SeletorMultiploComponent } from '../compartilhado/seletor-multiplo.component';
import {
  ContextoTransacoes, NOMES_MESES, dataIso, dataIsoValida, paraData, paramsTransacoes, primeiroDiaMes, ultimoDiaMes
} from '../compartilhado/utilidades';
import { MoedaPipe } from '../compartilhado/moeda.pipe';
import { TermoComponent } from '../compartilhado/termo.component';
import { IconeComponent } from '../compartilhado/icone.component';
import { LogoBancoComponent } from '../compartilhado/logo-banco.component';
import { GRUPOS_LANCAMENTO, NAO_E_GASTO, TIPOS_LANCAMENTO } from '../compartilhado/tipos-lancamento';

interface FormTransacao {
  id: number | null;
  openFinance: boolean;
  contaId: number | null;
  data: string;
  descricao: string;
  valor: number | null;
  tipo: TipoTransacao;
  categoriaId: number | null;
  observacao: string;
  ignorarRelatorios: boolean;
  pendente: boolean;
  descricaoOriginal: string | null;
  emergencia: boolean;
}

/** Linha editável do assistente de sugestões. */
interface LinhaSugestao extends SugestaoCategoria { escolhida: number | null; criarRegra: boolean; aplicada: boolean }

interface FormTransferencia {
  contaOrigemId: number | null;
  contaDestinoId: number | null;
  valor: number | null;
  data: string;
  descricao: string;
}

interface ChipFiltro {
  /** Categoria tem um chip por valor marcado: `categoria:12`, `categoria:sem`. */
  id: 'periodo' | `categoria:${string}` | 'conta' | 'tipo' | 'lancamento' | 'emergencias' | 'generica' | 'busca';
  rotulo: string;
  cor?: string;
}

/** #87: categoria escolhida no modo edição: 'auto' (voltar para as regras) ou o id da categoria. */
type EscolhaCategoria = 'auto' | `${number}`;

/** "Criar regra para as próximas": várias transações iguais foram para a mesma categoria. */
interface SugestaoRegra { texto: string; categoriaId: number; categoriaNome: string; quantidade: number; }

/** "outubro/2026" quando é o mês cheio; senão "01/09 – 15/10" (com ano se o período cruza anos). */
function rotuloPeriodo(inicio?: string | null, fim?: string | null): string {
  if (!inicio || !fim) return inicio ? `a partir de ${dataCurta(inicio, true)}` : fim ? `até ${dataCurta(fim, true)}` : 'Todo o período';
  const [ai, mi] = inicio.split('-').map(Number);
  if (inicio === primeiroDiaMes(ai, mi) && fim === ultimoDiaMes(ai, mi)) return `${NOMES_MESES[mi - 1]}/${ai}`;
  const cruzaAno = inicio.substring(0, 4) !== fim.substring(0, 4);
  return `${dataCurta(inicio, cruzaAno)} – ${dataCurta(fim, cruzaAno)}`;
}

function dataCurta(iso: string, comAno: boolean): string {
  const [a, m, d] = iso.split('-');
  return comAno ? `${d}/${m}/${a}` : `${d}/${m}`;
}

@Component({
  selector: 'app-transacoes',
  imports: [FormsModule, MoedaPipe, DatePipe, ModalComponent, PaginacaoComponent, SeletorMultiploComponent, TermoComponent,
    IconeComponent, LogoBancoComponent],
  template: `
    <header class="cabecalho-pagina">
      <h1>Transações</h1>
      <div class="acoes">
        <button type="button" class="botao" [class.botao-ativo]="modoEdicao()" [attr.aria-pressed]="modoEdicao()"
          (click)="alternarModoEdicao()">✎ {{ modoEdicao() ? 'Sair da edição' : 'Editar categorias' }}</button>
        <button type="button" class="botao" (click)="abrirSugestoes()">✦ Sugestões de categoria</button>
        <button type="button" class="botao" (click)="abrirTransferencia()">⇄ Transferência</button>
        <button type="button" class="botao botao-primario" (click)="novo()">+ Lançamento</button>
      </div>
    </header>

    <section class="filtros">
      <label>De <input type="date" [(ngModel)]="filtro.inicio" (change)="buscar(1)"></label>
      <label>Até <input type="date" [(ngModel)]="filtro.fim" (change)="buscar(1)"></label>
      <label>Conta
        <select [(ngModel)]="filtro.contaId" (change)="buscar(1)">
          <option [ngValue]="null">Todas</option>
          @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
        </select>
      </label>
      <app-seletor-multiplo rotulo="Categoria" [opcoes]="opcoesCategoria()" [selecionadas]="filtroCategorias"
        (mudou)="aplicarFiltroCategorias($event)" />
      <label>Tipo
        <select [(ngModel)]="filtro.tipo" (change)="buscar(1)">
          <option [ngValue]="null">Todos</option>
          <option [ngValue]="TipoTransacao.Despesa">Despesas</option>
          <option [ngValue]="TipoTransacao.Receita">Receitas</option>
          <option [ngValue]="TipoTransacao.Transferencia">Transferências</option>
        </select>
      </label>
      <div class="filtro-lancamento">
        <label for="filtro-lancamento">Lançamento <app-termo chave="tipoLancamento" /></label>
        <select id="filtro-lancamento" name="lancamento" [(ngModel)]="filtro.lancamento" (change)="buscar(1)">
          <option [ngValue]="null">Todos</option>
          @for (g of gruposLancamento; track g.valor) { <option [ngValue]="g.valor">{{ g.rotulo }}</option> }
        </select>
      </div>
      <label class="checkbox"><input type="checkbox" [(ngModel)]="filtro.somenteEmergencias" (change)="buscar(1)">
        <span class="texto-emergencia">Só emergências</span></label>
      <span class="filtro-generica">
        <label class="checkbox"><input type="checkbox" name="poucoInformativa" [(ngModel)]="filtro.poucoInformativa" (change)="buscar(1)">
          Descrição pouco informativa</label>
        <app-termo chave="descricaoPoucoInformativa" />
      </span>
      <label class="cresce">Buscar
        <input type="search" [(ngModel)]="filtro.busca" (keyup.enter)="buscar(1)" placeholder="descrição, estabelecimento…" maxlength="100">
      </label>
    </section>

    @if (chipsFiltro(); as chips) {
      @if (chips.length) {
        <section class="faixa-filtros" aria-label="Filtros ativos">
          <strong>Filtrando:</strong>
          @for (c of chips; track c.id) {
            <span class="chip chip-filtro">
              @if (c.cor) { <span class="ponto" [style.background]="c.cor"></span> }
              {{ c.rotulo }}
              <button type="button" class="chip-remover" (click)="removerFiltro(c.id)"
                [attr.aria-label]="'Remover filtro: ' + c.rotulo" [title]="'Remover filtro: ' + c.rotulo">✕</button>
            </span>
          }
          <button type="button" class="link" (click)="limparFiltros()">Limpar filtros</button>
        </section>
      }
    }

    @if (filtroAplicado().poucoInformativa) {
      <p class="dica-generica texto-suave">
        Transações cuja descrição não diz o que foi (PIX ENVIADO, PAGAMENTO, TED, só o nome do banco). Cada uma sai desta
        lista quando você escolhe a categoria (dá para várias de uma vez em "Editar categorias"), dá um nome mais claro
        a ela ou marca "revisada".
      </p>
    }

    @if (modoEdicao()) {
      <section class="painel-bloco barra-edicao" aria-labelledby="titulo-edicao">
        <div class="bloco-titulo">
          <h2 id="titulo-edicao">Editando categorias</h2>
          <div class="acoes">
            <span role="status">{{ quantidadePendentes() ? quantidadePendentes() + ' alteração(ões) não salva(s)' : 'Nenhuma alteração pendente' }}</span>
            <button type="button" class="botao" [disabled]="!quantidadePendentes()" (click)="descartarPendentes()">Descartar</button>
            <button type="button" class="botao botao-primario" [disabled]="!quantidadePendentes() || salvando()"
              (click)="salvarPendentes()">Salvar alterações ({{ quantidadePendentes() }})</button>
          </div>
        </div>
        <p class="texto-suave pequeno-texto">
          Troque a categoria direto na lista ou marque várias linhas e use a barra de seleção. Nada é gravado até
          "Salvar alterações"; Esc numa linha desfaz a troca dela. Só a categoria muda por aqui.
          @if (!soSemCategoriaAtivo()) {
            <button type="button" class="link" (click)="soSemCategoria()">Ver só as sem categoria</button>
          }
        </p>
        @if (pagina(); as p) {
          @if (p.total > 0) {
            <div class="todas-do-filtro">
              <label>Todas as {{ p.total }} transações do filtro
                <select [(ngModel)]="categoriaTodas" name="categoriaTodas">
                  <option [ngValue]="null">— escolher categoria —</option>
                  <option ngValue="auto">Categorização automática (regras)</option>
                  @for (c of categorias(); track c.id) { <option [ngValue]="'' + c.id">{{ rotuloCategoria(c) }}</option> }
                </select>
              </label>
              <button type="button" class="botao" [disabled]="!categoriaTodas || salvando()" (click)="aplicarATodasDoFiltro()">
                Aplicar a todas as {{ p.total }}</button>
            </div>
          }
        }
        @for (r of sugestoesRegra(); track r.texto + r.categoriaId) {
          <p class="sugestao-regra" role="status">
            {{ r.quantidade }} transações “{{ r.texto }}” foram para {{ r.categoriaNome }}. Criar regra para as próximas?
            <button type="button" class="botao botao-pequeno" (click)="criarRegraSugerida(r)">Criar regra</button>
            <button type="button" class="link" (click)="dispensarRegra(r)">agora não</button>
          </p>
        }
      </section>
    }

    @if (pagina(); as p) {
      <section class="resumo-filtro">
        <span>{{ p.total }} transação(ões)</span>
        <span class="positivo">Receitas {{ p.totalReceitas | currency }}</span>
        <span class="negativo">Despesas {{ p.totalDespesas | currency }}</span>
        <span>Resultado <strong>{{ p.totalReceitas - p.totalDespesas | currency }}</strong></span>
      </section>

      @if (selecionadas().size) {
        <section class="barra-lote">
          <span>{{ selecionadas().size }} selecionada(s)</span>
          <select [(ngModel)]="categoriaLote">
            <option [ngValue]="null">Categorização automática</option>
            @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
          </select>
          <button type="button" class="botao botao-primario" (click)="categorizarLote()">
            {{ modoEdicao() ? 'Trocar nas selecionadas' : 'Aplicar categoria' }}</button>
          @if (filtroAplicado().poucoInformativa) {
            <button type="button" class="botao" (click)="marcarRevisadasSelecionadas()">Marcar como revisadas</button>
          }
          <button type="button" class="botao botao-emergencia" (click)="marcarEmergenciaLote(true)">Marcar emergência</button>
          <button type="button" class="botao" (click)="marcarEmergenciaLote(false)">Desmarcar emergência</button>
          <button type="button" class="link" (click)="limparSelecao()">limpar seleção</button>
        </section>
      }

      <div class="tabela-rolagem">
        <table class="tabela" [class.tabela-edicao]="modoEdicao()">
          <thead>
            <tr>
              <th class="col-check"><input type="checkbox" [checked]="todasSelecionadas()" (change)="alternarTodas()" aria-label="Selecionar todas"></th>
              <th>Data</th>
              <th>Descrição</th>
              <th>Categoria</th>
              <th>Conta</th>
              <th class="direita">Valor</th>
            </tr>
          </thead>
          <tbody>
            @for (t of p.itens; track t.id) {
              <tr (click)="modoEdicao() ? null : editar(t)" [class.clicavel]="!modoEdicao()" [class.ignorada]="t.ignorarRelatorios"
                [class.alterada]="pendentes().has(t.id)">
                <td class="col-check" (click)="$event.stopPropagation()">
                  <input type="checkbox" [checked]="selecionadas().has(t.id)" (change)="alternar(t.id)" aria-label="Selecionar">
                </td>
                <td class="nowrap">{{ t.data | date: 'dd/MM/yy' }}</td>
                <td>
                  <div class="truncar" [title]="t.descricao">{{ t.descricao }}</div>
                  <div class="detalhe">
                    <span class="etiqueta etiqueta-lancamento"><app-icone [icone]="iconeLancamento(t)" [tamanho]="13" />{{ rotuloLancamento(t) }}</span>
                    @if (foraDosGastos(t)) {
                      <span class="etiqueta etiqueta-suave">fora dos gastos</span>
                    } @else if (somandoNosTotais(t); as onde) {
                      <span class="etiqueta etiqueta-alerta">somando {{ onde === 'gastos' ? 'nos gastos' : 'nas receitas' }}</span>
                    }
                    @if (t.parcelaTotal && t.parcelaTotal > 1) { <span class="etiqueta">{{ t.parcelaNumero }}/{{ t.parcelaTotal }}</span> }
                    @if (t.status === StatusTransacao.Pendente) { <span class="etiqueta etiqueta-alerta">pendente</span> }
                    @if (t.emergencia) { <span class="etiqueta etiqueta-emergencia">emergência</span> }
                    @if (t.openFinance) { <span class="etiqueta">open finance</span> }
                    @if (t.observacao) { <span class="texto-suave truncar">{{ t.observacao }}</span> }
                    @if (filtroAplicado().poucoInformativa) {
                      <button type="button" class="link botao-revisada" (click)="$event.stopPropagation(); marcarRevisadas([t.id])"
                        [attr.aria-label]="'Marcar como revisada: ' + t.descricao">✓ marcar como revisada</button>
                    }
                  </div>
                </td>
                <td>
                  @if (modoEdicao()) {
                    <div class="celula-edicao">
                      <select class="select-categoria" [ngModel]="valorCategoria(t)" (ngModelChange)="mudarCategoria(t, $event)"
                        (keydown.escape)="desfazerLinha(t)" [attr.aria-label]="'Categoria de ' + t.descricao + ', ' + (t.data | date: 'dd/MM')">
                        @if (t.categoriaId === null) { <option value="" disabled>Sem categoria</option> }
                        <option value="auto">Automática (regras)</option>
                        @if (t.categoriaId !== null && !idsCategoriasOferecidas().has(t.categoriaId)) {
                          <option [value]="'' + t.categoriaId">{{ t.categoriaNome }} (inativa)</option>
                        }
                        @for (c of categorias(); track c.id) { <option [value]="'' + c.id">{{ rotuloCategoria(c) }}</option> }
                      </select>
                      @if (pendentes().has(t.id)) { <span class="etiqueta etiqueta-alerta">alterada</span> }
                    </div>
                  } @else {
                    @if (t.categoriaNome) {
                      <span class="chip"><span class="ponto" [style.background]="t.categoriaCor"></span>{{ t.categoriaNome }}</span>
                    } @else { <span class="texto-suave">—</span> }
                  }
                </td>
                <td class="texto-suave">
                  <span class="conta-transacao">
                    <app-logo-banco [codigo]="contaDe(t)?.banco" [tamanho]="20" [cartao]="ehDeCartao(t)" />
                    <span>{{ t.contaNome }} @if (ehDeCartao(t)) {<small class="tipo-conta">cartão de crédito</small>}</span>
                  </span>
                </td>
                <td class="direita valor nowrap" [class.positivo]="t.valor > 0" [class.negativo]="t.valor < 0">{{ t.valor | currency }}</td>
              </tr>
            } @empty {
              <tr><td colspan="6" class="vazio">Nenhuma transação encontrada para os filtros.</td></tr>
            }
          </tbody>
        </table>
      </div>

      <app-paginacao [pagina]="p.pagina" [tamanho]="p.tamanhoPagina" [total]="p.total"
        (paginaMudou)="buscar($event)" (tamanhoMudou)="mudarTamanhoPagina($event)" />
    }

    <app-modal [aberto]="form !== null" [titulo]="form?.id ? 'Editar transação' : 'Novo lançamento'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          @if (fm.openFinance) {
            <p class="aviso-inline">Transação importada do Open Finance: valor, data e conta vêm do banco.
              @if (fm.descricaoOriginal) { <br><small>Original: {{ fm.descricaoOriginal }}</small> }
            </p>
          } @else {
            <div class="grade-campos">
              <label>Tipo
                <select name="tipo" [(ngModel)]="fm.tipo">
                  <option [ngValue]="TipoTransacao.Despesa">Despesa</option>
                  <option [ngValue]="TipoTransacao.Receita">Receita</option>
                </select>
              </label>
              <label>Conta
                <select name="conta" [(ngModel)]="fm.contaId" required>
                  @for (c of contasManuais(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
                </select>
              </label>
              <label>Data <input type="date" name="data" [(ngModel)]="fm.data" required></label>
              <label>Valor (R$) <input type="number" name="valor" [(ngModel)]="fm.valor" required min="0.01" step="0.01"></label>
            </div>
            @if (!contasManuais().length) {
              <p class="erro">Crie uma conta manual (ex.: "Carteira") em Contas e cartões para lançar manualmente.</p>
            }
          }
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="300"></label>
          <label>Categoria
            <select name="categoria" [(ngModel)]="fm.categoriaId">
              <option [ngValue]="null">Automática (regras)</option>
              @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
            </select>
          </label>
          @if (sugestoesForm().length) {
            <div class="sugestoes-chips">
              <span class="texto-suave">Sugestões:</span>
              @for (o of sugestoesForm(); track o.categoriaId) {
                <button type="button" class="chip-sugestao" [title]="o.motivo" (click)="fm.categoriaId = o.categoriaId">
                  <span class="ponto" [style.background]="o.cor"></span>{{ o.nome }}
                </button>
              }
            </div>
          }
          <label>Observação <textarea name="observacao" [(ngModel)]="fm.observacao" rows="2" maxlength="1000"></textarea></label>
          <label class="checkbox"><input type="checkbox" name="ignorar" [(ngModel)]="fm.ignorarRelatorios"> Ignorar nos relatórios e totais</label>
          @if (fm.tipo === TipoTransacao.Despesa) {
            <label class="checkbox"><input type="checkbox" name="emergencia" [(ngModel)]="fm.emergencia">
              <span class="texto-emergencia">Gasto de emergência</span>
              <small class="texto-suave">(destacado em laranja nos orçamentos e relatórios)</small></label>
          }
          @if (!fm.openFinance) {
            <label class="checkbox"><input type="checkbox" name="pendente" [(ngModel)]="fm.pendente"> Pendente (ainda não efetivada)</label>
          }
          <div class="rodape-form">
            @if (fm.id && !fm.openFinance) {
              <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button>
            }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || salvando()">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="sugestoesAbertas()" titulo="Sugestões de categoria" (fechar)="fecharSugestoes()">
      <div class="formulario">
        <div class="linha-sugestoes">
          <div class="segmentado" role="group" aria-label="O que revisar">
            <button type="button" [class.ativo]="somenteSemCategoria()" (click)="trocarModoSugestoes(true)">Sem categoria</button>
            <button type="button" [class.ativo]="!somenteSemCategoria()" (click)="trocarModoSugestoes(false)">Revisar automáticas</button>
          </div>
          <span class="cresce"></span>
          <button type="button" class="botao botao-suave" (click)="aplicarConfiancaAlta()" [disabled]="!quantidadeAlta() || salvando()">
            Aplicar as {{ quantidadeAlta() }} de confiança alta</button>
        </div>
        <p class="texto-suave">
          Transações parecidas foram agrupadas pela descrição. A sugestão vem de como você categorizou transações iguais
          antes (suas escolhas manuais pesam mais) e de palavras-chave conhecidas. Marque "criar regra" para que as próximas
          sejam categorizadas sozinhas.
        </p>
        @if (carregandoSugestoes()) {
          <p class="vazio">Analisando suas transações…</p>
        } @else {
          <div class="tabela-rolagem">
            <table class="tabela tabela-sugestoes">
              <thead>
                <tr><th>Transações</th><th class="direita">Qtd.</th><th class="direita">Total</th><th>Sugestão</th>
                  <th>Categoria</th><th>Regra</th><th></th></tr>
              </thead>
              <tbody>
                @for (l of linhasSugestao(); track l.chave) {
                  <tr [class.encerrada]="l.aplicada">
                    <td><div class="truncar" [title]="l.exemplo">{{ l.exemplo }}</div>
                      <small class="texto-suave">{{ l.chave }}</small></td>
                    <td class="direita">{{ l.quantidade }}</td>
                    <td class="direita nowrap" [class.negativo]="l.total < 0" [class.positivo]="l.total > 0">{{ l.total | currency }}</td>
                    <td>
                      <span [class]="'confianca confianca-' + l.confianca">{{ nomeConfianca(l.confianca) }}</span>
                      @if (l.motivo) { <br><small class="texto-suave">{{ l.motivo }}</small> }
                      @if (l.alternativas.length) {
                        <div class="sugestoes-chips">
                          @for (a of l.alternativas; track a.categoriaId) {
                            <button type="button" class="chip-sugestao" [title]="a.motivo" (click)="l.escolhida = a.categoriaId">
                              <span class="ponto" [style.background]="a.cor"></span>{{ a.nome }}</button>
                          }
                        </div>
                      }
                    </td>
                    <td>
                      <select [(ngModel)]="l.escolhida" [disabled]="l.aplicada" [attr.aria-label]="'Categoria para ' + l.exemplo">
                        <option [ngValue]="null">— escolher —</option>
                        @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
                      </select>
                    </td>
                    <td><label class="checkbox"><input type="checkbox" [(ngModel)]="l.criarRegra" [disabled]="l.aplicada">
                      <small>contém "{{ l.textoRegra }}"</small></label></td>
                    <td>
                      @if (l.aplicada) {
                        <span class="etiqueta etiqueta-sucesso">aplicada</span>
                      } @else {
                        <button type="button" class="botao botao-primario botao-pequeno" [disabled]="!l.escolhida || salvando()"
                          (click)="aplicarSugestao(l)">Aplicar</button>
                      }
                    </td>
                  </tr>
                } @empty {
                  <tr><td colspan="7" class="vazio">Nada para sugerir.</td></tr>
                }
              </tbody>
            </table>
          </div>
        }
      </div>
    </app-modal>

    <app-modal [aberto]="transferencia !== null" titulo="Transferência entre contas" (fechar)="transferencia = null">
      @if (transferencia; as tf) {
        <form (ngSubmit)="salvarTransferencia()" #ft="ngForm" class="formulario">
          <p class="texto-suave">Os lançamentos são criados apenas nas contas manuais. O lado do Open Finance chega pela sincronização.</p>
          <div class="grade-campos">
            <label>De
              <select name="origem" [(ngModel)]="tf.contaOrigemId" required>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Para
              <select name="destino" [(ngModel)]="tf.contaDestinoId" required>
                @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
            <label>Data <input type="date" name="data" [(ngModel)]="tf.data" required></label>
            <label>Valor (R$) <input type="number" name="valor" [(ngModel)]="tf.valor" required min="0.01" step="0.01"></label>
          </div>
          <label>Descrição <input name="descricao" [(ngModel)]="tf.descricao" maxlength="200" placeholder="Transferência"></label>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="transferencia = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="ft.invalid || salvando()">Transferir</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  styles: `
    .faixa-filtros {
      display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px;
      margin: -4px 4px 14px; font-size: 0.9rem;
    }
    .chip-filtro { gap: 2px; padding-right: 2px; background: var(--primaria-suave); color: var(--primaria-forte); font-weight: 550; }
    .chip-remover {
      display: inline-grid; place-items: center; width: 24px; height: 24px; margin-left: 2px;
      border: 0; border-radius: 50%; background: none; color: inherit; cursor: pointer; font-size: 0.8rem; line-height: 1;
    }
    .chip-remover:hover, .chip-remover:focus-visible { background: var(--superficie); }
    .linha-sugestoes { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .tabela-sugestoes td { vertical-align: top; }
    .tabela-sugestoes td:first-child { max-width: 280px; }
    .tabela-sugestoes select { min-width: 180px; }
    tr.encerrada td { opacity: 0.55; }
    .barra-edicao { margin-bottom: 14px; }
    .barra-edicao .bloco-titulo { flex-wrap: wrap; gap: 10px; }
    .barra-edicao p { margin: 6px 0; }
    .todas-do-filtro { display: flex; flex-wrap: wrap; gap: 10px; align-items: flex-end; margin-top: 8px; }
    .todas-do-filtro label { min-width: 240px; }
    .sugestao-regra { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; padding: 8px 10px;
      border-radius: var(--raio-pequeno); background: var(--primaria-suave); }
    .celula-edicao { display: flex; gap: 6px; align-items: center; }
    .select-categoria { min-width: 190px; min-height: 32px; padding: 5px 8px; }
    tr.alterada td { background: var(--alerta-suave); }
    .botao-ativo { background: var(--primaria-suave); color: var(--primaria-forte); }
    .filtro-generica { display: inline-flex; align-items: center; gap: 2px; padding-bottom: 8px; }
    .filtro-lancamento { display: grid; gap: 6px; min-width: 180px; }
    .filtro-lancamento label { display: flex; align-items: center; gap: 2px; min-width: 0; font-size: 0.85rem; font-weight: 600; color: var(--texto-suave); }
    .etiqueta-lancamento { display: inline-flex; align-items: center; gap: 4px; background: var(--primaria-suave); color: var(--primaria-forte); }
    .conta-transacao { display: inline-flex; align-items: center; gap: 8px; }
    .conta-transacao .tipo-conta { display: block; font-size: 0.75rem; }
    .filtro-generica label.checkbox { min-width: 0; padding-bottom: 0; }
    .dica-generica { margin: -4px 4px 14px; font-size: 0.88rem; max-width: 90ch; }
    .botao-revisada { min-height: 24px; padding: 0 4px; font-size: 0.82rem; }
    /* Celular no modo edição: cada transação vira um bloco, com o seletor de categoria em largura total (sem rolagem lateral). */
    @media (max-width: 760px) {
      .tabela-edicao thead { display: none; }
      .tabela-edicao tbody tr {
        display: grid; grid-template-columns: 28px minmax(0, 1fr) auto; gap: 4px 10px; align-items: center;
        grid-template-areas: "check desc valor" "check data conta" "check cat cat";
        padding: 10px 12px; border-bottom: 1px solid var(--linha);
      }
      .tabela-edicao tbody td { padding: 0; border: 0; }
      .tabela-edicao tbody tr.alterada { background: var(--alerta-suave); }
      .tabela-edicao tbody tr.alterada td { background: none; }
      .tabela-edicao tbody td:nth-child(1) { grid-area: check; }
      .tabela-edicao tbody td:nth-child(2) { grid-area: data; font-size: 0.8rem; color: var(--texto-suave); }
      .tabela-edicao tbody td:nth-child(3) { grid-area: desc; min-width: 0; }
      .tabela-edicao tbody td:nth-child(4) { grid-area: cat; }
      .tabela-edicao tbody td:nth-child(5) { grid-area: conta; font-size: 0.8rem; text-align: right; }
      .tabela-edicao tbody td:nth-child(6) { grid-area: valor; }
      .tabela-edicao tbody td.vazio { grid-column: 1 / -1; grid-area: auto; }
      .tabela-edicao .celula-edicao { width: 100%; }
      .tabela-edicao .select-categoria { flex: 1; min-width: 0; }
    }
    :host ::ng-deep .modal:has(.tabela-sugestoes) { width: min(1150px, 96vw); }
  `
})
export class TransacoesComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly TipoTransacao = TipoTransacao;
  readonly StatusTransacao = StatusTransacao;

  readonly contas = signal<Conta[]>([]);
  /** Todas as categorias (inclusive inativas), para validar a categoria que chega pela URL. */
  readonly todasCategorias = signal<Categoria[]>([]);
  /** Categorias oferecidas nos selects: as ativas e, se for o caso, as inativas que estão no filtro. */
  readonly categorias = computed(() =>
    this.todasCategorias().filter((c) => c.ativa || this.categoriasFiltradas().includes(String(c.id))));
  private readonly categoriasFiltradas = signal<string[]>([]);
  /** Opções do filtro de categorias: "Sem categoria" e cada categoria seguida das subcategorias (recuadas). */
  readonly opcoesCategoria = computed<OpcaoMultipla[]>(() => {
    const lista = this.categorias();
    const porNome = (a: Categoria, b: Categoria) => a.nome.localeCompare(b.nome, 'pt-BR');
    const ids = new Set(lista.map((c) => c.id));
    const raizes = lista.filter((c) => !c.categoriaPaiId || !ids.has(c.categoriaPaiId)).sort(porNome);
    const opcoes: OpcaoMultipla[] = [{ valor: 'sem', rotulo: 'Sem categoria' }];
    for (const r of raizes) {
      opcoes.push({ valor: String(r.id), rotulo: this.rotuloCategoria(r), cor: r.cor });
      for (const f of lista.filter((c) => c.categoriaPaiId === r.id).sort(porNome)) {
        opcoes.push({ valor: String(f.id), rotulo: f.nome, cor: f.cor, nivel: 1 });
      }
    }
    return opcoes;
  });
  readonly pagina = signal<PaginaTransacoes | null>(null);
  readonly selecionadas = signal<Set<number>>(new Set());
  readonly salvando = signal(false);
  readonly sugestoesForm = signal<OpcaoCategoria[]>([]);
  readonly sugestoesAbertas = signal(false);
  readonly carregandoSugestoes = signal(false);
  readonly somenteSemCategoria = signal(true);
  readonly linhasSugestao = signal<LinhaSugestao[]>([]);
  private sugestoesAlteraram = false;

  readonly contasManuais = computed(() => this.contas().filter((c) => c.origem === OrigemConta.Manual));
  readonly totalPaginas = computed(() => {
    const p = this.pagina();
    return p ? Math.max(1, Math.ceil(p.total / p.tamanhoPagina)) : 1;
  });
  readonly todasSelecionadas = computed(() => {
    const itens = this.pagina()?.itens ?? [];
    return itens.length > 0 && itens.every((t) => this.selecionadas().has(t.id));
  });

  /* ------------------------------ #87: modo edição ------------------------------ */
  /** Modo edição de categorias (?editar=1). */
  readonly modoEdicao = signal(false);
  /** Trocas de categoria ainda não gravadas: id da transação → escolha. */
  readonly pendentes = signal<Map<number, EscolhaCategoria>>(new Map());
  readonly quantidadePendentes = computed(() => this.pendentes().size);
  readonly idsCategoriasOferecidas = computed(() => new Set(this.categorias().map((c) => c.id)));
  readonly sugestoesRegra = signal<SugestaoRegra[]>([]);
  readonly soSemCategoriaAtivo = computed(() => {
    const f = this.filtroAplicado();
    return !!f.semCategoria && !(f.categoriaIds ?? []).length;
  });
  /** Escolha para "todas do filtro" (null = nada escolhido). */
  categoriaTodas: EscolhaCategoria | null = null;

  filtro: FiltroTransacoes = this.filtroInicial();
  /** Categorias marcadas no filtro: 'sem' ou o id, na ordem em que foram marcadas. */
  filtroCategorias: string[] = [];
  /** Filtro da última busca (o que está na tela), base dos chips de "Filtrando:". */
  readonly filtroAplicado = signal<FiltroTransacoes>(this.filtroInicial());

  /**
   * UX-2: um chip para cada filtro diferente do padrão (mês atual, sem filtros), para quem chega por um link
   * saber que a lista está filtrada. Nielsen 1 (visibilidade do estado) e 3 (controle: ✕ e "Limpar filtros").
   */
  readonly chipsFiltro = computed<ChipFiltro[]>(() => {
    const f = this.filtroAplicado();
    const padrao = this.filtroInicial();
    const chips: ChipFiltro[] = [];
    if (f.inicio !== padrao.inicio || f.fim !== padrao.fim) chips.push({ id: 'periodo', rotulo: rotuloPeriodo(f.inicio, f.fim) });
    if (f.semCategoria) chips.push({ id: 'categoria:sem', rotulo: 'Sem categoria' });
    for (const id of f.categoriaIds ?? []) {
      const c = this.todasCategorias().find((x) => x.id === id);
      chips.push({ id: `categoria:${id}`, rotulo: c ? this.rotuloCategoria(c) : 'Categoria', cor: c?.cor });
    }
    if (f.contaId) chips.push({ id: 'conta', rotulo: this.contas().find((c) => c.id === f.contaId)?.nome ?? 'Conta' });
    if (f.tipo) {
      const nomes: Record<number, string> = {
        [TipoTransacao.Despesa]: 'Despesas', [TipoTransacao.Receita]: 'Receitas', [TipoTransacao.Transferencia]: 'Transferências'
      };
      chips.push({ id: 'tipo', rotulo: nomes[f.tipo] });
    }
    if (f.lancamento) chips.push({ id: 'lancamento', rotulo: GRUPOS_LANCAMENTO.find((g) => g.valor === f.lancamento)?.rotulo ?? 'Lançamento' });
    if (f.somenteEmergencias) chips.push({ id: 'emergencias', rotulo: 'Só emergências' });
    if (f.poucoInformativa) chips.push({ id: 'generica', rotulo: 'Descrição pouco informativa' });
    if (f.busca?.trim()) chips.push({ id: 'busca', rotulo: `“${f.busca.trim()}”` });
    return chips;
  });

  removerFiltro(id: ChipFiltro['id']): void {
    const padrao = this.filtroInicial();
    if (id.startsWith('categoria:')) {
      const valor = id.substring('categoria:'.length);
      this.filtroCategorias = this.filtroCategorias.filter((v) => v !== valor);
      this.sincronizarFiltroCategoria();
    }
    switch (id) {
      case 'periodo': this.filtro.inicio = padrao.inicio; this.filtro.fim = padrao.fim; break;
      case 'conta': this.filtro.contaId = null; break;
      case 'tipo': this.filtro.tipo = null; break;
      case 'emergencias': this.filtro.somenteEmergencias = false; break;
      case 'generica': this.filtro.poucoInformativa = false; break;
      case 'lancamento': this.filtro.lancamento = null; break;
      case 'busca': this.filtro.busca = ''; break;
    }
    this.buscar(1);
  }

  limparFiltros(): void {
    this.filtro = this.filtroInicial();
    this.filtroCategorias = [];
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }
  categoriaLote: number | null = null;
  form: FormTransacao | null = null;
  transferencia: FormTransferencia | null = null;

  ngOnInit(): void {
    const params = this.rota.snapshot.queryParamMap;
    // Categoria/conta da URL só são aplicadas depois de conferir que existem; por isso, quando vierem,
    // a primeira busca espera as listas (uma requisição só, sem a lista piscar).
    const precisaListas = params.has('categoria') || params.has('conta');
    forkJoin({ contas: this.api.contas(), categorias: this.api.categorias() }).subscribe({
      next: ({ contas, categorias }) => {
        this.contas.set(contas);
        this.todasCategorias.set(categorias);
        if (precisaListas) {
          this.aplicarParametrosDaUrl(params);
          this.buscar(paginaDaUrl(params));
        }
      },
      error: (e) => {
        this.avisos.erro(mensagemDeErro(e));
        if (precisaListas) this.buscar(paginaDaUrl(params));
      }
    });
    if (!precisaListas) {
      this.aplicarParametrosDaUrl(params);
      this.buscar(paginaDaUrl(params));
    }
  }

  /**
   * UX-1: filtro vindo de outra tela (?categoria=12|sem&inicio=yyyy-MM-dd&fim=yyyy-MM-dd&tipo=despesa|receita&conta=3).
   * Várias categorias vêm separadas por vírgula (?categoria=12,20,sem).
   * UX-5: e o que a própria tela grava na URL (&emergencias=1&busca=texto&pagina=2), para o F5 e o link salvo.
   * Cada parâmetro é validado sozinho: um inválido é ignorado e os outros continuam valendo.
   */
  private aplicarParametrosDaUrl(params: ParamMap): void {
    const inicio = params.get('inicio');
    const fim = params.get('fim');
    const inicioOk = dataIsoValida(inicio);
    const fimOk = dataIsoValida(fim);
    if (inicioOk) this.filtro.inicio = inicio;
    if (fimOk) this.filtro.fim = fim;
    // Só uma das datas veio válida: a outra acompanha o mês dela, para não sobrar um período invertido (lista vazia).
    if (inicioOk !== fimOk && this.filtro.inicio! > this.filtro.fim!) {
      const base = paraData(inicioOk ? this.filtro.inicio! : this.filtro.fim!);
      if (inicioOk) this.filtro.fim = ultimoDiaMes(base.getFullYear(), base.getMonth() + 1);
      else this.filtro.inicio = primeiroDiaMes(base.getFullYear(), base.getMonth() + 1);
    }

    const tipos: Record<string, TipoTransacao> = {
      despesa: TipoTransacao.Despesa, receita: TipoTransacao.Receita, transferencia: TipoTransacao.Transferencia
    };
    const tipo = params.get('tipo');
    if (tipo && tipos[tipo]) this.filtro.tipo = tipos[tipo];

    const validas = new Set(['sem', ...this.todasCategorias().map((c) => String(c.id))]);
    const categorias = (params.get('categoria') ?? '').split(',').map((v) => v.trim()).filter((v) => validas.has(v));
    this.filtroCategorias = [...new Set(categorias)];
    this.sincronizarFiltroCategoria();

    const conta = Number(params.get('conta'));
    if (conta && this.contas().some((c) => c.id === conta)) this.filtro.contaId = conta;

    if (params.get('emergencias') === '1') this.filtro.somenteEmergencias = true;
    if (params.get('generica') === '1') this.filtro.poucoInformativa = true;
    const lancamento = params.get('lancamento');
    if (lancamento && GRUPOS_LANCAMENTO.some((g) => g.valor === lancamento)) this.filtro.lancamento = lancamento;
    if (params.get('editar') === '1') this.modoEdicao.set(true);
    const busca = params.get('busca')?.trim();
    if (busca) this.filtro.busca = busca.slice(0, 100);
  }

  rotuloCategoria(c: Categoria): string {
    if (!c.categoriaPaiId) return c.nome;
    const pai = this.todasCategorias().find((x) => x.id === c.categoriaPaiId);
    return pai ? `${pai.nome} › ${c.nome}` : c.nome;
  }

  aplicarFiltroCategorias(valores: string[]): void {
    this.filtroCategorias = valores;
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }

  /** Leva as categorias marcadas ('sem' | id) para o filtro da API. */
  private sincronizarFiltroCategoria(): void {
    this.filtro.semCategoria = this.filtroCategorias.includes('sem');
    this.filtro.categoriaIds = this.filtroCategorias.filter((v) => v !== 'sem').map(Number);
    this.filtro.categoriaId = null;
    this.categoriasFiltradas.set(this.filtroCategorias);
  }

  buscar(pagina: number): void {
    // #87: trocar de página ou de filtro perde as trocas não salvas; pergunta antes (Nielsen 5).
    if (this.pendentes().size) {
      if (!confirm(`Descartar ${this.pendentes().size} alteração(ões) de categoria não salva(s)?`)) {
        this.voltarAoFiltroAplicado();
        return;
      }
      this.pendentes.set(new Map());
    }
    this.filtro.pagina = pagina;
    this.filtroAplicado.set({ ...this.filtro });
    this.gravarFiltroNaUrl();
    this.api.transacoes(this.filtro).subscribe({
      next: (p) => {
        // Página da URL que não existe mais (link antigo, menos transações agora): vai para a última que existe.
        const ultima = Math.max(1, Math.ceil(p.total / p.tamanhoPagina));
        if (!p.itens.length && p.pagina > ultima) {
          this.buscar(ultima);
          return;
        }
        const trocouPagina = this.pagina() !== null && this.pagina()!.pagina !== p.pagina;
        this.pagina.set(p);
        this.limparSelecao();
        if (trocouPagina) this.rolarParaTopo();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /**
   * UX-5: o filtro da busca vai para a URL (só o que difere do padrão), para o F5 e o link salvo manterem a lista.
   * replaceUrl: o voltar do navegador leva à tela anterior, não ao filtro anterior (julgamento próprio; Nielsen 4).
   */
  private gravarFiltroNaUrl(): void {
    const f = this.filtro;
    const padrao = this.filtroInicial();
    const tipos: Record<number, ContextoTransacoes['tipo']> = {
      [TipoTransacao.Despesa]: 'despesa', [TipoTransacao.Receita]: 'receita', [TipoTransacao.Transferencia]: 'transferencia'
    };
    const mudouPeriodo = f.inicio !== padrao.inicio || f.fim !== padrao.fim;
    const params: Record<string, string> = paramsTransacoes({
      inicio: mudouPeriodo ? f.inicio ?? undefined : undefined,
      fim: mudouPeriodo ? f.fim ?? undefined : undefined,
      tipo: f.tipo ? tipos[f.tipo] : undefined,
      contaId: f.contaId ?? undefined
    });
    if (this.filtroCategorias.length) params['categoria'] = this.filtroCategorias.join(',');
    if (f.somenteEmergencias) params['emergencias'] = '1';
    if (f.poucoInformativa) params['generica'] = '1';
    if (f.lancamento) params['lancamento'] = f.lancamento;
    if (f.busca?.trim()) params['busca'] = f.busca.trim();
    if ((f.pagina ?? 1) > 1) params['pagina'] = String(f.pagina);
    if (this.modoEdicao()) params['editar'] = '1';
    this.router.navigate([], { relativeTo: this.rota, queryParams: params, replaceUrl: true });
  }

  mudarTamanhoPagina(tamanho: number): void {
    this.filtro.tamanhoPagina = tamanho;
    try { localStorage.setItem('financas.tamanhoPagina', String(tamanho)); } catch { /* ignora */ }
    this.buscar(1);
  }

  /** Volta ao topo da lista ao trocar de página. */
  private rolarParaTopo(): void {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  alternar(id: number): void {
    this.selecionadas.update((s) => {
      const novo = new Set(s);
      if (novo.has(id)) novo.delete(id); else novo.add(id);
      return novo;
    });
  }

  alternarTodas(): void {
    const itens = this.pagina()?.itens ?? [];
    this.selecionadas.set(this.todasSelecionadas() ? new Set() : new Set(itens.map((t) => t.id)));
  }

  limparSelecao(): void {
    this.selecionadas.set(new Set());
  }

  categorizarLote(): void {
    if (this.modoEdicao()) {
      // No modo edição a barra de seleção só marca a troca; grava junto com as outras em "Salvar alterações".
      const escolha: EscolhaCategoria = this.categoriaLote === null ? 'auto' : `${this.categoriaLote}`;
      const itens = (this.pagina()?.itens ?? []).filter((t) => this.selecionadas().has(t.id));
      for (const t of itens) this.mudarCategoria(t, escolha);
      this.avisos.info(`${itens.length} troca(s) marcada(s). Salve para gravar.`);
      this.limparSelecao();
      return;
    }
    this.api.categorizarEmLote([...this.selecionadas()], this.categoriaLote).subscribe({
      next: (r) => {
        this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) atualizada(s).`);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  marcarEmergenciaLote(emergencia: boolean): void {
    this.api.marcarEmergencia([...this.selecionadas()], emergencia).subscribe({
      next: (r) => {
        this.avisos.sucesso(emergencia
          ? `${r.transacoesAtualizadas} despesa(s) marcada(s) como emergência.`
          : `${r.transacoesAtualizadas} transação(ões) desmarcada(s).`);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  marcarRevisadasSelecionadas(): void {
    this.marcarRevisadas([...this.selecionadas()]);
  }

  /* ------------------------------ #111: tipo de lançamento ------------------------------ */
  readonly gruposLancamento = GRUPOS_LANCAMENTO;

  rotuloLancamento(t: Transacao): string {
    return TIPOS_LANCAMENTO[t.tipoLancamento]?.rotulo ?? 'Outro';
  }

  iconeLancamento(t: Transacao) {
    return (TIPOS_LANCAMENTO[t.tipoLancamento] ?? TIPOS_LANCAMENTO[TipoLancamento.Outro]).icone;
  }

  /** Não entra nas despesas nem nas receitas (transferência ou "ignorar nos relatórios"). */
  foraDosGastos(t: Transacao): boolean {
    return t.tipo === TipoTransacao.Transferencia || t.ignorarRelatorios;
  }

  /**
   * Pagamento de fatura ou transferência entre contas suas que está entrando nos totais: na saída, conta o gasto em
   * dobro (as compras já estão no cartão); na entrada (o crédito do pagamento no cartão), infla as receitas.
   */
  somandoNosTotais(t: Transacao): 'gastos' | 'receitas' | null {
    if (!NAO_E_GASTO.has(t.tipoLancamento) || t.tipo === TipoTransacao.Transferencia || t.ignorarRelatorios) return null;
    return t.valor < 0 ? 'gastos' : 'receitas';
  }

  contaDe(t: Transacao): Conta | undefined {
    return this.contas().find((c) => c.id === t.contaId);
  }

  ehDeCartao(t: Transacao): boolean {
    return this.contaDe(t)?.tipo === TipoConta.CartaoCredito;
  }

  /** #9: revisada sai do filtro "Descrição pouco informativa"; o aviso permite desfazer. */
  marcarRevisadas(ids: number[]): void {
    if (!ids.length) return;
    this.api.marcarRevisada(ids, true).subscribe({
      next: (r) => {
        this.limparSelecao();
        this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) marcada(s) como revisada(s).`, {
          rotulo: 'Desfazer',
          executar: () => this.api.marcarRevisada(ids, false).subscribe({
            next: () => this.buscar(this.filtro.pagina ?? 1),
            error: (e) => this.avisos.erro(mensagemDeErro(e))
          })
        });
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ #87: modo edição ------------------------------ */

  alternarModoEdicao(): void {
    if (this.modoEdicao() && this.pendentes().size
      && !confirm(`Sair da edição e descartar ${this.pendentes().size} alteração(ões) não salva(s)?`)) return;
    this.pendentes.set(new Map());
    this.sugestoesRegra.set([]);
    this.categoriaTodas = null;
    this.modoEdicao.update((v) => !v);
    this.gravarFiltroNaUrl();
  }

  /** Valor do select da linha: a troca pendente ou a categoria atual ('' = sem categoria). */
  valorCategoria(t: Transacao): string {
    return this.pendentes().get(t.id) ?? (t.categoriaId === null ? '' : String(t.categoriaId));
  }

  mudarCategoria(t: Transacao, valor: string): void {
    const atual = t.categoriaId === null ? '' : String(t.categoriaId);
    this.pendentes.update((m) => {
      const novo = new Map(m);
      // Voltou para o que já estava gravado: não há o que salvar nessa linha.
      if (valor === atual || valor === '') novo.delete(t.id);
      else novo.set(t.id, valor as EscolhaCategoria);
      return novo;
    });
  }

  desfazerLinha(t: Transacao): void {
    this.pendentes.update((m) => {
      const novo = new Map(m);
      novo.delete(t.id);
      return novo;
    });
  }

  descartarPendentes(): void {
    if (!this.pendentes().size || !confirm(`Descartar ${this.pendentes().size} alteração(ões)?`)) return;
    this.pendentes.set(new Map());
  }

  @HostListener('window:beforeunload', ['$event'])
  avisarAntesDeSair(evento: BeforeUnloadEvent): void {
    if (this.pendentes().size) evento.preventDefault();
  }

  soSemCategoria(): void {
    this.filtroCategorias = ['sem'];
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }

  salvarPendentes(): void {
    const pendentes = this.pendentes();
    if (!pendentes.size) return;
    const itens = [...pendentes].map(([id, v]) =>
      ({ id, categoriaId: v === 'auto' ? null : Number(v), automatica: v === 'auto' }));
    const sugestoes = this.regrasSugeridas(pendentes);
    this.salvando.set(true);
    this.api.alterarCategoriasEmLote({ itens }).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.pendentes.set(new Map());
        this.sugestoesRegra.set(sugestoes);
        this.avisarResultado(r);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** "Todas as N do filtro", inclusive as das outras páginas, com confirmação (Nielsen 5). */
  aplicarATodasDoFiltro(): void {
    const escolha = this.categoriaTodas;
    const total = this.pagina()?.total ?? 0;
    if (!escolha || !total) return;
    const f = this.filtroAplicado();
    const nome = escolha === 'auto' ? 'a categorização automática'
      : `“${this.rotuloCategoriaPorId(Number(escolha))}”`;
    if (!confirm(`Trocar a categoria das ${total} transações do filtro (${rotuloPeriodo(f.inicio, f.fim)}) para ${nome}?`)) return;
    this.salvando.set(true);
    this.api.alterarCategoriasEmLote({
      filtro: { ...f, pagina: 1 }, categoriaId: escolha === 'auto' ? null : Number(escolha), automatica: escolha === 'auto'
    }).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.categoriaTodas = null;
        this.pendentes.set(new Map());
        this.avisarResultado(r);
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** Aviso do que mudou, com "Desfazer" (a API guarda o estado anterior por alguns minutos). */
  private avisarResultado(r: ResultadoCategoriasEmLote): void {
    if (!r.alteradas || !r.desfazer) {
      this.avisos.info('Nada mudou: as transações já estavam com essa categoria.');
      return;
    }
    const chave = r.desfazer;
    this.avisos.sucesso(`${r.alteradas} transação(ões) atualizada(s).`, {
      rotulo: 'Desfazer',
      executar: () => this.api.desfazerCategoriasEmLote(chave).subscribe({
        next: (d) => {
          this.sugestoesRegra.set([]);
          this.avisos.sucesso(`Desfeito: ${d.transacoesAtualizadas} transação(ões) voltaram como estavam.`);
          this.buscar(this.filtro.pagina ?? 1);
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      })
    });
  }

  /** Várias transações com a mesma descrição foram para a mesma categoria: sugere regra (como nas Sugestões). */
  private regrasSugeridas(pendentes: Map<number, EscolhaCategoria>): SugestaoRegra[] {
    const grupos = new Map<string, SugestaoRegra>();
    for (const t of this.pagina()?.itens ?? []) {
      const escolha = pendentes.get(t.id);
      if (!escolha || escolha === 'auto') continue;
      const texto = (t.descricaoOriginal ?? t.descricao).trim().replace(/\s+/g, ' ').slice(0, 100);
      if (texto.length < 4) continue;
      const chave = `${texto.toLocaleUpperCase('pt-BR')}|${escolha}`;
      const g = grupos.get(chave);
      if (g) g.quantidade++;
      else grupos.set(chave, { texto, categoriaId: Number(escolha), categoriaNome: this.rotuloCategoriaPorId(Number(escolha)), quantidade: 1 });
    }
    return [...grupos.values()].filter((g) => g.quantidade >= 2).slice(0, 3);
  }

  criarRegraSugerida(r: SugestaoRegra): void {
    this.api.criarRegra({
      texto: r.texto, tipoCorrespondencia: TipoCorrespondencia.Contem, categoriaId: r.categoriaId, prioridade: 100, ativa: true
    }).subscribe({
      next: () => {
        this.dispensarRegra(r);
        this.avisos.sucesso(`Regra criada: as próximas “${r.texto}” entram em ${r.categoriaNome}.`);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  dispensarRegra(r: SugestaoRegra): void {
    this.sugestoesRegra.update((lista) => lista.filter((x) => x !== r));
  }

  private rotuloCategoriaPorId(id: number): string {
    const c = this.todasCategorias().find((x) => x.id === id);
    return c ? this.rotuloCategoria(c) : 'categoria';
  }

  /** Cancelou o descarte: os campos de filtro voltam ao que está na tela. */
  private voltarAoFiltroAplicado(): void {
    const f = this.filtroAplicado();
    this.filtro = { ...f };
    this.filtroCategorias = [...(f.semCategoria ? ['sem'] : []), ...(f.categoriaIds ?? []).map(String)];
    this.categoriasFiltradas.set(this.filtroCategorias);
  }

  /* ------------------------------ sugestões de categoria ------------------------------ */

  readonly quantidadeAlta = computed(() =>
    this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.categoriaId).length);

  abrirSugestoes(): void {
    this.sugestoesAbertas.set(true);
    this.sugestoesAlteraram = false;
    this.carregarSugestoes();
  }

  trocarModoSugestoes(semCategoria: boolean): void {
    this.somenteSemCategoria.set(semCategoria);
    this.carregarSugestoes();
  }

  private carregarSugestoes(): void {
    this.carregandoSugestoes.set(true);
    this.api.sugestoesCategoria(this.somenteSemCategoria(), 365).subscribe({
      next: (s) => {
        this.carregandoSugestoes.set(false);
        this.linhasSugestao.set(s.map((x) => ({
          ...x, escolhida: x.categoriaId, aplicada: false,
          criarRegra: x.confianca === 'alta' && x.quantidade >= 3 && x.textoRegra.length >= 4
        })));
      },
      error: (e) => {
        this.carregandoSugestoes.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  nomeConfianca(c: string): string {
    const nomes: Record<string, string> = {
      alta: 'confiança alta', media: 'confiança média', baixa: 'palavra-chave', nenhuma: 'sem sugestão'
    };
    return nomes[c] ?? c;
  }

  aplicarSugestao(l: LinhaSugestao, silencioso = false): void {
    if (!l.escolhida) return;
    const categoriaId = l.escolhida;
    this.salvando.set(true);
    this.api.categorizarEmLote(l.transacaoIds, categoriaId).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.sugestoesAlteraram = true;
        this.linhasSugestao.update((linhas) => linhas.map((x) => (x.chave === l.chave ? { ...x, aplicada: true } : x)));
        if (l.criarRegra && l.textoRegra.trim()) {
          this.api.criarRegra({
            texto: l.textoRegra.trim(), tipoCorrespondencia: TipoCorrespondencia.Contem, categoriaId, prioridade: 100, ativa: true
          }).subscribe({ error: (e) => this.avisos.erro(`Regra "${l.textoRegra}": ${mensagemDeErro(e)}`) });
        }
        if (!silencioso) this.avisos.sucesso(`${r.transacoesAtualizadas} transação(ões) categorizada(s).`);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  aplicarConfiancaAlta(): void {
    const linhas = this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.escolhida);
    if (!linhas.length || !confirm(`Aplicar ${linhas.length} sugestão(ões) de confiança alta?`)) return;
    for (const l of linhas) this.aplicarSugestao(l, true);
    this.avisos.sucesso(`${linhas.length} grupo(s) categorizado(s).`);
  }

  fecharSugestoes(): void {
    this.sugestoesAbertas.set(false);
    if (this.sugestoesAlteraram) this.buscar(this.filtro.pagina ?? 1);
  }

  novo(): void {
    this.form = {
      id: null, openFinance: false, contaId: this.contasManuais()[0]?.id ?? null, data: dataIso(), descricao: '',
      valor: null, tipo: TipoTransacao.Despesa, categoriaId: null, observacao: '', ignorarRelatorios: false,
      pendente: false, descricaoOriginal: null, emergencia: false
    };
    this.sugestoesForm.set([]);
  }

  editar(t: Transacao): void {
    this.form = {
      id: t.id, openFinance: t.openFinance, contaId: t.contaId, data: t.data, descricao: t.descricao,
      valor: Math.abs(t.valor), tipo: t.valor > 0 ? TipoTransacao.Receita : TipoTransacao.Despesa,
      categoriaId: t.categoriaId, observacao: t.observacao ?? '', ignorarRelatorios: t.ignorarRelatorios,
      pendente: t.status === StatusTransacao.Pendente, descricaoOriginal: t.descricaoOriginal, emergencia: t.emergencia
    };
    this.sugestoesForm.set([]);
    this.api.sugestoesDaTransacao(t.id).subscribe({ next: (s) => this.sugestoesForm.set(s), error: () => { /* opcional */ } });
  }

  salvar(): void {
    const f = this.form;
    if (!f) return;
    this.salvando.set(true);

    const requisicao = f.openFinance && f.id
      ? this.api.ajustarTransacao(f.id, {
          descricao: f.descricao, categoriaId: f.categoriaId, observacao: f.observacao || null,
          ignorarRelatorios: f.ignorarRelatorios, emergencia: f.emergencia && f.tipo === TipoTransacao.Despesa
        })
      : (() => {
          const dto = {
            contaId: f.contaId ?? 0, data: f.data, descricao: f.descricao, valor: Number(f.valor), tipo: f.tipo,
            categoriaId: f.categoriaId, observacao: f.observacao || null, ignorarRelatorios: f.ignorarRelatorios,
            status: f.pendente ? StatusTransacao.Pendente : StatusTransacao.Efetivada,
            emergencia: f.emergencia && f.tipo === TipoTransacao.Despesa
          };
          return f.id ? this.api.atualizarTransacao(f.id, dto) : this.api.criarTransacao(dto);
        })();

    requisicao.subscribe({
      next: () => {
        this.salvando.set(false);
        this.form = null;
        this.avisos.sucesso('Transação salva.');
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  excluir(): void {
    const f = this.form;
    if (!f?.id || !confirm('Excluir esta transação?')) return;
    this.api.excluirTransacao(f.id).subscribe({
      next: () => {
        this.form = null;
        this.avisos.sucesso('Transação excluída.');
        this.buscar(this.filtro.pagina ?? 1);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirTransferencia(): void {
    this.transferencia = { contaOrigemId: null, contaDestinoId: null, valor: null, data: dataIso(), descricao: '' };
  }

  salvarTransferencia(): void {
    const t = this.transferencia;
    if (!t?.contaOrigemId || !t.contaDestinoId || !t.valor) return;
    this.salvando.set(true);
    this.api.transferir({
      contaOrigemId: t.contaOrigemId, contaDestinoId: t.contaDestinoId, valor: Number(t.valor), data: t.data,
      descricao: t.descricao || null
    }).subscribe({
      next: () => {
        this.salvando.set(false);
        this.transferencia = null;
        this.avisos.sucesso('Transferência registrada.');
        this.buscar(1);
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  private tamanhoSalvo(): number {
    try {
      const n = Number(localStorage.getItem('financas.tamanhoPagina'));
      return [25, 50, 100, 200].includes(n) ? n : 50;
    } catch {
      return 50;
    }
  }

  private filtroInicial(): FiltroTransacoes {
    const hoje = new Date();
    return {
      inicio: primeiroDiaMes(hoje.getFullYear(), hoje.getMonth() + 1),
      fim: ultimoDiaMes(hoje.getFullYear(), hoje.getMonth() + 1),
      contaId: null, categoriaId: null, categoriaIds: [], semCategoria: false, somenteEmergencias: false, poucoInformativa: false,
      lancamento: null, tipo: null, busca: '', pagina: 1,
      tamanhoPagina: this.tamanhoSalvo()
    };
  }
}

/** Página da lista vinda da URL (UX-5); inválida ou ausente = 1. */
function paginaDaUrl(params: ParamMap): number {
  const n = Number(params.get('pagina'));
  return Number.isInteger(n) && n >= 1 && n <= 100000 ? n : 1;
}
