import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiService } from '../../../src/app/core/api.service';
import { Modulo } from '../../../src/app/core/modelos';
import { ModulosService, listarDestinos, trocarModulo } from '../../../src/app/core/modulos.service';
import { descreverEvento } from '../../../src/app/paginas/ajustes/auditoria';

/** #191 (ADR 0064): a lista de módulos no front-end, lida uma vez e trocada ao ligar ou desligar. */
describe('Módulos (#191)', () => {
  const trabalho: Modulo = { id: 'trabalho', nome: 'Trabalho', descricao: 'Salário.', acessaInternet: false, destinos: [], ligado: false };
  const noticias: Modulo = { id: 'noticias', nome: 'Notícias', descricao: 'Do dia.', acessaInternet: true, destinos: ['g1.globo.com'], ligado: false };

  it('trocarModulo troca pelo id, acrescenta o novo e começa a lista que ainda não veio', () => {
    expect(trocarModulo([trabalho, noticias], { ...trabalho, ligado: true })).toEqual([{ ...trabalho, ligado: true }, noticias]);
    expect(trocarModulo([trabalho], noticias)).toEqual([trabalho, noticias]);
    expect(trocarModulo(null, trabalho)).toEqual([trabalho]);
  });

  it('listarDestinos junta os domínios numa frase', () => {
    expect(listarDestinos([])).toBe('');
    expect(listarDestinos(['a.com'])).toBe('a.com');
    expect(listarDestinos(['a.com', 'b.com'])).toBe('a.com e b.com');
    expect(listarDestinos(['a.com', 'b.com', 'c.com'])).toBe('a.com, b.com e c.com');
  });

  it('os eventos de ligar e desligar aparecem em português nos acessos recentes', () => {
    expect(descreverEvento('ModuloLigado (trabalho)')).toBe('Módulo ligado (trabalho)');
    expect(descreverEvento('ModuloDesligado (noticias)')).toBe('Módulo desligado (noticias)');
  });

  function servico(api: Partial<ApiService>) {
    TestBed.configureTestingModule({ providers: [{ provide: ApiService, useValue: api }] });
    return TestBed.inject(ModulosService);
  }

  it('lê a lista uma vez só e responde se o módulo está ligado', () => {
    const modulos = jest.fn(() => of([trabalho, { ...noticias, ligado: true }]));
    const s = servico({ modulos });
    expect(s.lista()).toBeNull();
    expect(s.ligado('noticias')).toBe(false); // ainda não chegou: desligado
    s.carregar();
    s.carregar();
    expect(modulos).toHaveBeenCalledTimes(1);
    expect(s.ligado('noticias')).toBe(true);
    expect(s.ligado('trabalho')).toBe(false);
    expect(s.ligado('nenhum')).toBe(false);
  });

  it('a leitura que falha deixa a lista vazia: todo módulo conta como desligado', () => {
    const s = servico({ modulos: () => throwError(() => new Error('rede')) });
    s.carregar();
    expect(s.lista()).toEqual([]);
  });

  it('ligar troca o módulo na lista com o que a API devolveu', () => {
    const definirModulo = jest.fn(() => of({ ...trabalho, ligado: true }));
    const s = servico({ modulos: () => of([trabalho, noticias]), definirModulo });
    s.carregar();
    s.definir('trabalho', true).subscribe();
    expect(definirModulo).toHaveBeenCalledWith('trabalho', true);
    expect(s.ligado('trabalho')).toBe(true);
    expect(s.ligado('noticias')).toBe(false);
  });
});
