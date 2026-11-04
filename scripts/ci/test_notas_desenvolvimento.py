"""Testes do notas-desenvolvimento.sh (#600): as notas cabem no limite do GitHub.

Rodar: python3 -B scripts/ci/test_notas_desenvolvimento.py
Cada teste monta um repositório git temporário com um CHANGELOG.md e roda o script nele, sem o GitHub
(NOTAS_ANTERIORES vazio e um commit sem merge de PR: o script não chama o gh).
"""
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "notas-desenvolvimento.sh"
LINK = "As versões mais antigas não cabem aqui"


def secao(versao, linhas):
    corpo = "".join(f"- Mudança {i} da {versao}, com um texto comprido o bastante para pesar nas notas.\n"
                    for i in range(linhas))
    return f"## [{versao}] — 2026-11-02 · desenvolvimento\n\n### Correções\n\n{corpo}\n"


def changelog(quantas, linhas):
    versoes = "".join(secao(f"2.{quantas - i}.0", linhas) for i in range(quantas))
    return "# Changelog\n\n" + versoes + "## [2.0.0] — 2026-10-16\n\n### Funcionalidades\n\n- Publicada.\n"


class NotasDesenvolvimento(unittest.TestCase):
    def gerar(self, conteudo, limite=None):
        with tempfile.TemporaryDirectory() as pasta:
            git = ["git", "-C", pasta, "-c", "core.autocrlf=false", "-c", "user.name=teste", "-c", "user.email=teste@exemplo"]
            subprocess.run(["git", "init", "-q", pasta], check=True)
            Path(pasta, "CHANGELOG.md").write_text(conteudo, encoding="utf-8", newline="\n")
            Path(pasta, "anteriores.md").write_text("", encoding="utf-8")
            subprocess.run(git + ["add", "CHANGELOG.md"], check=True)
            subprocess.run(git + ["commit", "-q", "-m", "primeiro"], check=True)
            subprocess.run(git + ["commit", "-q", "--allow-empty", "-m", "segundo"], check=True)
            sha = subprocess.run(git + ["rev-parse", "HEAD"], check=True, capture_output=True, text=True).stdout.strip()
            env = dict(os.environ, ROTULO="2.9.0-dev.1", GITHUB_SHA=sha, GITHUB_REPOSITORY="dono/repo",
                       NOTAS_ANTERIORES=str(Path(pasta, "anteriores.md")))
            if limite is not None:
                env["LIMITE_NOTAS"] = str(limite)
            r = subprocess.run([shutil.which("bash") or "bash", str(SCRIPT)], cwd=pasta, env=env,
                               capture_output=True, check=True)
            return r.stdout.decode("utf-8")

    def test_changelog_pequeno_entra_inteiro(self):
        notas = self.gerar(changelog(3, 2))
        self.assertNotIn(LINK, notas)
        self.assertIn("<!-- por-versao:inicio -->", notas)
        self.assertIn("<!-- por-versao:fim -->", notas)
        resto = notas.split("## Tudo desde a última versão publicada\n\n", 1)[1]
        esperado = "".join(secao(f"2.{3 - i}.0", 2) for i in range(3))
        esperado = esperado.replace("## [", "### ").replace("] —", " —").replace("### Correções", "#### Correções")
        self.assertEqual(resto, esperado)  # o mesmo de antes do #600; para na versão publicada (2.0.0)

    def test_changelog_grande_e_cortado_no_fim_de_uma_versao(self):
        conteudo = changelog(300, 20)
        self.assertGreater(len(conteudo), 125000)
        notas = self.gerar(conteudo)
        self.assertLessEqual(len(notas.encode("utf-8")), 120000)
        self.assertIn("<!-- por-versao:fim -->", notas)
        self.assertIn(LINK, notas)
        self.assertIn("/blob/", notas.splitlines()[-1])
        antes_do_link = notas.split("\n\n_" + LINK, 1)[0]
        ultima = antes_do_link.rstrip("\n").splitlines()[-1]
        self.assertTrue(ultima.startswith("- Mudança 19 da "), ultima)  # a última versão que entrou está inteira

    def test_limite_menor_que_o_cabecalho_deixa_so_o_link(self):
        notas = self.gerar(changelog(3, 2), limite=10)
        self.assertNotIn("### 2.3.0", notas)
        self.assertIn(LINK, notas)


if __name__ == "__main__":
    unittest.main()
