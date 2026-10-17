import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { AuthService } from '../../src/app/core/auth.service';
import { ErroGlobalHandler } from '../../src/app/core/erro-global.handler';
import { ErroFrontEnd, NivelErro } from '../../src/app/core/modelos';
import { abrirErros, erro } from '../apoio/erros';
import { parametrosDaUrl } from '../apoio/transacoes';

/** Regressão de Configurações › Erros (#11): filtros na URL, resolver e o envio de erros do navegador. */
describe('Regressão: erros do sistema', () => {
  it('abre já filtrada pela URL, numa busca só', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&situacao=todos&origem=API&nivel=4&inicio=2026-10-01&fim=2026-10-16&pagina=2');
    expect(t.buscas).toHaveLength(1);
    expect(t.buscas[0]).toMatchObject({ situacao: 'todos', origem: 'API', nivel: NivelErro.Erro, inicio: '2026-10-01',
      fim: '2026-10-16', pagina: 2 });
  });

  it('parâmetros inválidos voltam ao padrão (em aberto, sem filtro)', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&situacao=xyz&nivel=9&inicio=2026-02-30&pagina=-3');
    expect(t.buscas[0]).toMatchObject({ situacao: 'abertos', origem: null, nivel: null, inicio: null, pagina: 1 });
  });

  it('mudar o filtro grava na URL com replaceUrl, mantém a aba e volta para a página 1', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&pagina=3');
    const navegar = jest.spyOn(t.router, 'navigate');
    t.tela.mudar({ nivel: NivelErro.Aviso });
    await t.assentar();
    expect(navegar).toHaveBeenLastCalledWith([], expect.objectContaining({ replaceUrl: true, queryParamsHandling: 'merge' }));
    expect(parametrosDaUrl(t.router)).toEqual({ aba: 'erros', nivel: '3' });
    expect(t.buscas.at(-1)).toMatchObject({ nivel: NivelErro.Aviso, pagina: 1 });
  });

  it('"Limpar filtros" tira tudo da URL menos a aba', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&situacao=resolvidos&origem=API');
    Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.trim() === 'Limpar filtros')!.click();
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({ aba: 'erros' });
  });

  it('"Resolvido" e "Reabrir" mandam a situação certa', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&situacao=todos', [erro({ id: 5 }), erro({ id: 6, resolvido: true })]);
    const botoes = Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('tbody button.botao-pequeno'));
    expect(botoes.map((b) => b.textContent!.trim())).toEqual(['Resolvido', 'Reabrir']);
    botoes.forEach((b) => b.click());
    expect(t.situacoes).toEqual([[5, true], [6, false]]);
  });

  it('resolver todos pede confirmação e manda o filtro atual', async () => {
    const t = await abrirErros('/configuracoes?aba=erros&origem=API');
    const confirmar = jest.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true);
    t.tela.resolverTodos();
    expect(t.resolvidosEmLote).toHaveLength(0);
    t.tela.resolverTodos();
    expect(confirmar).toHaveBeenCalledWith('Marcar os 1 erro(s) deste filtro como resolvidos?');
    expect(t.resolvidosEmLote[0]).toMatchObject({ origem: 'API', situacao: 'abertos' });
    confirmar.mockRestore();
  });
});

describe('Regressão: erros do navegador vão para a API', () => {
  function criar(autenticado = true) {
    const enviados: ErroFrontEnd[] = [];
    TestBed.configureTestingModule({
      providers: [
        ErroGlobalHandler,
        { provide: AuthService, useValue: { estado: signal({ configurado: true, autenticado, login: 'lucas' }) } },
        { provide: ApiService, useValue: { registrarErroFrontEnd: (dto: ErroFrontEnd) => { enviados.push(dto); return of(undefined); } } }
      ]
    });
    jest.spyOn(console, 'error').mockImplementation(() => undefined);
    return { handler: TestBed.inject(ErroGlobalHandler), enviados };
  }

  afterEach(() => jest.restoreAllMocks());

  it('envia mensagem, pilha e tela; desembrulha Promise rejeitada', () => {
    const { handler, enviados } = criar();
    handler.handleError(new TypeError('x is undefined'));
    handler.handleError({ rejection: new RangeError('fora') });
    expect(enviados.map((e) => e.mensagem)).toEqual(['TypeError: x is undefined', 'RangeError: fora']);
    expect(enviados[0].detalhe).toContain('TypeError: x is undefined');
    expect(enviados[0].rota).toBe(location.pathname);
  });

  it('não envia erro de chamada à API, nem sem login', () => {
    const { handler, enviados } = criar();
    handler.handleError(new HttpErrorResponse({ status: 500 }));
    expect(enviados).toEqual([]);
    TestBed.resetTestingModule();
    const semLogin = criar(false);
    semLogin.handler.handleError(new Error('qualquer'));
    expect(semLogin.enviados).toEqual([]);
  });

  it('não repete o mesmo erro e para em 10 por carga da página', () => {
    const { handler, enviados } = criar();
    handler.handleError(new Error('igual'));
    handler.handleError(new Error('igual'));
    for (let i = 0; i < 20; i++) handler.handleError(new Error('erro ' + i));
    expect(enviados).toHaveLength(ErroGlobalHandler.MAXIMO_POR_CARGA);
    expect(enviados.filter((e) => e.mensagem === 'Error: igual')).toHaveLength(1);
  });
});
