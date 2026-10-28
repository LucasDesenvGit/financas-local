package main

import "golang.org/x/sys/windows"

// asfwQualquer é o ASFW_ANY: qualquer processo pode trazer a sua janela para a frente.
const asfwQualquer = ^uintptr(0)

var allowSetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// permitirPrimeiroPlano: quem você abriu é que tem o direito de vir para a frente. Abrindo o gerenciador de novo, essa
// segunda cópia só avisa a primeira e fecha, e é a primeira que mostra a janela; sem passar o direito adiante, o
// Windows deixa a janela atrás das outras.
func permitirPrimeiroPlano() { _, _, _ = allowSetForegroundWindow.Call(asfwQualquer) }
