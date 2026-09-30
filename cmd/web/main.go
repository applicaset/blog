package main

import (
	webapp "github.com/applicaset/buildset/blog/web/app"
	"github.com/applicaset/buildset/pkg/serve"
)

func main() { serve.Main(webapp.Run) }
