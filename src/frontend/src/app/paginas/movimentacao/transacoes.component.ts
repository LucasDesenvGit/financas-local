import { DatePipe } from '@angular/common';
import { Component, ElementRef, HostListener, Injector, OnInit, afterNextRender, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { PrivacidadeService } from '../../core/privacidade.service';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { ActivatedRoute, Router } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ApiService } from '../../core/api.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import {
  Categoria, Conta, FiltroTransacoes, OpcaoCategoria, OrigemConta, PaginaTransacoes, ResultadoCategoriasEmLote, StatusTransacao,
  TipoConta, TipoCorrespondencia, TipoLancamento, Transacao, TipoTransacao, OrdemTransacoes
} from '../../core/modelos';
import { AvisosService } from '../../core/avisos.service';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { PaginacaoComponent } from '../../compartilhado/ui/paginacao.component';
import { OpcaoMultipla, SeletorMultiploComponent } from '../../compartilhado/ui/seletor-multiplo.component';
import { OrigemIdsUrl, dataIso } from '../../compartilhado/utilidades';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { TermoComponent } from '../../compartilhado/glossario/termo.component';
import { IconeComponent } from '../../compartilhado/ui/icone.component';
import { LogoBancoComponent } from '../../compartilhado/bancos/logo-banco.component';
import { GRUPOS_LANCAMENTO, TIPOS_LANCAMENTO } from '../../compartilhado/lancamentos/tipos-lancamento';
import { RenomearTransacaoComponent } from './renomear-transacao.component';
import { RenomearEmLoteComponent } from './renomear-em-lote.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { SeloServicoComponent } from '../../compartilhado/bancos/selo-servico.component';
import {
  COLUNAS_ORDENAVEIS, ChipFiltro, ParametrosUrl, EscolhaCategoria, FormTransacao, FormTransferencia, LinhaSugestao, SugestaoRegra,
  ajusteDoForm, alternarPagina, ariaSortDe, camposDeCategorias, categoriasDoFiltro, chipsDoFiltro, comEscolha, direcaoAoClicar,
  direcaoDe, filtroInicial, foraDosGastos, formDaTransacao, formNovo, itensDoLote, lerFiltroDaUrl, linhasDeSugestao,
  marcarIntervalo, nomeConfianca, nomeSelecao, opcoesCategoria, ordemDe, paginaDaUrl, parametrosDoFiltro, regrasSugeridas,
  rotuloCategoria, rotuloCategoriaPorId, rotuloPeriodo, semOChip, setaDe, somandoNosTotais, tamanhoPaginaValido, textoSelecao,
  conflitoRegra, transacaoDoForm, trocouFiltro, ultimaPagina, valorCategoria
} from './transacoes';

/**
 * Tela de Transações. A lógica (filtro ↔ URL, chips, ordenação, seleção, modo edição, sugestões, formulário) está em
 * funções puras em `./transacoes.ts` (#475, ADR 0059); aqui ficam os dados, o estado e as chamadas da API.
 */
@Component({
  selector: 'app-transacoes',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, DatePipe, ModalComponent, PaginacaoComponent, SeletorMultiploComponent, TermoComponent,
    IconeComponent, LogoBancoComponent, RenomearTransacaoComponent, RenomearEmLoteComponent, CampoValorComponent,
    CampoDataComponent, SeloServicoComponent],
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
      <label>De <app-campo-data [(ngModel)]="filtro.inicio" (alterado)="buscar(1)" /></label>
      <label>Até <app-campo-data [(ngModel)]="filtro.fim" (alterado)="buscar(1)" /></label>
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
        <section class="faixa-filtros expandir" aria-label="Filtros ativos" animate.leave="recolher">
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
      <p class="dica-generica texto-suave expandir" animate.leave="recolher">
        Transações cuja descrição não diz o que foi (PIX ENVIADO, PAGAMENTO, TED, só o nome do banco). Cada uma sai desta
        lista quando você escolhe a categoria (dá para várias de uma vez em "Editar categorias"), dá um nome mais claro
        a ela ou marca "revisada".
      </p>
    }

    @if (modoEdicao()) {
      <!-- Abre e recolhe animando a altura: a lista abaixo não pula de uma vez ao entrar/sair do modo edição. -->
      <section class="painel-bloco barra-edicao expandir" aria-labelledby="titulo-edicao" animate.leave="recolher">
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

      <!-- #133: a contagem é anunciada quando muda (região aria-live sempre presente). -->
      <span class="so-leitor" aria-live="polite">{{ textoSelecao() }}</span>
      <!-- A barra abre e fecha animando a altura (sem empurrar a tabela de uma vez) e fica fixa no topo ao rolar. -->
      <div class="barra-lote-espaco" [class.aberta]="selecionadas().size > 0" [attr.inert]="selecionadas().size ? null : ''">
        <div class="barra-lote-dentro">
        <section class="barra-lote" (keydown.escape)="limparSelecao()" aria-label="Ações nas transações selecionadas">
          <span class="contagem-selecao">{{ textoSelecao() }}</span>
          <select [(ngModel)]="categoriaLote">
            <option [ngValue]="null">Categorização automática</option>
            @for (c of categorias(); track c.id) { <option [ngValue]="c.id">{{ rotuloCategoria(c) }}</option> }
          </select>
          <button type="button" class="botao botao-primario" (click)="categorizarLote()">
            {{ modoEdicao() ? 'Trocar nas selecionadas' : 'Aplicar categoria' }}</button>
          <button type="button" class="botao" (click)="renomearSelecionadas()">✎ Renomear</button>
          @if (filtroAplicado().poucoInformativa) {
            <button type="button" class="botao" (click)="marcarRevisadasSelecionadas()">Marcar como revisadas</button>
          }
          <button type="button" class="botao botao-emergencia" (click)="marcarEmergenciaLote(true)">Marcar emergência</button>
          <button type="button" class="botao" (click)="marcarEmergenciaLote(false)">Desmarcar emergência</button>
          <button type="button" class="botao" (click)="limparSelecao()">✕ Limpar seleção</button>
        </section>
        </div>
      </div>

      <div class="tabela-rolagem" (keydown.escape)="limparSelecao()">
        <table class="tabela" [class.tabela-edicao]="modoEdicao()">
          <thead>
            <tr>
              <th class="col-check"><label class="celula-marcar"><input type="checkbox" [checked]="todasSelecionadas()"
                [indeterminate]="algumaSelecionadaNaPagina() && !todasSelecionadas()" (change)="alternarTodas()"
                aria-label="Selecionar todas desta página"></label></th>
              <!-- #244: ordenar pelo cabeçalho (WAI-ARIA APG, Sortable Table): botão no th, aria-sort só na coluna que vale. -->
              @for (col of colunasOrdenaveis.slice(0, 1); track col.chave) {
                <th [attr.aria-sort]="ariaSort(col.chave)"><button type="button" class="botao-ordenar" (click)="ordenar(col.chave)">
                  {{ col.rotulo }}<span class="seta-ordem" [class.ativa]="ordemAtual() === col.chave" aria-hidden="true">{{ seta(col.chave) }}</span></button></th>
              }
              <th>Descrição</th>
              @for (col of colunasOrdenaveis.slice(1); track col.chave) {
                <th [class.direita]="col.chave === 'valor'" [attr.aria-sort]="ariaSort(col.chave)"><button type="button" class="botao-ordenar"
                  (click)="ordenar(col.chave)">{{ col.rotulo }}<span class="seta-ordem" [class.ativa]="ordemAtual() === col.chave"
                  aria-hidden="true">{{ seta(col.chave) }}</span></button></th>
              }
            </tr>
          </thead>
          <tbody>
            @for (t of p.itens; track t.id; let i = $index) {
              <tr (click)="modoEdicao() ? null : editar(t)" [class.clicavel]="!modoEdicao()" [class.ignorada]="t.ignorarRelatorios"
                [class.alterada]="pendentes().has(t.id)" [class.selecionada]="selecionadas().has(t.id)"
                [class.destacada]="transacaoDestacada() === t.id" [attr.aria-current]="transacaoDestacada() === t.id ? 'true' : null"
                [attr.tabindex]="transacaoDestacada() === t.id ? -1 : null" [attr.data-transacao]="t.id">
                <!-- #133: a célula inteira marca (sem abrir a edição); Shift+clique seleciona o intervalo. -->
                <td class="col-check" (click)="$event.stopPropagation()" (mousedown)="shiftNoClique = $event.shiftKey">
                  <label class="celula-marcar"><input type="checkbox" [checked]="selecionadas().has(t.id)"
                    (click)="marcar(i, $event)" [attr.aria-label]="nomeSelecao(t)"></label>
                </td>
                <td class="nowrap">{{ t.data | date: 'dd/MM/yy' }}</td>
                <td>
                  <div class="linha-descricao">
                    <!-- #265: selo do serviço (pelo texto do banco) ou ícone da categoria; decorativo, o nome vem ao lado. -->
                    <app-selo-servico [texto]="t.descricaoOriginal" [estabelecimento]="t.estabelecimento" [nome]="t.descricao" [tipo]="t.tipoLancamento"
                      [categoria]="t.categoriaNome" [tamanho]="22" />
                    @if (modoEdicao()) {
                      <div class="truncar" [title]="t.descricao">{{ t.descricao }}</div>
                    } @else {
                      <!-- WCAG 2.1.1: o clique na linha é atalho de mouse; pelo teclado, a edição abre por este botão. -->
                      <button type="button" class="botao-texto truncar" [title]="t.descricao"
                        (click)="$event.stopPropagation(); editar(t)" [attr.aria-label]="'Editar: ' + t.descricao">{{ t.descricao }}</button>
                    }
                    <button type="button" class="botao-icone botao-renomear" (click)="$event.stopPropagation(); renomeando.set(t)"
                      [attr.aria-label]="'Renomear: ' + t.descricao">✎</button>
                  </div>
                  @if (t.descricaoOriginal && t.descricaoOriginal !== t.descricao) {
                    <div class="descricao-original truncar" [title]="t.descricaoOriginal">do banco: {{ t.descricaoOriginal }}</div>
                  }
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
                <!-- Fora dos gastos (pagamento de fatura, transferência): valor em cor neutra, para não parecer receita ou despesa. -->
                <td class="direita valor nowrap" [class.fora-dos-totais]="foraDosGastos(t)"
                  [class.positivo]="t.valor > 0 && !foraDosGastos(t)" [class.negativo]="t.valor < 0 && !foraDosGastos(t)">{{ t.valor | currency }}</td>
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

    <app-renomear-transacao [transacao]="renomeando()" (fechar)="renomeando.set(null)" (renomeada)="aoRenomear()" />
    <app-renomear-em-lote [transacoes]="renomeandoLote()" (fechar)="renomeandoLote.set(null)" (renomeada)="aoRenomearLote()" />

    <app-modal [aberto]="form !== null" [titulo]="form?.id ? 'Editar transação' : 'Novo lançamento'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          @if (fm.openFinance) {
            <p class="aviso-inline expandir" animate.leave="recolher">Transação importada do Open Finance: valor, data e conta vêm do banco.
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
              <label>Data <app-campo-data name="data" [(ngModel)]="fm.data" obrigatorio /></label>
              <label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>
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
            <div class="sugestoes-chips expandir" animate.leave="recolher">
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
                    <td>
                      @if (l.textoRegra) {
                        <label class="checkbox"><input type="checkbox" [(ngModel)]="l.criarRegra" [disabled]="l.aplicada">
                          <small>contém "{{ l.textoRegra }}"</small></label>
                        <!-- #254: o porquê de a regra vir desmarcada, em texto (WCAG 3.3.1), sem esconder a escolha. -->
                        @if (conflitoRegra(l); as outra) {
                          <small class="aviso-regra texto-alerta">Outra linha cria esta regra para {{ outra }}: só uma pode valer.</small>
                        } @else if (l.avisoRegra) {
                          <small class="aviso-regra texto-alerta">{{ l.avisoRegra }}</small>
                        }
                      } @else {
                        <small class="texto-suave">Sem texto que identifique só estas transações: categorize sem regra.</small>
                      }
                    </td>
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
            <label>Data <app-campo-data name="data" [(ngModel)]="tf.data" obrigatorio /></label>
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="tf.valor" obrigatorio [minimo]="0.01" /></label>
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
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: `
    /* #123: nome dado por você em cima; o do banco embaixo, menor (cor suave do tema, com contraste de texto). */
    .linha-descricao { display: flex; align-items: center; gap: 4px; min-width: 0; }
    /* #244: cabeçalho que ordena: o texto do th, com alvo de 24 px (WCAG 2.5.8) e a seta da direção. */
    .botao-ordenar {
      display: inline-flex; align-items: center; gap: 4px; min-height: 24px; padding: 0; border: 0; background: none;
      font: inherit; color: inherit; cursor: pointer; text-align: inherit;
    }
    .botao-ordenar:hover { color: var(--texto-forte); }
    .botao-ordenar:focus-visible { outline: 2px solid var(--primaria); outline-offset: 2px; border-radius: 4px; }
    .seta-ordem { font-size: 0.7rem; opacity: 0.35; min-width: 0.8em; }
    .seta-ordem.ativa { opacity: 1; color: var(--primaria-forte); }
    .linha-descricao .truncar { min-width: 0; }
    /* #265: o selo do serviço antes do nome, um pouco mais afastado que o ✎. */
    .linha-descricao app-selo-servico { margin-right: 6px; }
    .botao-renomear { min-width: 24px; min-height: 24px; font-size: 0.85rem; opacity: 0.65; flex: none; }
    .botao-renomear:hover, .botao-renomear:focus-visible { opacity: 1; }
    .descricao-original { font-size: 0.78rem; color: var(--texto-suave); margin-top: 1px; }
    .valor.fora-dos-totais { color: var(--texto-suave); }
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
    /* #254: o margin-top negativo dos chips (feito para o formulário) puxava os chips para cima do motivo. */
    .tabela-sugestoes .sugestoes-chips { margin-top: 6px; }
    .tabela-sugestoes .aviso-regra { display: block; max-width: 240px; margin-top: 4px; font-size: 0.78rem; }
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
    /* #141: transação aberta por link (Relatórios › Maiores despesas): fundo e faixa à esquerda, diferente da seleção. */
    tr.destacada td { background: var(--info-suave); }
    tr.destacada td:first-child { box-shadow: inset 4px 0 0 var(--primaria); }
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
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly elemento = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly TipoTransacao = TipoTransacao;
  readonly StatusTransacao = StatusTransacao;

  /**
   * #141: transação que veio no link (?transacao=id), destacada na lista. Vale só para o filtro e a página com que
   * a tela abriu: trocar qualquer um tira o destaque (e o parâmetro da URL).
   */
  readonly transacaoDestacada = signal<number | null>(null);
  /** #173 d): de que tela vieram os ?ids= (o chip diz "Rendimentos de FIIs: 2 lançamentos"). */
  readonly origemIds = signal<OrigemIdsUrl | null>(null);
  private filtroDoDestaque: string | null = null;
  /** Rolar até a linha e pôr o foco nela só na primeira vez, não a cada recarga (ex.: depois de editar). */
  private destaqueMostrado = false;

  readonly contas = signal<Conta[]>([]);
  /** Todas as categorias (inclusive inativas), para validar a categoria que chega pela URL. */
  readonly todasCategorias = signal<Categoria[]>([]);
  /** Categorias oferecidas nos selects: as ativas e, se for o caso, as inativas que estão no filtro. */
  readonly categorias = computed(() =>
    this.todasCategorias().filter((c) => c.ativa || this.categoriasFiltradas().includes(String(c.id))));
  private readonly categoriasFiltradas = signal<string[]>([]);
  /** Opções do filtro de categorias: "Sem categoria" e cada categoria seguida das subcategorias (recuadas). */
  readonly opcoesCategoria = computed<OpcaoMultipla[]>(() => opcoesCategoria(this.categorias(), this.todasCategorias()));
  readonly pagina = signal<PaginaTransacoes | null>(null);
  readonly selecionadas = signal<Set<number>>(new Set());
  readonly salvando = signal(false);
  readonly sugestoesForm = signal<OpcaoCategoria[]>([]);
  readonly sugestoesAbertas = signal(false);
  readonly carregandoSugestoes = signal(false);
  readonly somenteSemCategoria = signal(true);
  readonly linhasSugestao = signal<LinhaSugestao[]>([]);
  /** #254: regras criadas nesta janela de sugestões (texto em maiúsculas → categoria), para não criar outra igual. */
  private readonly regrasCriadas = new Map<string, number>();
  private sugestoesAlteraram = false;

  readonly contasManuais = computed(() => this.contas().filter((c) => c.origem === OrigemConta.Manual));
  readonly totalPaginas = computed(() => {
    const p = this.pagina();
    return p ? ultimaPagina(p.total, p.tamanhoPagina) : 1;
  });
  readonly todasSelecionadas = computed(() => {
    const itens = this.pagina()?.itens ?? [];
    return itens.length > 0 && itens.every((t) => this.selecionadas().has(t.id));
  });
  /** #133: quantas das selecionadas estão nesta página (o resto está em outras: a seleção continua ao trocar). */
  private readonly selecionadasNaPagina = computed(() =>
    (this.pagina()?.itens ?? []).filter((t) => this.selecionadas().has(t.id)).length);
  readonly algumaSelecionadaNaPagina = computed(() => this.selecionadasNaPagina() > 0);
  readonly textoSelecao = computed(() => textoSelecao(this.selecionadas().size, this.selecionadasNaPagina()));
  /** Shift+clique: a última caixa marcada/desmarcada (índice na página) e se o clique veio com Shift. */
  private ultimoMarcado: number | null = null;
  shiftNoClique = false;

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

  /** UX-2: um chip para cada filtro diferente do padrão (`chipsDoFiltro`). */
  readonly chipsFiltro = computed<ChipFiltro[]>(() =>
    chipsDoFiltro(this.filtroAplicado(), this.filtroInicial(), this.todasCategorias(), this.contas(), this.origemIds()));

  removerFiltro(id: ChipFiltro['id']): void {
    const r = semOChip(this.filtro, this.filtroCategorias, id, this.filtroInicial());
    this.filtro = r.filtro;
    if (r.categorias !== this.filtroCategorias) {
      this.filtroCategorias = r.categorias;
      this.sincronizarFiltroCategoria();
    }
    if (id === 'ids') this.origemIds.set(null);
    this.buscar(1);
  }

  limparFiltros(): void {
    this.filtro = this.filtroInicial();
    this.origemIds.set(null);
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

  /** UX-1 e UX-5: o filtro, o modo edição, a origem dos ids e o destaque que vêm na URL (`lerFiltroDaUrl`). */
  private aplicarParametrosDaUrl(params: ParametrosUrl): void {
    const lido = lerFiltroDaUrl(params, this.filtro, this.todasCategorias(), this.contas());
    this.filtro = lido.filtro;
    this.filtroCategorias = lido.categorias;
    this.sincronizarFiltroCategoria();
    if (lido.modoEdicao) this.modoEdicao.set(true);
    if (lido.origemIds) this.origemIds.set(lido.origemIds);
    if (lido.transacaoDestacada !== null) this.transacaoDestacada.set(lido.transacaoDestacada);
  }

  rotuloCategoria(c: Categoria): string {
    return rotuloCategoria(c, this.todasCategorias());
  }

  aplicarFiltroCategorias(valores: string[]): void {
    this.filtroCategorias = valores;
    this.sincronizarFiltroCategoria();
    this.buscar(1);
  }

  /** Leva as categorias marcadas ('sem' | id) para o filtro da API. */
  private sincronizarFiltroCategoria(): void {
    Object.assign(this.filtro, camposDeCategorias(this.filtroCategorias));
    this.categoriasFiltradas.set(this.filtroCategorias);
  }

  buscar(pagina: number): void {
    // #87: trocar de página ou de filtro perde as trocas não salvas; pergunta antes (Nielsen 5). Sem pendência, a busca
    // continua na hora (síncrona), como antes do #631.
    if (this.pendentes().size) {
      void this.confirmacao.confirmar({ titulo: 'Descartar alterações', mensagem: `Descartar ${this.pendentes().size} alteração(ões) de categoria não salva(s)?`, confirmar: 'Descartar', perigo: true })
        .then((ok) => {
          if (!ok) {
            this.voltarAoFiltroAplicado();
            return;
          }
          this.pendentes.set(new Map());
          this.executarBusca(pagina);
        });
      return;
    }
    this.executarBusca(pagina);
  }

  private executarBusca(pagina: number): void {
    this.filtro.pagina = pagina;
    if (this.transacaoDestacada() !== null) {
      const chave = JSON.stringify(this.filtro);
      this.filtroDoDestaque ??= chave;
      if (chave !== this.filtroDoDestaque) this.transacaoDestacada.set(null);
    }
    // #133: trocar só de página mantém a seleção; trocar o filtro limpa (`trocouFiltro`).
    if (trocouFiltro(this.filtroAplicado(), this.filtro) && this.selecionadas().size) {
      this.limparSelecao();
      this.avisos.info('Seleção limpa ao trocar o filtro.');
    }
    this.filtroAplicado.set({ ...this.filtro });
    this.gravarFiltroNaUrl();
    this.api.transacoes(this.filtro).subscribe({
      next: (p) => {
        // Página da URL que não existe mais (link antigo, menos transações agora): vai para a última que existe.
        const ultima = ultimaPagina(p.total, p.tamanhoPagina);
        if (!p.itens.length && p.pagina > ultima) {
          this.buscar(ultima);
          return;
        }
        const trocouPagina = this.pagina() !== null && this.pagina()!.pagina !== p.pagina;
        this.pagina.set(p);
        this.ultimoMarcado = null;   // o intervalo do Shift vale dentro de uma página
        if (trocouPagina) this.rolarParaTopo();
        this.mostrarDestaque(p);
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /**
   * UX-5: o filtro da busca vai para a URL (só o que difere do padrão), para o F5 e o link salvo manterem a lista.
   * replaceUrl: o voltar do navegador leva à tela anterior, não ao filtro anterior (julgamento próprio; Nielsen 4).
   */
  private gravarFiltroNaUrl(): void {
    const params = parametrosDoFiltro(this.filtro, this.filtroInicial(), {
      categorias: this.filtroCategorias, transacaoDestacada: this.transacaoDestacada(), origemIds: this.origemIds(),
      modoEdicao: this.modoEdicao()
    });
    this.router.navigate([], { relativeTo: this.rota, queryParams: params, replaceUrl: true });
  }

  /* ------------------------------ #244: ordenar pelo cabeçalho ------------------------------ */

  readonly colunasOrdenaveis = COLUNAS_ORDENAVEIS;

  ordemAtual(): OrdemTransacoes {
    return ordemDe(this.filtroAplicado());
  }

  direcaoAtual(): 'asc' | 'desc' {
    return direcaoDe(this.filtroAplicado());
  }

  ariaSort(coluna: OrdemTransacoes): 'ascending' | 'descending' | null {
    return ariaSortDe(this.filtroAplicado(), coluna);
  }

  seta(coluna: OrdemTransacoes): string {
    return setaDe(this.filtroAplicado(), coluna);
  }

  /** Clicar na coluna ordena por ela; clicar de novo inverte (`direcaoAoClicar`). Volta para a página 1. */
  ordenar(coluna: OrdemTransacoes): void {
    this.filtro.direcao = direcaoAoClicar(this.filtroAplicado(), coluna);
    this.filtro.ordem = coluna;
    this.buscar(1);
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

  /**
   * #141: rola até a transação do link e põe o foco nela (teclado e leitor de tela caem direto nela). Se ela não
   * está na lista, avisa e tira o destaque, para o F5 não repetir o aviso.
   */
  private mostrarDestaque(p: PaginaTransacoes): void {
    const id = this.transacaoDestacada();
    if (id === null || this.destaqueMostrado) return;
    this.destaqueMostrado = true;
    if (!p.itens.some((t) => t.id === id)) {
      this.transacaoDestacada.set(null);
      this.gravarFiltroNaUrl();
      this.avisos.info('A transação do link não aparece neste filtro: ela pode ter sido apagada ou mudado de data ou de conta.');
      return;
    }
    afterNextRender(() => {
      const linha = this.elemento.nativeElement.querySelector<HTMLElement>(`tr[data-transacao="${id}"]`);
      if (!linha) return;
      const semAnimacao = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
      linha.scrollIntoView({ block: 'center', behavior: semAnimacao ? 'auto' : 'smooth' });
      linha.focus({ preventScroll: true });
    }, { injector: this.injector });
  }

  alternar(id: number): void {
    this.selecionadas.update((s) => {
      const novo = new Set(s);
      if (novo.has(id)) novo.delete(id); else novo.add(id);
      return novo;
    });
  }

  /** #133: marca/desmarca a caixa da linha `indice`; com Shift (clique ou Shift+Espaço), o intervalo (`marcarIntervalo`). */
  marcar(indice: number, e: MouseEvent): void {
    const comShift = e.shiftKey || this.shiftNoClique;
    this.shiftNoClique = false;
    const marcar = (e.target as HTMLInputElement).checked;
    this.selecionadas.update((s) => marcarIntervalo(s, this.pagina()?.itens ?? [], indice, this.ultimoMarcado, comShift, marcar));
    this.ultimoMarcado = indice;
  }

  /** "Selecionar todas desta página" (`alternarPagina`). */
  alternarTodas(): void {
    this.selecionadas.update((s) => alternarPagina(s, this.pagina()?.itens ?? []));
    this.ultimoMarcado = null;
  }

  limparSelecao(): void {
    this.selecionadas.set(new Set());
    this.ultimoMarcado = null;
  }

  nomeSelecao(t: Transacao): string {
    return nomeSelecao(t, this.privacidade.oculto());
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
        this.limparSelecao();
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
        this.limparSelecao();
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

  /** #123: transação na janela "Renomear" (null = fechada). */
  readonly renomeando = signal<Transacao | null>(null);

  aoRenomear(): void {
    this.renomeando.set(null);
    this.buscar(this.filtro.pagina ?? 1);
  }

  /** #123: o mesmo nome nas selecionadas (as desta página). */
  readonly renomeandoLote = signal<Transacao[] | null>(null);

  renomearSelecionadas(): void {
    const selecionadas = (this.pagina()?.itens ?? []).filter((t) => this.selecionadas().has(t.id));
    if (selecionadas.length) this.renomeandoLote.set(selecionadas);
  }

  aoRenomearLote(): void {
    this.renomeandoLote.set(null);
    this.limparSelecao();
    this.buscar(this.filtro.pagina ?? 1);
  }

  readonly foraDosGastos = foraDosGastos;
  readonly somandoNosTotais = somandoNosTotais;

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

  async alternarModoEdicao(): Promise<void> {
    if (this.modoEdicao() && this.pendentes().size
      && !(await this.confirmacao.confirmar({ titulo: 'Sair da edição', mensagem: `Sair da edição e descartar ${this.pendentes().size} alteração(ões) não salva(s)?`, confirmar: 'Sair e descartar', perigo: true }))) return;
    this.pendentes.set(new Map());
    this.sugestoesRegra.set([]);
    this.categoriaTodas = null;
    this.modoEdicao.update((v) => !v);
    this.gravarFiltroNaUrl();
  }

  valorCategoria(t: Transacao): string {
    return valorCategoria(t, this.pendentes());
  }

  mudarCategoria(t: Transacao, valor: string): void {
    this.pendentes.update((m) => comEscolha(m, t, valor));
  }

  desfazerLinha(t: Transacao): void {
    this.pendentes.update((m) => {
      const novo = new Map(m);
      novo.delete(t.id);
      return novo;
    });
  }

  async descartarPendentes(): Promise<void> {
    if (!this.pendentes().size || !(await this.confirmacao.confirmar({ titulo: 'Descartar alterações', mensagem: `Descartar ${this.pendentes().size} alteração(ões)?`, confirmar: 'Descartar', perigo: true }))) return;
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
    const sugestoes = regrasSugeridas(this.pagina()?.itens ?? [], pendentes, this.todasCategorias());
    this.salvando.set(true);
    this.api.alterarCategoriasEmLote({ itens: itensDoLote(pendentes) }).subscribe({
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
  async aplicarATodasDoFiltro(): Promise<void> {
    const escolha = this.categoriaTodas;
    const total = this.pagina()?.total ?? 0;
    if (!escolha || !total) return;
    const f = this.filtroAplicado();
    const nome = escolha === 'auto' ? 'a categorização automática'
      : `“${rotuloCategoriaPorId(Number(escolha), this.todasCategorias())}”`;
    if (!(await this.confirmacao.confirmar({ titulo: 'Trocar a categoria', mensagem: `Trocar a categoria das ${total} transações do filtro (${rotuloPeriodo(f.inicio, f.fim)}) para ${nome}?`, confirmar: 'Trocar a categoria' }))) return;
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

  /** Cancelou o descarte: os campos de filtro voltam ao que está na tela. */
  private voltarAoFiltroAplicado(): void {
    const f = this.filtroAplicado();
    this.filtro = { ...f };
    this.filtroCategorias = categoriasDoFiltro(f);
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
        this.regrasCriadas.clear();
        this.linhasSugestao.set(linhasDeSugestao(s));
      },
      error: (e) => {
        this.carregandoSugestoes.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  readonly nomeConfianca = nomeConfianca;

  aplicarSugestao(l: LinhaSugestao, silencioso = false): void {
    if (!l.escolhida) return;
    const categoriaId = l.escolhida;
    this.salvando.set(true);
    this.api.categorizarEmLote(l.transacaoIds, categoriaId).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.sugestoesAlteraram = true;
        this.linhasSugestao.update((linhas) => linhas.map((x) => (x.chave === l.chave ? { ...x, aplicada: true } : x)));
        const conflito = this.conflitoRegra({ ...l, criarRegra: true });
        if (l.criarRegra && l.textoRegra.trim() && conflito) {
          this.avisos.erro(`A regra "${l.textoRegra}" não foi criada: outra linha a usa para ${conflito}. Categorize sem regra, ou crie a regra certa em Categorias e regras.`);
        } else if (l.criarRegra && l.textoRegra.trim()) {
          this.regrasCriadas.set(l.textoRegra.trim().toUpperCase(), categoriaId);
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

  async aplicarConfiancaAlta(): Promise<void> {
    const linhas = this.linhasSugestao().filter((l) => !l.aplicada && l.confianca === 'alta' && l.escolhida);
    if (!linhas.length) return;
    // #254: duas linhas com a mesma regra e categorias diferentes: categoriza as duas, mas nenhuma cria a regra.
    const semRegra = linhas.filter((l) => l.criarRegra && this.conflitoRegra(l));
    const nota = semRegra.length
      ? `\n\nSem regra (o mesmo texto com categorias diferentes): ${[...new Set(semRegra.map((l) => `"${l.textoRegra}"`))].join(', ')}.`
      : '';
    if (!(await this.confirmacao.confirmar({ titulo: 'Aplicar sugestões', mensagem: `Aplicar ${linhas.length} sugestão(ões) de confiança alta?${nota}`, confirmar: 'Aplicar' }))) return;
    for (const l of linhas) this.aplicarSugestao(semRegra.includes(l) ? { ...l, criarRegra: false } : l, true);
    this.avisos.sucesso(`${linhas.length} grupo(s) categorizado(s)` + (semRegra.length ? `; ${semRegra.length} sem regra, por conflito.` : '.'));
  }

  /** #254: a outra categoria que usa a mesma regra desta linha (`conflitoRegra`), ou null. */
  conflitoRegra(l: LinhaSugestao): string | null {
    return conflitoRegra(l, this.linhasSugestao(), this.regrasCriadas, this.categorias());
  }

  fecharSugestoes(): void {
    this.sugestoesAbertas.set(false);
    if (this.sugestoesAlteraram) this.buscar(this.filtro.pagina ?? 1);
  }

  novo(): void {
    this.form = formNovo(this.contasManuais()[0]?.id ?? null, dataIso());
    this.sugestoesForm.set([]);
  }

  editar(t: Transacao): void {
    this.form = formDaTransacao(t);
    this.sugestoesForm.set([]);
    this.api.sugestoesDaTransacao(t.id).subscribe({ next: (s) => this.sugestoesForm.set(s), error: () => { /* opcional */ } });
  }

  salvar(): void {
    const f = this.form;
    if (!f) return;
    this.salvando.set(true);

    const requisicao = f.openFinance && f.id
      ? this.api.ajustarTransacao(f.id, ajusteDoForm(f))
      : f.id ? this.api.atualizarTransacao(f.id, transacaoDoForm(f)) : this.api.criarTransacao(transacaoDoForm(f));

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

  async excluir(): Promise<void> {
    const f = this.form;
    if (!f?.id || !(await this.confirmacao.confirmar({ titulo: 'Excluir transação', mensagem: 'Excluir esta transação?', confirmar: 'Excluir', perigo: true }))) return;
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

  /** O filtro padrão: o mês atual, com o tamanho de página guardado no navegador (ADR 0017). */
  private filtroInicial(): FiltroTransacoes {
    let guardado: string | null = null;
    try { guardado = localStorage.getItem('financas.tamanhoPagina'); } catch { /* sem armazenamento: o padrão */ }
    return filtroInicial(new Date(), tamanhoPaginaValido(guardado));
  }
}
