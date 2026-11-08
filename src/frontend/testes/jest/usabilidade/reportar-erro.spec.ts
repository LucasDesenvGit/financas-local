import { TestBed } from '@angular/core/testing';
import { AuthService } from '../../../src/app/core/auth.service';
import { AvisosService } from '../../../src/app/core/avisos.service';
import { NivelErro } from '../../../src/app/core/modelos';
import { textoParaOClaude } from '../../../src/app/paginas/ajustes/reportar-erro';
import { abrirErros, erro } from '../apoio/erros';

/**
 * #332: "Reportar para o Claude", em Configurações › Erros. O botão copia o erro num texto que começa com
 * /reportar-erro, para colar no Claude Code (o site não chama o Claude: seria uma saída de rede nova).
 * - Nielsen 1 (Visibility of System Status): "Copiado. Cole no Claude Code." depois de copiar.
 * - Nielsen 3 (User Control and Freedom) e 9: sem a área de transferência, o texto aparece já selecionado, para copiar
 *   à mão, e fecha.
 * - WCAG 3.3.2 (Labels or Instructions): a caixa tem rótulo, que diz o que fazer.
 */
describe('Usabilidade: reportar um erro para o Claude (#332)', () => {
  const pilha = 'System.FormatException: x\n   at Metas.Listar()';

  describe('o texto', () => {
    it('começa pelo /reportar-erro e traz a mensagem, a origem, o nível, a tela, as ocorrências, a pilha e a versão', () => {
      const texto = textoParaOClaude(erro(), { id: 1, categoria: 'Erro', detalhe: pilha }, '3.4.0');
      expect(texto.split('\n')).toEqual([
        '/reportar-erro',
        'Erro registrado em Configurações › Erros do Finanças Local (já sem dados sensíveis).',
        '',
        "- Mensagem: Erro não tratado em /api/metas: String 'x' was not recognized as a valid DateOnly.",
        '- Origem: API',
        '- Nível: Erro',
        '- Tela: GET /api/metas',
        '- Ocorrências: 3, da primeira em 16/10/2026 09:00 à última em 16/10/2026 12:30',
        '- Situação: em aberto',
        '- Registrado por: Erro',
        '- Versão do sistema: 3.4.0',
        '',
        'Pilha:',
        '```',
        'System.FormatException: x',
        '   at Metas.Listar()',
        '```'
      ]);
    });

    it('não leva o login de quem estava na sessão', () => {
      expect(textoParaOClaude(erro({ usuario: 'ana.teste' }), null, '3.4.0')).not.toContain('ana.teste');
    });

    it('uma ocorrência, sem tela, sem pilha, resolvido e sem a versão', () => {
      const texto = textoParaOClaude(erro({ ocorrencias: 1, rota: null, resolvido: true, nivel: NivelErro.Aviso }),
        { id: 1, categoria: 'Financas.Go.Câmbio', detalhe: null }, undefined);
      expect(texto).toContain('- Ocorrências: 1, em 16/10/2026 12:30');
      expect(texto).toContain('- Tela: (sem tela)');
      expect(texto).toContain('- Nível: Aviso');
      expect(texto).toContain('- Situação: marcado como resolvido');
      expect(texto).toContain('- Versão do sistema: (não informada)');
      expect(texto).not.toContain('Pilha');
    });
  });

  describe('o botão', () => {
    const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    afterEach(() => {
      if (original) Object.defineProperty(navigator, 'clipboard', original);
      else delete (navigator as { clipboard?: Clipboard }).clipboard;
    });
    const areaDeTransferencia = (writeText: (t: string) => Promise<void>) =>
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });

    async function abrir() {
      const t = await abrirErros();
      const auth = TestBed.inject(AuthService);
      auth.estado.set({ configurado: true, autenticado: true, login: 'lucas', versao: '3.4.0' });
      const sucesso = jest.spyOn(TestBed.inject(AvisosService), 'sucesso');
      const botao = t.raiz.querySelector<HTMLButtonElement>('button[aria-label^="Reportar para o Claude o erro de"]')!;
      return { ...t, sucesso, botao };
    }

    it('copia o texto com a pilha do detalhe e avisa', async () => {
      const copiados: string[] = [];
      areaDeTransferencia(async (texto) => { copiados.push(texto); });
      const t = await abrir();
      expect(t.botao.textContent!.trim()).toBe('Reportar para o Claude');
      t.botao.click();
      await t.assentar();
      expect(t.detalhes).toEqual([1]);
      expect(copiados).toHaveLength(1);
      expect(copiados[0]).toMatch(/^\/reportar-erro\n/);
      expect(copiados[0]).toContain('at Metas.Listar()');
      expect(copiados[0]).toContain('- Versão do sistema: 3.4.0');
      expect(t.sucesso).toHaveBeenCalledWith('Copiado. Cole no Claude Code.');
      expect(t.raiz.querySelector('textarea')).toBeNull();
    });

    for (const [caso, preparar] of [
      ['o navegador recusa copiar', () => areaDeTransferencia(() => Promise.reject(new Error('sem permissão')))],
      ['não há área de transferência', () => Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined })]
    ] as const) {
      it(`${caso}: o texto aparece na linha, com rótulo, já com o foco e selecionado, e fecha`, async () => {
        preparar();
        const t = await abrir();
        t.botao.click();
        await t.assentar();
        await t.assentar();
        const caixa = t.raiz.querySelector<HTMLTextAreaElement>('textarea')!;
        expect(caixa.value).toMatch(/^\/reportar-erro\n/);
        expect(t.raiz.querySelector(`label[for="${caixa.id}"]`)!.textContent).toContain('copie o texto abaixo');
        expect(document.activeElement).toBe(caixa);
        expect([caixa.selectionStart, caixa.selectionEnd]).toEqual([0, caixa.value.length]);
        expect(t.sucesso).not.toHaveBeenCalled();
        Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('.linha-reporte button')).find((b) => b.textContent!.trim() === 'Fechar')!.click();
        await t.assentar();
        expect(t.raiz.querySelector('textarea')).toBeNull();
      });
    }
  });
});
