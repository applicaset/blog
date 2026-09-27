package main

import (
	"github.com/buildset/buildset/blog/app"
	"github.com/buildset/buildset/pkg/serve"
)

func main() { serve.Main(app.Run) }
