package main

import (
	contentapp "github.com/applicaset/blog/content/app"
	"github.com/applicaset/pkg/serve"
)

func main() { serve.Main(contentapp.Run) }
