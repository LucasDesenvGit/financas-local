import { Route } from '@angular/router';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { rotas } from '../../../src/app/app.routes';
import { GLOSSARIO } from '../../../src/app/compartilhado/glossario/glossario';
import { telasDoMenu } from '../../../src/app/layout/shell.component';
import { ajudaDaUrl } from '../../../src/app/paginas/manual/ajuda-por-rota';
import { MANUAL } from '../../../src/app/paginas/manual/manual-conteudo';
import { abrirManual } from '../apoio/manual';

/**
 * Regressão do Manual (#113, ADR 0026): o manual acompanha o site. Tela nova no menu sem seção no manual, link
 * "Abrir a tela" para rota que não existe ou âncora repetida quebram aqui.
 */
describe('Regressão: Manual acompanha o site', () => {
  // As rotas das telas, com as de dentro de um módulo (#210: "trabalho/salario", dentro do portão do módulo).
  const telas = (rotas.find((r) => r.path === '' && r.children)!.children as Route[])
    .flatMap((r) => (r.children ? r.children.map((f) => (f.path ? `${r.path}/${f.path}` : r.path!)) : [r.path!])) // #10: a tela no filho vazio
    .filter((p) => p && !p.startsWith('manual'));

  it('toda tela do menu tem capítulo (e seção, quando indicada) no manual, também as de dentro dos grupos', () => {
    const itens = telasDoMenu();
    expect(itens.length).toBe(20);   // #227: as 13 telas de antes, agora em grupos; as 6 do Trabalho (módulo, #209) e as Notícias (módulo, #10)
    for (const item of itens) {
      const ajuda = ajudaDaUrl(item.rota)!;
      expect(ajuda).toBeDefined();
      const capitulo = MANUAL.find((c) => c.id === ajuda.capitulo);
      expect(capitulo).toBeDefined();
      if (ajuda.secao) expect(capitulo!.secoes.some((s) => s.id === ajuda.secao)).toBe(true);
    }
  });

  it('todo "Abrir a tela" aponta para uma tela que existe', () => {
    for (const capitulo of MANUAL) {
      for (const secao of capitulo.secoes) {
        for (const link of secao.abrir ?? []) expect(telas).toContain(link.rota.slice(1));
      }
    }
  });

  it('capítulos e seções têm âncora única; termos existem no glossário', () => {
    const capitulos = MANUAL.map((c) => c.id);
    expect(new Set(capitulos).size).toBe(capitulos.length);
    for (const capitulo of MANUAL) {
      const secoes = capitulo.secoes.map((s) => s.id);
      expect(new Set(secoes).size).toBe(secoes.length);
      for (const secao of capitulo.secoes) {
        expect(secao.passos.length).toBeGreaterThan(0);
        for (const termo of secao.termos ?? []) expect(GLOSSARIO[termo]).toBeDefined();
      }
    }
  });

  it('o botão "Ajuda" acha a tela pela URL, com filtros e âncoras', () => {
    expect(ajudaDaUrl('/transacoes?categoria=sem')).toEqual({ capitulo: 'transacoes' });
    expect(ajudaDaUrl('/configuracoes?aba=erros')).toEqual({ capitulo: 'configuracoes' });
    expect(ajudaDaUrl('/painel')).toEqual({ capitulo: 'relatorios', secao: 'painel' });
    expect(ajudaDaUrl('/manual/contas#historico-faturas')).toBeNull();
  });

  it('o texto do Manual não entra no bundle inicial: nenhum import estático a partir do main.ts chega a ele (#755)', () => {
    // Os import() das rotas são lazy; o resto, seguido a partir do main.ts, vai para o main-*.js que toda página baixa.
    const src = resolve(__dirname, '../../../src');
    const vistos = new Set<string>();
    const fila = [join(src, 'main.ts')];
    while (fila.length) {
      const arquivo = fila.pop()!;
      if (vistos.has(arquivo)) continue;
      vistos.add(arquivo);
      for (const [, caminho] of readFileSync(arquivo, 'utf8').matchAll(/^(?:import|export)\s[^;]*?from\s+'(\.[^']+)'/gm)) {
        const destino = resolve(dirname(arquivo), caminho);
        fila.push(existsSync(`${destino}.ts`) ? `${destino}.ts` : join(destino, 'index.ts'));
      }
    }
    const iniciais = [...vistos].map((a) => relative(src, a).split(sep).join('/'));
    expect(iniciais).toContain('app/layout/shell.component.ts');
    expect(iniciais).toContain('app/paginas/manual/ajuda-por-rota.ts');
    expect(iniciais).not.toContain('app/paginas/manual/manual-conteudo.ts');
  });

  it('a busca vai para a URL com replaceUrl, e apagar tira o parâmetro', async () => {
    const t = await abrirManual('/manual');
    const navegar = jest.spyOn(t.router, 'navigate');
    t.tela.mudarBusca('regra');
    await t.atualizar();
    expect(navegar).toHaveBeenLastCalledWith([], expect.objectContaining({ replaceUrl: true, queryParams: { busca: 'regra' } }));
    expect(t.router.url).toBe('/manual?busca=regra');
    expect(t.raiz.querySelectorAll('.resultados-busca li').length).toBeGreaterThan(2);
    t.tela.mudarBusca('');
    await t.atualizar();
    expect(t.router.url).toBe('/manual');
    expect(t.raiz.querySelector('.grade-capitulos')).not.toBeNull();
  });

  it('capítulo que não existe avisa e leva ao índice', async () => {
    const t = await abrirManual('/manual/nao-existe');
    expect(t.raiz.querySelector('.vazio')!.textContent).toContain('Essa parte do manual não existe.');
    expect(t.raiz.querySelector('.vazio a')!.getAttribute('href')).toBe('/manual');
  });
});
