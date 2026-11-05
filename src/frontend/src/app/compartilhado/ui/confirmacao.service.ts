import { Injectable, signal } from '@angular/core';

/** O que a confirmação mostra. Só a mensagem é obrigatória. */
export interface PedidoConfirmacao {
  /** A pergunta, com o que vai acontecer: "Excluir a meta Viagem?". */
  mensagem: string;
  /** O título da janela (padrão: "Confirmar"). */
  titulo?: string;
  /** O texto do botão que confirma, com o verbo da ação (padrão: "Confirmar"). */
  confirmar?: string;
  /** Ação que apaga ou não tem volta: o botão de confirmar sai vermelho (padrão: false). */
  perigo?: boolean;
}

/** O pedido aberto, com quem responde a ele. */
export interface ConfirmacaoAberta extends PedidoConfirmacao {
  responder(ok: boolean): void;
}

/**
 * #631 (ADR 0057): a confirmação do site, no lugar do `confirm()` do navegador. Uma tela chama
 * `await this.confirmacao.confirmar({ mensagem, confirmar: 'Excluir', perigo: true })` e recebe `true` só se a pessoa
 * confirmou; fechar, Cancelar e Esc dão `false`. Quem desenha a janela é o `<app-confirmacao>`, uma vez só no `layout`.
 *
 * Um pedido novo com outro ainda aberto responde `false` ao anterior: só uma confirmação por vez.
 */
@Injectable({ providedIn: 'root' })
export class ConfirmacaoService {
  readonly aberta = signal<ConfirmacaoAberta | null>(null);

  confirmar(pedido: string | PedidoConfirmacao): Promise<boolean> {
    const p = typeof pedido === 'string' ? { mensagem: pedido } : pedido;
    this.aberta()?.responder(false);
    return new Promise<boolean>((resolve) => {
      const atual: ConfirmacaoAberta = {
        ...p,
        responder: (ok) => {
          if (this.aberta() === atual) this.aberta.set(null);
          resolve(ok);
        }
      };
      this.aberta.set(atual);
    });
  }
}
