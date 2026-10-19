import { Component, Injectable, inject, signal, ChangeDetectionStrategy } from '@angular/core';

/** Ação opcional do aviso (ex.: "Desfazer"); clicar executa e fecha o aviso. */
export interface AcaoAviso { rotulo: string; executar: () => void; }

export interface Aviso { id: number; texto: string; tipo: 'sucesso' | 'erro' | 'info'; acao?: AcaoAviso; }

@Injectable({ providedIn: 'root' })
export class AvisosService {
  /** Aviso com ação fica mais tempo: dá para ler e decidir (ex.: desfazer). */
  static readonly DURACAO_COM_ACAO = 10000;

  private sequencia = 0;
  private readonly timers = new Map<number, ReturnType<typeof setTimeout>>();
  private readonly duracoes = new Map<number, number>();
  readonly avisos = signal<Aviso[]>([]);

  sucesso(texto: string, acao?: AcaoAviso): void { this.adicionar(texto, 'sucesso', acao ? AvisosService.DURACAO_COM_ACAO : 4000, acao); }
  erro(texto: string): void { this.adicionar(texto, 'erro', 7000); }
  info(texto: string): void { this.adicionar(texto, 'info'); }

  fechar(id: number): void {
    clearTimeout(this.timers.get(id));
    this.timers.delete(id);
    this.duracoes.delete(id);
    this.avisos.update((lista) => lista.filter((a) => a.id !== id));
  }

  executar(aviso: Aviso): void {
    this.fechar(aviso.id);
    aviso.acao?.executar();
  }

  /** Mouse ou foco no aviso: ele não some (WCAG 2.2.1, tempo ajustável; 1.4.13, conteúdo persistente). */
  pausar(id: number): void {
    clearTimeout(this.timers.get(id));
    this.timers.delete(id);
  }

  /** Saiu do aviso: volta a contar o tempo inteiro. */
  retomar(id: number): void {
    const duracao = this.duracoes.get(id);
    if (duracao !== undefined && !this.timers.has(id)) this.agendar(id, duracao);
  }

  private adicionar(texto: string, tipo: Aviso['tipo'], duracao = 4000, acao?: AcaoAviso): void {
    const id = ++this.sequencia;
    this.avisos.update((lista) => [...lista, { id, texto, tipo, acao }]);
    this.duracoes.set(id, duracao);
    this.agendar(id, duracao);
  }

  private agendar(id: number, duracao: number): void {
    this.timers.set(id, setTimeout(() => this.fechar(id), duracao));
  }
}

@Component({
  selector: 'app-avisos',
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="avisos" aria-live="polite">
      @for (a of servico.avisos(); track a.id) {
        <div class="aviso" [class]="'aviso aviso-' + a.tipo" role="status" animate.leave="sair"
          (mouseenter)="servico.pausar(a.id)" (mouseleave)="servico.retomar(a.id)"
          (focusin)="servico.pausar(a.id)" (focusout)="servico.retomar(a.id)">
          <span>{{ a.texto }}</span>
          @if (a.acao) {
            <button type="button" class="aviso-acao" (click)="servico.executar(a)">{{ a.acao.rotulo }}</button>
          }
          <button type="button" class="aviso-fechar" (click)="servico.fechar(a.id)" aria-label="Fechar">×</button>
        </div>
      }
    </div>
  `
})
export class AvisosComponent {
  readonly servico = inject(AvisosService);
}
