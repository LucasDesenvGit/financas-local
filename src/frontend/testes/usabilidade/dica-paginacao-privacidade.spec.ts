import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { DicaDirective } from '../../src/app/compartilhado/dica.directive';
import { PaginacaoComponent } from '../../src/app/compartilhado/paginacao.component';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';

@Component({
  imports: [DicaDirective],
  template: `<button type="button" appDica="Ocultar valores" aria-label="Ocultar valores">👁</button>`
})
class HospedeiroDica {}

/** WCAG 1.4.13: a dica de botão só com ícone aparece no foco (não só no hover) e some com Esc. */
describe('Usabilidade: dica de botão só com ícone', () => {
  function montar() {
    const fixture = TestBed.createComponent(HospedeiroDica);
    fixture.detectChanges();
    return { fixture, botao: (fixture.nativeElement as HTMLElement).querySelector('button')! };
  }

  it('o texto da dica também é o nome acessível do botão', () => {
    const { botao } = montar();
    expect(botao.dataset['dica']).toBe('Ocultar valores');
    expect(botao.getAttribute('aria-label')).toBe(botao.dataset['dica']);
    expect(botao.classList).toContain('com-dica');
  });

  it('Esc dispensa a dica sem mover o foco; sair do botão a restaura', () => {
    const { botao } = montar();
    botao.focus();
    botao.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(botao.classList).toContain('dica-dispensada');
    expect(document.activeElement).toBe(botao);
    botao.dispatchEvent(new Event('blur'));
    expect(botao.classList).not.toContain('dica-dispensada');
  });
});

/** Paginação: Nielsen 1 (onde estou), WCAG 4.1.2 (página atual e setas com nome) e 2.1.1 (botões nativos). */
describe('Usabilidade: paginação', () => {
  function montar(pagina: number, total: number) {
    const fixture = TestBed.createComponent(PaginacaoComponent);
    fixture.componentRef.setInput('pagina', pagina);
    fixture.componentRef.setInput('tamanho', 50);
    fixture.componentRef.setInput('total', total);
    fixture.detectChanges();
    return { fixture, raiz: fixture.nativeElement as HTMLElement };
  }

  it('diz quais itens estão na tela e marca a página atual com aria-current', () => {
    const { raiz } = montar(3, 1000);
    expect(raiz.querySelector('.paginacao-info')!.textContent).toMatch(/101\s*–\s*150 de 1\.000/);
    expect(raiz.querySelector('[aria-current="page"]')!.textContent!.trim()).toBe('3');
  });

  it('setas têm nome acessível e ficam desabilitadas quando não há para onde ir', () => {
    const { raiz } = montar(1, 1000);
    const botao = (nome: string) => raiz.querySelector<HTMLButtonElement>(`[aria-label="${nome}"]`)!;
    expect(botao('Primeira página').disabled).toBe(true);
    expect(botao('Página anterior').disabled).toBe(true);
    expect(botao('Próxima página').disabled).toBe(false);
    expect(botao('Última página').disabled).toBe(false);
  });

  it('reticências são só visuais (aria-hidden)', () => {
    const { raiz } = montar(10, 1000);
    const reticencias = raiz.querySelectorAll('.pagina-reticencias');
    expect(reticencias.length).toBe(2);
    reticencias.forEach((r) => expect(r.getAttribute('aria-hidden')).toBe('true'));
  });

  it('lista vazia não mostra paginação', () => {
    const { raiz } = montar(1, 0);
    expect(raiz.querySelector('nav')).toBeNull();
  });
});

/** "Ocultar valores": privacidade na tela, respeitada também nas frases dos gráficos (padrão do CLAUDE.md). */
describe('Usabilidade: ocultar valores', () => {
  afterEach(() => document.documentElement.classList.remove('valores-ocultos'));

  it('alternar esconde os valores na página toda e a escolha fica lembrada', () => {
    const servico = TestBed.inject(PrivacidadeService);
    expect(servico.oculto()).toBe(false);
    servico.alternar();
    expect(document.documentElement.classList).toContain('valores-ocultos');
    expect(localStorage.getItem('financas.ocultarValores')).toBe('1');
    TestBed.resetTestingModule();
    expect(TestBed.inject(PrivacidadeService).oculto()).toBe(true);
  });
});
