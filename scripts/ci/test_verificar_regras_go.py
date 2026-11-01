#!/usr/bin/env python3
"""
Teste do scripts/ci/verificar-regras-go.py (#457): a regra pega o caso ruim, deixa passar o que é permitido e a exceção,
e a mensagem diz o que usar no lugar. Só a biblioteca padrão (unittest), sem Go instalado.

Uso:  python3 -B scripts/ci/test_verificar_regras_go.py
"""
import importlib.util
import os
import tempfile
import unittest

AQUI = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location('regras', os.path.join(AQUI, 'verificar-regras-go.py'))
regras = importlib.util.module_from_spec(spec)
spec.loader.exec_module(regras)


class Projeto:
    """Uma pasta de Go de mentira, com os arquivos e a lista de exceções do caso."""

    def __init__(self, caso, arquivos, excecoes=''):
        self.pasta = tempfile.TemporaryDirectory()
        caso.addCleanup(self.pasta.cleanup)
        for nome, conteudo in arquivos.items():
            caminho = os.path.join(self.pasta.name, nome)
            os.makedirs(os.path.dirname(caminho), exist_ok=True)
            with open(caminho, 'w', encoding='utf-8') as f:
                f.write(conteudo)
        self.excecoes = os.path.join(self.pasta.name, 'excecoes.txt')
        with open(self.excecoes, 'w', encoding='utf-8') as f:
            f.write(excecoes)

    def conferir(self):
        problemas, _ = regras.conferir(self.pasta.name, self.excecoes)
        return problemas


def dos_arquivos(problemas):
    return [(arquivo, numero) for arquivo, numero, _ in problemas]


class Float64(unittest.TestCase):
    def test_pega_o_float64_no_codigo(self):
        p = Projeto(self, {'internal/contas/contas.go': 'package contas\n\nvar saldo float64\n'})
        problemas = p.conferir()
        self.assertEqual(dos_arquivos(problemas), [('internal/contas/contas.go', 3)])
        self.assertIn('tipos.Decimal', problemas[0][2])

    def test_pega_o_decimal_virando_float(self):
        # o caminho por onde o dinheiro escorre sem escrever "float64" no código
        fontes = {
            'internal/a/a.go': 'package a\n\nfunc f(d D) { v, _ := d.Float64(); _ = v }\n',
            'internal/b/b.go': 'package b\n\nfunc f(t string) { strconv.ParseFloat(t, 64) }\n',
            'internal/c/c.go': 'package c\n\nfunc f(d D) { _ = d.ParaFloat64() }\n',
            'internal/d/d.go': 'package d\n\nvar v float32\n',
        }
        achados = dos_arquivos(Projeto(self, fontes).conferir())
        self.assertEqual(sorted(achados), [('internal/a/a.go', 3), ('internal/b/b.go', 3), ('internal/c/c.go', 3),
                                           ('internal/d/d.go', 3)])

    def test_comentario_e_texto_nao_contam(self):
        fonte = ('package x\n\n// o saldo nunca é float64\n/* nem float64\n   em bloco */\n'
                 'var a = "float64"\nvar b = `float64\ncru`\nvar c = \'"\'\nvar d = "aspas \\" float64"\n')
        self.assertEqual(Projeto(self, {'internal/x/x.go': fonte}).conferir(), [])

    def test_o_synctest_nao_perdoa_o_float64(self):
        # a bolha dá relógio de mentira, e não dinheiro certo
        fonte = 'package x\n\nfunc TestA(t *testing.T) {\n\tsynctest.Test(t, func(t *testing.T) {\n\t\tvar v float64\n\t})\n}\n'
        self.assertEqual(dos_arquivos(Projeto(self, {'internal/x/x_test.go': fonte}).conferir()),
                         [('internal/x/x_test.go', 5)])

    def test_vale_tambem_no_teste(self):
        p = Projeto(self, {'internal/x/x_test.go': 'package x\n\nvar v float64\n'})
        self.assertEqual(dos_arquivos(p.conferir()), [('internal/x/x_test.go', 3)])

    def test_a_linha_certa_depois_de_texto_cru_de_varias_linhas(self):
        fonte = 'package x\n\nvar sql = `\nSELECT 1\n`\n\nvar v float64\n'
        self.assertEqual(dos_arquivos(Projeto(self, {'internal/x/x.go': fonte}).conferir()), [('internal/x/x.go', 7)])


class TimeNow(unittest.TestCase):
    def test_pega_o_time_now_fora_do_relogio(self):
        p = Projeto(self, {'internal/contas/contas.go': 'package contas\n\nvar hoje = time.Now()\n'})
        problemas = p.conferir()
        self.assertEqual(dos_arquivos(problemas), [('internal/contas/contas.go', 3)])
        self.assertIn('relogio.Relogio', problemas[0][2])
        self.assertIn('relogio.Fixo', problemas[0][2])

    def test_pega_o_time_now_sem_parenteses(self):
        # agora: time.Now também lê o relógio de verdade
        p = Projeto(self, {'internal/x/x.go': 'package x\n\nvar l = limite{agora: time.Now}\n'})
        self.assertEqual(dos_arquivos(p.conferir()), [('internal/x/x.go', 3)])

    def test_o_pacote_relogio_pode(self):
        fontes = {'internal/relogio/relogio.go': 'package relogio\n\nfunc agora() time.Time { return time.Now() }\n',
                  'internal/relogio/relogio_test.go': 'package relogio\n\nvar x = time.Now()\n'}
        self.assertEqual(Projeto(self, fontes).conferir(), [])

    def test_so_o_pacote_relogio_e_so_o_dele(self):
        p = Projeto(self, {'internal/relogiodemais/x.go': 'package x\n\nvar x = time.Now()\n'})
        self.assertEqual(dos_arquivos(p.conferir()), [('internal/relogiodemais/x.go', 3)])

    def test_dentro_do_synctest_passa_e_fora_nao(self):
        fonte = ('package x\n\nfunc TestA(t *testing.T) {\n\tsynctest.Test(t, func(t *testing.T) {\n'
                 '\t\tinicio := time.Now()\n\t\t_ = fmt.Sprint("(")\n\t\t_ = inicio\n\t})\n\tfora := time.Now()\n}\n')
        problemas = Projeto(self, {'internal/x/x_test.go': fonte}).conferir()
        self.assertEqual(dos_arquivos(problemas), [('internal/x/x_test.go', 9)])


class TimeSleep(unittest.TestCase):
    def test_pega_o_sleep_de_espera_no_teste(self):
        fonte = 'package x\n\nfunc TestA(t *testing.T) {\n\ttime.Sleep(20 * time.Millisecond)\n}\n'
        problemas = Projeto(self, {'internal/x/x_test.go': fonte}).conferir()
        self.assertEqual(dos_arquivos(problemas), [('internal/x/x_test.go', 4)])
        self.assertIn('synctest', problemas[0][2])
        self.assertIn('sinal', problemas[0][2])

    def test_fora_de_teste_nao_e_desta_regra(self):
        p = Projeto(self, {'internal/x/x.go': 'package x\n\nfunc f() { time.Sleep(time.Second) }\n'})
        self.assertEqual(p.conferir(), [])

    def test_dentro_do_synctest_passa(self):
        fonte = ('package x\n\nfunc TestA(t *testing.T) {\n\tsynctest.Test(t, func(t *testing.T) {\n'
                 '\t\ttime.Sleep(time.Minute)\n\t})\n}\n')
        self.assertEqual(Projeto(self, {'internal/x/x_test.go': fonte}).conferir(), [])

    def test_o_sleep_depois_do_synctest_nao_e_perdoado(self):
        fonte = ('package x\n\nfunc TestA(t *testing.T) {\n\tsynctest.Test(t, func(t *testing.T) {})\n'
                 '\ttime.Sleep(time.Minute)\n}\n')
        self.assertEqual(dos_arquivos(Projeto(self, {'internal/x/x_test.go': fonte}).conferir()),
                         [('internal/x/x_test.go', 5)])


class Excecoes(unittest.TestCase):
    CODIGO = {'internal/selic/serie.go': 'package selic\n\nvar acumulado []float64\n'}

    def test_a_excecao_deixa_passar(self):
        p = Projeto(self, self.CODIGO,
                    'float64 | internal/selic/serie.go | var acumulado []float64 | fator da Selic, o C# usa double\n')
        self.assertEqual(p.conferir(), [])

    def test_a_excecao_vale_so_para_a_regra_e_o_arquivo_dela(self):
        excecao = 'float64 | internal/selic/serie.go | var acumulado []float64 | fator da Selic\n'
        outro = dict(self.CODIGO, **{'internal/outro/outro.go': 'package outro\n\nvar acumulado []float64\n'})
        self.assertEqual(dos_arquivos(Projeto(self, outro, excecao).conferir()), [('internal/outro/outro.go', 3)])

    def test_linha_mudada_perde_a_excecao_e_a_excecao_velha_aparece(self):
        p = Projeto(self, {'internal/selic/serie.go': 'package selic\n\nvar acumulado []float64 // mudou\nvar saldo float64\n'},
                    'float64 | internal/selic/serie.go | var acumulado []float64 | fator da Selic\n')
        # o comentário no fim da linha não conta: a exceção ainda vale; a linha nova do saldo não tem exceção
        self.assertEqual(dos_arquivos(p.conferir()), [('internal/selic/serie.go', 4)])
        p = Projeto(self, {'internal/selic/serie.go': 'package selic\n\nvar acumulado []float32\n'},
                    'float64 | internal/selic/serie.go | var acumulado []float64 | fator da Selic\n')
        problemas = p.conferir()
        self.assertEqual(dos_arquivos(problemas), [('internal/selic/serie.go', 3), ('internal/selic/serie.go', 0)])
        self.assertIn('não casa com nenhuma linha', problemas[1][2])

    def test_excecao_sem_motivo_e_erro(self):
        p = Projeto(self, self.CODIGO, 'float64 | internal/selic/serie.go | var acumulado []float64 |\n')
        self.assertTrue(any('esperado "regra | arquivo | linha do código | motivo"' in m for _, _, m in p.conferir()))
        p = Projeto(self, self.CODIGO, 'float64 | internal/selic/serie.go | var acumulado []float64\n')
        self.assertTrue(any('esperado' in m for _, _, m in p.conferir()))

    def test_regra_desconhecida_e_erro(self):
        p = Projeto(self, {}, 'int64 | a.go | var x int64 | motivo\n')
        self.assertTrue(any('desconhecida' in m for _, _, m in p.conferir()))

    def test_comentarios_e_linhas_em_branco_na_lista_sao_ignorados(self):
        p = Projeto(self, self.CODIGO,
                    '# um comentário\n\nfloat64 | internal/selic/serie.go | var acumulado []float64 | fator da Selic\n')
        self.assertEqual(p.conferir(), [])

    def test_o_time_sleep_de_teste_excetuado_passa(self):
        fonte = 'package banco\n\nfunc g() {\n\ttime.Sleep(r.segurar)\n}\n'
        p = Projeto(self, {'internal/banco/wal_test.go': fonte},
                    'time.Sleep | internal/banco/wal_test.go | time.Sleep(r.segurar) | o tempo é a carga do teste\n')
        self.assertEqual(p.conferir(), [])


class SaidaDoScript(unittest.TestCase):
    def test_codigo_de_saida(self):
        limpo = Projeto(self, {'internal/x/x.go': 'package x\n'})
        sujo = Projeto(self, {'internal/x/x.go': 'package x\n\nvar v = time.Now()\n'})
        for projeto, esperado in ((limpo, 0), (sujo, 1)):
            argv = ['verificar-regras-go.py', '--raiz', projeto.pasta.name, '--excecoes', projeto.excecoes]
            antigo, regras.sys.argv = regras.sys.argv, argv
            try:
                with open(os.devnull, 'w', encoding='utf-8') as nulo:
                    saida, regras.sys.stdout = regras.sys.stdout, nulo
                    try:
                        self.assertEqual(regras.main(), esperado)
                    finally:
                        regras.sys.stdout = saida
            finally:
                regras.sys.argv = antigo


if __name__ == '__main__':
    unittest.main()
