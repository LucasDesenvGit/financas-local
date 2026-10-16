import { Injectable, computed, inject, signal } from '@angular/core';
import { ApiService } from './api.service';
import { mensagemDeErro } from './erro.interceptor';
import { StatusSincronizacao } from './modelos';
import { AvisosService } from '../compartilhado/avisos.component';

/**
 * UX-8: se os dados estão atualizados. Consulta o status leve da API (GET /api/open-finance/status) a cada 2 min —
 * a cada 10 s enquanto há uma sincronização rodando, inclusive a automática — e quando a aba volta a ficar visível.
 * "agora" avança a cada 30 s para o "há X min" mudar sozinho, sem recarregar a página. Nielsen 1.
 */
@Injectable({ providedIn: 'root' })
export class StatusSincronizacaoService {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);

  readonly status = signal<StatusSincronizacao | null>(null);
  /** Sincronização disparada por esta tela (o botão fica desabilitado até a resposta). */
  readonly sincronizandoAqui = signal(false);
  readonly agora = signal(new Date());
  readonly sincronizando = computed(() => this.sincronizandoAqui() || !!this.status()?.emExecucao);
  readonly ultima = computed(() => {
    const u = this.status()?.ultimaSincronizacao;
    return u ? new Date(u) : null;
  });
  readonly conexoesComErro = computed(() => this.status()?.conexoesComErro ?? 0);
  readonly consentimentosVencendo = computed(() => this.status()?.consentimentosVencendo ?? 0);

  private consulta: ReturnType<typeof setTimeout> | null = null;
  private relogio: ReturnType<typeof setInterval> | null = null;
  private usuarios = 0;
  private readonly aoVoltarParaAba = () => {
    if (document.visibilityState === 'visible') this.atualizar();
  };

  /** Começa a acompanhar (chamado pela barra superior; para quando ninguém mais usa). */
  iniciar(): void {
    if (this.usuarios++ > 0) return;
    this.atualizar();
    this.relogio = setInterval(() => this.agora.set(new Date()), 30_000);
    document.addEventListener('visibilitychange', this.aoVoltarParaAba);
  }

  parar(): void {
    if (--this.usuarios > 0) return;
    if (this.consulta) clearTimeout(this.consulta);
    if (this.relogio) clearInterval(this.relogio);
    this.consulta = this.relogio = null;
    document.removeEventListener('visibilitychange', this.aoVoltarParaAba);
  }

  atualizar(): void {
    if (this.consulta) clearTimeout(this.consulta);
    this.api.statusSincronizacao().subscribe({
      next: (s) => {
        this.status.set(s);
        this.agora.set(new Date());
        this.agendar(s.emExecucao ? 10_000 : 120_000);
      },
      error: () => this.agendar(120_000)
    });
  }

  /** Sincroniza agora (botão da barra); o aviso de resultado continua como antes. */
  sincronizar(): void {
    if (this.sincronizando()) return;
    this.sincronizandoAqui.set(true);
    this.api.sincronizar().subscribe({
      next: (r) => {
        this.sincronizandoAqui.set(false);
        if (r.sucesso) this.avisos.sucesso(`Sincronizado: ${r.transacoesNovas} nova(s), ${r.transacoesAtualizadas} atualizada(s).`);
        else this.avisos.erro(r.mensagem ?? 'Falha na sincronização.');
        this.atualizar();
      },
      error: (e) => {
        this.sincronizandoAqui.set(false);
        this.avisos.erro(mensagemDeErro(e));
        this.atualizar();
      }
    });
  }

  private agendar(ms: number): void {
    if (this.usuarios <= 0) return;
    if (this.consulta) clearTimeout(this.consulta);
    this.consulta = setTimeout(() => this.atualizar(), ms);
  }
}
