import qrcode from 'qrcode-generator';
import { SituacaoCelular } from '../../core/modelos';

/** #677: a margem branca em volta do QR code, em módulos (a "zona quieta" de 4 que a norma pede para a câmera achar). */
export const MARGEM_DO_QR = 4;

/** O desenho de um QR code em SVG: o lado (com a margem), em módulos, e o caminho dos módulos escuros. */
export interface DesenhoQr {
  lado: number;
  caminho: string;
}

/**
 * #677: o QR code do texto (um endereço), com correção de erro média (M: ~15% do código pode estar borrado). O caminho
 * tem um quadradinho por módulo escuro ("M x y h1 v1 h-1 z"), já deslocado pela margem: o SVG desenha num <path> só,
 * sem innerHTML (a CSP e o sanitizador do Angular continuam iguais).
 */
export function desenharQr(texto: string): DesenhoQr {
  const qr = qrcode(0, 'M');
  qr.addData(texto, 'Byte');
  qr.make();
  const n = qr.getModuleCount();
  const partes: string[] = [];
  for (let linha = 0; linha < n; linha++) {
    for (let coluna = 0; coluna < n; coluna++) {
      if (qr.isDark(linha, coluna)) partes.push(`M${coluna + MARGEM_DO_QR} ${linha + MARGEM_DO_QR}h1v1h-1z`);
    }
  }
  return { lado: n + 2 * MARGEM_DO_QR, caminho: partes.join('') };
}

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
