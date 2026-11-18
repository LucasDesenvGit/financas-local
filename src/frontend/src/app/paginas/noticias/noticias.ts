import { FonteNoticias, Noticia } from '../../core/modelos';
import { tempoRelativo } from '../../compartilhado/utilidades';

/** #10 (ADR 0067): as regras da tela Notícias, sem Angular (ADR 0059). */

/** A data da API vem em UTC sem fuso ("2026-11-14T12:30:00"): lida como UTC. */
export function dataUtc(texto: string): Date {
  return new Date(/[zZ]|[+-]\d\d:\d\d$/.test(texto) ? texto : `${texto}Z`);
}

/** As notícias de uma fonte ('' = todas). */
export function filtrarPorFonte(lista: Noticia[], fonte: string): Noticia[] {
  return fonte ? lista.filter((n) => n.fonte === fonte) : lista;
}

/** A situação da fonte em palavras (Nielsen 1): quando atualizou e, se a última tentativa falhou, que falhou. */
export function situacaoDaFonte(f: FonteNoticias, agora: Date = new Date()): string {
  const quando = f.atualizadoEm ? `atualizado ${tempoRelativo(dataUtc(f.atualizadoEm), agora)}` : 'ainda não atualizado';
  return f.erro ? `${f.nome}: ${quando}; a última tentativa falhou` : `${f.nome}: ${quando}`;
}

/** "há 2 h" para a notícia, ou "" sem data. */
export function quandoFoiPublicada(n: Noticia, agora: Date = new Date()): string {
  return n.publicadaEm ? tempoRelativo(dataUtc(n.publicadaEm), agora) : '';
}
