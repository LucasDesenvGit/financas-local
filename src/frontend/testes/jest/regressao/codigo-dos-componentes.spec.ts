import { mkdtempSync, readdirSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { dirname, join } from 'path';
import { APP, arquivosApontados, codigo, codigosDoApp } from '../apoio/codigo';

/**
 * #475 (ADR 0059): os testes que leem o código como texto (template sem setinha, sem `type="date"`, clique com
 * teclado, lista suspensa…) leem pelo `apoio/codigo.ts`, que junta ao `.ts` o template e o CSS em arquivo próprio.
 * Sem isso, mover o template para um `.html` faria esses testes passarem sem olhar o template.
 */
describe('Regressão: o código dos componentes inclui o template e o CSS em arquivo próprio (#475)', () => {
  it('acha templateUrl, styleUrl e styleUrls, com aspas simples ou duplas', () => {
    expect(arquivosApontados(`@Component({ templateUrl: './a.component.html', styleUrl: "./a.component.css" })`))
      .toEqual(['./a.component.html', './a.component.css']);
    expect(arquivosApontados(`styleUrls: ['./a.css', "./b.css"]`)).toEqual(['./a.css', './b.css']);
    expect(arquivosApontados('template: `<p>inline</p>`, styles: `p {}`')).toEqual([]);
  });

  it('junta ao .ts o template e o CSS apontados; arquivo apontado que não existe é erro', () => {
    const pasta = mkdtempSync(join(tmpdir(), 'codigo-'));
    try {
      writeFileSync(join(pasta, 'a.component.ts'), `templateUrl: './a.component.html', styleUrl: './a.component.css'`);
      writeFileSync(join(pasta, 'a.component.html'), '<select [(ngModel)]="x"></select>');
      writeFileSync(join(pasta, 'a.component.css'), '.a { color: red; }');
      const texto = codigo(join(pasta, 'a.component.ts'));
      expect(texto).toContain('<select [(ngModel)]="x"></select>');
      expect(texto).toContain('.a { color: red; }');
      writeFileSync(join(pasta, 'b.component.ts'), `templateUrl: './sumiu.html'`);
      expect(() => codigo(join(pasta, 'b.component.ts'))).toThrow();
    } finally {
      rmSync(pasta, { recursive: true, force: true });
    }
  });

  it('todo .html e .css de src/app é apontado por um componente (senão os testes que leem o código não o veem)', () => {
    const apontados = new Set(codigosDoApp().flatMap(({ arquivo, texto }) =>
      arquivosApontados(texto).map((a) => join(dirname(arquivo), a).split('\\').join('/'))));
    const soltos = readdirSync(APP, { recursive: true, encoding: 'utf8' })
      .map((f) => f.split('\\').join('/'))
      .filter((f) => /\.(html|css)$/.test(f) && !apontados.has(f));
    expect(soltos).toEqual([]);
  });
});
