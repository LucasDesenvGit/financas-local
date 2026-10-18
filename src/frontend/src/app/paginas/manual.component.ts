import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { map } from 'rxjs';
import { GLOSSARIO } from '../compartilhado/glossario';
import { TermoComponent } from '../compartilhado/termo.component';
import { CapituloManual, MANUAL, SecaoManual } from './manual-conteudo';

/** Uma seção achada pela busca, com o capítulo dela. */
interface Resultado { capitulo: CapituloManual; secao: SecaoManual; }

/** Minúsculas e sem acento, para a busca achar "fatura" em "Fatura" e "credito" em "crédito". */
function normalizar(texto: string): string {
  return texto.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
}

/**
 * Manual do site (#113, ADR 0026). /manual: índice dos capítulos (Shneiderman: visão geral primeiro) e busca;
 * /manual/<capitulo>#<secao>: o capítulo, com cada seção em "para que serve → passo a passo → abrir a tela → dicas →
 * erros comuns" (Nielsen 10: ajuda focada na tarefa, com passos concretos). Capítulo e busca ficam na URL (ADR 0018).
 */
@Component({
  selector: 'app-manual',
  imports: [RouterLink, FormsModule, TermoComponent],
  template: `
    @if (idCapitulo()) {
      @if (capitulo(); as cap) {
        <nav class="trilha" aria-label="Você está em">
          <a routerLink="/manual">Manual</a> <span aria-hidden="true">›</span> <span aria-current="page">{{ cap.titulo }}</span>
        </nav>
        <header class="cabecalho-pagina">
          <h1>{{ numero(cap) }}. {{ cap.titulo }}</h1>
        </header>
        <p class="resumo-capitulo">{{ cap.resumo }}</p>

        <nav class="painel-bloco indice-secoes" aria-labelledby="titulo-indice-secoes">
          <h2 id="titulo-indice-secoes">Neste capítulo</h2>
          <ol>
            @for (s of cap.secoes; track s.id) {
              <li><a [routerLink]="[]" [fragment]="s.id">{{ s.titulo }}</a></li>
            }
          </ol>
        </nav>

        @for (s of cap.secoes; track s.id) {
          <section class="painel-bloco secao-manual" [id]="s.id" [attr.aria-labelledby]="'titulo-' + s.id">
            <h2 [id]="'titulo-' + s.id">{{ s.titulo }}</h2>
            <p class="para-que">{{ s.paraQueServe }}</p>
            <h3>Passo a passo</h3>
            <ol class="passos">
              @for (p of s.passos; track $index) { <li>{{ p }}</li> }
            </ol>
            @if (s.abrir?.length) {
              <p class="abrir-tela">
                @for (l of s.abrir; track l.rotulo) {
                  <a class="botao botao-pequeno" [routerLink]="l.rota" [queryParams]="l.params ?? null">{{ l.rotulo }} →</a>
                }
              </p>
            }
            @if (s.termos?.length) {
              <p class="termos-secao"><span class="texto-suave">Termos desta seção:</span>
                @for (k of s.termos; track k) { <app-termo [chave]="k">{{ glossario[k].termo }}</app-termo> }
              </p>
            }
            @if (s.dicas?.length) {
              <h3>Dicas</h3>
              <ul>@for (d of s.dicas; track $index) { <li>{{ d }}</li> }</ul>
            }
            @if (s.erros?.length) {
              <h3>Erros comuns</h3>
              <ul class="erros-comuns">@for (e of s.erros; track $index) { <li>{{ e }}</li> }</ul>
            }
          </section>
        }

        <nav class="navegacao-capitulos" aria-label="Outros capítulos">
          @if (vizinho(cap, -1); as ant) { <a [routerLink]="['/manual', ant.id]">‹ {{ ant.titulo }}</a> } @else { <span></span> }
          <a routerLink="/manual">Índice do manual</a>
          @if (vizinho(cap, 1); as prox) { <a [routerLink]="['/manual', prox.id]">{{ prox.titulo }} ›</a> } @else { <span></span> }
        </nav>
      } @else {
        <header class="cabecalho-pagina"><h1>Manual</h1></header>
        <p class="vazio">Essa parte do manual não existe. <a routerLink="/manual">Ver o índice</a></p>
      }
    } @else {
      <header class="cabecalho-pagina"><h1>Manual</h1></header>
      <p class="resumo-capitulo">Passo a passo para usar o Finanças Local: do primeiro acesso até transações bem categorizadas,
        relatórios confiáveis e metas no caminho. Em qualquer tela, o botão “Ajuda” abre a parte que explica aquela tela.</p>

      <div class="busca-manual">
        <label for="busca-manual">Buscar no manual</label>
        <input id="busca-manual" type="search" name="busca" [ngModel]="busca()" (ngModelChange)="mudarBusca($event)"
          placeholder="ex.: fatura, regra, Pix, meta" maxlength="60" autocomplete="off">
      </div>

      @if (busca().trim()) {
        <p class="texto-suave" role="status">{{ resultados().length === 1 ? '1 seção encontrada' : resultados().length + ' seções encontradas' }}</p>
        @if (resultados().length) {
          <ul class="resultados-busca">
            @for (r of resultados(); track r.capitulo.id + r.secao.id) {
              <li class="painel-bloco">
                <a [routerLink]="['/manual', r.capitulo.id]" [fragment]="r.secao.id">{{ r.capitulo.titulo }} › {{ r.secao.titulo }}</a>
                <p class="texto-suave">{{ r.secao.paraQueServe }}</p>
              </li>
            }
          </ul>
        } @else {
          <p class="vazio">Nada encontrado. Tente outra palavra (ou menos palavras).</p>
        }
      } @else {
        <ol class="grade-capitulos">
          @for (c of capitulos; track c.id; let i = $index) {
            <li class="painel-bloco">
              <h2><a [routerLink]="['/manual', c.id]">{{ i + 1 }}. {{ c.titulo }}</a></h2>
              <p>{{ c.resumo }}</p>
              <small class="texto-suave">{{ c.secoes.length }} seções</small>
            </li>
          }
        </ol>
      }
    }
  `,
  styles: `
    /* flex + gap: o Angular tira os espaços entre os elementos (ficaria "Manual›Contas"). */
    .trilha { display: flex; flex-wrap: wrap; gap: 6px; font-size: 0.85rem; margin-bottom: 4px; }
    .resumo-capitulo { max-width: 80ch; margin: -8px 0 18px; }
    .indice-secoes ol { margin: 6px 0 0; padding-left: 22px; columns: 2 260px; }
    .indice-secoes li { margin: 3px 0; }
    .secao-manual { margin-bottom: 16px; scroll-margin-top: 72px; }
    .secao-manual h2 { margin: 0 0 4px; }
    .secao-manual h3 { font-size: 0.95rem; margin: 14px 0 6px; }
    .para-que { margin: 0; color: var(--texto-suave); }
    .passos { margin: 0; padding-left: 22px; display: grid; gap: 6px; max-width: 90ch; }
    .secao-manual ul { margin: 0; padding-left: 20px; display: grid; gap: 4px; max-width: 90ch; }
    .abrir-tela { display: flex; flex-wrap: wrap; gap: 8px; margin: 14px 0 0; }
    .abrir-tela .botao { text-decoration: none; min-height: 32px; display: inline-flex; align-items: center; }
    .termos-secao { display: flex; flex-wrap: wrap; gap: 4px 12px; align-items: center; margin: 10px 0 0; font-size: 0.9rem; }
    .navegacao-capitulos { display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin: 8px 0 24px; }
    .busca-manual { display: grid; gap: 6px; max-width: 420px; margin-bottom: 18px; }
    .busca-manual label { font-weight: 600; }
    .grade-capitulos { list-style: none; padding: 0; margin: 0; display: grid; gap: 14px;
      grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr)); }
    .grade-capitulos li { margin: 0; }
    .grade-capitulos h2 { font-size: 1.05rem; margin: 0 0 6px; }
    .grade-capitulos p { margin: 0 0 6px; }
    .resultados-busca { list-style: none; padding: 0; margin: 0; display: grid; gap: 10px; }
    .resultados-busca li { margin: 0; }
    .resultados-busca p { margin: 4px 0 0; }
  `
})
export class ManualComponent {
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly capitulos = MANUAL;
  readonly glossario = GLOSSARIO;

  readonly idCapitulo = toSignal(this.rota.paramMap.pipe(map((p) => p.get('capitulo'))),
    { initialValue: this.rota.snapshot.paramMap.get('capitulo') });
  readonly capitulo = computed(() => MANUAL.find((c) => c.id === this.idCapitulo()) ?? null);
  readonly busca = toSignal(this.rota.queryParamMap.pipe(map((p) => (p.get('busca') ?? '').slice(0, 60))),
    { initialValue: (this.rota.snapshot.queryParamMap.get('busca') ?? '').slice(0, 60) });

  /** Seções que têm todas as palavras buscadas, em qualquer parte (título, passos, dicas, erros, links). */
  readonly resultados = computed<Resultado[]>(() => {
    const palavras = normalizar(this.busca()).split(/\s+/).filter((p) => p.length > 0);
    if (!palavras.length) return [];
    const achados: Resultado[] = [];
    for (const capitulo of MANUAL) {
      for (const secao of capitulo.secoes) {
        const texto = normalizar([capitulo.titulo, secao.titulo, secao.paraQueServe, ...secao.passos, ...(secao.dicas ?? []),
          ...(secao.erros ?? []), ...(secao.abrir ?? []).map((l) => l.rotulo)].join(' '));
        if (palavras.every((p) => texto.includes(p))) achados.push({ capitulo, secao });
      }
    }
    return achados;
  });

  numero(c: CapituloManual): number {
    return MANUAL.indexOf(c) + 1;
  }

  vizinho(c: CapituloManual, passo: -1 | 1): CapituloManual | null {
    return MANUAL[MANUAL.indexOf(c) + passo] ?? null;
  }

  /** A busca vai para a URL (replaceUrl: o voltar do navegador não passa por cada letra; ADR 0018). */
  mudarBusca(texto: string): void {
    void this.router.navigate([], {
      relativeTo: this.rota, queryParams: { busca: texto.trim() ? texto : null }, replaceUrl: true
    });
  }
}
