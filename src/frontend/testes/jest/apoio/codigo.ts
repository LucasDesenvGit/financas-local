import { readFileSync, readdirSync } from 'fs';
import { dirname, join, relative, sep } from 'path';

/**
 * #475 (ADR 0059): o código de src/app como os testes que o leem como texto precisam ver. Componente com o template e o
 * CSS em arquivo próprio (`templateUrl`, `styleUrl`, `styleUrls`) vem com eles juntos, como se estivessem inline; sem
 * isso, o teste que procura no template passaria sem olhar nada. O `scripts/ci/front/testes-afetados.py` acha pelo nome deste
 * arquivo os testes que o usam (eles rodam a cada mudança no front-end).
 */
export const APP = join(__dirname, '..', '..', '..', 'src', 'app');

const URLS = /\b(?:templateUrl|styleUrls?)\s*:\s*(\[[^\]]*\]|'[^']+'|"[^"]+")/g;

/** Os arquivos que o componente aponta em `templateUrl`, `styleUrl` e `styleUrls`, relativos a ele. */
export function arquivosApontados(texto: string): string[] {
  return [...texto.matchAll(URLS)].flatMap((m) => [...m[1].matchAll(/'([^']+)'|"([^"]+)"/g)].map((u) => u[1] ?? u[2]));
}

/** O texto do `.ts`, seguido do template e do CSS que ele aponta. Arquivo apontado que não existe: erro. */
export function codigo(caminhoTs: string): string {
  const ts = readFileSync(caminhoTs, 'utf8');
  return [ts, ...arquivosApontados(ts).map((a) => readFileSync(join(dirname(caminhoTs), a), 'utf8'))].join('\n');
}

/** Caminho relativo a src/app, com "/": `paginas/movimentacao/transacoes.component.ts`. */
export const relativoAoApp = (caminho: string) => relative(APP, caminho).split(sep).join('/');

/** Os `.ts` das pastas (relativas a src/app; sem pastas, src/app inteiro), cada um com o template e o CSS dele. */
export function codigosDoApp(...pastas: string[]): { arquivo: string; texto: string }[] {
  return (pastas.length ? pastas : ['.']).flatMap((p) => {
    const raiz = join(APP, p);
    return readdirSync(raiz, { recursive: true, encoding: 'utf8' })
      .filter((f) => f.endsWith('.ts'))
      .sort()
      .map((f) => ({ arquivo: relativoAoApp(join(raiz, f)), texto: codigo(join(raiz, f)) }));
  });
}
