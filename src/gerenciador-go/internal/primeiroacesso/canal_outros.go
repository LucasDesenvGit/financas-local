//go:build !windows

package primeiroacesso

// Pedir: fora do Windows a pasta das chaves é da conta do usuário, e o gerenciador a lê direto (sem elevação).
var Pedir PedirElevado
