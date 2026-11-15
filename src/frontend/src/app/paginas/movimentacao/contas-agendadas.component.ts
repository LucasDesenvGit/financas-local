import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { AssinaturasRecorrentesComponent } from './assinaturas-recorrentes.component';
import { ApiService } from '../../core/api.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import {
  AvisoContaAgendada, Categoria, Conta, ContaAgendada, FormaCobranca, NOMES_FORMA_COBRANCA, NOMES_RECORRENCIA, Recorrencia,
  Reserva, SalvarContaAgendada, SugestaoContaAgendada, TipoCategoria, TipoConta
} from '../../core/modelos';
import {
  OpcaoCofrinho, aceitaCofrinho, baixaSozinha, cofrinhoEncontrado, mostrarPagar, opcoesCofrinho, podeLancar,
  textoPagamentoCofrinho
} from './pago-com-cofrinho';
import { LogoBancoComponent } from '../../compartilhado/bancos/logo-banco.component';
import { SeloServicoComponent } from '../../compartilhado/bancos/selo-servico.component';
import { AvisosService } from '../../core/avisos.service';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { dataIso, dataIsoValida, paramsTransacoes, paraData, primeiroDiaMes, ultimoDiaMes } from '../../compartilhado/utilidades';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';

@Component({
  selector: 'app-contas-agendadas',
  imports: [ListaSuspensaDirective, FormsModule, RouterLink, MoedaPipe, DatePipe, ModalComponent, CampoValorComponent, CampoDataComponent,
    AssinaturasRecorrentesComponent, LogoBancoComponent, SeloServicoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina">
      <h1>Contas a pagar</h1>
      <button type="button" class="botao botao-primario" (click)="nova()">+ Nova conta</button>
    </header>

    <!-- #158: avisos das recorrentes, com a ação ao lado (Nielsen 1 e 9); "não veio" e "cartão arquivado" também no Painel. -->
    @if (avisosRecorrentes().length) {
      <section class="painel-bloco avisos-recorrentes" aria-labelledby="titulo-avisos">
        <div class="bloco-titulo"><h2 id="titulo-avisos">Avisos das recorrentes</h2></div>
        <ul class="lista-avisos">
          @for (a of avisosRecorrentes(); track a.contaAgendadaId + a.tipo) {
            <li>
              <span aria-hidden="true">⚠</span> {{ a.texto }}
              @if (a.tipo === 'aumentou' && a.valorNovo !== null) {
                <span class="nowrap">({{ a.valorAnterior | currency }} → {{ a.valorNovo | currency }})</span>
                <button type="button" class="botao botao-pequeno" (click)="atualizarPeloAviso(a)"
                  [attr.aria-label]="'Atualizar o valor de ' + a.descricao">Atualizar o valor</button>
              }
              @if (a.tipo === 'mudou-de-cartao' && a.contaNovaId !== null) {
                <button type="button" class="botao botao-pequeno" (click)="atualizarPeloAviso(a)"
                  [attr.aria-label]="'Atualizar o catálogo de ' + a.descricao + ': cobra em ' + a.contaNovaNome">Atualizar o catálogo</button>
              }
              <!-- #691: a cobrança não chegou pelo extrato depois de sincronizar: pode ter saído direto de um cofrinho. -->
              @if (a.tipo === 'nao-veio' && a.podeSerCofrinho) {
                <button type="button" class="botao botao-pequeno" (click)="pagarPeloAviso(a)"
                  [attr.aria-label]="'Foi paga com um cofrinho: ' + a.descricao">Foi paga com um cofrinho</button>
              }
            </li>
          }
        </ul>
      </section>
    }

    <!-- #158: o de-para das cobranças que se repetem, como aba da mesma tela (Nielsen 4); a aba fica na URL (ADR 0018). -->
    <div class="segmentado barra-abas" role="tablist" aria-label="Contas a pagar">
      <button type="button" role="tab" [class.ativo]="aba() === 'contas'" [attr.aria-selected]="aba() === 'contas'"
        (click)="mudarAba('contas')">Contas a pagar</button>
      <button type="button" role="tab" [class.ativo]="aba() === 'recorrentes'" [attr.aria-selected]="aba() === 'recorrentes'"
        (click)="mudarAba('recorrentes')">Assinaturas e recorrentes</button>
    </div>

    @if (aba() === 'recorrentes') {
      <app-assinaturas-recorrentes [itens]="itens()" [contas]="contas()" />
    } @else {
    <section class="kpis">
      <article class="kpi"><span>Vencidas</span><strong class="negativo">{{ totalVencido() | currency }}</strong></article>
      <article class="kpi"><span>Próximos 30 dias</span><strong>{{ total30() | currency }}</strong></article>
    </section>

    <div class="tabela-rolagem">
      <table class="tabela">
        <thead>
          <tr><th>Vencimento</th><th>Descrição</th><th>Recorrência</th><th>Categoria</th><th class="direita">Valor</th><th><span class="so-leitor">Ações</span></th></tr>
        </thead>
        <tbody>
          @for (c of itens(); track c.id) {
            <tr [class.ignorada]="!c.ativa">
              <td class="nowrap">
                @if (c.encerrada) {
                  <span class="etiqueta etiqueta-suave">encerrada</span>
                } @else {
                  {{ c.proximoVencimento | date: 'dd/MM/yyyy' }}
                }
                @if (c.ativa) {
                  <br><small [class.negativo]="c.diasParaVencer < 0" class="texto-suave">
                    {{ c.diasParaVencer < 0 ? 'vencida há ' + (-c.diasParaVencer) + 'd' : c.diasParaVencer === 0 ? 'vence hoje' : 'em ' + c.diasParaVencer + ' dias' }}
                  </small>
                }
              </td>
              <!-- #265: selo do serviço (ou ícone da categoria) ao lado do nome da cobrança. #379: o selo numa coluna, e o
                   nome, a conta, a forma de cobrança e a observação empilhados ao lado, todos começando no mesmo ponto. -->
              <td><div class="cobranca"><app-selo-servico [texto]="c.textoCobranca" [nome]="c.descricao"
                [categoria]="c.categoriaNome" [tamanho]="24" />
                <div class="cobranca-texto">
                  <span class="nome-cobranca">{{ c.descricao }}</span>
                  <!-- #242: selo do banco ou do cartão ao lado do nome (decorativo: o nome está escrito, ADR 0024). -->
                  @if (c.contaNome) {
                    <small class="texto-suave conta-da-cobranca"><app-logo-banco [codigo]="bancoDe(c.contaId)"
                      [cartao]="ehCartao(c.contaId) || c.formaCobranca === FormaCobranca.CartaoCredito" [tamanho]="16" />{{ c.contaNome }}</small>
                  }
                  <!-- #158: como é cobrada, em texto (Nielsen 6: onde cai fica escrito na tela). -->
                  <small class="texto-suave forma-cobranca">{{ nomeForma(c.formaCobranca) }}
                    <!-- #691: de onde sai o dinheiro fica escrito (Nielsen 6). -->
                    @if (c.reservaNome) { · cofrinho {{ c.reservaNome }}@if (c.contaNome) { ({{ c.contaNome }})} }
                    @if (c.reservaNome && c.formaCobranca === FormaCobranca.DebitoAutomatico) { · o app lança a despesa no vencimento }
                    @else if (baixaSozinha(c)) { · baixa sozinha quando a cobrança com “{{ c.textoCobranca }}” chega }</small>
                  @if (!cofrinhoEncontrado(c)) {
                    <small class="texto-alerta">Cofrinho “{{ c.reservaNome }}” não encontrado nas reservas de {{ c.contaNome }}:
                      a despesa continua sendo lançada com esse nome.</small>
                  }
                  <!-- #691: pagamento dado antes de marcar o cofrinho, sem a despesa: o app oferece lançar (Nielsen 1). -->
                  @if (c.ultimoSemDespesa && c.reservaNome) {
                    <small class="texto-alerta">o pagamento de {{ c.ultimoSemDespesa | date: 'dd/MM' }} está sem a despesa
                      <button type="button" class="link" (click)="lancarUltimoPeloCofrinho(c)"
                        [attr.aria-label]="'Lançar pelo cofrinho a despesa de ' + c.descricao">lançar pelo cofrinho</button></small>
                  }
                  <!-- #691: o que o app lançou sozinho fica à vista, com saída fácil (Nielsen 1 e 3). -->
                  @if (c.ultimoPeloCofrinho; as u) {
                    <small class="texto-suave pago-cofrinho">{{ textoPagamentoCofrinho(u) }} · {{ u.valor | currency }}
                      <button type="button" class="link" (click)="abrirAjuste(c)"
                        [attr.aria-label]="'Ajustar o valor pago de ' + c.descricao">ajustar valor</button>
                      @if (u.podeDesfazer) {
                        <button type="button" class="link" (click)="desfazerPagamento(c)"
                          [attr.aria-label]="'Desfazer o pagamento de ' + c.descricao">desfazer</button>
                      }</small>
                  }
                  @if (c.observacao) { <small class="texto-suave">{{ c.observacao }}</small> }
                </div></div></td>
              <td>{{ nomeRecorrencia(c.recorrencia) }}
                @if (c.encerrada) {
                  <br><small class="texto-suave">encerrada@if (c.ultimoPagamentoEm) { em {{ c.ultimoPagamentoEm | date: 'dd/MM/yyyy' }}}</small>
                } @else if (c.ativa && c.dataLimite) {
                  <br><small class="texto-suave">até {{ c.dataLimite | date: 'MMM/yyyy' }} ·
                    {{ c.pagamentosRestantes === 1 ? 'último pagamento' : 'faltam ' + c.pagamentosRestantes }}</small>
                }
              </td>
              <td>{{ c.categoriaNome ?? '—' }}</td>
              <td class="direita valor">{{ c.valor | currency }}</td>
              <td class="direita nowrap">
                @if (mostrarPagar(c)) {
                  <button type="button" class="botao botao-pequeno" (click)="abrirPagamento(c)"
                    [attr.aria-label]="'Pagar ' + c.descricao">Pagar</button>
                }
                <button type="button" class="link" (click)="editar(c)">editar</button>
              </td>
            </tr>
          } @empty {
            <tr><td colspan="6" class="vazio">Cadastre aluguel, condomínio, escola, assinaturas… para ser lembrado no painel.</td></tr>
          }
        </tbody>
      </table>
    </div>

    @if (sugestoesVisiveis().length || ignoradasNaLista().length) {
      <section class="painel-bloco sugestoes expandir" aria-labelledby="titulo-sugestoes" animate.leave="recolher">
        <div class="bloco-titulo">
          <h2 id="titulo-sugestoes">Sugestões do extrato</h2>
        </div>
        <p class="texto-suave pequeno-texto">
          Pagamentos que se repetem todo mês (ou todo ano) nas suas contas e cartões, e ainda não estão nesta lista:
          contas, débitos automáticos e assinaturas. O banco não informa boletos futuros, então o vencimento é estimado
          pelo dia em que você costuma pagar. Confira antes de salvar.
        </p>
        @if (sugestoesVisiveis().length) {
          <div class="tabela-rolagem">
            <table class="tabela">
              <thead>
                <tr><th>Próximo (estimado)</th><th>Descrição</th><th>Categoria</th><th class="direita">Valor</th><th><span class="so-leitor">Ações</span></th></tr>
              </thead>
              <tbody>
                @for (s of sugestoesVisiveis(); track s.chave) {
                  <tr>
                    <td class="nowrap">{{ s.proximoVencimento | date: 'dd/MM/yyyy' }}<br>
                      <small class="texto-suave">costuma ser dia {{ s.diaHabitual }}</small>
                      @if (s.vencimentoSemPagamento) {
                        <br><small class="texto-alerta">o de {{ s.vencimentoSemPagamento | date: 'dd/MM' }} ainda não apareceu no extrato</small>
                      }</td>
                    <td><div class="cobranca"><app-selo-servico [texto]="s.textoCobranca ?? s.termoBusca" [nome]="s.descricao"
                      [categoria]="s.categoriaNome" [tamanho]="24" />
                      <div class="cobranca-texto">
                      <span class="nome-cobranca">{{ s.descricao }}</span>
                      @if (s.recorrencia === Recorrencia.Anual) {
                        <small class="texto-suave">anual: pago em {{ s.primeiroPagamento | date: 'MM/yyyy' }} e {{ s.ultimoPagamento | date: 'MM/yyyy' }} ·
                          <span class="conta-da-cobranca"><app-logo-banco [codigo]="bancoDe(s.contaId)" [cartao]="ehCartao(s.contaId)" [tamanho]="16" />{{ s.contaNome }}</span></small>
                      } @else {
                        <small class="texto-suave">pago em {{ s.meses }} meses@if (s.pagamentos !== s.meses) { ({{ s.pagamentos }} pagamentos) } · último em {{ s.ultimoPagamento | date: 'dd/MM' }} ·
                          <span class="conta-da-cobranca"><app-logo-banco [codigo]="bancoDe(s.contaId)" [cartao]="ehCartao(s.contaId)" [tamanho]="16" />{{ s.contaNome }}</span></small>
                      }
                      <small class="texto-suave forma-cobranca">{{ nomeForma(s.formaCobranca) }}</small>
                      <!-- #205: conta a pagar parecida (valor, dia e conta) com outro nome: a pergunta e a resposta juntas
                           (Nielsen 6, reconhecer em vez de lembrar), num botão de verdade (WCAG 2.1.1). -->
                      @if (s.pareceSerId !== null && s.chaveExtrato) {
                        <small class="parece-ser">parece ser <strong>{{ s.pareceSerNome }}</strong>, que já está na lista</small>
                        <button type="button" class="botao botao-pequeno nowrap" (click)="eAMesma(s)"
                          [attr.aria-label]="'é a mesma: ' + s.descricao + ' é ' + s.pareceSerNome">é a mesma</button>
                      }
                      </div></div></td>
                    <td>{{ s.categoriaNome ?? '—' }}</td>
                    <td class="direita valor nowrap">
                      @if (s.valorVariavel) {
                        ≈ {{ s.valor | currency }}<br>
                        <small class="texto-suave">varia de {{ s.valorMinimo | currency }} a {{ s.valorMaximo | currency }}</small>
                      } @else {
                        {{ s.valor | currency }}
                      }
                    </td>
                    <td class="direita nowrap">
                      <button type="button" class="botao botao-pequeno" (click)="cadastrarSugestao(s)"
                        [attr.aria-label]="'Cadastrar ' + s.descricao">Cadastrar</button>
                      <a class="link" routerLink="/transacoes" [queryParams]="linkPagamentos(s)"
                        [attr.aria-label]="'Ver pagamentos de ' + s.descricao">ver pagamentos</a>
                      <button type="button" class="link" (click)="ignorar(s)"
                        [attr.aria-label]="'Ignorar sugestão ' + s.descricao">ignorar</button>
                    </td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
        } @else {
          <p class="vazio">Nenhuma sugestão nova.</p>
        }
        @if (ignoradasNaLista().length) {
          <p class="pequeno-texto">
            <button type="button" class="link" (click)="restaurarIgnoradas()">
              Mostrar de novo {{ ignoradasNaLista().length }} sugestão(ões) ignorada(s)
            </button>
          </p>
        }
      </section>
    }
    }

    <app-modal [aberto]="form !== null" [titulo]="editandoId ? 'Editar conta' : 'Nova conta a pagar'" (fechar)="form = null">
      @if (form; as fm) {
        <form (ngSubmit)="salvar()" #f="ngForm" class="formulario">
          <label>Descrição <input name="descricao" [(ngModel)]="fm.descricao" required maxlength="150"></label>
          <div class="grade-campos">
            <label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>
            <label>Próximo vencimento <app-campo-data name="venc" [(ngModel)]="fm.proximoVencimento" obrigatorio /></label>
            <label>Recorrência
              <select name="rec" [(ngModel)]="fm.recorrencia">
                <option [ngValue]="Recorrencia.Mensal">Mensal</option>
                <option [ngValue]="Recorrencia.Anual">Anual</option>
                <option [ngValue]="Recorrencia.Unica">Única</option>
              </select>
            </label>
            <label>Categoria
              <select name="cat" [(ngModel)]="fm.categoriaId">
                <option [ngValue]="null">—</option>
                @for (c of categoriasDespesa(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
              </select>
            </label>
          </div>
          @if (fm.recorrencia !== Recorrencia.Unica) {
            <div class="grade-campos">
              <label>Até (opcional)
                <app-campo-data name="limite" [(ngModel)]="fm.dataLimite" [minimo]="fm.proximoVencimento"
                  ajuda="ajuda-limite" [invalido]="limiteInvalido(fm)" /></label>
              <label>ou quantos pagamentos faltam
                <input type="number" name="restantes" min="1" step="1" [ngModel]="restantesDoForm(fm)"
                  (ngModelChange)="definirRestantes(fm, $event)" aria-describedby="ajuda-limite"></label>
            </div>
            @if (limiteInvalido(fm)) {
              <p class="negativo pequeno-texto" role="alert">A data limite não pode ser antes do próximo vencimento.</p>
            }
            <p class="texto-suave pequeno-texto" id="ajuda-limite">
              Depois do último pagamento a conta é encerrada sozinha e sai do Painel. Em branco, ela se repete sem fim.</p>
          }
          <label>Como é cobrada
            <select name="forma" [(ngModel)]="fm.formaCobranca" aria-describedby="ajuda-forma">
              @for (f of formas; track f) { <option [ngValue]="f">{{ nomeForma(f) }}</option> }
            </select>
          </label>
          <label>{{ fm.formaCobranca === FormaCobranca.CartaoCredito ? 'Cartão onde cai' : 'Conta de pagamento' }} (opcional)
            <select name="conta" [(ngModel)]="fm.contaId" (ngModelChange)="fm.reservaNome = null; outroCofrinho = false">
              <option [ngValue]="null">—</option>
              @for (c of contas(); track c.id) { <option [ngValue]="c.id">{{ c.nome }}</option> }
            </select>
          </label>
          <!-- #691 (ADR 0063): de onde sai o dinheiro. Só em conta do Open Finance, fora do cartão (divulgação
               progressiva); os cofrinhos da conta numa lista, para escolher em vez de digitar (Nielsen 6). -->
          @if (aceitaCofrinho(fm)) {
            <label>Pago com
              <select name="pagoCom" [ngModel]="outroCofrinho ? OUTRO : fm.reservaNome" (ngModelChange)="escolherCofrinho(fm, $event)"
                aria-describedby="ajuda-cofrinho">
                <option [ngValue]="null">Saldo da conta</option>
                @for (r of opcoesCofrinho(fm); track r.nome) {
                  <option [ngValue]="r.nome">Cofrinho {{ r.nome }}{{ r.saldo === null ? ' (não encontrado nas reservas)' : '' }}</option>
                }
                <option [ngValue]="OUTRO">Outro cofrinho…</option>
              </select>
            </label>
            @if (outroCofrinho) {
              <label>Nome do cofrinho
                <input name="reservaNome" [(ngModel)]="fm.reservaNome" maxlength="100" required placeholder="ex.: Emergência"
                  aria-describedby="ajuda-cofrinho"></label>
            }
            <p class="texto-suave pequeno-texto" id="ajuda-cofrinho">
              @if (!fm.reservaNome) {
                O dinheiro sai da conta, e o pagamento chega pela sincronização.
              } @else if (fm.formaCobranca === FormaCobranca.DebitoAutomatico) {
                Sai direto do cofrinho, sem passar pela conta: no vencimento, o app lança a despesa e baixa o cofrinho.
                Se a cobrança chegar pelo extrato mesmo assim, vale a do banco.
              } @else {
                Sai direto do cofrinho: o “Pagar” lança a despesa e baixa o cofrinho.
              }
            </p>
          }
          @if (fm.formaCobranca !== FormaCobranca.AMao) {
            <label>Texto da cobrança no extrato
              <input name="textoCobranca" [(ngModel)]="fm.textoCobranca" maxlength="100" placeholder="ex.: NETFLIX"
                aria-describedby="ajuda-forma"></label>
          }
          <p class="texto-suave pequeno-texto" id="ajuda-forma">
            @if (fm.formaCobranca === FormaCobranca.AMao) {
              Boleto ou Pix que você paga: use “Pagar” quando pagar.
            } @else {
              Cobrada sozinha: com o texto (a descrição do banco contém…), ela se dá como paga quando a cobrança chega
              no extrato. Sem o texto, use “Pagar”.
            }
          </p>
          <label>Observação (opcional)
            <input name="observacao" [(ngModel)]="fm.observacao" maxlength="300" placeholder="ex.: conta da família, cancelar em março"></label>
          <label class="checkbox"><input type="checkbox" name="ativa" [(ngModel)]="fm.ativa"> Ativa</label>
          <div class="rodape-form">
            @if (editandoId) { <button type="button" class="botao botao-perigo" (click)="excluir()">Excluir</button> }
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="form = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="f.invalid || limiteInvalido(fm)">Salvar</button>
          </div>
        </form>
      }
    </app-modal>

    <app-modal [aberto]="pagando !== null" [titulo]="'Pagar · ' + (pagando?.descricao ?? '')" (fechar)="pagando = null">
      @if (pagando; as pg) {
        <form (ngSubmit)="confirmarPagamento()" #fp="ngForm" class="formulario">
          <label>Data do pagamento <app-campo-data name="data" [(ngModel)]="dataPagamento" obrigatorio /></label>
          @if (pg.pagamentosRestantes === 1) {
            <p class="texto-suave" role="status">É o último pagamento (até {{ pg.dataLimite | date: 'dd/MM/yyyy' }}): a conta será encerrada.</p>
          }
          @if (cofrinhoNoPagamento !== null) {
            <!-- #691: "foi paga com um cofrinho?": escolher em vez de digitar (Nielsen 6), e guardar para as próximas. -->
            @if (opcoesCofrinhoDe(pg).length) {
              <label>Qual cofrinho
                <select name="cofrinhoPagamento" [(ngModel)]="cofrinhoNoPagamento" required>
                  @for (r of opcoesCofrinhoDe(pg); track r.nome) { <option [ngValue]="r.nome">Cofrinho {{ r.nome }}</option> }
                </select>
              </label>
            } @else {
              <label>Nome do cofrinho
                <input name="cofrinhoPagamento" [(ngModel)]="cofrinhoNoPagamento" required maxlength="100" placeholder="ex.: Emergência"></label>
            }
            <label class="checkbox"><input type="checkbox" name="guardarCofrinho" [(ngModel)]="guardarCofrinho">
              Usar este cofrinho nas próximas (o app lança no vencimento)</label>
            <p class="texto-suave">A despesa é lançada na conta {{ pg.contaNome }}, saindo do cofrinho: o saldo da conta não muda.</p>
          } @else if (podeLancar(pg) && pg.reservaNome) {
            <label class="checkbox"><input type="checkbox" name="lancar" [(ngModel)]="lancarTransacao">
              Lançar a despesa saindo do cofrinho {{ pg.reservaNome }} ({{ pg.contaNome }})</label>
          } @else if (podeLancar(pg)) {
            <label class="checkbox"><input type="checkbox" name="lancar" [(ngModel)]="lancarTransacao">
              Lançar a despesa na conta {{ pg.contaNome }}</label>
          } @else {
            <p class="texto-suave">Se o pagamento sair de uma conta do Open Finance, a transação chega pela sincronização.</p>
          }
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="pagando = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fp.invalid">Confirmar pagamento</button>
          </div>
        </form>
      }
    </app-modal>

    <!-- #691: "Ajustar valor" da última baixa pelo cofrinho (conta de consumo que veio diferente). -->
    <app-modal [aberto]="ajustando !== null" [titulo]="'Ajustar valor · ' + (ajustando?.descricao ?? '')" (fechar)="ajustando = null">
      @if (ajustando; as aj) {
        <form (ngSubmit)="confirmarAjuste()" #fa="ngForm" class="formulario">
          <label>Valor pago <app-campo-valor name="valorPago" [(ngModel)]="valorAjuste" obrigatorio [minimo]="0.01" /></label>
          <p class="texto-suave">Muda a despesa lançada em {{ aj.ultimoPeloCofrinho?.data | date: 'dd/MM/yyyy' }}. O valor da
            conta a pagar continua o mesmo para os próximos meses.</p>
          <div class="rodape-form">
            <span class="cresce"></span>
            <button type="button" class="botao" (click)="ajustando = null">Cancelar</button>
            <button class="botao botao-primario" [disabled]="fa.invalid">Salvar</button>
          </div>
        </form>
      }
    </app-modal>
  `,
  styles: `
    /* #379: o selo da cobrança numa coluna; à direita, o nome, a conta e a forma de cobrança empilhados, alinhados. */
    .cobranca { display: flex; align-items: flex-start; gap: 10px; min-width: 15rem; }
    .cobranca > app-selo-servico { flex: none; }
    .cobranca-texto { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; min-width: 0; }
    .nome-cobranca { min-height: 24px; display: inline-flex; align-items: center; overflow-wrap: anywhere; }
    /* #242: o selo do banco junto do nome da conta, na mesma linha. */
    .conta-da-cobranca { display: inline-flex; align-items: center; gap: 5px; }
    .cobranca-texto .botao { margin-top: 4px; }
    .pago-cofrinho .link { margin-left: 4px; }
  `
})
export class ContasAgendadasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  /** #158: aba aberta (?aba=recorrentes). */
  readonly aba = signal<'contas' | 'recorrentes'>(
    this.rota.snapshot.queryParamMap.get('aba') === 'recorrentes' ? 'recorrentes' : 'contas');

  constructor() {
    this.rota.queryParamMap.pipe(takeUntilDestroyed())
      .subscribe((p) => this.aba.set(p.get('aba') === 'recorrentes' ? 'recorrentes' : 'contas'));
  }

  /** Trocar de aba limpa os filtros da outra (replaceUrl, para o voltar do navegador não passar por cada clique). */
  mudarAba(aba: 'contas' | 'recorrentes'): void {
    this.aba.set(aba);
    void this.router.navigate([], { relativeTo: this.rota, replaceUrl: true,
      queryParams: { aba: aba === 'recorrentes' ? 'recorrentes' : null } });
  }

  readonly Recorrencia = Recorrencia;
  readonly FormaCobranca = FormaCobranca;
  readonly formas = [FormaCobranca.AMao, FormaCobranca.DebitoAutomatico, FormaCobranca.CartaoCredito];
  readonly itens = signal<ContaAgendada[]>([]);
  readonly contas = signal<Conta[]>([]);
  readonly categorias = signal<Categoria[]>([]);
  /** #691: as reservas (cofrinhos) das contas, para o "Pago com". */
  readonly reservas = signal<Reserva[]>([]);
  readonly sugestoes = signal<SugestaoContaAgendada[]>([]);
  /** #158: avisos das recorrentes. */
  readonly avisosRecorrentes = signal<AvisoContaAgendada[]>([]);
  readonly ignoradas = signal<string[]>(ContasAgendadasComponent.lerIgnoradas());
  readonly sugestoesVisiveis = computed(() => this.sugestoes().filter((s) => !this.ignoradas().includes(s.chave)));
  readonly ignoradasNaLista = computed(() => this.sugestoes().filter((s) => this.ignoradas().includes(s.chave)));
  readonly categoriasDespesa = computed(() => this.categorias().filter((c) => c.tipo === TipoCategoria.Despesa && c.ativa));
  readonly totalVencido = computed(() =>
    this.itens().filter((c) => c.ativa && c.diasParaVencer < 0).reduce((s, c) => s + c.valor, 0));
  readonly total30 = computed(() =>
    this.itens().filter((c) => c.ativa && c.diasParaVencer >= 0 && c.diasParaVencer <= 30).reduce((s, c) => s + c.valor, 0));

  form: SalvarContaAgendada | null = null;
  editandoId: number | null = null;
  /** Sugestão que originou o formulário aberto: ao salvar, ela sai da lista mesmo se a descrição for editada. */
  private chaveSugestaoEmCadastro: string | null = null;
  pagando: ContaAgendada | null = null;
  /** #691: "Pago com" → "Outro cofrinho…": o nome é digitado. */
  outroCofrinho = false;
  readonly OUTRO = '__outro__';
  /** #691: conta a pagar com o valor pago pelo cofrinho em ajuste. */
  ajustando: ContaAgendada | null = null;
  valorAjuste = 0;
  dataPagamento = dataIso();
  lancarTransacao = false;

  ngOnInit(): void {
    this.carregar();
    this.api.contas().subscribe((c) => this.contas.set(c));
    this.api.categorias().subscribe((c) => this.categorias.set(c));
    // Complementar: sem as reservas, o "Pago com" mostra só o saldo da conta e o "Outro cofrinho…".
    this.api.reservas().subscribe({ next: (r) => this.reservas.set(r), error: () => this.reservas.set([]) });
  }

  nomeRecorrencia(r: Recorrencia): string {
    return NOMES_RECORRENCIA[r];
  }

  nomeForma(f: FormaCobranca): string {
    return NOMES_FORMA_COBRANCA[f] ?? NOMES_FORMA_COBRANCA[FormaCobranca.AMao];
  }

  /** #158: cobrada sozinha e com o texto da cobrança: dá baixa quando a cobrança chega, sem "Pagar". */
  /** #242 (ADR 0024): o banco que vale para a conta (null = selo neutro). */
  bancoDe(contaId: number | null): string | null {
    return this.contas().find((c) => c.id === contaId)?.banco ?? null;
  }

  ehCartao(contaId: number | null): boolean {
    return this.contas().find((c) => c.id === contaId)?.tipo === TipoConta.CartaoCredito;
  }

  baixaSozinha(c: ContaAgendada): boolean {
    return baixaSozinha(c);
  }

  mostrarPagar(c: ContaAgendada): boolean {
    return mostrarPagar(c);
  }

  podeLancar(c: ContaAgendada): boolean {
    return podeLancar(c, this.contas());
  }

  /* -------------------------- #691: pago com o cofrinho -------------------------- */

  readonly textoPagamentoCofrinho = textoPagamentoCofrinho;

  aceitaCofrinho(fm: SalvarContaAgendada): boolean {
    return aceitaCofrinho(this.contas().find((c) => c.id === fm.contaId), fm.formaCobranca);
  }

  opcoesCofrinho(fm: SalvarContaAgendada): OpcaoCofrinho[] {
    return opcoesCofrinho(this.reservas(), fm.contaId, this.outroCofrinho ? null : fm.reservaNome);
  }

  cofrinhoEncontrado(c: ContaAgendada): boolean {
    return cofrinhoEncontrado(c, this.reservas());
  }

  escolherCofrinho(fm: SalvarContaAgendada, valor: string | null): void {
    this.outroCofrinho = valor === this.OUTRO;
    fm.reservaNome = this.outroCofrinho ? '' : valor;
  }

  abrirAjuste(c: ContaAgendada): void {
    this.ajustando = c;
    this.valorAjuste = c.ultimoPeloCofrinho?.valor ?? c.valor;
  }

  confirmarAjuste(): void {
    if (!this.ajustando) return;
    this.api.ajustarPagamentoCofrinho(this.ajustando.id, Number(this.valorAjuste)).subscribe({
      next: () => {
        this.ajustando = null;
        this.avisos.sucesso('Valor pago ajustado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** O lançamento sai e o vencimento volta: pede confirmação (Nielsen 5), e a baixa automática não se repete. */
  async desfazerPagamento(c: ContaAgendada): Promise<void> {
    const u = c.ultimoPeloCofrinho;
    if (!u || !(await this.confirmacao.confirmar({ titulo: 'Desfazer pagamento',
      mensagem: `Apagar a despesa de ${c.descricao} paga pelo cofrinho ${u.reservaNome} e voltar o vencimento? `
        + (u.automatico ? 'O app não lança de novo sozinho este vencimento: use “Pagar” se ele foi pago.' : ''),
      confirmar: 'Desfazer', perigo: true }))) return;
    this.api.desfazerPagamentoCofrinho(c.id).subscribe({
      next: () => {
        this.avisos.sucesso('Pagamento desfeito.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  carregar(): void {
    this.api.contasAgendadas().subscribe({
      next: (i) => this.itens.set(i),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
    // Sugestão e aviso são complementares: se falharem, a tela segue sem eles.
    this.api.avisosContasAgendadas().subscribe({ next: (a) => this.avisosRecorrentes.set(a), error: () => this.avisosRecorrentes.set([]) });
    this.api.sugestoesContasAgendadas().subscribe({ next: (s) => this.sugestoes.set(s), error: () => this.sugestoes.set([]) });
  }

  cadastrarSugestao(s: SugestaoContaAgendada): void {
    this.editandoId = null;
    this.chaveSugestaoEmCadastro = s.chave;
    this.form = {
      descricao: s.descricao, valor: s.valor, proximoVencimento: s.proximoVencimento, recorrencia: s.recorrencia ?? Recorrencia.Mensal,
      categoriaId: s.categoriaId, contaId: s.contaId, ativa: true, dataLimite: null,
      formaCobranca: s.formaCobranca ?? FormaCobranca.AMao, textoCobranca: s.textoCobranca ?? null, observacao: null,
      // #205: a conta a pagar guarda de que sugestão veio; renomear não a traz de volta.
      chaveExtrato: s.chaveExtrato ?? null, reservaNome: null
    };
    this.outroCofrinho = false;
  }

  /**
   * #205: "é a mesma": grava a ligação da sugestão com a conta a pagar parecida, e a sugestão sai da lista. O aviso tem
   * "Desfazer" (Nielsen 3: saída clara para o clique por engano).
   */
  eAMesma(s: SugestaoContaAgendada): void {
    const id = s.pareceSerId;
    if (id === null || !s.chaveExtrato) return;
    this.api.ligarExtratoContaAgendada(id, s.chaveExtrato).subscribe({
      next: () => {
        this.sugestoes.update((lista) => lista.filter((x) => x.chaveExtrato !== s.chaveExtrato));
        this.avisos.sucesso(`Ligada a ${s.pareceSerNome}: essa sugestão não aparece mais.`, {
          rotulo: 'Desfazer',
          executar: () => this.api.desligarExtratoContaAgendada(id).subscribe({
            next: () => this.carregar(),
            error: (e) => this.avisos.erro(mensagemDeErro(e))
          })
        });
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** #158: "Atualizar o valor" ou "Atualizar o catálogo": a conta a pagar fica como a última cobrança. */
  atualizarPeloAviso(a: AvisoContaAgendada): void {
    const c = this.itens().find((x) => x.id === a.contaAgendadaId);
    if (!c) return;
    const dto: SalvarContaAgendada = {
      descricao: c.descricao, valor: a.tipo === 'aumentou' && a.valorNovo !== null ? a.valorNovo : c.valor,
      proximoVencimento: c.proximoVencimento, recorrencia: c.recorrencia, categoriaId: c.categoriaId,
      contaId: a.tipo === 'mudou-de-cartao' && a.contaNovaId !== null ? a.contaNovaId : c.contaId,
      ativa: c.ativa, dataLimite: c.dataLimite, formaCobranca: c.formaCobranca, textoCobranca: c.textoCobranca,
      observacao: c.observacao, reservaNome: c.reservaNome
    };
    this.api.atualizarContaAgendada(c.id, dto).subscribe({
      next: () => {
        this.avisos.sucesso(a.tipo === 'aumentou' ? 'Valor atualizado.' : `Agora cobra em ${a.contaNovaNome}.`);
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  linkPagamentos(s: SugestaoContaAgendada): Record<string, string> {
    const inicio = paraData(s.primeiroPagamento);
    const fim = paraData(s.ultimoPagamento);
    return {
      ...paramsTransacoes({
        tipo: 'despesa',
        inicio: primeiroDiaMes(inicio.getFullYear(), inicio.getMonth() + 1),
        fim: ultimoDiaMes(fim.getFullYear(), fim.getMonth() + 1)
      }),
      busca: s.termoBusca
    };
  }

  ignorar(s: SugestaoContaAgendada): void {
    this.gravarIgnoradas([...this.ignoradas(), s.chave]);
  }

  restaurarIgnoradas(): void {
    const naLista = new Set(this.ignoradasNaLista().map((s) => s.chave));
    this.gravarIgnoradas(this.ignoradas().filter((c) => !naLista.has(c)));
  }

  /** Lembrado só neste navegador: é conveniência de tela, não dado financeiro. */
  private static readonly CHAVE_IGNORADAS = 'financas.sugestoesContasIgnoradas';

  private static lerIgnoradas(): string[] {
    try {
      const valor = JSON.parse(localStorage.getItem(ContasAgendadasComponent.CHAVE_IGNORADAS) ?? '[]');
      return Array.isArray(valor) ? valor.filter((c): c is string => typeof c === 'string') : [];
    } catch {
      return [];
    }
  }

  private gravarIgnoradas(chaves: string[]): void {
    const unicas = [...new Set(chaves)].slice(-200);
    this.ignoradas.set(unicas);
    try {
      localStorage.setItem(ContasAgendadasComponent.CHAVE_IGNORADAS, JSON.stringify(unicas));
    } catch {
      // Sem armazenamento (janela anônima, bloqueado): vale só até recarregar a tela.
    }
  }

  nova(): void {
    this.editandoId = null;
    this.chaveSugestaoEmCadastro = null;
    this.form = { descricao: '', valor: 0, proximoVencimento: dataIso(), recorrencia: Recorrencia.Mensal, categoriaId: null, contaId: null, ativa: true,
      dataLimite: null, formaCobranca: FormaCobranca.AMao, textoCobranca: null, observacao: null, reservaNome: null };
    this.outroCofrinho = false;
  }

  editar(c: ContaAgendada): void {
    this.editandoId = c.id;
    this.chaveSugestaoEmCadastro = null;
    this.form = {
      descricao: c.descricao, valor: c.valor, proximoVencimento: c.proximoVencimento, recorrencia: c.recorrencia,
      categoriaId: c.categoriaId, contaId: c.contaId, ativa: c.ativa, dataLimite: c.dataLimite,
      formaCobranca: c.formaCobranca ?? FormaCobranca.AMao, textoCobranca: c.textoCobranca, observacao: c.observacao,
      reservaNome: c.reservaNome
    };
    this.outroCofrinho = false;
  }

  /** #69: data limite antes do próximo vencimento (só importa com a conta ativa; a API recusa do mesmo jeito). */
  limiteInvalido(fm: SalvarContaAgendada): boolean {
    return fm.ativa && fm.recorrencia !== Recorrencia.Unica && !!fm.dataLimite && !!fm.proximoVencimento
      && fm.dataLimite < fm.proximoVencimento;
  }

  /** Pagamentos que faltam pela data limite, contando o próximo (o atalho do formulário mostra os dois jeitos). */
  restantesDoForm(fm: SalvarContaAgendada): number | null {
    if (!fm.dataLimite || !dataIsoValida(fm.proximoVencimento) || fm.dataLimite < fm.proximoVencimento) return null;
    let n = 0;
    for (let v = fm.proximoVencimento; v <= fm.dataLimite && n < 1200; v = avancarVencimento(v, fm.recorrencia)) n++;
    return n;
  }

  /** "Faltam N pagamentos" vira a data do N-ésimo vencimento a partir do próximo. */
  definirRestantes(fm: SalvarContaAgendada, valor: number | string | null): void {
    const n = Math.floor(Number(valor));
    if (!valor || !Number.isFinite(n) || n < 1 || !dataIsoValida(fm.proximoVencimento)) {
      fm.dataLimite = null;
      return;
    }
    let v = fm.proximoVencimento;
    for (let i = 1; i < Math.min(n, 1200); i++) v = avancarVencimento(v, fm.recorrencia);
    fm.dataLimite = v;
  }

  salvar(): void {
    if (!this.form) return;
    const dto = {
      ...this.form, valor: Number(this.form.valor),
      dataLimite: this.form.recorrencia === Recorrencia.Unica || !this.form.dataLimite ? null : this.form.dataLimite,
      // #158: o texto só vale para a cobrada sozinha.
      textoCobranca: this.form.formaCobranca === FormaCobranca.AMao ? null : this.form.textoCobranca?.trim() || null,
      observacao: this.form.observacao?.trim() || null,
      // #691: o cofrinho só vale em conta do Open Finance, fora do cartão.
      reservaNome: this.aceitaCofrinho(this.form) ? this.form.reservaNome?.trim() || null : null
    };
    const req = this.editandoId ? this.api.atualizarContaAgendada(this.editandoId, dto) : this.api.criarContaAgendada(dto);
    req.subscribe({
      next: () => {
        this.form = null;
        if (this.chaveSugestaoEmCadastro) this.gravarIgnoradas([...this.ignoradas(), this.chaveSugestaoEmCadastro]);
        this.chaveSugestaoEmCadastro = null;
        this.avisos.sucesso('Conta salva.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  async excluir(): Promise<void> {
    if (!this.editandoId || !(await this.confirmacao.confirmar({ titulo: 'Excluir conta a pagar', mensagem: 'Excluir esta conta a pagar?', confirmar: 'Excluir', perigo: true }))) return;
    this.api.excluirContaAgendada(this.editandoId).subscribe({
      next: () => {
        this.form = null;
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirPagamento(c: ContaAgendada): void {
    this.pagando = c;
    this.dataPagamento = dataIso();
    this.lancarTransacao = this.podeLancar(c);
    this.cofrinhoNoPagamento = null;
  }

  /** #691: o cofrinho escolhido no "Pagar" aberto pelo aviso (null = o "Pagar" de sempre) e se ele fica para as próximas. */
  cofrinhoNoPagamento: string | null = null;
  guardarCofrinho = true;

  opcoesCofrinhoDe(c: ContaAgendada): OpcaoCofrinho[] {
    return opcoesCofrinho(this.reservas(), c.contaId, null);
  }

  /** #691: "Foi paga com um cofrinho": o "Pagar" com a data do vencimento e o primeiro cofrinho da conta já escolhidos. */
  pagarPeloAviso(a: AvisoContaAgendada): void {
    const c = this.itens().find((x) => x.id === a.contaAgendadaId);
    if (!c) return;
    this.pagando = c;
    this.dataPagamento = a.data ?? dataIso();
    this.lancarTransacao = true;
    this.cofrinhoNoPagamento = this.opcoesCofrinhoDe(c)[0]?.nome ?? '';
    this.guardarCofrinho = true;
  }

  /** #691: lança, pelo cofrinho, a despesa do último pagamento que ficou sem ela (confirma antes: Nielsen 5). */
  async lancarUltimoPeloCofrinho(c: ContaAgendada): Promise<void> {
    const data = c.ultimoSemDespesa;
    if (!data || !c.reservaNome || c.contaId === null) return;
    const [, m, d] = data.split('-');
    if (!(await this.confirmacao.confirmar({ titulo: 'Lançar pelo cofrinho',
      mensagem: `Lançar a despesa de ${c.descricao} de ${d}/${m}, no valor da conta a pagar, saindo do cofrinho ${c.reservaNome}? `
        + 'Se o valor foi outro, ajuste depois em Transações.', confirmar: 'Lançar' }))) return;
    this.api.registrarSaidaReserva({ contaId: c.contaId, nome: c.reservaNome, data, valor: c.valor, descricao: c.descricao,
      categoriaId: c.categoriaId }).subscribe({
      next: () => {
        this.avisos.sucesso('Despesa lançada pelo cofrinho.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  confirmarPagamento(): void {
    if (!this.pagando) return;
    const cofrinho = this.cofrinhoNoPagamento?.trim()
      ? { reservaNome: this.cofrinhoNoPagamento.trim(), guardarCofrinho: this.guardarCofrinho } : undefined;
    this.api.pagarContaAgendada(this.pagando.id, cofrinho ? true : this.lancarTransacao && this.podeLancar(this.pagando),
      this.dataPagamento, cofrinho)
      .subscribe({
        next: () => {
          this.pagando = null;
          this.avisos.sucesso('Pagamento registrado.');
          this.carregar();
        },
        error: (e) => this.avisos.erro(mensagemDeErro(e))
      });
  }
}

/**
 * Próximo vencimento como a API avança ao pagar (DateOnly.AddMonths/AddYears a partir do anterior): dia que não
 * existe no mês cai no último dia (31/01 → 28/02 → 28/03).
 */
export function avancarVencimento(iso: string, recorrencia: Recorrencia): string {
  const [a, m, d] = iso.split('-').map(Number);
  const meses = recorrencia === Recorrencia.Anual ? 12 : 1;
  const alvo = new Date(a, m - 1 + meses, 1);
  const ultimo = new Date(alvo.getFullYear(), alvo.getMonth() + 1, 0).getDate();
  return dataIso(new Date(alvo.getFullYear(), alvo.getMonth(), Math.min(d, ultimo)));
}
