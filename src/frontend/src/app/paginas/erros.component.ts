import { DatePipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { CampoDataDirective } from '../compartilhado/campo-data.directive';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import { ApiService } from '../core/api.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { ErroSistema, ErroSistemaDetalhe, FiltroErros, NOMES_NIVEL_ERRO, NivelErro, PaginaErros, SituacaoErros } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';
import { PaginacaoComponent } from '../compartilhado/paginacao.component';
import { dataIsoValida } from '../compartilhado/utilidades';
import { ListaSuspensaDirective } from '../compartilhado/lista-suspensa.directive';

/**
 * Erros do sistema (#11, ADR 0022), na aba Configurações › Erros. Shneiderman: a lista primeiro (o que, quando,
 * quantas vezes), filtros por período, origem e nível, e a pilha só no detalhe. Filtros na URL (ADR 0018), com
 * replaceUrl para não encher o histórico.
 */
@Component({
  selector: 'app-erros',
  imports: [ListaSuspensaDirective, FormsModule, DatePipe, PaginacaoComponent, CampoDataDirective],
  template: `
    <section class="painel-bloco">
      <div class="bloco-titulo">
        <h2>Erros do sistema</h2>
        @if (pagina(); as p) { <span class="texto-suave">{{ p.abertos }} em aberto</span> }
      </div>
      <p class="texto-suave pequeno-texto">
        Avisos e erros da API, da sincronização do Open Finance, das cotações e do câmbio, e falhas desta tela no
        navegador. Antes de gravar, tokens, senhas, CPF e valores em reais são trocados por [removido]. O mesmo erro
        repetido aparece uma vez só, com quantas vezes aconteceu. Os antigos são apagados sozinhos (prazo em
        Parametrização).
      </p>

      <div class="filtros-erros">
        <div class="segmentado" role="group" aria-label="Situação">
          @for (s of situacoes; track s.id) {
            <button type="button" [class.ativo]="filtro().situacao === s.id" [attr.aria-pressed]="filtro().situacao === s.id"
              (click)="mudar({ situacao: s.id })">{{ s.rotulo }}</button>
          }
        </div>
        <label>Origem
          <select name="origem" [ngModel]="filtro().origem" (ngModelChange)="mudar({ origem: $event })">
            <option [ngValue]="null">Todas</option>
            @for (o of origens(); track o) { <option [ngValue]="o">{{ o }}</option> }
          </select>
        </label>
        <label>Nível
          <select name="nivel" [ngModel]="filtro().nivel" (ngModelChange)="mudar({ nivel: $event })">
            <option [ngValue]="null">Todos</option>
            @for (n of niveis; track n) { <option [ngValue]="n">{{ nomeNivel(n) }}</option> }
          </select>
        </label>
        <label>De <input type="date" name="inicio" [ngModel]="filtro().inicio" (ngModelChange)="mudar({ inicio: $event || null })"></label>
        <label>Até <input type="date" name="fim" [ngModel]="filtro().fim" (ngModelChange)="mudar({ fim: $event || null })"></label>
        @if (temFiltro()) {
          <button type="button" class="link" (click)="limpar()">Limpar filtros</button>
        }
      </div>

      <div class="tabela-rolagem">
        <table class="tabela tabela-erros">
          <thead>
            <tr><th>Última vez</th><th>Nível</th><th>Origem</th><th>Mensagem</th><th><span class="so-leitor">Ações</span></th></tr>
          </thead>
          <tbody>
            @for (e of pagina()?.itens ?? []; track e.id) {
              <tr [class.ignorada]="e.resolvido">
                <td class="nowrap">{{ e.ultimaEm | date: 'dd/MM/yyyy HH:mm' }}
                  @if (e.ocorrencias > 1) {
                    <br><small class="texto-suave">{{ e.ocorrencias }} vezes desde {{ e.primeiraEm | date: 'dd/MM HH:mm' }}</small>
                  }
                </td>
                <td><span class="etiqueta" [class]="'etiqueta nivel-' + e.nivel">{{ nomeNivel(e.nivel) }}</span></td>
                <td class="nowrap">{{ e.origem }}</td>
                <td class="mensagem-erro">{{ e.mensagem }}
                  @if (e.rota) { <br><small class="texto-suave">{{ e.rota }}</small> }
                </td>
                <td class="direita nowrap">
                  <button type="button" class="link" [attr.aria-expanded]="aberto() === e.id"
                    [attr.aria-controls]="aberto() === e.id ? 'detalhe-erro-' + e.id : null"
                    [attr.aria-label]="'Detalhe do erro de ' + (e.ultimaEm | date: 'dd/MM HH:mm')" (click)="alternarDetalhe(e)">detalhe</button>
                  <button type="button" class="botao botao-pequeno" (click)="alternarResolvido(e)"
                    [attr.aria-label]="(e.resolvido ? 'Reabrir o erro de ' : 'Marcar como resolvido o erro de ') + (e.ultimaEm | date: 'dd/MM HH:mm')">
                    {{ e.resolvido ? 'Reabrir' : 'Resolvido' }}</button>
                </td>
              </tr>
              @if (aberto() === e.id) {
                <tr class="linha-detalhe" [id]="'detalhe-erro-' + e.id">
                  <td colspan="5">
                    @if (detalhe(); as d) {
                      <dl class="dados-erro">
                        <dt>Registrado por</dt><dd>{{ d.categoria }}</dd>
                        @if (e.usuario) { <dt>Usuário</dt><dd>{{ e.usuario }}</dd> }
                        <dt>Primeira vez</dt><dd>{{ e.primeiraEm | date: 'dd/MM/yyyy HH:mm:ss' }}</dd>
                        @if (e.resolvidoEm) { <dt>Resolvido em</dt><dd>{{ e.resolvidoEm | date: 'dd/MM/yyyy HH:mm' }}</dd> }
                      </dl>
                      @if (d.detalhe) { <pre class="pilha-erro">{{ d.detalhe }}</pre> }
                      @else { <p class="texto-suave">Sem exceção associada: só a mensagem acima.</p> }
                    } @else {
                      <p class="texto-suave" role="status">Carregando…</p>
                    }
                  </td>
                </tr>
              }
            } @empty {
              <tr><td colspan="5" class="vazio">{{ textoVazio() }}</td></tr>
            }
          </tbody>
        </table>
      </div>

      @if (pagina(); as p) {
        <app-paginacao [pagina]="p.pagina" [tamanho]="p.tamanhoPagina" [total]="p.total"
          (paginaMudou)="mudar({ pagina: $event }, false)" (tamanhoMudou)="tamanho.set($event); mudar({ pagina: 1 }, false)" />
        @if (filtro().situacao !== 'resolvidos' && p.total > 0) {
          <p class="pequeno-texto">
            <button type="button" class="link" (click)="resolverTodos()">Marcar os {{ p.total }} deste filtro como resolvidos</button>
          </p>
        }
      }
    </section>
  `,
  styles: `
    .filtros-erros { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; margin: 12px 0 16px; }
    .filtros-erros label { min-width: 130px; }
    .mensagem-erro { overflow-wrap: anywhere; min-width: 220px; }
    .nivel-3 { background: var(--alerta-suave); color: var(--alerta); }
    .nivel-4, .nivel-5 { background: var(--negativo-suave); color: var(--negativo); }
    .nivel-5 { font-weight: 700; }
    .linha-detalhe td { background: var(--superficie-2); }
    .dados-erro { display: grid; grid-template-columns: max-content 1fr; gap: 4px 12px; margin: 0 0 8px; font-size: 0.85rem; }
    .dados-erro dt { color: var(--texto-suave); }
    .dados-erro dd { margin: 0; overflow-wrap: anywhere; }
    .pilha-erro { white-space: pre-wrap; overflow-wrap: anywhere; max-height: 320px; overflow: auto; margin: 0;
      font-size: 0.75rem; padding: 10px; border-radius: var(--raio-pequeno); background: var(--superficie); }
    /* Celular: cada erro vira um bloco (quando · nível · origem, a mensagem inteira e as ações), sem rolagem lateral. */
    @media (max-width: 760px) {
      .tabela-erros thead { display: none; }
      .tabela-erros tbody tr:not(.linha-detalhe) {
        display: grid; grid-template-columns: auto auto 1fr; gap: 6px 10px; align-items: center;
        padding: 12px 14px; border-bottom: 1px solid var(--linha);
      }
      .tabela-erros tbody tr:not(.linha-detalhe) td { padding: 0; border: 0; }
      .tabela-erros td.mensagem-erro, .tabela-erros td:last-child, .tabela-erros td.vazio { grid-column: 1 / -1; min-width: 0; }
      .tabela-erros td:last-child { text-align: left; }
      .tabela-erros tr.linha-detalhe, .tabela-erros tr.linha-detalhe td { display: block; }
    }
  `
})
export class ErrosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly situacoes: { id: SituacaoErros; rotulo: string }[] = [
    { id: 'abertos', rotulo: 'Em aberto' },
    { id: 'resolvidos', rotulo: 'Resolvidos' },
    { id: 'todos', rotulo: 'Todos' }
  ];
  readonly niveis = [NivelErro.Aviso, NivelErro.Erro, NivelErro.Critico];

  readonly filtro = signal<FiltroErros>(ErrosComponent.filtroDaUrl(this.rota.snapshot.queryParamMap, 25));
  readonly tamanho = signal(25);
  readonly pagina = signal<PaginaErros | null>(null);
  readonly aberto = signal<number | null>(null);
  readonly detalhe = signal<ErroSistemaDetalhe | null>(null);
  /** Origens que já aparecem no banco, mais a do filtro atual (para o select não ficar sem a opção escolhida). */
  readonly origens = computed(() => {
    const lista = this.pagina()?.origens ?? [];
    const atual = this.filtro().origem;
    return atual && !lista.includes(atual) ? [...lista, atual] : lista;
  });
  readonly temFiltro = computed(() => {
    const f = this.filtro();
    return f.situacao !== 'abertos' || !!f.origem || f.nivel !== null || !!f.inicio || !!f.fim;
  });
  readonly textoVazio = computed(() => {
    if (!this.pagina()) return 'Carregando…';
    if (this.temFiltro()) return 'Nenhum erro neste filtro.';
    return 'Nenhum erro em aberto.';
  });

  constructor() {
    // Botão voltar/avançar e links para a aba já filtrada: a URL manda.
    this.rota.queryParamMap.pipe(takeUntilDestroyed()).subscribe((p) => {
      const novo = ErrosComponent.filtroDaUrl(p, this.tamanho());
      if (JSON.stringify(novo) !== JSON.stringify(this.filtro())) {
        this.filtro.set(novo);
        this.carregar();
      }
    });
  }

  ngOnInit(): void {
    this.carregar();
  }

  nomeNivel(n: NivelErro): string {
    return NOMES_NIVEL_ERRO[n] ?? String(n);
  }

  carregar(): void {
    this.api.erros(this.filtro()).subscribe({
      next: (p) => this.pagina.set(p),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /** Muda o filtro (volta para a página 1, salvo quando a mudança é a própria página) e grava na URL. */
  mudar(parte: Partial<FiltroErros>, voltarAoInicio = true): void {
    const novo: FiltroErros = { ...this.filtro(), ...(voltarAoInicio ? { pagina: 1 } : {}), ...parte, tamanhoPagina: this.tamanho() };
    this.filtro.set(novo);
    this.aberto.set(null);
    this.carregar();
    void this.router.navigate([], {
      relativeTo: this.rota, replaceUrl: true, queryParamsHandling: 'merge',
      queryParams: {
        situacao: novo.situacao === 'abertos' ? null : novo.situacao,
        origem: novo.origem || null,
        nivel: novo.nivel ?? null,
        inicio: novo.inicio || null,
        fim: novo.fim || null,
        pagina: novo.pagina > 1 ? novo.pagina : null
      }
    });
  }

  limpar(): void {
    this.mudar({ situacao: 'abertos', origem: null, nivel: null, inicio: null, fim: null });
  }

  alternarDetalhe(e: ErroSistema): void {
    if (this.aberto() === e.id) {
      this.aberto.set(null);
      return;
    }
    this.aberto.set(e.id);
    this.detalhe.set(null);
    this.api.erroDetalhe(e.id).subscribe({
      next: (d) => { if (this.aberto() === e.id) this.detalhe.set(d); },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  alternarResolvido(e: ErroSistema): void {
    this.api.situacaoErro(e.id, !e.resolvido).subscribe({
      next: () => {
        this.avisos.sucesso(e.resolvido ? 'Erro reaberto.' : 'Erro marcado como resolvido.');
        this.carregar();
      },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  resolverTodos(): void {
    const total = this.pagina()?.total ?? 0;
    if (!confirm(`Marcar os ${total} erro(s) deste filtro como resolvidos?`)) return;
    this.api.resolverErros(this.filtro()).subscribe({
      next: (r) => {
        this.avisos.sucesso(`${r.quantidade} erro(s) marcado(s) como resolvido(s).`);
        this.mudar({ pagina: 1 }, false);
      },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  static filtroDaUrl(p: ParamMap, tamanho: number): FiltroErros {
    const situacao = p.get('situacao');
    const nivel = Number(p.get('nivel'));
    const pagina = Number(p.get('pagina'));
    const inicio = p.get('inicio');
    const fim = p.get('fim');
    return {
      situacao: situacao === 'resolvidos' || situacao === 'todos' ? situacao : 'abertos',
      origem: p.get('origem') || null,
      nivel: [NivelErro.Aviso, NivelErro.Erro, NivelErro.Critico].includes(nivel) ? (nivel as NivelErro) : null,
      inicio: dataIsoValida(inicio) ? inicio : null,
      fim: dataIsoValida(fim) ? fim : null,
      pagina: Number.isInteger(pagina) && pagina > 1 ? pagina : 1,
      tamanhoPagina: tamanho
    };
  }
}
