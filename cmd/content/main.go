package main

import (
	contentapp "github.com/buildset/buildset/blog/content/app"
	"github.com/buildset/buildset/pkg/serve"
)

func main() { serve.Main(contentapp.Run) }
