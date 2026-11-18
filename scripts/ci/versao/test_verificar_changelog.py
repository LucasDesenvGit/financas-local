"""Testes do verificar-changelog.py (ADR 0070): o formato dos arquivos de changelog.d/, o arquivo obrigatório nos PRs feat e
fix e o CHANGELOG.md em ordem.

Rodar: python3 -B scripts/ci/versao/test_verificar_changelog.py
"""
import importlib.util
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "verificar-changelog.py"
spec = importlib.util.spec_from_file_location("verificar_changelog", SCRIPT)
vc = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vc)

BOA = "### Correções\n- **Corrige** o que estava errado (#1).\n  - Técnico: a coluna.\n"


class FormatoDaEntrada(unittest.TestCase):
    def test_entrada_boa_nao_tem_problema(self):
        self.assertEqual(vc.problemas_da_entrada("changelog.d/a.md", BOA), [])

    def test_varios_tipos_e_varias_entradas(self):
        texto = "### Funcionalidades\n- **A** (#1).\n- **B** (#2).\n\n### Publicação e documentação\n- **C** (#3).\n"
        self.assertEqual(vc.problemas_da_entrada("changelog.d/a.md", texto), [])

    def test_arquivo_vazio(self):
        self.assertEqual(vc.problemas_da_entrada("changelog.d/a.md", "\n  \n"), ["changelog.d/a.md: vazio"])

    def test_sem_o_titulo_do_tipo(self):
        achados = vc.problemas_da_entrada("changelog.d/a.md", "- **A** (#1).\n")
        self.assertEqual(len(achados), 1)
        self.assertIn("a primeira linha é o título do tipo", achados[0])

    def test_tipo_que_nao_existe(self):
        achados = vc.problemas_da_entrada("changelog.d/a.md", "### Novidades\n- **A** (#1).\n")
        self.assertEqual(len(achados), 1)
        self.assertIn('o tipo "### Novidades" não existe', achados[0])

    def test_tipo_repetido(self):
        achados = vc.problemas_da_entrada("changelog.d/a.md", BOA + "\n" + BOA)
        self.assertEqual(len(achados), 1)
        self.assertIn("repetido", achados[0])

    def test_tipo_sem_entrada(self):
        achados = vc.problemas_da_entrada("changelog.d/a.md", "### Correções\ntexto solto, sem o traço\n")
        self.assertEqual(len(achados), 1)
        self.assertIn('não tem entrada', achados[0])

    def test_titulo_de_versao_nao_vale(self):
        achados = vc.problemas_da_entrada("changelog.d/a.md", "## [3.25.0] — 2026-11-16\n\n" + BOA)
        self.assertTrue(any("o número é calculado no merge" in a for a in achados), achados)


def palavras(n):
    return " ".join(["palavra"] * n)


class AvisoDeTamanho(unittest.TestCase):
    """#722: até ~60 palavras para quem usa; o sub-item "Técnico" não conta, e passar do limite só avisa."""

    def avisos(self, texto):
        return vc.avisos_da_entrada("changelog.d/a.md", texto)

    def test_limite_em_tabela(self):
        casos = {
            "no limite": (f"### Correções\n- {palavras(vc.LIMITE_PALAVRAS)}\n", 0),
            "um acima": (f"### Correções\n- {palavras(vc.LIMITE_PALAVRAS + 1)}\n", 1),
            "continua na linha de baixo": (f"### Correções\n- {palavras(40)}\n  {palavras(40)}\n", 1),
            "Técnico comprido não conta": (f"### Correções\n- {palavras(10)}\n  - Técnico: {palavras(200)}\n", 0),
            "continuação do Técnico não conta": (f"### Correções\n- {palavras(10)}\n  - Técnico: a\n    {palavras(200)}\n", 0),
            "outro sub-item conta": (f"### Correções\n- {palavras(10)}\n  - {palavras(60)}\n", 1),
            "cada entrada à parte": (f"### Correções\n- {palavras(40)}\n- {palavras(40)}\n", 0),
            "duas longas, dois avisos": (f"### Correções\n- {palavras(70)}\n### Funcionalidades\n- {palavras(70)}\n", 2),
        }
        for nome, (texto, quantos) in casos.items():
            with self.subTest(nome):
                self.assertEqual(len(self.avisos(texto)), quantos, self.avisos(texto))

    def test_o_aviso_diz_a_entrada_e_o_numero(self):
        aviso = self.avisos(f"### Correções\n- **Título** {palavras(70)}\n")[0]
        self.assertIn('"- **Título** palavra', aviso)
        self.assertIn("tem 71 palavras", aviso)

    def test_entrada_longa_continua_no_formato(self):
        # Aviso, não erro: o que barra (problemas_da_entrada) não olha o tamanho.
        self.assertEqual(vc.problemas_da_entrada("changelog.d/a.md", f"### Correções\n- {palavras(80)}\n"), [])


class PastaDoChangelog(unittest.TestCase):
    def test_lista_so_os_md_e_ignora_o_leia_me(self):
        with tempfile.TemporaryDirectory() as raiz:
            pasta = Path(raiz, "changelog.d")
            pasta.mkdir()
            (pasta / "LEIA-ME.md").write_text("x", encoding="utf-8")
            (pasta / "feature-a.md").write_text(BOA, encoding="utf-8")
            (pasta / "anotacao.txt").write_text("x", encoding="utf-8")
            (pasta / "sub").mkdir()
            achadas, problemas = vc.entradas(raiz)
            self.assertEqual([c for c, _ in achadas], ["changelog.d/feature-a.md"])
            self.assertEqual(len(problemas), 2)
            self.assertTrue(any("subpasta" in p for p in problemas))
            self.assertTrue(any("só arquivos .md" in p for p in problemas))

    def test_sem_a_pasta(self):
        with tempfile.TemporaryDirectory() as raiz:
            self.assertEqual(vc.entradas(raiz), ([], ["a pasta changelog.d/ não existe"]))


class ArquivoObrigatorioNoPr(unittest.TestCase):
    def test_so_feat_e_fix_precisam(self):
        for titulo in ("feat(web): x", "feat(banco)!: x", "fix: x", "  fix(api): x"):
            with self.subTest(titulo):
                self.assertEqual(len(vc.problemas_do_pr(titulo, [])), 1)
                self.assertEqual(vc.problemas_do_pr(titulo, ["changelog.d/a.md"]), [])
        for titulo in ("docs: x", "ci(go): x", "test: x", "build(deps): x", "chore: x", "refactor(web): x", "perf: x", "style: x",
                       "Atualiza uma coisa", ""):
            with self.subTest(titulo):
                self.assertEqual(vc.problemas_do_pr(titulo, []), [])

    def test_arquivos_novos_pelo_git(self):
        with tempfile.TemporaryDirectory() as raiz:
            def git(*args):
                subprocess.run(["git", "-C", raiz, "-c", "user.name=t", "-c", "user.email=t@e", "-c", "commit.gpgsign=false", *args],
                               check=True, capture_output=True)
            git("init", "-q", "-b", "develop")
            Path(raiz, "changelog.d").mkdir()
            Path(raiz, "changelog.d", "LEIA-ME.md").write_text("x", encoding="utf-8")
            Path(raiz, "changelog.d", "velha.md").write_text(BOA, encoding="utf-8")
            git("add", "-A")
            git("commit", "-q", "-m", "base")
            git("update-ref", "refs/remotes/origin/develop", "HEAD")
            git("switch", "-q", "-c", "fix/x")
            Path(raiz, "changelog.d", "fix-x.md").write_text(BOA, encoding="utf-8")
            Path(raiz, "changelog.d", "velha.md").write_text(BOA + "- editada\n", encoding="utf-8")
            Path(raiz, "changelog.d", "LEIA-ME.md").write_text("y", encoding="utf-8")
            Path(raiz, "outro.txt").write_text("x", encoding="utf-8")
            git("add", "-A")
            git("commit", "-q", "-m", "fix: x")
            # Só o arquivo acrescentado conta: editar um que já existia (ou o LEIA-ME.md) não é a entrada deste PR.
            self.assertEqual(vc.arquivos_novos("origin/develop", raiz), ["changelog.d/fix-x.md"])


class ChangelogEmOrdem(unittest.TestCase):
    def test_titulo_repetido_na_mesma_versao_e_versao_fora_de_ordem(self):
        texto = ("# Changelog\n\n## [3.0.0] — 2026-11-02\n\n### Correções\n- a\n\n### Correções\n- b\n\n"
                 "## [3.1.0] — 2026-11-03\n\n### Funcionalidades\n- c\n")
        versoes = vc.partes(texto)
        self.assertEqual(vc.repetidas(versoes), [("## [3.0.0] — 2026-11-02", "### Correções")])
        self.assertEqual(vc.problemas_de_versao(versoes), ["fora de ordem (a mais nova vai em cima): ## [3.0.0] — 2026-11-02"])
        juntado = vc.juntar(versoes)
        self.assertEqual(juntado.count("### Correções"), 1)
        self.assertIn("- a\n- b\n", juntado)

    def test_versao_repetida(self):
        texto = "## [3.0.0] — x\n\n### Correções\n- a\n\n## [3.0.0] — y\n\n### Correções\n- b\n"
        self.assertTrue(any("versão repetida" in p for p in vc.problemas_de_versao(vc.partes(texto))))

    def test_changelog_em_ordem(self):
        texto = "# C\n\n## [3.1.0] — x · desenvolvimento\n\n### Correções\n- a\n\n## [3.0.0] — y\n\n### Correções\n- b\n"
        self.assertEqual(vc.problemas_de_versao(vc.partes(texto)), [])


class DoRepositorio(unittest.TestCase):
    def test_o_changelog_d_e_o_changelog_do_repositorio_estao_em_ordem(self):
        arquivos, problemas = vc.entradas()
        achados = list(problemas)
        for caminho, conteudo in arquivos:
            achados += vc.problemas_da_entrada(caminho, conteudo)
        self.assertEqual(achados, [])
        versoes = vc.partes(io_ler(vc.RAIZ))
        self.assertEqual(vc.problemas_de_versao(versoes), [])
        self.assertEqual(vc.repetidas(versoes), [])


def io_ler(raiz):
    return Path(raiz, "CHANGELOG.md").read_text(encoding="utf-8").replace("\r\n", "\n")


if __name__ == "__main__":
    os.environ.pop("BASE", None)
    unittest.main()
