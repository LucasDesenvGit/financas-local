#!/usr/bin/env python3
"""
Teste do scripts/geral/atualizar-versao-do-pr.py (#587): a versão nova a partir da do develop, no nível que o PR subiu
e nunca abaixo do tipo do título, e a seção do PR no topo do CHANGELOG. Os casos com git de verdade (merge parado nos
conflitos) usam um repositório temporário. Só a biblioteca padrão (unittest).

Uso:  python3 -B scripts/geral/test_atualizar_versao_do_pr.py
"""
import datetime
import importlib.util
import os
import subprocess
import sys
import tempfile
import unittest

AQUI = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(AQUI, 'atualizar-versao-do-pr.py')
spec = importlib.util.spec_from_file_location('atualizar', SCRIPT)
atualizar = importlib.util.module_from_spec(spec)
spec.loader.exec_module(atualizar)

HOJE = datetime.date(2026, 11, 2)
CABECALHO = '# Changelog\n\nFormato.\n\n'


def props(v):
    return f'<Project>\n  <PropertyGroup>\n    <Version>{v}</Version>\n  </PropertyGroup>\n</Project>\n'


def secao(v, data, tipo, texto):
    return f'## [{v}] — {data} · desenvolvimento\n\n### {tipo}\n- {texto}\n\n'


class NovaVersao(unittest.TestCase):
    # Regra: parte da versão do develop; sobe no nível em que o PR tinha subido em relação à base dele, e pelo menos no
    # nível do título (feat → o do meio; o resto → o último); quem sobe um número zera os de depois.
    def test_casos(self):
        casos = [
            # develop, base do PR, PR, título, esperado
            ((2, 58, 0), (2, 57, 0), (2, 57, 1), 'fix(api): x', (2, 58, 1)),
            ((2, 58, 0), (2, 57, 0), (2, 58, 0), 'feat(api): x', (2, 59, 0)),
            ((2, 58, 3), (2, 57, 0), (2, 58, 0), 'feat: x', (2, 59, 0)),
            ((2, 58, 0), (2, 57, 0), (3, 0, 0), 'feat!: quebra', (3, 0, 0)),
            ((3, 1, 2), (2, 57, 0), (3, 0, 0), 'fix: quebra de verdade', (4, 0, 0)),
            # O PR subiu menos do que o título pede (só o último num feat): vale o título.
            ((2, 58, 0), (2, 57, 0), (2, 57, 1), 'feat(web): x', (2, 59, 0)),
            # O PR subiu mais do que o título pede (o do meio num docs): vale o PR.
            ((2, 58, 0), (2, 57, 0), (2, 58, 0), 'docs: x', (2, 59, 0)),
            # O PR não tinha subido (ou desceu): vale o título.
            ((2, 58, 0), (2, 57, 0), (2, 57, 0), 'docs: x', (2, 58, 1)),
            ((2, 58, 0), (2, 57, 0), (2, 56, 9), 'feat: x', (2, 59, 0)),
            # "feature" não é "feat".
            ((2, 58, 0), (2, 57, 0), (2, 57, 0), 'featured: x', (2, 58, 1)),
        ]
        for develop, base, pr, titulo, quer in casos:
            with self.subTest(develop=develop, base=base, pr=pr, titulo=titulo):
                self.assertEqual(atualizar.nova_versao(develop, base, pr, titulo), quer)


class Changelog(unittest.TestCase):
    # Regra: a seção que o PR acrescentou vai para o topo do CHANGELOG do develop, com o número novo e a data de hoje; o
    # resto do develop fica como está.
    def test_secao_no_topo(self):
        diff = ('--- a/CHANGELOG.md\n+++ b/CHANGELOG.md\n@@ -4,0 +5,5 @@\n'
                '+## [2.57.1] — 2026-11-01 · desenvolvimento\n+\n+### Correções\n+- **A** (#575).\n+\n')
        develop = CABECALHO + secao('2.58.0', '2026-11-02', 'Funcionalidades', 'B') + secao('2.57.0', '2026-11-01',
                                                                                             'Funcionalidades', 'C')
        novo = atualizar.montar_changelog(develop, atualizar.secao_do_pr(diff), (2, 58, 1), HOJE)
        self.assertEqual(novo, CABECALHO + secao('2.58.1', '2026-11-02', 'Correções', '**A** (#575).') + develop[len(
            CABECALHO):])

    def test_o_que_nao_resolve_sozinho(self):
        casos = {
            'tirou linha': '-## [2.57.0] — 2026-11-01 · desenvolvimento\n+## [2.57.1] — 2026-11-01 · desenvolvimento\n',
            'sem entrada': '',
            'só uma entrada, sem seção': '+- **A** (#575).\n',
            'duas seções': '+## [2.57.2] — 2026-11-01 · x\n+- a\n+## [2.57.1] — 2026-11-01 · x\n+- b\n',
        }
        for nome, diff in casos.items():
            with self.subTest(nome), self.assertRaises(atualizar.Problema):
                atualizar.secao_do_pr(diff)
        with self.assertRaises(atualizar.Problema):
            atualizar.montar_changelog('# Changelog\n', ['## [1.0.0] — 2026-01-01'], (1, 0, 1), HOJE)
        with self.assertRaises(atualizar.Problema):
            atualizar.versao('<Project/>')


class ComGit(unittest.TestCase):
    """Um repositório de mentira: o develop e um PR que subiram a versão ao mesmo tempo, e o merge parado."""

    def setUp(self):
        pasta = tempfile.TemporaryDirectory()
        self.addCleanup(pasta.cleanup)
        self.pasta = pasta.name
        self.git('init', '-q', '-b', 'develop')
        self.git('config', 'user.email', 'teste@exemplo')
        self.git('config', 'user.name', 'Teste')
        self.git('config', 'core.autocrlf', 'false')
        self.escrever('2.57.0', secao('2.57.0', '2026-11-01', 'Funcionalidades', 'Base'))
        self.git('add', '-A')
        self.git('commit', '-q', '-m', 'base')
        self.git('checkout', '-q', '-b', 'pr')
        self.escrever('2.57.1', secao('2.57.1', '2026-11-01', 'Correções', '**Do PR** (#1).') +
                      secao('2.57.0', '2026-11-01', 'Funcionalidades', 'Base'))
        self.git('commit', '-q', '-am', 'fix: do PR')
        self.git('checkout', '-q', 'develop')
        self.escrever('2.58.0', secao('2.58.0', '2026-11-02', 'Funcionalidades', 'Do outro') +
                      secao('2.57.0', '2026-11-01', 'Funcionalidades', 'Base'))
        self.git('commit', '-q', '-am', 'feat: do outro')
        self.git('checkout', '-q', 'pr')

    def git(self, *argumentos, check=True):
        return subprocess.run(['git', *argumentos], cwd=self.pasta, check=check, capture_output=True, text=True,
                              encoding='utf-8')

    def escrever(self, versao, secoes):
        with open(os.path.join(self.pasta, 'Directory.Build.props'), 'w', encoding='utf-8', newline='') as f:
            f.write(props(versao))
        with open(os.path.join(self.pasta, 'CHANGELOG.md'), 'w', encoding='utf-8', newline='') as f:
            f.write(CABECALHO + secoes)

    def ler(self, nome):
        with open(os.path.join(self.pasta, nome), encoding='utf-8') as f:
            return f.read()

    # Regra: no merge parado, o script grava a versão do develop subida no nível do PR e a seção do PR no topo, e os
    # dois arquivos ficam sem marcas de conflito.
    def test_resolve_o_merge(self):
        self.assertNotEqual(self.git('merge', 'develop', check=False).returncode, 0)
        rodou = subprocess.run([sys.executable, SCRIPT, 'fix(api): do PR'], cwd=self.pasta,
                               capture_output=True, text=True, encoding='utf-8')
        self.assertEqual(rodou.returncode, 0, rodou.stderr)
        self.assertIn('<Version>2.58.1</Version>', self.ler('Directory.Build.props'))
        hoje = f'{datetime.date.today():%Y-%m-%d}'
        self.assertEqual(self.ler('CHANGELOG.md'), CABECALHO + secao('2.58.1', hoje, 'Correções', '**Do PR** (#1).') +
                         secao('2.58.0', '2026-11-02', 'Funcionalidades', 'Do outro') +
                         secao('2.57.0', '2026-11-01', 'Funcionalidades', 'Base'))

    # Regra: o que não dá para resolver sozinho sai com erro e a mensagem, sem gravar nada.
    def test_sem_secao_avisa(self):
        self.git('reset', '-q', '--hard', 'HEAD~1')
        with open(os.path.join(self.pasta, 'Directory.Build.props'), 'w', encoding='utf-8', newline='') as f:
            f.write(props('2.57.1'))
        self.git('commit', '-q', '-am', 'fix: sem CHANGELOG')
        self.git('merge', 'develop', check=False)
        rodou = subprocess.run([sys.executable, SCRIPT, 'fix: x'], cwd=self.pasta,
                               capture_output=True, text=True, encoding='utf-8')
        self.assertNotEqual(rodou.returncode, 0)
        self.assertIn('não escreveu no CHANGELOG.md', rodou.stderr)


if __name__ == '__main__':
    unittest.main()
