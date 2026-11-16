#!/usr/bin/env python3
"""
Teste do scripts/ci/go/mutacao-go.py (#716): o pacote novo (fora da lista e de mutacao-go-fora.txt) não é erro, é
medido sem piso quando o PR o muda, e o aviso dá a linha da lista com a eficácia medida. Os erros que continuam:
pacote nas duas listas, linha de pacote que não existe e pacote de fora sem o motivo. Só a biblioteca padrão
(unittest), sem Go nem gremlins.

Uso:  python3 -B scripts/ci/go/test_mutacao_go.py
"""
import importlib.util
import os
import unittest

AQUI = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location('mutacao', os.path.join(AQUI, 'mutacao-go.py'))
mutacao = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mutacao)

EXISTENTES = {'internal/a/lista', 'internal/a/fora', 'internal/a/novo'}
LIMITES = {'internal/a/lista': 90.0}
FORA = {'internal/a/fora': 'só montagem'}


class ListaTeste(unittest.TestCase):
    def test_pacote_novo_nao_e_erro(self):
        self.assertEqual(mutacao.conferir_lista(LIMITES, FORA, EXISTENTES), [])
        self.assertEqual(mutacao.pacotes_novos(LIMITES, FORA, EXISTENTES), ['internal/a/novo'])

    def test_os_erros_que_continuam(self):
        casos = [
            ('nas duas listas', {'internal/a/lista': 90.0, 'internal/a/fora': 80.0}, FORA, 'deixe só num dos dois'),
            ('linha de pacote que não existe', {**LIMITES, 'internal/a/sumiu': 70.0}, FORA, 'não existe'),
            ('fora sem o motivo', LIMITES, {'internal/a/fora': ''}, 'sem o motivo'),
        ]
        for nome, limites, fora, trecho in casos:
            with self.subTest(nome):
                erros = mutacao.conferir_lista(limites, fora, EXISTENTES)
                self.assertEqual(len(erros), 1, erros)
                self.assertIn(trecho, erros[0])

    def test_sem_pacote_novo(self):
        self.assertEqual(mutacao.pacotes_novos({**LIMITES, 'internal/a/novo': 80.0}, FORA, EXISTENTES), [])


class NoPrTeste(unittest.TestCase):
    def test_pacote_novo_mudado_e_medido_e_o_de_fora_nao(self):
        alvos = list(LIMITES) + ['internal/a/novo']
        arquivos = ['src/backend/internal/a/novo/novo.go', 'src/backend/internal/a/fora/fora.go',
                    'src/backend/internal/a/lista/lista_test.go', 'docs/README.md']
        self.assertEqual(mutacao.pacotes_mudados(arquivos, alvos), ['internal/a/lista', 'internal/a/novo'])

    def test_so_a_mutacao_mudou_mede_o_pacote_de_prova(self):
        self.assertEqual(mutacao.escolher_no_pr(['scripts/ci/go/mutacao-go.py'], list(LIMITES)),
                         [mutacao.PACOTE_DE_PROVA])


class AvisoTeste(unittest.TestCase):
    def test_linha_para_a_lista_arredonda_para_baixo(self):
        casos = [(87.9, 'internal/a/novo 87'), (100.0, 'internal/a/novo 100'), (0.4, 'internal/a/novo 0')]
        for eficacia, esperada in casos:
            with self.subTest(eficacia):
                self.assertEqual(mutacao.linha_para_a_lista({'pacote': 'internal/a/novo', 'eficacia': eficacia}),
                                 esperada)

    def test_relatorio_mostra_o_novo_sem_piso(self):
        resultado = {'pacote': 'internal/a/novo', 'mortas': 7, 'vivas': [], 'equivalentes': [], 'sem_teste': 0,
                     'eficacia': 70.0}
        texto = mutacao.relatorio([resultado], LIMITES)
        self.assertIn('— (sem piso', texto)
        self.assertNotIn('❌', texto)


if __name__ == '__main__':
    unittest.main()
