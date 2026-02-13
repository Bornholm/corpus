package main

import (
	"github.com/bornholm/corpus/internal/command"
	"github.com/bornholm/corpus/internal/command/index"
	"github.com/bornholm/corpus/internal/command/watch"
)

func main() {
	command.Main(
		"corpus-cli", "a corpus client tool",
		watch.Command(),
		index.Command(),
	)
}
