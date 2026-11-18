import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Conta, OrigemConta, Reserva, TipoTransacao } from '../../../src/app/core/modelos';
import {
  FormTransacao, FormTransferencia, contasComCofrinho, formNovo, trocarContaDoLancamento, transacaoDoForm
} from '../../../src/app/paginas/movimentacao/transacoes';
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

  it('sem conta manual nem cofrinho, a lista fica desabilitada e diz onde criar uma (#758)', async () => {
    const t = await abrir(manual({ contaId: null }), { contasManuais: [], reservas: [] });
    expect(t.raiz.querySelector('.erro')!.textContent).toContain('Crie uma conta manual');
    expect(t.raiz.querySelector<HTMLSelectElement>('select[name=conta]')!.disabled).toBe(true);
    expect(t.botao('Salvar')!.disabled).toBe(true);
  });

  it('com as reservas ainda carregando, não avisa que falta conta (o aviso não pisca)', async () => {
    const t = await abrir(manual({ contaId: null }), { contasManuais: [] });
    expect(t.raiz.querySelector('.erro')).toBeNull();
  });

  it('fechada (null), não mostra o formulário', async () => {
    const t = await abrir(null);
    expect(t.raiz.querySelector('form')).toBeNull();
  });
});

/** #758 (ADR 0063): o gasto pago direto de um cofrinho, lançado pelo "Novo lançamento". */
const MP = { id: 7, nome: 'Mercado Pago', origem: OrigemConta.OpenFinance } as Conta;
const SEM_COFRINHO = { id: 8, nome: 'Nubank', origem: OrigemConta.OpenFinance } as Conta;
const reserva = (contaId: number, nome: string, campos: Partial<Reserva> = {}) =>
  ({ contaId, nome, saldo: 100, moeda: 'BRL', restanteDoBanco: false, ...campos }) as Reserva;
const RESERVAS = [reserva(7, 'Viagem'), reserva(7, 'Casa'), reserva(7, 'Méli Dólar', { moeda: 'USD' }),
  reserva(8, 'Restante', { restanteDoBanco: true })];

describe('Regressão: lançamento pelo cofrinho (#758)', () => {
  const abrir = (lancamento: FormTransacao | null, extras: Record<string, unknown> = {}) =>
    montar(TransacoesLancamentoComponent, { lancamento, contasManuais: CONTAS, contasOpenFinance: [MP, SEM_COFRINHO],
      reservas: RESERVAS, categorias: CATEGORIAS, ...extras });
  const escolher = async (t: Awaited<ReturnType<typeof abrir>>, nome: string, texto: string) => {
    const select = t.raiz.querySelector<HTMLSelectElement>(`select[name=${nome}]`)!;
    select.selectedIndex = [...select.options].findIndex((o) => o.textContent!.trim() === texto);
    select.dispatchEvent(new Event('change'));
    await t.assentar();
  };

  it('contas: só as do Open Finance com cofrinho em reais; a do lançamento aberto fica', () => {
    expect(contasComCofrinho([...CONTAS, MP, SEM_COFRINHO], RESERVAS, null).map((c) => c.id)).toEqual([7]);
    expect(contasComCofrinho([MP, SEM_COFRINHO], RESERVAS, 8).map((c) => c.id)).toEqual([7, 8]);
    expect(contasComCofrinho([MP], [], null)).toEqual([]);
  });

  it('trocar a conta: na do cofrinho, só despesa e o cofrinho de outra conta sai; na manual, sem cofrinho', () => {
    const receita = manual({ tipo: TipoTransacao.Receita, reservaNome: 'Outro' });
    expect(trocarContaDoLancamento(receita, 7, true, RESERVAS)).toMatchObject({ contaId: 7, tipo: TipoTransacao.Despesa, reservaNome: null });
    expect(trocarContaDoLancamento(manual({ contaId: 7, reservaNome: 'Viagem' }), 7, true, RESERVAS).reservaNome).toBe('Viagem');
    expect(trocarContaDoLancamento(manual({ contaId: 7, reservaNome: 'Viagem' }), 3, false, RESERVAS))
      .toMatchObject({ contaId: 3, reservaNome: null });
  });

  it('o cofrinho vai aparado para a API, e o vazio vai nulo', () => {
    expect(transacaoDoForm(manual({ contaId: 7, reservaNome: ' Viagem ' })).reservaNome).toBe('Viagem');
    expect(transacaoDoForm(manual({ reservaNome: '  ' })).reservaNome).toBeNull();
  });

  it('a lista de contas separa as manuais das do cofrinho; escolhida a do cofrinho, aparece o campo e a receita fica de fora', async () => {
    const t = await abrir(manual({ tipo: TipoTransacao.Receita }));
    const grupos = [...t.raiz.querySelectorAll('select[name=conta] optgroup')].map((g) => g.getAttribute('label'));
    expect(grupos).toEqual(['Contas manuais', 'Gasto pago por um cofrinho']);
    expect(t.raiz.querySelector('select[name=cofrinho]')).toBeNull();

    await escolher(t, 'conta', 'Mercado Pago');
    const opcoes = [...t.raiz.querySelectorAll('select[name=cofrinho] option')].map((o) => o.textContent!.trim());
    expect(opcoes).toEqual(['Casa', 'Viagem']);
    expect(t.componente.dados()).toMatchObject({ contaId: 7, tipo: TipoTransacao.Despesa, reservaNome: null });
    expect(t.raiz.querySelector<HTMLOptionElement>('select[name=tipo] option:nth-child(2)')!.disabled).toBe(true);
    expect(t.raiz.textContent).toContain('O saldo da conta não');
    expect(t.botao('Salvar')!.disabled).toBe(true);

    const salvos: FormTransacao[] = [];
    t.componente.salvar.subscribe((f) => salvos.push(f));
    await escolher(t, 'cofrinho', 'Viagem');
    t.botao('Salvar')!.click();
    await t.assentar();
    expect(salvos).toEqual([expect.objectContaining({ contaId: 7, reservaNome: 'Viagem', tipo: TipoTransacao.Despesa })]);
  });

  it('editar um lançamento do cofrinho: a conta e o cofrinho continuam escolhidos (antes, a conta sumia da lista)', async () => {
    const t = await abrir(manual({ id: 40, contaId: 7, reservaNome: 'Viagem' }), { contasManuais: [] });
    expect(t.raiz.querySelector('.erro')).toBeNull();
    expect(t.raiz.querySelector<HTMLSelectElement>('select[name=conta]')!.selectedOptions[0].textContent!.trim()).toBe('Mercado Pago');
    expect(t.raiz.querySelector<HTMLSelectElement>('select[name=cofrinho]')!.selectedOptions[0].textContent!.trim()).toBe('Viagem');
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
