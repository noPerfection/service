module github.com/noPerfection/service/examples/009-single-process

go 1.25.0

require (
	github.com/noPerfection/datatype v0.0.0
	github.com/noPerfection/os v0.0.0
	github.com/noPerfection/protocol/client v0.0.0
	github.com/noPerfection/protocol/handler v0.0.0
	github.com/noPerfection/protocol/message v0.0.0
	github.com/noPerfection/service v0.0.0
	github.com/noPerfection/topology/config v0.0.0
)

require (
	github.com/ahmetson/mushroom v0.0.0 // indirect
	github.com/anthropics/anthropic-sdk-go v1.26.0 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/cakturk/go-netstat v0.0.0-20200220111822-e5b49efee7a5 // indirect
	github.com/charmbracelet/lipgloss v0.8.0 // indirect
	github.com/charmbracelet/log v0.2.4 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/joho/godotenv v1.4.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mozilla-ai/any-llm-go v0.9.0 // indirect
	github.com/muesli/clusters v0.0.0-20200529215643-2700303c1762 // indirect
	github.com/muesli/gamut v0.3.1 // indirect
	github.com/muesli/kmeans v0.3.1 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/noPerfection/log v0.0.0 // indirect
	github.com/noPerfection/topology v0.0.0-20260618053405-c0164a6cc6e0 // indirect
	github.com/pebbe/zmq4 v1.2.10 // indirect
	github.com/phayes/freeport v0.0.0-20220201140144-74d24b5ae9f5 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/tools v0.46.0 // indirect
)

replace github.com/noPerfection/datatype => ../../../datatype

replace github.com/ahmetson/mushroom => ../../../../ahmetson/mushroom

replace github.com/noPerfection/log => ../../../log

replace github.com/noPerfection/os => ../../../os

replace github.com/noPerfection/protocol/client => ../../../protocol/client

replace github.com/noPerfection/protocol/handler => ../../../protocol/handler

replace github.com/noPerfection/protocol/message => ../../../protocol/message

replace github.com/noPerfection/service => ../..

replace github.com/noPerfection/topology => ../../../topology

replace github.com/noPerfection/topology/config => ../../../topology/config
