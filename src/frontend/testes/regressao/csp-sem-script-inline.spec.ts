import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * #157 (ADR 0034): a CSP da API tem script-src 'self', sem 'unsafe-inline'. Com "subresourceIntegrity" ligado, o
 * build do Angular põe no index.html um <script type="importmap"> inline com o hash dos chunks; a CSP o bloqueava e
 * o console mostrava erro em toda página. Nada pode gerar script inline no index.html publicado.
 */
describe('Regressão: index.html sem script inline que a CSP bloqueie', () => {
  const raiz = join(__dirname, '../..');

  it('o build de produção não liga o subresourceIntegrity (ele gera o import map inline)', () => {
    const angular = JSON.parse(readFileSync(join(raiz, 'angular.json'), 'utf8'));
    for (const projeto of Object.values<any>(angular.projects)) {
      const build = projeto.architect.build;
      expect(build.options?.subresourceIntegrity).not.toBe(true);
      for (const config of Object.values<any>(build.configurations ?? {})) expect(config.subresourceIntegrity).not.toBe(true);
    }
  });

  it('o index.html do código-fonte não tem <script> escrito dentro dele', () => {
    const html = readFileSync(join(raiz, 'src/index.html'), 'utf8');
    const inline = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)].filter(([, , corpo]) => corpo.trim());
    expect(inline).toEqual([]);
  });
});
