package celular

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

// comandoFalso grava cada comando e responde pelo que a tabela diz (a chave é o comando com os argumentos, separados
// por espaço). Comando fora da tabela responde vazio e sem erro.
type comandoFalso struct {
	rodados   []string
	respostas map[string]respostaFalsa
}

type respostaFalsa struct {
	saida string
	err   error
}

func (c *comandoFalso) rodar(_ context.Context, programa string, argumentos ...string) ([]byte, error) {
	linha := strings.Join(append([]string{programa}, argumentos...), " ")
	c.rodados = append(c.rodados, linha)
	r := c.respostas[linha]
	return []byte(r.saida), r.err
}

var (
	casa       = Endereco{IP: "192.168.0.10", Interface: "Wi-Fi", Rede: "192.168.0.0/24"}
	mostrar    = "netsh advfirewall firewall show rule name=FinancasLocal-celular"
	apagar     = "netsh advfirewall firewall delete rule name=FinancasLocal-celular"
	criar      = "netsh advfirewall firewall add rule name=FinancasLocal-celular dir=in action=allow protocol=TCP enable=yes localip=192.168.0.10 localport=5443 remoteip=localsubnet profile=private"
	semRegra   = respostaFalsa{saida: "Nenhuma regra corresponde aos critérios especificados.", err: errors.New("exit status 1")}
	perfil     = "powershell.exe -NoProfile -NonInteractive -Command (Get-NetConnectionProfile -InterfaceIndex (Get-NetIPAddress -IPAddress '192.168.0.10' -ErrorAction Stop).InterfaceIndex -ErrorAction Stop).NetworkCategory"
	falhaFalsa = errors.New("exit status 1")
)

// Regra (#328, ADR 0061): no Windows, a regra é de entrada, só TCP na porta e no IP do site, só para a sub-rede local
// e só no perfil Particular. Abrir de novo troca a regra (apaga a que existe), e não duplica.
func TestNetshAbrir(t *testing.T) {
	t.Run("sem regra antes: só cria", func(t *testing.T) {
		c := &comandoFalso{respostas: map[string]respostaFalsa{mostrar: semRegra}}
		if err := (netsh{rodar: c.rodar}).Abrir(t.Context(), casa, 5443); err != nil {
			t.Fatal(err)
		}
		if esperado := []string{mostrar, criar}; !reflect.DeepEqual(c.rodados, esperado) {
			t.Errorf("rodou %q\nesperado %q", c.rodados, esperado)
		}
	})
	t.Run("com regra antes: apaga e cria", func(t *testing.T) {
		c := &comandoFalso{}
		if err := (netsh{rodar: c.rodar}).Abrir(t.Context(), casa, 5443); err != nil {
			t.Fatal(err)
		}
		if esperado := []string{mostrar, apagar, criar}; !reflect.DeepEqual(c.rodados, esperado) {
			t.Errorf("rodou %q\nesperado %q", c.rodados, esperado)
		}
	})
	t.Run("criar falha: o erro diz o que o netsh disse", func(t *testing.T) {
		c := &comandoFalso{respostas: map[string]respostaFalsa{
			mostrar: semRegra, criar: {saida: "A operação solicitada\r\nrequer elevação.", err: falhaFalsa},
		}}
		err := (netsh{rodar: c.rodar}).Abrir(t.Context(), casa, 5443)
		if err == nil || !errors.Is(err, falhaFalsa) || !strings.Contains(err.Error(), "A operação solicitada requer elevação.") ||
			!strings.Contains(err.Error(), "como serviço") {
			t.Errorf("erro %v", err)
		}
	})
	t.Run("apagar a antiga falha: não cria", func(t *testing.T) {
		c := &comandoFalso{respostas: map[string]respostaFalsa{apagar: {saida: "negado", err: falhaFalsa}}}
		if err := (netsh{rodar: c.rodar}).Abrir(t.Context(), casa, 5443); !errors.Is(err, falhaFalsa) {
			t.Errorf("erro %v", err)
		}
		if esperado := []string{mostrar, apagar}; !reflect.DeepEqual(c.rodados, esperado) {
			t.Errorf("rodou %q\nesperado %q", c.rodados, esperado)
		}
	})
}

// Regra (#328): desligar apaga a regra; sem regra, não faz nada (e não é erro).
func TestNetshFechar(t *testing.T) {
	t.Run("sem regra", func(t *testing.T) {
		c := &comandoFalso{respostas: map[string]respostaFalsa{mostrar: semRegra}}
		if err := (netsh{rodar: c.rodar}).Fechar(t.Context()); err != nil {
			t.Fatal(err)
		}
		if esperado := []string{mostrar}; !reflect.DeepEqual(c.rodados, esperado) {
			t.Errorf("rodou %q", c.rodados)
		}
	})
	t.Run("com regra", func(t *testing.T) {
		c := &comandoFalso{}
		if err := (netsh{rodar: c.rodar}).Fechar(t.Context()); err != nil {
			t.Fatal(err)
		}
		if esperado := []string{mostrar, apagar}; !reflect.DeepEqual(c.rodados, esperado) {
			t.Errorf("rodou %q", c.rodados)
		}
	})
	t.Run("apagar falha", func(t *testing.T) {
		c := &comandoFalso{respostas: map[string]respostaFalsa{apagar: {saida: "negado", err: falhaFalsa}}}
		err := (netsh{rodar: c.rodar}).Fechar(t.Context())
		if !errors.Is(err, falhaFalsa) || !strings.Contains(err.Error(), "negado") {
			t.Errorf("erro %v", err)
		}
	})
}

// Regra (#328): a regra só vale no perfil Particular; numa rede que o Windows marcou como Pública, a tela avisa como
// mudar. Sem resposta do PowerShell, não inventa aviso.
func TestNetshAvisos(t *testing.T) {
	for nome, c := range map[string]struct {
		resposta respostaFalsa
		avisos   []string
	}{
		"pública":          {respostaFalsa{saida: "Public\r\n"}, []string{AvisoRedePublica}},
		"particular":       {respostaFalsa{saida: "Private\r\n"}, []string{}},
		"de domínio":       {respostaFalsa{saida: "DomainAuthenticated\r\n"}, []string{}},
		"powershell falha": {respostaFalsa{saida: "Public", err: falhaFalsa}, []string{}},
	} {
		t.Run(nome, func(t *testing.T) {
			f := &comandoFalso{respostas: map[string]respostaFalsa{perfil: c.resposta}}
			if obtido := (netsh{rodar: f.rodar}).Avisos(t.Context(), casa, 5443); !reflect.DeepEqual(obtido, c.avisos) {
				t.Errorf("avisos %q, esperado %q", obtido, c.avisos)
			}
			if esperado := []string{perfil}; !reflect.DeepEqual(f.rodados, esperado) {
				t.Errorf("rodou %q", f.rodados)
			}
		})
	}
}

func TestLimparSaida(t *testing.T) {
	if obtido := limparSaida([]byte("  linha 1\r\n\r\n  linha 2  ")); obtido != "linha 1 linha 2" {
		t.Errorf("%q", obtido)
	}
	longa := strings.Repeat("a", 301)
	if obtido := limparSaida([]byte(longa)); obtido != strings.Repeat("a", 300)+"…" {
		t.Errorf("longa: %d caracteres", len(obtido))
	}
	if obtido := limparSaida([]byte(strings.Repeat("a", 300))); obtido != strings.Repeat("a", 300) {
		t.Errorf("300 cabem inteiros: %d", len(obtido))
	}
}

// Regra (#328): no Linux o serviço não tem root, então não mexe no firewall: com o ufw ou o firewalld ligados, diz o
// comando para a sub-rede do IP; sem eles, não avisa nada.
func TestLinuxAvisos(t *testing.T) {
	ufw := "O firewall ufw está ligado. Para o celular chegar ao site, rode uma vez no terminal: sudo ufw allow from 192.168.0.0/24 to 192.168.0.10 port 5443 proto tcp"
	firewalld := `O firewalld está ligado. Para o celular chegar ao site, rode uma vez no terminal: sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="192.168.0.0/24" port port="5443" protocol="tcp" accept' && sudo firewall-cmd --reload`
	ativo := "systemctl is-active firewalld"
	for nome, c := range map[string]struct {
		conf      string
		errConf   error
		firewalld respostaFalsa
		avisos    []string
	}{
		"nenhum ligado":                {"ENABLED=no\n", nil, respostaFalsa{saida: "inactive\n", err: falhaFalsa}, []string{}},
		"sem ufw instalado":            {"", fs.ErrNotExist, respostaFalsa{saida: "inactive\n", err: falhaFalsa}, []string{}},
		"ufw ligado":                   {"# comentário\nENABLED=yes\nLOGLEVEL=low\n", nil, respostaFalsa{saida: "inactive\n", err: falhaFalsa}, []string{ufw}},
		"firewalld ligado":             {"ENABLED=no\n", nil, respostaFalsa{saida: "active\n"}, []string{firewalld}},
		"os dois":                      {"ENABLED=yes\n", nil, respostaFalsa{saida: "active\n"}, []string{ufw, firewalld}},
		"firewalld com saída estranha": {"ENABLED=no\n", nil, respostaFalsa{saida: "activating\n"}, []string{}},
	} {
		t.Run(nome, func(t *testing.T) {
			f := &comandoFalso{respostas: map[string]respostaFalsa{ativo: c.firewalld}}
			l := linux{rodar: f.rodar, lerArquivo: func(caminho string) ([]byte, error) {
				if caminho != "/etc/ufw/ufw.conf" {
					t.Errorf("leu %s", caminho)
				}
				return []byte(c.conf), c.errConf
			}}
			if obtido := l.Avisos(t.Context(), casa, 5443); !reflect.DeepEqual(obtido, c.avisos) {
				t.Errorf("avisos %q\nesperado %q", obtido, c.avisos)
			}
		})
	}
	// Abrir e Fechar não mexem em nada (nem rodam comando).
	f := &comandoFalso{}
	l := linux{rodar: f.rodar}
	if err := l.Abrir(t.Context(), casa, 5443); err != nil {
		t.Error(err)
	}
	if err := l.Fechar(t.Context()); err != nil {
		t.Error(err)
	}
	if len(f.rodados) != 0 {
		t.Errorf("rodou %q", f.rodados)
	}
}

func TestUfwLigado(t *testing.T) {
	for conf, esperado := range map[string]bool{
		"ENABLED=yes":               true,
		"ENABLED=YES\n":             true,
		` ENABLED = "yes" `:         true,
		"ENABLED='yes'":             true,
		"ENABLED=no":                false,
		"#ENABLED=yes\nENABLED=no":  false,
		"# ENABLED=yes":             false,
		"LOGLEVEL=low\nENABLED=yes": true,
		"ENABLED=no\nENABLED=yes":   true, // vale a última, como no shell que lê o arquivo
		"ENABLED=yes\nENABLED=no":   false,
		"ENABLEDX=yes":              false,
		"":                          false,
		"ENABLED":                   false,
	} {
		if obtido := ufwLigado(conf); obtido != esperado {
			t.Errorf("%q: %v, esperado %v", conf, obtido, esperado)
		}
	}
}

// O firewall de verdade é o do sistema em que o teste roda.
func TestFirewallDoSistema(t *testing.T) {
	switch FirewallDoSistema().(type) {
	case netsh, linux:
	default:
		t.Errorf("tipo %T", FirewallDoSistema())
	}
}
