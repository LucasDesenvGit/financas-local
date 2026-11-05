import { TestBed } from '@angular/core/testing';
import { Categoria, SugestaoCategoria } from '../../../src/app/core/modelos';
import {
  LinhaSugestao, MudancaSugestao, comMudanca, conflitosPorLinha, linhasDeSugestao
} from '../../../src/app/paginas/movimentacao/transacoes';
import { TransacoesSugestoesComponent } from '../../../src/app/paginas/movimentacao/transacoes-sugestoes.component';

/**
 * #475 (ADR 0059): a janela "Sugestões de categoria" é um componente de apresentação. Ela avisa cada mudança numa linha
 * (`mudar`) e não mexe nas linhas que recebe; a tela refaz a lista (`comMudanca`) e recalcula os conflitos de regra
 * (`conflitosPorLinha`, #254). O caminho inteiro, com o conflito aparecendo, está no navegador (sugestoes-categoria.spec.ts).
 */
const CATEGORIAS = [{ id: 12, nome: 'Mercado' }, { id: 20, nome: 'Lazer' }] as Categoria[];

const sugestao = (campos: Partial<SugestaoCategoria>): SugestaoCategoria => ({
  chave: 'a', exemplo: 'IFOOD *PEDIDO', quantidade: 5, total: -80, transacaoIds: [1, 2], categoriaId: 12, categoriaNome: 'Mercado',
  categoriaCor: '#f2d98b', confianca: 'alta', motivo: null, textoRegra: 'IFOOD', alternativas: [], ...campos
});

describe('Regressão: sugestões de categoria como componente de apresentação (#475)', () => {
  it('comMudanca troca só a linha da chave e devolve uma lista nova', () => {
    const linhas = linhasDeSugestao([sugestao({ chave: 'a' }), sugestao({ chave: 'b', textoRegra: 'UBER' })]);
    const depois = comMudanca(linhas, 'b', { escolhida: 20, criarRegra: false });
    expect(depois).not.toBe(linhas);
    expect(depois[0]).toBe(linhas[0]);
    expect(depois[1]).toMatchObject({ chave: 'b', escolhida: 20, criarRegra: false });
    expect(linhas[1]).toMatchObject({ escolhida: 12, criarRegra: true });
  });

  it('conflitosPorLinha: marcar "criar regra" com outra categoria na mesma regra acusa o conflito nas duas', () => {
    let linhas = linhasDeSugestao([sugestao({ chave: 'a' }), sugestao({ chave: 'b', categoriaId: 20 })]);
    expect([...conflitosPorLinha(linhas, new Map(), CATEGORIAS).values()]).toEqual([null, null]);
    linhas = comMudanca(comMudanca(linhas, 'a', { criarRegra: true }), 'b', { criarRegra: true });
    expect(Object.fromEntries(conflitosPorLinha(linhas, new Map(), CATEGORIAS))).toEqual({ a: 'Lazer', b: 'Mercado' });
  });

  describe('a janela', () => {
    async function abrir(linhas: LinhaSugestao[], conflitos = new Map<string, string | null>()) {
      const fixture = TestBed.createComponent(TransacoesSugestoesComponent);
      fixture.componentRef.setInput('aberta', true);
      fixture.componentRef.setInput('linhas', linhas);
      fixture.componentRef.setInput('categorias', [{ id: 12, rotulo: 'Mercado' }, { id: 20, rotulo: 'Lazer' }]);
      fixture.componentRef.setInput('conflitos', conflitos);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      const mudancas: { chave: string; mudanca: MudancaSugestao }[] = [];
      const aplicadas: LinhaSugestao[] = [];
      fixture.componentInstance.mudar.subscribe((m) => mudancas.push(m));
      fixture.componentInstance.aplicar.subscribe((l) => aplicadas.push(l));
      return { fixture, raiz: fixture.nativeElement as HTMLElement, mudancas, aplicadas };
    }

    it('trocar a categoria, marcar "criar regra" ou clicar na alternativa só avisa; a linha recebida não muda', async () => {
      const linhas = linhasDeSugestao([sugestao({
        alternativas: [{ categoriaId: 20, nome: 'Lazer', cor: '#f3b3d3', motivo: 'palavra-chave' }]
      })]);
      const t = await abrir(linhas);
      const select = t.raiz.querySelector<HTMLSelectElement>('tbody select')!;
      select.selectedIndex = 2;
      select.dispatchEvent(new Event('change'));
      t.raiz.querySelector<HTMLInputElement>('tbody input[type=checkbox]')!.click();
      t.raiz.querySelector<HTMLButtonElement>('.chip-sugestao')!.click();
      expect(t.mudancas).toEqual([
        { chave: 'a', mudanca: { escolhida: 20 } },
        { chave: 'a', mudanca: { criarRegra: false } },
        { chave: 'a', mudanca: { escolhida: 20 } }
      ]);
      expect(linhas[0]).toMatchObject({ escolhida: 12, criarRegra: true });
    });

    it('"Aplicar" avisa com a linha; o conflito vindo da tela aparece em texto embaixo da regra', async () => {
      const linhas = linhasDeSugestao([sugestao({})]);
      const t = await abrir(linhas, new Map([['a', 'Lazer']]));
      expect(t.raiz.querySelector('.aviso-regra')!.textContent).toContain('Outra linha cria esta regra para Lazer');
      [...t.raiz.querySelectorAll<HTMLButtonElement>('tbody button')].find((b) => b.textContent!.trim() === 'Aplicar')!.click();
      expect(t.aplicadas).toEqual([linhas[0]]);
    });
  });
});
