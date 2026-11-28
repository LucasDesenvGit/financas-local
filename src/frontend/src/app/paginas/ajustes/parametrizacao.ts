import { Parametro } from '../../core/modelos';

/**
 * #68: a edição muda algum parâmetro de segurança (`pedeSenha`, decidido pela API)? Então a tela pede a senha atual
 * antes de salvar. Voltar ao valor salvo não conta como mudança.
 */
export function mudaParametroComSenha(parametros: Parametro[], valores: Record<string, number>): boolean {
  return parametros.some((p) => p.pedeSenha && Number(valores[p.chave]) !== p.valor);
}
