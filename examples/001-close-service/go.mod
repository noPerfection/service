module github.com/noPerfection/service/examples/001-close-service

go 1.19

require (
	github.com/noPerfection/datatype v0.0.0
	github.com/noPerfection/os v0.0.0
	github.com/noPerfection/protocol/client v0.0.0
	github.com/noPerfection/protocol/message v0.0.0
	github.com/noPerfection/service v0.0.0
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/lipgloss v0.8.0 // indirect
	github.com/charmbracelet/log v0.2.4 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/muesli/clusters v0.0.0-20200529215643-2700303c1762 // indirect
	github.com/muesli/gamut v0.3.1 // indirect
	github.com/muesli/kmeans v0.3.1 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/noPerfection/log v0.0.0 // indirect
	github.com/noPerfection/protocol/handler v0.0.0 // indirect
	github.com/noPerfection/topology v0.0.0-20260602170203-c8fa84f22f77 // indirect
	github.com/noPerfection/topology/config v0.0.0 // indirect
	github.com/pebbe/zmq4 v1.2.10 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	golang.org/x/sys v0.12.0 // indirect
)

replace github.com/noPerfection/datatype => ../../../datatype

replace github.com/noPerfection/log => ../../../log

replace github.com/noPerfection/os => ../../../os

replace github.com/noPerfection/protocol/client => ../../../protocol/client

replace github.com/noPerfection/protocol/handler => ../../../protocol/handler

replace github.com/noPerfection/protocol/message => ../../../protocol/message

replace github.com/noPerfection/service => ../..

replace github.com/noPerfection/topology => ../../../topology

replace github.com/noPerfection/topology/config => ../../../topology/config
