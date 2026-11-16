import { TestBed } from '@angular/core/testing';
import { PosicaoAtivo, TipoAtivo, TipoOperacao } from '../../../src/app/core/modelos';
import { InvestimentosAtivoComponent } from '../../../src/app/paginas/patrimonio/investimentos-ativo.component';
import { InvestimentosImportacaoComponent } from '../../../src/app/paginas/patrimonio/investimentos-importacao.component';
import { LinhaImportacao } from '../../../src/app/paginas/patrimonio/investimentos';

/**
 * #731 (ADR 0059): as janelas "Ativo" e "Importar do extrato" de Investimentos são componentes de apresentação. Editam uma
 * cópia do que recebem e avisam a tela com ela; fechar sem salvar não muda nada (Nielsen 3). A conferência das linhas e as
 * chamadas da API são da tela (regra em investimentos.ts, testada em investimentos-logica.spec.ts).
 */
const ativo = { id: 1, ticker: 'GARE11', nome: 'Fundo', tipo: TipoAtivo.Fii, moeda: 'BRL', segmento: null, precoManual: false,
  precoAtual: 10 } as PosicaoAtivo;

const linha = (campos: Partial<LinhaImportacao> = {}): LinhaImportacao => ({
  transacaoId: 1, data: '2026-11-03', descricao: 'COMPRA MXRF11', valor: 98.5, contaNome: 'Corretora', ticker: 'MXRF11',
  tipo: TipoOperacao.Compra, quantidade: 10, precoUnitario: 9.85, fechamentoNoDia: null, origem: null, jaImportada: false,
  possivelDuplicata: false, selecionada: true, taxas: 0, ...campos
});

async function estavel(fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('Regressão: janela do ativo como componente de apresentação (#731)', () => {
  async function abrir(a: PosicaoAtivo | null) {
    const fixture = TestBed.createComponent(InvestimentosAtivoComponent);
    fixture.componentRef.setInput('ativo', a);
    await estavel(fixture);
    const salvos: unknown[] = [];
    let fechou = 0;
    let excluiu = 0;
    fixture.componentInstance.salvar.subscribe((f) => salvos.push(f));
    fixture.componentInstance.fechar.subscribe(() => fechou++);
    fixture.componentInstance.excluir.subscribe(() => excluiu++);
    return { fixture, raiz: fixture.nativeElement as HTMLElement, salvos, fechou: () => fechou, excluiu: () => excluiu };
  }

  it('sem ativo, a janela fica fechada', async () => {
    const t = await abrir(null);
    expect(t.raiz.querySelector('form')).toBeNull();
  });

  it('abre com os dados do ativo e avisa com a cópia editada, sem mudar o ativo recebido', async () => {
    const t = await abrir(ativo);
    expect(t.raiz.textContent).toContain('Ativo · GARE11');
    const nome = t.raiz.querySelector<HTMLInputElement>('input[name=nome]')!;
    expect(nome.value).toBe('Fundo');
    nome.value = 'Fundo novo';
    nome.dispatchEvent(new Event('input'));
    t.raiz.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit'));
    expect(t.salvos).toEqual([{ nome: 'Fundo novo', tipo: TipoAtivo.Fii, segmento: null, precoManual: false, precoAtual: 10 }]);
    expect(ativo.nome).toBe('Fundo');
  });

  it('só FII tem segmento; ativo em dólar não oferece FII nem BDR no tipo', async () => {
    const fii = await abrir(ativo);
    expect(fii.raiz.querySelector('select[name=segmento]')).not.toBeNull();
    const exterior = await abrir({ ...ativo, tipo: TipoAtivo.Acao, moeda: 'USD' });
    expect(exterior.raiz.querySelector('select[name=segmento]')).toBeNull();
    const tipos = Array.from(exterior.raiz.querySelectorAll('select[name=tipo] option')).map((o) => o.textContent!.trim());
    expect(tipos).toEqual(['Ação', 'ETF', 'Outro']);
  });

  it('Cancelar e Excluir só avisam', async () => {
    const t = await abrir(ativo);
    const botoes = Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('form button'));
    botoes.find((b) => b.textContent!.trim() === 'Cancelar')!.click();
    botoes.find((b) => b.textContent!.trim() === 'Excluir ativo')!.click();
    expect([t.fechou(), t.excluiu(), t.salvos.length]).toEqual([1, 1, 0]);
  });
});

describe('Regressão: janela de importação do extrato como componente de apresentação (#731)', () => {
  async function abrir(linhas: LinhaImportacao[]) {
    const fixture = TestBed.createComponent(InvestimentosImportacaoComponent);
    fixture.componentRef.setInput('aberta', true);
    fixture.componentRef.setInput('linhas', linhas);
    await estavel(fixture);
    const importadas: LinhaImportacao[][] = [];
    fixture.componentInstance.importar.subscribe((l) => importadas.push(l));
    return { fixture, raiz: fixture.nativeElement as HTMLElement, importadas };
  }

  it('desmarcar uma linha muda a contagem e a cópia que a janela avisa, não a lista recebida', async () => {
    const linhas = [linha({ transacaoId: 1 }), linha({ transacaoId: 2, ticker: 'HGLG11' })];
    const t = await abrir(linhas);
    expect(t.raiz.textContent).toContain('2 selecionada(s)');
    t.raiz.querySelector<HTMLInputElement>('tbody input[type=checkbox]')!.click();
    await estavel(t.fixture);
    expect(t.raiz.textContent).toContain('1 selecionada(s)');
    Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.trim() === 'Importar')!.click();
    expect(t.importadas).toHaveLength(1);
    expect(t.importadas[0].map((l) => l.selecionada)).toEqual([false, true]);
    expect(linhas.map((l) => l.selecionada)).toEqual([true, true]);
  });

  it('sem nenhuma linha marcada, "Importar" fica desabilitado; a linha já importada não se marca', async () => {
    const t = await abrir([linha({ selecionada: false }), linha({ transacaoId: 2, jaImportada: true, selecionada: false })]);
    const importar = Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.trim() === 'Importar')!;
    expect(importar.disabled).toBe(true);
    const caixas = t.raiz.querySelectorAll<HTMLInputElement>('tbody input[type=checkbox]');
    expect(caixas[1].disabled).toBe(true);
    expect(t.raiz.textContent).toContain('já importada');
  });

  it('sem linhas, explica que não achou nada; procurando, mostra o aguarde', async () => {
    const vazio = await abrir([]);
    expect(vazio.raiz.textContent).toContain('Nenhuma transação com código de ativo encontrada no extrato.');
    const fixture = TestBed.createComponent(InvestimentosImportacaoComponent);
    fixture.componentRef.setInput('aberta', true);
    fixture.componentRef.setInput('carregando', true);
    await estavel(fixture);
    expect(fixture.nativeElement.textContent).toContain('Procurando nas transações');
  });
});
