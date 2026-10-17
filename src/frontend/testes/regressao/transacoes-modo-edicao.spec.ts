import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { TipoCorrespondencia } from '../../src/app/core/modelos';
import { TestBed } from '@angular/core/testing';
import { abrirEdicao, transacao } from '../apoio/transacoes-edicao';

/** Regressão do modo edição de categorias (#87): o que vai para a API, pendências, desfazer e regra. */
describe('Regressão: modo edição de categorias', () => {
  afterEach(() => jest.restoreAllMocks());

  it('abrir com ?editar=1 já entra no modo edição', async () => {
    const t = await abrirEdicao('/transacoes?editar=1');
    expect(t.tela.modoEdicao()).toBe(true);
    expect(t.elemento.querySelectorAll('select.select-categoria')).toHaveLength(3);
  });

  it('salvar manda só as linhas trocadas, cada uma com a sua escolha', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    await t.escolher(3, 'auto');
    t.botao('Salvar alterações').click();
    // Sem whenStable: o aviso com "Desfazer" deixa um timer de 10 s pendente.
    t.harness.detectChanges();
    expect(t.lotes).toEqual([{ itens: [
      { id: 1, categoriaId: 20, automatica: false },
      { id: 3, categoriaId: null, automatica: true }
    ] }]);
    expect(t.tela.pendentes().size).toBe(0);
  });

  it('voltar a linha para a categoria que já tinha tira a pendência', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    await t.escolher(1, '12');
    expect(t.tela.pendentes().size).toBe(0);
  });

  it('trocar de página com pendência pergunta; "cancelar" mantém a troca e a página', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    const confirmar = jest.spyOn(window, 'confirm').mockReturnValue(false);
    const buscasAntes = t.buscas.length;
    t.tela.buscar(2);
    expect(confirmar).toHaveBeenCalledWith('Descartar 1 alteração(ões) de categoria não salva(s)?');
    expect(t.buscas.length).toBe(buscasAntes);
    expect(t.tela.pendentes().get(1)).toBe('20');
  });

  it('"todas do filtro" manda o filtro da lista na tela (não o que está sendo digitado)', async () => {
    const t = await abrirEdicao('/transacoes?editar=1&categoria=sem&inicio=2026-09-01&fim=2026-09-30', undefined, 240);
    jest.spyOn(window, 'confirm').mockReturnValue(true);
    t.tela.filtro.busca = 'digitando';
    t.tela.categoriaTodas = 'auto';
    t.tela.aplicarATodasDoFiltro();
    expect(t.lotes).toHaveLength(1);
    expect(t.lotes[0]).toMatchObject({ categoriaId: null, automatica: true,
      filtro: { semCategoria: true, inicio: '2026-09-01', fim: '2026-09-30', pagina: 1 } });
    expect(t.lotes[0].filtro!.busca ?? '').toBe('');
  });

  it('"Desfazer" do aviso manda a chave devolvida pela API', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    t.botao('Salvar alterações').click();
    // Sem whenStable: o aviso com "Desfazer" deixa um timer de 10 s pendente.
    t.harness.detectChanges();
    const aviso = TestBed.inject(AvisosService).avisos().find((a) => a.acao)!;
    expect(aviso.texto).toBe('1 transação(ões) atualizada(s).');
    aviso.acao!.executar();
    expect(t.desfeitos).toEqual(['chave-1']);
  });

  it('duas transações iguais para a mesma categoria sugerem regra; "Criar regra" usa a descrição original', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    await t.escolher(2, '20');
    t.botao('Salvar alterações').click();
    // Sem whenStable: o aviso com "Desfazer" deixa um timer de 10 s pendente.
    t.harness.detectChanges();
    const sugestao = t.elemento.querySelector('.sugestao-regra')!;
    expect(sugestao.textContent!.replace(/\s+/g, ' ')).toContain('2 transações “PADARIA PAO QUENTE” foram para Lazer.');
    t.botao('Criar regra').click();
    expect(t.regras).toEqual([{ texto: 'PADARIA PAO QUENTE', tipoCorrespondencia: TipoCorrespondencia.Contem, categoriaId: 20,
      prioridade: 100, ativa: true }]);
  });

  it('uma transação só não sugere regra', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    t.botao('Salvar alterações').click();
    // Sem whenStable: o aviso com "Desfazer" deixa um timer de 10 s pendente.
    t.harness.detectChanges();
    expect(t.elemento.querySelector('.sugestao-regra')).toBeNull();
  });

  it('no modo edição, a barra de seleção marca a troca em vez de gravar na hora', async () => {
    const t = await abrirEdicao();
    t.tela.alternar(1);
    t.tela.alternar(2);
    t.tela.categoriaLote = 20;
    t.tela.categorizarLote();
    await t.assentar();
    expect(t.categorizadosAntigo).toEqual([]);
    expect([...t.tela.pendentes()]).toEqual([[1, '20'], [2, '20']]);
  });

  it('sair do modo edição com pendência pergunta; confirmando, descarta e tira ?editar da URL', async () => {
    const t = await abrirEdicao('/transacoes?editar=1', [transacao({ id: 1 })]);
    await t.escolher(1, '20');
    jest.spyOn(window, 'confirm').mockReturnValue(true);
    t.tela.alternarModoEdicao();
    await t.assentar();
    expect(t.tela.modoEdicao()).toBe(false);
    expect(t.tela.pendentes().size).toBe(0);
    expect(t.router.url).toBe('/transacoes');
  });
});
