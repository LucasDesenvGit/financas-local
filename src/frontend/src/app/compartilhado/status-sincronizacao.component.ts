import { DatePipe } from '@angular/common';
import { Component, OnDestroy, OnInit, computed, effect, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { StatusSincronizacaoService } from '../core/status-sincronizacao.service';
import { DicaDirective } from './dica.directive';
import { IconeComponent } from './icone.component';
import { Icones } from './icones';
import { tempoRelativo } from './utilidades';

/**
 * UX-8: status da sincronização na barra superior, que também é o botão de sincronizar.
 * - normal: "⟳ Atualizado há 2 h" (data e hora completas na dica); no celular só o ícone;
 * - sincronizando (manual ou automática): ícone girando, "Sincronizando…", botão desabilitado;
 * - com problema: alerta "⚠ 1 conexão com erro" ao lado, que leva às conexões (Configurações › Open Finance).
 * Mudanças de estado são anunciadas a leitores de tela (aria-live="polite"). Nielsen 1.
 */
@Component({
  selector: 'app-status-sincronizacao',
  imports: [RouterLink, DicaDirective, IconeComponent],
  host: { class: 'status-sincronizacao' },
  template: `
    @if (alerta(); as a) {
      <a class="botao-barra alerta-sincronizacao" routerLink="/configuracoes" [attr.aria-label]="a + ': ver conexões'" animate.leave="sair"
        [appDica]="a + ' — ver conexões'" dicaLado="abaixo">
        <app-icone [icone]="Icones.alerta" [tamanho]="18" />
        <span class="rotulo-barra">{{ a }}</span>
      </a>
    }
    <button type="button" class="botao-barra botao-sincronizar" (click)="servico.sincronizar()"
      [disabled]="servico.sincronizando()" [attr.aria-label]="'Sincronizar bancos. ' + descricao()"
      [appDica]="dica()" dicaLado="abaixo">
      <app-icone [icone]="Icones.sincronizar" [tamanho]="18" [class.girando]="servico.sincronizando()" />
      <span class="rotulo-barra">{{ rotulo() }}</span>
    </button>
    <span class="so-leitor" aria-live="polite">{{ anuncio() }}</span>
  `
})
export class StatusSincronizacaoComponent implements OnInit, OnDestroy {
  readonly servico = inject(StatusSincronizacaoService);
  private readonly datas = new DatePipe('pt-BR');
  readonly Icones = Icones;
  readonly anuncio = signal('');

  /** "há 2 h", "ontem"… (avança sozinho com o relógio do serviço). */
  private readonly quando = computed(() => {
    const u = this.servico.ultima();
    return u ? tempoRelativo(u, this.servico.agora()) : null;
  });

  readonly rotulo = computed(() => {
    if (this.servico.sincronizando()) return 'Sincronizando…';
    if (!this.servico.status()) return 'Sincronizar';
    const q = this.quando();
    return q ? `Atualizado ${q}` : 'Nunca sincronizado';
  });

  readonly descricao = computed(() => {
    if (this.servico.sincronizando()) return 'Sincronizando agora.';
    const u = this.servico.ultima();
    return u ? `Última sincronização ${this.quando()}, em ${this.datas.transform(u, 'dd/MM/yyyy HH:mm')}.` : 'Ainda não sincronizado.';
  });

  readonly dica = computed(() => {
    if (this.servico.sincronizando()) return 'Sincronizando os bancos…';
    const u = this.servico.ultima();
    return u ? `Última sincronização: ${this.datas.transform(u, 'dd/MM/yyyy HH:mm')} · clique para sincronizar`
      : 'Clique para sincronizar';
  });

  /** Conexão com erro tem prioridade sobre consentimento vencendo (a mesma regra dos alertas do Painel). */
  readonly alerta = computed(() => {
    const erros = this.servico.conexoesComErro();
    if (erros > 0) return erros === 1 ? '1 conexão com erro' : `${erros} conexões com erro`;
    const vencendo = this.servico.consentimentosVencendo();
    if (vencendo > 0) return vencendo === 1 ? 'Consentimento vencendo' : `${vencendo} consentimentos vencendo`;
    return null;
  });

  constructor() {
    // Anuncia só as mudanças de estado (não o relógio): começou, terminou, apareceu ou sumiu um alerta.
    let anterior: { sincronizando: boolean; alerta: string | null } | null = null;
    effect(() => {
      const atual = { sincronizando: this.servico.sincronizando(), alerta: this.alerta() };
      if (anterior) {
        if (atual.sincronizando && !anterior.sincronizando) this.anuncio.set('Sincronizando os bancos…');
        else if (!atual.sincronizando && anterior.sincronizando) this.anuncio.set('Sincronização concluída.');
        else if (atual.alerta !== anterior.alerta) this.anuncio.set(atual.alerta ?? 'Conexões sem pendências.');
      }
      anterior = atual;
    });
  }

  ngOnInit(): void {
    this.servico.iniciar();
  }

  ngOnDestroy(): void {
    this.servico.parar();
  }
}
