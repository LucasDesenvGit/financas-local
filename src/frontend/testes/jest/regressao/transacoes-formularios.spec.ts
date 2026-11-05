import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Conta, TipoTransacao } from '../../../src/app/core/modelos';
import { FormTransacao, FormTransferencia, formNovo } from '../../../src/app/paginas/movimentacao/transacoes';
import { TransacoesLancamentoComponent } from '../../../src/app/paginas/movimentacao/transacoes-lancamento.component';
import { TransacoesTransferenciaComponent } from '../../../src/app/paginas/movimentacao/transacoes-transferencia.component';

/**
 * #475 (ADR 0059): as janelas de lançamento e de transferência de Transações são componentes de apresentação. Recebem o
 * formulário, editam uma cópia e avisam com ela: fechar sem salvar não muda nada (Nielsen 3), e a tela é quem grava.
 */
const CONTAS = [{ id: 3, nome: 'Carteira' }, { id: 4, nome: 'Poupança' }] as Conta[];
const CATEGORIAS = [{ id: 12, rotulo: 'Mercado' }, { id: 21, rotulo: 'Lazer › Cinema' }];

async function montar<T>(tipo: Type<T>, entradas: Record<string, unknown>) {
  const fixture = TestBed.createComponent(tipo);
  for (const [nome, valor] of Object.entries(entradas)) fixture.componentRef.setInput(nome, valor);
  const assentar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  await assentar();
  const raiz = fixture.nativeElement as HTMLElement;
  const botao = (texto: string) => [...raiz.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent!.trim() === texto);
  const digitar = async (seletor: string, valor: string) => {
    const campo = raiz.querySelector<HTMLInputElement>(seletor)!;
    campo.value = valor;
    campo.dispatchEvent(new Event('input'));
    await assentar();
  };
  return { fixture, componente: fixture.componentInstance, raiz, assentar, botao, digitar };
}

const manual = (campos: Partial<FormTransacao> = {}): FormTransacao =>
  ({ ...formNovo(3, '2026-10-10'), descricao: 'Feira', valor: 50, ...campos });

describe('Regressão: janela do lançamento em Transações (#475)', () => {
  const abrir = (lancamento: FormTransacao | null, extras: Record<string, unknown> = {}) =>
    montar(TransacoesLancamentoComponent, { lancamento, contasManuais: CONTAS, categorias: CATEGORIAS, ...extras });

  it('"Salvar" avisa com a cópia editada; o lançamento recebido não muda', async () => {
    const original = manual();
    const t = await abrir(original);
    const salvos: FormTransacao[] = [];
    t.componente.salvar.subscribe((f) => salvos.push(f));
    await t.digitar('input[name=descricao]', 'Feira do sábado');
    t.botao('Salvar')!.click();
    await t.assentar();
    expect(salvos).toEqual([{ ...original, descricao: 'Feira do sábado' }]);
    expect(original.descricao).toBe('Feira');
  });

  it('"Cancelar" só avisa que fechou: o que foi digitado não volta para a tela', async () => {
    const original = manual();
    const t = await abrir(original);
    let fechou = 0;
    t.componente.fechar.subscribe(() => fechou++);
    await t.digitar('input[name=descricao]', 'outra coisa');
    t.botao('Cancelar')!.click();
    expect(fechou).toBe(1);
    expect(original.descricao).toBe('Feira');
  });

  it('outro lançamento que chega refaz a cópia (abrir outra transação não traz o que foi digitado na anterior)', async () => {
    const t = await abrir(manual());
    await t.digitar('input[name=descricao]', 'digitado');
    t.fixture.componentRef.setInput('lancamento', manual({ id: 9, descricao: 'Padaria' }));
    await t.assentar();
    expect(t.raiz.querySelector<HTMLInputElement>('input[name=descricao]')!.value).toBe('Padaria');
    expect(t.raiz.textContent).toContain('Editar transação');
  });

  it('Open Finance: valor, data e conta não aparecem, com o porquê; sem "Excluir"', async () => {
    const t = await abrir(manual({ id: 5, openFinance: true, descricaoOriginal: 'PIX 123' }));
    expect(t.raiz.querySelector('.aviso-inline')!.textContent).toContain('valor, data e conta vêm do banco');
    expect(t.raiz.querySelector('select[name=conta]')).toBeNull();
    expect(t.raiz.querySelector('app-campo-valor')).toBeNull();
    expect(t.botao('Excluir')).toBeUndefined();
  });

  it('manual já gravado tem "Excluir", que só avisa a tela', async () => {
    const t = await abrir(manual({ id: 5 }));
    let excluir = 0;
    t.componente.excluir.subscribe(() => excluir++);
    t.botao('Excluir')!.click();
    expect(excluir).toBe(1);
  });

  it('"Gasto de emergência" só aparece em despesa', async () => {
    const t = await abrir(manual());
    expect(t.raiz.querySelector('input[name=emergencia]')).not.toBeNull();
    t.componente.dados()!.tipo = TipoTransacao.Receita;
    await t.assentar();
    expect(t.raiz.querySelector('input[name=emergencia]')).toBeNull();
  });

  it('a sugestão de categoria escolhe a categoria com um clique; as opções levam o nome do pai', async () => {
    const t = await abrir(manual(), { sugestoes: [{ categoriaId: 21, nome: 'Cinema', cor: '#d4a5e8', motivo: 'igual a outras' }] });
    const opcoes = [...t.raiz.querySelectorAll('select[name=categoria] option')].map((o) => o.textContent!.trim());
    expect(opcoes).toEqual(['Automática (regras)', 'Mercado', 'Lazer › Cinema']);
    t.raiz.querySelector<HTMLButtonElement>('.chip-sugestao')!.click();
    await t.assentar();
    expect(t.componente.dados()!.categoriaId).toBe(21);
  });

  it('sem conta manual, diz onde criar uma', async () => {
    const t = await abrir(manual({ contaId: null }), { contasManuais: [] });
    expect(t.raiz.querySelector('.erro')!.textContent).toContain('Crie uma conta manual');
  });

  it('fechada (null), não mostra o formulário', async () => {
    const t = await abrir(null);
    expect(t.raiz.querySelector('form')).toBeNull();
  });
});

describe('Regressão: janela da transferência em Transações (#475)', () => {
  const nova = (): FormTransferencia => ({ contaOrigemId: 3, contaDestinoId: 4, valor: 100, data: '2026-10-10', descricao: '' });

  it('"Transferir" avisa com a cópia editada; a transferência recebida não muda', async () => {
    const original = nova();
    const t = await montar(TransacoesTransferenciaComponent, { transferencia: original, contas: CONTAS });
    const salvas: FormTransferencia[] = [];
    t.componente.salvar.subscribe((f) => salvas.push(f));
    await t.digitar('input[name=descricao]', 'Reserva');
    t.botao('Transferir')!.click();
    await t.assentar();
    expect(salvas).toEqual([{ ...original, descricao: 'Reserva' }]);
    expect(original.descricao).toBe('');
  });

  it('"Cancelar" só avisa que fechou', async () => {
    const t = await montar(TransacoesTransferenciaComponent, { transferencia: nova(), contas: CONTAS });
    let fechou = 0;
    t.componente.fechar.subscribe(() => fechou++);
    t.botao('Cancelar')!.click();
    expect(fechou).toBe(1);
  });
});
