import { DatePipe } from '@angular/common';
import { Component, ChangeDetectionStrategy, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LogoBancoComponent } from '../../compartilhado/bancos/logo-banco.component';
import { SeloServicoComponent } from '../../compartilhado/bancos/selo-servico.component';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { Conta, Recorrencia, SugestaoContaAgendada } from '../../core/modelos';
import { bancoDe, ehCartao, nomeForma, paramsPagamentos } from './contas-agendadas';

/**
 * Bloco "Sugestões do extrato" de Contas a pagar (#777, ADR 0059). Componente de apresentação: recebe as sugestões
 * visíveis, quantas estão ignoradas e as contas (para o selo do banco) e avisa o que a pessoa quer fazer; não chama a
 * API nem lê a rota. Fica fora da tabela `APRESENTACAO` do teste de arquitetura só porque "ver pagamentos" é um
 * `RouterLink`: ele monta o link para Transações, e quem navega por código continua sendo a tela. O bloco some sozinho
 * (com a saída animada) sem sugestão nem ignorada; o elemento do componente é `display: contents`, para o `section`
 * seguir como filho direto do layout da página.
 */
@Component({
  selector: 'app-contas-agendadas-sugestoes',
  imports: [RouterLink, MoedaPipe, DatePipe, LogoBancoComponent, SeloServicoComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  styleUrl: './contas-agendadas-cobranca.css',
  styles: `:host { display: contents; }`,
  template: `
    @if (sugestoes().length || ignoradas()) {
      <section class="painel-bloco sugestoes expandir" aria-labelledby="titulo-sugestoes" animate.leave="recolher">
        <div class="bloco-titulo">
          <h2 id="titulo-sugestoes">Sugestões do extrato</h2>
        </div>
        <p class="texto-suave pequeno-texto">
          Pagamentos que se repetem todo mês (ou todo ano) nas suas contas e cartões, e ainda não estão nesta lista:
          contas, débitos automáticos e assinaturas. O banco não informa boletos futuros, então o vencimento é estimado
          pelo dia em que você costuma pagar. Confira antes de salvar.
        </p>
        @if (sugestoes().length) {
          <div class="tabela-rolagem">
            <table class="tabela">
              <thead>
                <tr><th>Próximo (estimado)</th><th>Descrição</th><th>Categoria</th><th class="direita">Valor</th><th><span class="so-leitor">Ações</span></th></tr>
              </thead>
              <tbody>
                @for (s of sugestoes(); track s.chave) {
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
                        <button type="button" class="botao botao-pequeno nowrap" (click)="mesma.emit(s)"
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
                      <button type="button" class="botao botao-pequeno" (click)="cadastrar.emit(s)"
                        [attr.aria-label]="'Cadastrar ' + s.descricao">Cadastrar</button>
                      <a class="link" routerLink="/transacoes" [queryParams]="paramsPagamentos(s)"
                        [attr.aria-label]="'Ver pagamentos de ' + s.descricao">ver pagamentos</a>
                      <button type="button" class="link" (click)="ignorar.emit(s)"
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
        @if (ignoradas()) {
          <p class="pequeno-texto">
            <button type="button" class="link" (click)="restaurar.emit()">
              Mostrar de novo {{ ignoradas() }} sugestão(ões) ignorada(s)
            </button>
          </p>
        }
      </section>
    }
  `
})
export class ContasAgendadasSugestoesComponent {
  /** As sugestões que a pessoa ainda não ignorou. */
  readonly sugestoes = input<SugestaoContaAgendada[]>([]);
  /** Quantas sugestões ela ignorou (o "Mostrar de novo"). */
  readonly ignoradas = input(0);
  readonly contas = input<Conta[]>([]);

  readonly cadastrar = output<SugestaoContaAgendada>();
  /** #205: "é a mesma": a sugestão é a conta a pagar parecida que já está na lista. */
  readonly mesma = output<SugestaoContaAgendada>();
  readonly ignorar = output<SugestaoContaAgendada>();
  readonly restaurar = output<void>();

  readonly Recorrencia = Recorrencia;
  readonly nomeForma = nomeForma;
  readonly paramsPagamentos = paramsPagamentos;

  bancoDe(contaId: number | null): string | null {
    return bancoDe(this.contas(), contaId);
  }

  ehCartao(contaId: number | null): boolean {
    return ehCartao(this.contas(), contaId);
  }
}
