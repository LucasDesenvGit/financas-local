import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { Route } from '@angular/router';
import { rotas } from '../../../src/app/app.routes';
import { GLOSSARIO } from '../../../src/app/compartilhado/glossario/glossario';
import { telasDoMenu } from '../../../src/app/layout/shell.component';
import { ajudaDaUrl } from '../../../src/app/paginas/manual/ajuda-por-rota';
import { MANUAL } from '../../../src/app/paginas/manual/manual-conteudo';
import { MEDIDAS_PRINTS } from '../../../src/app/paginas/manual/prints-manual';
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
        expect(secao.topicos.length).toBeGreaterThan(0);
        for (const termo of secao.termos ?? []) expect(GLOSSARIO[termo]).toBeDefined();
      }
    }
  });

  // #817, ADR 0071: o Manual é objetivo. 1 a 4 tópicos curtos, passo a passo só com poucos itens; o resto vai em "Mais detalhes".
  // (#817: os outros capítulos entram na próxima etapa, quando forem reescritos.)
  describe('seções objetivas (#817)', () => {
    const CAPITULOS_CONVERTIDOS = ['primeiros-passos', 'transacoes', 'relatorios'];
    const secoes = MANUAL.filter((c) => CAPITULOS_CONVERTIDOS.includes(c.id)).flatMap((c) => c.secoes.map((s) => ({ onde: `${c.id}/${s.id}`, s })));

    it('os capítulos convertidos existem', () => {
      expect(secoes.length).toBeGreaterThan(15);
    });

    it('de 1 a 4 tópicos, de até 120 caracteres cada, e no máximo 6 passos (também curtos)', () => {
      for (const { onde, s } of secoes) {
        expect([onde, s.topicos.length >= 1 && s.topicos.length <= 4]).toEqual([onde, true]);
        expect(s.topicos.filter((x) => x.length > 120).map((x) => `${onde}: ${x}`)).toEqual([]);
        if (s.passos) {
          expect([onde, s.passos.length >= 1 && s.passos.length <= 6]).toEqual([onde, true]);
          expect(s.passos.filter((x) => x.length > 120).map((x) => `${onde}: ${x}`)).toEqual([]);
        }
      }
    });

    it('a linha "para que serve" é uma frase só e curta', () => {
      for (const { onde, s } of secoes) expect([onde, s.paraQueServe.length <= 160]).toEqual([onde, true]);
    });
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

  // #752, ADR 0071: os prints do Manual. Gerados por `npm run prints:manual`; aqui só se confere o que o site precisa.
  describe('prints das seções (ADR 0071)', () => {
    const pasta = join(__dirname, '..', '..', '..', 'public', 'imagens', 'manual');
    const prints = MANUAL.flatMap((c) => c.secoes.flatMap((s) => (s.print ? [s.print] : [])));
    const TETO_ARQUIVO = 50 * 1024;
    const TETO_PASTA = 2 * 1024 * 1024;

    it('texto alternativo obrigatório: descreve a tela, até 160 caracteres, sem "Imagem", "Print" nem "Foto" no começo (WCAG 1.1.1)', () => {
      expect(prints.length).toBeGreaterThan(0);
      for (const { id, alt } of prints) {
        expect(alt.trim().length).toBeGreaterThan(10);
        expect(alt.length).toBeLessThanOrEqual(160);
        expect(`${id}: ${alt}`.slice(id.length + 2)).not.toMatch(/^\s*(imagem|print|foto)/i);
      }
    });

    it('cada print tem os dois temas, as medidas e um id só', () => {
      expect(new Set(prints.map((p) => p.id)).size).toBe(prints.length);
      for (const { id } of prints) {
        for (const tema of ['claro', 'escuro']) expect([`${id}-${tema}.webp`, existsSync(join(pasta, `${id}-${tema}.webp`))]).toEqual([`${id}-${tema}.webp`, true]);
        expect([id, MEDIDAS_PRINTS[id] !== undefined]).toEqual([id, true]);
      }
    });

    it('nada sobra: todo arquivo e toda medida pertencem a uma seção', () => {
      const esperados = new Set(prints.flatMap((p) => [`${p.id}-claro.webp`, `${p.id}-escuro.webp`]));
      expect(readdirSync(pasta).filter((a) => !esperados.has(a))).toEqual([]);
      expect(Object.keys(MEDIDAS_PRINTS).filter((id) => !prints.some((p) => p.id === id))).toEqual([]);
    });

    it('peso: até 50 KiB por arquivo e 2 MiB na pasta (o repositório e o .msi crescem junto)', () => {
      const arquivos = readdirSync(pasta).map((a) => ({ a, tamanho: statSync(join(pasta, a)).size }));
      expect(arquivos.filter((f) => f.tamanho > TETO_ARQUIVO)).toEqual([]);
      expect(arquivos.reduce((soma, f) => soma + f.tamanho, 0)).toBeLessThanOrEqual(TETO_PASTA);
    });
  });
});
