package main

import (
	webapp "github.com/buildset/buildset/blog/web/app"
	"github.com/buildset/buildset/pkg/serve"
)

func main() { serve.Main(webapp.Run) }
