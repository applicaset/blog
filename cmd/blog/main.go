package main

import (
	"github.com/applicaset/blog/app"
	"github.com/applicaset/pkg/serve"
)

func main() { serve.Main(app.Run) }
