import { Recorrencia } from '../../src/app/core/modelos';
import { abrirContasAPagar } from '../apoio/contas-agendadas';

/**
 * Usabilidade do campo "Até" de Contas a pagar (#69). Nielsen 2: "Até" e "quantos pagamentos faltam", as duas formas
 * como a pessoa pensa no fim da conta; Nielsen 5/9: a data errada é apontada no próprio formulário, antes de salvar,
 * e o campo diz o que acontece (encerra sozinha) e o que acontece em branco; WCAG 1.3.1/3.3.1: rótulo, ajuda ligada
 * por aria-describedby, erro com role="alert" e aria-invalid.
 */
describe('Usabilidade: data limite de conta a pagar', () => {
  async function abrirFormulario() {
    const t = abrirContasAPagar([]);
    t.tela.nova();
    t.tela.form!.proximoVencimento = '2026-11-10';
    await t.atualizar();
    const campo = (nome: string) => t.raiz.querySelector<HTMLInputElement>(`input[name=${nome}]`);
    const digitar = async (nome: string, valor: string) => {
      const el = campo(nome)!;
      el.value = valor;
      el.dispatchEvent(new Event('input'));
      await t.atualizar();
    };
    return { ...t, campo, digitar };
  }

  it('o campo é opcional, tem rótulo e explica o que acontece', async () => {
    const t = await abrirFormulario();
    const rotulo = t.campo('limite')!.closest('label')!;
    expect(rotulo.textContent).toContain('Até (opcional)');
    expect(t.campo('limite')!.required).toBe(false);
    const ajuda = document.getElementById(t.campo('limite')!.getAttribute('aria-describedby')!)!;
    expect(ajuda.textContent).toContain('encerrada sozinha');
    expect(ajuda.textContent).toContain('Em branco, ela se repete sem fim');
    expect(t.campo('restantes')!.getAttribute('aria-describedby')).toBe('ajuda-limite');
  });

  it('conta única não mostra o campo', async () => {
    const t = await abrirFormulario();
    t.tela.form!.recorrencia = Recorrencia.Unica;
    await t.atualizar();
    expect(t.campo('limite')).toBeNull();
    expect(t.campo('restantes')).toBeNull();
  });

  it('data antes do próximo vencimento: mensagem no formulário e Salvar desabilitado', async () => {
    const t = await abrirFormulario();
    await t.digitar('limite', '2026-10-01');
    const erro = t.raiz.querySelector('[role=alert]')!;
    expect(erro.textContent).toContain('A data limite não pode ser antes do próximo vencimento.');
    expect(t.campo('limite')!.getAttribute('aria-invalid')).toBe('true');
    const salvar = Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.trim() === 'Salvar')!;
    expect(salvar.disabled).toBe(true);
  });

  it('"quantos pagamentos faltam" preenche a data, e a data mostra quantos faltam', async () => {
    const t = await abrirFormulario();
    await t.digitar('restantes', '3');
    expect(t.tela.form!.dataLimite).toBe('2027-01-10');
    await t.digitar('limite', '2027-04-10');
    expect(t.campo('restantes')!.value).toBe('6');
  });
});
