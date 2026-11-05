#!/usr/bin/env python3
"""Mede o tempo de partida e o das telas principais com um volume de referência (#470).

As metas e o que fazer com os números estão em docs/REQUISITOS-NAO-FUNCIONAIS.md.

Roda sobre o site já publicado (o financas-go, #86), numa pasta temporária e numa porta própria, com o banco fictício do
contrato (src/backend/testes/contrato/dados/base.sql) e as transações dele repetidas pelos últimos
anos. Não toca na pasta dados/ nem no site instalado, e a saída para a internet vai para uma porta
que recusa na hora: nada de dado real entra, nada sai.

Uso (a pasta publicada, ou uma com o financas-go e o appsettings.json):
    (cd src/backend && CGO_ENABLED=0 go build -trimpath -o <pasta>/financas-go ./cmd/financas)
    cp src/backend/appsettings.json <pasta>/
    cp -r src/frontend/dist/financas-web/browser <pasta>/wwwroot   # depois do npm run build
    python3 scripts/geral/medir-desempenho.py --api <pasta>

Com a pasta do site instalado (publicado/api), o banco, as chaves e o certificado dela não são usados: o Go
recebe outros pelas opções.
"""
import argparse
import datetime
import http.client
import json
import os
import shutil
import socket
import sqlite3
import ssl
import statistics
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[2]
BASE_SQL = RAIZ / "src/backend/testes/contrato/dados/base.sql"
LOGIN = "ana.teste"
SENHA = "Senha-Forte-123"
SEM_VERIFICAR = ssl._create_unverified_context()  # só 127.0.0.1, com o certificado em memória desta rodada


def telas(hoje):
    """As chamadas que cada tela faz ao abrir. O navegador as faz em paralelo: vale a mais lenta."""
    return {
        "Painel": [f"/api/painel?ano={hoje.year}&mes={hoje.month}", "/api/painel/avisos", "/api/contas"],
        "Transações": ["/api/transacoes?pagina=1&tamanhoPagina=50", "/api/categorias", "/api/contas"],
        "Investimentos": ["/api/carteira", "/api/investimentos", "/api/reservas"],
    }


class Sessao:
    """Cliente mínimo: guarda os cookies e manda o anti-CSRF nos POST, como o Angular."""

    def __init__(self, porta):
        self.porta = porta
        self.cookies = {}

    def pedir(self, metodo, caminho, corpo=None):
        conexao = http.client.HTTPSConnection("127.0.0.1", self.porta, context=SEM_VERIFICAR, timeout=120)
        cabecalhos = {"Cookie": "; ".join(f"{k}={v}" for k, v in self.cookies.items())}
        if corpo is not None:
            cabecalhos["Content-Type"] = "application/json"
            cabecalhos["X-XSRF-TOKEN"] = self.cookies.get("__Host-XSRF-TOKEN", "")
            corpo = json.dumps(corpo)
        try:
            conexao.request(metodo, caminho, corpo, cabecalhos)
            resposta = conexao.getresponse()
            resposta.read()
            for nome, valor in resposta.getheaders():
                if nome.lower() == "set-cookie":
                    chave, _, resto = valor.split(";", 1)[0].partition("=")
                    self.cookies[chave] = resto
            return resposta.status
        finally:
            conexao.close()

    def responde(self):
        try:
            return self.pedir("GET", "/api/auth/estado") == 200
        except OSError:
            return False

    def enviar(self, caminho, corpo):
        self.pedir("GET", "/api/auth/estado")  # traz o cookie anti-CSRF
        return self.pedir("POST", caminho, corpo)


def internet_recusada():
    """Uma porta que recusa na hora. Numa porta fechada, o Windows leva 2 s para desistir de cada conexão, e a
    medida viraria essa espera: o GET /api/carteira tenta o BCB enquanto não tem uma consulta recente guardada."""
    servidor = socket.create_server(("127.0.0.1", 0))

    def atender():
        while True:
            conexao, _ = servidor.accept()
            conexao.sendall(b"HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n")
            conexao.close()

    threading.Thread(target=atender, daemon=True).start()
    return f"http://127.0.0.1:{servidor.getsockname()[1]}"


def subir(api, pasta, porta, ambiente):
    """O Go com a linha do serviço, mas com o banco, as chaves e um certificado em memória desta rodada."""
    nome = "financas-go.exe" if os.name == "nt" else "financas-go"
    return subprocess.Popen([str(api / nome), "-pasta", str(api), "-endereco", f"127.0.0.1:{porta}",
                             "-certificado-temporario", "-banco", str(pasta / "dados/sql/FinancasLocal.db"),
                             "-chaves", str(pasta / "dados/chaves"), "-aplicar-esquema", "-rotinas-de-partida"],
                            cwd=api, env=ambiente, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def parar(processo):
    if os.name == "nt":
        # Pelo PID deste processo. Nunca pelo nome: o site instalado tem o mesmo nome.
        subprocess.run(["taskkill", "/PID", str(processo.pid), "/T", "/F"], capture_output=True)
    else:
        processo.terminate()
    processo.wait(30)


def esperar_site(sessao, processo):
    """Espera a primeira resposta e devolve quantos segundos se passaram desde a chamada."""
    inicio = time.perf_counter()
    while time.perf_counter() - inicio < 120:
        if processo.poll() is not None:
            sys.exit(f"O site parou ao iniciar (código {processo.returncode}).")
        if sessao.responde():
            return time.perf_counter() - inicio
        time.sleep(0.05)
    sys.exit("O site não respondeu em 120 s.")


def criar_volume(banco, anos, por_mes, hoje):
    """Repete as transações fictícias do contrato em cada mês; só a data e o id externo mudam."""
    con = sqlite3.connect(banco)
    con.executescript(BASE_SQL.read_text(encoding="utf-8"))
    colunas = [c[1] for c in con.execute("PRAGMA table_info(Transacao)")]
    id_externo = colunas.index("PluggyTransactionId")
    data = colunas.index("Data")
    modelos = con.execute("SELECT * FROM Transacao ORDER BY Id").fetchall()
    espacos = ", ".join("?" for _ in colunas)
    sql = f"INSERT INTO Transacao ({', '.join(colunas)}) VALUES ({espacos})"
    linhas = []
    for m in range(anos * 12):
        ano, mes = divmod(hoje.year * 12 + hoje.month - 1 - m, 12)
        for i in range(por_mes):
            linha = list(modelos[(m * por_mes + i) % len(modelos)])
            linha[0] = None
            linha[id_externo] = f"medida-{m}-{i}"
            linha[data] = f"{ano:04d}-{mes + 1:02d}-{i % 28 + 1:02d}" + str(linha[data])[10:]
            linhas.append(linha)
    con.executemany(sql, linhas)
    con.commit()
    total = con.execute("SELECT COUNT(*) FROM Transacao").fetchone()[0]
    con.close()
    return total


def medir(args, pasta):
    api = Path(args.api).resolve()
    (pasta / "dados/sql").mkdir(parents=True)
    (pasta / "dados/chaves").mkdir(parents=True)
    banco = pasta / "dados/sql/FinancasLocal.db"
    proxy = internet_recusada()
    ambiente = dict(os.environ, HTTPS_PROXY=proxy, HTTP_PROXY=proxy, NO_PROXY="127.0.0.1")

    # A primeira partida cria o banco, como numa instalação nova.
    sessao = Sessao(args.porta)
    processo = subir(api, pasta, args.porta, ambiente)
    try:
        esperar_site(sessao, processo)
        if sessao.enviar("/api/auth/configurar", {"login": LOGIN, "senha": SENHA}) not in (200, 204):
            sys.exit("Não deu para criar o usuário de teste.")
    finally:
        parar(processo)

    hoje = datetime.date.today()
    total = criar_volume(banco, args.anos, args.por_mes, hoje)
    print(f"Volume: {total} transações, {args.anos} anos, {banco.stat().st_size / 1e6:.0f} MB de banco")

    partidas = []
    for _ in range(args.vezes):
        sessao = Sessao(args.porta)
        processo = subir(api, pasta, args.porta, ambiente)
        try:
            partidas.append(esperar_site(sessao, processo))
        finally:
            parar(processo)
    print("Partida: " + ", ".join(f"{p:.2f} s" for p in partidas) + f" (mediana {statistics.median(partidas):.2f} s)")

    sessao = Sessao(args.porta)
    processo = subir(api, pasta, args.porta, ambiente)
    try:
        esperar_site(sessao, processo)
        if sessao.enviar("/api/auth/entrar", {"login": LOGIN, "senha": SENHA}) != 200:
            sys.exit("Não deu para entrar com o usuário de teste.")
        for tela, chamadas in telas(hoje).items():
            tempos = {}
            for caminho in chamadas:
                sessao.pedir("GET", caminho)  # a primeira aquece; as outras são as que contam
                medidas = []
                for _ in range(args.repeticoes):
                    inicio = time.perf_counter()
                    situacao = sessao.pedir("GET", caminho)
                    medidas.append((time.perf_counter() - inicio) * 1000)
                    if situacao != 200:
                        sys.exit(f"GET {caminho}: {situacao}")
                tempos[caminho.split("?")[0]] = statistics.median(medidas)
            detalhe = ", ".join(f"{c} {t:.0f}" for c, t in tempos.items())
            print(f"{tela}: {max(tempos.values()):.0f} ms ({detalhe})")
    finally:
        parar(processo)


def main():
    entrada = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    entrada.add_argument("--api", required=True, help="pasta publicada, com o financas-go e o appsettings.json")
    entrada.add_argument("--porta", type=int, default=5461)
    entrada.add_argument("--anos", type=int, default=5)
    entrada.add_argument("--por-mes", type=int, default=150)
    entrada.add_argument("--vezes", type=int, default=3, help="partidas medidas")
    entrada.add_argument("--repeticoes", type=int, default=5, help="medidas por chamada")
    args = entrada.parse_args()
    pasta = Path(tempfile.mkdtemp(prefix="medir-desempenho-"))
    try:
        medir(args, pasta)
    finally:
        shutil.rmtree(pasta, ignore_errors=True)


if __name__ == "__main__":
    main()
