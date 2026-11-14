import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { ListaNoticias } from '../../core/modelos';
import { filtrarPorFonte, quandoFoiPublicada, situacaoDaFonte } from './noticias';

/**
 * #10 (ADR 0067): Notícias, um módulo (ADR 0064). As notícias guardadas do G1, da BBC News Brasil e do TabNews: o
 * título (o link abre no site da notícia, fora daqui), a fonte e quando saiu. A situação de cada fonte em palavras,
 * com a que falhou sem esconder as outras (Nielsen 1). Fica dentro do portão do módulo: desligado, nem é criada.
 */
@Component({
  selector: 'app-noticias',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina">
      <h1>Notícias</h1>
      <div class="acoes">
        <button type="button" class="botao" [disabled]="ocupado()" (click)="atualizar()">Atualizar agora</button>
      </div>
    </header>

    @if (dados(); as d) {
      <nav class="segmentado" aria-label="Fonte das notícias">
        <button type="button" [class.ativo]="fonte() === ''" [attr.aria-pressed]="fonte() === ''" (click)="fonte.set('')">Todas</button>
        @for (f of d.fontes; track f.id) {
          <button type="button" [class.ativo]="fonte() === f.id" [attr.aria-pressed]="fonte() === f.id" (click)="fonte.set(f.id)">{{ f.nome }}</button>
        }
      </nav>
      <ul class="situacao-fontes texto-suave" aria-label="Situação das fontes">
        @for (f of d.fontes; track f.id) {
          <li [class.falhou]="f.erro">{{ situacao(f) }}</li>
        }
      </ul>
      @if (lista().length) {
        <ul class="noticias">
          @for (n of lista(); track n.link) {
            <li class="noticia">
              <a [href]="n.link" target="_blank" rel="noopener noreferrer">{{ n.titulo }}<span class="so-leitor"> (abre o site {{ n.fonteNome }} em outra aba)</span></a>
              <span class="texto-suave meta">{{ n.fonteNome }}@if (quando(n); as q) { · {{ q }}}</span>
            </li>
          }
        </ul>
      } @else {
        <p class="vazio">Nenhuma notícia guardada ainda. Clique em “Atualizar agora”, ou espere a próxima busca (a cada 45 minutos).</p>
      }
    } @else {
      <p class="texto-suave" role="status">Carregando…</p>
    }
  `,
  styles: [`
    .situacao-fontes { list-style: none; margin: 12px 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px 16px; font-size: 0.85rem; }
    .situacao-fontes .falhou { color: var(--negativo); }
    .noticias { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; }
    .noticia { display: grid; gap: 2px; padding: 10px 12px; border-radius: 10px; background: var(--superficie); }
    .noticia a { font-weight: 600; color: inherit; text-decoration: none; }
    .noticia a:hover, .noticia a:focus-visible { text-decoration: underline; }
    .meta { font-size: 0.82rem; }
  `]
})
export class NoticiasComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  readonly dados = signal<ListaNoticias | null>(null);
  readonly fonte = signal('');
  readonly ocupado = signal(false);
  readonly lista = computed(() => filtrarPorFonte(this.dados()?.noticias ?? [], this.fonte()));
  readonly situacao = situacaoDaFonte;
  readonly quando = quandoFoiPublicada;

  ngOnInit(): void {
    this.api.noticias().subscribe({ next: (d) => this.dados.set(d), error: (e) => this.avisos.erro(mensagemDeErro(e)) });
  }

  atualizar(): void {
    this.ocupado.set(true);
    this.api.atualizarNoticias().subscribe({
      next: (d) => {
        this.ocupado.set(false);
        this.dados.set(d);
        const falhas = d.fontes.filter((f) => f.erro).map((f) => f.nome);
        if (falhas.length) this.avisos.erro(`Não deu para atualizar: ${falhas.join(', ')}. As outras fontes foram atualizadas.`);
        else this.avisos.sucesso('Notícias atualizadas.');
      },
      error: (e) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }
}
