import { TestBed } from '@angular/core/testing';
import { ConfirmacaoService } from '../../../src/app/compartilhado/confirmacao.service';
import { abrirMetas, meta } from '../apoio/metas';

/**
 * #631: cada meta tem "Excluir" ao lado de "Editar", para apagar sem abrir a edição. Antes de apagar, a confirmação do
 * site diz o nome da meta (Nielsen 5); Cancelar não apaga nada. O "Excluir" do formulário de edição pede a mesma.
 */
const VIAGEM = meta({ id: 7, nome: 'Viagem', contaId: null, reservaNome: null, saldoReserva: null });

describe('Regressão: Excluir no card das metas (#631)', () => {
  beforeEach(() => localStorage.clear());

  const excluirDoCard = (raiz: HTMLElement) => raiz.querySelector<HTMLButtonElement>('button[aria-label="Excluir a meta Viagem"]')!;

  it('o card tem "Excluir" ao lado de "Editar"', () => {
    const { raiz } = abrirMetas([VIAGEM]);
    const editar = raiz.querySelector<HTMLButtonElement>('button[aria-label="Editar a meta Viagem"]')!;
    const excluir = excluirDoCard(raiz);
    expect(excluir.textContent!.trim()).toBe('Excluir');
    expect(excluir.classList).toContain('perigo');
    expect(editar.nextElementSibling).toBe(excluir);
  });

  it('pergunta com o nome da meta; confirmando, apaga a meta do card', async () => {
    const t = abrirMetas([VIAGEM]);
    const confirmar = jest.spyOn(TestBed.inject(ConfirmacaoService), 'confirmar').mockResolvedValue(true);
    excluirDoCard(t.raiz).click();
    await t.atualizar();
    expect(confirmar).toHaveBeenCalledWith(expect.objectContaining({ mensagem: 'Excluir a meta “Viagem”?', confirmar: 'Excluir', perigo: true }));
    expect(t.excluidas).toEqual([7]);
  });

  it('Cancelar não apaga nada', async () => {
    const t = abrirMetas([VIAGEM]);
    jest.spyOn(TestBed.inject(ConfirmacaoService), 'confirmar').mockResolvedValue(false);
    excluirDoCard(t.raiz).click();
    await t.atualizar();
    expect(t.excluidas).toEqual([]);
  });

  it('o "Excluir" do formulário de edição pede a mesma confirmação', async () => {
    const t = abrirMetas([VIAGEM]);
    const confirmar = jest.spyOn(TestBed.inject(ConfirmacaoService), 'confirmar').mockResolvedValue(true);
    t.tela.editar(VIAGEM);
    await t.atualizar();
    t.tela.excluir();
    await t.atualizar();
    expect(confirmar).toHaveBeenCalledWith(expect.objectContaining({ mensagem: 'Excluir a meta “Viagem”?' }));
    expect(t.excluidas).toEqual([7]);
  });
});
