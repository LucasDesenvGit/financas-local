import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, ElementRef, OnInit, computed, inject, signal, viewChild, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../core/api.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { InfoSistema, Parametro, TipoParametro } from '../../core/modelos';
import { AvisosService } from '../../core/avisos.service';
import { mudaParametroComSenha } from './parametrizacao';

interface GrupoParametros { nome: string; itens: Parametro[]; }

/** Aba "Parametrização" de Configurações: parâmetros editáveis (catálogo da API) e diagnóstico do sistema. */
@Component({
  selector: 'app-parametrizacao',
  imports: [FormsModule, DatePipe, DecimalPipe],
  template: `
    <form (ngSubmit)="salvar()">
      @for (g of grupos(); track g.nome) {
        <section class="painel-bloco">
          <h2>{{ g.nome }}</h2>
          <div class="parametros">
            @for (p of g.itens; track p.chave) {
              <div class="parametro" [class.alterado]="valores[p.chave] !== p.valor">
                <div class="parametro-texto">
                  <label [for]="p.chave"><strong>{{ p.nome }}</strong></label>
                  <p class="texto-suave">{{ p.descricao }}</p>
                </div>
                <div class="parametro-campo">
                  @if (p.tipo === Tipo.Booleano) {
                    <label class="checkbox">
                      <input type="checkbox" [id]="p.chave" [name]="p.chave"
                        [ngModel]="valores[p.chave] === 1" (ngModelChange)="valores[p.chave] = $event ? 1 : 0">
                      {{ valores[p.chave] === 1 ? 'Ligada' : 'Desligada' }}
                    </label>
                  } @else {
                    <div class="campo-unidade">
                      <input type="number" [id]="p.chave" [name]="p.chave" [(ngModel)]="valores[p.chave]"
                        [min]="p.minimo" [max]="p.maximo" step="1" required>
                      @if (p.unidade) { <span class="texto-suave">{{ p.unidade }}</span> }
                    </div>
                    <small class="texto-suave">de {{ p.minimo }} a {{ p.maximo }}</small>
                  }
                  @if (valores[p.chave] !== p.padrao) {
                    <button type="button" class="link pequeno" (click)="valores[p.chave] = p.padrao">
                      restaurar padrão ({{ rotuloValor(p, p.padrao) }})
                    </button>
                  }
                </div>
              </div>
            }
          </div>
        </section>
      }

      @if (grupos().length) {
        <div class="rodape-form barra-salvar" [class.flutuante]="alterados()">
          @if (pedeSenha()) {
            <div class="campo-senha">
              <label for="senha-parametros"><strong>Senha atual</strong></label>
              <span class="texto-suave pequeno" id="dica-senha-parametros">Mudar a segurança do login pede a sua senha.</span>
              <input #campoSenha id="senha-parametros" name="senhaAtual" type="password" autocomplete="current-password" required
                [ngModel]="senhaAtual" (ngModelChange)="senhaAtual = $event; erroSenha.set('')"
                [attr.aria-invalid]="erroSenha() ? 'true' : null"
                [attr.aria-describedby]="erroSenha() ? 'dica-senha-parametros erro-senha-parametros' : 'dica-senha-parametros'">
              @if (erroSenha()) { <p class="erro pequeno" id="erro-senha-parametros" role="alert">{{ erroSenha() }}</p> }
            </div>
          }
          <span class="texto-suave cresce">
            {{ alterados() ? alterados() + ' alteração(ões) não salva(s)' : 'Tudo salvo.' }}
            As mudanças valem na hora, sem reiniciar.
          </span>
          <button type="button" class="botao" (click)="desfazer()" [disabled]="!alterados()">Desfazer</button>
          <button class="botao botao-primario" [disabled]="!alterados() || salvando()">Salvar parâmetros</button>
        </div>
      }
    </form>

    @if (sistema(); as s) {
      <section class="painel-bloco">
        <h2>Sistema</h2>
        <p class="texto-suave">
          Somente leitura. Caminho do banco, pasta das chaves, porta e certificado ficam no
          <code>appsettings.Production.json</code>, gerado pelo <code>scripts/geral/3-publicar-atualizar.sh</code>,
          porque são necessários antes de o banco ser aberto.
        </p>
        <dl class="info-sistema">
          <dt>Versão</dt><dd>{{ s.versao }}</dd>
          <dt>Em execução desde</dt><dd>{{ s.iniciadoEm | date: 'dd/MM/yyyy HH:mm' }}</dd>
          <dt>Banco de dados</dt>
          <dd><code>{{ s.caminhoBanco }}</code>
            @if (s.tamanhoBancoBytes !== null) { <span class="texto-suave">({{ s.tamanhoBancoBytes / 1048576 | number: '1.1-1' }} MB)</span> }
          </dd>
          <dt>Chaves de proteção</dt><dd><code>{{ s.pastaChaves }}</code></dd>
          <dt>Registros</dt><dd>{{ s.contas }} conta(s), {{ s.transacoes | number }} transação(ões), {{ s.conexoesAtivas }} conexão(ões) ativa(s)</dd>
          <dt>Última sincronização</dt><dd>{{ s.ultimaSincronizacao ? (s.ultimaSincronizacao | date: 'dd/MM/yyyy HH:mm') : '—' }}</dd>
          <dt>Ambiente</dt><dd>{{ s.dotNet }} · {{ s.sistemaOperacional }}</dd>
        </dl>
      </section>
    }
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [`
    .parametros { display: grid; gap: 4px; }
    .parametro {
      display: grid; grid-template-columns: minmax(0, 1fr) 260px; gap: 16px; align-items: start;
      padding: 12px 10px; border-radius: var(--raio-pequeno); border-top: 1px solid var(--linha);
    }
    .parametro:first-child { border-top: 0; }
    .parametro.alterado { background: var(--primaria-suave); }
    .parametro-texto p { margin: 3px 0 0; font-size: 0.86rem; }
    .parametro-campo { display: grid; gap: 4px; justify-items: start; }
    .campo-unidade { display: flex; align-items: center; gap: 8px; }
    .campo-unidade input { width: 110px; }
    .pequeno { font-size: 0.82rem; }
    .barra-salvar {
      flex-wrap: wrap; background: var(--superficie); padding: 12px 16px;
      border-radius: var(--raio); box-shadow: var(--sombra); margin-bottom: 16px;
    }
    .barra-salvar > span { min-width: 220px; }
    .campo-senha { flex-basis: 100%; display: grid; gap: 4px; justify-items: start; }
    .campo-senha input { width: min(100%, 260px); }
    .campo-senha p { margin: 0; }
    /* Só acompanha a rolagem quando há algo para salvar. */
    .barra-salvar.flutuante { position: sticky; bottom: 12px; z-index: 2; box-shadow: var(--sombra-forte); }
    .info-sistema { display: grid; grid-template-columns: 190px minmax(0, 1fr); gap: 8px 16px; margin: 0; }
    .info-sistema dt { color: var(--texto-suave); }
    .info-sistema dd { margin: 0; word-break: break-all; }
    @media (max-width: 720px) {
      .parametro, .info-sistema { grid-template-columns: 1fr; }
    }
  `]
})
export class ParametrizacaoComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly Tipo = TipoParametro;
  readonly parametros = signal<Parametro[]>([]);
  readonly sistema = signal<InfoSistema | null>(null);
  readonly salvando = signal(false);
  /** Valores em edição (o sinal "parametros" guarda os salvos). */
  valores: Record<string, number> = {};
  /** #68: a senha atual, pedida só quando muda um parâmetro de segurança; o erro dela aparece embaixo do campo. */
  senhaAtual = '';
  readonly erroSenha = signal('');
  private readonly campoSenha = viewChild<ElementRef<HTMLInputElement>>('campoSenha');

  readonly grupos = computed<GrupoParametros[]>(() => {
    const grupos: GrupoParametros[] = [];
    for (const p of this.parametros()) {
      let g = grupos.find((x) => x.nome === p.grupo);
      if (!g) grupos.push((g = { nome: p.grupo, itens: [] }));
      g.itens.push(p);
    }
    return grupos;
  });

  ngOnInit(): void {
    this.api.parametros().subscribe({
      next: (p) => this.carregar(p),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
    this.api.infoSistema().subscribe({ next: (s) => this.sistema.set(s), error: () => this.sistema.set(null) });
  }

  alterados(): number {
    return this.parametros().filter((p) => Number(this.valores[p.chave]) !== p.valor).length;
  }

  /** #68: a edição muda algum parâmetro de segurança, e o salvar pede a senha atual. */
  pedeSenha(): boolean {
    return mudaParametroComSenha(this.parametros(), this.valores);
  }

  desfazer(): void {
    this.carregar(this.parametros());
  }

  salvar(): void {
    const mudancas: Record<string, number> = {};
    for (const p of this.parametros()) {
      const v = Number(this.valores[p.chave]);
      if (v === p.valor) continue;
      if (!Number.isInteger(v) || (p.minimo !== null && v < p.minimo) || (p.maximo !== null && v > p.maximo)) {
        this.avisos.erro(`${p.nome}: informe um valor entre ${p.minimo} e ${p.maximo}.`);
        return;
      }
      mudancas[p.chave] = v;
    }
    if (!Object.keys(mudancas).length) return;
    const pedeSenha = this.pedeSenha();
    if (pedeSenha && !this.senhaAtual) {
      this.mostrarErroSenha('Informe a senha atual para mudar os parâmetros de segurança.');
      return;
    }
    this.salvando.set(true);
    this.api.salvarParametros(mudancas, pedeSenha ? this.senhaAtual : undefined).subscribe({
      next: (p) => {
        this.salvando.set(false);
        this.carregar(p);
        this.avisos.sucesso('Parâmetros salvos.');
      },
      error: (e) => {
        this.salvando.set(false);
        if (pedeSenha) this.mostrarErroSenha(mensagemDeErro(e));
        else this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  rotuloValor(p: Parametro, valor: number): string {
    if (p.tipo === TipoParametro.Booleano) return valor === 1 ? 'ligada' : 'desligada';
    return p.unidade ? `${valor} ${p.unidade}` : String(valor);
  }

  private carregar(lista: Parametro[]): void {
    this.parametros.set(lista);
    this.valores = Object.fromEntries(lista.map((p) => [p.chave, p.valor]));
    this.senhaAtual = '';
    this.erroSenha.set('');
  }

  /** O erro fica embaixo do campo da senha, que recebe o foco (WCAG 3.3.1). */
  private mostrarErroSenha(mensagem: string): void {
    this.erroSenha.set(mensagem);
    setTimeout(() => this.campoSenha()?.nativeElement.focus());
  }
}
