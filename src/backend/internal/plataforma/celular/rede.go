package celular

import (
	"net"
	"slices"
	"strings"
)

// Endereco é um IP deste computador na rede de casa, com a interface e a sub-rede dele.
type Endereco struct {
	IP        string `json:"ip"`
	Interface string `json:"interface"`
	Rede      string `json:"rede"` // a sub-rede, ex.: 192.168.0.0/24 (a regra do firewall do Linux usa)
}

// interfaceDeRede é o que o EnderecosDoComputador lê de cada net.Interface: separado para o teste montar a lista.
type interfaceDeRede struct {
	nome      string
	ativa     bool
	loopback  bool
	enderecos []net.Addr
}

// daRedeDeCasa diz se o IP é de uma rede de casa: IPv4 privado (10/8, 172.16/12 e 192.168/16, RFC 1918), o que o
// roteador dá aos aparelhos atrás dele. IP público, de loopback, de link local (169.254) ou IPv6 fica de fora: o site
// nunca escuta num endereço alcançável pela internet.
func daRedeDeCasa(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4.IsPrivate()
}

// enderecosDe filtra as interfaces ativas, sem a de loopback, e fica só com os IPs da rede de casa. O preferido (o da
// rota padrão, quando se sabe) vem primeiro; os outros, pelo nome da interface e pelo IP.
func enderecosDe(interfaces []interfaceDeRede, preferido string) []Endereco {
	lista := []Endereco{}
	for _, i := range interfaces {
		if !i.ativa || i.loopback {
			continue
		}
		for _, a := range i.enderecos {
			rede, ok := a.(*net.IPNet)
			if !ok || !daRedeDeCasa(rede.IP) {
				continue
			}
			ip := rede.IP.To4()
			sub := &net.IPNet{IP: ip.Mask(rede.Mask), Mask: rede.Mask}
			lista = append(lista, Endereco{IP: ip.String(), Interface: i.nome, Rede: sub.String()})
		}
	}
	slices.SortStableFunc(lista, func(a, b Endereco) int {
		if (a.IP == preferido) != (b.IP == preferido) {
			if a.IP == preferido {
				return -1
			}
			return 1
		}
		if c := strings.Compare(a.Interface, b.Interface); c != 0 {
			return c
		}
		return strings.Compare(a.IP, b.IP)
	})
	return lista
}

// O que vem do sistema; o teste troca.
var (
	interfacesDoSistema  = net.Interfaces
	enderecosDaInterface = func(i net.Interface) ([]net.Addr, error) { return i.Addrs() }
	discar               = net.Dial
)

// EnderecosDoComputador lista os IPs deste computador na rede de casa (ver enderecosDe).
func EnderecosDoComputador() ([]Endereco, error) {
	todas, err := interfacesDoSistema()
	if err != nil {
		return nil, err
	}
	lista := make([]interfaceDeRede, 0, len(todas))
	for _, i := range todas {
		enderecos, err := enderecosDaInterface(i)
		if err != nil {
			continue // uma interface que não responde não impede as outras
		}
		lista = append(lista, interfaceDe(i, enderecos))
	}
	return enderecosDe(lista, ipDaRotaPadrao()), nil
}

// interfaceDe lê do net.Interface o que o enderecosDe usa: o nome e se está ligada ou é a de loopback.
func interfaceDe(i net.Interface, enderecos []net.Addr) interfaceDeRede {
	return interfaceDeRede{
		nome: i.Name, ativa: i.Flags&net.FlagUp != 0, loopback: i.Flags&net.FlagLoopback != 0, enderecos: enderecos,
	}
}

// ipDaRotaPadrao é o IP que o sistema usaria para sair pela rota padrão: abrir um "UDP" para um endereço de
// documentação (192.0.2.1, RFC 5737) não manda nenhum pacote, só escolhe a interface. Sem rede, volta vazio.
func ipDaRotaPadrao() string {
	conexao, err := discar("udp4", "192.0.2.1:9")
	if err != nil {
		return ""
	}
	defer conexao.Close()
	if a, ok := conexao.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}
