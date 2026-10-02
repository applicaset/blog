package main

import (
	webapp "github.com/applicaset/blog/web/app"
	"github.com/applicaset/pkg/serve"
)

func main() { serve.Main(webapp.Run) }
