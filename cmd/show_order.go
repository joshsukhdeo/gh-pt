package cmd

import (
	"os"
	"strings"
)

func getShowFlagOrder() []string {
	var order []string
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--versions") || arg == "-v" {
			order = append(order, "versions")
		} else if strings.HasPrefix(arg, "--assets") {
			order = append(order, "assets")
		} else if strings.HasPrefix(arg, "--description") {
			order = append(order, "description")
		} else if strings.HasPrefix(arg, "--readme") {
			order = append(order, "readme")
		}
	}
	return order
}
