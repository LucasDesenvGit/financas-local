import { SituacaoCelular } from '../../core/modelos';

/** #328: o caminho do certificado da autoridade, que o celular baixa e instala (a API o serve sem login). */
export const CAMINHO_AUTORIDADE = '/api/celular/autoridade';

/** Os nomes do próprio computador: aberto por eles, a página não está no celular. */
const NOMES_LOCAIS = new Set(['127.0.0.1', 'localhost', '[::1]', '::1']);

/** A página foi aberta pelo IP da rede (no celular, ou em outro aparelho), e não pelo próprio computador? */
export function abertoPelaRede(hostname: string): boolean {
  return !NOMES_LOCAIS.has(hostname.trim().toLowerCase());
}

/**
 * O IP já marcado na lista: o escolhido antes, se ainda é deste computador; senão o primeiro (o da rota padrão, que a
 * API põe primeiro). Sem endereço na rede, vazio.
 */
export function ipInicial(situacao: SituacaoCelular): string {
  if (situacao.ip && situacao.enderecos.some((e) => e.ip === situacao.ip)) return situacao.ip;
  return situacao.enderecos[0]?.ip ?? '';
}

/** O endereço do certificado no celular: o do site ligado, com o caminho da autoridade. */
export function enderecoDaAutoridade(situacao: SituacaoCelular): string | null {
  return situacao.endereco ? situacao.endereco + CAMINHO_AUTORIDADE : null;
}
