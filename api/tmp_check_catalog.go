package main

import (
	"fmt"
	svc "github.com/enjoydarts/sifto/api/internal/service"
)

func main() {
	c := svc.LLMCatalogData()
	counts := map[string]int{}
	for _, m := range c.ChatModels {
		counts[m.ID]++
	}
	for id, n := range counts {
		if n > 1 {
			fmt.Println(id, n)
		}
	}
}
