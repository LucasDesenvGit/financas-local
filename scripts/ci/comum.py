"""
O que os scripts de scripts/ci/ repetiam (#655): a lista de arquivos que um PR mudou.

Os scripts têm hífen no nome e não se importam, mas este arquivo se importa: o Python põe a pasta do script no
sys.path, então cada script põe a pasta scripts/ci no dele antes do import:

    sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), '..'))   # scripts em scripts/ci/<assunto>/
    from comum import arquivos_mudados

(A versão e a regra "feat sobe o número do meio" saíram daqui na ADR 0070: o número é calculado pelo
src/backend/cmd/versao, em Go.)
"""
import subprocess


def arquivos_mudados(base, cwd=None, check=True):
    """Arquivos que o PR mudou em relação à base (o que o merge leva: `git diff --name-only <base>...HEAD`)."""
    saida = subprocess.run(['git', 'diff', '--name-only', f'{base}...HEAD'], cwd=cwd, check=check,
                           capture_output=True, text=True).stdout
    return [a for a in saida.splitlines() if a.strip()]
