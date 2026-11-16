import { Router } from '@angular/router';
import { AvisosService } from '../core/avisos.service';
import { LancamentosSincronizados, ResultadoSincronizacao } from '../core/modelos';
import { paramsTransacoes } from './utilidades';

/**
 * #688: o aviso de sucesso de uma sincronização (o botão da barra de cima, o da conexão e o "Adicionar" em
 * Configurações). Com nova ou atualizada, ele leva às Transações só nesses lançamentos (Nielsen 1, visibilidade do
 * estado; Shneiderman, feedback informativo: o número sozinho não diz o que mudou).
 */
export interface AvisoDaSincronizacao {
  texto: string;
  /** Query params de /transacoes para a ação "Ver em Transações"; null: o aviso fica sem ação. */
  verEmTransacoes: Record<string, string> | null;
}

/** Rótulo da ação: o mesmo do aviso dos trocos no Painel (Nielsen 4, consistência). */
export const ROTULO_VER_EM_TRANSACOES = 'Ver em Transações';

/** Mais de 200: a lista de ids não vai na URL, e a ação abre o período inteiro (o que a issue sugeriu). */
export const TEXTO_MUITOS = ' São muitos para listar um a um: a ação abre o período deles.';

export function textoDaSincronizacao(r: ResultadoSincronizacao): string {
  return `Sincronizado: ${r.transacoesNovas} nova(s), ${r.transacoesAtualizadas} atualizada(s).`;
}

export function textoDaConexaoAdicionada(r: ResultadoSincronizacao): string {
  return `Conexão adicionada: ${r.transacoesNovas} transação(ões) importada(s).`;
}

/** Os params de /transacoes: os ids com o chip da origem, ou só o período quando passaram do limite. */
export function paramsDaSincronizacao(l: LancamentosSincronizados | null | undefined): Record<string, string> | null {
  if (!l) return null;
  if (!l.ids.length) return paramsTransacoes({ inicio: l.inicio, fim: l.fim });
  return paramsTransacoes({ inicio: l.inicio, fim: l.fim, ids: l.ids, origemIds: 'sincronizacao' });
}

/** O aviso inteiro; `texto` é o do botão que sincronizou (por padrão, o de "Sincronizar"). */
export function avisoDaSincronizacao(r: ResultadoSincronizacao, texto = textoDaSincronizacao(r)): AvisoDaSincronizacao {
  const verEmTransacoes = paramsDaSincronizacao(r.lancamentos);
  const muitos = !!r.lancamentos && !r.lancamentos.ids.length;
  return { texto: muitos ? texto + TEXTO_MUITOS : texto, verEmTransacoes };
}

/** Mostra o aviso: o mesmo para a barra de cima e para Configurações. */
export function avisarSincronizacao(avisos: AvisosService, router: Router, r: ResultadoSincronizacao, texto?: string): void {
  const aviso = avisoDaSincronizacao(r, texto);
  const queryParams = aviso.verEmTransacoes;
  if (!queryParams) {
    avisos.sucesso(aviso.texto);
    return;
  }
  avisos.sucesso(aviso.texto, {
    rotulo: ROTULO_VER_EM_TRANSACOES,
    executar: () => void router.navigate(['/transacoes'], { queryParams })
  });
}
