package main

import (
	contentapp "github.com/applicaset/buildset/blog/content/app"
	"github.com/applicaset/buildset/pkg/serve"
)

func main() { serve.Main(contentapp.Run) }
