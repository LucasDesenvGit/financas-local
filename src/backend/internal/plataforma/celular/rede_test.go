package celular

import (
	"errors"
	"net"
	"reflect"
	"testing"
)

// Regra (#328): o site só escuta num IPv4 privado (RFC 1918), o da rede de casa. Nunca num IP alcançável pela internet,
// nem no loopback (esse é o do computador), nem no de link local ou IPv6.
func TestDaRedeDeCasa(t *testing.T) {
	for ip, esperado := range map[string]bool{
		"10.0.0.1":        true,
		"10.255.255.254":  true,
		"172.16.0.1":      true,
		"172.31.255.1":    true,
		"192.168.0.10":    true,
		"192.168.255.1":   true,
		"172.15.0.1":      false, // logo antes de 172.16/12
		"172.32.0.1":      false, // logo depois
		"192.169.0.1":     false,
		"11.0.0.1":        false,
		"8.8.8.8":         false,
		"127.0.0.1":       false,
		"169.254.10.1":    false,
		"0.0.0.0":         false,
		"::1":             false,
		"fd00::1":         false, // IPv6 privado: fora, de propósito (só IPv4)
		"::ffff:10.0.0.1": true,  // o mesmo 10.0.0.1, escrito como IPv6
	} {
		if obtido := daRedeDeCasa(net.ParseIP(ip)); obtido != esperado {
			t.Errorf("%s: %v, esperado %v", ip, obtido, esperado)
		}
	}
}

func rede(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, sub, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	sub.IP = ip // o endereço da interface, e não o da rede
	return sub
}

// Regra (#328): a lista para escolher tem só os IPs da rede de casa das interfaces ligadas, com a sub-rede, e o da
// rota padrão primeiro; os outros pelo nome da interface e pelo IP.
func TestEnderecosDe(t *testing.T) {
	interfaces := []interfaceDeRede{
		{nome: "loopback", ativa: true, loopback: true, enderecos: []net.Addr{rede(t, "127.0.0.1/8")}},
		{nome: "desligada", ativa: false, enderecos: []net.Addr{rede(t, "192.168.5.5/24")}},
		{nome: "Wi-Fi", ativa: true, enderecos: []net.Addr{
			rede(t, "192.168.0.10/24"), rede(t, "fe80::1/64"), &net.IPAddr{IP: net.ParseIP("192.168.0.11")},
		}},
		// O IP público vem antes dos bons: pular um endereço não pode pular os seguintes.
		{nome: "Ethernet", ativa: true, enderecos: []net.Addr{rede(t, "200.1.2.3/24"), rede(t, "10.0.0.20/8"), rede(t, "10.0.0.3/8")}},
		// O IP da VPN viria antes dos da Ethernet pela ordem do IP: a ordem é a da interface, primeiro.
		{nome: "VPN", ativa: true, enderecos: []net.Addr{rede(t, "10.0.0.1/8")}},
	}
	esperado := []Endereco{
		{IP: "192.168.0.10", Interface: "Wi-Fi", Rede: "192.168.0.0/24"},
		{IP: "10.0.0.20", Interface: "Ethernet", Rede: "10.0.0.0/8"},
		{IP: "10.0.0.3", Interface: "Ethernet", Rede: "10.0.0.0/8"},
		{IP: "10.0.0.1", Interface: "VPN", Rede: "10.0.0.0/8"},
	}
	if obtido := enderecosDe(interfaces, "192.168.0.10"); !reflect.DeepEqual(obtido, esperado) {
		t.Errorf("com o preferido:\n%v\nesperado\n%v", obtido, esperado)
	}
	// Sem saber a rota padrão: só pelo nome e pelo IP.
	semPreferido := []Endereco{esperado[1], esperado[2], esperado[3], esperado[0]}
	if obtido := enderecosDe(interfaces, ""); !reflect.DeepEqual(obtido, semPreferido) {
		t.Errorf("sem o preferido:\n%v\nesperado\n%v", obtido, semPreferido)
	}
	// O preferido no meio da lista também sobe.
	meio := []Endereco{esperado[2], esperado[1], esperado[3], esperado[0]}
	if obtido := enderecosDe(interfaces, "10.0.0.3"); !reflect.DeepEqual(obtido, meio) {
		t.Errorf("preferido no meio:\n%v\nesperado\n%v", obtido, meio)
	}
	if obtido := enderecosDe(nil, ""); obtido == nil || len(obtido) != 0 {
		t.Errorf("sem interfaces: %#v, esperado [] (vira [] no JSON, e não null)", obtido)
	}
}

// EnderecosDoComputador lê as interfaces de verdade: o que sair tem de ser da rede de casa, com a sub-rede certa.
func TestEnderecosDoComputador(t *testing.T) {
	lista, err := EnderecosDoComputador()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range lista {
		ip := net.ParseIP(e.IP)
		_, sub, err := net.ParseCIDR(e.Rede)
		if !daRedeDeCasa(ip) || err != nil || !sub.Contains(ip) || e.Interface == "" {
			t.Errorf("endereço fora da regra: %+v", e)
		}
	}
}

// interfaceDe lê se a interface está ligada e se é a de loopback pelos Flags.
func TestInterfaceDe(t *testing.T) {
	for nome, c := range map[string]struct {
		flags           net.Flags
		ativa, loopback bool
	}{
		"ligada":             {net.FlagUp | net.FlagBroadcast, true, false},
		"desligada":          {net.FlagBroadcast, false, false},
		"loopback ligada":    {net.FlagUp | net.FlagLoopback, true, true},
		"loopback desligada": {net.FlagLoopback, false, true},
	} {
		enderecos := []net.Addr{rede(t, "192.168.0.10/24")}
		obtido := interfaceDe(net.Interface{Name: "eth0", Flags: c.flags}, enderecos)
		if obtido.nome != "eth0" || obtido.ativa != c.ativa || obtido.loopback != c.loopback || len(obtido.enderecos) != 1 {
			t.Errorf("%s: %+v", nome, obtido)
		}
	}
}

// trocar põe um falso no lugar do que vem do sistema, só durante o teste.
func trocar[T any](t *testing.T, alvo *T, falso T) {
	t.Helper()
	antes := *alvo
	*alvo = falso
	t.Cleanup(func() { *alvo = antes })
}

// O que vem do sistema: interfaces que falham, uma que não responde (as outras continuam) e a rota padrão.
func TestEnderecosDoComputadorComOSistemaFalso(t *testing.T) {
	t.Run("sem interfaces", func(t *testing.T) {
		trocar(t, &interfacesDoSistema, func() ([]net.Interface, error) { return nil, errors.New("sem permissão") })
		if lista, err := EnderecosDoComputador(); err == nil || lista != nil {
			t.Errorf("%v %v", lista, err)
		}
	})
	t.Run("uma interface não responde", func(t *testing.T) {
		trocar(t, &interfacesDoSistema, func() ([]net.Interface, error) {
			return []net.Interface{{Name: "quebrada", Flags: net.FlagUp}, {Name: "Wi-Fi", Flags: net.FlagUp}}, nil
		})
		trocar(t, &enderecosDaInterface, func(i net.Interface) ([]net.Addr, error) {
			if i.Name == "quebrada" {
				return []net.Addr{rede(t, "10.9.9.9/8")}, errors.New("não responde")
			}
			return []net.Addr{rede(t, "192.168.0.10/24")}, nil
		})
		trocar(t, &discar, func(string, string) (net.Conn, error) { return nil, errors.New("sem rede") })
		lista, err := EnderecosDoComputador()
		if err != nil || !reflect.DeepEqual(lista, []Endereco{{IP: "192.168.0.10", Interface: "Wi-Fi", Rede: "192.168.0.0/24"}}) {
			t.Errorf("%v %v", lista, err)
		}
	})
}

// A rota padrão: o IP local da "conexão" UDP; sem rede, ou sem endereço UDP, vazio.
func TestIpDaRotaPadrao(t *testing.T) {
	trocar(t, &discar, func(string, string) (net.Conn, error) { return net.Dial("udp4", "127.0.0.1:9") })
	if ip := ipDaRotaPadrao(); ip != "127.0.0.1" {
		t.Errorf("com rede: %q", ip)
	}
	trocar(t, &discar, func(string, string) (net.Conn, error) { return nil, errors.New("sem rede") })
	if ip := ipDaRotaPadrao(); ip != "" {
		t.Errorf("sem rede: %q", ip)
	}
	trocar(t, &discar, func(string, string) (net.Conn, error) { a, _ := net.Pipe(); return a, nil })
	if ip := ipDaRotaPadrao(); ip != "" {
		t.Errorf("sem endereço UDP: %q", ip)
	}
}
