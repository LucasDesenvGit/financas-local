"""Testes do notas.sh, nos dois modos (#641, #600): as notas dos Releases cabem no limite do GitHub.

Rodar: python3 -B scripts/ci/versao/test_notas.py
--versao: cada teste grava um CHANGELOG.md numa pasta temporária e roda o script nela, sem o GitHub.
--desenvolvimento: monta um repositório git temporário com um CHANGELOG.md e roda o script nele, sem o GitHub
(NOTAS_ANTERIORES vazio e um commit sem merge de PR: o script não chama o gh).
"""
import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "notas.sh"
LINK = "As versões mais antigas não cabem aqui"
LINHA_LINK = (f"_{LINK}: veja o [CHANGELOG.md](https://github.com/dono/repo/blob/v2.9.0/CHANGELOG.md)._")
MUDOU = "## O que mudou\n\n"


def secao(versao, linhas, rotulo=" · desenvolvimento"):
    corpo = "".join(f"- Mudança {i} da {versao}, com um texto comprido o bastante para pesar nas notas.\n"
                    for i in range(linhas))
    return f"## [{versao}] — 2026-11-05{rotulo}\n\n### Correções\n\n{corpo}\n"


def changelog(quantas, linhas, topo="2.9.0"):
    """A versão do topo publicada, `quantas` de desenvolvimento abaixo dela e uma publicada antes delas."""
    versoes = "".join(secao(f"2.8.{quantas - i}", linhas) for i in range(quantas))
    return ("# Changelog\n\n" + secao(topo, linhas, rotulo="") + versoes
            + "## [2.0.0] — 2026-10-16\n\n### Funcionalidades\n\n- Publicada antes.\n")


def um_nivel_abaixo(texto):
    return (texto.replace("## [", "### ").replace("] —", " —").replace(" · desenvolvimento", "")
            .replace("### Correções", "#### Correções"))


class NotasDaVersao(unittest.TestCase):
    def gerar(self, conteudo, versao="2.9.0", limite=None, crlf=False):
        with tempfile.TemporaryDirectory() as pasta:
            texto = conteudo.replace("\n", "\r\n") if crlf else conteudo
            Path(pasta, "CHANGELOG.md").write_text(texto, encoding="utf-8", newline="")
            env = dict(os.environ, VERSAO=versao, GITHUB_REPOSITORY="dono/repo")
            if limite is not None:
                env["LIMITE_NOTAS"] = str(limite)
            r = subprocess.run([shutil.which("bash") or "bash", str(SCRIPT), "--versao"], cwd=pasta, env=env,
                               capture_output=True, check=True)
            return r.stdout.decode("utf-8")

    def test_changelog_pequeno_entra_inteiro(self):
        notas = self.gerar(changelog(3, 2))
        self.assertIn("FinancasLocal-2.9.0.msi", notas)
        self.assertIn("https://github.com/dono/repo/blob/v2.9.0/docs/INSTALACAO-PELO-CODIGO.md", notas)
        self.assertNotIn(LINK, notas)
        esperado = secao("2.9.0", 2, rotulo="") + "".join(secao(f"2.8.{3 - i}", 2) for i in range(3))
        # O mesmo de antes do #641: para na versão publicada antes (2.0.0), sem as linhas em branco do fim.
        self.assertEqual(notas.split(MUDOU, 1)[1], um_nivel_abaixo(esperado).rstrip("\n") + "\n")

    def test_changelog_grande_e_cortado_no_fim_de_uma_versao(self):
        conteudo = changelog(300, 20)
        self.assertGreater(len(conteudo), 125000)
        notas = self.gerar(conteudo)
        self.assertLessEqual(len(notas), 125000)
        self.assertLessEqual(len(notas.encode("utf-8")), 120000)
        self.assertIn("https://github.com/dono/repo/blob/v2.9.0/CHANGELOG.md", notas)
        resto, link = notas.split(MUDOU, 1)[1].rsplit("\n\n", 1)
        self.assertIn(LINK, link)
        self.assertTrue(resto.startswith("### 2.9.0 — 2026-11-05\n"))
        # Toda versão que entrou está inteira: as 20 linhas dela.
        versoes = re.split(r"(?m)^### ", resto)[1:]
        self.assertGreater(len(versoes), 1)
        for v in versoes:
            self.assertEqual(v.count("- Mudança "), 20, v[:30])

    def test_corta_no_byte_em_que_o_link_nao_cabe(self):
        conteudo = changelog(3, 2)
        inteiro = self.gerar(conteudo)
        # Tudo cabe quando sobra lugar para o link depois da última versão (com as 2 linhas em branco que vêm antes
        # do link, e a linha em branco do fim da versão, que sai das notas).
        cabe = len(inteiro.encode("utf-8")) + 1 + len(LINHA_LINK.encode("utf-8")) + 2
        self.assertEqual(self.gerar(conteudo, limite=cabe), inteiro)
        # Um byte a menos: a última versão sai inteira, e o link entra no lugar dela.
        notas = self.gerar(conteudo, limite=cabe - 1)
        sem_a_ultima = inteiro.split("### 2.8.1 —", 1)[0].rstrip("\n")
        self.assertEqual(notas, sem_a_ultima + "\n\n" + LINHA_LINK + "\n")

    def test_versoes_acima_da_tag_ficam_de_fora(self):
        conteudo = changelog(2, 2).replace("# Changelog\n\n", "# Changelog\n\n" + secao("2.10.0", 2), 1)
        resto = self.gerar(conteudo).split(MUDOU, 1)[1]
        self.assertTrue(resto.startswith("### 2.9.0 —"))
        self.assertNotIn("2.10.0", resto)

    def test_sem_a_secao_da_versao_usa_a_nao_publicado(self):
        conteudo = changelog(1, 2).replace("## [2.9.0] — 2026-11-05", "## [Não publicado]", 1)
        resto = self.gerar(conteudo).split(MUDOU, 1)[1]
        self.assertTrue(resto.startswith("### Não publicado\n"))
        self.assertIn("### 2.8.1 —", resto)

    def test_sem_versao_publicada_antes_vai_ate_o_fim(self):
        conteudo = "# Changelog\n\n" + secao("2.9.0", 2, rotulo="") + secao("2.8.1", 2)
        resto = self.gerar(conteudo).split(MUDOU, 1)[1]
        self.assertEqual(resto, um_nivel_abaixo(conteudo.split("\n\n", 1)[1]).rstrip("\n") + "\n")

    def test_crlf_da_o_mesmo_que_lf(self):
        conteudo = changelog(3, 2)
        self.assertEqual(self.gerar(conteudo, crlf=True), self.gerar(conteudo))



def secao_desenvolvimento(versao, linhas):
    corpo = "".join(f"- Mudança {i} da {versao}, com um texto comprido o bastante para pesar nas notas.\n"
                    for i in range(linhas))
    return f"## [{versao}] — 2026-11-02 · desenvolvimento\n\n### Correções\n\n{corpo}\n"


def changelog_desenvolvimento(quantas, linhas):
    versoes = "".join(secao_desenvolvimento(f"2.{quantas - i}.0", linhas) for i in range(quantas))
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
            r = subprocess.run([shutil.which("bash") or "bash", str(SCRIPT), "--desenvolvimento"], cwd=pasta, env=env,
                               capture_output=True, check=True)
            return r.stdout.decode("utf-8")

    def test_changelog_pequeno_entra_inteiro(self):
        notas = self.gerar(changelog_desenvolvimento(3, 2))
        self.assertNotIn(LINK, notas)
        self.assertIn("<!-- por-versao:inicio -->", notas)
        self.assertIn("<!-- por-versao:fim -->", notas)
        resto = notas.split("## Tudo desde a última versão publicada\n\n", 1)[1]
        esperado = "".join(secao_desenvolvimento(f"2.{3 - i}.0", 2) for i in range(3))
        esperado = esperado.replace("## [", "### ").replace("] —", " —").replace("### Correções", "#### Correções")
        self.assertEqual(resto, esperado)  # o mesmo de antes do #600; para na versão publicada (2.0.0)

    def test_changelog_grande_e_cortado_no_fim_de_uma_versao(self):
        conteudo = changelog_desenvolvimento(300, 20)
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
        notas = self.gerar(changelog_desenvolvimento(3, 2), limite=10)
        self.assertNotIn("### 2.3.0", notas)
        self.assertIn(LINK, notas)


if __name__ == "__main__":
    unittest.main()
