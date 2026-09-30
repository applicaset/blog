package main

import (
	"github.com/applicaset/buildset/blog/app"
	"github.com/applicaset/buildset/pkg/serve"
)

func main() { serve.Main(app.Run) }
